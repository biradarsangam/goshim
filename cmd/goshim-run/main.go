package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// The libkrun 1.x API scans at most 4096 argument pointers, including the
// terminating NULL.
const krunArgSlots = 4096

type vmConfig struct {
	rootfs  string
	command string
	args    []string
	cpus    uint8
	ramMiB  uint32
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	cfg, err := parseArgs(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "goshim-run:", err)
		return 2
	}
	if err := startVM(cfg); err != nil {
		fmt.Fprintln(stderr, "goshim-run:", err)
		return 125
	}
	return 0
}

func parseArgs(args []string, output io.Writer) (vmConfig, error) {
	flags := flag.NewFlagSet("goshim-run", flag.ContinueOnError)
	flags.SetOutput(output)
	root := flags.String("rootfs", "", "Directory containing the guest Linux root filesystem")
	cpus := flags.Uint64("cpus", 1, "Number of virtual CPUs")
	memory := flags.Uint64("memory-mib", 512, "Guest RAM in MiB")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: goshim-run --rootfs DIR [--cpus N] [--memory-mib N] -- /guest/executable [ARG ...]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return vmConfig{}, err
	}
	if *root == "" {
		return vmConfig{}, errors.New("--rootfs is required")
	}
	if *cpus == 0 || *cpus > math.MaxUint8 {
		return vmConfig{}, fmt.Errorf("--cpus must be between 1 and %d", math.MaxUint8)
	}
	if *memory == 0 || *memory > math.MaxUint32 {
		return vmConfig{}, fmt.Errorf("--memory-mib must be between 1 and %d", uint64(math.MaxUint32))
	}

	guestArgs := flags.Args()
	if len(guestArgs) == 0 || !path.IsAbs(guestArgs[0]) {
		return vmConfig{}, errors.New("an absolute guest executable path is required")
	}
	if len(guestArgs) > krunArgSlots {
		return vmConfig{}, fmt.Errorf("at most %d guest arguments are supported", krunArgSlots-1)
	}
	// Libkrun 1.x serializes the executable and arguments through the kernel
	// command line. It wraps argv entries in quotes without escaping quotes.
	for _, value := range guestArgs {
		if !utf8.ValidString(value) || strings.ContainsAny(value, "\"\x00\n\r\t\v\f") {
			return vmConfig{}, errors.New("the guest executable and arguments must be UTF-8 without double quotes or whitespace control characters")
		}
	}
	// KRUN_INIT, which carries the executable path, is emitted unquoted.
	if strings.ContainsRune(guestArgs[0], ' ') {
		return vmConfig{}, errors.New("the guest executable path cannot contain spaces")
	}

	rootfs, err := filepath.Abs(*root)
	if err != nil {
		return vmConfig{}, fmt.Errorf("resolve rootfs: %w", err)
	}
	info, err := os.Stat(rootfs)
	if err != nil {
		return vmConfig{}, fmt.Errorf("open rootfs: %w", err)
	}
	if !info.IsDir() {
		return vmConfig{}, fmt.Errorf("rootfs %q is not a directory", rootfs)
	}
	return vmConfig{
		rootfs:  rootfs,
		command: guestArgs[0],
		args:    guestArgs[1:],
		cpus:    uint8(*cpus),
		ramMiB:  uint32(*memory),
	}, nil
}
