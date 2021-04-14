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

// Package archive provides support for creating .tar.gz/.tgz archives of local folders and returning the
// in-memory buffer.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// TGZ adds the contents of the provided directory to an in-memory .tar.gz/.tgz and returns the bytes.
func TGZ(dir, prefixPathInsideTar string, useDefaultExcludes bool) ([]byte, error) {
	buffer := &bytes.Buffer{}
	gw := gzip.NewWriter(buffer)
	writer := tar.NewWriter(gw)

	// We trim `dir` from the pathname of every file we add, but we actually want to ensure the files
	// directly under `path` are not added with a path prefix, so we add an extra os.PathSeparator
	// here to the end of the string if it doesn't already end with one.
	if !os.IsPathSeparator(dir[len(dir)-1]) {
		dir = dir + string(os.PathSeparator)
	}

	if err := addDirectoryToTar(writer, dir, dir, prefixPathInsideTar, useDefaultExcludes, nil); err != nil {
		return nil, err
	}

	// Close the tar and gzip writers to flush and write footers.
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	logging.V(5).Infof("project archive is %v bytes", buffer.Len())

	return buffer.Bytes(), nil
}

func extractFile(r *tar.Reader, header *tar.Header, dir string) error {
	// TODO: check the name to ensure that it does not contain path traversal characters.
	//
	//nolint: gosec
	path := filepath.Join(dir, header.Name)

	switch header.Typeflag {
	case tar.TypeDir:
		// Create any directories as needed.
		if _, err := os.Stat(path); err != nil {
			if err = os.MkdirAll(path, 0700); err != nil {
				return errors.Wrapf(err, "extracting dir %s", path)
			}
		}
	case tar.TypeReg:
		// Create any directories as needed. Some tools (notably `npm pack`) don't list
		// directories individually, so if a file is in a directory that doesn't exist, we need
		// to create it here.
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); err != nil {
			if err = os.MkdirAll(dir, 0700); err != nil {
				return errors.Wrapf(err, "extracting dir %s", dir)
			}
		}

		// Expand files into the target directory.
		dst, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return errors.Wrapf(err, "opening file %s for extraction", path)
		}
		defer contract.IgnoreClose(dst)

		// We're not concerned with potential tarbombs, so disable gosec.
		// nolint:gosec
		if _, err = io.Copy(dst, r); err != nil {
			return errors.Wrapf(err, "untarring file %s", path)
		}
	default:
		return errors.Errorf("unexpected plugin file type %s (%v)", header.Name, header.Typeflag)
	}

	return nil
}

// ExtractTGZ uncompresses a .tar.gz/.tgz file into a specific directory.
func ExtractTGZ(r io.Reader, dir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return errors.Wrapf(err, "uncompressing")
	}
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return errors.Wrapf(err, "extracting")
		}

		if err = extractFile(tr, header, dir); err != nil {
			return err
		}
	}
}

const (
	gitDir        = ".git"
	gitIgnoreFile = ".gitignore"
)

func addDirectoryToTar(writer *tar.Writer, root, dir, prefixPathInsideTar string,
	useDefaultIgnores bool, ignores *ignoreState) error {
	ignoreFilePath := filepath.Join(dir, gitIgnoreFile)

	// If there is an ignorefile, process it before looking at any child paths.
	if stat, err := os.Stat(ignoreFilePath); err == nil && !stat.IsDir() {
		logging.V(9).Infof("processing ignore file in %v", dir)

		ignore, err := newGitIgnoreIgnorer(ignoreFilePath)
		if err != nil {
			return errors.Wrapf(err, "could not read ignore file in %v", dir)
		}

		ignores = ignores.Append(ignore)
	}

	if useDefaultIgnores {
		dotGitPath := filepath.Join(dir, gitDir)
		if stat, err := os.Stat(dotGitPath); err == nil {
			ignores = ignores.Append(newPathIgnorer(dotGitPath, stat.IsDir()))
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
		fullName := filepath.Join(dir, info.Name())

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
			err = addDirectoryToTar(writer, root, fullName, prefixPathInsideTar, useDefaultIgnores, ignores)
			if err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			logging.V(9).Infof("adding %v to archive", fullName)

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			// Specify the file name, by removing the root prefix.
			// If prefixPathInsideTar is set, use it as a prefix path inside the tar (this, for example,
			// enables all files to be added in the tar file within a "packages" parent directory).
			name := strings.TrimPrefix(fullName, root)
			if prefixPathInsideTar != "" {
				name = filepath.Join(prefixPathInsideTar, name)
			}
			header.Name = filepath.ToSlash(name)

			if err := writer.WriteHeader(header); err != nil {
				return err
			}

			file, err := os.Open(fullName)
			if err != nil {
				return err
			}
			// no defer because we want to close file as soon as possible (right after we call Copy)

			_, err = io.Copy(writer, file)
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
