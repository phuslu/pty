//go:build !windows

package pty

import "io"

// NewConsoleANSIWriter returns w unchanged on non-Windows platforms, or
// io.Discard when w is nil.
func NewConsoleANSIWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
