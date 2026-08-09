// SPDX-License-Identifier: GPL-3.0-or-later

// arena is the measurement and conformance harness for how-small-can-we-go:
// it checks entries against their task's test cases and measures their
// audit surface in audit units (comments free, identifiers flat, data
// per byte — see SPEC.md).
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

var version = "dev"

const usage = `arena — measurement and conformance harness

Usage:
  arena check [--no-sandbox] [--json] <entry-dir>...
  arena surface <dir>
  arena board [--no-sandbox] [--out <dir>]
  arena version

check    Build an entry, run its task's test cases, print the verdict.
         By default build and cases run in a no-network container (needs
         docker); --no-sandbox executes on the host for local iteration.
surface  Print the audit surface (in audit units) of an entry directory.
         Files priced at plain bytes are noted on stderr.
board    Re-check every entry and write board.json + index.html.

Exit codes: 0 pass · 1 fail · 2 usage · 3 infrastructure failure
(sandbox/runner broke — no verdict, retry instead of judging the entry).

Run from the repository root.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "check":
		err = cmdCheck(os.Args[2:])
	case "surface":
		err = cmdSurface(os.Args[2:])
	case "board":
		err = cmdBoard(os.Args[2:])
	case "version":
		fmt.Println(versionString())
	default:
		fmt.Fprintf(os.Stderr, "arena: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "arena: %v\n", err)
		if errors.Is(err, arena.ErrInfra) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}

func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// flagsFirst reorders args so flags precede positionals — Go's flag
// package stops at the first non-flag, but "arena check <dir> --json"
// is how people (and agents) naturally type it. All our flags are
// booleans, so reordering is safe.
func flagsFirst(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

func repoRoot() (string, error) {
	if _, err := os.Stat("languages.json"); err != nil {
		return "", fmt.Errorf("languages.json not found — run arena from the repository root")
	}
	return ".", nil
}
