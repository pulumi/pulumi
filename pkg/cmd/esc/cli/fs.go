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

package cli

import (
	"io"
	"io/fs"
	"os"
)

type escFS interface {
	fs.FS

	CreateTemp(dir, pattern string) (string, io.ReadWriteCloser, error)
	ReadFile(filename string) ([]byte, error)
	Remove(name string) error
}

type defaultFS struct{}

func newFS() escFS {
	return defaultFS{}
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

func (defaultFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
