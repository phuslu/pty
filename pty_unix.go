//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

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
	aixTIOCGWINSZ = 0x40087468
	aixTIOCSWINSZ = ^uintptr(0x7ff78b98)

	darwinTIOCPTYGNAME = 0x40807453
	darwinTIOCPTYGRANT = 0x20007454
	darwinTIOCPTYUNLK  = 0x20007452

	dragonflySpecNameLen  = 0x3f
	dragonflyTIOCPTMASTER = 0x20007455

	freebsdTIOCGPTN = 0x4004740f

	linuxTIOCGWINSZ      = 0x5413
	linuxTIOCSWINSZ      = 0x5414
	linuxTIOCGWINSZBSD   = 0x40087468
	linuxTIOCSWINSZBSD   = 0x80087467
	linuxSyscallIOCTL    = 54
	linuxSyscallIOCTL64  = 29
	linuxSyscallIOCTLX   = 16
	linuxSyscallIOCTLM   = 4054
	linuxSyscallIOCTLM64 = 5015

	netbsdTIOCPTMGET    = 0x40287446
	netbsdTIOCPTMGETArm = 0x48087446

	openbsdPTMGET = 0x40287401

	solarisTIOCGWINSZ = 0x5468
	solarisTIOCSWINSZ = 0x5467
	solarisIPush      = uintptr((int32('S') << 8) | 002)
	solarisIStr       = uintptr((int32('S') << 8) | 010)
	solarisIFind      = uintptr((int32('S') << 8) | 013)
	solarisISPTM      = (int32('P') << 8) | 1
	solarisUNLKPT     = (int32('P') << 8) | 2
	solarisOWNERPT    = (int32('P') << 8) | 5

	ttyTIOCGWINSZ = 0x40087468
	ttyTIOCSWINSZ = 0x80087467

	zosSYSIOCTL       = 0x355
	zosSYSGrantpt     = 0x37a
	zosSYSUnlockpt    = 0x37b
	zosSYSPosixOpenpt = 0xc66
	zosSYSFCNTL       = 0x18c
	zosSYSPtsnameA    = 0x718
	zosTIOCGWINSZ     = 0x4008a368
	zosTIOCSWINSZ     = 0x8008a367
	zosORDWR          = 0x03
	zosONOCTTY        = 0x20
	zosFControlCVT    = 13
	zosSetCVTOn       = 1
)

// Open a pty and its corresponding tty.
func Open() (pty, tty Pty, err error) {
	return open()
}

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
	if process := cmd.Process; ctx.Done() != nil && process != nil {
		context.AfterFunc(ctx, func() {
			_ = process.Kill()
		})
	}
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
	var req uintptr
	switch runtime.GOOS {
	case "aix":
		req = aixTIOCSWINSZ
	case "linux":
		switch runtime.GOARCH {
		case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
			req = linuxTIOCSWINSZBSD
		default:
			req = linuxTIOCSWINSZ
		}
	case "solaris":
		req = solarisTIOCSWINSZ
	case "zos":
		req = zosTIOCSWINSZ
	default:
		req = ttyTIOCSWINSZ
	}
	return ioctl(pty.Fd(), req, uintptr(unsafe.Pointer(&ws)))
}

// GetSize returns pty's current terminal window size.
func GetSize(pty Pty) (*Winsize, error) {
	if pty == nil {
		return nil, syscall.EINVAL
	}
	var ws winsize
	var req uintptr
	switch runtime.GOOS {
	case "aix":
		req = aixTIOCGWINSZ
	case "linux":
		switch runtime.GOARCH {
		case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
			req = linuxTIOCGWINSZBSD
		default:
			req = linuxTIOCGWINSZ
		}
	case "solaris":
		req = solarisTIOCGWINSZ
	case "zos":
		req = zosTIOCGWINSZ
	default:
		req = ttyTIOCGWINSZ
	}
	if err := ioctl(pty.Fd(), req, uintptr(unsafe.Pointer(&ws))); err != nil {
		return nil, err
	}
	return &Winsize{
		Rows: ws.row,
		Cols: ws.col,
		X:    ws.xpixel,
		Y:    ws.ypixel,
	}, nil
}

type winsize struct {
	row    uint16
	col    uint16
	xpixel uint16
	ypixel uint16
}

func open() (pty, tty *os.File, err error) {
	switch runtime.GOOS {
	case "aix":
		return openAIX()
	case "darwin":
		return openDarwin()
	case "dragonfly":
		return openDragonFly()
	case "freebsd":
		return openFreeBSD()
	case "linux":
		return openLinux()
	case "netbsd":
		return openNetBSD()
	case "openbsd":
		return openOpenBSD()
	case "solaris":
		return openSolaris()
	case "zos":
		return openZOS()
	default:
		return nil, nil, errors.ErrUnsupported
	}
}

func ioctl(fd, req, arg uintptr) error {
	switch runtime.GOOS {
	case "aix":
		return aixIoctl(fd, req, arg)
	case "solaris":
		return solarisIoctl(fd, req, arg)
	case "zos":
		_, err := zosCall(zosSYSIOCTL, fd, req, arg)
		return err
	case "linux":
		var trap uintptr
		switch runtime.GOARCH {
		case "amd64":
			trap = linuxSyscallIOCTLX
		case "arm64", "loong64", "riscv64":
			trap = linuxSyscallIOCTL64
		case "mips", "mipsle":
			trap = linuxSyscallIOCTLM
		case "mips64", "mips64le":
			trap = linuxSyscallIOCTLM64
		default:
			trap = linuxSyscallIOCTL
		}
		_, _, errno := syscall.Syscall(trap, fd, req, arg)
		if errno != 0 {
			return errno
		}
		return nil
	default:
		_, _, errno := syscall.Syscall(linuxSyscallIOCTL, fd, req, arg)
		if errno != 0 {
			return errno
		}
		return nil
	}
}

