# toml-subset-parse

Parse a pinned TOML subset and print it flat. Config parsing is a job
almost every project delegates to a library — every ecosystem has one
(tomllib, toml-rs, BurntSushi/toml, …), because full TOML is a large
grammar: floats, dates, arrays, inline tables, three string flavors,
underscored numbers. Subsetting is the honest move here: this task pins
down the slice worth hand-writing and rejects everything outside it.
Here it is a topic: implement it from what you ship.

## Interface

```
prog <file>
```

- Exactly one argument, the path of the TOML file. Any other argv shape
  is a usage error: exit nonzero.
- On success, print one `dotted.key=value\n` line per key-value pair to
  stdout, in the order the keys were defined in the file, and exit 0.
- On any invalid input, print nothing to stdout and exit nonzero. stderr
  is free for diagnostics. All-or-nothing: parse the whole file before
  emitting anything.

## Dialect

The file must be valid UTF-8; anything else is invalid. Lines are
separated by `\n` — a bare `\r` is not a line separator; a trailing `\r`
on a line is stripped (CRLF input is fine). Whitespace in this spec
means ASCII space and tab, nothing else — no vertical tab, no form
feed, no Unicode space.

- Comments: `#` starts a comment that runs to the end of the line,
  anywhere it appears outside an open string — on its own line, or
  trailing after a header or a value. A line that is empty, or all
  comment, after this stripping is ignored.
- Bare keys and table-header segments match `[A-Za-z0-9_-]+` — ASCII
  letters, digits, underscore, dash. No quoted keys. No dotted keys on
  an assignment line (`a.b = 1` is invalid — dotted paths only exist as
  table headers).
- Assignment: `key = value`, one per line, with optional whitespace
  around `=`.
- Values:
  - **Strings** — `"..."` only (no single-quoted literal strings, no
    triple-quoted multiline strings). Escapes: `\"`, `\\`, `\n`, `\t`;
    any other backslash sequence is invalid. A literal control
    character (codepoint below `0x20`, or `0x7F`) inside the string is
    invalid — a real tab must be written `\t`. The string must close
    with a `"` on the same line; anything but whitespace or a comment
    after the closing quote is invalid.
  - **Integers** — optional `-`, then digits, no `+`, no underscores,
    no leading zeros (`0` itself is fine, `-0` is fine and prints as
    `0`). Magnitude must fit in 63 bits: `-2^63 < n < 2^63`
    (`-9223372036854775808` and `9223372036854775808` themselves are
    out of range). Output is canonical decimal — this falls out for
    free, since the value is parsed to an integer and re-printed.
  - **Booleans** — exactly `true` or `false`.
  - Nothing else: no floats, no datetimes, no arrays, no inline
    tables.

## Tables

```
header-line := WS* "[" WS* segment ("." segment)* WS* "]" WS*
segment     := [A-Za-z0-9_-]+
```

- `[a]` or `[a.b.c]` on its own line opens a table; every key on a
  following line, up to the next header, is prefixed with that table's
  dotted path in the output.
- Whitespace is allowed around the whole bracketed name — `[ a.b.c ]`
  — but not around a `.`: `[a . b]` and `[a. b]` are invalid. Pinned
  this way (rather than also allowing space around dots) because "pad
  the two ends, nothing else" is one rule to state and implement,
  versus "whitespace is insignificant anywhere inside the brackets" —
  and padding next to a dot is the rarer style in the wild anyway.
- Redefining the same table header twice is invalid, and assigning the
  same fully-dotted key twice is invalid — whether as two plain
  assignments, or a key that collides with an existing table path (or
  vice versa) — TOML's duplicate-key rule.
- A header that is only a **prefix** of an earlier header may still be
  declared later: `[a.b.c]` implicitly creates `a` and `a.b` as tables
  without giving either its own header, so a later `[a.b]` is declaring
  a table that has no header of its own yet — allowed, per TOML 1.0
  (only re-declaring the *same* header is rejected). A second `[a.b]`
  after that, or a second `[a.b.c]`, would then be the duplicate.

## Output

- Order: definition order in the file — the order keys were assigned,
  not sorted, not grouped by table.
- Values: strings decoded (escapes resolved; a decoded value may
  contain a real newline or tab, same convention as dotenv-parse),
  integers in canonical decimal, booleans as `true`/`false`.

## Expected outputs

Derived by hand from the dialect above; the reference entry is the
executable cross-check.
