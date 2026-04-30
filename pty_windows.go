//go:build windows

package pty

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const maxCoord = 1<<15 - 1

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

// Start assigns a pseudo-terminal tty to cmd, starts cmd, and returns the pty
// master side. It kills cmd when ctx is done.
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

	pty, process, err := startWithSize(cmd, size)
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = process.terminate()
		case <-process.done:
		}
	}()
	return pty, nil
}

func startWithSize(cmd *exec.Cmd, size *Winsize) (*windowsPty, *windowsProcess, error) {
	pty, tty, err := newPty(size)
	if err != nil {
		return nil, nil, err
	}
	process, err := startProcess(cmd, pty, tty)
	if err != nil {
		_ = pty.Close()
		_ = tty.Close()
		return nil, nil, err
	}
	return pty, process, nil
}

type windowsProcess struct {
	handle syscall.Handle
	done   chan struct{}

	mu sync.Mutex
}

func newWindowsProcess(handle syscall.Handle) *windowsProcess {
	return &windowsProcess{handle: handle, done: make(chan struct{})}
}

func (p *windowsProcess) wait() {
	_, _ = syscall.WaitForSingleObject(p.handle, syscall.INFINITE)
	_ = p.close()
	close(p.done)
}

func (p *windowsProcess) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return nil
	}
	err := syscall.CloseHandle(p.handle)
	p.handle = 0
	return err
}

func (p *windowsProcess) terminate() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return os.ErrProcessDone
	}
	return syscall.TerminateProcess(p.handle, 1)
}

func newPty(size *Winsize) (*windowsPty, *windowsTty, error) {
	coord, err := defaultCoord(size)
	if err != nil {
		return nil, nil, err
	}

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
		p.consoleErr = closePseudoConsole(p.handle)
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
	if pty == nil {
		return syscall.EINVAL
	}
	coord, err := resizeCoord(size)
	if err != nil {
		return err
	}
	return resizePseudoConsole(syscall.Handle(pty.Fd()), coord)
}

func defaultCoord(size *Winsize) (coord, error) {
	coord := coord{X: 80, Y: 30}
	if size == nil {
		return coord, nil
	}
	if size.Cols > maxCoord || size.Rows > maxCoord {
		return coord, syscall.EINVAL
	}
	if size.Cols != 0 {
		coord.X = int16(size.Cols)
	}
	if size.Rows != 0 {
		coord.Y = int16(size.Rows)
	}
	return coord, nil
}

func resizeCoord(size *Winsize) (coord, error) {
	if size.Cols > maxCoord || size.Rows > maxCoord {
		return coord{}, syscall.EINVAL
	}
	return coord{
		X: int16(size.Cols),
		Y: int16(size.Rows),
	}, nil
}

