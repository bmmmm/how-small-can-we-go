/* json-pretty: re-emit a JSON file in the task's canonical pretty form.
 *
 * - Object keys are sorted by the byte-wise order of their UTF-8 encoding;
 *   duplicate keys keep only the last occurrence.
 * - Number tokens are copied byte-for-byte from the input, never
 *   reformatted.
 * - Strings are decoded to UTF-8 bytes, then re-escaped by a fixed rule
 *   (minimal escaping, plus U+2028/U+2029 for JS pasteability).
 */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ---- byte buffers -------------------------------------------------- */

typedef struct {
    char *data;
    size_t len;
} bytes;

typedef struct {
    char *data;
    size_t len;
    size_t cap;
} strbuf;

static void strbuf_push(strbuf *b, char c) {
    if (b->len == b->cap) {
        b->cap = b->cap ? b->cap * 2 : 32;
        b->data = realloc(b->data, b->cap);
    }
    b->data[b->len++] = c;
}

/* Encode a Unicode code point as UTF-8 and append it to b. Lone surrogates
 * (only reachable via an unpaired \uD8xx-\uDFxx escape) are encoded with
 * the 3-byte form; no case in this task's spec exercises that input. */
static void strbuf_push_utf8(strbuf *b, uint32_t cp) {
    if (cp <= 0x7F) {
        strbuf_push(b, (char)cp);
    } else if (cp <= 0x7FF) {
        strbuf_push(b, (char)(0xC0 | (cp >> 6)));
        strbuf_push(b, (char)(0x80 | (cp & 0x3F)));
    } else if (cp <= 0xFFFF) {
        strbuf_push(b, (char)(0xE0 | (cp >> 12)));
        strbuf_push(b, (char)(0x80 | ((cp >> 6) & 0x3F)));
        strbuf_push(b, (char)(0x80 | (cp & 0x3F)));
    } else {
        strbuf_push(b, (char)(0xF0 | (cp >> 18)));
        strbuf_push(b, (char)(0x80 | ((cp >> 12) & 0x3F)));
        strbuf_push(b, (char)(0x80 | ((cp >> 6) & 0x3F)));
        strbuf_push(b, (char)(0x80 | (cp & 0x3F)));
    }
}

/* ---- JSON value tree ------------------------------------------------ */

typedef enum {
    JSON_NULL,
    JSON_TRUE,
    JSON_FALSE,
    JSON_NUMBER,
    JSON_STRING,
    JSON_ARRAY,
    JSON_OBJECT,
} json_type;

typedef struct json_value json_value;

typedef struct {
    bytes key; /* decoded key bytes */
    json_value *value;
} member;

struct json_value {
    json_type type;
    bytes number;        /* JSON_NUMBER: raw input token, copied verbatim */
    bytes string;         /* JSON_STRING: decoded UTF-8 bytes */
    json_value **items;   /* JSON_ARRAY */
    size_t item_count;
    member *members;      /* JSON_OBJECT: deduplicated, unsorted until sort_tree() */
    size_t member_count;
};

static json_value *new_value(json_type type) {
    json_value *v = calloc(1, sizeof(json_value));
    v->type = type;
    return v;
}

/* ---- parser ----------------------------------------------------------
 *
 * The whole input file is kept in memory for the parser's lifetime:
 * JSON_NUMBER values point directly into it (byte-for-byte preservation),
 * so the buffer must outlive printing. Everything else is discarded on
 * error, and this short-lived process never frees it. On a duplicate
 * object key, the earlier value tree is silently orphaned rather than
 * freed, for the same reason.
 */

typedef struct {
    const uint8_t *buf;
    size_t len;
    size_t pos;
    int error;
} parser;

static int is_digit(int c) { return c >= '0' && c <= '9'; }

static int peek(const parser *p) {
    return (p->pos < p->len) ? p->buf[p->pos] : -1;
}

static void skip_ws(parser *p) {
    while (p->pos < p->len) {
        uint8_t c = p->buf[p->pos];
        if (c != ' ' && c != '\t' && c != '\n' && c != '\r') {
            break;
        }
        p->pos++;
    }
}

static json_value *parse_value(parser *p);

static json_value *parse_literal(parser *p, const char *lit, json_type type) {
    size_t n = strlen(lit);
    if (p->pos + n > p->len || memcmp(p->buf + p->pos, lit, n) != 0) {
        p->error = 1;
        return NULL;
    }
    p->pos += n;
    return new_value(type);
}

