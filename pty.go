package pty

import "io"

// Pty is the master side of a pseudo terminal.
type Pty interface {
	io.ReadWriteCloser
	Fd() uintptr
	Name() string
}

// Winsize describes a terminal window size.
type Winsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}
