//go:build !darwin && !linux && !windows

package pty

import (
	"context"
	"errors"
	"os/exec"
)

// Start returns errors.ErrUnsupported on platforms without a pty backend.
func Start(cmd *exec.Cmd) (Pty, error) {
	return nil, errors.ErrUnsupported
}

// StartContext returns errors.ErrUnsupported on platforms without a pty backend.
func StartContext(ctx context.Context, cmd *exec.Cmd) (Pty, error) {
	return StartContextWithSize(ctx, cmd, nil)
}

// StartWithSize returns errors.ErrUnsupported on platforms without a pty
// backend.
func StartWithSize(cmd *exec.Cmd, size *Winsize) (Pty, error) {
	return nil, errors.ErrUnsupported
}

// StartContextWithSize returns errors.ErrUnsupported on platforms without a pty
// backend.
func StartContextWithSize(ctx context.Context, cmd *exec.Cmd, size *Winsize) (Pty, error) {
	if ctx == nil {
		panic("nil Context")
	}
	return nil, errors.ErrUnsupported
}

// SetSize returns errors.ErrUnsupported on platforms without a pty backend.
func SetSize(pty Pty, size *Winsize) error {
	return errors.ErrUnsupported
}
