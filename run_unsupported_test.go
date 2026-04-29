//go:build !darwin && !linux && !windows

package pty

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestStartUnsupported(t *testing.T) {
	_, err := Start(exec.Command("test"))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Start error = %v, want errors.ErrUnsupported", err)
	}

	_, err = StartWithSize(exec.Command("test"), &Winsize{Rows: 1, Cols: 1})
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("StartWithSize error = %v, want errors.ErrUnsupported", err)
	}

	_, err = StartContext(context.Background(), exec.Command("test"))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("StartContext error = %v, want errors.ErrUnsupported", err)
	}

	_, err = StartContextWithSize(context.Background(), exec.Command("test"), &Winsize{Rows: 1, Cols: 1})
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("StartContextWithSize error = %v, want errors.ErrUnsupported", err)
	}

	if err := SetSize(nil, &Winsize{Rows: 1, Cols: 1}); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("SetSize error = %v, want errors.ErrUnsupported", err)
	}
}
