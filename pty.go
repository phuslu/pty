package pty

import (
	"io"
	"os"
)

// Pty is the master side of a pseudo terminal.
type Pty interface {
	io.ReadWriteCloser
	Fd() uintptr
	Name() string
}

// Winsize describes a terminal window size.
type Winsize struct {
	Rows uint16 // ws_row: Number of rows (in cells).
	Cols uint16 // ws_col: Number of columns (in cells).
	X    uint16 // ws_xpixel: Width in pixels.
	Y    uint16 // ws_ypixel: Height in pixels.
}

var _ Pty = (*os.File)(nil)
