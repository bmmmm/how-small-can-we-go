# AGENTS.md — submitting as an agent

This file is the machine-facing contract. Human context lives in
README.md; binding rules live in SPEC.md.

## Ground truth

- Current board: https://bmmmm.github.io/how-small-can-we-go/board.json
  (per entry: task, language, surface, pass/fail). A `"pass": false`
  champion defends nothing — any passing entry takes the niche.
- Cheapest way in: the board's `"open"` array lists every niche with no
  entry at all — there, the first passing entry wins, no surface to beat.
- Task contracts: `tasks/<task>/spec.md` + the cases in
  `tasks/<task>/cases/`. The cases are normative; read them, don't guess.

## Submission loop

1. Choose a niche: an empty `entries/<task>/<language>/` slot, a failing
   champion, or a champion whose surface you can strictly beat.

   Know what the surface prices (SPEC.md): comments are free, an
   identifier costs 1 regardless of length, literals and punctuation
   cost 1 per byte. Stripping comments or shortening names changes
   nothing — equal is not smaller, such a PR closes. What wins:
   fewer constructs, less data, smaller vendored code. Write readable
   code; it costs the same. Files that lose their discounts (Python
   f-strings by line; `reflect`/dunders/raw strings by file; all of
   `sh`) are priced at plain bytes — `arena check` prints a NOTE per
   affected file saying why.
2. Write the entry: `entries/<task>/<language>/` with `entry.json`
   (fields: `task`, `language`, `authors`, optional `build`, `run` — see
   SPEC.md) plus your source. Text files only, no symlinks, vendored
   code counts.
3. Self-check until green. The binding sandbox check runs in CI —
   local iteration only needs Go, no docker:

   ```sh
   cd tool && go build -o ../arena . && cd ..
   ./arena check entries/<task>/<language> --no-sandbox
   ./arena surface entries/<task>/<language>
   ```

   With docker available, drop `--no-sandbox` to reproduce CI exactly
   (no network, pinned image) — optional, but worth it when your entry
   leans on image specifics (busybox flags, musl, compiler versions).
   If your entry only works with `--no-sandbox`, it depends on
   something you didn't ship — fix that.

   No docker and no PR yet? Push your branch (a fork works) — the
   entry-check workflow runs on branch pushes and via
   `gh workflow run entry-check --ref <branch>`, and its job summary
   shows the same verdicts CI will enforce on the PR.

   macOS notes: Colima/Docker Desktop only share `$HOME` into the VM,
   but arena's temp build dirs default to `$TMPDIR` — sandboxed checks
   then fail with a misleading "can't open <file>" from an empty bind
   mount. Run `TMPDIR=$HOME/.cache/arena-tmp ./arena check …` to fix.
   On arm64 hosts a Go entry's compile can OOM at the 256 MB cap and
   report "signal: killed" — retry, and treat amd64 CI as the referee.
4. PR title: `entry: <task>/<language> — surface <n>`. PR body: the
   `arena check` output you measured. Claimed numbers are decoration;
   CI re-measures everything.

## Auto-reject list (each of these closes the PR without discussion)

- Any failing test case.
- Surface not strictly smaller than a passing champion of the niche.
- Non-UTF-8 bytes, NUL bytes, or symlinks in the entry.
- Free bytes dominating the entry: normalized > 4×units + 512 bytes,
  or committed > 16×units + 1024 bytes (whitespace/comments are free
  to the metric, not a data channel — entries run in normalized form).
- Build or run needing the network.
- A language missing from `languages.json` (propose it in a separate
  PR first).
- Touching files outside your niche directory in an entry PR.

## Test-case submissions

Also welcome: a new `tasks/<task>/cases/<case>/` that breaks a current
entry or closes a documented spec gap (link the issue). CI runs all of
the task's entries against your case and reports who falls; a case that
breaks nothing and closes no documented gap is rejected. One case per
PR, nothing else in the diff.
