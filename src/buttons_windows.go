package main

import (
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

type buttonStyle struct {
	button  *walk.PushButton
	surface walk.Color
	primary bool
	hover   bool
}

var (
	themedButtons         = make(map[win.HWND]*buttonStyle)
	commonControls        = windows.NewLazySystemDLL("comctl32.dll")
	setButtonSubclass     = commonControls.NewProc("SetWindowSubclass")
	removeButtonSubclass  = commonControls.NewProc("RemoveWindowSubclass")
	defaultButtonSubclass = commonControls.NewProc("DefSubclassProc")
	gdi                   = windows.NewLazySystemDLL("gdi32.dll")
	setDCBrush            = gdi.NewProc("SetDCBrushColor")
	setDCPen              = gdi.NewProc("SetDCPenColor")
	fillButtonRect        = windows.NewLazySystemDLL("user32.dll").NewProc("FillRect")
	themedButtonCallback  uintptr
)

func init() {
	themedButtonCallback = windows.NewCallback(themedButtonProc)
}

func themeButton(button *walk.PushButton, surface walk.Color, primary bool) {
	handle := button.Handle()
	if setButtonSubclass.Find() != nil {
		return
	}
	if ok, _, _ := setButtonSubclass.Call(uintptr(handle), themedButtonCallback, 1, 0); ok != 0 {
		themedButtons[handle] = &buttonStyle{button: button, surface: surface, primary: primary}
		win.InvalidateRect(handle, nil, false)
	}
}

func setButtonPrimary(button *walk.PushButton, primary bool) {
	handle := button.Handle()
	if style := themedButtons[handle]; style != nil {
		style.primary = primary
		win.InvalidateRect(handle, nil, false)
	}
}

func themedButtonProc(hwnd uintptr, message uint32, wparam, lparam, id, _ uintptr) uintptr {
	handle := win.HWND(hwnd)
	style := themedButtons[handle]
	if style != nil {
		switch message {
		case win.WM_PAINT:
			var paint win.PAINTSTRUCT
			dc := win.BeginPaint(handle, &paint)
			if dc != 0 {
				style.paint(handle, dc)
			}
			win.EndPaint(handle, &paint)
			return 0
		case win.WM_ERASEBKGND:
			return 1
		case win.WM_PRINTCLIENT:
			style.paint(handle, win.HDC(wparam))
			return 0
		case win.WM_MOUSEMOVE:
			if !style.hover {
				style.hover = true
				track := win.TRACKMOUSEEVENT{DwFlags: win.TME_LEAVE, HwndTrack: handle}
				track.CbSize = uint32(unsafe.Sizeof(track))
				win.TrackMouseEvent(&track)
				win.InvalidateRect(handle, nil, false)
			}
		case win.WM_MOUSELEAVE:
			style.hover = false
			win.InvalidateRect(handle, nil, false)
		case win.WM_NCDESTROY:
			delete(themedButtons, handle)
			removeButtonSubclass.Call(hwnd, themedButtonCallback, id)
		}
	}
	result, _, _ := defaultButtonSubclass.Call(hwnd, uintptr(message), wparam, lparam)
	switch message {
	case win.BM_SETSTATE, win.WM_ENABLE, win.WM_SETFOCUS, win.WM_KILLFOCUS, win.WM_SETTEXT:
		win.InvalidateRect(handle, nil, false)
	}
	return result
}

func (s *buttonStyle) paint(handle win.HWND, dc win.HDC) {
	var rect win.RECT
	win.GetClientRect(handle, &rect)
	bg, edge, text := walk.RGB(47, 50, 61), walk.RGB(82, 87, 106), walk.RGB(245, 243, 251)
	if s.primary {
		bg, edge = walk.RGB(116, 55, 191), walk.RGB(154, 97, 222)
	}
	switch {
	case !win.IsWindowEnabled(handle):
		bg, edge, text = walk.RGB(39, 41, 52), walk.RGB(58, 62, 76), walk.RGB(143, 148, 167)
	case win.SendMessage(handle, win.BM_GETSTATE, 0, 0)&win.BST_PUSHED != 0:
		bg, edge = walk.RGB(60, 40, 93), walk.RGB(136, 101, 193)
	case s.hover:
		if s.primary {
			bg = walk.RGB(140, 77, 218)
		} else {
			bg, edge = walk.RGB(61, 55, 82), walk.RGB(135, 102, 187)
		}
	}
	if win.GetFocus() == handle {
		edge = walk.RGB(202, 164, 250)
	}
	brush := win.GetStockObject(win.DC_BRUSH)
	setDCBrush.Call(uintptr(dc), uintptr(s.surface))
	fillButtonRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&rect)), uintptr(brush))
	oldBrush := win.SelectObject(dc, brush)
	oldPen := win.SelectObject(dc, win.GetStockObject(win.DC_PEN))
	setDCBrush.Call(uintptr(dc), uintptr(bg))
	setDCPen.Call(uintptr(dc), uintptr(edge))
	win.RoundRect(dc, 0, 0, rect.Right, rect.Bottom, 11, 11)
	win.SelectObject(dc, oldPen)
	win.SelectObject(dc, oldBrush)
	if font := win.SendMessage(handle, win.WM_GETFONT, 0, 0); font != 0 {
		oldFont := win.SelectObject(dc, win.HGDIOBJ(font))
		defer win.SelectObject(dc, oldFont)
	}
	win.SetBkMode(dc, win.TRANSPARENT)
	win.SetTextColor(dc, win.COLORREF(text))
	rect.Left += 7
	rect.Right -= 7
	name := windows.StringToUTF16(s.button.Text())
	win.DrawTextEx(dc, &name[0], int32(len(name)-1), &rect,
		win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX|win.DT_END_ELLIPSIS, nil)
}
