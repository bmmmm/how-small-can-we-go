# semver-range-check

Decide whether a semantic version satisfies a version range — the core
of every dependency resolver, and a job almost every ecosystem delegates
to a library (node-semver, python-semver, Masterminds/semver, …). Here
it is a topic: implement it from what you ship.

## Interface

```
prog <range> <version>
```

- Both arguments are required. Any other argv shape is a usage error:
  exit nonzero.
- If the version satisfies the range, print exactly `yes\n` to stdout;
  otherwise print exactly `no\n`. Exit 0 in both cases.
- If the range or the version does not conform to the grammar below,
  print nothing to stdout and exit nonzero. stderr is free for
  diagnostics.

## Versions

`MAJOR.MINOR.PATCH` — exactly three dot-separated components, each a
base-10 integer without leading zeros (`0` itself is fine). Nothing
else: no `v` prefix, no pre-release (`-alpha`), no build metadata
(`+build`). Those are out of scope for this task and must be rejected
as invalid.

Precedence: compare major, then minor, then patch, numerically.

## Ranges

```
range      := clause ( "||" clause )*
clause     := comparator ( whitespace comparator )*
comparator := op? version
op         := "=" | "<" | "<=" | ">" | ">=" | "^" | "~"
```

- A clause is the AND of its comparators; the range is the OR of its
  clauses. AND binds tighter than OR.
- Whitespace (spaces/tabs) around `||` and between comparators is
  allowed and insignificant. An empty clause (`"^1.0.0 ||"`) is
  invalid.
- No operator means `=` (exact match).
- `~X.Y.Z` allows patch-level changes: `>=X.Y.Z <X.(Y+1).0`.
- `^X.Y.Z` allows changes up to the next breaking version:
  - `X > 0`: `>=X.Y.Z <(X+1).0.0`
  - `X = 0, Y > 0`: `>=0.Y.Z <0.(Y+1).0`
  - `X = 0, Y = 0`: exactly `Z` (`>=0.0.Z <0.0.(Z+1)`)
- All versions inside ranges are full `X.Y.Z` — partial forms (`1.2`,
  `1.x`, `*`) and hyphen ranges (`1.2.3 - 2.0.0`) are out of scope and
  invalid.

## Expected outputs

Derived by hand from the grammar above; the semantics of `^` and `~`
follow node-semver's documented behavior for full versions. The
reference entry is the executable cross-check.
