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
