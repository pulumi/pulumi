// Copyright (c) 2019 FOSS contributors of https://github.com/nxadm/tail
//go:build !windows
// +build !windows

package tail

import (
	"os"
)

// openFile proxies a os.Open call for a file so it can be correctly tailed
// on POSIX and non-POSIX OSes like MS Windows.
func openFile(name string) (file *os.File, err error) {
	return os.Open(name)
}
