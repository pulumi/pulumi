// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tools

import (
	"os"
	"path/filepath"
)

// EnsureDir ensures that a target directory exists (like `mkdir -p`), returning a non-nil error if any problem occurs.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0700)
}

// EnsureFileDir ensures that a target file's parent directory exists, returning a non-nil error if any problem occurs.
func EnsureFileDir(path string) error {
	return EnsureDir(filepath.Dir(path))
}
