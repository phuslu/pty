//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package pty

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// syscall only defines these ioctl constants for the target GOOS.
const (
	darwinTIOCPTYGNAME = 0x40807453
	darwinTIOCPTYGRANT = 0x20007454
	darwinTIOCPTYUNLK  = 0x20007452

	dragonflySpecNameLen  = 0x3f
	dragonflyTIOCPTMASTER = 0x20007455

	freebsdTIOCGPTN = 0x4004740f

	netbsdTIOCPTMGET    = 0x40287446
	netbsdTIOCPTMGETArm = 0x48087446

	openbsdPTMGET = 0x40287401
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
	case "dragonfly":
		return openDragonFly()
	case "freebsd":
		return openFreeBSD()
	case "netbsd":
		return openNetBSD()
	case "openbsd":
		return openOpenBSD()
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
	if name, ok := stringFromNul(name[:]); ok {
		return name, nil
	}
	return "", errors.New("pty name is not NUL-terminated")
}

func openDragonFly() (pty, tty *os.File, err error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	name, err := ptsnameDragonFly(ptmx)
	if err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func ptsnameDragonFly(file *os.File) (string, error) {
	if err := ioctl(file.Fd(), dragonflyTIOCPTMASTER, 0); err != nil {
		return "", err
	}

	var name [dragonflySpecNameLen]byte
	arg := dragonflyFiodgnameArg{
		name: (*byte)(unsafe.Pointer(&name[0])),
		len:  dragonflySpecNameLen,
	}
	if err := ioctl(file.Fd(), iow('f', 120, unsafe.Sizeof(arg)), uintptr(unsafe.Pointer(&arg))); err != nil {
		return "", err
	}
	if name, ok := stringFromNul(name[:]); ok {
		return strings.Replace("/dev/"+name, "ptm", "pts", 1), nil
	}
	return "", errors.New("pty name is not NUL-terminated")
}

type dragonflyFiodgnameArg struct {
	name *byte
	len  uint32
	pad  [4]byte
}

func openFreeBSD() (pty, tty *os.File, err error) {
	fd, err := freebsdPosixOpenpt(syscall.O_RDWR | syscall.O_CLOEXEC)
	if err != nil {
		return nil, nil, err
	}
	ptmx := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	var n uint32
	if err := ioctl(ptmx.Fd(), freebsdTIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func freebsdPosixOpenpt(flag int) (int, error) {
	fd, _, errno := syscall.Syscall(504, uintptr(flag), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func iow(group byte, num byte, size uintptr) uintptr {
	return 0x80000000 | (size << 16) | uintptr(group)<<8 | uintptr(num)
}

func openNetBSD() (pty, tty *os.File, err error) {
	ptm, err := os.OpenFile("/dev/ptm", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	defer ptm.Close()

	ptmget := ptmgetNetBSD{cfd: -1, sfd: -1}
	if err := ioctl(ptm.Fd(), netbsdTIOCPTMGETValue(), uintptr(unsafe.Pointer(&ptmget))); err != nil {
		closePtmget(ptmget.cfd, ptmget.sfd)
		return nil, nil, err
	}
	return fileFromPtmget(ptmget.cfd, ptmget.cn[:], "pty"), fileFromPtmget(ptmget.sfd, ptmget.sn[:], "tty"), nil
}

func netbsdTIOCPTMGETValue() uintptr {
	if runtime.GOARCH == "arm" {
		return netbsdTIOCPTMGETArm
	}
	return netbsdTIOCPTMGET
}

func openOpenBSD() (pty, tty *os.File, err error) {
	ptm, err := os.OpenFile("/dev/ptm", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	defer ptm.Close()

	ptmget := ptmgetOpenBSD{cfd: -1, sfd: -1}
	if err := ioctl(ptm.Fd(), openbsdPTMGET, uintptr(unsafe.Pointer(&ptmget))); err != nil {
		closePtmget(ptmget.cfd, ptmget.sfd)
		return nil, nil, err
	}
	return fileFromPtmget(ptmget.cfd, ptmget.cn[:], "pty"), fileFromPtmget(ptmget.sfd, ptmget.sn[:], "tty"), nil
}

type ptmgetNetBSD struct {
	cfd int32
	sfd int32
	cn  [1024]byte
	sn  [1024]byte
}

type ptmgetOpenBSD struct {
	cfd int32
	sfd int32
	cn  [16]byte
	sn  [16]byte
}

func closePtmget(fds ...int32) {
	for _, fd := range fds {
		if fd >= 0 {
			_ = syscall.Close(int(fd))
		}
	}
}

func fileFromPtmget(fd int32, nameBuf []byte, fallback string) *os.File {
	name, ok := stringFromNul(nameBuf)
	if !ok {
		name = fallback
	}
	return os.NewFile(uintptr(fd), name)
}

func stringFromNul(buf []byte) (string, bool) {
	for i, c := range buf {
		if c == 0 {
			return string(buf[:i]), true
		}
	}
	return "", false
}
