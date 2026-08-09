# TODO — next steps

Working state for the next session. Mirrored as issue #1 on the private
origin. Context for a cold start: three tasks are live (`sha256-file`,
`base64-file`, `json-pretty`, one Go reference entry each), the
auto-close bot answers losing entry PRs (not-improving and
conformance-fail; infra failures are their own class — arena exit 3 —
and never close anything), and the contribution intake (issue forms,
CONTRIBUTING.md, SECURITY.md) is in place. Rules: [SPEC.md](SPEC.md);
agent contract: [AGENTS.md](AGENTS.md).

## 1. Merge automation — deferred until there is volume

Decision (2026-08-09): no merge bot before real contributor volume
exists. Until then: maintainer approve + GitHub native auto-merge.

Design to build when it is needed — the artifact trust asymmetry rules
out everything simpler: the close bot may act on the attacker-controlled
artifact because its worst case is a PR staying open; a merge bot never
may, because its worst case is hostile code on main. So the bot
re-computes the verdict itself: a `workflow_run` workflow from the
default branch, job 1 (`contents: read`) fetches the PR head as data
only — `entries/` files, never workflows or `tool/` — and re-runs
main's arena in the no-network sandbox; job 2 (`contents: write`,
needs job 1) verifies the PR binding like the close bot does and
squash-merges. A container escape in job 1 only ever sees a read token.

## 2. Housekeeping

- Announce: ready — 14 entries across 5 languages landed 2026-08-09
  (json-pretty/sh stays a documented open niche: busybox awk cannot
  meet the spec). Where and how to announce is a human call.
- `languages.json` image bumps stay manual and deliberate; consider a
  quarterly reminder.

## v2 north star: chain league

Entries may depend on the pinned base images + other entries of the
arena. Needs dependency declarations in `entry.json`, rebuild cascades,
per-chain surface totals. Don't start before the solo game has ≥3 tasks
× several languages alive — tasks are there, languages are not yet.
