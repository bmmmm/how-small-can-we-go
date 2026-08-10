# url-parse

Split an absolute URI into its components — the first step of almost
every HTTP client, proxy, and router, and a job every ecosystem
delegates to a library (`net/url`, Python's `urllib.parse`, Node's
`url` module and the WHATWG `URL` class, …). Here it is a topic:
implement it from what you ship.

## Interface

```
prog <url>
```

- Exactly one argument. Any other argv shape is a usage error: exit
  nonzero.
- If the argument parses as a valid URL under the grammar below, print
  its present components, one per line, in this fixed order: `scheme`,
  `userinfo`, `host`, `port`, `path`, `query`, `fragment`, each as a
  `name=value` line. A component that is absent (see *Presence* below)
  is omitted entirely — no blank placeholder line for it. Exit 0.
- If the argument does not conform to the grammar, print nothing to
  stdout and exit nonzero. stderr is free for diagnostics.
- No whitespace is trimmed anywhere. ASCII space and tab are not
  members of any RFC 3986 component grammar below, so a URL containing
  one — leading, trailing, or internal — is invalid, not silently
  cleaned up.

## Scope

RFC 3986 §3 defines a URI as `scheme ":" hier-part [ "?" query ] [ "#"
fragment ]`, with `hier-part` having four possible shapes depending on
whether an authority is present. This task pins `hier-part` to exactly
one of those shapes — the authority form — giving this dialect:

```
url          = scheme "://" authority path-abempty [ "?" query ] [ "#" fragment ]
scheme       = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )
authority    = [ userinfo "@" ] host [ ":" port ]
userinfo     = *( unreserved / pct-encoded / sub-delims / ":" )
host         = reg-name                     ; see Host, below
port         = *DIGIT
path-abempty = *( "/" segment )
segment      = *pchar
pchar        = unreserved / pct-encoded / sub-delims / ":" / "@"
query        = *( pchar / "/" / "?" )
fragment     = *( pchar / "/" / "?" )
reg-name     = 1*( unreserved / pct-encoded / sub-delims )
pct-encoded  = "%" HEXDIG HEXDIG
unreserved   = ALPHA / DIGIT / "-" / "." / "_" / "~"
sub-delims   = "!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" / "," / ";" / "="
```

Only the `scheme "://" authority path-abempty ...` shape is in scope.
The other three `hier-part` shapes (`path-absolute`, `path-rootless`,
`path-empty` — used by authority-less schemes like `mailto:` or
`urn:`) are out of scope and rejected as invalid: this is a
"url-parse" for network-style URLs (`http://`, `ftp://`, `ws://`, …),
not a general RFC 3986 reference implementation. No IDNA, no punycode
processing, no WHATWG rules, no relative references — a bare path or a
scheme-less `//host/path` is invalid, since a `scheme` is mandatory.

### Host

`host` is validated against `reg-name`'s character class only.
`IPv4address`'s stricter grammar (four dot-separated `dec-octet`s,
each numerically 0–255) is not applied: every string `IPv4address`
would accept is already a valid `reg-name`, so `999.999.999.999` is
accepted as a syntactically fine host with no numeric range checked —
consistent with "no WHATWG rules" above. `reg-name` is pinned
non-empty: `host` must contain at least one character, so
`http://:8080/` and `http:///x` (empty host) are invalid. `IP-literal`
(bracketed IPv6/IPvFuture addresses, e.g. `[::1]`) is out of scope:
neither `[` nor `:` is in `reg-name`'s character class, so a bracketed
host fails host validation and the URL is rejected — the same outcome
as any other invalid host, no special-cased detection needed.

### Percent-encoding

`pct-encoded` (`%` followed by two hex digits) is validated for shape
only, wherever the grammar above allows it — `userinfo`, `host`,
`path`, `query`, `fragment` — every `%` in those components must be
followed by exactly two hex digits (`0-9`, `a-f`, `A-F`), or the URL
is invalid. Values are never percent-decoded: a valid `path=` line for
input path `/a%2Fb` prints the six literal characters `/a%2Fb`, not
`/a/b`. `scheme` and `port` do not permit `pct-encoded` at all, so a
`%` anywhere in either makes the URL invalid.

### Case

`scheme` is case-insensitive per RFC 3986 §3.1 and is normalized to
lowercase in the output: `HTTP://example.com` and `http://example.com`
both print `scheme=http`. `host` is printed exactly as written, with
no case folding. RFC 3986 §3.2.2 calls `host` case-insensitive too,
but folding it would also raise the question of case-folding
percent-encoded octets inside it — a decoding-flavored question this
split-only task does not answer, so `host` passes through unchanged.

### Presence

- `scheme`, `host`, and `path` are always printed. `scheme` and `host`
  because the grammar requires each to have at least one character —
  an empty one is invalid input, not an absent component. `path`
  because `path-abempty` is concatenated directly onto `authority`
  with no surrounding `[ ]` in the grammar above: it is mandatory,
  though it may be zero length (`http://example.com` has no `/` at
  all, and still prints `path=` with an empty value).
- `userinfo` and `port` are present if and only if their delimiter
  appears (`@` for `userinfo`; the `:` between `host` and `port`) —
  even when nothing follows the delimiter. `http://example.com:/x`
  prints `port=` with an empty value (the `:` is there; no digits
  are). `http://@example.com/x` prints `userinfo=` empty the same
  way.
- `query` and `fragment` are present if and only if their delimiter
  (`?`, `#`) appears, by the same empty-but-present rule:
  `http://example.com/x?` prints `query=` with an empty value, while
  `http://example.com/x` (no `?` anywhere) omits the `query` line
  entirely. Likewise for `#` and `fragment`.

## Expected outputs

Derived by hand from the RFC 3986 ABNF above, restricted to the pinned
`hier-part` shape and the presence/case decisions in this file. The
reference entry is the executable cross-check.
