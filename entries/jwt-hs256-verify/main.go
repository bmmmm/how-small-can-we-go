// SPDX-License-Identifier: GPL-3.0-or-later

// jwt-hs256-verify: verify a JWS compact-serialization token signed
// with HMAC-SHA256 and print its payload. Implements
// tasks/jwt-hs256-verify/spec.md — three unpadded base64url segments, a
// JOSE header whose "alg" is exactly the string HS256, and a MAC taken
// over the still-encoded "header.payload" bytes. The payload stays
// opaque: no claim is read and no clock is consulted, so the verdict
// depends on the two arguments alone.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// b64 is RFC 7515's base64url: the URL alphabet, no padding, and Strict
// so that a final character carrying stray bits is rejected — one octet
// string must have exactly one spelling here.
var b64 = base64.RawURLEncoding.Strict()

// checkHeader enforces the JOSE header rules. "alg" must be the exact
// string HS256: accepting anything else is the alg-confusion hole —
// "none" turns the verifier into a rubber stamp, and a wrong-case or
// wrong-family name would pick an algorithm the caller never agreed to.
// "crit" means the token relies on an extension; RFC 7515 requires an
// implementation that does not understand it to fail, and this one
// understands none. Every other member is ignored, as the RFC asks.
func checkHeader(raw []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("header is not a JSON object: %v", err)
	}
	if obj == nil {
		return errors.New("header is JSON null, not an object")
	}
	if _, ok := obj["crit"]; ok {
		return errors.New(`header carries "crit": this verifier implements no extensions, so it must fail`)
	}
	var alg string
	if err := json.Unmarshal(obj["alg"], &alg); err != nil {
		return errors.New(`header "alg" is missing or not a string`)
	}
	if alg != "HS256" {
		return fmt.Errorf("header alg is %q, want HS256", alg)
	}
	return nil
}

// verify checks token against key and returns the payload octets. Every
// rejection is an error: the caller keeps stdout empty and exits
// nonzero, so a caller cannot mistake a failed verification for a
// payload.
func verify(token, key string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token has %d dot-separated segments, want 3", len(parts))
	}
	// An empty payload is a legal (zero-octet) payload; an empty header
	// or signature is not.
	if parts[0] == "" || parts[2] == "" {
		return nil, errors.New("header and signature segments must not be empty")
	}
	rawHeader, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header segment is not unpadded base64url: %v", err)
	}
	payload, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload segment is not unpadded base64url: %v", err)
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("signature segment is not unpadded base64url: %v", err)
	}
	if err := checkHeader(rawHeader); err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, []byte(key))
	// The MAC covers the ASCII of the two encoded segments joined by the
	// dot — the signing input, never the decoded bytes.
	mac.Write([]byte(parts[0] + "." + parts[1]))
	// hmac.Equal, not ==: the comparison must not leak how far it got.
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, errors.New("signature does not match")
	}
	return payload, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: prog <token> <secret>")
		os.Exit(2)
	}
	payload, err := verify(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// The payload is opaque octets: write them through unchanged, framed
	// by exactly one trailing newline.
	if _, err := os.Stdout.Write(append(payload, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
