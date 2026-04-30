//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package pty

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestStartUnsupported(t *testing.T) {
	_, err := Start(context.Background(), exec.Command("test"))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Start error = %v, want errors.ErrUnsupported", err)
	}

	_, err = StartWithSize(context.Background(), exec.Command("test"), &Winsize{Rows: 1, Cols: 1})
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("StartWithSize error = %v, want errors.ErrUnsupported", err)
	}

	if err := SetSize(nil, &Winsize{Rows: 1, Cols: 1}); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("SetSize error = %v, want errors.ErrUnsupported", err)
	}
}
