/* sha256-file: print the lowercase hex SHA-256 digest of a file.
 *
 * The pinned build image (gcc:14, Debian) ships no OpenSSL headers and the
 * build has no network access, so SHA-256 is implemented here from the
 * FIPS 180-4 specification instead of linking a crypto library.
 */
#include <stdint.h>
#include <stdio.h>

#define SHA256_BLOCK_SIZE 64
#define SHA256_DIGEST_SIZE 32

typedef struct {
    uint32_t state[8];
    uint64_t total_len; /* total input bytes processed */
    uint8_t buf[SHA256_BLOCK_SIZE];
    size_t buf_len; /* bytes currently held in buf */
} sha256_ctx;

static const uint32_t K[64] = {
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1,
    0x923f82a4, 0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
    0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786,
    0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147,
    0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
    0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
    0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a,
    0x5b9cca4f, 0x682e6ff3, 0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
    0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
};

static uint32_t rotr(uint32_t x, unsigned n) {
    return (x >> n) | (x << (32 - n));
}

/* Process exactly one 64-byte block, updating the running state. */
static void sha256_transform(sha256_ctx *ctx, const uint8_t block[SHA256_BLOCK_SIZE]) {
    uint32_t w[64];
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)block[i * 4] << 24) | ((uint32_t)block[i * 4 + 1] << 16) |
               ((uint32_t)block[i * 4 + 2] << 8) | (uint32_t)block[i * 4 + 3];
    }
    for (int i = 16; i < 64; i++) {
        uint32_t s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >> 3);
        uint32_t s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >> 10);
        w[i] = w[i - 16] + s0 + w[i - 7] + s1;
    }

    uint32_t a = ctx->state[0], b = ctx->state[1], c = ctx->state[2], d = ctx->state[3];
    uint32_t e = ctx->state[4], f = ctx->state[5], g = ctx->state[6], h = ctx->state[7];

    for (int i = 0; i < 64; i++) {
        uint32_t S1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
        uint32_t ch = (e & f) ^ (~e & g);
        uint32_t temp1 = h + S1 + ch + K[i] + w[i];
        uint32_t S0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
        uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
        uint32_t temp2 = S0 + maj;

        h = g;
        g = f;
        f = e;
        e = d + temp1;
        d = c;
        c = b;
        b = a;
        a = temp1 + temp2;
    }

    ctx->state[0] += a;
    ctx->state[1] += b;
    ctx->state[2] += c;
    ctx->state[3] += d;
    ctx->state[4] += e;
    ctx->state[5] += f;
    ctx->state[6] += g;
    ctx->state[7] += h;
}

static void sha256_init(sha256_ctx *ctx) {
    ctx->state[0] = 0x6a09e667;
    ctx->state[1] = 0xbb67ae85;
    ctx->state[2] = 0x3c6ef372;
    ctx->state[3] = 0xa54ff53a;
    ctx->state[4] = 0x510e527f;
    ctx->state[5] = 0x9b05688c;
    ctx->state[6] = 0x1f83d9ab;
    ctx->state[7] = 0x5be0cd19;
    ctx->total_len = 0;
    ctx->buf_len = 0;
}

/* Absorb data into the block buffer, transforming every full block.
 * Does not touch total_len, so it can also be used to feed padding. */
static void sha256_process(sha256_ctx *ctx, const uint8_t *data, size_t len) {
    if (ctx->buf_len > 0) {
        size_t take = SHA256_BLOCK_SIZE - ctx->buf_len;
        if (take > len) {
            take = len;
        }
        for (size_t i = 0; i < take; i++) {
            ctx->buf[ctx->buf_len + i] = data[i];
        }
        ctx->buf_len += take;
        data += take;
        len -= take;
        if (ctx->buf_len == SHA256_BLOCK_SIZE) {
            sha256_transform(ctx, ctx->buf);
            ctx->buf_len = 0;
        }
    }

    while (len >= SHA256_BLOCK_SIZE) {
        sha256_transform(ctx, data);
        data += SHA256_BLOCK_SIZE;
        len -= SHA256_BLOCK_SIZE;
    }

    for (size_t i = 0; i < len; i++) {
        ctx->buf[ctx->buf_len + i] = data[i];
    }
    ctx->buf_len += len;
}

static void sha256_update(sha256_ctx *ctx, const uint8_t *data, size_t len) {
    ctx->total_len += len;
    sha256_process(ctx, data, len);
}

static void sha256_final(sha256_ctx *ctx, uint8_t digest[SHA256_DIGEST_SIZE]) {
    uint64_t bit_len = ctx->total_len * 8;

    /* Pad with 0x80 then zeros until 56 bytes into the final block, so an
     * 8-byte big-endian bit length fills out the last 64-byte block. */
    uint8_t pad[SHA256_BLOCK_SIZE] = {0x80};
    size_t pad_len = (ctx->buf_len < 56) ? (56 - ctx->buf_len) : (2 * SHA256_BLOCK_SIZE - 8 - ctx->buf_len);
    sha256_process(ctx, pad, pad_len);

    uint8_t len_bytes[8];
    for (int i = 0; i < 8; i++) {
        len_bytes[i] = (uint8_t)(bit_len >> (56 - 8 * i));
    }
    sha256_process(ctx, len_bytes, 8);

    for (int i = 0; i < 8; i++) {
        digest[i * 4] = (uint8_t)(ctx->state[i] >> 24);
        digest[i * 4 + 1] = (uint8_t)(ctx->state[i] >> 16);
        digest[i * 4 + 2] = (uint8_t)(ctx->state[i] >> 8);
        digest[i * 4 + 3] = (uint8_t)(ctx->state[i]);
    }
}

int main(int argc, char **argv) {
    if (argc != 2) {
        return 1;
    }

    FILE *f = fopen(argv[1], "rb");
    if (!f) {
        return 1;
    }

    sha256_ctx ctx;
    sha256_init(&ctx);

    uint8_t chunk[65536];
    size_t n;
    while ((n = fread(chunk, 1, sizeof(chunk), f)) > 0) {
        sha256_update(&ctx, chunk, n);
    }
    if (ferror(f)) {
        fclose(f);
        return 1;
    }
    fclose(f);

    uint8_t digest[SHA256_DIGEST_SIZE];
    sha256_final(&ctx, digest);

    char hex[SHA256_DIGEST_SIZE * 2 + 1];
    static const char nibble[] = "0123456789abcdef";
    for (int i = 0; i < SHA256_DIGEST_SIZE; i++) {
        hex[i * 2] = nibble[digest[i] >> 4];
        hex[i * 2 + 1] = nibble[digest[i] & 0x0f];
    }
    hex[SHA256_DIGEST_SIZE * 2] = '\0';

    printf("%s\n", hex);
    return 0;
}
