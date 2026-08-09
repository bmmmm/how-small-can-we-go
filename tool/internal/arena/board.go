// SPDX-License-Identifier: GPL-3.0-or-later

package arena

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed board.html.tmpl
var boardTmpl string

type boardData struct {
	Commit string
	Repo   string
	Rows   []Result
	Open   []string // tasks with no entry yet — the cheapest way in
}

// WriteBoard renders board.json and index.html for the given results.
// tasks is the full task list; a task without an entry is an open niche.
func WriteBoard(results []Result, outDir, commit string, tasks []string) error {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = "bmmmm/how-small-can-we-go"
	}
	rows := append([]Result{}, results...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Task < rows[j].Task })
	taken := map[string]bool{}
	for _, r := range rows {
		taken[r.Task] = true
	}
	data := boardData{Commit: commit, Repo: repo, Rows: rows}
	for _, t := range tasks {
		if !taken[t] {
			data.Open = append(data.Open, t)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(map[string]any{
		"commit":  commit,
		"entries": rows,
		"open":    data.Open,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "board.json"), append(blob, '\n'), 0o644); err != nil {
		return err
	}
	tmpl, err := template.New("board").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(boardTmpl)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("render board: %w", err)
	}
	return f.Close()
}
