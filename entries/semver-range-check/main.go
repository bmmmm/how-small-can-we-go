// SPDX-License-Identifier: GPL-3.0-or-later

// semver-range-check: does <version> satisfy <range>? Implements the
// grammar of tasks/semver-range-check/spec.md — full X.Y.Z versions,
// comparators (= < <= > >= ^ ~), space = AND, || = OR.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type version struct{ major, minor, patch uint64 }

// compare returns -1, 0, or 1: major first, then minor, then patch.
// Direct comparisons, not subtraction — subtraction wraps.
func compare(a, b version) int {
	for _, p := range [3][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if p[0] < p[1] {
			return -1
		}
		if p[0] > p[1] {
			return 1
		}
	}
	return 0
}

func parseVersion(s string) (version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("version %q: want MAJOR.MINOR.PATCH", s)
	}
	var n [3]uint64
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return version{}, fmt.Errorf("version %q: component %q is empty or has a leading zero", s, p)
		}
		// ParseUint accepts digits only (no sign) and rejects anything
		// beyond 63 bits — the spec's component bound.
		v, err := strconv.ParseUint(p, 10, 63)
		if err != nil {
			return version{}, fmt.Errorf("version %q: component %q is not a base-10 integer within 63 bits", s, p)
		}
		n[i] = v
	}
	return version{n[0], n[1], n[2]}, nil
}

type comparator struct {
	op string
	v  version
}

func (c comparator) matches(v version) bool {
	switch c.op {
	case "<":
		return compare(v, c.v) < 0
	case "<=":
		return compare(v, c.v) <= 0
	case ">":
		return compare(v, c.v) > 0
	case ">=":
		return compare(v, c.v) >= 0
	case "~":
		return compare(v, c.v) >= 0 && compare(v, version{c.v.major, c.v.minor + 1, 0}) < 0
	case "^":
		upper := version{c.v.major + 1, 0, 0}
		if c.v.major == 0 {
			upper = version{0, c.v.minor + 1, 0}
			if c.v.minor == 0 {
				upper = version{0, 0, c.v.patch + 1}
			}
		}
		return compare(v, c.v) >= 0 && compare(v, upper) < 0
	default: // "" and "=": exact
		return compare(v, c.v) == 0
	}
}

// parseRange returns the range as OR-of-AND clauses.
func parseRange(s string) ([][]comparator, error) {
	var clauses [][]comparator
	for _, part := range strings.Split(s, "||") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			return nil, fmt.Errorf("range %q: empty clause", s)
		}
		var clause []comparator
		for _, f := range fields {
			op := ""
			for _, p := range []string{"<=", ">=", "<", ">", "=", "^", "~"} {
				if strings.HasPrefix(f, p) {
					op = p
					break
				}
			}
			v, err := parseVersion(f[len(op):])
			if err != nil {
				return nil, err
			}
			clause = append(clause, comparator{op, v})
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: prog <range> <version>")
		os.Exit(2)
	}
	clauses, err := parseRange(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	v, err := parseVersion(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, clause := range clauses {
		all := true
		for _, c := range clause {
			if !c.matches(v) {
				all = false
				break
			}
		}
		if all {
			fmt.Println("yes")
			return
		}
	}
	fmt.Println("no")
}
