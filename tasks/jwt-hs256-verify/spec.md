# jwt-hs256-verify

Verify a JSON Web Token signed with HMAC-SHA256 and hand back its
payload — the gate in front of every authenticated request, and a job
every ecosystem delegates to a library (jsonwebtoken, PyJWT,
golang-jwt, …). Those libraries are also where the alg-confusion bugs
live: a verifier that asks the token which algorithm to verify it with
verifies nothing. Here it is a topic: implement it from what you ship.

## Interface

```
prog <token> <secret>
```

- Exactly two arguments: the compact-serialization token, and the raw
  HMAC key as the argv bytes — the secret is not hex, not base64, it is
  taken as given. Any other argv shape is a usage error: exit nonzero.
- On a valid, correctly signed token, print the decoded payload octets
  verbatim to stdout followed by a single `\n`, and exit 0. That
  newline is framing, not payload — a payload that itself ends in a
  newline produces two.
- On anything else — bad grammar, rejected header, signature mismatch —
  print nothing to stdout and exit nonzero. stderr is free for
  diagnostics.

## Token grammar

A token is exactly three base64url segments joined by `.`:
`header.payload.signature`. Two segments, four segments, or no dot at
all is invalid.

Segments are base64url as RFC 7515 defines it: the alphabet
`A-Za-z0-9-_`, **without padding**. A segment containing `=`, `+`, `/`,
whitespace, or any other byte is invalid — no leniency, not even for
padding a decoder would happily ignore. The last character must be
canonical too: the unused bits of the final quantum must be zero, so
`YWI` decodes to `ab` while `YWJ` — the same two octets with stray bits
— is invalid. One octet string, one spelling.

- The header segment must not be empty, and the signature segment must
  not be empty. `h.p.` — the classic `alg:none` shape — is therefore
  already invalid on grammar alone.
- The payload segment **may** be empty: a JWS payload is an arbitrary
  octet sequence and zero octets is one of them. `h..s`, signed over
  `h.`, is valid and prints just the framing newline.

## Header

The decoded header must be a JSON object. An array, a string, a
number, `null`, or anything that does not parse is invalid.

- `alg` must be present and must be exactly the string `HS256`.
  Anything else is invalid: `none`, `hs256`, `HS384`, a number, a
  missing `alg`. This is the entire point of the check. `alg:none` is
  the classic attack — JWS really does define an unsecured token, so a
  verifier that reads the algorithm out of the very header the attacker
  wrote accepts a token carrying no signature at all; a case-insensitive
  comparison or an unpinned algorithm family buys the attacker the same
  result by a different route. The algorithm is not negotiated by the
  token, it is fixed by this program.
- `crit` is fatal whenever it is present, whatever its value. It
  declares that the token depends on an extension the recipient has to
  understand, and RFC 7515 requires an implementation that does not
  understand it to fail. This one understands no extensions.
- `typ` is ignored if present. So is every other member — `kid`, `cty`,
  anything: RFC 7515 says unrecognized parameters are to be ignored
  unless `crit` lists them, and `crit` is already fatal here.
- Duplicate member names inside the header object are not diagnosed;
  the last occurrence wins. No case depends on it.

## Signature

The signing input is the ASCII of the first two segments **exactly as
they appear in the token** — still encoded — joined by a dot:
`header.payload`. Compute HMAC-SHA256 over those bytes with the secret
as the key and compare the result against the decoded third segment.
The comparison must be constant-time (Go: `hmac.Equal`, or the
equivalent elsewhere): verification is the one place where a
byte-at-a-time `==` hands out the answer.

Any mismatch exits nonzero — including a perfectly valid token checked
against the wrong secret.

## Out of scope

Deliberately not part of this task, and no case tests it:

- **Claim validation.** `exp`, `nbf`, `iat`, `iss`, `aud`, `sub` — none
  are inspected. A token whose `exp` lies far in the past verifies
  fine.
- **Reading the clock.** Verification is a pure function of the two
  arguments; a case that passes today passes in ten years.
- **Parsing the payload.** It need not be JSON at all. Whatever octets
  the payload segment decodes to are printed unchanged.
