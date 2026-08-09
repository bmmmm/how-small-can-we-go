// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
)

// Audit-unit pricing. The surface counts what an auditor must process,
// not what a disk must store:
//
//   - a comment costs 0 — provably inert text is free to ship
//   - an identifier or keyword costs 1 (plus 1 per byte beyond
//     identFlatLen, so names cannot become a bulk data channel)
//   - string/number literals and everything else cost 1 per byte —
//     data is read byte by byte, so it is priced byte by byte
//   - whitespace outside literals costs 0
//
// Discounts are granted only where the scanner can PROVE them safe from
// the language's declared syntax. Anything doubtful — a construct the
// scanner cannot lex, a file matching a no-discount pattern, a language
// without a syntax config — is priced at 1 unit per non-whitespace byte.
// A mistake can therefore only ever overprice an entry, never underprice
// it: fail-expensive by construction.
const identFlatLen = 16

// Syntax describes the lexical shape of a language just precisely enough
// to grant measurement discounts. A language without one gets no
// discounts and ships verbatim.
type Syntax struct {
	LineComments  []string       `json:"lineComments,omitempty"`
	BlockComments [][2]string    `json:"blockComments,omitempty"`
	Strings       []StringSyntax `json:"strings,omitempty"`
	// WsSensitive keeps leading whitespace verbatim in the normalized
	// form (Python: indentation is grammar).
	WsSensitive bool `json:"wsSensitive,omitempty"`
	// BlockCommentIsNewline replaces a removed block comment that spans
	// lines with a newline instead of a space (Go: such a comment acts
	// as a newline for semicolon insertion).
	BlockCommentIsNewline bool `json:"blockCommentIsNewline,omitempty"`
	// NoDiscountFile patterns price the whole file at plain bytes.
	// They fence off reflection doors (constructs that read identifier
	// names as data, which flat identifier pricing would make cheap)
	// and multi-line constructs the scanner cannot lex safely.
	NoDiscountFile []string `json:"noDiscountFile,omitempty"`
	// NoDiscountFileExempt matches are removed before testing
	// NoDiscountFile — for idioms that are safe despite the pattern
	// (Python's `if __name__ == "__main__":`).
	NoDiscountFileExempt []string `json:"noDiscountFileExempt,omitempty"`
	// NoDiscountLine patterns price a single physical line at plain
	// bytes, verbatim. For single-line lexing hazards (Python f-strings,
	// whose nested quotes can desync a scanner within the line but — the
	// grammar guarantees — never beyond it).
	NoDiscountLine []string `json:"noDiscountLine,omitempty"`
	// KeepComments marks comments that carry semantics (`//go:` build
	// directives, shebangs): they are kept in the normalized form and
	// priced like code.
	KeepComments []string `json:"keepComments,omitempty"`

	fileRe, lineRe, keepRe, exemptRe []*regexp.Regexp
}

type StringSyntax struct {
	Open   string `json:"open"`
	Close  string `json:"close"`
	Escape string `json:"escape,omitempty"`
	// Multiline strings may span raw newlines (Go backticks, Python
	// triple quotes, Rust strings). Single-line strings are force-closed
	// at the newline, which re-syncs the scanner with the language at
	// every line end.
	Multiline bool `json:"multiline,omitempty"`
}

