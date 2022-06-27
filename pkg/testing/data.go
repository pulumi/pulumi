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

package testing

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
)

type projectGeneratorFunc func(string, workspace.Project, *pcl.Program) error
type packageGeneratorFunc func(string, *schema.Package) (map[string][]byte, error)

func Datatest(t *testing.T, language string, dir string) {
	tests, err := ioutil.ReadDir(dir)
	assert.NoError(t, err)

	addPath(t, filepath.Join("..", "testprovider"))

	tmpsdk, err := os.MkdirTemp("", "pulumi-data-test-sdk")
	assert.NoError(t, err)
	defer os.Remove(tmpsdk)

	var packageGenerator packageGeneratorFunc
	var projectGenerator projectGeneratorFunc
	switch language {
	case "dotnet":
		packageGenerator = func(s string, p *schema.Package) (map[string][]byte, error) {
			return dotnet.GeneratePackage(s, p, nil)
		}
		projectGenerator = dotnet.GenerateProject
	case "go":
		packageGenerator = gogen.GeneratePackage
		projectGenerator = gogen.GenerateProject
	case "nodejs":
		packageGenerator = func(s string, p *schema.Package) (map[string][]byte, error) {
			return nodejs.GeneratePackage(s, p, nil)
		}
		projectGenerator = nodejs.GenerateProject
	case "python":
		packageGenerator = func(s string, p *schema.Package) (map[string][]byte, error) {
			return python.GeneratePackage(s, p, nil)
		}
		projectGenerator = python.GenerateProject
	case "java":
		packageGenerator = func(s string, p *schema.Package) (map[string][]byte, error) {
			return javagen.GeneratePackage(s, p, nil)
		}
		projectGenerator = javagen.GenerateProject
	case "yaml":
		packageGenerator = func(s string, p *schema.Package) (map[string][]byte, error) {
			return nil, nil
		}
		projectGenerator = yamlgen.GenerateProject
	default:
		assert.Fail(t, "unrecognized langauge %s", language)
	}

	// Convert the testprovider into an sdk
	ctx, err := plugin.NewContext(nil, nil, nil, nil, ".", nil, false, nil)
	assert.NoError(t, err)
	loader := schema.NewPluginLoader(ctx.Host)
	testSchema, err := loader.LoadPackage("testprovider", nil)
	assert.NoError(t, err)

	files, err := packageGenerator("testing", testSchema)
	assert.NoError(t, err)
	for path, data := range files {
		os.WriteFile(filepath.Join(tmpsdk, path), data, 066)
	}

	for _, test := range tests {
		if !test.IsDir() {
			continue
		}

		test := test
		t.Run(test.Name(), func(t *testing.T) {
			t.Parallel()

			tmp, err := os.MkdirTemp("", "pulumi-data-test-program")
			assert.NoError(t, err)
			defer os.Remove(tmp)

			// Read the yaml file or new up a tiny project description
			projectPath := filepath.Join(test.Name(), "Pulumi.yaml")
			var project *workspace.Project
			if _, err := os.Stat(projectPath); err == nil {
				project, err = workspace.LoadProject(projectPath)
				assert.NoError(t, err)
			} else {
				project = &workspace.Project{Name: tokens.PackageName(test.Name())}
			}

			// Read the PCL file
			pclFile, err := os.Open(filepath.Join(test.Name(), "program.pp"))
			assert.NoError(t, err)

			// Convert the pcl file to the right language
			parser := syntax.NewParser()
			err = parser.ParseFile(pclFile, "program.pp")
			assert.NoError(t, err)
			assert.Empty(t, parser.Diagnostics.Errs())

			pclProgram, diagnostics, err := pcl.BindProgram(parser.Files)
			assert.NoError(t, err)
			assert.Empty(t, diagnostics.Errs())

			err = projectGenerator(tmp, *project, pclProgram)
			assert.NoError(t, err)

			// Now try and run the project
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:        tmp,
				NoParallel: true,
			})
		})
	}
}

func addPath(t *testing.T, path ...string) {
	pathEnv := []string{os.Getenv("PATH")}
	for _, p := range path {
		absPath, err := filepath.Abs(p)
		if err != nil {
			t.Fatal(err)
		}
		pathEnv = append(pathEnv, absPath)
	}
	pathSeparator := ":"
	if runtime.GOOS == "windows" {
		pathSeparator = ";"
	}
	t.Setenv("PATH", strings.Join(pathEnv, pathSeparator))
}
