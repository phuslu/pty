//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package pty

import "errors"

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
	return 0, errors.ErrUnsupported
}

func zosCallPtr(sys uintptr, args ...uintptr) (uintptr, error) {
	return 0, errors.ErrUnsupported
}
