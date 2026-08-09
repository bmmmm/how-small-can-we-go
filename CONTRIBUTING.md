# Contributing

Two paths in, depending on what you're bringing. This file only routes —
the rules live in [SPEC.md](SPEC.md) and [AGENTS.md](AGENTS.md); it
doesn't restate them.

## Straight to PR

No issue needed first — open the PR, CI measures it.

- **Entry** for a niche (`entries/<task>/<language>/`). The submission
  loop, including the local self-check to run before opening the PR, is
  in [AGENTS.md](AGENTS.md).
- **Test case** for an existing task. Must break a current entry or close
  a documented spec gap that you link to — SPEC.md → "Test-case
  contributions". One case per PR, nothing else in the diff.
- **New task**: `spec.md` + at least 4 cases (one a failure mode) + one
  passing reference entry, all in the same PR — SPEC.md → "New tasks".
  Consider filing a
  [task proposal](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=02-task-proposal.yml)
  issue first if you want the idea sanity-checked before doing the work.

## Start as an issue

- **Spec gap** — a task's `spec.md` doesn't say what should happen in
  some situation, so two spec-compliant implementations could disagree.
  File a
  [spec gap report](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=01-spec-gap.yml);
  if accepted, it becomes the `#<issue>` a case PR links to.
- **Language proposal** — a new entry in `languages.json`: official
  image, pinned to at least major.minor, works with `--network none`
  (SPEC.md → "Languages"). File a
  [language proposal](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=03-language-proposal.yml)
  so the image choice gets a maintainer look before you write the PR —
  image additions and bumps are deliberate, not automatic.
- **Feature or tooling idea** — anything about the `arena` tool, CI
  gates, or the board that isn't one of the above, including a proposed
  change to an existing task's spec. File a
  [feature request](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=04-feature-request.yml).
- **Not sure, or just asking** —
  [ask a question](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=05-question.yml).

## Before any PR

- Read [SPEC.md](SPEC.md) for the contracts (entry format, measurement,
  gates) and [AGENTS.md](AGENTS.md) for the submission loop and the
  auto-reject list — most rejected PRs hit something already listed
  there.
- [README.md](README.md) has the commands to build `arena` and run it
  locally, including without Docker (`--no-sandbox`, for iteration only —
  the real gate needs the sandbox).

## Security

Found a way to escape the sandbox, escalate CI privilege, or bypass a
gate? Don't open an issue — see [SECURITY.md](SECURITY.md).

## License

Contributions, entries included, are accepted under
[GPL-3.0-or-later](LICENSE) — same as the rest of the repo.