- Every algorithm other than HS256, JWE, the JSON serializations, and
  key lookup by `kid`.

## Expected outputs

Every fixture token was constructed by hand from this spec: pick the
header JSON and the payload octets, encode each with unpadded
base64url, join them with a dot, and sign that string with
HMAC-SHA256. The key is `arena-demo-key` everywhere except
`wrong-secret`. The finished tokens are literals in each case's `args`
file — any independent HMAC-SHA256 implementation reproduces them from
the ingredients below, and the reference entry is the executable
cross-check.

Passing cases; stdout is the payload followed by one newline:

| case                    | header JSON                                            | payload                                                      |
| ----------------------- | ------------------------------------------------------ | ------------------------------------------------------------ |
| `valid-hs256`           | `{"alg":"HS256","typ":"JWT"}`                          | `{"sub":"1234567890","name":"Ada Lovelace","iat":1516239022}` |
| `payload-url-alphabet`  | `{"alg":"HS256"}`                                      | `Is this base64url?` — not JSON, and its encoding ends in `_` |
| `header-unknown-params` | `{"alg":"HS256","typ":"JWT","kid":"key-1","cty":"JWT"}`| `{"ok":true}`                                                 |
| `claims-not-validated`  | `{"alg":"HS256"}`                                      | `{"iss":"arena","exp":1000000000,"nbf":4000000000}` — expired long ago, not yet valid |
| `empty-payload`         | `{"alg":"HS256"}`                                      | empty; stdout is the single framing newline                   |

Failing cases; stdout empty, exit nonzero. Where the table says
*correctly signed*, the signature is a genuine HMAC-SHA256 over that
token's own signing input with `arena-demo-key` — so only the rule
under test can reject it, and an implementation that skips that rule
would print the payload:

| case                       | built from                                                                | rejected because                                     |
| -------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------- |
| `alg-none`                 | header `{"alg":"none"}`, payload `{"sub":"mallory","admin":true}`, correctly signed | `alg` is not `HS256`                                 |
| `alg-none-empty-signature` | the same header and payload, third segment empty                          | empty signature segment                              |
| `alg-lowercase`            | header `{"alg":"hs256"}`, payload `{"ok":true}`, correctly signed         | the comparison is case-sensitive                     |
| `alg-hs384`                | header `{"alg":"HS384"}`, payload `{"ok":true}`, correctly signed         | wrong algorithm, however good the MAC                |
| `alg-missing`              | header `{"typ":"JWT"}`, payload `{"ok":true}`, correctly signed           | no `alg` member                                      |
| `alg-not-a-string`         | header `{"alg":256}`, payload `{"ok":true}`, correctly signed             | `alg` is not a string                                |
| `header-not-object`        | header `["HS256"]`, payload `{"ok":true}`, correctly signed               | the header is not a JSON object                      |
| `crit-header`              | header `{"alg":"HS256","crit":["exp"],"exp":1363284000}`, correctly signed | `crit` is present                                    |
| `tampered-payload`         | `valid-hs256` with the name changed to `Mallory Jones`, original signature kept | signature mismatch                              |
| `wrong-secret`             | the `valid-hs256` token, secret `arena-demo-KEY`                          | signature mismatch                                   |
| `padded-header`            | header `{"alg":"HS256","kid":"k1"}` encoded **with** padding (`…In0=`), signed over that padded input | `=` is not allowed in a segment    |
| `std-alphabet-payload`     | payload `Is this base64url?` encoded with the **standard** alphabet (`…cmw/`), signed over that input | `/` is not in the base64url alphabet |
| `noncanonical-base64`      | payload segment `YWJ`, correctly signed                                   | non-canonical final quantum — `ab` is spelled `YWI`  |
| `two-segments`             | `valid-hs256` without its signature segment                               | three segments required                              |
| `four-segments`            | `valid-hs256` plus a fourth segment `.ZXh0cmE`                            | three segments required                              |
| `empty-header`             | `.<payload>.<signature>`, correctly signed over `.<payload>`              | empty header segment                                 |
| `one-argument`             | the `valid-hs256` token, no secret                                        | usage error                                          |
| `no-arguments`             | no argv at all                                                            | usage error                                          |
