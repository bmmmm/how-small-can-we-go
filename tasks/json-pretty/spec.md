# json-pretty

Re-emit a JSON file in a canonical pretty form.

## Contract

- The program receives exactly one argument: the path of a file in the
  current working directory.
- Success: write the canonical form (below) of the file's JSON value to
  stdout. Exit 0. Nothing else on stdout.
- Failure: if the file cannot be read, or its content is not exactly one
  JSON value, write nothing to stdout and exit nonzero. stderr is yours.

The input is a UTF-8 file containing one JSON value (RFC 8259), optionally
surrounded by whitespace. Anything beyond that one value — a second value,
a stray bracket, a comment — makes the input invalid. An empty file is
invalid.

Behaviour on input that is not valid UTF-8 is not specified by this task
and no case exercises it.

## Canonical form

The output is the value rendered by the rules below, followed by a single
`\n`. Nothing else — no BOM, no second newline.

### Layout

- Indent is two spaces per nesting level.
- An object with members: `{`, newline, then each member as
  `<indent>"key": <value>`, members separated by `,` + newline, then
  newline + closing indent + `}`. Exactly one space after the `:`, none
  before it.
- An array with elements: `[`, newline, then each element as
  `<indent><value>`, separated by `,` + newline, then newline + closing
  indent + `]`.
- An **empty** object is `{}` and an empty array is `[]` — two bytes, no
  newline, no space in between, at any depth.
- No trailing commas. No whitespace at the end of a line.
- Whitespace in the input carries no information and is discarded; the
  output layout depends only on the value.

### Object keys

- Members are sorted by their key. The order is a byte-wise comparison of
  the keys' UTF-8 encodings (`memcmp`, shorter key first when one is a
  prefix of the other). For valid UTF-8 this is identical to Unicode
  code-point order. It is not locale-, case-, or width-aware: `"Z"`
  precedes `"a"`, `"a b"` precedes `"ab"`, and every non-ASCII key sorts
  after every ASCII one.
- Two members of the same object whose keys are equal after unescaping
  are duplicates. **The last occurrence in the input wins**; earlier ones
  are dropped entirely. The surviving member is placed by the sort like
  any other.

### Numbers

**A number token is copied to the output byte for byte, exactly as it
appears in the input.** No normalisation, no float round-trip, no
re-formatting.

So `1.0` stays `1.0`, `1.500` stays `1.500`, `-0` stays `-0` (and is not
merged with `0` or with `-0.0`), `9007199254740993` stays itself rather
than becoming `9007199254740992`, and a 30-digit integer or a 22-digit
decimal survives unharmed.

Rationale: this is the only number rule that is deterministic without
naming a numeric model. Every canonicalising alternative has to pin down
a precision, a rounding mode and a shortest-representation algorithm, and
implementations disagree on all three. Preservation makes the rule
implementable in any language that can hold the raw token.

### Strings

Input escapes are **not** preserved. Every string is decoded to a
sequence of code points first, then re-emitted by the fixed rule below.
An input escape for U+0041 comes back as a plain `A`; a surrogate pair
for U+1F600 comes back as the raw four-byte UTF-8 encoding of that code
point. See the `escapes` case for both.

Escaped on output, and nothing else:

| character                          | emitted as                     |
| ---------------------------------- | ------------------------------ |
| `"` U+0022                         | `\"`                           |
| `\` U+005C                         | `\\`                           |
| U+0008 U+000C U+000A U+000D U+0009 | `\b` `\f` `\n` `\r` `\t`       |
| other U+0000–U+001F                | `\u00XX`, hex digits lowercase |
| U+2028, U+2029                     | `\u` + `2028` / `\u` + `2029`  |

Everything else is emitted as raw UTF-8. In particular `/` is **not**
escaped — an input escape for U+002F comes back as a plain `/` — and
neither are `<`, `>`, `&` or `'`. A JSON emitter that HTML-escapes those
by default has to be told not to. U+007F (DEL) is raw as well.

U+2028 and U+2029 are the one deliberate exception to minimal escaping:
they are line terminators in JavaScript, and escaping them keeps the
output pasteable into a JS source file. It costs nothing to implement and
it is a rule, not a judgement call.

### Top level

A JSON value of any type may appear at the top level. `42`, `"str"`,
`true` and `null` are valid inputs and are emitted as themselves plus the
terminating `\n`.

## Cases

| case         | what it pins down                                              |
| ------------ | -------------------------------------------------------------- |
| nested       | recursive sorting, nesting indent, `": "` and `,` + newline     |
| reindent     | input whitespace is discarded, not preserved or patched         |
| empties      | `{}` / `[]` / `""`, at top level and nested                     |
| escapes      | escape normalisation, surrogate pairs, `/` and `<` `>` `&` raw  |
| numbers      | byte-for-byte token preservation past double precision          |
| keysort      | byte-wise key order — case-sensitive, space-significant         |
| dup-keys     | duplicate key: last occurrence wins                             |
| scalar       | bare top-level scalar with surrounding whitespace               |
| invalid-json | malformed JSON → empty stdout, exit ≠ 0                         |
| two-values   | two JSON values in one file → empty stdout, exit ≠ 0            |

## Expected outputs

The `stdout` of every success case was generated with **jq 1.8.2**:

```sh
jq --sort-keys --indent 2 . tasks/json-pretty/cases/<case>/files/input.json
```

The two failure cases have no generated output: empty stdout and a
nonzero exit are what this spec requires, not what a tool printed.

### Where jq 1.8.2 and this spec part ways

jq reproduces the spec byte for byte on every committed case. It does not
implement the spec everywhere, and the divergences are listed here so
nobody mistakes "jq did it" for "the spec says so". **On these inputs the
spec text decides, not jq.**

- **Exponent and very small numbers.** jq re-renders number literals
  through decNumber's scientific-string form: `1e10` becomes `1E+10`,
  `0.0000001` becomes `1E-7`. The spec preserves the token. No case uses
  a number that jq would rewrite.
- **U+007F.** jq escapes it as `\u` + `007f`; the spec emits the raw
  byte.
- **U+2028 / U+2029.** jq emits them raw; the spec escapes them.
- **Multiple values.** jq reads a *stream* of JSON values and prints each
  one, so `{"a":1} {"b":2}` succeeds under jq. This task reads exactly
  one value, which is why `two-values` is a failure case.
- **Empty file.** jq exits 0 with no output; the spec calls it invalid.

Each of these is a candidate for a future case PR under SPEC.md
"Test-case contributions" — with the expected bytes taken from the spec
above rather than from jq.
