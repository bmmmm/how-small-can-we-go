// SPDX-License-Identifier: GPL-3.0-or-later

// cron-next-run: print the next fire time strictly after <instant> for a
// five-field cron expression. Implements tasks/cron-next-run/spec.md —
// minute hour day-of-month month day-of-week, everything UTC. The one
// rule worth naming twice is the Vixie day rule: when both day fields
// are restricted the day matches if EITHER of them does. A field counts
// as restricted unless its text is exactly "*", so a stepped star like
// */2 is restricted here; the spec pins that corner deliberately, and
// classic Vixie cron would read it the other way.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// instantLayout is the only accepted instant shape. In a Go layout a
// trailing "Z" not followed by an offset pattern is a literal byte, so
// numeric offsets and a lowercase z already fail to parse. Fractions do
// not: Parse eats a fractional second after the seconds even when the
// layout has none — which is why instantWidth below does the rejecting.
const instantLayout = "2006-01-02T15:04:05Z"

// instantWidth is the byte length of a conforming instant. Every
// component is fixed width, so any other length is malformed.
const instantWidth = len(instantLayout)

// searchMinutes bounds the scan at 4 * 366 days of minute boundaries —
// more than any four consecutive calendar years, so a schedule that
// misses it (February 30th) is unsatisfiable in practice.
const searchMinutes = 4 * 366 * 24 * 60

// field is one parsed cron field: a bitmask of the values it matches,
// plus whether it constrains anything at all. Only the exact text "*"
// is unrestricted — the day rule needs that distinction.
type field struct {
	bits       uint64
	restricted bool
}

func (f field) has(v int) bool { return f.bits&(uint64(1)<<uint(v)) != 0 }

// parseDigits accepts ASCII digits only: leading zeros are allowed and
// meaningless ("05" is 5), signs and whitespace are not. strconv.Atoi
// alone would take "+5" and " 5"; it does reject overflow, which is how
// an absurdly long number dies here.
func parseDigits(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("%q is not a base-10 number", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q does not fit in an int", s)
	}
	return n, nil
}

// parseField expands one field's text into a bitmask over lo..hi.
func parseField(text string, lo, hi int) (field, error) {
	f := field{restricted: text != "*"}
	for _, item := range strings.Split(text, ",") {
		base, stepText, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			n, err := parseDigits(stepText)
			if err != nil || n < 1 {
				return field{}, fmt.Errorf("field %q: step in %q must be a number >= 1", text, item)
			}
			step = n
		}
		var from, to int
		switch {
		case base == "*":
			from, to = lo, hi
		case strings.Contains(base, "-"):
			a, b, _ := strings.Cut(base, "-")
			var err error
			if from, err = parseDigits(a); err != nil {
				return field{}, fmt.Errorf("field %q: %v", text, err)
			}
			if to, err = parseDigits(b); err != nil {
				return field{}, fmt.Errorf("field %q: %v", text, err)
			}
			if from > to {
				return field{}, fmt.Errorf("field %q: range %q runs backwards; wrap-around is not part of this dialect", text, base)
			}
		case hasStep:
			return field{}, fmt.Errorf("field %q: %q steps over a single value, which has nothing to step over", text, item)
		default:
			n, err := parseDigits(base)
			if err != nil {
				return field{}, fmt.Errorf("field %q: %v", text, err)
			}
			from, to = n, n
		}
		if from < lo || to > hi {
			return field{}, fmt.Errorf("field %q: %q leaves the range %d-%d", text, item, lo, hi)
		}
		// A step wider than the whole field can never reach a second
		// value, so clamping it keeps the loop arithmetic small without
		// changing the result — an unclamped huge step would overflow.
		if step > hi-lo+1 {
			step = hi - lo + 1
		}
		for v := from; v <= to; v += step {
			f.bits |= uint64(1) << uint(v)
		}
	}
	return f, nil
}

// splitFields splits on runs of ASCII space and tab — the only
// whitespace this dialect grants. strings.Fields would also split on
// \v, \f, \r and Unicode spaces, silently widening the grammar.
func splitFields(s string) []string {
	var fields []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

type schedule struct {
	minute, hour, dom, month, dow field
}

func parseExpr(expr string) (schedule, error) {
	parts := splitFields(expr)
	if len(parts) != 5 {
		return schedule{}, fmt.Errorf("expression %q has %d fields, want 5: minute hour day-of-month month day-of-week", expr, len(parts))
	}
	var s schedule
	specs := []struct {
		dst    *field
		lo, hi int
	}{
		{&s.minute, 0, 59},
		{&s.hour, 0, 23},
		{&s.dom, 1, 31},
		{&s.month, 1, 12},
		{&s.dow, 0, 6},
	}
	for i, sp := range specs {
		f, err := parseField(parts[i], sp.lo, sp.hi)
		if err != nil {
			return schedule{}, err
		}
		*sp.dst = f
	}
	return s, nil
}

// matches decides whether t is a fire time. Minute, hour and month are
// plain conjunctions; the day is the Vixie rule — both day fields
// restricted means OR, one restricted means only that one counts,
// neither means every day qualifies.
func (s schedule) matches(t time.Time) bool {
	if !s.minute.has(t.Minute()) || !s.hour.has(t.Hour()) || !s.month.has(int(t.Month())) {
		return false
	}
	switch {
	case s.dom.restricted && s.dow.restricted:
		return s.dom.has(t.Day()) || s.dow.has(int(t.Weekday()))
	case s.dom.restricted:
		return s.dom.has(t.Day())
	case s.dow.restricted:
		return s.dow.has(int(t.Weekday()))
	}
	return true
}

func parseInstant(s string) (time.Time, error) {
	if len(s) != instantWidth {
		return time.Time{}, fmt.Errorf("instant %q: want exactly %s — %d characters, UTC, no fractional seconds", s, instantLayout, instantWidth)
	}
	t, err := time.Parse(instantLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("instant %q: %v", s, err)
	}
	return t, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: prog <expr> <instant>")
		os.Exit(2)
	}
	sched, err := parseExpr(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	instant, err := parseInstant(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Fire times sit on minute boundaries with second 0, and "next"
	// means strictly after: drop the seconds, then step one minute on,
	// so an instant that is itself a fire time is passed over.
	t := time.Date(instant.Year(), instant.Month(), instant.Day(), instant.Hour(), instant.Minute(), 0, 0, time.UTC).Add(time.Minute)
	for i := 0; i < searchMinutes; i++ {
		if sched.matches(t) {
			fmt.Println(t.Format(instantLayout))
			return
		}
		t = t.Add(time.Minute)
	}
	fmt.Fprintf(os.Stderr, "no fire time for %q within %d minutes after %s\n", os.Args[1], searchMinutes, os.Args[2])
	os.Exit(1)
}
