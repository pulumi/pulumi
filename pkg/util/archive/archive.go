// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package archive provides support for creating zip archives of local folders and returning them
// as string. (Which may be rather large.) This is how we pass Pulumi program source to the Cloud
// for hosted scenarios, so the program can execute in a different environment and create the
// resources off of the local machine.
package archive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Process returns an in-memory buffer with the archived contents of the provided file path.
func Process(path string) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	if err := addPathToZip(writer, path, path); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer, nil
}

// addPathToZip adds all the files in a given directory to a zip archive. All files in the archive are relative to the
// root path. As a result, path must be underneath root.
func addPathToZip(writer *zip.Writer, root, p string) error {
	if !strings.HasPrefix(p, root) {
		return fmt.Errorf("'%s' is not underneath '%s'", p, root)
	}

	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	h, err := zip.FileInfoHeader(stat)
	if err != nil {
		return err
	}
	// Strip out the root prefix from the file we put into the archive.
	h.Name = strings.TrimPrefix(p, root)

	if stat.IsDir() {
		h.Name += "/"
	}

	w, err := writer.CreateHeader(h)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		if _, err = io.Copy(w, file); err != nil {
			return err
		}
	} else {
		names, err := file.Readdirnames(-1)
		if err != nil {
			return err
		}

		for _, n := range names {
			if err := addPathToZip(writer, root, path.Join(p, n)); err != nil {
				return err
			}
		}
	}

	return nil
}
