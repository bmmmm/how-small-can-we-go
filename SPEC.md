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
- **Audit surface** — the count of non-whitespace bytes across all files
  of an entry directory, except `entry.json`. Multi-byte UTF-8 runes cost
  their encoded size. Lower is better.

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

- Every file in the entry directory except `entry.json` is measured.
- Files must be valid UTF-8 without NUL bytes. Symlinks are forbidden.
  Violations fail the check — nothing unauditable ships.
- Reported alongside: non-blank line count (informational only).

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

`languages.json` maps a language to its pinned container image. The image
is the trusted base — runtime and stdlib are free, everything else ships
in the entry. Adding a language is a PR touching only `languages.json`:
the image must be an official Docker Hub image, pinned to at least
major.minor, and usable with networking disabled. Image bumps are
deliberate maintainer PRs, not automatic.

## For the paranoid (rightly so)

Untrusted code runs only inside the no-network container with resource
caps, as an unprivileged user, in a throwaway working directory. CI for
first-time contributors requires maintainer approval before any workflow
runs — that is a GitHub default and it stays on.
