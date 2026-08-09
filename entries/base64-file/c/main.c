/* base64-file: print the unwrapped RFC 4648 section 4 Base64 encoding of a
 * file, with standard alphabet and '=' padding, followed by one '\n'.
 */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

static const char alphabet[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
                                "abcdefghijklmnopqrstuvwxyz"
                                "0123456789+/";

/* Read the whole file into a malloc'd buffer, growing as needed. Returns
 * NULL on any read error; *out_len is only valid on success. */
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
    uint8_t *data = read_all(f, &len);
    fclose(f);
    if (!data) {
        return 1;
    }

    size_t out_len = ((len + 2) / 3) * 4;
    char *out = malloc(out_len + 1); /* + trailing '\n' */
    if (!out) {
        free(data);
        return 1;
    }

    size_t oi = 0;
    for (size_t i = 0; i < len; i += 3) {
        size_t rem = len - i;
        uint32_t triple = (uint32_t)data[i] << 16;
        if (rem > 1) {
            triple |= (uint32_t)data[i + 1] << 8;
        }
        if (rem > 2) {
            triple |= data[i + 2];
        }

        out[oi++] = alphabet[(triple >> 18) & 0x3f];
        out[oi++] = alphabet[(triple >> 12) & 0x3f];
        out[oi++] = (rem > 1) ? alphabet[(triple >> 6) & 0x3f] : '=';
        out[oi++] = (rem > 2) ? alphabet[triple & 0x3f] : '=';
    }
    out[oi++] = '\n';

    fwrite(out, 1, oi, stdout);

    free(out);
    free(data);
    return 0;
}
