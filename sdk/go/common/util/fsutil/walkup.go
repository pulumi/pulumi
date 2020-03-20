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

package fsutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// WalkUp walks each file in path, passing the full path to `walkFn`. If walkFn returns true,
// this method returns the path that was passed to walkFn. Before visiting the parent directory,
// visitParentFn is called, if that returns false, WalkUp stops its search
func WalkUp(path string, walkFn func(string) bool, visitParentFn func(string) bool) (string, error) {
	if visitParentFn == nil {
		visitParentFn = func(dir string) bool { return true }
	}

	curr := pathDir(path)

	for {
		// visit each file
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			return "", err
		}
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if walkFn(path) {
				return path, nil
			}
		}

		// If we are at the root, stop walking
		if isTop(curr) {
			break
		}

		if !visitParentFn(curr) {
			break
		}

		// visit the parent
		curr = filepath.Dir(curr)
	}

	return "", nil
}

// pathDir returns the nearest directory to the given path (identity if a directory; parent otherwise).
func pathDir(path string) string {
	// If the path is a file, we want the directory it is in
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// isTop returns true if the path represents the top of the filesystem.
func isTop(path string) bool {
	return os.IsPathSeparator(path[len(path)-1])
}
