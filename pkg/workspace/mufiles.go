// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/marapongo/mu/pkg/schema"
)

// DetectMufile locates the closest Mufile from the given path, searching "upwards" in the directory hierarchy.  If
// no Mufile is found, a non-nil error is returned.
func DetectMufile(from string) (string, error) {
	abs, err := filepath.Abs(from)
	if err != nil {
		return "", err
	}

	// It's possible the target is already the file we seek; if so, return right away.
	if IsMufile(abs) {
		return abs, nil
	}

	curr := abs
	for {
		// If the target is a directory, enumerate its files, checking each to see if it's a Mufile.
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			return "", err
		}
		for _, file := range files {
			path := filepath.Join(curr, file.Name())
			if IsMufile(path) {
				return path, nil
			}
		}

		// If neither succeeded, keep looking in our parent directory.
		curr = filepath.Dir(curr)
		if os.IsPathSeparator(curr[len(curr)-1]) {
			break
		}
	}

	return "", errors.New("No Mufile found")
}

// IsMufile returns true if the path references what appears to be a valid Mufile.
func IsMufile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Directories can't be Mufiles.
	if info.IsDir() {
		return false
	}

	// Check all supported extensions.
	for ext := range schema.MufileExts {
		if info.Name() == schema.MufileBase+ext {
			return true
		}
	}

	return false
}
