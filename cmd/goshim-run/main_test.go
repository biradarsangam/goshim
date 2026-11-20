package main

import (
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	rootfs := t.TempDir()
	guestArgs := []string{"two words", "", "--guest-flag"}
	args := append([]string{
		"--rootfs", rootfs, "--cpus", "2", "--memory-mib", "768", "--", "/bin/example",
	}, guestArgs...)

	cfg, err := parseArgs(args, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.rootfs != rootfs || cfg.command != "/bin/example" || cfg.cpus != 2 || cfg.ramMiB != 768 {
		t.Fatalf("unexpected VM configuration: %+v", cfg)
	}
	if !slices.Equal(cfg.args, guestArgs) {
		t.Fatalf("guest arguments = %q, want %q", cfg.args, guestArgs)
	}
}

func TestParseArgsRejectsInvalidConfig(t *testing.T) {
	rootfs := t.TempDir()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing rootfs", []string{"/bin/example"}, "--rootfs is required"},
		{"missing directory", []string{"--rootfs", filepath.Join(rootfs, "missing"), "/bin/example"}, "open rootfs"},
		{"relative executable", []string{"--rootfs", rootfs, "bin/example"}, "absolute guest executable"},
		{"executable with spaces", []string{"--rootfs", rootfs, "/bin/my example"}, "path cannot contain spaces"},
		{"unrepresentable argument", []string{"--rootfs", rootfs, "/bin/example", `a"b`}, "without double quotes"},
		{"too many CPUs", []string{"--rootfs", rootfs, "--cpus", "256", "/bin/example"}, "--cpus must be"},
		{"missing RAM", []string{"--rootfs", rootfs, "--memory-mib", "0", "/bin/example"}, "--memory-mib must be"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseArgs(test.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseArgs error = %v, want %q", err, test.want)
			}
		})
	}
}
