// Copyright 2020-2024, Pulumi Corporation.
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

package executable

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

const unableToFindProgramTemplate = "unable to find program: %s"

type NotFoundError struct {
	Path string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf(unableToFindProgramTemplate, e.Path)
}

// FindExecutable attempts to find the needed executable in various locations on the
// filesystem, eventually resorting to searching in $PATH.
func FindExecutable(program string) (string, error) {
	if runtime.GOOS == "windows" && !strings.HasSuffix(program, ".exe") &&
		!strings.HasSuffix(program, ".cmd") {
		notFoundPaths := make([]string, 0, 2)
		for _, suffix := range []string{".exe", ".cmd"} {
			found, err := findExecutableWithSuffix(program, suffix)
			var notFoundErr *NotFoundError
			if errors.As(err, &notFoundErr) {
				notFoundPaths = append(notFoundPaths, notFoundErr.Path)
				continue
			} else if err != nil {
				return "", err
			}
			return found, nil
		}
		return "", fmt.Errorf("%s. Tried: %s", unableToFindProgramTemplate, strings.Join(notFoundPaths, ", "))
	}
	return findExecutableWithSuffix(program, "")
}

func findExecutableWithSuffix(program, suffix string) (string, error) {
	program = program + suffix
	// look in the same directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to get current working directory: %w", err)
	}

	cwdProgram := filepath.Join(cwd, program)
	fileInfo, err := os.Stat(cwdProgram)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && !fileInfo.Mode().IsDir() {
		logging.V(5).Infof("program %s found in CWD", program)
		return cwdProgram, nil
	}

	// look in potentials $GOPATH/bin
	if goPath := os.Getenv("GOPATH"); len(goPath) > 0 {
		// splitGoPath will return a list of paths in which to look for the binary.
		// Because the GOPATH can take the form of multiple paths (https://golang.org/cmd/go/#hdr-GOPATH_environment_variable)
		// we need to split the GOPATH, and look into each of the paths.
		// If the GOPATH hold only one path, there will only be one element in the slice.
		goPathParts := splitGoPath(goPath, runtime.GOOS)
		for _, pp := range goPathParts {
			goPathProgram := filepath.Join(pp, "bin", program)
			fileInfo, err := os.Stat(goPathProgram)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}

			if fileInfo != nil && !fileInfo.Mode().IsDir() {
				logging.V(5).Infof("program %s found in %s/bin", program, pp)
				return goPathProgram, nil
			}
		}
	}

	// look in the $PATH somewhere
	if fullPath, err := exec.LookPath(program); err == nil {
		logging.V(5).Infof("program %s found in $PATH", program)
		return fullPath, nil
	}

	return "", fmt.Errorf(unableToFindProgramTemplate, program)
}

func splitGoPath(goPath string, os string) []string {
	var sep string
	switch os {
	case "windows":
		sep = ";"
	case "linux", "darwin":
		sep = ":"
	}

	return strings.Split(goPath, sep)
}
