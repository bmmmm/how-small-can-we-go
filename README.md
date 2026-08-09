# how small can we go?

**The trust-golf arena.** Small here does not mean short code — it
means a small *trust footprint*: how little foreign code, how few
dangerous constructs a program needs to do a real job. Pick a topic,
implement it in any allowed language, dethrone the champion by needing
less trust than it does.

**Board:** https://bmmmm.github.io/how-small-can-we-go/

## The game in 60 seconds

1. Pick a topic in [`tasks/`](tasks/). One topic = one niche = one
   champion — the entry in `entries/<task>/`. The language is *your*
   choice (bash, c, go, python, rust — see
   [`languages.json`](languages.json)); languages don't compete here,
   entries do.
2. Your entry must pass every test case of the topic, build and run
   with **no network**, ship **only text files**, and have a strictly
   better **trust score** than the champion:
   - **third-party bytes** first — everything under `vendor/`; zero
     means the topic is solved from the language and stdlib alone,
   - **hazards** second — occurrences of the language's declared
     danger patterns (process execution, eval, reflection, FFI, unsafe
     memory), each one named with file, line, and reason.
   Ties defend the champion.
3. Open a PR that replaces the niche directory. CI measures; it never
   believes. Beat the score or the PR closes.

An entry is deliberately small ceremony: one tiny `entry.json`
manifest plus your source — the whole format is in [SPEC.md](SPEC.md).

An empty niche (a topic with no entry yet)? The first passing entry
takes it.

## Why this score? (the honest version)

Every dependency you pull is code you didn't read, maintained by
people you don't know, fetched through infrastructure you don't
control. Supply-chain attacks live exactly there. This arena flips the
habit: the usual library import becomes the *expensive* move — vendor
it and every byte weighs — while implementing from the stdlib weighs
nothing.

The second dimension prices the constructs that make code hard to
trust even when you can read it: spawning processes, evaluating data
as code, reflection, FFI, unsafe memory. Each language's list is
curated and argued in [`languages.json`](languages.json) — every
pattern carries its *why*, and every hit is reported with file and
line.

What deliberately does **not** count: code length, name length,
comments. Readable, well-documented code costs nothing extra —
comments are stripped before the hazard scan wherever the language is
provably lexable (bash isn't; it scans raw, comments included). This
is a game about demanded trust, not a security certification: a
zero-score entry can still hold a bug — that is what the
ever-sharpening test suites are for, and a breaking test case is
exactly how a buggy champion gets dethroned.

## What keeps the slop out

- **Measured, never claimed.** CI builds in a container with networking
  disabled and runs the topic's full test suite. Numbers in a PR body
  are decoration; the gate only trusts its own measurement.
- **Improve or be closed.** A challenger must score *strictly* better
  than the champion it replaces. An entry that beats nothing is closed
  automatically — zero maintainer attention spent.
- **Text only.** Invalid UTF-8, NUL bytes, or symlinks anywhere in an
  entry fail the check. An entry is reviewable in full or it does not
  ship.
- **Tests are contributions too.** A new test case is accepted when it
  breaks the current champion or closes a documented gap in a topic's
  spec. The suite gets sharper with every round, and the weekly re-run
  demotes champions that no longer pass.

AI-written entries are welcome — see [AGENTS.md](AGENTS.md) for the
machine-readable contract. AI slop is not, and the difference is
measured, not vibes.

## Run it locally

```sh
cd tool && go build -o ../arena . && cd ..
./arena check entries/semver-range-check --no-sandbox   # host run, for iteration
./arena check entries/semver-range-check                # the real thing (needs docker)
./arena score entries/semver-range-check                # just the two numbers
./arena board --no-sandbox                              # render docs/ locally
./scripts/smoke-test.sh                                 # end-to-end: real sandbox + gaming-resistance
```

`scripts/smoke-test.sh` is the real-world check: it drives the built
binary through the actual no-network docker sandbox and builds live
evasion entries to prove the scorer still catches them. With no docker
daemon it says so, asserts the daemon-down path reports INFRA rather
than a false failure, and falls back to a host run. CI runs it on every
`tool/` change.

No docker? Push your branch — the entry-check workflow runs the real
sandboxed measurement on branch pushes touching `entries/`, `tasks/`,
or `languages.json`, and on
`gh workflow run entry-check --ref <branch>`, so CI does the containers
for you. Local `--no-sandbox` plus Go is all you need.

The full rules live in [SPEC.md](SPEC.md).

## Roadmap

No timelines, no "coming soon" — this is what's planned, not what's
built:

- **More topics.** The board grows one niche at a time, by preference
  functions lifted from real projects where the world usually reaches
  for a library. Topic proposals are welcome via issues.
- **Auto-merge for green entries.** A least-privilege bot merges a
  passing challenger PR on its own — beaten champion out, better
  challenger in, no human in the loop.
- **The chain (v2) — north star.** Once the solo game works: entries
  that may depend only on the pinned base images *and other entries of
  this arena*. Compose bigger programs from small trusted ones and
  watch the total score of the chain. That flips the usual dependency
  story: instead of trusting an ecosystem, you trust a board where
  every link was measured and fought over.

## Support

If this arena is your kind of fun, you can support it on
[Ko-fi](https://ko-fi.com/bmabma).

## License

[GPL-3.0-or-later](LICENSE). Contributions — entries included — are
accepted under the same license.