static json_value *parse_number(parser *p) {
    size_t start = p->pos;

    if (peek(p) == '-') {
        p->pos++;
    }
    if (peek(p) == '0') {
        p->pos++;
    } else if (is_digit(peek(p))) {
        while (is_digit(peek(p))) {
            p->pos++;
        }
    } else {
        p->error = 1;
        return NULL;
    }

    if (peek(p) == '.') {
        p->pos++;
        if (!is_digit(peek(p))) {
            p->error = 1;
            return NULL;
        }
        while (is_digit(peek(p))) {
            p->pos++;
        }
    }

    if (peek(p) == 'e' || peek(p) == 'E') {
        p->pos++;
        if (peek(p) == '+' || peek(p) == '-') {
            p->pos++;
        }
        if (!is_digit(peek(p))) {
            p->error = 1;
            return NULL;
        }
        while (is_digit(peek(p))) {
            p->pos++;
        }
    }

    json_value *v = new_value(JSON_NUMBER);
    v->number.data = (char *)(p->buf + start);
    v->number.len = p->pos - start;
    return v;
}

/* Read exactly 4 hex digits at the current position into a code unit. */
static uint32_t parse_hex4(parser *p) {
    if (p->pos + 4 > p->len) {
        p->error = 1;
        return 0;
    }
    uint32_t v = 0;
    for (int i = 0; i < 4; i++) {
        uint8_t c = p->buf[p->pos + i];
        v <<= 4;
        if (c >= '0' && c <= '9') {
            v |= (uint32_t)(c - '0');
        } else if (c >= 'a' && c <= 'f') {
            v |= (uint32_t)(c - 'a' + 10);
        } else if (c >= 'A' && c <= 'F') {
            v |= (uint32_t)(c - 'A' + 10);
        } else {
            p->error = 1;
            return 0;
        }
    }
    p->pos += 4;
    return v;
}

/* Parse a JSON string literal (starting at the opening quote) into decoded
 * UTF-8 bytes. Raw multi-byte UTF-8 in the input is copied through
 * unexamined; only backslash escapes are interpreted. */
static bytes parse_string_raw(parser *p) {
    strbuf b = {0};

    p->pos++; /* opening quote */
    for (;;) {
        if (p->pos >= p->len) {
            p->error = 1;
            break;
        }
        uint8_t c = p->buf[p->pos];

        if (c == '"') {
            p->pos++;
            break;
        }

        if (c == '\\') {
            p->pos++;
            if (p->pos >= p->len) {
                p->error = 1;
                break;
            }
            uint8_t e = p->buf[p->pos];
            switch (e) {
                case '"':  strbuf_push(&b, '"');  p->pos++; break;
                case '\\': strbuf_push(&b, '\\'); p->pos++; break;
                case '/':  strbuf_push(&b, '/');  p->pos++; break;
                case 'b':  strbuf_push(&b, '\b'); p->pos++; break;
                case 'f':  strbuf_push(&b, '\f'); p->pos++; break;
                case 'n':  strbuf_push(&b, '\n'); p->pos++; break;
                case 'r':  strbuf_push(&b, '\r'); p->pos++; break;
                case 't':  strbuf_push(&b, '\t'); p->pos++; break;
                case 'u': {
                    p->pos++;
                    uint32_t cp = parse_hex4(p);
                    if (p->error) {
                        goto done;
                    }
                    if (cp >= 0xD800 && cp <= 0xDBFF && p->pos + 1 < p->len &&
                        p->buf[p->pos] == '\\' && p->buf[p->pos + 1] == 'u') {
                        size_t save = p->pos;
                        p->pos += 2;
                        uint32_t low = parse_hex4(p);
                        if (!p->error && low >= 0xDC00 && low <= 0xDFFF) {
                            cp = 0x10000 + ((cp - 0xD800) << 10) + (low - 0xDC00);
                        } else {
                            p->pos = save; /* not a low surrogate: leave it alone */
                            p->error = 0;
                        }
                    }
                    strbuf_push_utf8(&b, cp);
                    break;
                }
                default:
                    p->error = 1;
                    goto done;
            }
            continue;
        }

        if (c < 0x20) {
            p->error = 1; /* control characters must be escaped */
            break;
        }
        strbuf_push(&b, (char)c);
        p->pos++;
    }

done:
    return (bytes){b.data, b.len};
}

static json_value *parse_string_value(parser *p) {
    bytes s = parse_string_raw(p);
    if (p->error) {
        return NULL;
    }
    json_value *v = new_value(JSON_STRING);
    v->string = s;
    return v;
}

