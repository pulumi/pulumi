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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	i "github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAll(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	//get ~ path
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	pulumiDir := fmt.Sprintf("%s/.pulumi/", home)

	tester, err := NewTester([]PluginOptions{
		//nolint:gosec // Complains about filepath join
		{
			Name: "command",
			Kind: workspace.ResourcePlugin,
			Build: []exec.Cmd{
				*exec.Command("pulumi", "plugin", "install", "resource", "command", "0.4.1"),
				*exec.Command("cp", filepath.Join(pulumiDir, "plugins",
					"resource-command-v0.4.1/pulumi-resource-command"), filepath.Join(pwd, "tests", "bin", "pulumi-resource-command")),
			},
			Bin:     "./tests/bin",
			Version: semver.MustParse("0.4.1"),
		},
		//nolint:gosec // Complains about filepath join
		{
			Name: "yaml",
			Kind: workspace.LanguagePlugin,
			Build: []exec.Cmd{
				*exec.Command("pulumi", "plugin", "install", "language", "yaml", "0.5.4"),
				*exec.Command("cp", filepath.Join(pulumiDir, "plugins",
					"language-yaml-v0.5.4/pulumi-language-yaml"), filepath.Join(pwd, "tests", "bin", "pulumi-language-yaml")),
			},
			Bin:     "./tests/bin",
			Version: semver.MustParse("0.5.4"),
		}},
		allLanguages(),
		nil)
	if err != nil {
		t.Fatal(err)
	}

	opts := []i.ProgramTestOptions{
		//Tests are commented out because codegen currently fails them.

		{
			Dir: "tests/empty",
		},

		/*
			{
				Dir: "tests/scalar",
			},
		*/
		/*{
		/*
		{
			Dir: "tests/structured",
		},
		*/
		/*
			{
				Dir: "tests/reference",
				Aux: []i.AuxiliaryStack{
					{
						Dir:         "tests/scalar",
						Initialized: false,
					},
				},
			},
		*/
		{
			Dir:              "tests/provider",
			PulumiBin:        filepath.Join(home, ".pulumi-dev", "bin", "pulumi"),
			SkipRefresh:      true,
			SkipExportImport: true,
		},
	}
	for _, opt := range opts { //nolint:paralleltest
		//"Range statement for test TestAll does not reinitialise the variable opt"
		t.Run(opt.Dir, func(t *testing.T) {
			for _, lang := range allLanguages() { //nolint:paralleltest
				t.Run(lang.Language, func(t *testing.T) {
					tester.TestLang(t, &opt, lang.Language)
				})
			}
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
