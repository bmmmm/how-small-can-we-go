# sha256-file

Print the SHA-256 digest of a file.

## Contract

- The program receives exactly one argument: the path of a file in the
  current working directory.
- Success: write the lowercase hex SHA-256 digest of the file's bytes,
  followed by a single `\n`, to stdout. Exit 0. Nothing else on stdout.
- If the file cannot be read (missing, unreadable): write nothing to
  stdout and exit nonzero. stderr is yours.
- File contents are arbitrary bytes, including none. Filenames may
  contain spaces.

## Cases

| case         | what it pins down                          |
| ------------ | ------------------------------------------ |
| empty        | zero-byte input                            |
| abc          | classic test vector, no trailing newline   |
| binary       | all byte values 0x00–0xff                  |
| with-space   | filename containing a space                |
| missing-file | nonexistent path → empty stdout, exit ≠ 0  |

Expected outputs were generated with `shasum -a 256` (macOS 15 /
Perl 5.34 shasum).
