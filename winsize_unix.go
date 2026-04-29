//go:build linux || darwin

package pty

import (
	"syscall"
	"unsafe"
)

// SetSize resizes pty to size.
func SetSize(pty Pty, size *Winsize) error {
	if size == nil {
		return nil
	}
	ws := winsize{
		row:    size.Rows,
		col:    size.Cols,
		xpixel: size.X,
		ypixel: size.Y,
	}
	return ioctl(pty.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(&ws)))
}

type winsize struct {
	row    uint16
	col    uint16
	xpixel uint16
	ypixel uint16
}
