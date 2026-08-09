// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testRepo builds a minimal arena repo with an echo-file task and a
// passing sh entry, checked in host mode (no docker in unit tests).
func testRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "languages.json"), `{"sh": {"image": "alpine:3.21"}}`)
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/hello/args"), "f.txt")
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/hello/files/f.txt"), "hi\n")
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/hello/stdout"), "hi\n")
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/missing/args"), "nope.txt")
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/missing/exit"), "nonzero")
	writeFile(t, filepath.Join(root, "entries/echo-file/sh/entry.json"),
		`{"task": "echo-file", "language": "sh", "authors": ["test"], "run": "sh main.sh"}`)
	writeFile(t, filepath.Join(root, "entries/echo-file/sh/main.sh"), "cat \"$1\"\n")
	return root
}

func TestCheckEntryPasses(t *testing.T) {
	root := testRepo(t)
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root})
	if res.Err != "" {
		t.Fatalf("unexpected error: %s", res.Err)
	}
	if !res.Pass {
		t.Fatalf("want pass, got %+v", res)
	}
	if len(res.Cases) != 2 {
		t.Fatalf("want 2 cases, got %d", len(res.Cases))
	}
	if res.Measure.Surface != 7 {
		t.Errorf("surface = %d, want 7", res.Measure.Surface)
	}
}

// The gate must be able to go red: a wrong expectation has to fail.
func TestCheckEntryDetectsWrongOutput(t *testing.T) {
	root := testRepo(t)
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/hello/stdout"), "bye\n")
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root})
	if res.Pass {
		t.Fatal("entry passed against a wrong expectation — the gate cannot go red")
	}
	var detail string
	for _, c := range res.Cases {
		if c.Name == "hello" {
			detail = c.Detail
		}
	}
	if !strings.Contains(detail, "stdout mismatch") {
		t.Errorf("failure detail not actionable: %q", detail)
	}
}

func TestCheckEntryDetectsWrongExit(t *testing.T) {
	root := testRepo(t)
	// Expect nonzero for a file that exists: cat succeeds, case must fail.
	writeFile(t, filepath.Join(root, "tasks/echo-file/cases/hello/exit"), "nonzero")
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root})
	if res.Pass {
		t.Fatal("entry passed although exit expectation is nonzero")
	}
}

// fakeDocker puts a docker stub with the given script body first in
// PATH, so sandbox-mode tests run without a real daemon.
func fakeDocker(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	path := filepath.Join(bin, "docker")
	writeFile(t, path, "#!/bin/sh\n"+script)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

// docker exit 125 means the container never ran (daemon error, e.g. a
// rate-limited pull). That is no verdict on the entry — and it must not
// satisfy `exit: nonzero` cases either.
func TestSandboxDockerDaemonFailureIsInfra(t *testing.T) {
	fakeDocker(t, "echo 'docker: Error response from daemon: toomanyrequests' >&2\nexit 125\n")
	root := testRepo(t)
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root, Sandbox: true})
	if !res.Infra {
		t.Fatalf("docker exit 125 not classified as infra: %+v", res)
	}
	if res.Pass {
		t.Fatal("docker exit 125 produced a PASS verdict")
	}
	for _, c := range res.Cases {
		if c.Pass {
			t.Errorf("case %s passed on docker exit 125 — the exit:nonzero mirror bug", c.Name)
		}
	}
}

// Exit 1 in sandbox mode is the program's own exit code, not docker's:
// it must stay a regular verdict, or every failing entry would read as
// an infra problem.
func TestSandboxProgramExitIsAVerdict(t *testing.T) {
	fakeDocker(t, "exit 1\n")
	root := testRepo(t)
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root, Sandbox: true})
	if res.Infra {
		t.Fatalf("program exit 1 misclassified as infra: %+v", res)
	}
	for _, c := range res.Cases {
		switch c.Name {
		case "hello": // expects exit 0
			if c.Pass {
				t.Error("hello passed although the program exited 1")
			}
		case "missing": // expects nonzero
			if !c.Pass {
				t.Errorf("missing failed although exit 1 satisfies nonzero: %s", c.Detail)
			}
		}
	}
}

func TestCheckEntryRejectsUnknownLanguage(t *testing.T) {
	root := testRepo(t)
	writeFile(t, filepath.Join(root, "entries/echo-file/sh/entry.json"),
		`{"task": "echo-file", "language": "cobol", "authors": ["test"], "run": "run"}`)
	res := CheckEntry(filepath.Join(root, "entries/echo-file/sh"), Options{RepoRoot: root})
	if res.Err == "" || !strings.Contains(res.Err, "languages.json") {
		t.Errorf("unknown language not rejected: %+v", res)
	}
}

func TestReadArgsKeepsSpacesDropsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "args")
	writeFile(t, path, "in put.bin\n")
	args, err := readArgs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != "in put.bin" {
		t.Errorf("args = %#v, want one arg with the space kept", args)
	}
	if args, _ := readArgs(filepath.Join(dir, "absent")); args != nil {
		t.Errorf("missing args file should mean no args, got %#v", args)
	}
}
