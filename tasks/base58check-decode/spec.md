# base58check-decode

Decode and verify a Base58Check string — the encoding behind every
Bitcoin address and WIF private key. Every wallet, block explorer, and
address validator imports a base58 library for exactly this: turn the
human-typed string back into raw bytes and check the embedded
checksum before trusting it. Here it is a topic: implement it from
what you ship.

## Interface

```
prog <string>
```

- Exactly one argument. Any other argv shape is a usage error: exit
  nonzero.
- Valid input with a correct checksum: print the payload — the
  version byte plus data, **without** the 4 checksum bytes — as
  lowercase hex, followed by a newline, and exit 0.
- Anything else — a character outside the alphabet, a decoded length
  under 4 bytes, a checksum mismatch, or an empty string — prints
  nothing to stdout and exits nonzero. stderr is free for
  diagnostics.

## Alphabet

The Bitcoin Base58 alphabet, 58 characters, in order:

```
123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
```

`0` (zero), `O` (capital o), `I` (capital i), and `l` (lowercase L)
are excluded — they are visually confusable with other characters in
common fonts. Any input byte not in this list is invalid.

## Decoding rules

- The string is a big-endian base-58 number: reading left to right,
  each character contributes `value = value * 58 + digit`, where
  `digit` is the character's 0-based position in the alphabet above.
- Each **leading** `1` in the input (a run of `1` characters starting
  at position 0) maps to one leading `0x00` byte of the decoded
  bytes, prepended *in addition to* the big-58 number's own bytes —
  it is not part of the numeric value, the same way a leading `0` in
  decimal notation isn't part of the number's magnitude. A `1` that
  isn't part of that leading run is just digit 0 and folds into the
  number normally.
- The checksum is the first 4 bytes of `SHA-256(SHA-256(payload))`,
  appended to the payload before it is Base58-encoded. Decoding
  therefore splits the decoded bytes into `payload = all but the last
  4 bytes` and `checksum = the last 4 bytes`, and the checksum must
  equal `SHA-256(SHA-256(payload))[:4]`.
- Decoded length under 4 bytes means there is no room for a checksum
  at all — invalid, regardless of what the bytes are.
- An empty payload — the decoded bytes are exactly 4 bytes long, all
  of them the checksum — **is** checksum-valid whenever those 4 bytes
  equal `SHA-256(SHA-256(""))[:4]`. That is a real, reachable state
  (Base58Check does not forbid an empty payload), so it is pinned
  valid here: such input prints an empty line and exits 0.
- The empty string decodes to zero bytes (no leading `1`s, no digits)
  — under the 4-byte floor, so invalid.

## Expected outputs

Two published vectors anchor the derivation; every other case is
worked out by hand from the rules above and cross-checked against the
reference entry (the executable cross-check).

- The Bitcoin genesis block's coinbase-reward address,
  `1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa`, decodes to payload
  `0062e907b15cbf27d5425399ebf6f0fb50ebb88f18` — version byte `0x00`
  (P2PKH, mainnet) followed by the well-known genesis output's
  HASH160. This address/payload pairing is reproduced across
  essentially every Base58Check reference, including the Bitcoin Wiki's
  "Base58Check encoding" page.
- The WIF-encoded private key
  `5JuW2AMDYu4xVwRG9DZW18VbzQrGcd5RCgb99sS6ehJsNQXu5b9` is one of
  Bitcoin Core's own conformance vectors
  (`src/test/data/key_io_valid.json` in
  https://github.com/bitcoin/bitcoin, the entry with `"chain": "main"`,
  `"isPrivkey": true`, `"isCompressed": false`). That file lists the
  raw 32-byte private key as
  `8f8943bf956de595665c38ffff23827e17c10cdc1c27a028caae6c9810626198`;
  prefixed with the mainnet WIF version byte `0x80` (uncompressed,
  no compression-flag byte) that is exactly this task's payload:
  `808f8943bf956de595665c38ffff23827e17c10cdc1c27a028caae6c9810626198`.

The remaining cases are constructed, not looked up, and verified by
running the reference entry:

- A payload with **two** leading `0x00` bytes (version `0x00` plus a
  HASH160 that itself starts with `0x00`) checks that the leading-`1`
  count is tracked as a count, not a single special case for the
  version byte. Payload
  `00000102030405060708090a0b0c0d0e0f10111213` encodes to
  `112D2adLM3UKy4Z4giRbReR6gjWuvHUqB` — two leading `1`s for two
  leading zero bytes.
- The empty-payload corner: `SHA-256(SHA-256(""))` starts with
  `5df6e0e2`, whose Base58 encoding is `3QJmnh` — 4 bytes, no leading
  zero byte among them, so no leading `1`. Decoding it must yield an
  empty payload and exit 0.
- The checksum-mismatch case flips the last character of the genesis
  address (`...DivfNa` to `...DivfNb`, both valid alphabet
  characters) and confirms by direct computation that the flipped
  string does **not** coincidentally decode-and-verify — it still
  parses as valid Base58, but the trailing 4 bytes no longer match
  `SHA-256(SHA-256(payload))[:4]`.
