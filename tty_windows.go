//go:build windows

package pty

import (
	"syscall"
)

// IsTerminal return true if the file descriptor is terminal.
func IsTerminal(fd uintptr) bool {
	var mode uint32
	err := syscall.GetConsoleMode(syscall.Handle(fd), &mode)
	if err != nil {
		return false
	}

	return true
}
