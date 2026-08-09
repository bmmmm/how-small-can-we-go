// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"path/filepath"
	"testing"
)

// The repo's real languages.json is the deployed scoring config — a
// hazard pattern that silently stops matching would open a scoring hole
// no hand-written test double could catch. Exercise every language's
// triggers against the committed file.
func repoLanguages(t *testing.T) map[string]Language {
	t.Helper()
	langs, err := LoadLanguages("../../..")
	if err != nil {
		t.Fatalf("repo languages.json broken: %v", err)
	}
	return langs
}

func TestRepoConfigShape(t *testing.T) {
	langs := repoLanguages(t)
	for _, name := range []string{"bash", "c", "go", "python", "rust"} {
		lang, ok := langs[name]
		if !ok {
			t.Errorf("language %q missing from languages.json", name)
			continue
		}
		if len(lang.Hazards) == 0 {
			t.Errorf("language %q has no hazard list — the second score dimension would be dead", name)
		}
	}
	if langs["bash"].Strip != nil {
		t.Error("bash grew a strip config — heredocs defeat safe comment detection, review deliberately")
	}
	for _, name := range []string{"c", "go", "python", "rust"} {
		if langs[name].Strip == nil {
			t.Errorf("language %q has no strip config — comments would cost hazards", name)
		}
	}
}

// Hazard triggers against the real config: dangerous constructs count,
// comments about them do not (except in raw-scanned bash — documented
// as fail-suspicious).
func TestRepoConfigHazardTriggers(t *testing.T) {
	langs := repoLanguages(t)
	cases := []struct {
		lang, file, src string
		want            int
	}{
		{"go", "m.go", "package m\nimport \"unsafe\"\nvar p = unsafe.Pointer(nil)\n", 1},
		{"go", "m.go", "package m\nimport \"os/exec\"\n", 1},
		{"go", "m.go", "package m\nimport \"reflect\"\nvar t = reflect.TypeOf(1)\n", 1},
		{"go", "m.go", "package m\n//go:linkname x runtime.y\n", 1},
		{"go", "m.go", "package m\n// unsafe imports and reflect tricks, discussed only\n", 0},
		{"go", "m.go", "package m\nvar x = 1\n", 0},
		// Evasion: aliased and dot imports still carry the import path.
		{"go", "m.go", "package m\nimport u \"unsafe\"\nvar p = u.Pointer(nil)\n", 1},
		{"go", "m.go", "package m\nimport . \"reflect\"\nvar t = TypeOf(1)\n", 1},
		{"python", "m.py", "eval(input())\n", 1},
		{"python", "m.py", "import subprocess\n", 1},
		{"python", "m.py", "x = getattr(o, name)\n", 1},
		{"python", "m.py", "# eval() is what we avoid\nx = 1\n", 0},
		{"python", "m.py", "print(\"hello\")\n", 0},
		// The f-string guard needs a real prefix position: a word-final
		// f before a quote (stuff") must not trigger the line guard.
		{"python", "m.py", "print(\"stuff\")  # eval mention in a comment\n", 0},
		// Evasion: aliasing the builtin is the same door.
		{"python", "m.py", "f = eval\nf(src)\n", 1},
		// Evasion: NFKC-normalized homoglyph identifiers execute as the
		// builtin — the non-ASCII run counts instead.
		{"python", "m.py", "x = ｅval(\"1+1\")\n", 1},
		// Evasion: a file without the .py extension still scans (raw).
		{"python", "prog", "eval(input())\n", 1},
		{"rust", "m.rs", "fn main() { unsafe { std::ptr::null::<u8>(); } }\n", 1},
		{"rust", "m.rs", "use std::process::Command;\n", 1},
		{"rust", "m.rs", "// unsafe is not used here\nfn main() {}\n", 0},
		// Evasion: module aliasing keeps the process:: path visible.
		{"rust", "m.rs", "use std as s;\nfn m() { s::process::exit(0); }\n", 1},
		{"c", "m.c", "int main(void) { system(\"ls\"); }\n", 1},
		{"c", "m.c", "char b[9]; int main(void) { strcpy(b, \"x\"); }\n", 1},
		{"c", "m.c", "#define GLUE(a, b) a##b\n", 1},
		{"c", "m.c", "/* system() would be wrong here */\nint main(void) { return 0; }\n", 0},
		// Evasion: a line splice must not split the name apart.
		{"c", "m.c", "int main(void) { sys\\\ntem(\"ls\"); }\n", 1},
		// Evasion: taking the function's address is the same door.
		{"c", "m.c", "int (*f)(const char *) = system;\n", 1},
		{"bash", "m.sh", "eval \"$cmd\"\n", 1},
		// bash scans raw: even a comment mention counts — documented
		// price of a language without provable comment stripping.
		{"bash", "m.sh", "# eval is avoided\necho ok\n", 1},
		{"bash", "m.sh", "echo \"${!ref}\"\n", 1},
		{"bash", "m.sh", "echo plain\n", 0},
		// Evasion: bash joins spliced lines before parsing.
		{"bash", "m.sh", "ev\\\nal \"$cmd\"\n", 1},
	}
	for _, c := range cases {
		lang := langs[c.lang]
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, c.file), c.src)
		sc, err := ScoreDir(dir, lang)
		if err != nil {
			t.Errorf("%s: %q: %v", c.lang, c.src, err)
			continue
		}
		if sc.HazardCount != c.want {
			t.Errorf("%s: %q counted %d hazards, want %d (%+v)", c.lang, c.src, sc.HazardCount, c.want, sc.Hazards)
		}
	}
}

// Rust raw strings desync the scanner, so such files scan raw — a
// comment mention then counts (fail-suspicious), never hides.
func TestRepoConfigRustRawStringGuard(t *testing.T) {
	lang := repoLanguages(t)["rust"]
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "m.rs"), "let s = r#\"x\"#; // unsafe mentioned\n")
	sc, err := ScoreDir(dir, lang)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Notes) == 0 {
		t.Fatal("raw string file was not guard-flagged")
	}
	if sc.HazardCount != 1 {
		t.Errorf("raw-scanned file must count the comment mention: %+v", sc)
	}
}

// Python f-string lines are guarded per line, not per file: the rest of
// the file still strips normally.
func TestRepoConfigPythonFStringGuardIsPerLine(t *testing.T) {
	lang := repoLanguages(t)["python"]
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "m.py"), "y = f\"{x}\"\n# eval() discussed in a comment\n")
	sc, err := ScoreDir(dir, lang)
	if err != nil {
		t.Fatal(err)
	}
	if sc.HazardCount != 0 {
		t.Errorf("comment on an unguarded line was counted: %+v", sc.Hazards)
	}
}
