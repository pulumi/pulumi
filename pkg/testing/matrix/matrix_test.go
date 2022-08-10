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
	"os/exec"
	"testing"

	"github.com/blang/semver"
	i "github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAll(t *testing.T) {
	t.Parallel()
	opts := []TestOptions{
		/*{
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
		{
			Program: &i.ProgramTestOptions{
				Dir: "tests/reference",
				Aux: []i.AuxiliaryStack{
					{
						Dir:         "tests/scalar",
						Initialized: false,
					},
				},
			},
			Languages: allLanguages(),
		},*/
		{
			Program: &i.ProgramTestOptions{
				Dir:              "tests/provider",
				PulumiBin:        "/Users/hliuson/.pulumi-dev/bin/pulumi",
				SkipRefresh:      true,
				SkipPreview:      true,
				SkipExportImport: true,
			},
			Languages: allLanguages(),
			Plugins: []PluginOptions{
				{
					Name:    "command",
					Kind:    workspace.ResourcePlugin,
					Build:   []exec.Cmd{},
					Bin:     "./bin", //I've put a local version of the binary here, but it's not checked into git
					Version: semver.MustParse("0.4.2"),
				},
				{
					Name:    "yaml",
					Kind:    workspace.LanguagePlugin,
					Build:   []exec.Cmd{},
					Bin:     "./bin",
					Version: semver.MustParse("0.5.4"),
				},
			},
		},
	}

	t.Parallel()
	for _, opt := range opts {
		t.Run(opt.Program.Dir, func(t *testing.T) {
			t.Parallel()
			Test(t, opt)
		})
	}
}

func allLanguages() []LangTestOption {
	return []LangTestOption{
		{
			Language: "go",
			Opts:     nil,
		},
		{
			Language: "python",
			Opts:     nil,
		},
		{
			Language: "nodejs",
			Opts:     nil,
		},
		{
			Language: "dotnet",
			Opts:     nil,
		},
		/*{
			Language: "java",
			Version:  nil,
			Opts:     nil,
		},*/
		{
			Language: "yaml",
			Opts:     nil,
		},
	}
}
