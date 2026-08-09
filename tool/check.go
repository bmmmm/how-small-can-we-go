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
		return fmt.Errorf("%w: check: no entry directory given (e.g. arena check entries/semver-range-check)", errUsage)
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	opts := arena.Options{RepoRoot: root, Sandbox: !*noSandbox}
	var results []arena.Result
	failed, infra := 0, false
	for _, dir := range fs.Args() {
		res := arena.CheckEntry(dir, opts)
		results = append(results, res)
		if !res.Pass {
			failed++
		}
		if res.Infra {
			infra = true
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
	if infra {
		// Wrapped so main can exit 3: this run holds no verdict and
		// nothing downstream may treat it as a conformance failure.
		return fmt.Errorf("%d of %d entries failed on an %w — retry, or check docker", failed, len(results), arena.ErrInfra)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d entries failed", failed, len(results))
	}
	return nil
}

func printResult(r arena.Result) {
	fmt.Printf("== %s (%s / %s)\n", r.Entry, r.Task, r.Language)
	if r.Err != "" {
		fmt.Printf("   %s  %s\n", failLabel(r.Infra), r.Err)
		return
	}
	fmt.Printf("   trust score: %d third-party bytes, %d hazards   (%d files)\n", r.Score.VendoredBytes, r.Score.HazardCount, r.Score.Files)
	for _, n := range r.Score.Notes {
		fmt.Printf("   NOTE  %s\n", n)
	}
	for _, h := range r.Score.Hazards {
		fmt.Printf("   HAZARD  %s:%d %s — %s\n", h.File, h.Line, h.Pattern, h.Why)
	}
	for _, c := range r.Cases {
		if c.Pass {
			fmt.Printf("   PASS  %s\n", c.Name)
		} else {
			fmt.Printf("   %s  %s\n         %s\n", failLabel(c.Infra), c.Name, c.Detail)
		}
	}
	switch {
	case r.Pass:
		fmt.Println("   RESULT: PASS")
	case r.Infra:
		fmt.Println("   RESULT: INFRA — sandbox/runner failure, no verdict on the entry")
	default:
		fmt.Println("   RESULT: FAIL")
	}
}

func failLabel(infra bool) string {
	if infra {
		return "INFRA"
	}
	return "FAIL"
}
