#!/usr/bin/env python3
"""Print the SHA-256 digest of a file (see tasks/sha256-file/spec.md)."""
import hashlib
import sys


def main():
    try:
        with open(sys.argv[1], "rb") as f:
            digest = hashlib.file_digest(f, "sha256").hexdigest()
    except OSError:
        sys.exit(1)
    print(digest)


if __name__ == "__main__":
    main()
