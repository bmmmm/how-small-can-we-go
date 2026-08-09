// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Options configures how entries are checked.
type Options struct {
	RepoRoot string
	Sandbox  bool // run build and cases inside a no-network container (docker)
}

type CaseResult struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Infra  bool   `json:"infra,omitempty"` // sandbox/runner failure — no verdict on the entry
	Detail string `json:"detail,omitempty"`
}

type Result struct {
	Entry    string       `json:"entry"`
	Task     string       `json:"task"`
	Language string       `json:"language"`
	Authors  []string     `json:"authors,omitempty"`
	Measure  Measure      `json:"measure"`
	Cases    []CaseResult `json:"cases,omitempty"`
	Pass     bool         `json:"pass"`
	Infra    bool         `json:"infra,omitempty"` // some failure was infrastructure, not the entry
	Err      string       `json:"error,omitempty"`
}

const (
	buildTimeout = 2 * time.Minute
	caseTimeout  = 20 * time.Second
)

// ErrInfra marks failures of the harness or sandbox itself — docker
// missing, or the daemon failing before the container ran (exit 125:
// rate-limited pull, daemon hiccup). Such a case has no verdict:
// counting it as a conformance failure would let a transient outage
// reject a correct entry, and counting exit 125 as the program's own
// exit would satisfy `exit: nonzero` cases. A timeout is NOT infra —
// a program that outruns the cap is a verdict on the entry.
var ErrInfra = errors.New("infrastructure failure")

// CheckEntry validates one entry: manifest, measurement, offline build,
// and every test case of its task.
func CheckEntry(entryDir string, opts Options) Result {
	res := Result{Entry: filepath.ToSlash(filepath.Clean(entryDir))}
	fail := func(format string, a ...any) Result {
		res.Err = fmt.Sprintf(format, a...)
		return res
	}
	man, err := LoadManifest(entryDir)
	if err != nil {
		return fail("%v", err)
	}
	res.Task, res.Language, res.Authors = man.Task, man.Language, man.Authors
	langs, err := LoadLanguages(opts.RepoRoot)
	if err != nil {
		return fail("%v", err)
	}
	lang, ok := langs[man.Language]
	if !ok {
		return fail("language %q is not in languages.json — propose it in a separate PR first", man.Language)
	}
	casesDir := filepath.Join(opts.RepoRoot, "tasks", man.Task, "cases")
	caseNames, err := listDirs(casesDir)
	if err != nil || len(caseNames) == 0 {
		return fail("task %q has no test cases under %s", man.Task, casesDir)
	}
	res.Measure, err = MeasureDir(entryDir)
	if err != nil {
		return fail("%v", err)
	}

	buildDir, err := os.MkdirTemp("", "arena-build-*")
	if err != nil {
		return fail("%v", err)
	}
	defer os.RemoveAll(buildDir)
	if err := copyTree(entryDir, buildDir); err != nil {
		return fail("%v", err)
	}
	if man.Build != "" {
		_, stderr, code, err := execute(buildDir, lang.Image, strings.Fields(man.Build), opts, buildTimeout)
		if err != nil {
			res.Infra = errors.Is(err, ErrInfra)
			return fail("build: %v", err)
		}
		if code != 0 {
			return fail("build failed (exit %d):\n%s", code, tail(stderr, 2000))
		}
	}
	runArgv := strings.Fields(man.Run)
	res.Pass = true
	for _, name := range caseNames {
		cr := runCase(buildDir, filepath.Join(casesDir, name), name, lang.Image, runArgv, opts)
		res.Cases = append(res.Cases, cr)
		if !cr.Pass {
			res.Pass = false
		}
		if cr.Infra {
			res.Infra = true
		}
	}
	return res
}

