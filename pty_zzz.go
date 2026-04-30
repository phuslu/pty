//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package pty

import (
	"context"
	"errors"
	"os/exec"
)

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
