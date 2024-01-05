// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#ifndef TKEY_TOTP_H
#define TKEY_TOTP_H

#include <stdint.h>


const uint8_t app_name0[4] = "tk1 ";
const uint8_t app_name1[4] = "totp";
const uint32_t app_version = 0x00000001;

typedef struct
{
    uint8_t name[32];
    uint8_t name_len;
    uint8_t key[32];
    uint8_t key_len;
    uint8_t digits;
    uint8_t config;
} record_t;

typedef struct
{
    uint8_t nbr_of_records;
    record_t record[32];
    uint8_t config;
} records_t;

#define XCHACHA20_NONCE_LEN 24
#define XCHACHA20_MAC_LEN 16

typedef struct
{
    records_t records;
    uint8_t nonce[XCHACHA20_NONCE_LEN];
    uint8_t mac[XCHACHA20_MAC_LEN];
} encrypted_records_t;

// cmdlen - responsecode
#define PAYLOAD_MAXBYTES (CMDLEN_MAXBYTES - 1)


#endif