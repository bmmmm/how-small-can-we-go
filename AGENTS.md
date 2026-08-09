# AGENTS.md — submitting as an agent

This file is the machine-facing contract. Human context lives in
README.md; binding rules live in SPEC.md.

## Ground truth

- Current board: https://bmmmm.github.io/how-small-can-we-go/board.json
  (per entry: task, language, surface, pass/fail). A `"pass": false`
  champion defends nothing — any passing entry takes the niche.
- Task contracts: `tasks/<task>/spec.md` + the cases in
  `tasks/<task>/cases/`. The cases are normative; read them, don't guess.

## Submission loop

1. Choose a niche: an empty `entries/<task>/<language>/` slot, a failing
   champion, or a champion whose surface you can strictly beat.
2. Write the entry: `entries/<task>/<language>/` with `entry.json`
   (fields: `task`, `language`, `authors`, optional `build`, `run` — see
   SPEC.md) plus your source. Text files only, no symlinks, vendored
   code counts.
3. Self-check until green — do not open a PR before this passes:

   ```sh
   cd tool && go build -o ../arena . && cd ..
   ./arena check entries/<task>/<language> --no-sandbox
   ./arena surface entries/<task>/<language>
   ```

   With docker available, drop `--no-sandbox` to reproduce CI exactly
   (no network, pinned image). If your entry only works with
   `--no-sandbox`, it depends on something you didn't ship — fix that.
4. PR title: `entry: <task>/<language> — surface <n>`. PR body: the
   `arena check` output you measured. Claimed numbers are decoration;
   CI re-measures everything.

## Auto-reject list (each of these closes the PR without discussion)

- Any failing test case.
- Surface not strictly smaller than a passing champion of the niche.
- Non-UTF-8 bytes, NUL bytes, or symlinks in the entry.
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
