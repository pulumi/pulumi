// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package archive

import (
	"os"
	"strings"
)

// newPathIgnorer creates an ignorer based that ignores either a single file (when dir is false) or
// and entire directory tree (when dir is true).
func newPathIgnorer(path string, isDir bool) ignorer {
	if !isDir {
		return &fileIgnorer{path: path}
	}

	return &directoryIgnorer{path: path + string(os.PathSeparator)}
}

type fileIgnorer struct {
	path string
}

func (fi *fileIgnorer) IsIgnored(f string) bool {
	return f == fi.path
}

type directoryIgnorer struct {
	path string
}

func (di *directoryIgnorer) IsIgnored(f string) bool {
	return strings.HasPrefix(f, di.path)
}
