//nolint:goheader // This file is a vendored copy of the tail package from, so we keep the original license header.
// Copyright (c) 2024, Pulumi Corporation.
// Copyright (c) 2019 FOSS contributors of https://github.com/nxadm/tail
//go:build windows
// +build windows

package tail

import (
	"os"

	"github.com/nxadm/tail/winfile"
)

// openFile proxies a os.Open call for a file so it can be correctly tailed
// on POSIX and non-POSIX OSes like MS Windows.
func openFile(name string) (file *os.File, err error) {
	return winfile.OpenFile(name, os.O_RDONLY, 0)
}
