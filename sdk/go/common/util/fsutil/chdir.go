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

package fsutil

import (
	"os"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// Chdir changes the directory so that all operations from now on are relative to the project we are working with.
// It returns a function that, when run, restores the old working directory.
func Chdir(pwd string) (func(), error) {
	if pwd == "" {
		return func() {}, nil
	}
	oldpwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err = os.Chdir(pwd); err != nil {
		return nil, errors.Wrapf(err, "could not change to the project working directory")
	}
	return func() {
		// Restore the working directory after planning completes.
		cderr := os.Chdir(oldpwd)
		contract.IgnoreError(cderr)
	}, nil
}
