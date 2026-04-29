//go:build !darwin && !linux && !windows

package pty

import (
	"errors"
	"os/exec"
)

// Start returns errors.ErrUnsupported on platforms without a pty backend.
func Start(cmd *exec.Cmd) (Pty, error) {
	return nil, errors.ErrUnsupported
}

// StartWithSize returns errors.ErrUnsupported on platforms without a pty
// backend.
func StartWithSize(cmd *exec.Cmd, size *Winsize) (Pty, error) {
	return nil, errors.ErrUnsupported
}

// SetSize returns errors.ErrUnsupported on platforms without a pty backend.
func SetSize(pty Pty, size *Winsize) error {
	return errors.ErrUnsupported
}
