# TODO — next steps

Working state for the next session. Mirrored as issue #1 on the private
origin. Context for a cold start: three tasks are live (`sha256-file`,
`base64-file`, `json-pretty`, one Go reference entry each), the
auto-close bot answers losing entry PRs (comment + close via
`workflow_run`, no untrusted code in the privileged context), and the
contribution intake (issue forms, CONTRIBUTING.md, SECURITY.md) is in
place. Rules: [SPEC.md](SPEC.md); agent contract: [AGENTS.md](AGENTS.md).

## 1. Merge automation

Auto-merge green entry PRs (merge queue or bot approval). Decide the
least-privilege path before building — same standard as the auto-close
bot: the privileged context never executes PR code.

## 2. Auto-close bot follow-ups (from the 2026-08-09 security review)

- Runner can't distinguish infra failure from conformance failure:
  docker exit 125 (rate-limited pull, daemon hiccup) reads as a case
  FAIL — and as a PASS for `exit: nonzero` cases. Fix in
  `tool/internal/arena/runner.go` (treat sandbox exit 125 as an error,
  emit an `infra-error` class), then re-add `conformance-fail` to
  `AUTO_REJECT` in entry-close.yml — the comment branch for it is
  already there.
- Replace `actions/download-artifact` in entry-close.yml with
  `gh api …/zip` + `unzip -p` (removes the zip-extraction dependency
  from the privileged job; also makes the size cap binding).
- Smaller: multi-fault PRs cite only the dominant class in the close
  comment; SHA-pin actions in the privileged workflow; paginate the
  comment-dedupe scan; `base.sha` vs recomputed merge ref drift;
  `tonumber? // .` in the emit jq hides producer bugs.

## 3. Housekeeping

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
