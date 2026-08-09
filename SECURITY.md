# Security Policy

## Reporting a vulnerability

Report privately via GitHub's
[private vulnerability reporting](https://github.com/bmmmm/how-small-can-we-go/security/advisories/new) —
never a public issue, and not the general "spec gap" or "feature request"
forms either. This is a single-maintainer hobby project, not a security
team with an SLA — expect an acknowledgement within a few days, not
hours.

## What counts as a security issue here

This arena runs untrusted, contributor-submitted code (an entry's `build`
and `run` from `entry.json`) inside CI. Anything that breaks the isolation
model SPEC.md describes under "For the paranoid" is in scope:

- **Sandbox escape from the entry runner** — an entry's `build` or `run`
  step reaches outside its no-network, resource-capped, unprivileged
  container: network access despite `--network none`, filesystem access
  outside the throwaway working directory, privilege escalation inside
  the container, or a resource-cap bypass (CPU, memory, pids).
- **CI privilege escalation** — a way for entry or case content to run
  with a workflow's write permissions, or to influence a run beyond the
  sandboxed check step — workflow-command injection via stdout/stderr or
  `GITHUB_STEP_SUMMARY`, or getting a privileged job to check out and
  execute PR-controlled code. (This is exactly the failure mode the
  auto-close bot design in TODO.md is built to avoid — a job with
  `pull-requests: write` must never execute untrusted code.)
- **Gate bypass** — a way to get an entry measured, checked, or merged
  incorrectly: a surface count that doesn't reflect actual bytes, a
  conformance check that reports a pass on a failing case, or a niche
  awarded without a strictly smaller measured surface (SPEC.md gates
  1–4).
- **Fork PR workflow abuse** — extracting secrets or tokens, or getting
  elevated permissions, through a first-time contributor's PR before it
  has maintainer approval to run.

## Not a security issue — just a game bug

- An entry that's small but wrong in a way the test suite already
  catches. That's a failing conformance check; let CI reject the PR, or
  open a [question](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=05-question.yml)
  if you think the check itself is wrong.
- An entry that's ugly or unreadable but stays inside the sandbox and
  touches nothing it shouldn't. That's a case for a smaller or cleaner
  challenger, not a report — the game handles it.
- A task spec that's underspecified. That's a
  [spec gap report](https://github.com/bmmmm/how-small-can-we-go/issues/new?template=01-spec-gap.yml),
  not a vulnerability.

## Scope

In scope: this repository's tooling (`tool/`), its CI workflows
(`.github/workflows/`), and the sandboxing model they implement.

Out of scope: the underlying container runtime, Docker itself, or GitHub
Actions' own isolation — report those upstream, to their maintainers.
