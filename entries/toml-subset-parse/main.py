# SPDX-License-Identifier: GPL-3.0-or-later

# toml-subset-parse: read a pinned TOML subset, print "dotted.key=value"
# per entry. Implements the dialect of tasks/toml-subset-parse/spec.md -
# all-or-nothing: nothing reaches stdout unless the whole file parses.
# Whitespace in the spec means ASCII space and tab ONLY, so every trim
# names those two characters. The one CRLF artifact - a single trailing
# \r per line - is stripped before anything else. Comments are
# quote-aware: a '#' only opens a comment when it is not inside an open
# double-quoted string, so it works both on its own line and trailing
# after a header or a value.
#
# This subset is parsed by hand on purpose: the topic exists because
# real code reaches for a TOML library, and a full TOML reader accepts
# many shapes (arrays, floats, dates, inline tables, dotted assignment
# keys) that this pinned dialect rejects.
import string
import sys

WS = " \t"
KEY_CHARS = set(string.ascii_letters + string.digits + "_-")

STRING_ESCAPES = {"n": "\n", "t": "\t", '"': '"', "\\": "\\"}

# Magnitude bound from spec.md: -2**63 < n < 2**63.
INT_MAGNITUDE_LIMIT = 2 ** 63


def fail(lineno, msg):
    print("line " + str(lineno) + ": " + msg, file=sys.stderr)
    sys.exit(1)


def is_bare_key(s):
    return len(s) > 0 and all(c in KEY_CHARS for c in s)


def dotted(path):
    return ".".join(path)


def strip_comment(line):
    # Quote-aware scan: '#' outside an open double-quoted string starts a
    # comment running to the end of the line. A backslash inside a string
    # always consumes the next character too, so an escaped quote never
    # closes the string early - this only needs to track open/closed, not
    # whether the escape itself is one the value parser will later accept.
    in_str = False
    i = 0
    n = len(line)
    while i < n:
        c = line[i]
        if in_str:
            if c == "\\":
                i += 2
                continue
            if c == '"':
                in_str = False
            i += 1
            continue
        if c == '"':
            in_str = True
            i += 1
            continue
        if c == "#":
            return line[:i]
        i += 1
    return line


def parse_string(token, lineno):
    # token starts with the opening quote; the caller already trimmed
    # trailing whitespace, so the closing quote must be the final char.
    out = []
    i = 1
    n = len(token)
    while i < n:
        c = token[i]
        if c == '"':
            if i != n - 1:
                fail(lineno, "content after closing quote")
            return "".join(out)
        if c == "\\":
            i += 1
            esc = STRING_ESCAPES.get(token[i]) if i < n else None
            if esc is None:
                fail(lineno, "unsupported escape in string value")
            out.append(esc)
            i += 1
            continue
        if ord(c) < 0x20 or ord(c) == 0x7F:
            fail(lineno, "control character in string value")
        out.append(c)
        i += 1
    fail(lineno, "unclosed string")


def parse_int(token, lineno):
    body = token[1:] if token[:1] == "-" else token
    if body == "" or not body.isdigit():
        fail(lineno, "invalid value " + repr(token))
    if len(body) > 1 and body[0] == "0":
        fail(lineno, "leading zero in integer " + repr(token))
    n = int(token)
    if n <= -INT_MAGNITUDE_LIMIT or n >= INT_MAGNITUDE_LIMIT:
        fail(lineno, "integer out of range " + repr(token))
    return n


def parse_value(token, lineno):
    if token[:1] == '"':
        return parse_string(token, lineno)
    if token == "true":
        return "true"
    if token == "false":
        return "false"
    if token[:1] == "-" or token[:1].isdigit():
        return str(parse_int(token, lineno))
    fail(lineno, "invalid value " + repr(token))


def parse_header(body, lineno):
    # body is the text between '[' and ']'. Whitespace is allowed only
    # at the two outer ends - stripping it off the joined body first,
    # then splitting on '.', means any whitespace left touching a
    # segment (next to a dot) survives into that segment and fails the
    # bare-key check below, which is exactly the point being pinned.
    trimmed = body.strip(WS)
    if trimmed == "":
        fail(lineno, "empty table header")
    path = []
    for seg in trimmed.split("."):
        if not is_bare_key(seg):
            fail(lineno, "invalid table header segment " + repr(seg))
        path.append(seg)
    return tuple(path)


def declare_table(path, entries, lineno):
    # Every proper prefix of path becomes a table if it is not one
    # already - this is how "[a.b.c]" implicitly creates "a" and "a.b".
    for i in range(1, len(path)):
        prefix = path[:i]
        kind = entries.get(prefix)
        if kind == "value":
            fail(lineno, "table header conflicts with an existing key " + repr(dotted(prefix)))
        if kind is None:
            entries[prefix] = "implicit"
    kind = entries.get(path)
    if kind == "value":
        fail(lineno, "table header conflicts with an existing key " + repr(dotted(path)))
    if kind == "explicit":
        fail(lineno, "table redefined " + repr(dotted(path)))
    # kind is None (brand new) or "implicit" (only a parent so far,
    # never given its own header) - both become explicit now.
    entries[path] = "explicit"


def parse(text):
    order = []  # list of (dotted_key, value) in definition order
    entries = {}  # tuple(path) -> "value" | "implicit" | "explicit"
    table = ()
    for lineno, raw in enumerate(text.split("\n"), 1):
        if raw.endswith("\r"):
            raw = raw[:-1]  # exactly one: the CRLF artifact
        content = strip_comment(raw).strip(WS)
        if not content:
            continue
        if content[0] == "[" and content[-1] == "]" and len(content) >= 2:
            path = parse_header(content[1:-1], lineno)
            declare_table(path, entries, lineno)
            table = path
            continue
        key, eq, rest = content.partition("=")
        if not eq:
            fail(lineno, "expected key = value or a [table] header")
        key = key.strip(WS)
        if not is_bare_key(key):
            fail(lineno, "invalid key " + repr(key))
        value_token = rest.strip(WS)
        if value_token == "":
            fail(lineno, "missing value")
        value = parse_value(value_token, lineno)
        full = table + (key,)
        if full in entries:
            fail(lineno, "duplicate key " + repr(dotted(full)))
        entries[full] = "value"
        order.append((dotted(full), value))
    return order


def main():
    if len(sys.argv) != 2:
        print("usage: prog <file>", file=sys.stderr)
        sys.exit(2)
    try:
        # newline="" keeps bare \r out of universal-newline translation:
        # only \n separates lines, per the spec.
        with open(sys.argv[1], encoding="utf-8", newline="") as f:
            text = f.read()
    except (OSError, UnicodeDecodeError) as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    order = parse(text)
    sys.stdout.write("".join(k + "=" + v + "\n" for k, v in order))


main()
