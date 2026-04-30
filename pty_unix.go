//go:build linux || darwin

package pty

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

// syscall only defines these ioctl constants for the target GOOS.
const (
	darwinTIOCPTYGNAME = 0x40807453
	darwinTIOCPTYGRANT = 0x20007454
	darwinTIOCPTYUNLK  = 0x20007452
)

func open() (pty, tty *os.File, err error) {
	switch runtime.GOOS {
	case "linux":
		return openLinux()
	case "darwin":
		return openDarwin()
	default:
		return nil, nil, errors.ErrUnsupported
	}
}

func openLinux() (pty, tty *os.File, err error) {
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
	if err := ioctl(ptmx.Fd(), linuxTIOCGPTN(), uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, nil, err
	}
	var unlock int32
	if err := ioctl(ptmx.Fd(), linuxTIOCSPTLCK(), uintptr(unsafe.Pointer(&unlock))); err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func linuxTIOCGPTN() uintptr {
	switch runtime.GOARCH {
	case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
		return 0x40045430
	default:
		return 0x80045430
	}
}

func linuxTIOCSPTLCK() uintptr {
	switch runtime.GOARCH {
	case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
		return 0x80045431
	default:
		return 0x40045431
	}
}

func openDarwin() (pty, tty *os.File, err error) {
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
	if err := ioctl(ptmx.Fd(), darwinTIOCPTYGRANT, 0); err != nil {
		return nil, nil, err
	}
	if err := ioctl(ptmx.Fd(), darwinTIOCPTYUNLK, 0); err != nil {
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
	if err := ioctl(file.Fd(), darwinTIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); err != nil {
		return "", err
	}
	for i, c := range name {
		if c == 0 {
			return string(name[:i]), nil
		}
	}
	return "", errors.New("pty name is not NUL-terminated")
}
