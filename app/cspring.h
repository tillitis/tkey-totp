// Copyright (C) 2022 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#ifndef CSPRING_H
#define CSPRING_H

#include <stdint.h>

// state context
typedef struct {
	uint32_t state_ctr_lsb;
	uint32_t state_ctr_msb;
	uint32_t reseed_ctr;
	uint32_t state[16];
	uint32_t digest[8];
} cspring_ctx;

void cspring_init(cspring_ctx *ctx, uint32_t *init_state_input);
int cspring_get(uint32_t *output, cspring_ctx *ctx, int bytes);

#endif
