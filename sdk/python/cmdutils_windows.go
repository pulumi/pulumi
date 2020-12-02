package python

import (
	"os"
	"syscall"
)

func isReparsePoint(info os.FileInfo) bool {
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return sys.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 &&
			sys.FileAttributes&syscall.FILE_ATTRIBUTE_ARCHIVE != 0
	}
	return false
}
