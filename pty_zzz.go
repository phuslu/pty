//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !zos

package pty

import (
	"context"
	"errors"
	"os/exec"
)

// Open returns errors.ErrUnsupported on platforms without a pty backend.
func Open() (pty, tty Pty, err error) {
	return nil, nil, errors.ErrUnsupported
}

// Start returns errors.ErrUnsupported on platforms without a pty backend.
func Start(ctx context.Context, cmd *exec.Cmd) (Pty, error) {
	return StartWithSize(ctx, cmd, nil)
}

// StartWithSize returns errors.ErrUnsupported on platforms without a pty
// backend.
func StartWithSize(ctx context.Context, cmd *exec.Cmd, size *Winsize) (Pty, error) {
	if ctx == nil {
		panic("nil Context")
	}
	return nil, errors.ErrUnsupported
}

// SetSize returns errors.ErrUnsupported on platforms without a pty backend.
func SetSize(pty Pty, size *Winsize) error {
	return errors.ErrUnsupported
}

// GetSize returns errors.ErrUnsupported on platforms without a pty backend.
func GetSize(pty Pty) (*Winsize, error) {
	return nil, errors.ErrUnsupported
}

func ioctl(fd, req, arg uintptr) error {
	return errors.ErrUnsupported
}
