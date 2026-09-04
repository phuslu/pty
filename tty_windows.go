//go:build windows

package pty

import (
	"syscall"
	"unsafe"
)

var (
	procGetStdHandle   = kernel32.NewProc("GetStdHandle")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

// IsTerminal reports whether the file descriptor refers to a Windows console.
func IsTerminal(fd uintptr) bool {
	var mode uint32
	err := syscall.GetConsoleMode(syscall.Handle(fd), &mode)
	if err != nil {
		return false
	}

	return true
}

// EnableVirtualTerminal enables ConHost's ENABLE_VIRTUAL_TERMINAL_INPUT and
// ENABLE_VIRTUAL_TERMINAL_PROCESSING modes on the current process's standard
// handles. It returns an error if a selected handle is not a console.
func EnableVirtualTerminal(stdin, stdout, stderr bool) error {
	const (
		STD_INPUT_HANDLE  = ^uint32(9)  // -10
		STD_OUTPUT_HANDLE = ^uint32(10) // -11
		STD_ERROR_HANDLE  = ^uint32(11) // -12

		ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
		ENABLE_VIRTUAL_TERMINAL_INPUT      = 0x0200
	)

	enable := func(stdHandle uint32, mask uint32) error {
		r0, _, e1 := procGetStdHandle.Call(uintptr(stdHandle))
		handle := syscall.Handle(r0)

		if handle == syscall.InvalidHandle {
			if e1 != syscall.Errno(0) {
				return error(e1)
			}
			return syscall.EINVAL
		}

		var mode uint32

		r1, _, e1 := procGetConsoleMode.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&mode)),
		)
		if r1 == 0 {
			if e1 != syscall.Errno(0) {
				return error(e1)
			}
			return syscall.EINVAL
		}

		if mode&mask == mask {
			return nil
		}

		r1, _, e1 = procSetConsoleMode.Call(
			uintptr(handle),
			uintptr(mode|mask),
		)
		if r1 == 0 {
			if e1 != syscall.Errno(0) {
				return error(e1)
			}
			return syscall.EINVAL
		}

		return nil
	}

	if stdin {
		if err := enable(STD_INPUT_HANDLE, ENABLE_VIRTUAL_TERMINAL_INPUT); err != nil {
			return err
		}
	}

	if stdout {
		if err := enable(STD_OUTPUT_HANDLE, ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
			return err
		}
	}

	if stderr {
		if err := enable(STD_ERROR_HANDLE, ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
			return err
		}
	}

	return nil
}