func runCase(buildDir, caseDir, name, image string, runArgv []string, opts Options) CaseResult {
	cr := CaseResult{Name: name}
	work, err := os.MkdirTemp("", "arena-case-*")
	if err != nil {
		cr.Detail = err.Error()
		return cr
	}
	defer os.RemoveAll(work)
	if err := copyTree(buildDir, work); err != nil {
		cr.Detail = err.Error()
		return cr
	}
	if fi, err := os.Stat(filepath.Join(caseDir, "files")); err == nil && fi.IsDir() {
		if err := copyTree(filepath.Join(caseDir, "files"), work); err != nil {
			cr.Detail = err.Error()
			return cr
		}
	}
	args, err := readArgs(filepath.Join(caseDir, "args"))
	if err != nil {
		cr.Detail = err.Error()
		return cr
	}
	wantOut, _ := os.ReadFile(filepath.Join(caseDir, "stdout")) // absent = empty expected
	wantExit := "0"
	if b, err := os.ReadFile(filepath.Join(caseDir, "exit")); err == nil {
		wantExit = strings.TrimSpace(string(b))
	}

	argv := append(append([]string{}, runArgv...), args...)
	stdout, stderr, code, err := execute(work, image, argv, opts, caseTimeout)
	if err != nil {
		cr.Infra = errors.Is(err, ErrInfra)
		cr.Detail = fmt.Sprintf("run: %v", err)
		return cr
	}
	if wantExit == "nonzero" {
		if code == 0 {
			cr.Detail = "expected a nonzero exit code, got 0"
			return cr
		}
	} else if strconv.Itoa(code) != wantExit {
		cr.Detail = fmt.Sprintf("exit code: got %d, want %s\nstderr: %s", code, wantExit, tail(stderr, 500))
		return cr
	}
	if !bytes.Equal(stdout, wantOut) {
		cr.Detail = fmt.Sprintf("stdout mismatch:\n  got  %s\n  want %s", preview(stdout), preview(wantOut))
		return cr
	}
	cr.Pass = true
	return cr
}

// execute runs argv with dir as working directory and returns captured
// stdout, stderr, and the exit code; err is reserved for infrastructure
// problems (a failing program is a code, not an error). In sandbox mode
// the command runs inside the pinned language image with networking
// disabled and resource caps applied.
func execute(dir, image string, argv []string, opts Options, timeout time.Duration) (stdout, stderr []byte, code int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if opts.Sandbox {
		docker := append([]string{
			"run", "--rm", "--network=none", "--cpus=1", "--memory=256m",
			"--pids-limit=256",
			"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"-e", "HOME=/tmp", "-e", "GOCACHE=/tmp/gocache",
			"-v", dir + ":/w", "-w", "/w", image,
		}, argv...)
		cmd = exec.CommandContext(ctx, "docker", docker...)
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = dir
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, -1, fmt.Errorf("timed out after %s", timeout)
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			// docker run reserves 125 for "docker itself failed" — the
			// container never ran. A program deliberately exiting 125
			// inside the container is indistinguishable; that false
			// positive only defers the PR to a human, never rejects.
			if opts.Sandbox && ee.ExitCode() == 125 {
				return nil, nil, -1, fmt.Errorf("%w: docker run exit 125: %s", ErrInfra, tail(errBuf.Bytes(), 500))
			}
			return outBuf.Bytes(), errBuf.Bytes(), ee.ExitCode(), nil
		}
		if opts.Sandbox {
			// argv[0] is always docker here, so a start failure is the
			// host's problem, never the entry's.
			return nil, nil, -1, fmt.Errorf("%w: %v — is docker available? (or use --no-sandbox for local checks)", ErrInfra, runErr)
		}
		// Without the sandbox, argv comes from the manifest's run/build
		// command: failing to start it is a verdict on the entry.
		return nil, nil, -1, fmt.Errorf("%v — is %s available?", runErr, argv[0])
	}
	return outBuf.Bytes(), errBuf.Bytes(), 0, nil
}

// copyTree copies src into dst. Symlinks are rejected — an entry or test
// case must not reference anything outside its own directory.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink — symlinks are not allowed", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}

// readArgs reads one argument per line; a trailing newline does not add
// an empty argument. A missing file means no arguments.
func readArgs(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines, nil
}

func listDirs(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func preview(b []byte) string {
	const max = 120
	if len(b) <= max {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q… (%d bytes total)", b[:max], len(b))
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "…" + string(b[len(b)-n:])
}
