//go:build libkrun && cgo && ((linux && (amd64 || arm64)) || (darwin && arm64))

package main

/*
#cgo pkg-config: libkrun
#include <libkrun.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

func init() {
	// Keep Hypervisor.framework calls on the process's initial thread.
	runtime.LockOSThread()
}

func startVM(cfg vmConfig) error {
	id := C.krun_create_ctx()
	if err := krunError("create libkrun VM context (check libkrunfw availability)", id); err != nil {
		return err
	}
	ctx := C.uint32_t(id)
	// krun_start_enter consumes the context, including on startup failure.
	if err := configureVM(ctx, cfg); err != nil {
		C.krun_free_ctx(ctx)
		return err
	}
	return krunError("start libkrun VM", C.krun_start_enter(ctx))
}

func configureVM(ctx C.uint32_t, cfg vmConfig) error {
	if err := krunError("configure VM resources", C.krun_set_vm_config(ctx, C.uint8_t(cfg.cpus), C.uint32_t(cfg.ramMiB))); err != nil {
		return err
	}
	if err := krunError("disable implicit vsock", C.krun_disable_implicit_vsock(ctx)); err != nil {
		return err
	}
	root := C.CString(cfg.rootfs)
	defer C.free(unsafe.Pointer(root))
	if err := krunError("configure guest rootfs", C.krun_set_root(ctx, root)); err != nil {
		return err
	}

	command := C.CString(cfg.command)
	defer C.free(unsafe.Pointer(command))
	// The 1.x implementation views the arrays as 4096 pointers before
	// scanning for NULL. Their entries point to C-owned strings.
	var argv, envp [krunArgSlots]*C.char
	for i, arg := range cfg.args {
		argv[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i]))
	}
	// Passing NULL as envp would forward the entire host environment.
	envp[0] = C.CString("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	defer C.free(unsafe.Pointer(envp[0]))
	return krunError("configure guest command", C.krun_set_exec(ctx, command, &argv[0], &envp[0]))
}

func krunError(operation string, status C.int32_t) error {
	if status < 0 {
		return fmt.Errorf("%s: %w", operation, syscall.Errno(-status))
	}
	return nil
}
