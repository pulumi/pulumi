// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package archive provides support for creating zip archives of local folders and returning the
// in-memory buffer. This is how we pass Pulumi program source to the Cloud for hosted scenarios,
// for execution in a different environment and creating the resources off of the local machine.
package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/sabhiram/go-gitignore"
)

// Process returns an in-memory buffer with the archived contents of the provided file path.
func Process(path string) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	// We trim `path` from the pathname of every file we add to the zip, but we actaually
	// want to ensure the files directly under `path` are not added with a path prefix,
	// so we add an extra os.PathSeparator here to the end of the string if it doesn't
	// already end with one.
	if !os.IsPathSeparator(path[len(path)-1]) {
		path = path + string(os.PathSeparator)
	}

	if err := addDirectoryToZip(writer, path, path, nil); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	glog.V(5).Infof("project archive is %v bytes", buffer.Len())

	return buffer, nil
}

func addDirectoryToZip(writer *zip.Writer, root string, dir string, ignores *ignoreState) error {
	ignoreFilePath := path.Join(dir, workspace.IgnoreFile)

	// If there is an ignorefile, process it before looking at any child paths.
	if stat, err := os.Stat(ignoreFilePath); err == nil && !stat.IsDir() {
		glog.V(9).Infof("processing ignore file in %v", dir)

		ignore, err := readIgnoreFile(ignoreFilePath)
		if err != nil {
			return errors.Wrapf(err, "could not read ignore file in %v", dir)
		}

		ignores = ignores.Append(dir, *ignore)
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
			glog.V(9).Infof("skip archiving of %v due to ignore file", fullName)
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
			// Work around an issue that will be addressed by pulumi/pulumi-ppc#95, by ensuring
			// our zip files contain directory entries instead of just having files with paths.
			// When the PPC fix is everywhere, we can delete this code in favor of just calling
			// addDirectoryToZip recursively
			zh, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			// Add a trailing slash since this is a directory.
			zh.Name = convertPathsForZip(strings.TrimPrefix(fullName, root)) + "/"

			_, err = writer.CreateHeader(zh)
			if err != nil {
				return err
			}

			err = addDirectoryToZip(writer, root, fullName, ignores)
			if err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			glog.V(9).Infof("adding %v to archive", fullName)

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
			glog.V(9).Infof("ignoring special file %v with mode %v", fullName, info.Mode())
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

func readIgnoreFile(path string) (*ignore.GitIgnore, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	patterns := []string{".git/"}
	patterns = append(patterns, strings.Split(string(buf), "\n")...)

	return ignore.CompileIgnoreLines(patterns...)
}

type ignoreState struct {
	path    string
	ignorer ignore.GitIgnore
	next    *ignoreState
}

func (s *ignoreState) Append(path string, ignorer ignore.GitIgnore) *ignoreState {
	return &ignoreState{path: path, ignorer: ignorer, next: s}
}

func (s *ignoreState) IsIgnored(path string) bool {
	if s == nil {
		return false
	}

	if s.ignorer.MatchesPath(strings.TrimPrefix(path, s.path)) {
		return true
	}

	return s.next.IsIgnored(path)
}
