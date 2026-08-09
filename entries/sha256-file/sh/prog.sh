#!/bin/sh
# Print the SHA-256 digest of the file named by $1, followed by a newline.
# busybox sha256sum prints "<hash>  <filename>\n"; keep only the hash.
set -eu

sum=$(sha256sum -- "$1") || exit 1
printf '%s\n' "${sum%% *}"
