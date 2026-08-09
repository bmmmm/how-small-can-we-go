# TODO — next steps

Working state for the next session. Mirrored as issue #1 on the private
origin. Context for a cold start: v1 is live — board on Pages,
`sha256-file` task with a Go reference entry (surface 193), all CI gates
green and proven to go red. Rules: [SPEC.md](SPEC.md); agent contract:
[AGENTS.md](AGENTS.md).

## 1. Second and third task — make the board a game, not a demo

Each task PR ships `spec.md` + ≥4 cases (incl. one failure mode) + one
passing reference entry (SPEC.md → "New tasks"). Expected outputs come
from a real tool, tool + version named in `spec.md`.

- `base64-file` (easy ramp-up): RFC 4648 encode a file to stdout.
  Vectors via the system `base64`. Mind the 76-col wrapping question —
  the spec must pick one behavior explicitly.
- `json-pretty` (medium): canonical re-emit of a JSON file — sorted
  keys, 2-space indent, `\n`-terminated. Deterministic by construction;
  failure-mode case: invalid JSON → empty stdout, nonzero exit.
- **Constraint to remember:** server-style tasks (http-static etc.) do
  NOT fit the current runner — cases are argv+stdout comparisons and the
  sandbox has `--network=none`. Needs a new case model first; don't ramp
  into it casually.

## 2. Auto-close bot for non-improving entry PRs

Today CI fails the PR and a human closes it. Wanted: a job that comments
the measured numbers and closes. Security design note: that job needs
`pull-requests: write`, so it must NEVER check out or execute PR code —
read the finished check result only (e.g. `workflow_run` trigger).
Acceptance: losing PR gets comment + close with zero untrusted code in
the privileged context.

## 3. Contribution intake

Run the `issue-templates` skill: issue forms (spec-gap report, task
proposal, language proposal), CONTRIBUTING.md, SECURITY.md. The PR
template already exists.

## 4. Merge automation (after 2)

Auto-merge green entry PRs (merge queue or bot approval). Decide the
least-privilege path before building.

## 5. Housekeeping

- `languages.json` image bumps stay manual and deliberate; consider a
  quarterly reminder.
- Announce/seed once ≥3 tasks are live — the board should look fightable
  before inviting people.

## v2 north star: chain league

Entries may depend on the pinned base images + other entries of the
arena. Needs dependency declarations in `entry.json`, rebuild cascades,
per-chain surface totals. Don't start before the solo game has ≥3 tasks
× several languages alive.
