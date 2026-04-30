//go:build windows

package pty

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	extendedStartupInfoPresent       = 0x00080000
	procThreadAttributeParentProcess = 0x00020000
	procThreadAttributeHandleList    = 0x00020002
	procThreadAttributePseudoConsole = 0x00020016
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procClosePseudoConsole                = kernel32.NewProc("ClosePseudoConsole")
	procCreatePseudoConsole               = kernel32.NewProc("CreatePseudoConsole")
	procDeleteProcThreadAttributeList     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procInitializeProcThreadAttributeList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procLocalAlloc                        = kernel32.NewProc("LocalAlloc")
	procResizePseudoConsole               = kernel32.NewProc("ResizePseudoConsole")
	procUpdateProcThreadAttribute         = kernel32.NewProc("UpdateProcThreadAttribute")
)

type coord struct {
	X int16
	Y int16
}

func (c coord) pack() uintptr {
	return uintptr(uint32(uint16(c.X)) | uint32(uint16(c.Y))<<16)
}

func createPseudoConsole(size coord, in, out syscall.Handle, flags uint32, console *syscall.Handle) error {
	if err := procCreatePseudoConsole.Find(); err != nil {
		return errors.Join(errors.ErrUnsupported, err)
	}
	r0, _, _ := procCreatePseudoConsole.Call(
		size.pack(),
		uintptr(in),
		uintptr(out),
		uintptr(flags),
		uintptr(unsafe.Pointer(console)),
	)
	return hresultError(r0)
}

func closePseudoConsole(console syscall.Handle) error {
	if err := procClosePseudoConsole.Find(); err != nil {
		return errors.Join(errors.ErrUnsupported, err)
	}
	// ClosePseudoConsole is a void API; there is no HRESULT to inspect here.
	procClosePseudoConsole.Call(uintptr(console))
	return nil
}

func resizePseudoConsole(console syscall.Handle, size coord) error {
	if err := procResizePseudoConsole.Find(); err != nil {
		return errors.Join(errors.ErrUnsupported, err)
	}
	r0, _, _ := procResizePseudoConsole.Call(uintptr(console), size.pack())
	return hresultError(r0)
}

func hresultError(r0 uintptr) error {
	if int32(r0) >= 0 {
		return nil
	}
	if r0&0x1fff0000 == 0x00070000 {
		r0 &= 0xffff
	}
	return syscall.Errno(r0)
}

type procThreadAttributeList struct{}

type procThreadAttributeListContainer struct {
	data     *procThreadAttributeList
	pointers []unsafe.Pointer
}

func newProcThreadAttributeList(maxAttrCount uint32) (*procThreadAttributeListContainer, error) {
	var size uintptr
	err := initializeProcThreadAttributeList(nil, maxAttrCount, 0, &size)
	if err != syscall.ERROR_INSUFFICIENT_BUFFER {
		if err == nil {
			return nil, errors.New("unable to query buffer size from InitializeProcThreadAttributeList")
		}
		return nil, err
	}

	alloc, err := localAlloc(0, uint32(size))
	if err != nil {
		return nil, err
	}
	al := &procThreadAttributeListContainer{data: (*procThreadAttributeList)(unsafe.Pointer(alloc))}
	if err := initializeProcThreadAttributeList(al.data, maxAttrCount, 0, &size); err != nil {
		syscall.LocalFree(syscall.Handle(unsafe.Pointer(al.data)))
		return nil, err
	}
	al.pointers = make([]unsafe.Pointer, 0, maxAttrCount)
	return al, nil
}

func (al *procThreadAttributeListContainer) update(attribute uintptr, value unsafe.Pointer, size uintptr) error {
	al.pointers = append(al.pointers, value)
	return updateProcThreadAttribute(al.data, 0, attribute, value, size, nil, nil)
}

func (al *procThreadAttributeListContainer) delete() {
	if al == nil || al.data == nil {
		return
	}
	deleteProcThreadAttributeList(al.data)
	syscall.LocalFree(syscall.Handle(unsafe.Pointer(al.data)))
	al.data = nil
	al.pointers = nil
}

type startupInfoEx struct {
	syscall.StartupInfo
	ProcThreadAttributeList *procThreadAttributeList
}

func initializeProcThreadAttributeList(attrList *procThreadAttributeList, attrCount uint32, flags uint32, size *uintptr) error {
	r1, _, err := procInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(attrList)),
		uintptr(attrCount),
		uintptr(flags),
		uintptr(unsafe.Pointer(size)),
	)
	if r1 == 0 {
		return errnoError(err)
	}
	return nil
}

func deleteProcThreadAttributeList(attrList *procThreadAttributeList) {
	procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(attrList)))
}

func updateProcThreadAttribute(attrList *procThreadAttributeList, flags uint32, attr uintptr, value unsafe.Pointer, size uintptr, prevValue unsafe.Pointer, returnedSize *uintptr) error {
	r1, _, err := procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(attrList)),
		uintptr(flags),
		attr,
		uintptr(value),
		size,
		uintptr(prevValue),
		uintptr(unsafe.Pointer(returnedSize)),
	)
	if r1 == 0 {
		return errnoError(err)
	}
	return nil
}

func localAlloc(flags uint32, length uint32) (uintptr, error) {
	r0, _, err := procLocalAlloc.Call(uintptr(flags), uintptr(length))
	if r0 == 0 {
		return 0, errnoError(err)
	}
	return r0, nil
}

func errnoError(err error) error {
	if err == syscall.Errno(0) {
		return syscall.EINVAL
	}
	return err
}
