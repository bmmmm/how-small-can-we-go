// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

func cmdSurface(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("surface: exactly one directory expected")
	}
	// Pricing needs the language's syntax config; without a readable
	// manifest + languages.json the fallback is plain byte pricing.
	var syn *arena.Syntax
	if man, err := arena.LoadManifest(args[0]); err == nil {
		root, rootErr := repoRoot()
		if rootErr != nil {
			return rootErr
		}
		langs, err := arena.LoadLanguages(root)
		if err != nil {
			return err
		}
		lang, ok := langs[man.Language]
		if !ok {
			return fmt.Errorf("language %q is not in languages.json — propose it in a separate PR first", man.Language)
		}
		syn = lang.Syntax
	} else {
		fmt.Fprintf(os.Stderr, "surface: %s has no readable entry.json — plain byte pricing\n", args[0])
	}
	m, err := arena.MeasureDir(args[0], syn)
	if err != nil {
		return err
	}
	for _, n := range m.Notes {
		fmt.Fprintln(os.Stderr, "surface: "+n)
	}
	fmt.Println(m.Surface)
	return nil
}
