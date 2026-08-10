# TODO — next steps

Working state for the next session. Context for a cold start: the game
was reworked 2026-08-10 — one niche per topic (no per-language
brackets), trust score instead of audit units: third-party bytes
(vendor/) first, hazard hits second, lexicographic, ties defend. Two
topics are live (`semver-range-check` — go reference,
`dotenv-parse` — python reference), both at score 0/0. The auto-close
bot answers losing entry PRs (not-improving and conformance-fail,
result schema 3; infra failures are their own class — arena exit 3 —
and never close anything), and the contribution intake (issue forms,
CONTRIBUTING.md, SECURITY.md) is in place. Rules: [SPEC.md](SPEC.md);
agent contract: [AGENTS.md](AGENTS.md).

## 1. More topics — the actual growth axis

Shipped 2026-08-10: `url-parse`, `jwt-hs256-verify`, `glob-match`,
`csv-parse`, `cron-next-run`, `toml-subset-parse`,
`base58check-decode` — nine niches live, all at 0/0. Remaining
candidate: `markdown-subset-render`. Preference stays: functions
lifted from real projects, pinned dialect in spec.md, ≥4 cases (one
failure mode), one passing reference entry.

## 2. Merge automation — deferred until there is volume

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

## 3. Housekeeping

- `scripts/smoke-test.sh` hardcodes `entries/semver-range-check` and
  `entries/dotenv-parse` in its conformance probes; the seven 2026-08-10
  topics are not covered. Enumerate `entries/*` instead of hardcoding.

- Announce: the rework needs fresh announce drafts (tmp/ has the old
  ones, now stale). Where and how to announce is a human call.
- `languages.json` image bumps stay manual and deliberate; consider a
  quarterly reminder. `bash:5.2` could move to 5.3 on the next pass.
- Hazard lists are v1 curation — expect sharpening PRs (e.g. Go
  `os.ReadFile` on paths from env? C `longjmp`?). Every addition needs
  an argued why.

## v2 north star: chain league

Entries may depend on the pinned base images + other entries of the
arena. Needs dependency declarations in `entry.json`, rebuild cascades,
per-chain score totals. Don't start before the solo game has ≥5 topics
and at least one dethroning fight on the board.
