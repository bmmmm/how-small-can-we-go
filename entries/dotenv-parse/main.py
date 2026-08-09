# SPDX-License-Identifier: GPL-3.0-or-later

# dotenv-parse: read a dotenv file, print KEY=VALUE per variable.
# Implements the dialect of tasks/dotenv-parse/spec.md — all-or-nothing:
# nothing reaches stdout unless the whole file parses.
import string
import sys

KEY_START = set(string.ascii_letters + "_")
KEY_CHARS = KEY_START | set(string.digits)

DOUBLE_ESCAPES = {"n": "\n", "t": "\t", '"': '"', "\\": "\\"}


def fail(lineno, msg):
    print(f"line {lineno}: {msg}", file=sys.stderr)
    sys.exit(1)


def parse_quoted(rest, lineno):
    quote = rest[0]
    out = []
    i = 1
    while i < len(rest):
        c = rest[i]
        if c == quote:
            if rest[i + 1:].strip():
                fail(lineno, "only whitespace may follow a closing quote")
            return "".join(out)
        if quote == '"' and c == "\\":
            i += 1
            esc = DOUBLE_ESCAPES.get(rest[i]) if i < len(rest) else None
            if esc is None:
                fail(lineno, "unsupported escape in double-quoted value")
            out.append(esc)
            i += 1
            continue
        out.append(c)
        i += 1
    fail(lineno, "unclosed quote")


def parse(text):
    env = {}
    for lineno, raw in enumerate(text.split("\n"), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export") and line[6:7] in (" ", "\t"):
            line = line[7:].strip()
        key, eq, rest = line.partition("=")
        key = key.strip()
        if not eq:
            fail(lineno, "expected KEY=VALUE")
        if not key or key[0] not in KEY_START or any(c not in KEY_CHARS for c in key):
            fail(lineno, f"invalid key {key!r}")
        rest = rest.strip()
        if rest[:1] in ('"', "'"):
            value = parse_quoted(rest, lineno)
        else:
            value = rest
        env[key] = value  # dict order = first occurrence, last value wins
    return env


def main():
    if len(sys.argv) != 2:
        print("usage: prog <file>", file=sys.stderr)
        sys.exit(2)
    try:
        with open(sys.argv[1], encoding="utf-8") as f:
            text = f.read()
    except OSError as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    env = parse(text)
    sys.stdout.write("".join(f"{k}={v}\n" for k, v in env.items()))


main()
