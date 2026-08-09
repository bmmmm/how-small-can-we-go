// SPDX-License-Identifier: GPL-3.0-or-later

// Package arena implements measurement and conformance checking for
// how-small-can-we-go entries.
package arena

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Manifest is an entry's entry.json.
type Manifest struct {
	Task     string   `json:"task"`
	Language string   `json:"language"`
	Authors  []string `json:"authors"`
	Build    string   `json:"build,omitempty"`
	Run      string   `json:"run"`
}

// Language is one allowed language from languages.json.
type Language struct {
	Image string `json:"image"`
}

func LoadManifest(entryDir string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(entryDir, "entry.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("every entry needs an entry.json (see SPEC.md): %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s/entry.json is not valid JSON: %w", entryDir, err)
	}
	switch {
	case m.Task == "":
		return Manifest{}, fmt.Errorf("%s/entry.json: \"task\" is required", entryDir)
	case m.Language == "":
		return Manifest{}, fmt.Errorf("%s/entry.json: \"language\" is required", entryDir)
	case m.Run == "":
		return Manifest{}, fmt.Errorf("%s/entry.json: \"run\" is required", entryDir)
	case len(m.Authors) == 0:
		return Manifest{}, fmt.Errorf("%s/entry.json: \"authors\" is required", entryDir)
	}
	return m, nil
}

func LoadLanguages(repoRoot string) (map[string]Language, error) {
	path := filepath.Join(repoRoot, "languages.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var langs map[string]Language
	if err := json.Unmarshal(b, &langs); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	return langs, nil
}

// DiscoverEntries returns every directory under entries/ that holds an
// entry.json, sorted, as repo-relative slash paths.
func DiscoverEntries(repoRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(repoRoot, "entries", "*", "*", "entry.json"))
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(repoRoot, filepath.Dir(m))
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, filepath.ToSlash(rel))
	}
	sort.Strings(dirs)
	return dirs, nil
}
