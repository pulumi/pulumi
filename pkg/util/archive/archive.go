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

// Package archive provides support for creating zip archives of local folders and returning the
// in-memory buffer. This is how we pass Pulumi program source to the Cloud for hosted scenarios,
// for execution in a different environment and creating the resources off of the local machine.
package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Process returns an in-memory buffer with the archived contents of the provided file path.
func Process(path string, useDefaultExcludes bool) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	// We trim `path` from the pathname of every file we add to the zip, but we actaually
	// want to ensure the files directly under `path` are not added with a path prefix,
	// so we add an extra os.PathSeparator here to the end of the string if it doesn't
	// already end with one.
	if !os.IsPathSeparator(path[len(path)-1]) {
		path = path + string(os.PathSeparator)
	}

	if err := addDirectoryToZip(writer, path, path, useDefaultExcludes, nil); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	logging.V(5).Infof("project archive is %v bytes", buffer.Len())

	return buffer, nil
}

func addDirectoryToZip(writer *zip.Writer, root string, dir string,
	useDefaultIgnores bool, ignores *ignoreState) error {
	ignoreFilePath := path.Join(dir, workspace.IgnoreFile)

	// If there is an ignorefile, process it before looking at any child paths.
	if stat, err := os.Stat(ignoreFilePath); err == nil && !stat.IsDir() {
		logging.V(9).Infof("processing ignore file in %v", dir)

		ignore, err := newPulumiIgnorerIgnorer(ignoreFilePath)
		if err != nil {
			return errors.Wrapf(err, "could not read ignore file in %v", dir)
		}

		ignores = ignores.Append(ignore)
	}

	if useDefaultIgnores {
		dotGitPath := path.Join(dir, ".git")
		if stat, err := os.Stat(dotGitPath); err == nil {
			ignores = ignores.Append(newPathIgnorer(dotGitPath, stat.IsDir()))
		}

		// If there is a package.json file here, let's build a node_modules ignorer from it.
		packageJSONFilePath := path.Join(dir, packageJSONFileName)
		if stat, err := os.Stat(packageJSONFilePath); err == nil && !stat.IsDir() {
			logging.V(9).Infof("building ignore filter from package.json in %v", dir)
			ignore, err := newNodeModulesIgnorer(packageJSONFilePath)
			if err != nil {
				return errors.Wrapf(err, "could not read ignores from package.json file in %v", dir)
			}

			ignores = ignores.Append(ignore)
		}

	}

	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	// No defer because we want to close file as soon as possible (right after we call Readdir).

	infos, err := file.Readdir(-1)
	contract.IgnoreClose(file)
	if err != nil {
		return err
	}

	for _, info := range infos {
		fullName := path.Join(dir, info.Name())

		if !info.IsDir() && ignores.IsIgnored(fullName) {
			logging.V(9).Infof("skip archiving of %v due to ignore file", fullName)
			continue
		}

		// Resolve symlinks (Readdir above calls os.Lstat which does not follow symlinks).
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			info, err = os.Stat(fullName)
			if err != nil {
				return err
			}
		}

		if info.Mode().IsDir() {
			err = addDirectoryToZip(writer, root, fullName, useDefaultIgnores, ignores)
			if err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			logging.V(9).Infof("adding %v to archive", fullName)

			w, err := writer.Create(convertPathsForZip(strings.TrimPrefix(fullName, root)))
			if err != nil {
				return err
			}

			file, err := os.Open(fullName)
			if err != nil {
				return err
			}
			// no defer because we want to close file as soon as possible (right after we call Copy)

			_, err = io.Copy(w, file)
			contract.IgnoreClose(file)
			if err != nil {
				return err
			}
		} else {
			logging.V(9).Infof("ignoring special file %v with mode %v", fullName, info.Mode())
		}
	}

	return nil
}

// convertPathsForZip ensures that '/' is uses at the path separator in zip files.
func convertPathsForZip(path string) string {
	if os.PathSeparator != '/' {
		return strings.Replace(path, string(os.PathSeparator), "/", -1)
	}

	return path
}
