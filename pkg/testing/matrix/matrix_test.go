// Copyright 2016-2021, Pulumi Corporation.
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

package matrix

import (
	"testing"

	"github.com/blang/semver"
	i "github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestAll(t *testing.T) {
	t.Parallel()
	opts := []MatrixTestOptions{
		{
			Program: &i.ProgramTestOptions{
				Dir: "tests/empty",
			},
			Languages: allLanguages(),
		},
		{
			Program: &i.ProgramTestOptions{
				Dir: "tests/scalar",
			},
			Languages: allLanguages(),
		},
		{
			Program: &i.ProgramTestOptions{
				Dir: "tests/structured",
			},
			Languages: allLanguages(),
		},
	}

	for _, opt := range opts {
		t.Run(opt.Program.Dir, func(t *testing.T) {
			Test(t, opt)
		})
	}
}

func allLanguages() []LangTestOption {
	return []LangTestOption{
		{
			Language: "go",
			Version:  &semver.Version{Major: 1, Minor: 7, Patch: 0},
			Opts:     nil,
		},
		{
			Language: "python",
			Version:  &semver.Version{Major: 3, Minor: 7, Patch: 0},
			Opts:     nil,
		},
		{
			Language: "nodejs",
			Version:  &semver.Version{Major: 8, Minor: 0, Patch: 0},
			Opts:     nil,
		},
		{
			Language: "dotnet",
			Version:  &semver.Version{Major: 2, Minor: 0, Patch: 0},
			Opts:     nil,
		},
		{
			Language: "yaml",
			Version:  nil,
			Opts:     nil,
		},
	}
}
