//go:build !libkrun || !cgo || (!linux && !darwin) || (darwin && !arm64) || (linux && !amd64 && !arm64)

package main

import "errors"

func startVM(vmConfig) error {
	return errors.New("libkrun support requires -tags libkrun and cgo on Linux amd64/arm64 or macOS arm64")
}
