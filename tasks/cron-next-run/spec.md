# cron-next-run

Given a five-field cron expression and an instant, compute the next time
the job fires. Every scheduler needs this and almost none of them writes
it: the world imports croniter, cron-parser, robfig/cron, Quartz. The
famous surprise lives in the day fields — when day-of-month *and*
day-of-week are both restricted they are OR-ed, not AND-ed, so
`0 0 13 * 5` means "the 13th **or** any Friday". Here it is a topic:
implement it from what you ship.

## Interface

```
prog <expr> <instant>
```

- Exactly two arguments. Any other argv shape is a usage error: exit
  nonzero.
- `<expr>` is the whole cron expression in a single argument; its five
  fields are separated inside it by whitespace.
- `<instant>` is an RFC 3339 UTC timestamp: exactly
  `YYYY-MM-DDTHH:MM:SSZ`, 20 characters, uppercase `T` and `Z`, every
  component zero-padded to its full width. Anything else is invalid —
  a numeric offset (`+00:00`, `-05:00`), a fractional second
  (`...:00.5Z`), a lowercase `t` or `z`, a space instead of `T`, a
  missing zone. A date that does not exist (`2026-02-30T…`) is invalid
  too.
- On success, print the next fire time in that same
  `YYYY-MM-DDTHH:MM:SSZ` form followed by `\n`, and exit 0.
- On an invalid expression, an invalid instant, or an expression with no
  fire time inside the search bound, print nothing to stdout and exit
  nonzero. stderr is free for diagnostics.

Whitespace in this spec means ASCII space (0x20) and tab (0x09), and
nothing else. Fields are separated by one or more of those two
characters; leading and trailing whitespace around the whole expression
is ignored. After that split the expression must have exactly five
fields — four or six is invalid.

## Fields

In order, with their value ranges:

| # | field        | range              |
| - | ------------ | ------------------ |
| 1 | minute       | 0–59               |
| 2 | hour         | 0–23               |
| 3 | day-of-month | 1–31               |
| 4 | month        | 1–12               |
| 5 | day-of-week  | 0–6, 0 = Sunday    |

Pinned out of scope, all invalid: `7` for Sunday (only 0–6 exist here),
three-letter names (`JAN`, `MON`), `@macros` (`@daily`, `@reboot` — they
are not five fields and there is no expansion step), `?`, `L`, `W`, `#`,
and any second or year field.

## Field grammar

```
field   := item ( "," item )*
item    := "*" | "*" "/" step | number | number "-" number
         | number "-" number "/" step
number  := digit+
step    := digit+
```

- `number` is base-10 ASCII digits only. Leading zeros are allowed and
  carry no meaning: `05` is `5`, `007` is `7`. Real crontabs contain
  them. No sign, no whitespace inside a field.
- Every `number` must lie inside the field's range; `60` as a minute or
  `0` as a day-of-month is invalid.
- In a range `a-b`, `a <= b` is required. Wrap-around ranges (`50-10`,
  `22-2`) are invalid — they are not part of this dialect.
- `step` must be `>= 1`; `0` is invalid. `*/n` steps over the field's
  full range, `a-b/n` over `a..b`: the values are `a`, `a+n`, `a+2n`, …
  up to and including `b`. A step larger than the span is legal and
  simply yields the range start alone — `10-12/9` is the single hour
  `10`.
- A step on a single value (`5/10`) is invalid: there is no range to
  step over.
- A list may repeat values; the field is a set, so duplicates are
  harmless (`1,1,2` is `{1,2}`).
- Nothing else. Any other character anywhere in a field — a space
  survived the split, a `-` with an empty side, an empty list item
  (`1,,2`), a trailing comma, a second `/` — makes the expression
  invalid.

## Matching semantics

A minute is a fire time when all of the following hold. Fire times
always have second 0; cron has no sub-minute resolution.

1. The minute field matches the minute.
2. The hour field matches the hour.
3. The month field matches the month.
4. The day matches, by the classic Vixie rule:
   - A day field counts as **restricted** unless its text is exactly
     `*`. This is the corner every dialect decides differently, so it is
     decided here: `*/2` is **restricted**, and so is `*,5`. Only the
     bare single character `*` is unrestricted. (Vixie cron itself keys
     off the field's first character and would call `*/2`
     unrestricted; this spec deliberately takes the other reading, and
     the `star-step-restricted` case pins it.)
   - Both day fields restricted → the day matches when **either** the
     day-of-month **or** the day-of-week matches. This is the OR rule.
   - Exactly one restricted → that one must match, the other is ignored.
   - Neither restricted → every day matches.

"Next" means **strictly after** the instant: if the instant is itself a
fire time, the answer is the following one. Seconds in the instant are
ignored beyond that — the search starts at the first minute boundary
after the instant, so `12:00:30` and `12:00:00` both look at `12:01`
first, but `12:00:00` has already had its own minute rejected for not
being strictly later.

Everything is UTC. There is no local zone, no DST, no leap second — a
day is 1440 minutes, always.

## Search bound

The search examines the 2 108 160 minute boundaries following the
instant — 4 × 366 days, i.e. 1 464 days ahead, comfortably more than any
four consecutive calendar years. If none of them is a fire time, the
expression is unsatisfiable in practice: print nothing to stdout, write
a diagnostic to stderr, exit nonzero. `0 0 30 2 *` (February 30th) is
the canonical example. `0 0 29 2 *` is *not* — a leap day always falls
inside the bound for the years this task cares about.

## Expected outputs

Derived by hand from the semantics above: the fields were expanded to
their value sets, the calendar walked forward from the instant day by
day, and the first matching minute written down — including the weekday
arithmetic for the OR-rule cases (2026-08-08 is a Saturday, which fixes
every weekday used in the fixtures). The reference entry is the
executable cross-check, not the source of the numbers.
