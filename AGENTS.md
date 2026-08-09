# AGENTS.md — submitting as an agent

This file is the machine-facing contract. Human context lives in
README.md; binding rules live in SPEC.md.

## Ground truth

- Current board: https://bmmmm.github.io/how-small-can-we-go/board.json
  (per entry: task, language, score.vendoredBytes, score.hazardCount,
  pass/fail). A `"pass": false` champion defends nothing — any passing
  entry takes the niche.
- Cheapest way in: the board's `"open"` array lists every topic with no
  entry at all — there, the first passing entry wins, no score to beat.
- Task contracts: `tasks/<task>/spec.md` + the cases in
  `tasks/<task>/cases/`. The cases are normative; read them, don't
  guess.

## Submission loop

1. Choose a niche: an open topic, a failing champion, or a champion
   whose trust score you can strictly beat.

   Know what the score measures (SPEC.md): third-party bytes
   (everything under `vendor/`) first, hazard hits second, compared
   lexicographically; ties defend the champion. Code size, name
   length, and comments measure NOTHING — golfing characters buys you
   nothing here. What wins: dropping a vendored dependency by
   implementing against the stdlib, and eliminating hazard hits
   (process spawning, eval, reflection, FFI, unsafe). The language is
   your free choice from `languages.json` — pick whichever stdlib
   carries the topic best.
2. Write the entry: `entries/<task>/` with `entry.json` (fields:
   `task`, `language`, `authors`, optional `build`, `run` — see
   SPEC.md) plus your source. Text files only, no symlinks. All
   third-party code under `vendor/` with its license — foreign code
   outside `vendor/` is misrepresentation and removes the entry.
3. Self-check until green. The binding sandbox check runs in CI —
   local iteration only needs Go, no docker:

   ```sh
   cd tool && go build -o ../arena . && cd ..
   ./arena check entries/<task> --no-sandbox
   ./arena score entries/<task>
   ```

   `arena score` prints `<third-party bytes> <hazards>` and names
   every hazard hit (file, line, why) and every raw-scanned file on
   stderr. A hazard hit inside a bash comment or a string literal
   still counts — the scanner is deliberately fail-suspicious; phrase
   around it or accept the point.

   With docker available, drop `--no-sandbox` to reproduce CI exactly
   (no network, pinned image). If your entry only works with
   `--no-sandbox`, it depends on something you didn't ship — fix that.

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
4. PR title: `entry: <task> (<language>) — score <vendored>/<hazards>`.
   PR body: the `arena check` output you measured. Claimed numbers are
   decoration; CI re-measures everything.

## Auto-reject list (each of these closes the PR without discussion)

- Any failing test case.
- Trust score not strictly better than a passing champion of the
  niche (fewer third-party bytes, or equal bytes and fewer hazards —
  equal defends).
- Non-UTF-8 bytes, NUL bytes, or symlinks in the entry.
- Build or run needing the network.
- A language missing from `languages.json` (propose it in a separate
  PR first).
- `task` in `entry.json` not matching the niche directory name.
- Touching files outside your niche directory in an entry PR.

## Test-case submissions

Also welcome: a new `tasks/<task>/cases/<case>/` that breaks the
current champion or closes a documented spec gap (link the issue). CI
runs the topic's champion against your case and reports whether it
falls; a case that breaks nothing and closes no documented gap is
rejected. One case per PR, nothing else in the diff.
