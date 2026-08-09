#!/usr/bin/env python3
"""Re-emit a JSON file in canonical pretty form (see tasks/json-pretty/spec.md).

Numbers must survive byte-for-byte, so we never let json.loads convert them
to int/float: parse_int and parse_float are overridden to keep the raw
source token. Everything else (object/array/string/bool/null parsing,
duplicate-key last-wins, UTF-8 decoding incl. surrogate pairs) comes from
the standard decoder; only the rendering side is custom, since the output
layout, key sort and string-escaping rules are specific to this task.
"""
import json
import sys


class RawNumber:
    """A JSON number, kept as the exact substring that produced it."""

    def __init__(self, token):
        self.token = token


def reject_constant(name):
    # json.loads accepts NaN/Infinity/-Infinity by default; RFC 8259 does
    # not, so treat them as the parse error they should be.
    raise ValueError(f"{name} is not valid JSON")


NAMED_ESCAPES = {
    '"': '\\"',
    "\\": "\\\\",
    "\b": "\\b",
    "\f": "\\f",
    "\n": "\\n",
    "\r": "\\r",
    "\t": "\\t",
}


def render_string(s):
    out = ['"']
    for ch in s:
        named = NAMED_ESCAPES.get(ch)
        if named is not None:
            out.append(named)
            continue
        cp = ord(ch)
        if cp <= 0x1F or cp in (0x2028, 0x2029):
            out.append(f"\\u{cp:04x}")
        else:
            out.append(ch)
    out.append('"')
    return "".join(out)


def render(value, depth):
    if isinstance(value, RawNumber):
        return value.token
    if value is True:
        return "true"
    if value is False:
        return "false"
    if value is None:
        return "null"
    if isinstance(value, str):
        return render_string(value)
    if isinstance(value, list):
        return render_array(value, depth)
    if isinstance(value, dict):
        return render_object(value, depth)
    raise TypeError(f"unexpected decoded type: {type(value)!r}")


def render_array(items, depth):
    if not items:
        return "[]"
    pad = "  " * (depth + 1)
    lines = [pad + render(v, depth + 1) for v in items]
    return "[\n" + ",\n".join(lines) + "\n" + "  " * depth + "]"


def render_object(members, depth):
    if not members:
        return "{}"
    # Byte-wise key order: compare the keys' UTF-8 encodings, not the
    # code points or any locale collation.
    keys = sorted(members.keys(), key=lambda k: k.encode("utf-8"))
    pad = "  " * (depth + 1)
    lines = [
        pad + render_string(k) + ": " + render(members[k], depth + 1)
        for k in keys
    ]
    return "{\n" + ",\n".join(lines) + "\n" + "  " * depth + "}"


def main():
    try:
        path = sys.argv[1]
        with open(path, "r", encoding="utf-8") as f:
            text = f.read()
        # dict() keeps the last value for a repeated key, which matches
        # the spec's "last occurrence wins" rule for duplicate members.
        value = json.loads(
            text,
            parse_int=RawNumber,
            parse_float=RawNumber,
            parse_constant=reject_constant,
        )
    except (OSError, ValueError, IndexError):
        sys.exit(1)

    sys.stdout.buffer.write((render(value, 0) + "\n").encode("utf-8"))


if __name__ == "__main__":
    main()
