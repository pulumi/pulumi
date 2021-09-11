// +build !windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func canDisplayImages() (bool, int, int) {
	if os.Getenv("TERM") != "xterm-kitty" {
		return false, 0, 0
	}

	var winsize struct {
		row, col       uint16
		xpixel, ypixel uint16
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdout.Fd()), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&winsize)))
	if err != 0 || winsize.xpixel == 0 || winsize.ypixel == 0 {
		return false, 0, 0
	}

	return true, int(winsize.xpixel), int(winsize.ypixel)
}