// compile validates the config and prepares its patterns. Called once at
// load time so a broken languages.json fails loudly, not per entry.
func (s *Syntax) compile() error {
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
	if err := comp(s.NoDiscountFile, &s.fileRe, "noDiscountFile"); err != nil {
		return err
	}
	if err := comp(s.NoDiscountLine, &s.lineRe, "noDiscountLine"); err != nil {
		return err
	}
	if err := comp(s.KeepComments, &s.keepRe, "keepComments"); err != nil {
		return err
	}
	if err := comp(s.NoDiscountFileExempt, &s.exemptRe, "noDiscountFileExempt"); err != nil {
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

type fileResult struct {
	units int
	lines int    // non-blank lines of the original, informational
	norm  []byte // normalized form: what ships, builds, and runs
	note  string // non-empty when priced at full bytes, says why
}

// measureFile prices one file and produces its normalized form.
func measureFile(b []byte, syn *Syntax) fileResult {
	if syn == nil {
		return fullPrice(b, "no syntax config for this language — plain byte pricing")
	}
	probe := b
	for _, re := range syn.exemptRe {
		probe = re.ReplaceAll(probe, nil)
	}
	for i, re := range syn.fileRe {
		if re.Match(probe) {
			return fullPrice(b, fmt.Sprintf("matches no-discount pattern %q — plain byte pricing", syn.NoDiscountFile[i]))
		}
	}
	r, ok := scan(b, syn)
	if !ok {
		return fullPrice(b, r.note)
	}
	return r
}

// fullPrice is the no-proof fallback: every non-whitespace byte costs 1
// and the file ships verbatim.
func fullPrice(b []byte, why string) fileResult {
	s, l := measureBytes(b)
	return fileResult{units: s, lines: l, norm: b, note: why}
}

// emitter builds the normalized form: whitespace runs collapse to one
// space, newline runs to one newline, indentation survives only for
// whitespace-sensitive languages, trailing whitespace never survives.
type emitter struct {
	out       []byte
	nlPending bool
	spPending bool
	lead      []byte // ws since the last newline: candidate indentation
	started   bool
	sensitive bool
}

func (e *emitter) ws(c byte) {
	if c == '\n' {
		e.nlPending = true
		e.lead = e.lead[:0]
		return
	}
	e.spPending = true
	if e.nlPending || !e.started {
		e.lead = append(e.lead, c)
	}
}

func (e *emitter) flush() {
	switch {
	case !e.started:
		if e.sensitive {
			e.out = append(e.out, e.lead...)
		}
	case e.nlPending:
		e.out = append(e.out, '\n')
		if e.sensitive {
			e.out = append(e.out, e.lead...)
		}
	case e.spPending:
		e.out = append(e.out, ' ')
	}
	e.nlPending, e.spPending = false, false
	e.lead = e.lead[:0]
	e.started = true
}

func (e *emitter) emit(bs []byte) {
	e.flush()
	e.out = append(e.out, bs...)
}

// scan walks the file with a three-mode lexer (code, string, comment)
// and prices as it goes. ok=false means a construct the scanner cannot
// price safely was found; the caller falls back to full byte pricing.
func scan(b []byte, syn *Syntax) (fileResult, bool) {
	var res fileResult
	e := emitter{sensitive: syn.WsSensitive}
	trig := triggeredLines(b, syn)
	line := 0
	pos := 0

	for pos < len(b) {
		c := b[pos]
		if c == '\n' {
			line++
		}
		if isWS(c) {
			e.ws(c)
			pos++
			continue
		}
		noDiscount := trig != nil && trig[line]

		// Comments — only where discounts apply; on a triggered line the
		// marker may sit inside a construct we cannot see (an f-string),
		// so nothing there is treated as a comment.
		if !noDiscount {
			if m := matchAny(b, pos, syn.LineComments); m != "" {
				end := lineEnd(b, pos)
				comment := b[pos:end]
				if matchRe(comment, syn.keepRe) {
					res.units += countNonWS(comment)
					e.emit(comment)
				}
				pos = end
				continue
			}
			if open, cl, found := matchBlock(b, pos, syn.BlockComments); found {
				end := bytes.Index(b[pos+len(open):], []byte(cl))
				if end < 0 {
					return fileResult{note: "unterminated block comment — plain byte pricing"}, false
				}
				comment := b[pos : pos+len(open)+end+len(cl)]
				// A backslash before a line break inside a block comment
				// can splice the closer apart in C — the compiler would
				// end the comment earlier than we do. No proof, no
				// discount.
				if bytes.Contains(comment, []byte("\\\n")) {
					return fileResult{note: "line splice inside a block comment — plain byte pricing"}, false
				}
				line += bytes.Count(comment, []byte("\n"))
				if matchRe(comment, syn.keepRe) {
					res.units += countNonWS(comment)
					e.emit(comment)
				} else if bytes.ContainsRune(comment, '\n') && syn.BlockCommentIsNewline {
					e.ws('\n')
				} else {
					e.ws(' ')
				}
				pos += len(comment)
				continue
			}
		}

		// Strings — tracked on every line so the mode machine stays in
		// sync; content is data and costs every byte, whitespace included.
		if str, found := matchString(b, pos, syn.Strings); found {
			end, nl, ok := stringEnd(b, pos, str)
			if !ok {
				return fileResult{note: "unterminated string literal — plain byte pricing"}, false
			}
			lit := b[pos:end]
			res.units += len(lit) // all of it, whitespace included: string content is data
			line += nl
			e.emit(lit)
			pos = end
			continue
		}

		// Identifiers and keywords: flat price, so `digest` costs what
		// `d` costs and renaming buys nothing. The flat rate ends at
		// identFlatLen bytes so names cannot smuggle bulk data.
		if !noDiscount && isIdentStart(c) && (pos == 0 || !isIdentByte(b[pos-1])) {
			end := pos
			for end < len(b) && isIdentByte(b[end]) {
				end++
			}
			res.units += 1 + max(0, (end-pos)-identFlatLen)
			e.emit(b[pos:end])
			pos = end
			continue
		}

		// Everything else — operators, punctuation, digits, and all of a
		// triggered line: 1 unit per byte.
		res.units++
		e.emit(b[pos : pos+1])
		pos++
	}

	if e.started {
		e.out = append(e.out, '\n')
	}
	res.norm = e.out
	_, res.lines = measureBytes(b)
	return res, true
}

// triggeredLines maps each physical line to whether a NoDiscountLine
// pattern matches it. nil when the language has no line patterns.
func triggeredLines(b []byte, syn *Syntax) []bool {
	if len(syn.lineRe) == 0 {
		return nil
	}
	lines := bytes.Split(b, []byte("\n"))
	trig := make([]bool, len(lines))
	for i, l := range lines {
		trig[i] = matchRe(l, syn.lineRe)
	}
	return trig
}

// stringEnd finds the end of a string literal opened at pos. Single-line
// strings are force-closed at a raw newline — that is what the grammars
// this scanner trusts do, and it re-syncs the mode machine at every line
// end. Returns the index one past the literal and the newline count.
func stringEnd(b []byte, pos int, str StringSyntax) (end, newlines int, ok bool) {
	i := pos + len(str.Open)
	for i < len(b) {
		if bytes.HasPrefix(b[i:], []byte(str.Close)) {
			return i + len(str.Close), newlines, true
		}
		if str.Escape != "" && bytes.HasPrefix(b[i:], []byte(str.Escape)) && i+len(str.Escape) < len(b) {
			skip := b[i+len(str.Escape)]
			if skip == '\n' {
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

func matchRe(b []byte, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.Match(b) {
			return true
		}
	}
	return false
}

func lineEnd(b []byte, pos int) int {
	if i := bytes.IndexByte(b[pos:], '\n'); i >= 0 {
		return pos + i
	}
	return len(b)
}

func countNonWS(b []byte) int {
	s, _ := measureBytes(b)
	return s
}

func isWS(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '\v', '\f':
		return true
	}
	return false
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentByte(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