func startProcess(cmd *exec.Cmd, pty *windowsPty, tty *windowsTty) (*windowsProcess, error) {
	if cmd.Process != nil {
		return nil, errors.New("exec: already started")
	}
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	if cmd.Path == "" {
		return nil, errors.New("exec: no command")
	}

	sys := cmd.SysProcAttr
	if sys == nil {
		sys = &syscall.SysProcAttr{}
	}

	argv0, err := lookExtensions(cmd.Path, cmd.Dir)
	if err != nil {
		return nil, err
	}
	if cmd.Dir != "" && !filepath.IsAbs(argv0) {
		argv0 = filepath.Join(cmd.Dir, argv0)
	}
	argv0p, err := syscall.UTF16PtrFromString(argv0)
	if err != nil {
		return nil, err
	}

	args := cmd.Args
	if len(args) == 0 {
		args = []string{cmd.Path}
	}
	cmdline := sys.CmdLine
	if cmdline == "" {
		cmdline = composeCommandLine(args)
	}
	var argvp *uint16
	if cmdline != "" {
		argvp, err = syscall.UTF16PtrFromString(cmdline)
		if err != nil {
			return nil, err
		}
	}

	var dirp *uint16
	if cmd.Dir != "" {
		dirp, err = syscall.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return nil, err
		}
	}

	envBlock, err := createEnvBlock(cmd.Environ())
	if err != nil {
		return nil, err
	}

	inheritedHandles := inheritedHandleList(sys)
	attrList, err := newProcThreadAttributeList(attributeCount(sys, inheritedHandles))
	if err != nil {
		return nil, err
	}
	defer attrList.delete()

	if err := attrList.update(
		procThreadAttributePseudoConsole,
		unsafe.Pointer(pty.handle),
		unsafe.Sizeof(pty.handle),
	); err != nil {
		return nil, err
	}
	if sys.ParentProcess != 0 {
		if err := attrList.update(
			procThreadAttributeParentProcess,
			unsafe.Pointer(&sys.ParentProcess),
			unsafe.Sizeof(sys.ParentProcess),
		); err != nil {
			return nil, err
		}
	}
	if len(inheritedHandles) != 0 {
		if err := attrList.update(
			procThreadAttributeHandleList,
			unsafe.Pointer(&inheritedHandles[0]),
			uintptr(len(inheritedHandles))*unsafe.Sizeof(inheritedHandles[0]),
		); err != nil {
			return nil, err
		}
	}

	startup := &startupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(*startup))
	if sys.HideWindow {
		startup.Flags |= syscall.STARTF_USESHOWWINDOW
		startup.ShowWindow = syscall.SW_HIDE
	}
	startup.ProcThreadAttributeList = attrList.data

	flags := sys.CreationFlags | syscall.CREATE_UNICODE_ENVIRONMENT | extendedStartupInfoPresent
	inheritHandles := len(inheritedHandles) != 0
	var pi syscall.ProcessInformation
	if sys.Token != 0 {
		err = syscall.CreateProcessAsUser(
			sys.Token,
			argv0p,
			argvp,
			sys.ProcessAttributes,
			sys.ThreadAttributes,
			inheritHandles,
			flags,
			&envBlock[0],
			dirp,
			&startup.StartupInfo,
			&pi,
		)
	} else {
		err = syscall.CreateProcess(
			argv0p,
			argvp,
			sys.ProcessAttributes,
			sys.ThreadAttributes,
			inheritHandles,
			flags,
			&envBlock[0],
			dirp,
			&startup.StartupInfo,
			&pi,
		)
	}
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(pi.Thread)
	runtime.KeepAlive(inheritedHandles)
	runtime.KeepAlive(sys)

	_ = tty.Close()

	process, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		_ = syscall.TerminateProcess(pi.Process, 1)
		_ = syscall.CloseHandle(pi.Process)
		return nil, err
	}
	cmd.Process = process
	windowsProcess := newWindowsProcess(pi.Process)
	go func() {
		windowsProcess.wait()
		_ = pty.closeConsole()
	}()
	return windowsProcess, nil
}

func attributeCount(sys *syscall.SysProcAttr, inheritedHandles []syscall.Handle) uint32 {
	n := uint32(1)
	if sys.ParentProcess != 0 {
		n++
	}
	if len(inheritedHandles) != 0 {
		n++
	}
	return n
}

func inheritedHandleList(sys *syscall.SysProcAttr) []syscall.Handle {
	if sys.NoInheritHandles {
		return nil
	}
	handles := make([]syscall.Handle, 0, len(sys.AdditionalInheritedHandles))
	for _, handle := range sys.AdditionalInheritedHandles {
		if handle != 0 {
			handles = append(handles, handle)
		}
	}
	return handles
}

func composeCommandLine(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, arg := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(syscall.EscapeArg(arg))
	}
	return sb.String()
}

func lookExtensions(path, dir string) (string, error) {
	if filepath.Base(path) == path {
		path = filepath.Join(".", path)
	}
	if dir == "" || filepath.VolumeName(path) != "" || filepath.IsAbs(path) {
		return exec.LookPath(path)
	}
	dirAndPath := filepath.Join(dir, path)
	lp, err := exec.LookPath(dirAndPath)
	if err != nil {
		return "", err
	}
	return path + strings.TrimPrefix(lp, dirAndPath), nil
}

func createEnvBlock(env []string) ([]uint16, error) {
	if len(env) == 0 {
		return []uint16{0, 0}, nil
	}
	env = append([]string(nil), env...)
	sort.SliceStable(env, func(i, j int) bool {
		return strings.ToUpper(envKey(env[i])) < strings.ToUpper(envKey(env[j]))
	})

	var block []uint16
	for _, kv := range env {
		if strings.IndexByte(kv, 0) >= 0 {
			return nil, errors.New("exec: environment variable contains NUL")
		}
		block = append(block, utf16.Encode([]rune(kv))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	return block, nil
}

func envKey(kv string) string {
	i := strings.IndexByte(kv, '=')
	if i == 0 {
		if j := strings.IndexByte(kv[1:], '='); j >= 0 {
			i = j + 1
		}
	}
	if i < 0 {
		return kv
	}
	return kv[:i]
}
