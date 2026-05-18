//go:build aix || solaris

package pty

import (
	"errors"
	"syscall"
	"unsafe"
)

//go:cgo_import_dynamic libc_aix_ioctl ioctl "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_aix_posix_openpt posix_openpt "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_aix_grantpt grantpt "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_aix_unlockpt unlockpt "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_aix_ptsname ptsname "libc.a/shr_64.o"

//go:linkname aixLibcIoctl libc_aix_ioctl
//go:linkname aixLibcPosixOpenpt libc_aix_posix_openpt
//go:linkname aixLibcGrantpt libc_aix_grantpt
//go:linkname aixLibcUnlockpt libc_aix_unlockpt
//go:linkname aixLibcPtsname libc_aix_ptsname
//go:linkname syscallIoctl syscall.ioctl

var (
	aixLibcIoctl,
	aixLibcPosixOpenpt,
	aixLibcGrantpt,
	aixLibcUnlockpt,
	aixLibcPtsname uintptr
)

func aixcall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

func syscallIoctl(fd, req, arg uintptr) syscall.Errno

func aixIoctl(fd, req, arg uintptr) error {
	_, _, errno := aixcall6(uintptr(unsafe.Pointer(&aixLibcIoctl)), 3, fd, req, arg, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func aixOpenpt(flag int) (int, error) {
	fd, _, errno := aixcall6(uintptr(unsafe.Pointer(&aixLibcPosixOpenpt)), 1, uintptr(flag), 0, 0, 0, 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func aixGrant(fd int) error {
	_, _, errno := aixcall6(uintptr(unsafe.Pointer(&aixLibcGrantpt)), 1, uintptr(fd), 0, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func aixUnlock(fd int) error {
	_, _, errno := aixcall6(uintptr(unsafe.Pointer(&aixLibcUnlockpt)), 1, uintptr(fd), 0, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func aixPts(fd int) (string, error) {
	ptr, _, errno := aixcall6(uintptr(unsafe.Pointer(&aixLibcPtsname)), 1, uintptr(fd), 0, 0, 0, 0, 0)
	if ptr == 0 {
		if errno != 0 {
			return "", errno
		}
		return "", syscall.EINVAL
	}
	name, ok := stringFromNul((*[1024]byte)(unsafe.Pointer(ptr))[:])
	if !ok {
		return "", syscall.EINVAL
	}
	return name, nil
}

func solarisIoctl(fd, req, arg uintptr) error {
	errno := syscallIoctl(fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func zosCall(sys uintptr, args ...uintptr) (uintptr, error) {
	return 0, errors.ErrUnsupported
}

func zosCallPtr(sys uintptr, args ...uintptr) (uintptr, error) {
	return 0, errors.ErrUnsupported
}
