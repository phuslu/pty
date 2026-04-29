//go:build linux || darwin

package pty

import (
	"os/exec"
	"syscall"
)

// Start assigns a pseudo-terminal tty to cmd's standard streams, starts cmd,
// and returns the pty master side.
func Start(cmd *exec.Cmd) (Pty, error) {
	return StartWithSize(cmd, nil)
}

// StartWithSize starts cmd attached to a pseudo terminal with the requested
// initial size.
func StartWithSize(cmd *exec.Cmd, size *Winsize) (Pty, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	attr := *cmd.SysProcAttr
	attr.Setsid = true
	attr.Setctty = true
	return startWithAttrs(cmd, size, &attr)
}

func startWithAttrs(cmd *exec.Cmd, size *Winsize, attr *syscall.SysProcAttr) (Pty, error) {
	ptmx, tty, err := open()
	if err != nil {
		return nil, err
	}
	defer tty.Close()

	if size != nil {
		if err := SetSize(ptmx, size); err != nil {
			_ = ptmx.Close()
			return nil, err
		}
	}
	if cmd.Stdin == nil {
		cmd.Stdin = tty
	}
	if cmd.Stdout == nil {
		cmd.Stdout = tty
	}
	if cmd.Stderr == nil {
		cmd.Stderr = tty
	}
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	return ptmx, nil
}
