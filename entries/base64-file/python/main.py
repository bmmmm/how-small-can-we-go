#!/usr/bin/env python3
"""Print the Base64 encoding of a file (see tasks/base64-file/spec.md)."""
import base64
import sys


def main():
    try:
        with open(sys.argv[1], "rb") as f:
            data = f.read()
    except OSError:
        sys.exit(1)
    sys.stdout.buffer.write(base64.b64encode(data) + b"\n")


if __name__ == "__main__":
    main()
