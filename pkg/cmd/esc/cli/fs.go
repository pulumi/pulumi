// Copyright 2023, Pulumi Corporation.

package cli

import (
	"io"
	"io/fs"
	"os"

	"github.com/pulumi/esc/cmd/esc/cli/workspace"
)

type escFS interface {
	workspace.FS
	fs.FS

	CreateTemp(dir, pattern string) (string, io.ReadWriteCloser, error)
	Remove(name string) error
}

type defaultFS struct {
	workspace.FS
}

func newFS() escFS {
	return defaultFS{workspace.DefaultFS()}
}

func (defaultFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (defaultFS) CreateTemp(dir, pattern string) (string, io.ReadWriteCloser, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", nil, err
	}
	return f.Name(), f, nil
}

func (defaultFS) Remove(name string) error {
	return os.Remove(name)
}
