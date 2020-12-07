package python

import (
	"os"
	"syscall"
)

// This is to trigger a workaround for https://github.com/golang/go/issues/42919
func needsPythonShim(pythonPath string) bool {
	info, err := os.Lstat(pythonPath)
	if err != nil {
		panic(err) // Should never happen!
	}
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return sys.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 &&
			sys.FileAttributes&syscall.FILE_ATTRIBUTE_ARCHIVE != 0
	}
	return false
}
