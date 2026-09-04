//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"runtime"
	"syscall"
	"unsafe"
)

// IsTerminal reports whether the file descriptor refers to a terminal.
func IsTerminal(fd uintptr) bool {
	var trap uintptr // SYS_IOCTL
	switch runtime.GOOS {
	case "linux", "android":
		switch runtime.GOARCH {
		case "amd64":
			trap = 16 // SYS_IOCTL
		case "arm64", "loong64", "riscv64":
			trap = 29 // SYS_IOCTL
		case "mips", "mipsle":
			trap = 4054 // SYS_IOCTL
		case "mips64", "mips64le":
			trap = 5015 // SYS_IOCTL
		default:
			trap = 54 // SYS_IOCTL
		}
	default:
		trap = 54 // SYS_IOCTL
	}

	var req uintptr // TIOCGETA
	switch runtime.GOOS {
	case "aix":
		req = 0x5401 // TCGETS
	case "linux", "android":
		switch runtime.GOARCH {
		case "ppc64", "ppc64le":
			req = 0x402c7413 // TCGETS
		case "mips", "mipsle", "mips64", "mips64le":
			req = 0x540d // TCGETS
		default:
			req = 0x5401 // TCGETS
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			req = 0x40487413 // TIOCGETA
		default:
			req = 0x402c7413 // TIOCGETA
		}
	case "solaris":
		req = 0x540d // TCGETS
	case "zos":
		req = 3 // TCGETS
	default:
		req = 0x402c7413 // TIOCGETA
	}

	var termios [256]byte
	if runtime.GOOS == "aix" || runtime.GOOS == "solaris" || runtime.GOOS == "zos" {
		return ioctl(fd, req, uintptr(unsafe.Pointer(&termios[0]))) == nil
	}
	_, _, err := syscall.Syscall6(trap, fd, req, uintptr(unsafe.Pointer(&termios[0])), 0, 0, 0)
	return err == 0
}

// EnableVirtualTerminal is a no-op on Unix: terminal drivers and terminal
// emulators already pass ANSI/VT escape sequences through unmodified. On
// Windows it enables ConHost's VT input and output modes instead.
func EnableVirtualTerminal(stdin, stdout, stderr bool) error {
	return nil
}