func openAIX() (pty, tty *os.File, err error) {
	fd, err := aixOpenpt(syscall.O_RDWR | syscall.O_NOCTTY)
	if err != nil {
		return nil, nil, err
	}
	syscall.CloseOnExec(fd)

	ptmx := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	if err := aixGrant(fd); err != nil {
		return nil, nil, err
	}
	if err := aixUnlock(fd); err != nil {
		return nil, nil, err
	}

	name, err := aixPts(fd)
	if err != nil {
		return nil, nil, err
	}
	tty, err = os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func openSolaris() (pty, tty *os.File, err error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	syscall.CloseOnExec(fd)

	ptmx := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	name, err := ptsnameSolaris(ptmx)
	if err != nil {
		return nil, nil, err
	}
	if err := grantptSolaris(ptmx); err != nil {
		return nil, nil, err
	}
	if err := unlockptSolaris(ptmx); err != nil {
		return nil, nil, err
	}

	fd, err = syscall.Open(name, syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	syscall.CloseOnExec(fd)

	tty = os.NewFile(uintptr(fd), name)
	defer func() {
		if err != nil {
			_ = tty.Close()
		}
	}()

	for _, mod := range []string{"ptem", "ldterm", "ttcompat"} {
		if err := streamsPushSolaris(tty, mod); err != nil {
			return nil, nil, err
		}
	}
	return ptmx, tty, nil
}

func ptsnameSolaris(file *os.File) (string, error) {
	dev, err := ptsdevSolaris(file)
	if err != nil {
		return "", err
	}
	name := "/dev/pts/" + strconv.FormatInt(int64(dev), 10)
	if err := syscall.Access(name, 0); err != nil {
		return "", err
	}
	return name, nil
}

func ptsdevSolaris(file *os.File) (uint64, error) {
	istr := solarisStrioctl{
		cmd: solarisISPTM,
	}
	if err := ioctl(file.Fd(), solarisIStr, uintptr(unsafe.Pointer(&istr))); err != nil {
		return 0, err
	}

	var stat syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Rdev) & 0377, nil
}

func grantptSolaris(file *os.File) error {
	if _, err := ptsdevSolaris(file); err != nil {
		return err
	}
	owner := solarisPtOwn{
		uid: int32(os.Getuid()),
		gid: int32(os.Getgid()),
	}
	istr := solarisStrioctl{
		cmd: solarisOWNERPT,
		len: int32(unsafe.Sizeof(solarisStrioctl{})),
		dp:  unsafe.Pointer(&owner),
	}
	return ioctl(file.Fd(), solarisIStr, uintptr(unsafe.Pointer(&istr)))
}

func unlockptSolaris(file *os.File) error {
	istr := solarisStrioctl{
		cmd: solarisUNLKPT,
	}
	return ioctl(file.Fd(), solarisIStr, uintptr(unsafe.Pointer(&istr)))
}

func streamsPushSolaris(file *os.File, mod string) error {
	buf := append([]byte(mod), 0)
	if err := ioctl(file.Fd(), solarisIFind, uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		return nil
	}
	return ioctl(file.Fd(), solarisIPush, uintptr(unsafe.Pointer(&buf[0])))
}

type solarisStrioctl struct {
	cmd     int32
	timeout int32
	len     int32
	dp      unsafe.Pointer
}

type solarisPtOwn struct {
	uid int32
	gid int32
}

func openZOS() (pty, tty *os.File, err error) {
	r0, err := zosCall(zosSYSPosixOpenpt, zosORDWR|zosONOCTTY)
	if err != nil {
		return nil, nil, err
	}
	fd := int(r0)

	cvt := zosFCnvrt{cvtcmd: zosSetCVTOn, fccsid: 1047}
	if _, err = zosCall(zosSYSFCNTL, uintptr(fd), zosFControlCVT, uintptr(unsafe.Pointer(&cvt))); err != nil {
		_ = syscall.Close(fd)
		return nil, nil, err
	}

	ptmx := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			_ = ptmx.Close()
		}
	}()

	r0, err = zosCallPtr(zosSYSPtsnameA, uintptr(fd))
	if err != nil {
		return nil, nil, err
	}
	if r0 == 0 {
		return nil, nil, syscall.EINVAL
	}
	name, ok := stringFromNul((*[1024]byte)(unsafe.Pointer(r0))[:])
	if !ok {
		return nil, nil, syscall.EINVAL
	}

	if _, err = zosCall(zosSYSGrantpt, uintptr(fd)); err != nil {
		return nil, nil, err
	}
	if _, err = zosCall(zosSYSUnlockpt, uintptr(fd)); err != nil {
		return nil, nil, err
	}

	fd, err = syscall.Open(name, zosORDWR|zosONOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	if _, err = zosCall(zosSYSFCNTL, uintptr(fd), zosFControlCVT, uintptr(unsafe.Pointer(&cvt))); err != nil {
		_ = syscall.Close(fd)
		return nil, nil, err
	}

	tty = os.NewFile(uintptr(fd), name)
	return ptmx, tty, nil
}

type zosFCnvrt struct {
	cvtcmd int32
	pccsid int16
	fccsid int16
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
	var req uintptr
	switch runtime.GOARCH {
	case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
		req = 0x40045430
	default:
		req = 0x80045430
	}
	if err := ioctl(ptmx.Fd(), req, uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, nil, err
	}
	var unlock int32
	switch runtime.GOARCH {
	case "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le":
		req = 0x80045431
	default:
		req = 0x40045431
	}
	if err := ioctl(ptmx.Fd(), req, uintptr(unsafe.Pointer(&unlock))); err != nil {
		return nil, nil, err
	}

	tty, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
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
