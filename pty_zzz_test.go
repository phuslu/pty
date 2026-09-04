//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !zos

package pty

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestStartUnsupported(t *testing.T) {
	pty, tty, err := Open()
	if pty != nil || tty != nil || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Open = (%v, %v, %v), want errors.ErrUnsupported", pty, tty, err)
	}

	_, err = Start(context.Background(), exec.Command("test"))
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

	_, err = GetSize(nil)
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("GetSize error = %v, want errors.ErrUnsupported", err)
	}
}