static json_value *parse_array(parser *p) {
    json_value *v = new_value(JSON_ARRAY);
    size_t cap = 0;

    p->pos++; /* '[' */
    skip_ws(p);
    if (peek(p) == ']') {
        p->pos++;
        return v;
    }

    for (;;) {
        json_value *item = parse_value(p);
        if (p->error || !item) {
            p->error = 1;
            return v;
        }
        if (v->item_count == cap) {
            cap = cap ? cap * 2 : 4;
            v->items = realloc(v->items, cap * sizeof(json_value *));
        }
        v->items[v->item_count++] = item;

        skip_ws(p);
        int c = peek(p);
        if (c == ',') {
            p->pos++;
            skip_ws(p);
            continue;
        }
        if (c == ']') {
            p->pos++;
            break;
        }
        p->error = 1;
        return v;
    }
    return v;
}

static json_value *parse_object(parser *p) {
    json_value *v = new_value(JSON_OBJECT);
    size_t cap = 0;

    p->pos++; /* '{' */
    skip_ws(p);
    if (peek(p) == '}') {
        p->pos++;
        return v;
    }

    for (;;) {
        if (peek(p) != '"') {
            p->error = 1;
            return v;
        }
        bytes key = parse_string_raw(p);
        if (p->error) {
            return v;
        }

        skip_ws(p);
        if (peek(p) != ':') {
            p->error = 1;
            free(key.data);
            return v;
        }
        p->pos++;
        skip_ws(p);

        json_value *val = parse_value(p);
        if (p->error || !val) {
            p->error = 1;
            free(key.data);
            return v;
        }

        /* Duplicate key: the last occurrence wins, in place. */
        int replaced = 0;
        for (size_t i = 0; i < v->member_count; i++) {
            member *m = &v->members[i];
            if (m->key.len == key.len && memcmp(m->key.data, key.data, key.len) == 0) {
                m->value = val;
                free(key.data);
                replaced = 1;
                break;
            }
        }
        if (!replaced) {
            if (v->member_count == cap) {
                cap = cap ? cap * 2 : 4;
                v->members = realloc(v->members, cap * sizeof(member));
            }
            v->members[v->member_count].key = key;
            v->members[v->member_count].value = val;
            v->member_count++;
        }

        skip_ws(p);
        int c = peek(p);
        if (c == ',') {
            p->pos++;
            skip_ws(p);
            continue;
        }
        if (c == '}') {
            p->pos++;
            break;
        }
        p->error = 1;
        return v;
    }
    return v;
}

static json_value *parse_value(parser *p) {
    skip_ws(p);
    int c = peek(p);
    switch (c) {
        case '{': return parse_object(p);
        case '[': return parse_array(p);
        case '"': return parse_string_value(p);
        case 't': return parse_literal(p, "true", JSON_TRUE);
        case 'f': return parse_literal(p, "false", JSON_FALSE);
        case 'n': return parse_literal(p, "null", JSON_NULL);
        default:
            if (c == '-' || is_digit(c)) {
                return parse_number(p);
            }
            p->error = 1;
            return NULL;
    }
}

/* ---- key sort ---------------------------------------------------------
 *
 * Byte-wise comparison of the decoded key bytes (memcmp), shorter key
 * first when one is a prefix of the other.
 */
static int member_cmp(const void *a, const void *b) {
    const member *ma = a;
    const member *mb = b;
    size_t min_len = ma->key.len < mb->key.len ? ma->key.len : mb->key.len;
    int c = memcmp(ma->key.data, mb->key.data, min_len);
    if (c != 0) {
        return c;
    }
    if (ma->key.len != mb->key.len) {
        return ma->key.len < mb->key.len ? -1 : 1;
    }
    return 0;
}

static void sort_tree(json_value *v) {
    if (v->type == JSON_OBJECT) {
        qsort(v->members, v->member_count, sizeof(member), member_cmp);
        for (size_t i = 0; i < v->member_count; i++) {
            sort_tree(v->members[i].value);
        }
    } else if (v->type == JSON_ARRAY) {
        for (size_t i = 0; i < v->item_count; i++) {
            sort_tree(v->items[i]);
        }
    }
}

/* ---- printer ----------------------------------------------------------
 *
 * Strings are re-escaped from their decoded UTF-8 bytes: quote, backslash,
 * the five named control escapes, other C0 controls as \u00XX, and the
 * line terminators U+2028/U+2029 (the fixed 3-byte UTF-8 sequences E2 80
 * A8 / E2 80 A9) as  / . Everything else is copied raw.
 */
