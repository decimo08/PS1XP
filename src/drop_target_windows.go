package main

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

type shellDropFormat struct {
	format uint16
	device uintptr
	aspect uint32
	index  int32
	medium uint32
}

type shellDropMedium struct {
	kind uint32
	data uintptr
	free uintptr
}

type shellDropTarget struct {
	vtable    *[7]uintptr
	refs      uint32
	accept    func() bool
	highlight func(bool)
	drop      func([]string)
	canDrop   bool
}

var (
	ole                = windows.NewLazySystemDLL("ole32.dll")
	registerShellDrop  = ole.NewProc("RegisterDragDrop")
	revokeShellDrop    = ole.NewProc("RevokeDragDrop")
	releaseShellMedium = ole.NewProc("ReleaseStgMedium")
	iidDropTarget      = windows.GUID{Data1: 0x00000122, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidUnknown         = windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	shellDropVtable    = [7]uintptr{
		windows.NewCallback(dropQuery),
		windows.NewCallback(dropAddRef),
		windows.NewCallback(dropRelease),
		windows.NewCallback(dropEnter),
		windows.NewCallback(dropOver),
		windows.NewCallback(dropLeave),
		windows.NewCallback(dropFiles),
	}
)

// Attach provides Explorer-style drag enter, leave and drop in the disc panel.
func attachShellDrop(hwnd win.HWND, accept func() bool, highlight func(bool), drop func([]string)) (func(), bool) {
	if hr := win.OleInitialize(); hr != win.S_OK && hr != win.S_FALSE {
		return func() {}, false
	}
	target := &shellDropTarget{vtable: &shellDropVtable, refs: 1, accept: accept, highlight: highlight, drop: drop}
	var pin runtime.Pinner
	pin.Pin(target)
	result, _, _ := registerShellDrop.Call(uintptr(hwnd), uintptr(unsafe.Pointer(target)))
	if int32(result) < 0 {
		pin.Unpin()
		win.OleUninitialize()
		return func() {}, false
	}
	return func() {
		revokeShellDrop.Call(uintptr(hwnd))
		pin.Unpin()
		win.OleUninitialize()
		runtime.KeepAlive(target)
	}, true
}

func dropQuery(t *shellDropTarget, id *windows.GUID, result **shellDropTarget) uintptr {
	if result == nil {
		return 0x80004003
	}
	*result = nil
	if id == nil || (*id != iidUnknown && *id != iidDropTarget) {
		return 0x80004002
	}
	*result = t
	dropAddRef(t)
	return 0
}

func dropAddRef(t *shellDropTarget) uintptr  { t.refs++; return uintptr(t.refs) }
func dropRelease(t *shellDropTarget) uintptr { t.refs--; return uintptr(t.refs) }

func shellFileFormat() shellDropFormat {
	return shellDropFormat{format: win.CF_HDROP, aspect: 1, index: -1, medium: 1}
}

func dropEnter(t *shellDropTarget, object *win.IDataObject, _, point uintptr, effect *uint32) uintptr {
	t.canDrop = false
	if object != nil {
		format := shellFileFormat()
		result, _, _ := syscall.SyscallN(object.LpVtbl.QueryGetData, uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&format)))
		t.canDrop = result == 0
	}
	return dropOver(t, 0, point, effect)
}

func dropOver(t *shellDropTarget, _, _ uintptr, effect *uint32) uintptr {
	ok := t.canDrop && t.accept() && *effect&1 != 0
	*effect = 0
	if ok {
		*effect = 1
	}
	t.highlight(ok)
	return 0
}

func dropLeave(t *shellDropTarget) uintptr {
	t.canDrop = false
	t.highlight(false)
	return 0
}

func dropFiles(t *shellDropTarget, object *win.IDataObject, _, _ uintptr, effect *uint32) uintptr {
	ok := t.canDrop && t.accept() && *effect&1 != 0
	*effect = 0
	dropLeave(t)
	if !ok || object == nil {
		return 0
	}
	format := shellFileFormat()
	var medium shellDropMedium
	result, _, _ := syscall.SyscallN(object.LpVtbl.GetData, uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))
	if result != 0 {
		return 0
	}
	defer releaseShellMedium.Call(uintptr(unsafe.Pointer(&medium)))
	handle := win.HDROP(medium.data)
	count := win.DragQueryFile(handle, 0xffffffff, nil, 0)
	if count > 1024 {
		return 0
	}
	paths := make([]string, 0, count)
	for i := uint(0); i < count; i++ {
		size := win.DragQueryFile(handle, i, nil, 0)
		if size == 0 {
			continue
		}
		buf := make([]uint16, size+1)
		win.DragQueryFile(handle, i, &buf[0], uint(len(buf)))
		paths = append(paths, windows.UTF16ToString(buf))
	}
	if len(paths) > 0 {
		*effect = 1
		t.drop(paths)
	}
	return 0
}
