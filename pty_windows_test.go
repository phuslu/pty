//go:build windows

package pty

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var errTimeout = errors.New("timeout")

func TestStartCmdEchoReadAll(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "echo pty-windows-ok")
	pty, err := Start(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pty.Close()

	output, err := readUntil(pty, "pty-windows-ok", 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("read output: %v; output=%q", err, output)
	}
	if err := waitTimeout(cmd, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("cmd.Wait: %v", err)
	}
}

func TestStartOverridesPresetStandardStreams(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "echo pty-override-ok")
	cmd.Stdin = strings.NewReader("not a tty")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	pty, err := Start(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pty.Close()

	output, err := readUntil(pty, "pty-override-ok", 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("read output: %v; output=%q", err, output)
	}
	if err := waitTimeout(cmd, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("cmd.Wait: %v", err)
	}
}

func TestStartKillsCommandWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("cmd.exe", "/d", "/c", "ping -n 60 127.0.0.1 > nul")
	pty, err := Start(ctx, cmd)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pty.Close()

	cancel()
	if err := waitTimeout(cmd, 5*time.Second); err == nil {
		t.Fatal("command wait succeeded, want error")
	}
}

func TestOpen(t *testing.T) {
	pty, tty, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		_ = tty.Close()
		_ = pty.Close()
	}()

	if pty.Fd() == 0 {
		t.Fatal("pty handle is zero")
	}

	var buf [1]byte
	if _, err := tty.Read(buf[:]); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("tty.Read error = %v, want errors.ErrUnsupported", err)
	}
	if _, err := tty.Write(buf[:]); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("tty.Write error = %v, want errors.ErrUnsupported", err)
	}
}

func TestStartCmdReadAllAfterWait(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "echo pty-tail-ok")
	pty, err := Start(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pty.Close()

	if err := waitTimeout(cmd, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("cmd.Wait: %v", err)
	}
	output, err := readAllTimeout(pty, 5*time.Second)
	if err != nil {
		t.Fatalf("read all output: %v; output=%q", err, output)
	}
	if !strings.Contains(output, "pty-tail-ok") {
		t.Fatalf("output %q does not contain pty-tail-ok", output)
	}
}

func TestStartInteractiveCmdRoundTrip(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d")
	pty, err := Start(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pty.Close()

	if _, err := pty.Write([]byte("echo pty-interactive-ok\r\nexit\r\n")); err != nil {
		t.Fatalf("write commands: %v", err)
	}

	output, err := readUntil(pty, "pty-interactive-ok", 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("read interactive output: %v; output=%q", err, output)
	}
	if err := waitTimeout(cmd, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("cmd.Wait: %v", err)
	}
}

func TestGetSizeUnsupported(t *testing.T) {
	_, err := GetSize(nil)
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("GetSize error = %v, want errors.ErrUnsupported", err)
	}
}

func readAllTimeout(pty Pty, timeout time.Duration) (string, error) {
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := io.ReadAll(pty)
		done <- result{output: string(output), err: err}
	}()
	select {
	case res := <-done:
		return res.output, res.err
	case <-time.After(timeout):
		return "", errTimeout
	}
}

func readUntil(pty Pty, want string, timeout time.Duration) (string, error) {
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		buf := make([]byte, 256)
		var output strings.Builder
		for {
			n, err := pty.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
				if strings.Contains(output.String(), want) {
					done <- result{output: output.String()}
					return
				}
			}
			if err != nil {
				done <- result{output: output.String(), err: err}
				return
			}
		}
	}()
	select {
	case res := <-done:
		return res.output, res.err
	case <-time.After(timeout):
		return "", errTimeout
	}
}

func waitTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return errTimeout
	}
}
