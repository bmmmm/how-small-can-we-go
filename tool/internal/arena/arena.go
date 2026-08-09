// SPDX-License-Identifier: GPL-3.0-or-later

// Package arena implements trust-score measurement and conformance
// checking for how-small-can-we-go entries.
package arena

import (
	"bytes"
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

// Language is one allowed language from languages.json. Strip enables
// comment stripping before the hazard scan; a language without one is
// scanned raw — comments included, fail-suspicious.
type Language struct {
	Image      string   `json:"image"`
	Extensions []string `json:"extensions"`
	Strip      *Strip   `json:"strip,omitempty"`
	Hazards    []Hazard `json:"hazards,omitempty"`
}

// maxManifestBytes bounds entry.json. The manifest is metadata, so it
// must stay a manifest: five known fields, no bulk.
const maxManifestBytes = 4096

func LoadManifest(entryDir string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(entryDir, "entry.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("every entry needs an entry.json (see SPEC.md): %w", err)
	}
	if len(b) > maxManifestBytes {
		return Manifest{}, fmt.Errorf("%s/entry.json is %d bytes — a manifest holds five short fields, max %d bytes", entryDir, len(b), maxManifestBytes)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // unknown keys would be unreviewed freight
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%s/entry.json is not a valid manifest (fields: task, language, authors, build, run): %w", entryDir, err)
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
	for name, lang := range langs {
		if len(lang.Extensions) == 0 {
			return nil, fmt.Errorf("%s: language %q needs extensions — without them the hazard scan would apply to no file", path, name)
		}
		if err := lang.compile(); err != nil {
			return nil, fmt.Errorf("%s: language %q: %w", path, name, err)
		}
		langs[name] = lang
	}
	return langs, nil
}

// DiscoverEntries returns every directory under entries/ that holds an
// entry.json, sorted, as repo-relative slash paths. One directory per
// task: the niche is the task, the language is the entry's choice.
func DiscoverEntries(repoRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(repoRoot, "entries", "*", "entry.json"))
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

// DiscoverTasks returns every task directory that has test cases,
// sorted by name. Tasks without an entry are the board's open niches.
func DiscoverTasks(repoRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(repoRoot, "tasks", "*", "cases"))
	if err != nil {
		return nil, err
	}
	tasks := make([]string, 0, len(matches))
	for _, m := range matches {
		tasks = append(tasks, filepath.Base(filepath.Dir(m)))
	}
	sort.Strings(tasks)
	return tasks, nil
}
