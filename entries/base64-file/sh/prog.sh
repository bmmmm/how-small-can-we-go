#!/bin/sh
# Print the Base64 (RFC 4648) encoding of the file named by $1, unwrapped,
# followed by a newline. busybox base64 does not append a trailing
# newline, so add it explicitly.
set -eu

out=$(base64 -w 0 -- "$1") || exit 1
printf '%s\n' "$out"
