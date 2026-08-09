# SPEC — the rules of the arena

Definitions first, then the contracts. When README and SPEC disagree,
SPEC wins.

## Vocabulary

- **Task** (topic) — a precisely specified program to write, defined by
  `tasks/<task>/spec.md` and pinned down by its test cases.
- **Niche** — one task. One directory: `entries/<task>/`. There is no
  per-language bracket: languages do not compete with each other here,
  entries do.
- **Champion** — the entry currently occupying a niche. One entry per
  task; history lives in git.
- **Trust score** — how much unreviewed trust an entry demands, in two
  dimensions compared lexicographically (details under *Measurement*):
  1. **third-party bytes** — every byte under `vendor/`,
  2. **hazards** — occurrences of the language's declared hazard
     patterns.
  Lower is better. `(0, 0)` — no foreign code, no hazardous
  constructs — is the perfect score.

## Entry contract

```
entries/<task>/
  entry.json      # manifest, excluded from scoring
  ...             # your source
  vendor/...      # all third-party code, license files included
```

`entry.json`:

```json
{
  "task": "semver-range-check",
  "language": "go",
  "authors": ["your-handle"],
  "build": "go build -o prog main.go",
  "run": "./prog"
}
```

- `task`, `language`, `run`, `authors` are required; `build` is
  optional. `task` must equal the directory name — the directory is the
  niche.
- `language` is your choice, one of the languages in `languages.json`
  (currently: bash, c, go, python, rust). The language is part of the
  submission, not of the niche — a Python champion can be dethroned by
  a C entry and vice versa.
- `build` and `run` are split on whitespace and executed **without a
  shell** — no pipes, no globs, no quoting. Test-case arguments are
  appended to `run` as separate argv entries.
- Both run with the entry directory (after build) as working directory.
- **All third-party code lives under `vendor/`**, with its license.
  There is no other way to depend on anything: the build has no
  network. Passing foreign code off as your own — pasting it outside
  `vendor/`, or compiling it into "data" files your code interprets —
  is misrepresentation; such entries are removed regardless of score.

## Measurement

The score prices trust, not size. Code length, name length, comment
volume — none of it measures anything. What measures:

1. **Third-party bytes.** The byte total of everything under a
   vendored path segment — `vendor`, `vendored`, `third_party`,
   `thirdparty`, `third-party`, `deps`, `extern`, `external`, matched
   case-insensitively — data and license files included: a vendored
   blob is trusted freight either way. Ideal: zero — the task solved
   from the language and its standard library alone. The pinned
   container image (runtime + stdlib) is the trusted base and free;
   everything else you ship weighs.
2. **Hazards.** Each language declares, in `languages.json`, a curated
   list of hazard patterns with a documented `why` — constructs that
   demand extra reviewer trust: process execution, dynamic code
   evaluation, reflection, FFI, unsafe memory, unbounded writes. Every
   occurrence in **every shipped file** counts, vendored or not — an
   entry can execute a file of any name, so files without the
   language's source extension are scanned too (raw). The manifest's
   `build` and `run` commands are scanned as well: they execute, so
   they are code. For languages whose reader joins `\`-newline
   continuations (c, python, bash) the scan joins them first — a
   spliced spelling is the same construct. `arena check` and
   `arena score` name every hit with file, line, and reason.

A challenger beats a champion when its score is **strictly better**:
fewer third-party bytes, or equally many and fewer hazards. Equal is
not better — the champion defends ties; first-mover advantage is
deliberate, churn without improvement is noise. A **buggy** champion
is dethroned differently: ship the breaking test case first (see
*Test-case contributions*) — a failing champion defends nothing, so
the niche opens to any passing entry, score regardless.

**Documentation is not penalized — where that is provable.** Before
the hazard scan, comments are stripped wherever the language's
declared syntax (`languages.json`) lets the scanner prove the strip
safe. Anything doubtful is scanned raw, comments included: files
matching a lex guard (Rust raw strings, C trigraphs), single lines
matching a line guard (Python f-strings), constructs the scanner
cannot lex, and every file of a language without a strip config
(bash — heredocs defeat safe comment detection). A doubtful corner
can therefore only ever overcount hazards, never hide one. The tools
name every raw-scanned file and why. Two deliberate exceptions:
semantic comments (Go's `//go:` directives) are instructions to the
toolchain, so they survive the strip and can score — that is how
`//go:linkname` costs. And string literals are tracked but not
stripped: a hazard pattern inside a string may count.
Fail-suspicious is the accepted trade everywhere.

- Every file in the entry directory except `entry.json` is subject to
  scoring. `entry.json` must stay a manifest: only the five known
  fields (`build` optional), at most 4096 bytes — and it is not
  copied into the run directory at all.
- Files must be valid UTF-8 without NUL bytes. Symlinks are forbidden.
  Violations fail the check — an entry is reviewable in full or it
  does not ship.
- Entries build and run exactly as committed.

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
   strictly better trust score. Equal is not better.
2. **New niche.** A first entry for an empty niche needs only to pass.
3. **No removals.** An entry leaves the board by being beaten or by
   failing the suite — not by deletion. Deletion PRs are flagged for a
   maintainer.
4. Everything above (scoring, conformance, text-only) must pass.

## Test-case contributions

A new case for an existing task is accepted when at least one of:

- it **breaks the current champion** (CI runs the champion against the
  PR and reports the fall), or
- it **closes a documented spec gap** — link the issue describing the
  underspecified behavior.

A case that discriminates nothing adds runtime and proves nothing; it
is closed. Merged breaking cases take effect at the next board build: a
broken champion shows as *failing* and its niche is open to any passing
entry (better-scored than the failing champion or not — a failing
champion defends nothing).

Cases must respect the task's `spec.md`. Changing the spec itself is a
separate discussion in an issue, not a case PR.

## New tasks

Topics are the maintainer's cut of real-world work — often a function
lifted from an actual project, and by preference something the world
usually solves by importing a library: parsers, format converters,
validators, protocol pieces. That is where the score has tension —
vendor the usual dependency and weigh it, or write it yourself and
weigh nothing.

A task PR ships: `spec.md` with a contract precise enough to implement
against (dialect pinned, corner cases decided), at least 4 cases
including at least one failure-mode case, and at least one passing
entry as proof the spec is implementable. `spec.md` must state how the
expected outputs were derived.

## Languages

`languages.json` maps each playable language to:

- its pinned container image — the trusted base: runtime and stdlib are
  free, everything else ships in the entry;
- its source `extensions` — which files the hazard scan applies to;
- its `hazards` — the curated patterns, each with a `why`. Curation is
  deliberate: a hazard list is an argument about the language, not a
  style guide. Extending or correcting one is a PR touching only
  `languages.json`;
- optionally a `strip` config — comment syntax, string shapes, lex
  guards — so documentation is never counted as a hazard. A language
  without one scans raw (bash).

Adding a language is a `languages.json` PR: an official Docker Hub
image, pinned to at least major.minor, usable with networking disabled,
plus a hazard list with argued whys. Image bumps and config changes are
deliberate maintainer PRs, not automatic.

## For the paranoid (rightly so)

Untrusted code runs only inside the no-network container with resource
caps, as an unprivileged user, in a throwaway working directory. CI for
first-time contributors requires maintainer approval before any
workflow runs — that is a GitHub default and it stays on.
