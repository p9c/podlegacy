/*
 * Copyright (c) 2016 abc at openwall dot com
 * Copyright (c) 2016 Jack Grigg
 * Copyright (c) 2016 The Zcash developers
 * Copyright (c) 2016 John Tromp
 *
 * Distributed under the MIT software license, see the accompanying
 * file COPYING or http://www.opensource.org/licenses/mit-license.php.
 *
 * Port to C of C++ implementation of the Equihash Proof-of-Work
 * algorithm from zcashd.
 */

#include <string.h>
#include <stdint.h>
#include <stdbool.h>
#include <assert.h>
#include <stdlib.h>

#include <sodium.h>

#include "endian.h"

#define N 200
#define K 9

static const uint32_t PROOFSIZE = 1 << K;
static const uint32_t le_N = htole32(N);
static const uint32_t le_K = htole32(K);

static void digestInit(crypto_generichash_blake2b_state *S) {
  unsigned char personalization[crypto_generichash_blake2b_PERSONALBYTES] = {};
  memcpy(personalization, "ZcashPoW", 9);
  memcpy(personalization + 8,  &le_N, 4);
  memcpy(personalization + 12, &le_K, 4);
  crypto_generichash_blake2b_init_salt_personal(S,
    NULL, 0, (PROOFSIZE / N) * N / 8, NULL, personalization);
}

static void expandArray(const unsigned char *in, const size_t in_len,
    unsigned char *out, const size_t out_len,
    const size_t bit_len, const size_t byte_pad)
{
    assert(bit_len >= 8);
    assert(8 * sizeof(uint32_t) >= 7 + bit_len);

    const size_t out_width = (bit_len + 7) / 8 + byte_pad;
    assert(out_len == 8 * out_width * in_len / bit_len);

    const uint32_t bit_len_mask = ((uint32_t)1 << bit_len) - 1;

    // The acc_bits least-significant bits of acc_value represent a bit sequence
    // in big-endian order.
    size_t acc_bits = 0;
    uint32_t acc_value = 0;

    size_t j = 0;
    for (size_t i = 0; i < in_len; i++) {
      acc_value = (acc_value << 8) | in[i];
      acc_bits += 8;

      // When we have bit_len or more bits in the accumulator, write the next
      // output element.
      if (acc_bits >= bit_len) {
        acc_bits -= bit_len;
        for (size_t x = 0; x < byte_pad; x++) {
          out[j + x] = 0;
        }
        for (size_t x = byte_pad; x < out_width; x++) {
          out[j + x] = (
            // Big-endian
            acc_value >> (acc_bits + (8 * (out_width - x - 1)))
          ) & (
            // Apply bit_len_mask across byte boundaries
            (bit_len_mask >> (8 * (out_width - x - 1))) & 0xFF
          );
        }
        j += out_width;
      }
    }
}

static int isZero(const uint8_t *hash, size_t len) {
  // This doesn't need to be constant time.
  for (int i = 0; i < len; i++) {
    if (hash[i] != 0)
      return 0;
  }
  return 1;
}

static void generateHash(crypto_generichash_blake2b_state *S, const uint32_t g, uint8_t *hash, const size_t hashLen) {
  const uint32_t le_g = htole32(g);
  crypto_generichash_blake2b_state digest = *S; /* copy */

  crypto_generichash_blake2b_update(&digest, (uint8_t *)&le_g, sizeof(le_g));
  crypto_generichash_blake2b_final(&digest, hash, hashLen);
}

int compu32(const void *pa, const void *pb) {
  uint32_t a = *(uint32_t *)pa, b = *(uint32_t *)pb;
  return a < b ? -1 : a == b ? 0 : +1;
}

bool duplicate(uint32_t prf[PROOFSIZE]) {
  uint32_t sortprf[PROOFSIZE];
  memcpy(sortprf, prf, sizeof(uint32_t[PROOFSIZE]));
  qsort(sortprf, PROOFSIZE, sizeof(uint32_t), &compu32);

  for (uint32_t i = 1; i < PROOFSIZE; i++)
    if (sortprf[i] <= sortprf[i-1])
      return true;
  return false;
}

// hdr -> header including nonce (140 bytes)
// soln -> equihash solution (excluding 3 bytes with size, so 1344 bytes length)
bool verify(const char *hdr, const char *soln) {
  const int collisionBitLength  = N / (K + 1);
  const int collisionByteLength = (collisionBitLength + 7) / 8;
  const int hashLength = (K + 1) * collisionByteLength;
  const int indicesPerHashOutput = PROOFSIZE / N;
  const int hashOutput = indicesPerHashOutput * N / 8;
  const int equihashSolutionSize = (1 << K) * (N / (K + 1) + 1) / 8;

  uint32_t indices[PROOFSIZE];

  crypto_generichash_blake2b_state state;
  digestInit(&state);
  crypto_generichash_blake2b_update(&state, hdr, 140);

  expandArray(soln, equihashSolutionSize, (char *)&indices, sizeof(indices), collisionBitLength + 1, 1);

  uint8_t vHash[hashLength];
  memset(vHash, 0 , sizeof(vHash));

  if (duplicate(indices))
    return false;

  for (int j = 0; j < PROOFSIZE; j++) {
    uint8_t tmpHash[hashOutput];
    uint8_t hash[hashLength];
    int i = be32toh(indices[j]);
    generateHash(&state, i / indicesPerHashOutput, tmpHash, hashOutput);
    expandArray(tmpHash + (i % indicesPerHashOutput * N / 8), N / 8, hash, hashLength, collisionBitLength, 0);

    if ((j % 2 == 0) && !(i < be32toh(indices[j+1])))
      return false;

    for (int k = 0; k < hashLength; ++k)
  	    vHash[k] ^= hash[k];
  }
  return isZero(vHash, sizeof(vHash));
}
