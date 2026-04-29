//go:build windows

package pty

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestComposeCommandLine(t *testing.T) {
	args := []string{`C:\Program Files\liner.exe`, `two words`, `quote"mark`}
	got := composeCommandLine(args)
	for _, arg := range args {
		if !strings.Contains(got, syscall.EscapeArg(arg)) {
			t.Fatalf("command line %q does not contain escaped arg %q", got, syscall.EscapeArg(arg))
		}
	}
}

func TestCreateEnvBlock(t *testing.T) {
	block, err := createEnvBlock([]string{"A=B", "C=D"})
	if err != nil {
		t.Fatalf("createEnvBlock: %v", err)
	}
	if len(block) == 0 || block[len(block)-1] != 0 {
		t.Fatalf("env block is not NUL terminated: %#v", block)
	}

	_, err = createEnvBlock([]string{"A=B\x00C"})
	if err == nil {
		t.Fatal("createEnvBlock accepted an environment variable containing NUL")
	}
}

func TestCoordPack(t *testing.T) {
	got := coord{X: 80, Y: 30}.pack()
	want := uintptr(uint32(80) | uint32(30)<<16)
	if got != want {
		t.Fatalf("coord.pack() = %#x, want %#x", got, want)
	}
}

func TestHRESULTError(t *testing.T) {
	if err := hresultError(0); err != nil {
		t.Fatalf("hresultError(0): %v", err)
	}

	err := hresultError(0x80070005)
	if !errors.Is(err, syscall.Errno(5)) {
		t.Fatalf("hresultError(0x80070005) = %v, want errno 5", err)
	}
}
