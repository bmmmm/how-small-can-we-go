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
	Surface int `json:"surface"` // non-whitespace bytes across all shipped files
	Lines   int `json:"lines"`   // non-blank lines, informational
	Files   int `json:"files"`
}

// MeasureDir measures every file under dir except the entry.json manifest.
// Non-text files (invalid UTF-8 or NUL bytes) and symlinks are rejected:
// an entry must ship auditable source only.
func MeasureDir(dir string) (Measure, error) {
	var m Measure
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink — symlinks are not allowed in entries", path)
		}
		if d.IsDir() {
			return nil
		}
		if rel, _ := filepath.Rel(dir, path); rel == "entry.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isText(b) {
			return fmt.Errorf("%s is not UTF-8 text — entries must ship auditable source, no binaries", path)
		}
		s, l := measureBytes(b)
		m.Surface += s
		m.Lines += l
		m.Files++
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

// measureBytes counts non-whitespace bytes (the surface) and non-blank
// lines. Byte-based on purpose: multi-byte runes cost their encoded size.
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
