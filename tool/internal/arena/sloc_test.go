// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasureBytes(t *testing.T) {
	tests := []struct {
		in      string
		surface int
		lines   int
	}{
		{"", 0, 0},
		{"abc\n", 3, 1},
		{"a b\tc\n\n  \n d\n", 4, 2},
		{"no trailing newline", 17, 1},
		{"ü\n", 2, 1}, // multi-byte rune costs its encoded size
	}
	for _, tt := range tests {
		s, l := measureBytes([]byte(tt.in))
		if s != tt.surface || l != tt.lines {
			t.Errorf("measureBytes(%q) = (%d, %d), want (%d, %d)", tt.in, s, l, tt.surface, tt.lines)
		}
	}
}

func TestIsText(t *testing.T) {
	if isText([]byte{0x68, 0x00, 0x69}) {
		t.Error("NUL byte accepted as text")
	}
	if isText([]byte{0xff, 0xfe}) {
		t.Error("invalid UTF-8 accepted as text")
	}
	if !isText([]byte("plain\n")) {
		t.Error("plain ASCII rejected")
	}
}

func TestMeasureDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "entry.json"), `{"ignored": true}`)
	writeFile(t, filepath.Join(dir, "main.sh"), "cat \"$1\"\n")
	m, err := MeasureDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Files != 1 || m.Surface != 7 || m.Lines != 1 {
		t.Errorf("got %+v, want 1 file, surface 7, 1 line", m)
	}
}

func TestMeasureDirRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blob"), []byte{0x7f, 0x45, 0x4c, 0x46, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MeasureDir(dir); err == nil || !strings.Contains(err.Error(), "not UTF-8") {
		t.Errorf("binary file not rejected, err = %v", err)
	}
}

func TestMeasureDirRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "x\n")
	if err := os.Symlink("/etc/hosts", filepath.Join(dir, "sneaky")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := MeasureDir(dir); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("symlink not rejected, err = %v", err)
	}
}
