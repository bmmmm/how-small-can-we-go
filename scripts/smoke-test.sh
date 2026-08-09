#!/usr/bin/env bash
# Real-world smoke test for the trust-score arena. Unlike the Go unit
# tests (which fake docker and score in-process), this drives the built
# `arena` binary the way a contributor and CI do, and exercises the one
# thing the unit tests cannot: the real no-network docker sandbox.
#
# It checks three things:
#   A. Gaming resistance — build actual evasion entries and prove the
#      scorer still catches the hazard or the third-party bytes. This is
#      the guarantee the rework's hardening rests on.
#   B. Real conformance — run the reference entries through the actual
#      sandbox (pinned image, --network none). If no docker daemon is
#      reachable it asserts the daemon-down path reports INFRA (exit 3),
#      never a false conformance failure, then falls back to a host run
#      and says loudly that the sandbox path was skipped.
#   C. Board render — the generated board.json and index.html carry the
#      new trust-score shape, not the retired surface metric.
#
# Exit 0 = every assertion held. Nonzero = at least one failed; the
# summary names which. Run from anywhere inside the repo.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

docker_up=false
if docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
  docker_up=true
fi

# macOS + Colima/Docker Desktop only bind-mount $HOME into the VM, but
# arena's temp build dirs default to $TMPDIR — a sandboxed run then fails
# with a misleading "can't open <file>". Steer TMPDIR under $HOME so the
# real sandbox path works (AGENTS.md documents this). Only when a daemon
# is actually up: a host-only run must not depend on that path existing.
if $docker_up && [ "$(uname)" = "Darwin" ]; then
  export TMPDIR="${HOME}/.cache/arena-tmp"
  mkdir -p "$TMPDIR"
fi

PASS=0
FAIL=0
FAILED_NAMES=()

ok() {
  PASS=$((PASS + 1))
  printf '  \033[32mPASS\033[0m  %s\n' "$1"
}
bad() {
  FAIL=$((FAIL + 1))
  FAILED_NAMES+=("$1")
  printf '  \033[31mFAIL\033[0m  %s\n' "$1"
  [ -n "${2:-}" ] && printf '        %s\n' "$2"
}

# mktemp with no path honors $TMPDIR itself (and the Darwin override
# above) — no hardcoded temp path.
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

ARENA="$WORK/arena"

echo "== build"
( cd tool && go build -o "$ARENA" . )
ok "arena builds"

# --- helpers --------------------------------------------------------------

# make_entry <dir> <language> <run> — write a probe entry.json.
make_entry() {
  local dir="$1" lang="$2" run="$3"
  mkdir -p "$dir"
  printf '{"task": "probe", "language": "%s", "authors": ["smoke"], "run": "%s"}\n' \
    "$lang" "$run" > "$dir/entry.json"
}

# expect_score <name> <dir> <want-vendored> <want-hazards>
# Asserts `arena score` prints exactly "<vendored> <hazards>" on stdout.
expect_score() {
  local name="$1" dir="$2" wv="$3" wh="$4" got
  if ! got=$("$ARENA" score "$dir" 2>/dev/null); then
    bad "$name" "arena score exited nonzero"
    return
  fi
  if [ "$got" = "$wv $wh" ]; then
    ok "$name (score $got)"
  else
    bad "$name" "want '$wv $wh', got '$got'"
  fi
}

# expect_hazard_note <name> <dir> <needle> — assert the stderr diagnostics
# (hazard lines + raw-scan notes) contain <needle>.
expect_hazard_note() {
  local name="$1" dir="$2" needle="$3" err
  err=$("$ARENA" score "$dir" 2>&1 >/dev/null || true)
  if printf '%s' "$err" | grep -qF "$needle"; then
    ok "$name"
  else
    bad "$name" "diagnostics did not mention: $needle"
  fi
}

echo
echo "== A. gaming resistance (scorer, no docker needed)"

# The reference champions must sit at the perfect score.
expect_score "clean champion: semver-range-check = 0 0" entries/semver-range-check 0 0
expect_score "clean champion: dotenv-parse = 0 0"       entries/dotenv-parse 0 0

# Go: aliasing the unsafe import must not hide it (matched as import path).
d="$WORK/go-alias"; make_entry "$d" go "./prog"
printf 'package main\nimport u "unsafe"\nvar _ = u.Sizeof(0)\nfunc main(){}\n' > "$d/main.go"
expect_score "go: aliased unsafe import still scores 1 hazard" "$d" 0 1

