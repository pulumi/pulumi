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

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
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
		// Note: I had to relax constraints on main being a subfolder of root. I tried computing a relative
		// path from the auto api tmp directory. But the tmp directory on osx is `/private/var/tmp` and
		// computing a relative path doesn't work as `/private/User/...` (where the program would be) isn't
		// a valid file path.
		//
		// The reason we had this check in the first place is due to PPC constraints for hosted execution. Look through the original PR
		// https://github.com/pulumi/pulumi/pull/615 https://github.com/pulumi/pulumi-ppc/pull/102
		// It would appear that it is now safe to allow root and main to be in disjoint file trees.
		// Empyrically from my testing it seems to work, but would be worth checking with @chrsmith

		// The path can be relative from the package root.
		if !path.IsAbs(main) {
			cleanPwd := filepath.Clean(pwd)
			main = filepath.Clean(filepath.Join(cleanPwd, main))
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
