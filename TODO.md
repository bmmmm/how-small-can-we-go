# TODO — next steps

Working state for the next session. Mirrored as issue #1 on the private
origin. Context for a cold start: three tasks are live (`sha256-file`,
`base64-file`, `json-pretty`, one Go reference entry each), the
auto-close bot answers losing entry PRs (not-improving and
conformance-fail; infra failures are their own class — arena exit 3 —
and never close anything), and the contribution intake (issue forms,
CONTRIBUTING.md, SECURITY.md) is in place. Rules: [SPEC.md](SPEC.md);
agent contract: [AGENTS.md](AGENTS.md).

## 1. Merge automation

Auto-merge green entry PRs (merge queue or bot approval). Decide the
least-privilege path before building — same standard as the auto-close
bot: the privileged context never executes PR code.

## 2. Housekeeping

- Announce/seed: the board now has 3 tasks — fightable. Seeding entries
  in more languages (python, sh, c, rust) makes it look alive before
  inviting people.
- `languages.json` image bumps stay manual and deliberate; consider a
  quarterly reminder.

## v2 north star: chain league

Entries may depend on the pinned base images + other entries of the
arena. Needs dependency declarations in `entry.json`, rebuild cascades,
per-chain surface totals. Don't start before the solo game has ≥3 tasks
× several languages alive — tasks are there, languages are not yet.
