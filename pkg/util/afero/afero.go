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
func CopyDir(fs afero.Fs, src, dst string, filter func(os.FileInfo) bool) error {
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
			if err := CopyDir(fs, sourcePath, destPath, filter); err != nil {
				return err
			}
		} else {
			if filter != nil && !filter(entry) {
				continue
			}
			if err := Copy(fs, sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// Copies a file from src to dst
func Copy(fs afero.Fs, src, dst string) error {
	out, err := fs.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}
