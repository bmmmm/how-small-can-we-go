// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	noSandbox := fs.Bool("no-sandbox", false, "run build and cases on the host instead of a no-network container")
	asJSON := fs.Bool("json", false, "print results as JSON")
	_ = fs.Parse(flagsFirst(args))
	if fs.NArg() == 0 {
		return fmt.Errorf("check: no entry directory given (e.g. arena check entries/sha256-file/go)")
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	opts := arena.Options{RepoRoot: root, Sandbox: !*noSandbox}
	var results []arena.Result
	failed := 0
	for _, dir := range fs.Args() {
		res := arena.CheckEntry(dir, opts)
		results = append(results, res)
		if !res.Pass {
			failed++
		}
		if !*asJSON {
			printResult(res)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d entries failed", failed, len(results))
	}
	return nil
}

func printResult(r arena.Result) {
	fmt.Printf("== %s (%s / %s)\n", r.Entry, r.Task, r.Language)
	if r.Err != "" {
		fmt.Printf("   FAIL  %s\n", r.Err)
		return
	}
	fmt.Printf("   surface: %d bytes   lines: %d   files: %d\n", r.Measure.Surface, r.Measure.Lines, r.Measure.Files)
	for _, c := range r.Cases {
		if c.Pass {
			fmt.Printf("   PASS  %s\n", c.Name)
		} else {
			fmt.Printf("   FAIL  %s\n         %s\n", c.Name, c.Detail)
		}
	}
	if r.Pass {
		fmt.Println("   RESULT: PASS")
	} else {
		fmt.Println("   RESULT: FAIL")
	}
}
