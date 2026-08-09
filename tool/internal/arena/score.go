// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Trust scoring. The score ranks how much unreviewed trust an entry
// demands, in two dimensions compared lexicographically:
//
//  1. vendored bytes — every byte under a vendor/ directory is code the
//     submitter did not write for this entry. Third-party code MUST live
//     there (SPEC.md); the ideal is zero.
//  2. hazard count — occurrences of the language's declared hazard
//     patterns (process execution, dynamic evaluation, reflection, FFI,
//     unsafe memory). Each hit is named with file, line, and reason.
//
// Comments are stripped before the hazard scan wherever the language's
// declared syntax lets the scanner prove the strip safe — documentation
// must never cost a hazard. Anything doubtful (a lex guard match, a
// construct the scanner cannot lex, a language without a strip config)
// is scanned raw, comments included. A mistake can therefore only ever
// overcount hazards, never hide one: fail-suspicious by construction.

// Hazard is one dangerous construct of a language: a pattern to count
// and the reason it costs. Curated in languages.json, extended by PR.
type Hazard struct {
	Pattern string `json:"pattern"`
	Why     string `json:"why"`

	re *regexp.Regexp
}

// Strip describes the lexical shape of a language just precisely enough
// to remove comments before the hazard scan. String literals are
// tracked so a comment marker inside a string never eats code, but they
// are NOT removed: a hazard call spelled in code always sits outside a
// string, and blanking string content could hide nothing anyway.
type Strip struct {
	LineComments  []string       `json:"lineComments,omitempty"`
	BlockComments [][2]string    `json:"blockComments,omitempty"`
	Strings       []StringSyntax `json:"strings,omitempty"`
	// KeepComments marks comments that carry semantics (`//go:`
	// directives, shebangs): they survive the strip — and stay visible
	// to the hazard scan, which is how `//go:linkname` gets counted.
	KeepComments []string `json:"keepComments,omitempty"`
	// GuardFile patterns mark files the scanner cannot lex safely
	// (Rust raw strings, C trigraphs): the whole file scans raw.
	GuardFile []string `json:"guardFile,omitempty"`
	// GuardLine patterns mark single lines whose comment markers cannot
	// be trusted (Python f-strings): those lines keep their bytes.
	GuardLine []string `json:"guardLine,omitempty"`

	fileRe, lineRe, keepRe []*regexp.Regexp
}

type StringSyntax struct {
	Open   string `json:"open"`
	Close  string `json:"close"`
	Escape string `json:"escape,omitempty"`
	// Multiline strings may span raw newlines (Go backticks, Python
	// triple quotes, Rust strings). Single-line strings are force-closed
	// at the newline, which re-syncs the scanner at every line end.
	Multiline bool `json:"multiline,omitempty"`
}

// compile validates a language config and prepares its patterns. Called
// once at load time so a broken languages.json fails loudly, not per
// entry.
func (l *Language) compile() error {
	for i := range l.Hazards {
		h := &l.Hazards[i]
		if h.Why == "" {
			return fmt.Errorf("hazard %q needs a why — every hazard is a documented, argued cost", h.Pattern)
		}
		re, err := regexp.Compile(h.Pattern)
		if err != nil {
			return fmt.Errorf("hazard pattern %q: %w", h.Pattern, err)
		}
		h.re = re
	}
	s := l.Strip
	if s == nil {
		return nil
	}
	s.fileRe, s.lineRe, s.keepRe = nil, nil, nil
	comp := func(pats []string, dst *[]*regexp.Regexp, field string) error {
		for _, p := range pats {
			re, err := regexp.Compile(p)
			if err != nil {
				return fmt.Errorf("%s pattern %q: %w", field, p, err)
			}
			*dst = append(*dst, re)
		}
		return nil
	}
	if err := comp(s.GuardFile, &s.fileRe, "guardFile"); err != nil {
		return err
	}
	if err := comp(s.GuardLine, &s.lineRe, "guardLine"); err != nil {
		return err
	}
	if err := comp(s.KeepComments, &s.keepRe, "keepComments"); err != nil {
		return err
	}
	for _, str := range s.Strings {
		if str.Open == "" || str.Close == "" {
			return fmt.Errorf("string syntax needs open and close markers")
		}
	}
	// Longest openers first, so `"""` wins over `"`.
	sort.SliceStable(s.Strings, func(i, j int) bool {
		return len(s.Strings[i].Open) > len(s.Strings[j].Open)
	})
	return nil
}

