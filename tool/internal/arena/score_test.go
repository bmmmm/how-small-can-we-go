// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"path/filepath"
	"strings"
	"testing"
)

// testLang returns a compiled Go-shaped language with one hazard, the
// shape most score tests need.
func testLang(t *testing.T) Language {
	t.Helper()
	lang := Language{
		Image:      "img",
		Extensions: []string{".go"},
		Strip: &Strip{
			LineComments:  []string{"//"},
			BlockComments: [][2]string{{"/*", "*/"}},
			Strings: []StringSyntax{
				{Open: `"`, Close: `"`, Escape: `\`},
			},
			KeepComments: []string{"^//go:"},
		},
		Hazards: []Hazard{{Pattern: `\bunsafe\.`, Why: "test hazard"}},
	}
	if err := lang.compile(); err != nil {
		t.Fatal(err)
	}
	return lang
}

func scoreOne(t *testing.T, lang Language, name, src string) Score {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, name), src)
	sc, err := ScoreDir(dir, lang)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func TestScoreCountsVendoredBytes(t *testing.T) {
	lang := testLang(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "vendor/lib/lib.go"), "package lib\n")
	writeFile(t, filepath.Join(dir, "vendor/lib/LICENSE"), "MIT\n")
	sc, err := ScoreDir(dir, lang)
	if err != nil {
		t.Fatal(err)
	}
	want := len("package lib\n") + len("MIT\n")
	if sc.VendoredBytes != want {
		t.Errorf("vendored bytes = %d, want %d — everything under vendor/ is trusted freight", sc.VendoredBytes, want)
	}
}

func TestScoreCountsHazardsWithFileAndLine(t *testing.T) {
	lang := testLang(t)
	sc := scoreOne(t, lang, "main.go", "package main\n\nvar p = unsafe.Pointer(nil)\n")
	if sc.HazardCount != 1 {
		t.Fatalf("hazard count = %d, want 1 (%+v)", sc.HazardCount, sc.Hazards)
	}
	h := sc.Hazards[0]
	if h.File != "main.go" || h.Line != 3 {
		t.Errorf("hazard located at %s:%d, want main.go:3", h.File, h.Line)
	}
	if h.Why == "" {
		t.Error("hazard hit lost its why — the report must explain the cost")
	}
}

// Hazards inside vendored source count too: code you ship is code that
// runs, wherever it sits in the tree.
func TestScoreCountsHazardsInVendoredCode(t *testing.T) {
	lang := testLang(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "vendor/lib/lib.go"), "package lib\nvar p = unsafe.Pointer(nil)\n")
	sc, err := ScoreDir(dir, lang)
	if err != nil {
		t.Fatal(err)
	}
	if sc.HazardCount != 1 {
		t.Errorf("hazard in vendor/ not counted: %+v", sc)
	}
}

// Documentation must never cost a hazard: a comment naming a hazard is
// stripped before the scan.
func TestScoreStripsCommentsBeforeScan(t *testing.T) {
	lang := testLang(t)
	sc := scoreOne(t, lang, "main.go",
		"package main\n// unsafe.Pointer is what we deliberately avoid here\n/* also unsafe.Pointer */\n")
	if sc.HazardCount != 0 {
		t.Errorf("comment mentioning a hazard was counted: %+v", sc.Hazards)
	}
}

// Semantic comments survive the strip — that is how //go:linkname stays
// visible to the hazard scan.
func TestScoreKeepCommentsStayVisible(t *testing.T) {
	lang := testLang(t)
	lang.Hazards = append(lang.Hazards, Hazard{Pattern: `//go:linkname`, Why: "invisible coupling"})
	if err := lang.compile(); err != nil {
		t.Fatal(err)
	}
	sc := scoreOne(t, lang, "main.go", "package main\n//go:linkname x runtime.y\n")
	if sc.HazardCount != 1 {
		t.Errorf("//go:linkname not counted although keepComments keeps it: %+v", sc)
	}
}

// Line numbers must survive the strip: blanked comments keep their
// newlines, so a hazard after a multi-line comment reports its true line.
func TestScoreLineNumbersSurviveStripping(t *testing.T) {
	lang := testLang(t)
	sc := scoreOne(t, lang, "main.go", "package main\n/*\n three\n line\n comment\n*/\nvar p = unsafe.Pointer(nil)\n")
	if len(sc.Hazards) != 1 || sc.Hazards[0].Line != 7 {
		t.Errorf("hazard line = %+v, want line 7", sc.Hazards)
	}
}

// A file the scanner cannot lex safely scans raw — comments included.
// Fail-suspicious: a doubtful corner may overcount, never hide.
func TestScoreRawScanOnLexFailure(t *testing.T) {
	lang := testLang(t)
	sc := scoreOne(t, lang, "main.go", "package main\n/* unterminated unsafe.Pointer\n")
	if len(sc.Notes) == 0 {
		t.Fatal("unlexable file produced no note")
	}
	if sc.HazardCount != 1 {
		t.Errorf("raw scan must count the comment mention: %+v", sc)
	}
}

// A language without a strip config scans raw and says so.
func TestScoreNoStripConfigScansRaw(t *testing.T) {
	lang := Language{Image: "img", Extensions: []string{".sh"}, Hazards: []Hazard{{Pattern: `\beval\b`, Why: "evaluates data as code"}}}
	if err := lang.compile(); err != nil {
		t.Fatal(err)
	}
	sc := scoreOne(t, lang, "main.sh", "# eval is avoided here\necho ok\n")
	if sc.HazardCount != 1 {
		t.Errorf("raw language must count comment mentions: %+v", sc)
	}
	if len(sc.Notes) == 0 || !strings.Contains(sc.Notes[0], "no strip config") {
		t.Errorf("raw scan not noted: %v", sc.Notes)
	}
}

// A guarded line keeps its bytes: a comment marker inside an f-string
// must not eat the rest of the line.
func TestScoreGuardLineScansRaw(t *testing.T) {
	lang := Language{
		Image:      "img",
		Extensions: []string{".py"},
		Strip: &Strip{
			LineComments: []string{"#"},
			Strings:      []StringSyntax{{Open: `"`, Close: `"`, Escape: `\`}},
			GuardLine:    []string{`[fF]["']`},
		},
		Hazards: []Hazard{{Pattern: `\beval\s*\(`, Why: "evaluates data as code"}},
	}
	if err := lang.compile(); err != nil {
		t.Fatal(err)
	}
	sc := scoreOne(t, lang, "main.py", "x = f\"{d}\" # eval( in a trailing comment\ny = 1 # eval( stripped fine\n")
	// Line 1 is guarded (f-string): its comment is not trusted, the
	// mention counts. Line 2 strips normally: no count.
	if sc.HazardCount != 1 || sc.Hazards[0].Line != 1 {
		t.Errorf("guarded line handling wrong: %+v", sc)
	}
}

// Data files are not source: no hazard scan, but vendored data still
// weighs.
func TestScoreDataFilesNotScanned(t *testing.T) {
	lang := testLang(t)
	sc := scoreOne(t, lang, "table.txt", "unsafe.Pointer all over\n")
	if sc.HazardCount != 0 {
		t.Errorf("data file was hazard-scanned: %+v", sc)
	}
}

func TestScoreBetterIsLexicographic(t *testing.T) {
	cases := []struct {
		a, b Score
		want bool
	}{
		{Score{VendoredBytes: 0, HazardCount: 5}, Score{VendoredBytes: 1, HazardCount: 0}, true},  // fewer vendored bytes dominates
		{Score{VendoredBytes: 1, HazardCount: 0}, Score{VendoredBytes: 0, HazardCount: 5}, false}, // more vendored bytes loses despite fewer hazards
		{Score{VendoredBytes: 3, HazardCount: 2}, Score{VendoredBytes: 3, HazardCount: 4}, true},  // tie on bytes, fewer hazards wins
		{Score{VendoredBytes: 3, HazardCount: 4}, Score{VendoredBytes: 3, HazardCount: 4}, false}, // equal is not better — the champion defends ties
	}
	for i, c := range cases {
		if got := c.a.Better(c.b); got != c.want {
			t.Errorf("case %d: (%d,%d).Better(%d,%d) = %v, want %v", i,
				c.a.VendoredBytes, c.a.HazardCount, c.b.VendoredBytes, c.b.HazardCount, got, c.want)
		}
	}
}

func TestHazardNeedsWhy(t *testing.T) {
	lang := Language{Image: "img", Extensions: []string{".go"}, Hazards: []Hazard{{Pattern: `x`}}}
	if err := lang.compile(); err == nil {
		t.Error("hazard without why compiled — every hazard must be an argued cost")
	}
}
