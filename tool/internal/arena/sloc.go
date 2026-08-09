// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// Measure is the audit-surface measurement of an entry directory.
type Measure struct {
	Surface   int      `json:"surface"` // audit units across all shipped files
	Bytes     int      `json:"bytes"`   // non-whitespace bytes, informational
	Lines     int      `json:"lines"`   // non-blank lines, informational
	Files     int      `json:"files"`
	RawBytes  int      `json:"raw"`             // total bytes as committed
	NormBytes int      `json:"normalized"`      // total bytes of the normalized form — what ships and runs
	Notes     []string `json:"notes,omitempty"` // files priced at full bytes, and why
}

// MeasureDir measures every file under dir except the entry.json
// manifest, using the language's syntax for audit-unit pricing (nil
// syntax = plain byte pricing). File paths are priced at 1 unit per
// byte — an auditor reads names too, and unmeasured names would be a
// free data channel. Non-text files (invalid UTF-8 or NUL bytes) and
// symlinks are rejected: an entry must ship auditable source only.
func MeasureDir(dir string, syn *Syntax) (Measure, error) {
	var m Measure
	err := walkEntryFiles(dir, func(path, rel string, b []byte) error {
		r := measureFile(b, syn.forFile(rel))
		if syn != nil && syn.forFile(rel) == nil {
			r.note = "not a source extension of this language — data is priced at plain bytes and ships verbatim"
		}
		m.Surface += r.units + len(rel)
		m.Bytes += countNonWS(b)
		m.Lines += r.lines
		m.RawBytes += len(b) + len(rel)
		m.NormBytes += len(r.norm) + len(rel)
		m.Files++
		if r.note != "" {
			m.Notes = append(m.Notes, rel+": "+r.note)
		}
		return nil
	})
	if err != nil {
		return Measure{}, err
	}
	if m.Files == 0 {
		return Measure{}, fmt.Errorf("%s contains no measurable files", dir)
	}
	return m, nil
}

// NormalizeTree writes the normalized form of the entry at src into dst:
// source files with comments stripped and whitespace collapsed, data
// files verbatim. The normalized form is what builds and runs — you play
// what you weigh, so bytes that were measured as free provably never
// execute. entry.json is not shipped at all: it is arena metadata, and
// as an unmeasured file in the working directory it would be a covert
// data store readable at runtime.
func NormalizeTree(src, dst string, syn *Syntax) error {
	return walkAllFiles(src, func(path, rel string, b []byte, perm fs.FileMode) error {
		if rel == "entry.json" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, measureFile(b, syn.forFile(rel)).norm, perm)
	})
}

// walkEntryFiles visits every measurable file (skipping entry.json),
// rejecting symlinks and non-text content.
func walkEntryFiles(dir string, fn func(path, rel string, b []byte) error) error {
	return walkAllFiles(dir, func(path, rel string, b []byte, _ fs.FileMode) error {
		if rel == "entry.json" {
			return nil
		}
		return fn(path, rel, b)
	})
}

func walkAllFiles(dir string, fn func(path, rel string, b []byte, perm fs.FileMode) error) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink — symlinks are not allowed in entries", path)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isText(b) {
			return fmt.Errorf("%s is not UTF-8 text — entries must ship auditable source, no binaries", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fn(path, filepath.ToSlash(rel), b, info.Mode().Perm())
	})
}

// measureBytes counts non-whitespace bytes and non-blank lines.
// Byte-based on purpose: multi-byte runes cost their encoded size.
func measureBytes(b []byte) (surface, lines int) {
	lineHasContent := false
	for _, c := range b {
		switch c {
		case ' ', '\t', '\r', '\v', '\f':
		case '\n':
			if lineHasContent {
				lines++
				lineHasContent = false
			}
		default:
			surface++
			lineHasContent = true
		}
	}
	if lineHasContent {
		lines++
	}
	return surface, lines
}

func isText(b []byte) bool {
	return utf8.Valid(b) && !bytes.Contains(b, []byte{0})
}
