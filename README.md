# how small can we go?

**The attack-surface golf arena.** Pick a task, write the smallest honest
program that passes its test suite, dethrone the reigning champion. Every
byte you ship is a byte someone has to audit — so that is exactly what we
count.

**Board:** https://bmmmm.github.io/how-small-can-we-go/

## The game in 60 seconds

1. Pick a task in [`tasks/`](tasks/) and a language from
   [`languages.json`](languages.json). That pair is a *niche*; the entry
   sitting in `entries/<task>/<language>/` is its *champion*.
2. Your entry must pass every test case of the task, build and run with
   **no network**, ship **only text files**, and have a smaller **audit
   surface** than the champion — the non-whitespace bytes of everything
   in your entry directory, vendored code included.
3. Open a PR that replaces the niche directory. CI measures; it never
   believes. Beat the number or the PR closes.

An entry is deliberately small ceremony: one 6-line `entry.json`
manifest (excluded from measurement) plus your source files — that's
the whole format, spelled out in [SPEC.md](SPEC.md).

An empty niche (no entry yet for that task × language)? The first passing
entry takes it.

## Why smaller? (the honest version)

Small is **not** automatically secure — ten lines can hold a command
injection. What smallness buys is *auditability*: an entry you can read
in one sitting, that builds offline from nothing but its own directory
and a pinned base image. No transitive dependency graph, no post-install
scripts, no "trust me" blobs. Supply-chain attacks live in the code you
didn't read; this arena minimizes the code there is to read.

This is a game about audit surface, not a security certification.

## What keeps the slop out

- **Measured, never claimed.** CI builds in a container with networking
  disabled and runs the task's full test suite. Numbers in a PR body are
  decoration; the gate only trusts its own measurement.
- **Improve or be closed.** A challenger must be *strictly* smaller than
  the champion it replaces. An entry that beats nothing is closed
  automatically — zero maintainer attention spent.
- **Text only.** Invalid UTF-8, NUL bytes, or symlinks anywhere in an
  entry fail the check. Nothing unauditable gets in.
- **Tests are contributions too.** A new test case is accepted when it
  breaks a current entry or closes a documented gap in a task's spec.
  The suite gets sharper with every round, and the weekly re-run demotes
  entries that no longer pass.

AI-written entries are welcome — see [AGENTS.md](AGENTS.md) for the
machine-readable contract. AI slop is not, and the difference is
measured, not vibes.

## Run it locally

```sh
cd tool && go build -o ../arena . && cd ..
./arena check entries/sha256-file/go --no-sandbox   # host run, for iteration
./arena check entries/sha256-file/go                # the real thing (needs docker)
./arena surface entries/sha256-file/go              # just the number
./arena board --no-sandbox                          # render docs/ locally
```

No docker? Push your branch — the entry-check workflow runs the real
sandboxed measurement on every branch push and on
`gh workflow run entry-check --ref <branch>`, so CI does the containers
for you. Local `--no-sandbox` plus Go is all you need.

The full rules live in [SPEC.md](SPEC.md).

## Roadmap

No timelines, no "coming soon" — this is what's planned, not what's
built:

- **More tasks, more languages.** The board grows one niche at a time.
  Task and language proposals are welcome via issues.
- **Auto-merge for green entries.** A least-privilege bot merges a
  passing challenger PR on its own — beaten champion out, smaller
  challenger in, no human in the loop.
- **The chain (v2) — north star.** Once the solo game works: entries
  that may depend only on the pinned base images *and other entries of
  this arena*. Compose bigger programs from small audited ones, watch
  the total surface of the chain — climb until the chain gets too
  heavy, then golf the links. That flips the usual dependency story:
  instead of trusting an ecosystem, you trust a board where every link
  was measured and fought over.

## Support

If this arena is your kind of fun, you can support it on
[Ko-fi](https://ko-fi.com/bmabma).

## License

[GPL-3.0-or-later](LICENSE). Contributions — entries included — are
accepted under the same license.
