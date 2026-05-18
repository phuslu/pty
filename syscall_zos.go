//go:build zos

package pty

import (
	"errors"
	"runtime"
	"syscall"
)

func aixIoctl(fd, req, arg uintptr) error {
	return errors.ErrUnsupported
}

func aixOpenpt(flag int) (int, error) {
	return 0, errors.ErrUnsupported
}

func aixGrant(fd int) error {
	return errors.ErrUnsupported
}

func aixUnlock(fd int) error {
	return errors.ErrUnsupported
}

func aixPts(fd int) (string, error) {
	return "", errors.ErrUnsupported
}

func solarisIoctl(fd, req, arg uintptr) error {
	return errors.ErrUnsupported
}

func zosCall(sys uintptr, args ...uintptr) (uintptr, error) {
	r0, _, e1 := runtime.CallLeFuncWithErr(runtime.GetZosLibVec()+(sys<<4), args...)
	if e1 != 0 {
		return 0, syscall.Errno(e1)
	}
	return r0, nil
}

func zosCallPtr(sys uintptr, args ...uintptr) (uintptr, error) {
	r0, _, e1 := runtime.CallLeFuncWithPtrReturn(runtime.GetZosLibVec()+(sys<<4), args...)
	if e1 != 0 {
		return 0, syscall.Errno(e1)
	}
	return r0, nil
}
