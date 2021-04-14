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

package integration

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DecodeMapString takes a string of the form key1=value1:key2=value2 and returns a go map.
func DecodeMapString(val string) (map[string]string, error) {
	newMap := make(map[string]string)

	if val != "" {
		for _, overrideClause := range strings.Split(val, ":") {
			data := strings.Split(overrideClause, "=")
			if len(data) != 2 {
				return nil, errors.Errorf(
					"could not decode %s as an override, should be of the form <package>=<version>", overrideClause)
			}
			packageName := data[0]
			packageVersion := data[1]
			newMap[packageName] = packageVersion
		}
	}

	return newMap, nil
}

// ReplaceInFile does a find and replace for a given string within a file.
func ReplaceInFile(old, new, path string) error {
	rawContents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	newContents := strings.Replace(string(rawContents), old, new, -1)
	return ioutil.WriteFile(path, []byte(newContents), os.ModePerm)
}

// getCmdBin returns the binary named bin in location loc or, if it hasn't yet been initialized, will lazily
// populate it by either using the default def or, if empty, looking on the current $PATH.
func getCmdBin(loc *string, bin, def string) (string, error) {
	if *loc == "" {
		*loc = def
		if *loc == "" {
			var err error
			*loc, err = exec.LookPath(bin)
			if err != nil {
				return "", errors.Wrapf(err, "Expected to find `%s` binary on $PATH", bin)
			}
		}
	}
	return *loc, nil
}

func uniqueSuffix() string {
	// .<timestamp>.<five random hex characters>
	timestamp := time.Now().Format("20060102-150405")
	suffix, err := resource.NewUniqueHex("."+timestamp+".", 5, -1)
	contract.AssertNoError(err)
	return suffix
}

const (
	commandOutputFolderName = "command-output"
)

func writeCommandOutput(commandName, runDir string, output []byte) (string, error) {
	logFileDir := filepath.Join(runDir, commandOutputFolderName)
	if err := os.MkdirAll(logFileDir, 0700); err != nil {
		return "", errors.Wrapf(err, "Failed to create '%s'", logFileDir)
	}

	logFile := filepath.Join(logFileDir, commandName+uniqueSuffix()+".log")

	if err := ioutil.WriteFile(logFile, output, 0600); err != nil {
		return "", errors.Wrapf(err, "Failed to write '%s'", logFile)
	}

	return logFile, nil
}

// CopyFile copies a single file from src to dst
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

// CopyDir copies a whole directory recursively
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyDir(src, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = CopyFile(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}
