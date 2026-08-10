# csv-parse

Parse an RFC 4180 CSV file and print it as JSON. Every language ships
a CSV library (Python's `csv`, Go's `encoding/csv`, Node's ecosystem of
them, …); the corners — quoting, embedded separators, line endings —
are where the dialects actually differ. Here it is a topic: implement
it from what you ship.

## Interface

```
prog <file>
```

- Exactly one argument, the path of the CSV file. Any other argv shape
  is a usage error: exit nonzero.
- On success, print exactly one line to stdout: a JSON array of
  records, each record a JSON array of field strings, with no spaces
  after any separator, followed by a newline. Exit 0.
- On any invalid input, print nothing to stdout and exit nonzero.
  stderr is free for diagnostics. All-or-nothing, like dotenv-parse:
  parse the whole file before emitting anything.

## Input dialect (RFC 4180, pinned)

The file must be valid UTF-8; anything else is invalid (exit
nonzero). No header handling, no type inference — every field is
always a string, whatever it looks like.

- Fields are separated by `,`. Records are separated by a record
  terminator, either `CRLF` or a bare `LF` — **this is a deliberate
  deviation from the strict RFC 4180 grammar**, which mandates CRLF
  only. Accepting a bare LF too matches what every real-world CSV
  library and every *nix-authored file actually does, so it is pinned
  in rather than rejected.
- A final trailing record terminator does not open a further, empty
  record: `a,b\n` is one record, not two. An *interior* line that is
  otherwise empty is still a record of its own — one field, the empty
  string — the grammar has no production for a record with zero
  fields, only fields that happen to be empty.
- Empty file (zero bytes) is pinned to `[]` — zero records. This is a
  deliberate special case, not a consequence of the terminator rule
  above (which, read literally, would also allow "one record with one
  empty field" for zero bytes; this pin picks the more useful reading).
- A bare `CR` not followed by `LF`, when it appears **outside** a
  quoted field, is ambiguous as a record terminator and is pinned to
  invalid. Inside a quoted field, `CR` is just literal data like any
  other character the dialect allows there — RFC 4180 places no such
  restriction on quoted content, and neither does this pin.
- **Unquoted field**: any run of bytes except `,`, `CR`, `LF`, and `"`.
  A `"` appearing inside an unquoted field is invalid: once a field has
  started unquoted it cannot turn into a quoted one partway through.
- **Quoted field**: starts with `"`, runs to the matching closing `"`.
  `""` inside a quoted field is an escaped quote (one literal `"` in
  the value); any other character — including `,`, `CR`, and `LF` — is
  literal content. After the closing `"`, only a `,`, a record
  terminator, or end of file may follow; anything else is invalid. An
  unclosed quote — reaching end of file while still inside one — is
  invalid.

## JSON output escaping

Exact and implementation-independent — this defines every byte the
program may emit for a field string, not just the common cases:

- `"` (U+0022) → `\"`
- `\` (U+005C) → `\\`
- U+0008 → `\b`, U+000C → `\f`, U+000A → `\n`, U+000D → `\r`,
  U+0009 → `\t`
- any other code point in U+0000–U+001F → `\u00XX`, lowercase hex,
  zero-padded to two digits
- U+007F and any code point ≥ U+0080 → `\uXXXX`, lowercase hex,
  zero-padded to four digits; a code point above U+FFFF is encoded as
  a UTF-16 surrogate pair, each half its own `\uXXXX`
- every other code point (U+0020–U+007E except `"` and `\`) is
  emitted literally, unescaped

This is exactly `json.dumps(value, ensure_ascii=True)`'s behavior in
CPython — checked against a live interpreter, casing and surrogate
pairs included — so the reference entry uses it directly for field
encoding. Array structure uses `separators=(",", ":")` to drop the
spaces `json.dumps` inserts by default, since the interface allows
none.

## Expected outputs

Derived by hand from the dialect rules above, field by field, then
JSON-escaped by hand from the escaping table above; the reference
entry is the executable cross-check.
