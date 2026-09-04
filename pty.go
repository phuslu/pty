// Package pty starts commands attached to pseudo terminals.
//
// On Unix platforms the master side is backed by an *os.File whose Fd is the
// pty master descriptor and whose Name is the pty device path. On Windows the
// master wraps a ConPTY: Read and Write transfer console I/O through internal
// pipes, Fd returns the ConPTY handle (an opaque HANDLE that must only be
// passed back to this package, for example to SetSize), and Name is only
// useful for diagnostics.
//
// Start runs cmd as the leader of its own process group on Unix and inside a
// job object on Windows (when the environment permits), so descendants of cmd
// do not outlive the started process after its context is done. Unsupported
// platforms compile but return errors.ErrUnsupported from Start and friends.
package pty

import (
	"io"
	"os"
)

// Pty is the master side of a pseudo terminal. See the package documentation
// for how Fd and Name behave on Unix versus Windows.
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

const (
	defaultRows = 30
	defaultCols = 80
)

// normalizeWinsize returns a copy of size with nil, zero rows, and zero
// columns filled with the conventional 80x30 default.
func normalizeWinsize(size *Winsize) *Winsize {
	if size == nil {
		size = &Winsize{}
	}
	size = &Winsize{Rows: size.Rows, Cols: size.Cols, X: size.X, Y: size.Y}
	if size.Rows == 0 {
		size.Rows = defaultRows
	}
	if size.Cols == 0 {
		size.Cols = defaultCols
	}
	return size
}
