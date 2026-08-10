# glob-match

Decide whether a string matches a glob pattern — the question behind
`fnmatch`, shell globbing, and every "does this path match my ignore
pattern" library (Python's `fnmatch`, glob(3), minimatch, …). Every
one of those picks its own dialect in the corners — whether `]` can
open a bracket, whether `-` at the edges is a range or a literal,
whether ranges compare bytes or codepoints. Here it is a topic:
implement it from what you ship, against one dialect pinned below.

## Interface

```
prog <pattern> <string>
```

- Both arguments are required. Any other argv shape is a usage error:
  exit nonzero.
- If the string matches the pattern, print exactly `yes\n` to stdout;
  otherwise print exactly `no\n`. Exit 0 in both cases.
- If the pattern does not conform to the dialect below, print nothing
  to stdout and exit nonzero. stderr is free for diagnostics. (The
  string is never "invalid" — any sequence of Unicode codepoints is a
  legal string to test; only the pattern can be malformed.)

## Dialect

A POSIX `fnmatch` subset — pure string matching, no filesystem
involvement, no shell semantics:

- `*` matches any run of characters, including the empty run.
- `?` matches exactly one character.
- `[...]` is a bracket expression: it matches exactly one character
  drawn from the set described inside the brackets. The set is built
  from literal characters and ranges `a-z`; multiple ranges and
  literals may mix in one bracket expression (`[a-cx-z0]` is the set
  `{a, b, c, x, y, z, 0}`).
- `[!...]` negates the bracket expression: it matches exactly one
  character that is *not* in the described set.
- Any other character matches itself, literally.
- No `**`, no `\` escaping, no path or dotfile special-casing — `/`
  and `.` are ordinary characters like any other. A pattern with no
  `*`, `?`, or `[` only matches the identical string.

Matching runs over the Unicode codepoints of the (UTF-8-decoded)
arguments, not bytes: `?` and each bracket expression consume one
codepoint, not one UTF-8 byte, so a multi-byte character is a single
unit to match against.

### Bracket-expression corners

- `]` as the first character inside a bracket — right after `[`, or
  right after the `[!` negation marker — is a literal member of the
  set, not the closing bracket; the closing bracket is the *next* `]`
  after that position. So `[]ab]` is the set `{], a, b}`, and `[!]ab]`
  is "anything except `]`, `a`, `b`".
- Because a leading `]` is always literal, an empty bracket `[]`
  cannot occur: `[` immediately followed by `]` opens the set with a
  literal `]` and keeps scanning for the real closing bracket. If
  nothing closes it, the pattern falls through to the unclosed-bracket
  rule below — "empty" brackets are unclosed brackets, not a separate
  case.
- `-` as the first or last character of the bracket's content (right
  after the optional leading-`]` literal, or right before the closing
  `]`) is a literal `-`, not a range operator — a range needs a
  character on both sides. `[-az]` is `{-, a, z}`; `[az-]` is the same
  set.
- `!` anywhere in a bracket expression other than immediately after
  `[` (or immediately after the bracket's own opening position) is a
  literal `!`. `[a!]` is `{a, !}`.
- An unclosed `[` — one with no matching `]` anywhere after it — is an
  invalid pattern: exit nonzero, nothing on stdout.
- A range `lo-hi` where `lo` sorts after `hi` by codepoint (`[z-a]`)
  is an invalid pattern: exit nonzero, nothing on stdout. Range
  endpoints compare as Unicode codepoint values — not bytes, not
  locale collation.

## Expected outputs

Derived by hand from the dialect rules above: each pattern is walked
into its token sequence (literals, `?`, `*`, and each bracket's
literal/range membership set and negation flag), then that sequence is
traced against the codepoints of the string, including which `*`
tokens have to backtrack to find a match. The reference entry is the
executable cross-check.
