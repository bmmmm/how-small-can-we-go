// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"strings"
	"testing"
)

// The repo's real languages.json is the deployed measurement config — a
// pattern that silently stops matching would open a pricing hole no
// hand-written test double could catch. Exercise every language's
// triggers against the committed file.
func repoLanguages(t *testing.T) map[string]Language {
	t.Helper()
	langs, err := LoadLanguages("../../..")
	if err != nil {
		t.Fatalf("repo languages.json broken: %v", err)
	}
	return langs
}

func TestRepoConfigCompilesAndHasExtensions(t *testing.T) {
	for name, lang := range repoLanguages(t) {
		if name == "sh" {
			if lang.Syntax != nil {
				t.Errorf("sh grew a syntax config — heredocs defeat safe comment detection, review deliberately")
			}
			continue
		}
		if lang.Syntax == nil {
			t.Errorf("language %q has no syntax config", name)
		}
	}
}

func TestRepoConfigTriggers(t *testing.T) {
	langs := repoLanguages(t)
	cases := []struct {
		lang string
		src  string
		want string // substring of the expected note; "" = discounts must apply
	}{
		{"go", "import \"reflect\"\n", "no-discount"},
		{"go", "import \"runtime\"\n", "no-discount"},
		{"go", "fmt.Printf(\"%+v\", x)\n", "no-discount"},
		{"go", "x := 1 // fine\n", ""},
		{"python", "print(globals())\n", "no-discount"},
		{"python", "x = y.__dict__\n", "no-discount"},
		{"python", "import inspect\n", "no-discount"},
		{"python", "x = 1  # fine\n", ""},
		{"c", "#define S(x) #x\n", "no-discount"},
		{"c", "puts(__func__);\n", "no-discount"},
		{"c", "int x = 1; // fine\n", ""},
		{"rust", "let s = r#\"x\"#;\n", "no-discount"},
		{"rust", "stringify!(abc)\n", "no-discount"},
		{"rust", "#[derive(Debug)]\nstruct S { field_name: u8 }\n", "no-discount"},
		{"rust", "let x = 1; // fine\n", ""},
	}
	for _, c := range cases {
		syn := langs[c.lang].Syntax
		if syn == nil {
			t.Fatalf("%s: no syntax", c.lang)
		}
		r := measureFile([]byte(c.src), syn)
		if c.want == "" && r.note != "" {
			t.Errorf("%s: %q unexpectedly lost discounts: %s", c.lang, c.src, r.note)
		}
		if c.want != "" && !strings.Contains(r.note, c.want) {
			t.Errorf("%s: %q not flagged (note %q)", c.lang, c.src, r.note)
		}
	}
}

// The __main__ guard and __init__ are idiomatic Python, not reflection
// doors — they must keep their discounts against the real config.
func TestRepoConfigPythonExemptions(t *testing.T) {
	syn := repoLanguages(t)["python"].Syntax
	src := "class A:\n    def __init__(self):\n        pass\nif __name__ == \"__main__\":\n    A()\n"
	if r := measureFile([]byte(src), syn); r.note != "" {
		t.Errorf("idiomatic dunders lost the discounts: %s", r.note)
	}
	// The doors themselves must stay shut even next to an exempt idiom.
	src += "print(A().__dict__)\n"
	if r := measureFile([]byte(src), syn); r.note == "" {
		t.Error("__dict__ slipped past the exemptions")
	}
}

func TestRepoConfigFStringLineTrigger(t *testing.T) {
	syn := repoLanguages(t)["python"].Syntax
	r := measureFile([]byte("x = 1\ny = f\"{x}\"\n"), syn)
	if r.note != "" {
		t.Fatalf("f-string must trigger per line, not per file: %s", r.note)
	}
	if !strings.Contains(string(r.norm), "f\"{x}\"") {
		t.Errorf("f-string line must ship verbatim, norm = %q", r.norm)
	}
}
