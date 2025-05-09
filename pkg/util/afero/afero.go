// Copyright 2016-2023, Pulumi Corporation.
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

package afero

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Copies all file and directories from src to dst that match the filter
func CopyDirWhen(fs afero.Fs, src, dst string, filter func(afero.File) bool) error {
	entries, err := afero.ReadDir(fs, src)
	if err != nil {
		return err
	}
	err = fs.Mkdir(dst, 0o755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDirWhen(fs, sourcePath, destPath, filter); err != nil {
				return err
			}
		} else {
			if err := CopyWhen(fs, sourcePath, destPath, filter); err != nil {
				return err
			}
		}
	}
	return nil
}

// Copies a file from src to dst if the filter returns true
func CopyWhen(fs afero.Fs, src, dst string, filter func(afero.File) bool) error {
	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if filter != nil && !filter(in) {
		// skip copying if the file doesn't pass the filter
		return nil
	}

	out, err := fs.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

// CopyDir copies all files and directories from src to dst
func CopyDir(fs afero.Fs, src, dst string) error {
	return CopyDirWhen(fs, src, dst, func(file afero.File) bool {
		return true
	})
}

// Copy copies a file from src to dst
func Copy(fs afero.Fs, src, dst string) error {
	return CopyWhen(fs, src, dst, func(file afero.File) bool {
		return true
	})
}
