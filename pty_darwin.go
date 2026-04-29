//go:build darwin

package pty

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func open() (pty, tty *os.File, err error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	ptmx := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	name, err := ptsname(ptmx)
	if err != nil {
		return nil, nil, err
	}
	if err := ioctl(ptmx.Fd(), uintptr(syscall.TIOCPTYGRANT), 0); err != nil {
		return nil, nil, err
	}
	if err := ioctl(ptmx.Fd(), uintptr(syscall.TIOCPTYUNLK), 0); err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func ptsname(file *os.File) (string, error) {
	var name [128]byte
	if err := ioctl(file.Fd(), uintptr(syscall.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0]))); err != nil {
		return "", err
	}
	for i, c := range name {
		if c == 0 {
			return string(name[:i]), nil
		}
	}
	return "", errors.New("pty name is not NUL-terminated")
}
