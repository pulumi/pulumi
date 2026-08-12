// Copyright 2024, Pulumi Corporation.
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

package newcmd

import (
	"fmt"
	"os"
)

// Ensure the directory exists and uses it as the current working
// directory.
func UseSpecifiedDir(dir string) (string, error) {
	// Ensure the directory exists.
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("creating the directory: %w", err)
	}

	// Change the working directory to the specified directory.
	if err := os.Chdir(dir); err != nil {
		return "", fmt.Errorf("changing the working directory: %w", err)
	}

	// Get the new working directory.
	var cwd string
	var err error
	if cwd, err = os.Getwd(); err != nil {
		return "", fmt.Errorf("getting the working directory: %w", err)
	}
	return cwd, nil
}

// File or directory names that are considered invisible
// when considering whether a directory is empty.
var invisibleDirEntries = map[string]struct{}{
	".git": {},
	".hg":  {},
	".bzr": {},
}

// ErrorIfNotEmptyDirectory returns an error if path is not empty.
func ErrorIfNotEmptyDirectory(path string) error {
	infos, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var nonEmpty bool
	for _, info := range infos {
		if _, ignore := invisibleDirEntries[info.Name()]; ignore {
			continue
		}
		nonEmpty = true
		break
	}

	if nonEmpty {
		return fmt.Errorf("%s is not empty; "+
			"use --force to continue and overwrite existing files, or use --dir to specify an empty directory.", path)
	}

	return nil
}
