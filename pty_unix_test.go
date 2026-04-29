//go:build linux || darwin

package pty

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartConnectsStandardStreamsToPTY(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "test -t 0 && test -t 1 && test -t 2 && printf tty-ok")
	pty, err := Start(context.Background(), cmd)
	requirePTY(t, err)
	cleanupPTYCommand(t, pty, cmd)

	readUntil(t, pty, "tty-ok")
	waitForCommand(t, cmd)
}

func TestStartOverridesPresetStandardStreams(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "test -t 0 && test -t 1 && test -t 2 && printf tty-override-ok")
	cmd.Stdin = strings.NewReader("not a tty")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	pty, err := Start(context.Background(), cmd)
	requirePTY(t, err)
	cleanupPTYCommand(t, pty, cmd)

	readUntil(t, pty, "tty-override-ok")
	waitForCommand(t, cmd)
}

func TestStartKillsCommandWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("/bin/sh", "-c", "read line")
	pty, err := Start(ctx, cmd)
	requirePTY(t, err)
	cleanupPTYCommand(t, pty, cmd)

	cancel()
	waitForCommandError(t, cmd)
}

func TestStartWithSizeAndSetSize(t *testing.T) {
	if _, err := exec.LookPath("stty"); err != nil {
		t.Skipf("stty not available: %v", err)
	}

	cmd := exec.Command("/bin/sh", "-c", "stty size; read line; stty size")
	pty, err := StartWithSize(context.Background(), cmd, &Winsize{Rows: 31, Cols: 97})
	requirePTY(t, err)
	cleanupPTYCommand(t, pty, cmd)

	readUntil(t, pty, "31 97")

	if err := SetSize(pty, &Winsize{Rows: 33, Cols: 101}); err != nil {
		t.Fatalf("SetSize: %v", err)
	}
	if _, err := pty.Write([]byte("\n")); err != nil {
		t.Fatalf("write newline to pty: %v", err)
	}

	readUntil(t, pty, "33 101")
	waitForCommand(t, cmd)
}

func TestSetSizeNilPTY(t *testing.T) {
	if err := SetSize(nil, &Winsize{Rows: 1, Cols: 1}); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("SetSize error = %v, want syscall.EINVAL", err)
	}
}

func requirePTY(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.ENODEV) ||
		errors.Is(err, syscall.ENXIO) ||
		errors.Is(err, syscall.EACCES) {
		t.Skipf("pty not available: %v", err)
	}
	t.Fatalf("start pty: %v", err)
}

func cleanupPTYCommand(t *testing.T, pty Pty, cmd *exec.Cmd) {
	t.Helper()
	t.Cleanup(func() {
		_ = pty.Close()
		if cmd.Process != nil && cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
}

func readUntil(t *testing.T, r io.Reader, needle string) string {
	t.Helper()

	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)

	go func() {
		var buf bytes.Buffer
		tmp := make([]byte, 128)
		for {
			n, err := r.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
				if strings.Contains(buf.String(), needle) {
					done <- result{text: buf.String()}
					return
				}
			}
			if err != nil {
				done <- result{text: buf.String(), err: err}
				return
			}
		}
	}()

	select {
	case res := <-done:
		if !strings.Contains(res.text, needle) {
			t.Fatalf("pty output %q does not contain %q; read error: %v", res.text, needle, res.err)
		}
		return res.text
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for pty output containing %q", needle)
		return ""
	}
}

func waitForCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	err := waitForCommandResult(t, cmd)
	if err != nil {
		t.Fatalf("command wait: %v", err)
	}
}

func waitForCommandError(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	err := waitForCommandResult(t, cmd)
	if err == nil {
		t.Fatal("command wait succeeded, want error")
	}
}

func waitForCommandResult(t *testing.T, cmd *exec.Cmd) error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		err := <-done
		t.Fatalf("timeout waiting for command; after kill: %v", err)
		return nil
	}
}