# Python: a program shipped under a name without the .py extension (run
# as `python3 prog`) is still scanned — and an aliased eval still counts.
d="$WORK/py-noext"; make_entry "$d" python "python3 prog"
printf 'f = eval\nf(open("x").read())\n' > "$d/prog"
expect_score "python: extension-less file + eval alias scores 1 hazard" "$d" 0 1

# C: a backslash-newline line splice must not break a hazard name apart.
d="$WORK/c-splice"; make_entry "$d" c "./prog"
printf 'int main(void){ sys\\\ntem("id"); return 0; }\n' > "$d/main.c"
expect_score "c: line-spliced system() still scores 1 hazard" "$d" 0 1

# Vendored bytes count under an aliased directory name, not just vendor/.
d="$WORK/vendor-alias"; make_entry "$d" go "./prog"
printf 'package main\nfunc main(){}\n' > "$d/main.go"
mkdir -p "$d/third_party"; printf '1234567890' > "$d/third_party/lib.go"
expect_score "vendor alias: third_party/ bytes are counted" "$d" 10 0

# The manifest's run command is code too: python3 -c exec(...) must score.
d="$WORK/manifest-payload"; make_entry "$d" python 'python3 -c exec(open(logic).read())'
printf 'print(1)\n' > "$d/logic"
expect_hazard_note "manifest: exec in run command is scanned" "$d" "entry.json(run)"

# A guarded (f-string) line that opens a multiline string forces a raw
# scan instead of silently swallowing a hazard in the "comment".
d="$WORK/guard-desync"; make_entry "$d" python "python3 main.py"
printf 'a = f"" + """\n# subprocess mention\n""" + f""\n' > "$d/main.py"
expect_hazard_note "python: guarded-line desync falls back to raw scan" "$d" "subprocess"

echo
echo "== B. real conformance (the sandbox path unit tests fake)"

if $docker_up; then
  for e in entries/semver-range-check entries/dotenv-parse; do
    if "$ARENA" check "$e" >/dev/null 2>&1; then
      ok "sandboxed check PASS: $e"
    else
      bad "sandboxed check: $e" "arena check failed in the real sandbox — run '$ARENA check $e' to see why"
    fi
  done
else
  printf '  \033[33mNOTE\033[0m  no docker daemon reachable — sandbox path skipped\n'
  # The daemon-down fix: a sandboxed check with no daemon must report
  # INFRA (exit 3), never a conformance failure (exit 1) that would
  # auto-close a healthy PR.
  rc=0; "$ARENA" check entries/semver-range-check >/dev/null 2>&1 || rc=$?
  if [ "$rc" -eq 3 ]; then
    ok "daemon-down reports INFRA (exit 3), not a false conformance fail"
  else
    bad "daemon-down classification" "want exit 3 (infra), got $rc"
  fi
  # Fall back to host mode so conformance is still verified.
  for e in entries/semver-range-check entries/dotenv-parse; do
    if "$ARENA" check "$e" --no-sandbox >/dev/null 2>&1; then
      ok "host-mode check PASS (sandbox skipped): $e"
    else
      bad "host-mode check: $e" "arena check --no-sandbox failed — run it to see why"
    fi
  done
fi

echo
echo "== C. board render carries the trust-score shape"
"$ARENA" board --no-sandbox --out "$WORK/board" >/dev/null 2>&1
bj="$WORK/board/board.json"
bh="$WORK/board/index.html"
if grep -q '"vendoredBytes"' "$bj" && grep -q '"hazardCount"' "$bj"; then
  ok "board.json exposes vendoredBytes + hazardCount"
else
  bad "board.json shape" "missing the trust-score fields"
fi
if grep -qi 'third-party bytes' "$bh" && grep -qi 'hazards' "$bh"; then
  ok "index.html shows the third-party-bytes / hazards columns"
else
  bad "index.html shape" "missing the new columns"
fi
if grep -qiE 'audit unit|surface' "$bj" "$bh"; then
  bad "no retired metric leaked" "board still mentions the audit-surface metric"
else
  ok "no retired 'audit surface' wording in the board"
fi

echo
echo "== summary: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  printf 'failed: %s\n' "${FAILED_NAMES[*]}"
  exit 1
fi
echo "all good."
