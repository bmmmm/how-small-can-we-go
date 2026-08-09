// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// walkEntryFiles visits every shipped file of an entry (skipping the
// entry.json manifest), rejecting symlinks and non-text content: an
// entry must be fully reviewable, so nothing unauditable ships.
func walkEntryFiles(dir string, fn func(path, rel string, b []byte) error) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink — symlinks are not allowed in entries", path)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "entry.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isText(b) {
			return fmt.Errorf("%s is not UTF-8 text — entries must ship auditable source, no binaries", path)
		}
		return fn(path, rel, b)
	})
}

func isText(b []byte) bool {
	return utf8.Valid(b) && !bytes.Contains(b, []byte{0})
}
