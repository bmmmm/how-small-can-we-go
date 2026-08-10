// SPDX-License-Identifier: GPL-3.0-or-later

// url-parse: split <url> into its RFC 3986 components. Implements the
// grammar of tasks/url-parse/spec.md — scheme "://" authority
// path-abempty ["?" query]["#" fragment], authority = [userinfo "@"]
// host [":" port]. Percent-encoding is validated for shape only and
// never decoded, host follows reg-name's character class (so a
// bracketed IPv6 literal fails on '[' and ':', out of scope for this
// task), scheme is lowercased on output while host is left as
// written, and userinfo/port/query/fragment are present exactly when
// their delimiter was seen, even with nothing after it.
package main

import (
	"fmt"
	"os"
	"strings"
)

// components holds the pieces of one parsed URL. The has* flags track
// presence of the delimiter-gated components (userinfo, port, query,
// fragment): each prints only when its delimiter appeared, even if
// the value after it is empty — see spec.md's Presence section.
type components struct {
	scheme, userinfo, host, port, path, query, fragment string
	hasUserinfo, hasPort, hasQuery, hasFragment         bool
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: prog <url>")
		os.Exit(2)
	}
	c, err := parseURL(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "scheme=%s\n", c.scheme)
	if c.hasUserinfo {
		fmt.Fprintf(&b, "userinfo=%s\n", c.userinfo)
	}
	fmt.Fprintf(&b, "host=%s\n", c.host)
	if c.hasPort {
		fmt.Fprintf(&b, "port=%s\n", c.port)
	}
	fmt.Fprintf(&b, "path=%s\n", c.path)
	if c.hasQuery {
		fmt.Fprintf(&b, "query=%s\n", c.query)
	}
	if c.hasFragment {
		fmt.Fprintf(&b, "fragment=%s\n", c.fragment)
	}
	fmt.Print(b.String())
}

// parseURL splits s into components under the grammar pinned in
// spec.md, or returns an error describing the first thing that broke
// it — stderr is free-form diagnostics, not scored output.
func parseURL(s string) (components, error) {
	var c components
	rest := s

	// fragment: the first '#' in the whole string starts it. Anything
	// after it — including further '?' or '#'-lookalikes — is
	// fragment content, per the grammar's "*( pchar / "/" / "?" )".
	// Cut it away before looking for query, so a later '?' inside a
	// fragment is never mistaken for the query delimiter.
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		c.fragment = rest[i+1:]
		c.hasFragment = true
		rest = rest[:i]
		if !validQueryOrFragment(c.fragment) {
			return c, fmt.Errorf("url %q: invalid fragment %q", s, c.fragment)
		}
	}

	// query: the first '?' in what remains (fragment already cut).
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		c.query = rest[i+1:]
		c.hasQuery = true
		rest = rest[:i]
		if !validQueryOrFragment(c.query) {
			return c, fmt.Errorf("url %q: invalid query %q", s, c.query)
		}
	}

	// scheme: everything up to the first ':'. scheme's own charset
	// excludes ':', so that first ':' is unambiguously its delimiter.
	ci := strings.IndexByte(rest, ':')
	if ci <= 0 {
		return c, fmt.Errorf("url %q: missing scheme", s)
	}
	scheme := rest[:ci]
	if !validScheme(scheme) {
		return c, fmt.Errorf("url %q: invalid scheme %q", s, scheme)
	}
	c.scheme = strings.ToLower(scheme)
	rest = rest[ci+1:]

	// This task pins hier-part to the authority form only.
	if !strings.HasPrefix(rest, "//") {
		return c, fmt.Errorf("url %q: missing // authority — authority-less URIs are out of scope for this task", s)
	}
	rest = rest[2:]

	// authority runs to the next '/' — path-abempty always starts
	// with '/' when it is non-empty — or to the end of what is left.
	authEnd := len(rest)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authEnd = i
	}
	authority := rest[:authEnd]
	c.path = rest[authEnd:]
	if !validPath(c.path) {
		return c, fmt.Errorf("url %q: invalid path %q", s, c.path)
	}

	// userinfo: '@' is not in userinfo's own charset, so the first
	// '@' left in authority is unambiguously its delimiter.
	auth := authority
	if i := strings.IndexByte(auth, '@'); i >= 0 {
		c.userinfo = auth[:i]
		c.hasUserinfo = true
		auth = auth[i+1:]
		if !validUserinfo(c.userinfo) {
			return c, fmt.Errorf("url %q: invalid userinfo %q", s, c.userinfo)
		}
	}

	// host/port: ':' is not in reg-name's charset, so the first ':'
	// left in auth is unambiguously the host/port delimiter.
	host := auth
	if i := strings.IndexByte(auth, ':'); i >= 0 {
		host = auth[:i]
		c.port = auth[i+1:]
		c.hasPort = true
		if !validPort(c.port) {
			return c, fmt.Errorf("url %q: invalid port %q", s, c.port)
		}
	}
	if host == "" {
		return c, fmt.Errorf("url %q: empty host", s)
	}
	// Bracketed IP-literal hosts ('[::1]') are out of scope. '[' is
	// not in reg-name's charset either, so validHost below would
	// reject them regardless; this check only gives a clearer reason
	// on stderr.
	if strings.HasPrefix(host, "[") {
		return c, fmt.Errorf("url %q: bracketed IPv6/IP-literal hosts are out of scope", s)
	}
	if !validHost(host) {
		return c, fmt.Errorf("url %q: invalid host %q", s, host)
	}
	c.host = host

	return c, nil
}

func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '.' || b == '_' || b == '~'
}

func isSubDelim(b byte) bool {
	switch b {
	case '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=':
		return true
	}
	return false
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// validPctClass reports whether s consists only of unreserved,
// sub-delims, pct-encoded triples, and whatever extra literal bytes
// the caller's component allows on top of that (e.g. ':' for
// userinfo). Every '%' must be followed by exactly two hex digits —
// this is where pct-encoded shape is enforced; the value is never
// decoded.
func validPctClass(s string, extra func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '%' {
			if i+2 >= len(s) || !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
				return false
			}
			i += 2
			continue
		}
		if isUnreserved(b) || isSubDelim(b) || (extra != nil && extra(b)) {
			continue
		}
		return false
	}
	return true
}

func validScheme(s string) bool {
	if s == "" || !((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		return false
	}
	for i := 1; i < len(s); i++ {
		b := s[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '+' || b == '-' || b == '.' {
			continue
		}
		return false
	}
	return true
}

func validUserinfo(s string) bool {
	return validPctClass(s, func(b byte) bool { return b == ':' })
}

// validHost applies reg-name's charset only — no IPv4address
// dec-octet range checking (999.999.999.999 is a fine reg-name; see
// spec.md's Host section) and no IP-literal brackets.
func validHost(s string) bool {
	return validPctClass(s, nil)
}

func validPort(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validPath applies pchar's charset plus the literal '/' that
// separates segments. path-abempty's grammar allows repeated and
// empty segments ("//a" is a fine path), so no extra structural check
// beyond the character class is needed.
func validPath(s string) bool {
	return validPctClass(s, func(b byte) bool { return b == '/' || b == ':' || b == '@' })
}

// validQueryOrFragment applies pchar's charset plus '/' and '?' — the
// grammar for both query and fragment. '#' is deliberately not in
// this set: it is fragment's own delimiter and cannot appear inside
// either component's value.
func validQueryOrFragment(s string) bool {
	return validPctClass(s, func(b byte) bool { return b == '/' || b == ':' || b == '@' || b == '?' })
}
