// Copyright 2023, Pulumi Corporation. All rights reserved.

package workspace

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/lockedfile"
)

type FS interface {
	MkdirAll(path string, perm fs.FileMode) error

	LockedRead(name string) ([]byte, error)
	LockedWrite(name string, content io.Reader, perm os.FileMode) error
}

type defaultFS int

func (defaultFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(filepath.FromSlash(path), perm)
}

func (defaultFS) LockedRead(name string) ([]byte, error) {
	return lockedfile.Read(filepath.FromSlash(name))
}

func (defaultFS) LockedWrite(name string, content io.Reader, perm os.FileMode) error {
	return lockedfile.Write(filepath.FromSlash(name), content, perm)
}

func DefaultFS() FS {
	return defaultFS(0)
}

type Workspace struct {
	fs     FS
	pulumi PulumiWorkspace
}

func New(fs FS, pulumi PulumiWorkspace) *Workspace {
	return &Workspace{fs: fs, pulumi: pulumi}
}
