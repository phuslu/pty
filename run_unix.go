//go:build linux || darwin

package pty

import (
	"context"
	"os/exec"
	"syscall"
)

// Start assigns a pseudo-terminal tty to cmd's standard streams, starts cmd,
// and returns the pty master side.
func Start(cmd *exec.Cmd) (Pty, error) {
	return StartWithSize(cmd, nil)
}

// StartContext is like Start but kills cmd when ctx is done.
func StartContext(ctx context.Context, cmd *exec.Cmd) (Pty, error) {
	return StartContextWithSize(ctx, cmd, nil)
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
	attr.Ctty = 0
	return startWithAttrs(cmd, size, &attr)
}

// StartContextWithSize is like StartWithSize but kills cmd when ctx is done.
func StartContextWithSize(ctx context.Context, cmd *exec.Cmd, size *Winsize) (Pty, error) {
	if ctx == nil {
		panic("nil Context")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	pty, err := StartWithSize(cmd, size)
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	return pty, nil
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
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	return ptmx, nil
}