// sourceFile reports whether rel has one of the language's source
// extensions. Only source files are hazard-scanned; everything else is
// data to its runtime.
func (l Language) sourceFile(rel string) bool {
	for _, ext := range l.Extensions {
		if strings.HasSuffix(rel, ext) {
			return true
		}
	}
	return false
}

// HazardHit is one counted occurrence of a hazard pattern.
type HazardHit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
	Why     string `json:"why"`
}

// Score is the trust score of an entry directory. Lower is better,
// compared lexicographically: vendored bytes first, hazards second.
type Score struct {
	VendoredBytes int         `json:"vendoredBytes"`
	HazardCount   int         `json:"hazardCount"`
	Hazards       []HazardHit `json:"hazards,omitempty"`
	Files         int         `json:"files"`
	Notes         []string    `json:"notes,omitempty"` // files scanned raw (comments count) and why
}

// Better reports whether s strictly beats c. Equal is not better: the
// champion defends ties.
func (s Score) Better(c Score) bool {
	if s.VendoredBytes != c.VendoredBytes {
		return s.VendoredBytes < c.VendoredBytes
	}
	return s.HazardCount < c.HazardCount
}

// ScoreDir scores every file under dir except the entry.json manifest.
// Vendored bytes are everything under a vendor/ path segment — data
// files included, because a vendored blob is trusted freight either
// way. Hazards are counted in every source file, vendored or not: code
// you ship is code that runs.
func ScoreDir(dir string, lang Language) (Score, error) {
	var sc Score
	err := walkEntryFiles(dir, func(path, rel string, b []byte) error {
		sc.Files++
		if isVendored(rel) {
			sc.VendoredBytes += len(b)
		}
		if !lang.sourceFile(rel) {
			return nil
		}
		text := b
		if lang.Strip == nil {
			sc.Notes = append(sc.Notes, rel+": no strip config for this language — scanned raw, comments count")
		} else if guard := matchRe(b, lang.Strip.fileRe); guard != "" {
			sc.Notes = append(sc.Notes, fmt.Sprintf("%s: matches lex guard %q — scanned raw, comments count", rel, guard))
		} else if stripped, why, ok := stripComments(b, lang.Strip); ok {
			text = stripped
		} else {
			sc.Notes = append(sc.Notes, fmt.Sprintf("%s: %s — scanned raw, comments count", rel, why))
		}
		for _, h := range lang.Hazards {
			for _, loc := range h.re.FindAllIndex(text, -1) {
				sc.HazardCount++
				sc.Hazards = append(sc.Hazards, HazardHit{
					File:    rel,
					Line:    1 + bytes.Count(text[:loc[0]], []byte("\n")),
					Pattern: h.Pattern,
					Why:     h.Why,
				})
			}
		}
		return nil
	})
	if err != nil {
		return Score{}, err
	}
	if sc.Files == 0 {
		return Score{}, fmt.Errorf("%s contains no files to score", dir)
	}
	return sc, nil
}

// isVendored reports whether rel (a slash path) sits under a vendor/
// directory at any depth.
func isVendored(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "vendor" {
			return true
		}
	}
	return false
}

