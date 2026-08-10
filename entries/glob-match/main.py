# SPDX-License-Identifier: GPL-3.0-or-later

# glob-match: does <string> match the <pattern> glob?
# Implements the dialect of tasks/glob-match/spec.md - a POSIX fnmatch
# subset matched over Unicode codepoints, not bytes. The matcher below
# is hand-written: no re, no fnmatch import, so the pinned dialect and
# the code that runs it are the same thing, not two things that can
# drift apart.
import sys

# Token kinds. A token list is the parsed form of a pattern; building
# one can fail (return None) on an unclosed bracket or a bracket range
# whose endpoints are reversed - both are invalid-pattern conditions.
STAR = 0
ANY = 1
LITERAL = 2
BRACKET = 3


def build_tokens(pattern):
    tokens = []
    i = 0
    n = len(pattern)
    while i < n:
        c = pattern[i]
        if c == "*":
            tokens.append((STAR, None))
            i += 1
        elif c == "?":
            tokens.append((ANY, None))
            i += 1
        elif c == "[":
            parsed = parse_bracket(pattern, i)
            if parsed is None:
                return None
            token, i = parsed
            tokens.append(token)
        else:
            tokens.append((LITERAL, ord(c)))
            i += 1
    return tokens


def parse_bracket(pattern, start):
    # pattern[start] is the opening '['. Returns (token, index-after-
    # closing-bracket), or None if there is no closing ']' for this
    # bracket, or a range inside it has lo > hi.
    n = len(pattern)
    i = start + 1
    negate = False
    if i < n and pattern[i] == "!":
        negate = True
        i += 1
    content_start = i
    if i < n and pattern[i] == "]":
        # A ']' right after '[' or '[!' is a literal member of the
        # set, not the closing bracket - the real close is the next
        # ']' found from here on.
        i += 1
    close = pattern.find("]", i)
    if close == -1:
        return None
    content = pattern[content_start:close]
    literals = set()
    ranges = []
    j = 0
    m = len(content)
    while j < m:
        if j + 1 < m and content[j + 1] == "-" and j + 2 < m:
            # A '-' with a character on both sides is a range; at the
            # first or last content position it has no such pair and
            # falls through to the literal branch below instead.
            lo = ord(content[j])
            hi = ord(content[j + 2])
            if lo > hi:
                return None
            ranges.append((lo, hi))
            j += 3
        else:
            literals.add(ord(content[j]))
            j += 1
    return (BRACKET, (negate, literals, ranges)), close + 1


def bracket_matches(spec, cp):
    negate, literals, ranges = spec
    member = cp in literals
    if not member:
        for lo, hi in ranges:
            if lo <= cp <= hi:
                member = True
                break
    return (not member) if negate else member


def token_matches(token, cp):
    kind, data = token
    if kind == ANY:
        return True
    if kind == LITERAL:
        return data == cp
    return bracket_matches(data, cp)


def matches(tokens, s):
    # Iterative two-pointer scan with backtracking on the most recent
    # '*': the classic wildcard-matching algorithm. Every non-STAR
    # token consumes exactly one codepoint of s (a bracket expression
    # does, same as '?'), so the same two-pointer shape that works for
    # plain wildcards works here unchanged.
    ti = 0
    si = 0
    star_ti = -1
    star_si = -1
    tn = len(tokens)
    sn = len(s)
    while si < sn:
        if ti < tn and tokens[ti][0] != STAR and token_matches(tokens[ti], ord(s[si])):
            ti += 1
            si += 1
        elif ti < tn and tokens[ti][0] == STAR:
            star_ti = ti
            star_si = si
            ti += 1  # first try: the star consumes nothing
        elif star_ti != -1:
            # No match here - make the last star consume one more
            # codepoint and replay from just after it.
            star_si += 1
            ti = star_ti + 1
            si = star_si
        else:
            return False
    while ti < tn and tokens[ti][0] == STAR:
        ti += 1  # trailing stars may match the empty remainder
    return ti == tn


def main():
    if len(sys.argv) != 3:
        print("usage: prog <pattern> <string>", file=sys.stderr)
        sys.exit(2)
    pattern = sys.argv[1]
    string = sys.argv[2]
    tokens = build_tokens(pattern)
    if tokens is None:
        print("invalid pattern", file=sys.stderr)
        sys.exit(1)
    if matches(tokens, string):
        print("yes")
    else:
        print("no")


main()
