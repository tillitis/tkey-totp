// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#include <monocypher/monocypher-ed25519.h>
#include <stdint.h>
#include <tkey/qemu_debug.h>
#include <tkey/tk1_mem.h>
#include <tkey/led.h>

#include "app_proto.h"
#include "cspring.h"
#include "tkey_totp.h"

// clang-format off
static volatile	uint32_t *cdi           = (volatile uint32_t *)TK1_MMIO_TK1_CDI_FIRST;
static volatile uint32_t *cpu_mon_ctrl  = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_CTRL;
static volatile uint32_t *cpu_mon_first = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_FIRST;
static volatile uint32_t *cpu_mon_last  = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_LAST;
static volatile uint32_t *app_addr      = (volatile uint32_t *) TK1_MMIO_TK1_APP_ADDR;
static volatile uint32_t *app_size      = (volatile uint32_t *) TK1_MMIO_TK1_APP_SIZE;

// clang-format on

int main(void)
{
#ifndef NODEBUG
    uint32_t stack;
#endif
    struct frame_header hdr; // Used in both directions
    uint8_t cmd[CMDLEN_MAXBYTES];
    uint8_t rsp[CMDLEN_MAXBYTES];
    uint8_t in;
    uint16_t nbytes = 0;
    uint16_t nbytes_left = 0;
    uint8_t msg_idx;
    uint32_t local_cdi[8];
    cspring_ctx cspring_ctx;

    uint8_t records_buffer[sizeof(encrypted_records_t)];
    records_t records = {};

    memset(records_buffer, 0x00, sizeof(encrypted_records_t));

    // Use Execution Monitor on RAM after app
    *cpu_mon_first = *app_addr + *app_size;
    *cpu_mon_last = TK1_RAM_BASE + TK1_RAM_SIZE;
    *cpu_mon_ctrl = 1;

#ifndef NODEBUG
    qemu_puts("Hello, I'm totp-app! &stack is on: ");
    qemu_putinthex((uint32_t)&stack);
    qemu_lf();
#endif

    // Generate public key
    wordcpy(local_cdi, (void *)cdi, 8);
    cspring_init(&cspring_ctx, local_cdi);


    /* Temp debug: */
    memcpy(records.record[0].name, "Test\0", sizeof("Test\0"));
    records.record[0].name_len = sizeof("Test");
    memcpy(records.record[0].key, "1234\0", sizeof("1234"));
    records.record[0].key_len = sizeof("1234");
    records.nbr_of_records = 1;

    qemu_puts("Test record: name: ");
    qemu_puts((char*)records.record[0].name);
    qemu_lf();
    qemu_puts("Test record: key: ");
    qemu_puts((char*)records.record[0].key);
    qemu_lf();

    set_led(LED_BLUE);
    for (;;)
    {
        in = readbyte();
        qemu_puts("Read byte: ");
        qemu_puthex(in);
        qemu_lf();

        if (parseframe(in, &hdr) == -1)
        {
            qemu_puts("Couldn't parse header\n");
            continue;
        }

        memset(cmd, 0, CMDLEN_MAXBYTES);
        // Read app command, blocking
        read(cmd, hdr.len);

        if (hdr.endpoint == DST_FW)
        {
            appreply_nok(hdr);
            qemu_puts("Responded NOK to message meant for fw\n");
            continue;
        }

        // Is it for us?
        if (hdr.endpoint != DST_SW)
        {
            qemu_puts("Message not meant for app. endpoint was 0x");
            qemu_puthex(hdr.endpoint);
            qemu_lf();
            continue;
        }

        // Reset response buffer
        memset(rsp, 0, CMDLEN_MAXBYTES);

        // Min length is 1 byte so this should always be here
        switch (cmd[0])
        {
        case APP_CMD_GET_NAMEVERSION:
            qemu_puts("APP_CMD_GET_NAMEVERSION\n");
            // only zeroes if unexpected cmdlen bytelen
            if (hdr.len == 1) {
                memcpy(rsp, app_name0, 4);
                memcpy(rsp + 4, app_name1, 4);
                memcpy(rsp + 8, &app_version, 4);
            }
            appreply(hdr, APP_RSP_GET_NAMEVERSION, rsp);
            break;

        // Load encrypted records from client
        case APP_CMD_LOAD_RECORDS:
            qemu_puts("APP_CMD_LOAD_RECORDS\n");
            if (nbytes == 0) {
                // Not received anything yet
                msg_idx = 0;
                nbytes_left = sizeof(encrypted_records_t);
            }

            if (nbytes_left > PAYLOAD_MAXBYTES) {
                nbytes = PAYLOAD_MAXBYTES;
            } else  {
                nbytes = nbytes_left;
            }
            memcpy(&records_buffer[msg_idx], &cmd[1], nbytes);
            msg_idx += nbytes;
            nbytes_left -= nbytes;

            if(nbytes_left == 0) {
                // All data received, decypt.
                encrypted_records_t* enc_records = (encrypted_records_t*) records_buffer;

                crypto_aead_unlock((uint8_t*)&records, enc_records->mac, (const uint8_t *) local_cdi,
                                    enc_records->nonce, NULL, 0, (uint8_t*)&enc_records->records,
                                    sizeof(records_t));

                // Reset transfer helpers.
                nbytes_left = 0;
                msg_idx = 0;

            }

                rsp[0] = STATUS_OK;
				appreply(hdr, APP_RSP_LOAD_RECORDS, rsp);
				break;

            break;
        // Encrypt and return records to client
        case APP_CMD_GET_RECORDS:
            qemu_puts("APP_CMD_GET_RECORDS\n");

            if(records.nbr_of_records == 0 ){
                // Empty, return bad.
                rsp[0] = STATUS_BAD;
				appreply(hdr, APP_RSP_GET_RECORDS, rsp);
				break;
            }

            if (nbytes_left == 0) {
                // First cmd
                nbytes_left = sizeof(encrypted_records_t);
                msg_idx = 0;

                encrypted_records_t* enc_records = (encrypted_records_t*) records_buffer;
                // Encrypt, get new nonce.
                cspring_get((uint32_t *) enc_records->nonce, &cspring_ctx, XCHACHA20_NONCE_LEN);

                crypto_aead_lock((uint8_t*)&enc_records->records, enc_records->mac, (const uint8_t*)local_cdi,
                                enc_records->nonce, NULL, 0, (const uint8_t*)&records, sizeof(records_t));
            }

            if (nbytes_left > PAYLOAD_MAXBYTES) {
                nbytes = PAYLOAD_MAXBYTES;
            } else  {
                nbytes = nbytes_left;
            }

            qemu_puts("nbytes_left: ");
            qemu_putinthex((uint32_t)nbytes_left);
            qemu_lf();

            // Protocol [ status_code (1), bytes_left (2), data (1-125) ]
            memcpy(&rsp[1], &nbytes_left, sizeof(nbytes_left));
            memcpy(rsp + 3, &records_buffer[msg_idx], nbytes);
            msg_idx += nbytes;
            nbytes_left -= nbytes;

            rsp[0] = STATUS_OK;
            appreply(hdr, APP_RSP_GET_RECORDS, rsp);

            break;
        // Get the name of stored records
        case APP_CMD_GET_LIST:
            break;
        // Calculate oath
        case APP_CMD_CALC_TOKEN:
            break;
        // Add a record
        case APP_CMD_ADD_TOKEN:
            break;
        // Delete a record
        case APP_CMD_DEL_TOKEN:
            break;
        // Reset app, clear all memory.
        case APP_CMD_RESET_APP:
            break;
        default:
            qemu_puts("Received unknown command: ");
            qemu_puthex(cmd[0]);
            qemu_lf();
            appreply(hdr, APP_RSP_UNKNOWN_CMD, rsp);
        }
    }
}