// stripComments blanks every non-semantic comment in b — comment bytes
// become spaces, newlines survive — so hazard offsets and line numbers
// still match the committed file. String literals are tracked but kept.
// ok=false means a construct the scanner cannot lex safely was found;
// the caller scans the file raw instead (fail-suspicious).
func stripComments(b []byte, st *Strip) (out []byte, why string, ok bool) {
	out = bytes.Clone(b)
	trig := triggeredLines(b, st)
	line := 0
	pos := 0
	for pos < len(b) {
		c := b[pos]
		if c == '\n' {
			line++
			pos++
			continue
		}
		// A guarded line may hide a comment marker inside a construct
		// the scanner cannot see (an f-string): nothing on it is
		// treated as a comment or a string — its bytes stay.
		if trig != nil && trig[line] {
			pos++
			continue
		}
		if m := matchAny(b, pos, st.LineComments); m != "" {
			end := lineEnd(b, pos)
			comment := b[pos:end]
			// A backslash at the end of a line comment splices the next
			// line into it in C — the compiler's comment is longer than
			// ours, so stripping here could blank real code elsewhere
			// or keep commented-out code visible. No proof, no strip.
			if bytes.HasSuffix(bytes.TrimRight(comment, "\r"), []byte("\\")) {
				return nil, "line splice at the end of a line comment", false
			}
			if !matchRegexps(comment, st.keepRe) {
				blank(out[pos:end])
			}
			pos = end
			continue
		}
		if open, cl, found := matchBlock(b, pos, st.BlockComments); found {
			end := bytes.Index(b[pos+len(open):], []byte(cl))
			if end < 0 {
				return nil, "unterminated block comment", false
			}
			comment := b[pos : pos+len(open)+end+len(cl)]
			// A backslash before a line break inside a block comment can
			// splice the closer apart in C — the compiler would end the
			// comment elsewhere than we do. No proof, no strip.
			if bytes.Contains(comment, []byte("\\\n")) {
				return nil, "line splice inside a block comment", false
			}
			// A nested opener ends earlier here than in Rust (whose
			// block comments nest) — stripping would misjudge the span.
			if bytes.Contains(comment[len(open):len(comment)-len(cl)], []byte(open)) {
				return nil, "nested block comment", false
			}
			if !matchRegexps(comment, st.keepRe) {
				blank(out[pos : pos+len(comment)])
			}
			line += bytes.Count(comment, []byte("\n"))
			pos += len(comment)
			continue
		}
		// Strings — tracked so a comment marker inside one never eats
		// code; their bytes stay for the hazard scan.
		if str, found := matchString(b, pos, st.Strings); found {
			end, nl, sok := stringEnd(b, pos, str)
			if !sok {
				return nil, "unterminated string literal", false
			}
			line += nl
			pos = end
			continue
		}
		pos++
	}
	return out, "", true
}

// blank overwrites every byte except newlines with a space.
func blank(b []byte) {
	for i, c := range b {
		if c != '\n' {
			b[i] = ' '
		}
	}
}

// triggeredLines maps each physical line to whether a GuardLine pattern
// matches it. nil when the language has no line guards.
func triggeredLines(b []byte, st *Strip) []bool {
	if len(st.lineRe) == 0 {
		return nil
	}
	lines := bytes.Split(b, []byte("\n"))
	trig := make([]bool, len(lines))
	for i, l := range lines {
		trig[i] = matchRegexps(l, st.lineRe)
	}
	return trig
}

// stringEnd finds the end of a string literal opened at pos. Single-line
// strings are force-closed at a raw newline — that is what the grammars
// this scanner trusts do, and it re-syncs the scanner at every line
// end. Returns the index one past the literal and the newline count.
func stringEnd(b []byte, pos int, str StringSyntax) (end, newlines int, ok bool) {
	i := pos + len(str.Open)
	for i < len(b) {
		if bytes.HasPrefix(b[i:], []byte(str.Close)) {
			return i + len(str.Close), newlines, true
		}
		if str.Escape != "" && bytes.HasPrefix(b[i:], []byte(str.Escape)) && i+len(str.Escape) < len(b) {
			if b[i+len(str.Escape)] == '\n' {
				newlines++
			}
			i += len(str.Escape) + 1
			continue
		}
		if b[i] == '\n' {
			if !str.Multiline {
				return i, newlines, true // force-closed at the line end
			}
			newlines++
		}
		i++
	}
	if !str.Multiline {
		return len(b), newlines, true
	}
	return 0, 0, false
}

func matchAny(b []byte, pos int, markers []string) string {
	for _, m := range markers {
		if bytes.HasPrefix(b[pos:], []byte(m)) {
			return m
		}
	}
	return ""
}

func matchBlock(b []byte, pos int, blocks [][2]string) (open, cl string, found bool) {
	for _, blk := range blocks {
		if bytes.HasPrefix(b[pos:], []byte(blk[0])) {
			return blk[0], blk[1], true
		}
	}
	return "", "", false
}

func matchString(b []byte, pos int, strs []StringSyntax) (StringSyntax, bool) {
	for _, s := range strs {
		if bytes.HasPrefix(b[pos:], []byte(s.Open)) {
			return s, true
		}
	}
	return StringSyntax{}, false
}

// matchRe returns the source of the first matching pattern, "" if none.
func matchRe(b []byte, res []*regexp.Regexp) string {
	for _, re := range res {
		if re.Match(b) {
			return re.String()
		}
	}
	return ""
}

func matchRegexps(b []byte, res []*regexp.Regexp) bool {
	return matchRe(b, res) != ""
}

func lineEnd(b []byte, pos int) int {
	if i := bytes.IndexByte(b[pos:], '\n'); i >= 0 {
		return pos + i
	}
	return len(b)
}
