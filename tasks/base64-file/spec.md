# base64-file

Print the Base64 encoding of a file.

## Contract

- The program receives exactly one argument: the path of a file in the
  current working directory.
- Encoding: RFC 4648 section 4 ("Base 64 Encoding"), the standard
  alphabet (`A`-`Z`, `a`-`z`, `0`-`9`, `+`, `/`), with `=` padding.
- **No line wrapping.** The encoded output is a single line regardless
  of input size, followed by exactly one trailing `\n`. This is the
  known ambiguity this spec pins down: several common `base64` tools
  (e.g. GNU coreutils `base64`) wrap output at 76 columns by default.
  This task requires unwrapped output — implementations that wrap will
  fail the `binary` case below.
- Success: write the Base64 encoding of the file's bytes, followed by a
  single `\n`, to stdout. Exit 0. Nothing else on stdout. For a
  zero-byte file, the encoding is the empty string, so stdout is just
  `\n`.
- If the file cannot be read (missing, unreadable): write nothing to
  stdout and exit nonzero. stderr is yours.
- File contents are arbitrary bytes, including none. Filenames may
  contain spaces.

## Cases

| case         | what it pins down                                        |
| ------------ | --------------------------------------------------------- |
| empty        | zero-byte input → output is just `\n`                     |
| abc          | no padding needed (length ≡ 0 mod 3), no trailing newline in input |
| one-byte     | length ≡ 1 mod 3 → `==` padding                            |
| two-byte     | length ≡ 2 mod 3 → `=` padding                              |
| binary       | all byte values 0x00–0xff; output is 344 chars on one line — long enough that a wrapping implementation (e.g. 76-column wrap) fails |
| with-space   | filename containing a space                                |
| missing-file | nonexistent path → empty stdout, exit ≠ 0                  |

Expected outputs were generated with `/usr/bin/base64` (macOS 26.6).
That binary already emits unwrapped, single-line output with no `-b`
flag given, and appends a trailing `\n` after the encoded data — no
transformation was needed to match the format pinned down above.
