// Copyright 2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
