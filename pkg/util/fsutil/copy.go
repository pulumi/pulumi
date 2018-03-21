// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package fsutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// CopyFile is a braindead simple function that copies a src file to a dst file.  Note that it is not general purpose:
// it doesn't handle symbolic links, it doesn't try to be efficient, it doesn't handle copies where src and dst overlap,
// and it makes no attempt to preserve file permissions.  It is what we need for this utility package, no more, no less.
func CopyFile(dst string, src string, excl map[string]bool) error {
	info, err := os.Lstat(src)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if excl[info.Name()] {
		return nil
	}

	if info.IsDir() {
		// Recursively copy all files in a directory.
		files, err := ioutil.ReadDir(src)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := file.Name()
			copyerr := CopyFile(filepath.Join(dst, name), filepath.Join(src, name), excl)
			if copyerr != nil {
				return copyerr
			}
		}
	} else if info.Mode().IsRegular() {
		// Copy files by reading and rewriting their contents.  Skip symlinks and other special files.
		data, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		dstdir := filepath.Dir(dst)
		if err = os.MkdirAll(dstdir, 0700); err != nil {
			return err
		}
		if err = ioutil.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}