static void print_escaped_string(const bytes *s) {
    putchar('"');
    for (size_t i = 0; i < s->len;) {
        uint8_t c = (uint8_t)s->data[i];
        switch (c) {
            case '"':  fputs("\\\"", stdout); i++; continue;
            case '\\': fputs("\\\\", stdout); i++; continue;
            case '\b': fputs("\\b", stdout);  i++; continue;
            case '\f': fputs("\\f", stdout);  i++; continue;
            case '\n': fputs("\\n", stdout);  i++; continue;
            case '\r': fputs("\\r", stdout);  i++; continue;
            case '\t': fputs("\\t", stdout);  i++; continue;
            default: break;
        }
        if (c < 0x20) {
            printf("\\u%04x", c);
            i++;
            continue;
        }
        if (c == 0xE2 && i + 2 < s->len && (uint8_t)s->data[i + 1] == 0x80 &&
            ((uint8_t)s->data[i + 2] == 0xA8 || (uint8_t)s->data[i + 2] == 0xA9)) {
            fputs((uint8_t)s->data[i + 2] == 0xA8 ? "\\u2028" : "\\u2029", stdout);
            i += 3;
            continue;
        }
        putchar((char)c);
        i++;
    }
    putchar('"');
}

static void print_indent(int depth) {
    for (int i = 0; i < depth; i++) {
        fputs("  ", stdout);
    }
}

static void print_value(const json_value *v, int depth) {
    switch (v->type) {
        case JSON_NULL:
            fputs("null", stdout);
            break;
        case JSON_TRUE:
            fputs("true", stdout);
            break;
        case JSON_FALSE:
            fputs("false", stdout);
            break;
        case JSON_NUMBER:
            fwrite(v->number.data, 1, v->number.len, stdout);
            break;
        case JSON_STRING:
            print_escaped_string(&v->string);
            break;
        case JSON_ARRAY:
            if (v->item_count == 0) {
                fputs("[]", stdout);
                break;
            }
            fputs("[\n", stdout);
            for (size_t i = 0; i < v->item_count; i++) {
                print_indent(depth + 1);
                print_value(v->items[i], depth + 1);
                if (i + 1 < v->item_count) {
                    fputc(',', stdout);
                }
                fputc('\n', stdout);
            }
            print_indent(depth);
            fputc(']', stdout);
            break;
        case JSON_OBJECT:
            if (v->member_count == 0) {
                fputs("{}", stdout);
                break;
            }
            fputs("{\n", stdout);
            for (size_t i = 0; i < v->member_count; i++) {
                print_indent(depth + 1);
                print_escaped_string(&v->members[i].key);
                fputs(": ", stdout);
                print_value(v->members[i].value, depth + 1);
                if (i + 1 < v->member_count) {
                    fputc(',', stdout);
                }
                fputc('\n', stdout);
            }
            print_indent(depth);
            fputc('}', stdout);
            break;
    }
}

/* ---- entry point -------------------------------------------------- */

static uint8_t *read_all(FILE *f, size_t *out_len) {
    size_t cap = 65536, len = 0;
    uint8_t *buf = malloc(cap);
    if (!buf) {
        return NULL;
    }
    for (;;) {
        if (len == cap) {
            cap *= 2;
            uint8_t *grown = realloc(buf, cap);
            if (!grown) {
                free(buf);
                return NULL;
            }
            buf = grown;
        }
        size_t n = fread(buf + len, 1, cap - len, f);
        len += n;
        if (n == 0) {
            break;
        }
    }
    if (ferror(f)) {
        free(buf);
        return NULL;
    }
    *out_len = len;
    return buf;
}

int main(int argc, char **argv) {
    if (argc != 2) {
        return 1;
    }

    FILE *f = fopen(argv[1], "rb");
    if (!f) {
        return 1;
    }
    size_t len;
    uint8_t *buf = read_all(f, &len);
    fclose(f);
    if (!buf) {
        return 1;
    }

    parser p = {buf, len, 0, 0};
    skip_ws(&p);
    if (p.pos >= p.len) {
        return 1; /* empty (or all-whitespace) file: no value present */
    }

    json_value *v = parse_value(&p);
    if (p.error || !v) {
        return 1;
    }

    skip_ws(&p);
    if (p.pos != p.len) {
        return 1; /* trailing content after the one JSON value */
    }

    sort_tree(v);
    print_value(v, 0);
    putchar('\n');
    return 0;
}
