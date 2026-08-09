// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

func cmdBoard(args []string) error {
	fs := flag.NewFlagSet("board", flag.ExitOnError)
	noSandbox := fs.Bool("no-sandbox", false, "run build and cases on the host instead of a no-network container")
	out := fs.String("out", "docs", "output directory for board.json and index.html")
	_ = fs.Parse(args)
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dirs, err := arena.DiscoverEntries(root)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return fmt.Errorf("no entries found under entries/")
	}
	opts := arena.Options{RepoRoot: root, Sandbox: !*noSandbox}
	var results []arena.Result
	for _, d := range dirs {
		fmt.Fprintf(os.Stderr, "checking %s\n", d)
		res := arena.CheckEntry(d, opts)
		if res.Infra {
			// A docker hiccup must not publish a board that shows a
			// healthy entry as failing — abort instead.
			return fmt.Errorf("%s: %w — no board written, retry", d, arena.ErrInfra)
		}
		results = append(results, res)
	}
	langMap, err := arena.LoadLanguages(root)
	if err != nil {
		return err
	}
	langs := make([]string, 0, len(langMap))
	for l := range langMap {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	commit := os.Getenv("GITHUB_SHA")
	if commit == "" {
		commit = "local"
	}
	if err := arena.WriteBoard(results, *out, commit, langs); err != nil {
		return err
	}
	fmt.Printf("board written to %s (%d entries)\n", *out, len(results))
	return nil
}
