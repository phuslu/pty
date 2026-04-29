//go:build windows

package pty

import (
	"os"
	"sync"
	"syscall"
)

type windowsPty struct {
	handle syscall.Handle
	r      *os.File
	w      *os.File

	consoleOnce sync.Once
	consoleErr  error
	closeOnce   sync.Once
	closeErr    error
}

type windowsTty struct {
	r *os.File
	w *os.File

	closeOnce sync.Once
	closeErr  error
}

func newPty(size *Winsize) (*windowsPty, *windowsTty, error) {
	ptyR, consoleW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	consoleR, ptyW, err := os.Pipe()
	if err != nil {
		_ = ptyR.Close()
		_ = consoleW.Close()
		return nil, nil, err
	}

	coord := coord{X: 80, Y: 30}
	if size != nil {
		if size.Cols != 0 {
			coord.X = int16(size.Cols)
		}
		if size.Rows != 0 {
			coord.Y = int16(size.Rows)
		}
	}

	var handle syscall.Handle
	if err := createPseudoConsole(
		coord,
		syscall.Handle(consoleR.Fd()),
		syscall.Handle(consoleW.Fd()),
		0,
		&handle,
	); err != nil {
		_ = ptyR.Close()
		_ = consoleW.Close()
		_ = consoleR.Close()
		_ = ptyW.Close()
		return nil, nil, err
	}

	return &windowsPty{
			handle: handle,
			r:      ptyR,
			w:      ptyW,
		}, &windowsTty{
			r: consoleR,
			w: consoleW,
		}, nil
}

func (p *windowsPty) Fd() uintptr {
	return uintptr(p.handle)
}

func (p *windowsPty) Name() string {
	return p.r.Name()
}

func (p *windowsPty) Read(b []byte) (int, error) {
	return p.r.Read(b)
}

func (p *windowsPty) Write(b []byte) (int, error) {
	return p.w.Write(b)
}

func (p *windowsPty) Close() error {
	p.closeOnce.Do(func() {
		p.closeErr = firstErr(
			p.closeConsole(),
			p.r.Close(),
			p.w.Close(),
		)
	})
	return p.closeErr
}

func (p *windowsPty) closeConsole() error {
	p.consoleOnce.Do(func() {
		closePseudoConsole(p.handle)
	})
	return p.consoleErr
}

func (t *windowsTty) Close() error {
	t.closeOnce.Do(func() {
		t.closeErr = firstErr(t.r.Close(), t.w.Close())
	})
	return t.closeErr
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// SetSize resizes pty to size.
func SetSize(pty Pty, size *Winsize) error {
	if size == nil {
		return nil
	}
	return resizePseudoConsole(syscall.Handle(pty.Fd()), coord{
		X: int16(size.Cols),
		Y: int16(size.Rows),
	})
}
