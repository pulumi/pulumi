// Copyright 2016-2018, Pulumi Corporation.
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
