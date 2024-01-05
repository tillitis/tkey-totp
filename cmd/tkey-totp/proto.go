// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/tillitis/tkeyclient"
)

var (
	cmdGetNameVersion = appCmd{0x01, "cmdGetNameVersion", tkeyclient.CmdLen1}
	rspGetNameVersion = appCmd{0x02, "rspGetNameVersion", tkeyclient.CmdLen32}
	cmdLoadRecords    = appCmd{0x03, "cmdLoadRecords", tkeyclient.CmdLen128}
	rspLoadRecords    = appCmd{0x04, "rspLoadRecords", tkeyclient.CmdLen4}
	cmdGetRecords     = appCmd{0x05, "cmdGetRecords", tkeyclient.CmdLen1}
	rspGetRecords     = appCmd{0x06, "rspGetRecords", tkeyclient.CmdLen128}
	cmdGetList        = appCmd{0x07, "cmdGetList", tkeyclient.CmdLen4}
	rspGetList        = appCmd{0x08, "rspGetList", tkeyclient.CmdLen128}
	cmdGetSig         = appCmd{0x09, "cmdCalcToken", tkeyclient.CmdLen4}
	rspCmdSig         = appCmd{0x0a, "rspCalcToken", tkeyclient.CmdLen128}
	cmdAddToken       = appCmd{0x0b, "cmdAddToken", tkeyclient.CmdLen128}
	rspAddToken       = appCmd{0x0c, "rspAddToken", tkeyclient.CmdLen4}
	cmdDelToken       = appCmd{0x0d, "cmdDelToken", tkeyclient.CmdLen4}
	rspDelToken       = appCmd{0x0e, "rspDelToken", tkeyclient.CmdLen4}
	cmdResetApp       = appCmd{0x0f, "cmdResetApp", tkeyclient.CmdLen1}
	rspResetApp       = appCmd{0x10, "rspResetApp", tkeyclient.CmdLen1}
)

// cmdlen - (responsecode + status)
var PayloadMaxBytes = cmdLoadRecords.CmdLen().Bytelen() - (1 + 1)

type appCmd struct {
	code   byte
	name   string
	cmdLen tkeyclient.CmdLen
}

func (c appCmd) Code() byte {
	return c.code
}

func (c appCmd) CmdLen() tkeyclient.CmdLen {
	return c.cmdLen
}

func (c appCmd) Endpoint() tkeyclient.Endpoint {
	return tkeyclient.DestApp
}

func (c appCmd) String() string {
	return c.name
}

type Totp struct {
	tk *tkeyclient.TillitisKey // A connection to a TKey
}

// New allocates a struct for communicating with the random app
// running on the TKey. You're expected to pass an existing connection
// to it, so use it like this:
//
//	tk := tkeyclient.New()
//	err := tk.Connect(port)
//	totp := New(tk)
func New(tk *tkeyclient.TillitisKey) Totp {
	var totp Totp

	totp.tk = tk

	return totp
}

// Close closes the connection to the TKey
func (t Totp) Close() error {
	if err := t.tk.Close(); err != nil {
		return fmt.Errorf("tk.Close: %w", err)
	}
	return nil
}

// GetAppNameVersion gets the name and version of the running app in
// the same style as the stick itself.
func (t Totp) GetAppNameVersion() (*tkeyclient.NameVersion, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetAppNameVersion tx", tx)
	if err = t.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	err = t.tk.SetReadTimeout(2)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	rx, _, err := t.tk.ReadFrame(rspGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	err = t.tk.SetReadTimeout(0)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	nameVer := &tkeyclient.NameVersion{}
	nameVer.Unpack(rx[2:])

	return nameVer, nil
}

func (t Totp) GetRecords() (int, []byte, error) {
	id := 1
	tx, err := tkeyclient.NewFrameBuf(cmdGetRecords, id)
	if err != nil {
		return 0, nil, fmt.Errorf("NewFramebuf: %w", err)
	}
	tkeyclient.Dump("cmdGetRecords tx", tx)
	if err = t.tk.Write(tx); err != nil {
		return 0, nil, fmt.Errorf("Write: %w", err)
	}

	err = t.tk.SetReadTimeout(2)
	if err != nil {
		return 0, nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	rx, _, err := t.tk.ReadFrame(rspGetRecords, id)
	if err != nil {
		return 0, nil, fmt.Errorf("ReadFrame: %w", err)
	}

	err = t.tk.SetReadTimeout(0)
	if err != nil {
		return 0, nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return 0, nil, fmt.Errorf("GetRecords NOK")
	}

	// Bytes left to recieve
	var bytesLeft int16
	buf := bytes.NewReader(rx[3:5])
	err = binary.Read(buf, binary.LittleEndian, &bytesLeft)
	if err != nil {
		return 0, nil, fmt.Errorf("Binary.Read() %w", err)
	}
	le.Printf("%d\n", bytesLeft)

	return int(bytesLeft), rx[5:], nil
}
