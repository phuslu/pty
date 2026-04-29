//go:build linux

package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

func open() (pty, tty *os.File, err error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	var n uint32
	if err := ioctl(ptmx.Fd(), uintptr(syscall.TIOCGPTN), uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, nil, err
	}
	var unlock int32
	if err := ioctl(ptmx.Fd(), uintptr(syscall.TIOCSPTLCK), uintptr(unsafe.Pointer(&unlock))); err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}
