//go:build !windows

package tools

import "errors"

func ScreenSize() (int, int) { return 0, 0 }

func ScreenshotPNG(string) error { return errors.New("仅支持 Windows") }

func MouseMove(int, int) error { return errors.New("仅支持 Windows") }

func MouseClick(int, int, bool) error { return errors.New("仅支持 Windows") }

func Topmost(uintptr, bool) error { return errors.New("仅支持 Windows") }
