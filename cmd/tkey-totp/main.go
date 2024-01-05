// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/tillitis/tkeyclient"
	"github.com/tillitis/tkeyutil"
)

// nolint:typecheck // Avoid lint error when the embedding file is missing.
// Build copies the built device app here
//
//go:embed app.bin
var deviceAppBinary []byte

// Use when printing err/diag msgs
var le = log.New(os.Stderr, "", 0)

var (
	version string
	verbose = false
)

// loadApp loads the device app into the TKey at device
// devPath with speed b/s, possibly using a User Supplied Secret
// either in fileUSS or prompting for the USS interactively if
// enterUSS is true.
//
// It then connects to the running app and returns an interface to
// the app, and possible errors.
func loadApp(devPath string, speed int, fileUSS string, enterUSS bool) (*Totp, error) {
	if !verbose {
		tkeyclient.SilenceLogging()
	}

	if devPath == "" {
		var err error
		devPath, err = tkeyclient.DetectSerialPort(false)
		if err != nil {
			return nil, fmt.Errorf("DetectSerialPort: %w", err)
		}
	}

	tk := tkeyclient.New()
	if verbose {
		le.Printf("Connecting to TKey on serial port %s ...", devPath)
	}
	if err := tk.Connect(devPath, tkeyclient.WithSpeed(speed)); err != nil {
		return nil, fmt.Errorf("could not open %s: %w", devPath, err)
	}

	if isFirmwareMode(tk) {
		var secret []byte
		var err error

		if enterUSS {
			secret, err = tkeyutil.InputUSS()
			if err != nil {
				tk.Close()
				return nil, fmt.Errorf("InputUSS: %w", err)
			}
		}
		if fileUSS != "" {
			secret, err = tkeyutil.ReadUSS(fileUSS)
			if err != nil {
				tk.Close()
				return nil, fmt.Errorf("ReadUSS: %w", err)
			}
		}

		if err := tk.LoadApp(deviceAppBinary, secret); err != nil {
			tk.Close()
			return nil, fmt.Errorf("couldn't load device app: %w", err)
		}

		if verbose {
			le.Printf("Device app loaded.")
		}
	} else {
		if enterUSS || fileUSS != "" {
			le.Printf("WARNING: App already loaded, your USS won't be used.")
		} else {
			le.Printf("WARNING: App already loaded.")
		}
	}

	totp := New(tk)

	handleSignals(func() { os.Exit(1) }, os.Interrupt, syscall.SIGTERM)

	if !isWantedApp(totp) {
		totp.Close()
		return nil, fmt.Errorf("no TKey on the serial port, or it's running wrong app (and is not in firmware mode)")
	}

	return &totp, nil
}

func usage() {
	desc := fmt.Sprintf(`Usage:

%[1]s -h/--help

%[1]s --version

%[1]s calculate totp tokens`,
		os.Args[0])

	le.Printf("%s\n\n%s", desc,
		pflag.CommandLine.FlagUsagesWrapped(86))
}

func main() {
	var cmdArgs int
	devPath := pflag.StringP("port", "d", "",
		"Set serial port `device`. If this is not used, auto-detection will be attempted.")
	speed := pflag.IntP("speed", "s", tkeyclient.SerialSpeed,
		"Set serial port `speed` in bits per second.")
	enterUss := pflag.Bool("uss", false,
		"Enable typing of a phrase to be hashed as the User Supplied Secret. The USS is loaded onto the TKey along with the app itself. A different USS results in different public/private keys.")
	ussFile := pflag.String("uss-file", "",
		"Read `ussfile` and hash its contents as the USS. Use '-' (dash) to read from stdin. The full contents are hashed unmodified (e.g. newlines are not stripped).")
	versionOnly := pflag.BoolP("version", "v", false, "Output version information.")
	helpOnly := pflag.BoolP("help", "h", false, "Output this help.")

	if version == "" {
		version = readBuildInfo()
	}

	pflag.BoolVar(&verbose, "verbose", false, "Enable verbose output.")
	pflag.Usage = usage
	pflag.Parse()

	if pflag.NArg() > 0 {
		le.Printf("Unexpected argument: %s\n\n", strings.Join(pflag.Args(), " "))
		pflag.Usage()
		os.Exit(2)
	}

	if *versionOnly {
		le.Printf("tkey-totp %s", version)
		os.Exit(0)
	}

	if *helpOnly {
		pflag.Usage()
		os.Exit(0)

	}
	if cmdArgs > 1 {
		pflag.Usage()
		os.Exit(1)
	}

	_, err := loadApp(*devPath, *speed, *ussFile, *enterUss)
	if err != nil {
		fmt.Errorf("loadApp: %w", err)
	}
	// Reset app, to have nothing stored.

	// Success
	os.Exit(0)
}
