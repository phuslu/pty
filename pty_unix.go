//go:build linux || darwin

package pty

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

// Start assigns a pseudo-terminal tty to cmd's standard streams, starts cmd,
// and returns the pty master side. It kills cmd when ctx is done.
func Start(ctx context.Context, cmd *exec.Cmd) (Pty, error) {
	return StartWithSize(ctx, cmd, nil)
}

// StartWithSize starts cmd attached to a pseudo terminal with the requested
// initial size. It kills cmd when ctx is done.
func StartWithSize(ctx context.Context, cmd *exec.Cmd, size *Winsize) (Pty, error) {
	if ctx == nil {
		panic("nil Context")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	attr := *cmd.SysProcAttr
	attr.Setsid = true
	attr.Setctty = true
	attr.Ctty = 0

	pty, err := startWithAttrs(cmd, size, &attr)
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	return pty, nil
}

func startWithAttrs(cmd *exec.Cmd, size *Winsize, attr *syscall.SysProcAttr) (Pty, error) {
	ptmx, tty, err := open()
	if err != nil {
		return nil, err
	}
	defer tty.Close()

	if size != nil {
		if err := SetSize(ptmx, size); err != nil {
			_ = ptmx.Close()
			return nil, err
		}
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	return ptmx, nil
}

// SetSize resizes pty to size.
func SetSize(pty Pty, size *Winsize) error {
	if size == nil {
		return nil
	}
	if pty == nil {
		return syscall.EINVAL
	}
	ws := winsize{
		row:    size.Rows,
		col:    size.Cols,
		xpixel: size.X,
		ypixel: size.Y,
	}
	return ioctl(pty.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(&ws)))
}

type winsize struct {
	row    uint16
	col    uint16
	xpixel uint16
	ypixel uint16
}

func ioctl(fd, req, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

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
