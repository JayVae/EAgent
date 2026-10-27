//go:build windows

// Package tools 提供本机工具实现：截屏、鼠标控制、窗口置顶。
// 全部通过 syscall 调用系统 API，坐标统一为物理像素(进程启动即 DPI aware)。
package tools

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procSendInput          = user32.NewProc("SendInput")
	procSetCursorPos       = user32.NewProc("SetCursorPos")
	procSetWindowPos       = user32.NewProc("SetWindowPos")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

func init() {
	procSetProcessDPIAware.Call()
}

func ScreenSize() (int, int) {
	w, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
	h, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	return int(int32(w)), int(int32(h))
}

func ScreenshotPNG(path string) error {
	w, h := ScreenSize()
	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)
	hmem, _, _ := procCreateCompatibleDC.Call(hdc)
	if hmem == 0 {
		return fmt.Errorf("CreateCompatibleDC 失败")
	}
	defer procDeleteDC.Call(hmem)
	hbmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	if hbmp == 0 {
		return fmt.Errorf("CreateCompatibleBitmap 失败")
	}
	defer procDeleteObject.Call(hbmp)
	procSelectObject.Call(hmem, hbmp)
	const SRCCOPY = 0x00CC0020
	if r, _, _ := procBitBlt.Call(hmem, 0, 0, uintptr(w), uintptr(h), hdc, 0, 0, SRCCOPY); r == 0 {
		return fmt.Errorf("BitBlt 失败")
	}

	type bmiHeader struct {
		Size          uint32
		Width         int32
		Height        int32 // 负值 = 自顶向下
		Planes        uint16
		BitCount      uint16
		Compression   uint32
		SizeImage     uint32
		XPelsPerMeter int32
		YPelsPerMeter int32
		ClrUsed       uint32
		ClrImportant  uint32
	}
	bi := bmiHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	buf := make([]byte, w*h*4)
	if r, _, _ := procGetDIBits.Call(hmem, hbmp, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0); r == 0 {
		return fmt.Errorf("GetDIBits 失败")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4+0] = buf[i*4+2] // BGRA -> RGBA
		img.Pix[i*4+1] = buf[i*4+1]
		img.Pix[i*4+2] = buf[i*4+0]
		img.Pix[i*4+3] = 0xFF
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

type mouseInput struct {
	DX, DY    int32
	MouseData uint32
	DWFlags   uint32
	Time      uint32
	ExtraInfo uintptr
}

func sendMouse(flags uint32) error {
	var in struct {
		Type uint32
		_    uint32
		MI   mouseInput
	}
	in.Type = 0 // INPUT_MOUSE
	in.MI.DWFlags = flags
	r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if r == 0 {
		return fmt.Errorf("SendInput 失败: %v", err)
	}
	return nil
}

func MouseMove(x, y int) error {
	if r, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y)); r == 0 {
		return fmt.Errorf("SetCursorPos 失败: %v", err)
	}
	return nil
}

func MouseClick(x, y int, right bool) error {
	if err := MouseMove(x, y); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	down, up := uint32(0x0002), uint32(0x0004) // LEFTDOWN/LEFTUP
	if right {
		down, up = 0x0008, 0x0010 // RIGHTDOWN/RIGHTUP
	}
	if err := sendMouse(down); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return sendMouse(up)
}

func Topmost(hwnd uintptr, on bool) error {
	flag := ^uintptr(1) // HWND_NOTOPMOST(-2)
	if on {
		flag = ^uintptr(0) // HWND_TOPMOST(-1)
	}
	const SWP_NOSIZE = 0x0001
	const SWP_NOMOVE = 0x0002
	const SWP_SHOWWINDOW = 0x0040
	r, _, err := procSetWindowPos.Call(hwnd, flag, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	if r == 0 {
		return fmt.Errorf("SetWindowPos 失败(句柄可能无效): %v", err)
	}
	return nil
}
