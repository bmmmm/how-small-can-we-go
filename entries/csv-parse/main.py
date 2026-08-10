# SPDX-License-Identifier: GPL-3.0-or-later

# csv-parse: read an RFC 4180 CSV file, print it as a JSON array of
# records (each record a JSON array of field strings). Implements the
# dialect of tasks/csv-parse/spec.md - all-or-nothing: nothing reaches
# stdout unless the whole file parses. Hand-rolled state machine over
# the decoded text - no csv module, so every dialect corner is spelled
# out here instead of hidden behind a library default.
import json
import sys

QUOTE = '"'
COMMA = ","
CR = "\r"
LF = "\n"

START = "start"      # about to read a field; may still turn into
                      # nothing if EOF hits here with no pending record
UNQUOTED = "unquoted"  # inside an unquoted field
QUOTED = "quoted"      # inside a quoted field, before its closing quote
QQUOTE = "qquote"      # just saw a quote while inside a quoted field -
                       # the next char decides: another quote is an
                       # escaped literal, anything else closes the field


def fail(pos, msg):
    print("at position " + str(pos) + ": " + msg, file=sys.stderr)
    sys.exit(1)


def parse(text):
    n = len(text)
    records = []
    record = []
    field = []
    state = START
    i = 0

    def end_field():
        record.append("".join(field))
        field.clear()

    def end_record():
        end_field()
        records.append(list(record))
        record.clear()

    while i < n:
        c = text[i]

        if state == QUOTED:
            if c == QUOTE:
                state = QQUOTE
            else:
                field.append(c)  # comma, CR, LF: literal inside quotes
            i += 1
            continue

        if state == QQUOTE:
            if c == QUOTE:
                field.append(QUOTE)  # "" is an escaped literal quote
                state = QUOTED
                i += 1
                continue
            if c == COMMA:
                end_field()
                state = START
                i += 1
                continue
            if c == CR:
                if i + 1 < n and text[i + 1] == LF:
                    end_record()
                    state = START
                    i += 2
                    continue
                fail(i, "bare CR not followed by LF after closing quote")
            if c == LF:
                end_record()
                state = START
                i += 1
                continue
            fail(i, "only a comma or record end may follow a closing quote")
            continue

        # state is START or UNQUOTED: both outside any quote.
        if c == COMMA:
            end_field()
            state = START
            i += 1
        elif c == CR:
            if i + 1 < n and text[i + 1] == LF:
                end_record()
                state = START
                i += 2
            else:
                fail(i, "bare CR not followed by LF")
        elif c == LF:
            end_record()
            state = START
            i += 1
        elif c == QUOTE:
            if state == START:
                state = QUOTED
                i += 1
            else:
                fail(i, "quote inside an unquoted field")
        else:
            field.append(c)
            state = UNQUOTED
            i += 1

    if state == QUOTED:
        fail(n, "unclosed quote at end of file")
    if state == UNQUOTED or state == QQUOTE:
        end_record()
    elif state == START and record:
        end_record()  # a trailing comma left one more, empty, field

    return records


def main():
    if len(sys.argv) != 2:
        print("usage: prog <file>", file=sys.stderr)
        sys.exit(2)
    try:
        # newline="" disables newline translation entirely: CRLF and a
        # bare LF both reach the parser untouched, exactly as the
        # dialect needs - the parser itself decides what a terminator
        # is, not the file layer.
        with open(sys.argv[1], encoding="utf-8", newline="") as f:
            text = f.read()
    except (OSError, UnicodeDecodeError) as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    records = parse(text)
    # separators=(",", ":") drops the spaces json.dumps adds by
    # default; ensure_ascii=True's escaping is documented in the spec
    # to match this program's contract byte for byte.
    out = json.dumps(records, ensure_ascii=True, separators=(",", ":"))
    sys.stdout.write(out + "\n")


main()
