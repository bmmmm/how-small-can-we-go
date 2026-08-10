// SPDX-License-Identifier: GPL-3.0-or-later

// base58check-decode: decode <string> as Base58Check, print the
// payload (version byte plus data, without the 4 checksum bytes) as
// lowercase hex. Implements tasks/base58check-decode/spec.md.
//
// Two things the naive approach gets wrong:
//
//   - The decoded magnitude routinely exceeds 64 bits (a 25-byte
//     address payload is a ~200-bit number), so the digit
//     accumulation needs math/big, not a machine integer.
//   - big.Int.Bytes() strips leading zero bytes from the numeric
//     value, but the spec's leading-'1' rule is about *decoded byte
//     count*, not magnitude: each leading '1' must reappear as one
//     leading 0x00 byte even though it contributes nothing to the
//     number. So leading '1's are counted separately and the zero
//     bytes are prepended by hand.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
)

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// decodeBase58 turns s into the raw bytes it encodes, or an error if
// s contains a character outside the alphabet.
func decodeBase58(s string) ([]byte, error) {
	leading := 0
	for leading < len(s) && s[leading] == '1' {
		leading++
	}

	base := big.NewInt(58)
	n := new(big.Int)
	for i := 0; i < len(s); i++ {
		digit := strings.IndexByte(alphabet, s[i])
		if digit < 0 {
			return nil, fmt.Errorf("byte %d (%q) is not in the base58 alphabet", i, s[i])
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(digit)))
	}

	body := n.Bytes() // big-endian, no leading zero bytes of its own
	full := make([]byte, leading+len(body))
	copy(full[leading:], body)
	return full, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: prog <string>")
		os.Exit(2)
	}

	full, err := decodeBase58(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(full) < 4 {
		fmt.Fprintln(os.Stderr, "decoded length under 4 bytes: no room for a checksum")
		os.Exit(1)
	}

	payload := full[:len(full)-4]
	checksum := full[len(full)-4:]

	// Base58Check's checksum is the first 4 bytes of the double
	// SHA-256 of the payload alone.
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	if !bytes.Equal(second[:4], checksum) {
		fmt.Fprintln(os.Stderr, "checksum mismatch")
		os.Exit(1)
	}

	fmt.Println(hex.EncodeToString(payload))
}
