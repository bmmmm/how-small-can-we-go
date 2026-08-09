// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"strings"
	"testing"
)

// Test syntaxes mirroring languages.json — kept in code so scanner tests
// don't depend on the repo file.
func goSyntax(t *testing.T) *Syntax {
	t.Helper()
	s := &Syntax{
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings: []StringSyntax{
			{Open: `"`, Close: `"`, Escape: `\`},
			{Open: "`", Close: "`", Multiline: true},
			{Open: `'`, Close: `'`, Escape: `\`},
		},
		BlockCommentIsNewline: true,
		NoDiscountFile:        []string{`\breflect\b`},
		KeepComments:          []string{`^//go:`},
	}
	if err := s.compile(); err != nil {
		t.Fatal(err)
	}
	return s
}

func pySyntax(t *testing.T) *Syntax {
	t.Helper()
	s := &Syntax{
		LineComments: []string{"#"},
		Strings: []StringSyntax{
			{Open: `"""`, Close: `"""`, Escape: `\`, Multiline: true},
			{Open: `'''`, Close: `'''`, Escape: `\`, Multiline: true},
			{Open: `"`, Close: `"`, Escape: `\`},
			{Open: `'`, Close: `'`, Escape: `\`},
		},
		WsSensitive:          true,
		NoDiscountFile:       []string{`__`, `\b(globals|locals|vars|dir|eval|exec|getattr)\s*\(`},
		NoDiscountFileExempt: []string{`if __name__ == "__main__":`, `__init__`},
		NoDiscountLine:       []string{`[fF][rRbB]?["']`},
		KeepComments:         []string{`^#!`},
	}
	if err := s.compile(); err != nil {
		t.Fatal(err)
	}
	return s
}

func units(t *testing.T, src string, syn *Syntax) int {
	t.Helper()
	r := measureFile([]byte(src), syn)
	if r.note != "" {
		t.Fatalf("measureFile(%q) fell back to byte pricing: %s", src, r.note)
	}
	return r.units
}

// The core alignment property: a semantics-preserving minification —
// short names, no comments — must measure EQUAL, so "equal is not
// smaller" closes the strip-and-rename challenger.
func TestMinifiedTwinMeasuresEqual(t *testing.T) {
	syn := goSyntax(t)
	readable := "// checksum of one file\nsum := hashfile(reader, \"x\")\n"
	minified := "s:=h(r,\"x\")\n"
	ur, um := units(t, readable, syn), units(t, minified, syn)
	if ur != um {
		t.Errorf("readable = %d units, minified = %d units — uglification must not pay", ur, um)
	}
}

func TestCommentsAreFreeAndStripped(t *testing.T) {
	syn := goSyntax(t)
	r := measureFile([]byte("x = 1 // a note\n"), syn)
	if r.units != 3 {
		t.Errorf("units = %d, want 3 (x, =, 1)", r.units)
	}
	if string(r.norm) != "x = 1\n" {
		t.Errorf("norm = %q, want comment gone", r.norm)
	}
}

func TestKeepCommentsArePricedAndKept(t *testing.T) {
	syn := goSyntax(t)
	r := measureFile([]byte("//go:embed data\nx\n"), syn)
	if !strings.Contains(string(r.norm), "//go:embed data") {
		t.Errorf("norm = %q, semantic comment must survive", r.norm)
	}
	if r.units != 15 { // 14 non-ws comment bytes + x
		t.Errorf("units = %d, want 15 — semantic comments are priced", r.units)
	}
}

func TestStringContentCostsEveryByte(t *testing.T) {
	syn := goSyntax(t)
	if u := units(t, "x = \"a b\"\n", syn); u != 7 {
		t.Errorf("units = %d, want 7 — string whitespace is data and costs", u)
	}
}

func TestLongIdentifierCostsExcess(t *testing.T) {
	syn := goSyntax(t)
	long := strings.Repeat("a", 20)
	if u := units(t, long+"\n", syn); u != 5 { // 1 + (20-16)
		t.Errorf("units = %d, want 5 — names beyond %d bytes cost their excess", u, identFlatLen)
	}
}

// 0x-prefixed data must not ride the identifier discount: an alnum run
// glued to a digit is priced per byte.
func TestHexLiteralIsPricedPerByte(t *testing.T) {
	syn := goSyntax(t)
	if u := units(t, "x=0x4141414141414141\n", syn); u != 20 {
		t.Errorf("units = %d, want 20 — hex data must cost its mass", u)
	}
}

func TestReflectionDoorForcesBytePricing(t *testing.T) {
	syn := goSyntax(t)
	r := measureFile([]byte("import \"reflect\"\n"), syn)
	if r.note == "" {
		t.Fatal("reflect not flagged — identifier names would become a data channel")
	}
	if r.units != 15 { // plain non-ws bytes
		t.Errorf("units = %d, want 15 (byte pricing)", r.units)
	}
}

func TestDunderTriggersBytePricing(t *testing.T) {
	r := measureFile([]byte("x = y.__dict__\n"), pySyntax(t))
	if r.note == "" {
		t.Fatal("__dict__ not flagged")
	}
}

func TestMainGuardIsExempt(t *testing.T) {
	src := "if __name__ == \"__main__\":\n    main()\n"
	r := measureFile([]byte(src), pySyntax(t))
	if r.note != "" {
		t.Fatalf("the __main__ idiom must not cost the discounts: %s", r.note)
	}
	if string(r.norm) != src {
		t.Errorf("norm = %q, want indentation preserved", r.norm)
	}
}

// f-string lines are a lexing hazard (nested quotes can desync a scanner
// within the line): the whole physical line is priced at plain bytes and
// ships verbatim — nothing on it is treated as a comment.
func TestFStringLineIsBytePricedVerbatim(t *testing.T) {
	syn := pySyntax(t)
	r := measureFile([]byte("y = 1\nz = f\"{d} #x\"\n"), syn)
	if r.note != "" {
		t.Fatalf("file-level fallback not expected: %s", r.note)
	}
	if r.units != 3+11 { // y,=,1 + 11 non-ws bytes of the f-string line
		t.Errorf("units = %d, want 14", r.units)
	}
	if !strings.Contains(string(r.norm), "f\"{d} #x\"") {
		t.Errorf("norm = %q — nothing on an f-string line may be stripped", r.norm)
	}
}

// The C exploit this rule exists for: a backslash-newline inside a block
// comment splices `*\` + `/` into `*/`, so the compiler ends the comment
// earlier than a naive scanner — code the scanner thinks is free would
// execute. No proof, no discount.
func TestBlockCommentLineSpliceForcesBytePricing(t *testing.T) {
	syn := goSyntax(t)
	r := measureFile([]byte("/* x *\\\n/ evil(); /* y */\n"), syn)
	if r.note == "" {
		t.Fatal("line splice inside block comment not flagged — the comment discount is exploitable")
	}
}

func TestRustRawStringTriggersBytePricing(t *testing.T) {
	s := &Syntax{
		LineComments:   []string{"//"},
		BlockComments:  [][2]string{{"/*", "*/"}},
		Strings:        []StringSyntax{{Open: `"`, Close: `"`, Escape: `\`, Multiline: true}},
		NoDiscountFile: []string{`'"`, `\br#*"`},
	}
	if err := s.compile(); err != nil {
		t.Fatal(err)
	}
	if r := measureFile([]byte("let s = r#\"a \" // b\"#;\n"), s); r.note == "" {
		t.Fatal("raw string not flagged — its unescaped quotes desync the scanner")
	}
	if r := measureFile([]byte("let c = '\"';\n"), s); r.note == "" {
		t.Fatal("quote char literal not flagged")
	}
}

func TestUnterminatedMultilineStringForcesBytePricing(t *testing.T) {
	syn := goSyntax(t)
	if r := measureFile([]byte("x := `abc\n"), syn); r.note == "" {
		t.Fatal("unterminated raw string not flagged")
	}
}

func TestNormalizationPython(t *testing.T) {
	src := "#!/usr/bin/env python3\n# strip me\n\ndef f(x):\n    # gone\n    return x  \n\nf(1)\n"
	want := "#!/usr/bin/env python3\ndef f(x):\n    return x\nf(1)\n"
	r := measureFile([]byte(src), pySyntax(t))
	if r.note != "" {
		t.Fatalf("unexpected fallback: %s", r.note)
	}
	if string(r.norm) != want {
		t.Errorf("norm = %q\nwant   %q", r.norm, want)
	}
}

// A removed block comment that spans lines must act as a newline in Go —
// semicolon insertion depends on it.
func TestGoBlockCommentBecomesNewline(t *testing.T) {
	r := measureFile([]byte("a()\n/*\nc\n*/\nb()\n"), goSyntax(t))
	if string(r.norm) != "a()\nb()\n" {
		t.Errorf("norm = %q, want %q", r.norm, "a()\nb()\n")
	}
}

func TestNilSyntaxMeansBytePricing(t *testing.T) {
	r := measureFile([]byte("cat \"$1\"\n"), nil)
	if r.units != 7 || r.note == "" {
		t.Errorf("nil syntax: units = %d (want 7), note = %q (want non-empty)", r.units, r.note)
	}
	if string(r.norm) != "cat \"$1\"\n" {
		t.Errorf("nil syntax must ship verbatim, got %q", r.norm)
	}
}
