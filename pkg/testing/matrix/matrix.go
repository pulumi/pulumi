// Copyright 2016-2022, Pulumi Corporation.
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
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	i "github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	jsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
)

type TestOption struct {
	Language string
	Version  *semver.Version //Array of versions?
	Opts     *i.ProgramTestOptions
}

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func Test(t *testing.T, opts *i.ProgramTestOptions, langOpts []TestOption) {

	dir := opts.Dir
	if !filepath.IsAbs(dir) {
		pwd, err := os.Getwd()
		assert.NoError(t, err)
		dir = filepath.Join(pwd, dir)
	}
	projfile := filepath.Join(dir, workspace.ProjectFile+".yaml")
	proj, err := workspace.LoadProject(projfile)
	assert.NoError(t, err)
	projinfo := &engine.Projinfo{Proj: proj, Root: dir}

	assert.NoError(t, err)

	//Assert runtime is YAML
	assert.Equal(t, projinfo.Proj.Runtime.Name(), "yaml")

	pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil)
	assert.NoError(t, err)

	proj, pclProgram, err := yamlgen.Eject(pwd, nil)
	assert.NoError(t, err)

	//check if *.pp file exists in dir
	files, err := ioutil.ReadDir(dir)
	assert.NoError(t, err)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".pp") {
			parser := syntax.NewParser()
			//get file content
			ppFilePath := filepath.Join(dir, file.Name())
			contents, err := ioutil.ReadFile(ppFilePath)
			assert.NoError(t, err)
			err = parser.ParseFile(bytes.NewReader(contents), ppFilePath)
			assert.NoError(t, err)
			program, diags, err := pcl.BindProgram(parser.Files, pcl.PluginHost(ctx.Host))
			assert.NoError(t, err)
			assert.Empty(t, diags)
			pclProgram = program

		}
	}

	//Replace relative paths with absolute paths

	if proj.Plugins != nil {
		for _, provider := range proj.Plugins.Providers {
			if !filepath.IsAbs(provider.Path) {
				provider.Path = filepath.Join(dir, provider.Path)
			}
		}
		for _, language := range proj.Plugins.Languages {
			if !filepath.IsAbs(language.Path) {
				language.Path = filepath.Join(dir, language.Path)
			}
		}

		for _, analyzer := range proj.Plugins.Analyzers {
			if !filepath.IsAbs(analyzer.Path) {
				analyzer.Path = filepath.Join(dir, analyzer.Path)
			}
		}
	}

	assert.NoError(t, err)

	//Instantiate new directories and run pulumi convert on each language with new directories as output.
	for _, langOpt := range langOpts {

		var projectGenerator projectGeneratorFunc
		switch langOpt.Language {
		case "dotnet":
			projectGenerator = dotnetgen.GenerateProject
		case "go":
			projectGenerator = gogen.GenerateProject
		case "nodejs":
			projectGenerator = jsgen.GenerateProject
		case "python":
			projectGenerator = pygen.GenerateProject
		case "java":
			projectGenerator = javagen.GenerateProject
		case "yaml": // nolint: goconst
			projectGenerator = yamlgen.GenerateProject
		default:
			assert.FailNow(t, "Unsupported language: "+langOpt.Language)
		}

		subdir := filepath.Join(dir, langOpt.Language)
		//check if subdir exists
		if _, err := os.Stat(subdir); !os.IsNotExist(err) {
			assert.NoError(t, os.RemoveAll(subdir))
		}

		//create subdir
		err = os.MkdirAll(subdir, 0755)
		assert.NoError(t, err)

		//This feels a little sketchy to me but the alternative is to go in and modify it after it's been written to file.
		//Either that or just don't mutate the name.
		//projinfo.Proj.Name = tokens.PackageName(fmt.Sprintf("%s-%s", name, langOpt.Language))

		//generate project

		err = projectGenerator(subdir, *proj, pclProgram)
		assert.NoError(t, err)

		//Configure opts
		langPtOpts := i.ProgramTestOptions{}
		if opts != nil {
			langPtOpts = *opts
		}
		if langOpt.Opts != nil {
			langPtOpts = langPtOpts.With(*langOpt.Opts)
		}
		langPtOpts = langPtOpts.With(i.ProgramTestOptions{
			Dir: subdir,
		})
		t.Run(langOpt.Language, func(t *testing.T) {
			i.ProgramTest(t, &langPtOpts)
		})
		//clean up subdir

		/*
			if err := os.RemoveAll(subdir); err != nil {
				t.Errorf("error removing subdir: %v", err)
			}*/
	}
}
