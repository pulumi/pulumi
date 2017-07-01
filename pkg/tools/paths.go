// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tools

import (
	"os"
	"path/filepath"
)

// EnsurePath ensures that a target filepath exists (like `mkdir -p`), returning a non-nil error if any problem occurs.
func EnsurePath(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0700)
}
