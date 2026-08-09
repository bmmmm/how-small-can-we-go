# SPEC — the rules of the arena

Definitions first, then the contracts. When README and SPEC disagree,
SPEC wins.

## Vocabulary

- **Task** — a precisely specified program to write, defined by
  `tasks/<task>/spec.md` and pinned down by its test cases.
- **Niche** — a (task, language) pair. One directory:
  `entries/<task>/<language>/`.
- **Champion** — the entry currently occupying a niche. One entry per
  niche; history lives in git.
- **Audit surface** — what an auditor must process, counted in *audit
  units* across all files of an entry directory, except `entry.json`.
  Lower is better. The pricing (details under *Measurement*): comments
  are free, an identifier costs 1 flat, literals and everything else
  cost 1 per byte, whitespace outside literals is free.

## Entry contract

```
entries/<task>/<language>/
  entry.json      # manifest, excluded from measurement
  ...             # everything else: measured, shipped, audited
```

`entry.json`:

```json
{
  "task": "sha256-file",
  "language": "go",
  "authors": ["your-handle"],
  "build": "go build -o prog main.go",
  "run": "./prog"
}
```

- `task`, `language`, `run`, `authors` are required; `build` is optional.
- `build` and `run` are split on whitespace and executed **without a
  shell** — no pipes, no globs, no quoting. Test-case arguments are
  appended to `run` as separate argv entries.
- Both run with the entry directory (after build) as working directory.
- Vendored dependencies are allowed and count toward the surface. There
  is no other way to depend on anything: the build has no network.

## Measurement

Auditors read tokens, not bytes: `digest` and `d` cost a reader the
same, while every byte of a string literal must be checked one by one.
The surface prices exactly that. Per file:

| construct                        | cost in audit units                |
| -------------------------------- | ---------------------------------- |
| comment                          | 0                                  |
| identifier / keyword occurrence  | 1, plus 1 per byte beyond 16       |
| string & number literals         | 1 per byte, whitespace included    |
| any other non-whitespace byte    | 1                                  |
| whitespace outside literals      | 0                                  |

Renaming `digest` to `d` and stripping comments changes nothing — a
challenger that only uglifies measures *equal*, and equal is not
smaller (gate 1). What still shrinks the number is structure: fewer
constructs, less data, smaller dependencies.

**Discounts only on proof.** The scanner grants the two discounts
(comments, flat identifiers) only where the language's declared syntax
(`languages.json`) lets it prove them safe. Everything doubtful is
priced at 1 unit per non-whitespace byte: files matching a language's
no-discount patterns (reflection doors like Go's `reflect` or Python
dunders, which would turn identifier names into a cheap data channel;
constructs the scanner cannot lex, like Rust raw strings), single
lines matching a line pattern (Python f-strings), and every file of a
language without a syntax config (`sh` — heredocs defeat safe comment
detection, so it ships verbatim at byte prices). A mispriced corner can
therefore only ever cost too much, never too little. `arena check` and
`arena surface` name every file that lost its discounts and why.

**You play what you weigh.** The measured form is the executed form:
before building, comments are stripped and whitespace is collapsed
(indentation survives where it is grammar, e.g. Python; semantic
comments like `//go:` directives and shebangs survive and are priced).
Whatever the metric counted as free provably never runs. Two volume
caps keep the free channels from becoming a covert data store an entry
reads back at runtime: the normalized entry must stay ≤ 4×units + 512
bytes, the committed entry ≤ 16×units + 1024 bytes.

- Every file in the entry directory except `entry.json` is measured.
- Files must be valid UTF-8 without NUL bytes. Symlinks are forbidden.
  Violations fail the check — nothing unauditable ships.
- Multi-byte UTF-8 runes cost their encoded size wherever bytes are
  priced.
- Reported alongside: non-whitespace bytes and non-blank line count
  (informational only).

## Conformance

A task's cases live in `tasks/<task>/cases/<case>/`:

| file     | meaning                                                        |
| -------- | -------------------------------------------------------------- |
| `args`   | one argument per line, appended to `run` (absent = no args)    |
| `files/` | copied into the working directory before the run (optional)    |
| `stdout` | expected stdout, byte-exact (absent = must be empty)           |
| `exit`   | expected exit code: a number, or `nonzero` (absent = `0`)      |

Execution environment:

- The pinned image from `languages.json` for the entry's language.
- **Networking disabled**, 1 CPU, 256 MB memory, pids capped.
- Build timeout 2 min, per-case timeout 20 s.
- stdout is compared byte-exact; stderr is free for diagnostics.

## Gates (what CI enforces on a PR)

1. **Challenger rule.** Replacing an existing champion requires a
   strictly smaller surface. Equal is not smaller.
2. **New niche.** A first entry for an empty niche needs only to pass.
3. **No removals.** An entry leaves the board by being beaten or by
   failing the suite — not by deletion. Deletion PRs are flagged for a
   maintainer.
4. Everything above (measurement, conformance, text-only) must pass.

## Test-case contributions

A new case for an existing task is accepted when at least one of:

- it **breaks a current entry** (CI runs all entries of the task against
  the PR and reports who falls), or
- it **closes a documented spec gap** — link the issue describing the
  underspecified behavior.

A case that discriminates nothing adds runtime and proves nothing; it is
closed. Merged breaking cases take effect at the next board build: broken
champions show as *failing* and their niche is open to any passing entry
(smaller than the failing champion or not — a failing champion defends
nothing).

Cases must respect the task's `spec.md`. Changing the spec itself is a
separate discussion in an issue, not a case PR.

## New tasks

A task PR ships: `spec.md` with a contract precise enough to implement
against, at least 4 cases including at least one failure-mode case, and
at least one passing entry as proof the spec is implementable. Expected
outputs must state the tool that generated them (in `spec.md`).

## Languages

`languages.json` maps a language to its pinned container image and its
measurement syntax (comment markers, string shapes, no-discount
patterns). The image is the trusted base — runtime and stdlib are free,
everything else ships in the entry. Adding a language is a PR touching
only `languages.json`: the image must be an official Docker Hub image,
pinned to at least major.minor, and usable with networking disabled. A
new language may start without a `syntax` block — byte pricing, shipped
verbatim — and gain one in a later PR arguing why each discount is safe
against that language's grammar. Image bumps and syntax changes are
deliberate maintainer PRs, not automatic.

## For the paranoid (rightly so)

Untrusted code runs only inside the no-network container with resource
caps, as an unprivileged user, in a throwaway working directory. CI for
first-time contributors requires maintainer approval before any workflow
runs — that is a GitHub default and it stays on.
