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

type boardRow struct {
	Result
	Champion bool // smallest passing surface of its task
}

type boardTask struct {
	Name string
	Rows []boardRow
	Open []string // languages from languages.json with no entry yet
}

type boardData struct {
	Commit string
	Repo   string
	Tasks  []boardTask
}

// WriteBoard renders board.json and index.html for the given results.
// langs is the sorted list of playable languages; niches without an
// entry are shown as open so visitors see the cheapest way in.
func WriteBoard(results []Result, outDir, commit string, langs []string) error {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = "bmmmm/how-small-can-we-go"
	}
	byTask := map[string][]Result{}
	for _, r := range results {
		task := r.Task
		if task == "" {
			task = "(broken manifest)"
		}
		byTask[task] = append(byTask[task], r)
	}
	data := boardData{Commit: commit, Repo: repo}
	for _, name := range sortedKeys(byTask) {
		rs := byTask[name]
		sort.Slice(rs, func(i, j int) bool {
			if rs[i].Pass != rs[j].Pass {
				return rs[i].Pass
			}
			if rs[i].Measure.Surface != rs[j].Measure.Surface {
				return rs[i].Measure.Surface < rs[j].Measure.Surface
			}
			return rs[i].Entry < rs[j].Entry
		})
		t := boardTask{Name: name}
		taken := map[string]bool{}
		for i, r := range rs {
			t.Rows = append(t.Rows, boardRow{Result: r, Champion: i == 0 && r.Pass})
			taken[r.Language] = true
		}
		for _, l := range langs {
			if !taken[l] {
				t.Open = append(t.Open, l)
			}
		}
		data.Tasks = append(data.Tasks, t)
	}

	// Open niches go into board.json too: agents read the JSON as
	// ground truth, and an open niche is the cheapest way in.
	open := []map[string]string{}
	for _, t := range data.Tasks {
		for _, l := range t.Open {
			open = append(open, map[string]string{"task": t.Name, "language": l})
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(map[string]any{
		"commit":  commit,
		"entries": results,
		"open":    open,
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

func sortedKeys(m map[string][]Result) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
