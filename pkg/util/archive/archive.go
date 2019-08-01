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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Untgz uncompresses a .tar.gz/.tgz file into a specific directory.
func Untgz(tarball []byte, dir string) error {
	tarReader := bytes.NewReader(tarball)
	gzr, err := gzip.NewReader(tarReader)
	if err != nil {
		return errors.Wrapf(err, "unzipping")
	}
	r := tar.NewReader(gzr)
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrapf(err, "untarring")
		}

		path := filepath.Join(dir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// Create any directories as needed.
			if _, err := os.Stat(path); err != nil {
				if err = os.MkdirAll(path, 0700); err != nil {
					return errors.Wrapf(err, "untarring dir %s", path)
				}
			}
		case tar.TypeReg:
			// Expand files into the target directory.
			dst, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return errors.Wrapf(err, "opening file %s for untar", path)
			}
			defer contract.IgnoreClose(dst)
			if _, err = io.Copy(dst, r); err != nil {
				return errors.Wrapf(err, "untarring file %s", path)
			}
		default:
			return errors.Errorf("unexpected plugin file type %s (%v)", header.Name, header.Typeflag)
		}
	}

	return nil
}
