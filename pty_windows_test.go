//go:build windows

package pty

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var errTimeout = errors.New("timeout")

func TestStartCmdEchoReadAll(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "echo pty-windows-ok")
	pty, err := Start(cmd)
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

func TestStartInteractiveCmdRoundTrip(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d")
	pty, err := Start(cmd)
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
