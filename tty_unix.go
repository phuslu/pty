//go:build !windows

package pty

import (
	"runtime"
	"syscall"
	"unsafe"
)

// IsTerminal return true if the file descriptor is terminal.
func IsTerminal(fd uintptr) bool {
	var trap uintptr // SYS_IOCTL
	switch runtime.GOOS {
	case "plan9", "js", "nacl":
		return false
	case "linux", "android":
		switch runtime.GOARCH {
		case "amd64":
			trap = 16
		case "arm64":
			trap = 29
		case "mips", "mipsle":
			trap = 4054
		case "mips64", "mips64le":
			trap = 5015
		default:
			trap = 54
		}
	default:
		trap = 54
	}

	var req uintptr // TIOCGETA
	switch runtime.GOOS {
	case "linux", "android":
		switch runtime.GOARCH {
		case "ppc64", "ppc64le":
			req = 0x402c7413
		case "mips", "mipsle", "mips64", "mips64le":
			req = 0x540d
		default:
			req = 0x5401
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			req = 0x40487413
		default:
			req = 0x402c7413
		}
	default:
		req = 0x402c7413
	}

	var termios [256]byte
	_, _, err := syscall.Syscall6(trap, fd, req, uintptr(unsafe.Pointer(&termios[0])), 0, 0, 0)
	return err == 0
}
