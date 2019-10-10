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

package engine

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/workspace"
)

type Projinfo struct {
	Proj *workspace.Project
	Root string
}

// GetPwdMain returns the working directory and main entrypoint to use for this package.
func (projinfo *Projinfo) GetPwdMain() (string, string, error) {
	return getPwdMain(projinfo.Root, projinfo.Proj.Main)
}

type PolicyPackInfo struct {
	Proj *workspace.PolicyPackProject
	Root string
}

// GetPwdMain returns the working directory and main entrypoint to use for this package.
func (projinfo *PolicyPackInfo) GetPwdMain() (string, string, error) {
	return getPwdMain(projinfo.Root, projinfo.Proj.Main)
}

func getPwdMain(root, main string) (string, string, error) {
	pwd := root
	if main == "" {
		main = "."
	} else {
		// The path must be relative from the package root.
		if filepath.IsAbs(main) {
			return "", "", errors.New("project 'main' must be a relative path")
		}

		// Check that main is a subdirectory.
		cleanPwd := filepath.Clean(pwd)
		main = filepath.Clean(path.Join(cleanPwd, main))
		if !strings.HasPrefix(main, cleanPwd) {
			return "", "", errors.New("project 'main' must be a subfolder")
		}

		// So that any relative paths inside of the program are correct, we still need to pass the pwd
		// of the main program's parent directory.  How we do this depends on if the target is a dir or not.
		maininfo, err := os.Stat(main)
		if err != nil {
			return "", "", errors.Wrapf(err, "project 'main' could not be read")
		}
		if maininfo.IsDir() {
			pwd = main
			main = "."
		} else {
			pwd = filepath.Dir(main)
			main = filepath.Base(main)
		}
	}

	return pwd, main, nil
}
