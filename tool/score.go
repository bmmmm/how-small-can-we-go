// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

func cmdScore(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: score: exactly one entry directory expected", errUsage)
	}
	man, err := arena.LoadManifest(args[0])
	if err != nil {
		return err
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	langs, err := arena.LoadLanguages(root)
	if err != nil {
		return err
	}
	lang, ok := langs[man.Language]
	if !ok {
		return fmt.Errorf("language %q is not in languages.json — propose it in a separate PR first", man.Language)
	}
	sc, err := arena.ScoreDir(args[0], lang)
	if err != nil {
		return err
	}
	for _, n := range sc.Notes {
		fmt.Fprintln(os.Stderr, "score: "+n)
	}
	for _, h := range sc.Hazards {
		fmt.Fprintf(os.Stderr, "score: hazard %s:%d %s — %s\n", h.File, h.Line, h.Pattern, h.Why)
	}
	// Machine-readable verdict: "<third-party bytes> <hazards>", the
	// two score dimensions in comparison order.
	fmt.Printf("%d %d\n", sc.VendoredBytes, sc.HazardCount)
	return nil
}
