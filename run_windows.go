//go:build windows

package pty

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Start assigns a pseudo-terminal tty to cmd, starts cmd, and returns the pty
// master side.
func Start(cmd *exec.Cmd) (Pty, error) {
	return StartWithSize(cmd, nil)
}

// StartWithSize starts cmd attached to a pseudo terminal with the requested
// initial size.
func StartWithSize(cmd *exec.Cmd, size *Winsize) (Pty, error) {
	pty, tty, err := newPty(size)
	if err != nil {
		return nil, err
	}
	if err := startProcess(cmd, pty, tty); err != nil {
		_ = pty.Close()
		_ = tty.Close()
		return nil, err
	}
	return pty, nil
}

func startProcess(cmd *exec.Cmd, pty *windowsPty, tty *windowsTty) error {
	if cmd.Process != nil {
		return errors.New("exec: already started")
	}
	if cmd.Err != nil {
		return cmd.Err
	}
	if cmd.Path == "" {
		return errors.New("exec: no command")
	}

	sys := cmd.SysProcAttr
	if sys == nil {
		sys = &syscall.SysProcAttr{}
	}

	argv0, err := lookExtensions(cmd.Path, cmd.Dir)
	if err != nil {
		return err
	}
	if cmd.Dir != "" && !filepath.IsAbs(argv0) {
		argv0 = filepath.Join(cmd.Dir, argv0)
	}
	argv0p, err := syscall.UTF16PtrFromString(argv0)
	if err != nil {
		return err
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
			return err
		}
	}

	var dirp *uint16
	if cmd.Dir != "" {
		dirp, err = syscall.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return err
		}
	}

	envBlock, err := createEnvBlock(cmd.Environ())
	if err != nil {
		return err
	}

	attrList, err := newProcThreadAttributeList(1)
	if err != nil {
		return err
	}
	if err := attrList.update(
		procThreadAttributePseudoConsole,
		unsafe.Pointer(pty.handle),
		unsafe.Sizeof(pty.handle),
	); err != nil {
		attrList.delete()
		return err
	}

	startup := &startupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(*startup))
	startup.Flags = syscall.STARTF_USESTDHANDLES
	if sys.HideWindow {
		startup.Flags |= syscall.STARTF_USESHOWWINDOW
		startup.ShowWindow = syscall.SW_HIDE
	}
	startup.ProcThreadAttributeList = attrList.data

	flags := sys.CreationFlags | syscall.CREATE_UNICODE_ENVIRONMENT | extendedStartupInfoPresent
	var pi syscall.ProcessInformation
	if sys.Token != 0 {
		err = syscall.CreateProcessAsUser(
			sys.Token,
			argv0p,
			argvp,
			sys.ProcessAttributes,
			sys.ThreadAttributes,
			false,
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
			false,
			flags,
			&envBlock[0],
			dirp,
			&startup.StartupInfo,
			&pi,
		)
	}
	if err != nil {
		attrList.delete()
		return err
	}
	defer syscall.CloseHandle(pi.Thread)

	process, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		attrList.delete()
		_ = syscall.CloseHandle(pi.Process)
		return err
	}
	cmd.Process = process
	go func(handle syscall.Handle, attrList *procThreadAttributeListContainer) {
		if event, err := syscall.WaitForSingleObject(handle, syscall.INFINITE); err == nil && event == syscall.WAIT_OBJECT_0 {
			_ = pty.Close()
			_ = tty.Close()
		}
		attrList.delete()
		_ = syscall.CloseHandle(handle)
	}(pi.Process, attrList)
	return nil
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
