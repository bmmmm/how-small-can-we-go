# dotenv-parse

Parse a dotenv file and print its variables, normalized. Every
ecosystem has a library for this (dotenv, python-dotenv, godotenv, …);
dialects differ in the corners, so this spec pins one dialect down. Here
it is a topic: implement it from what you ship.

## Interface

```
prog <file>
```

- Exactly one argument, the path of the dotenv file. Any other argv
  shape is a usage error: exit nonzero.
- On success, print one `KEY=VALUE\n` per variable to stdout (VALUE with
  escapes decoded — it may contain real newlines) and exit 0.
- On any invalid line, print nothing to stdout and exit nonzero. stderr
  is free for diagnostics. All-or-nothing: parse the whole file before
  emitting anything.

## Input dialect

The file must be valid UTF-8; anything else is invalid (exit nonzero).
Lines are separated by `\n` — a bare `\r` is not a line separator; a
trailing `\r` on a line is stripped (CRLF input is fine). Whitespace
in this spec means ASCII space and tab.

- A line whose content is empty after trimming whitespace, or whose
  first non-whitespace character is `#`, is ignored.
- Every other line must be `KEY=VALUE`, optionally prefixed with
  `export` and whitespace. Whitespace around `=` is allowed and
  ignored. After the `export` prefix is stripped, the same rules
  apply — `export = 1` is invalid (empty key), while `export=1`
  assigns the key `export`.
- KEY matches `[A-Za-z_][A-Za-z0-9_]*` (ASCII only). Anything else is
  invalid.
- VALUE, after trimming surrounding whitespace:
  - starts with `"`: double-quoted. Runs to the closing `"`. Escapes:
    `\n` (newline), `\t` (tab), `\"` (quote), `\\` (backslash); any
    other `\` sequence is invalid. The quote must close on the same
    line, and only whitespace may follow it — multiline values are out
    of scope and invalid.
  - starts with `'`: single-quoted, literal — no escapes. Same closing
    rules as double quotes.
  - otherwise: unquoted — the trimmed rest of the line, taken
    literally. There is no inline-comment stripping: a `#` inside an
    unquoted value is part of the value.
  - nothing after `=`: the empty string.

## Output

- Order: keys appear in the order of their first occurrence in the
  file.
- Duplicates: the last assignment wins (only the value, not the
  position).

## Expected outputs

Written by hand from this dialect definition. The reference entry is
the executable cross-check.
