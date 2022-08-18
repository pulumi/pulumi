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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	i "github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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

const NODEJS = "nodejs"
const PYTHON = "python"
const JAVA = "java"
const DOTNET = "dotnet"
const YAML = "yaml"
const GO = "go"

type LangTestOption struct {
	Language string
	Opts     *i.ProgramTestOptions
}

type PluginOptions struct {
	Name    string
	Kind    workspace.PluginKind
	Version semver.Version

	// Build is the set of commands to run to build the plugin and SDK
	// In the future we want to just generate the SDK using the plugin.
	Build []exec.Cmd

	// Bin is the path to the plugin to use.
	Bin string
}

type Tester struct {
	Plugins       workspace.Plugins
	Languages     []LangTestOption
	LocalProjects map[string]map[string]string
}

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program,
	localProjects map[string]map[string]string) error

func NewTester(PluginList []PluginOptions, Languages []LangTestOption, PulumiSDKs map[string]string) (Tester, error) {
	plugins := workspace.Plugins{}
	localProjects := map[string]map[string]string{}

	root, err := os.Getwd()
	if err != nil {
		return Tester{}, err
	}

	for _, plugin := range PluginList {
		for _, cmd := range plugin.Build {
			std, err := cmd.CombinedOutput()
			if err != nil {
				return Tester{}, fmt.Errorf("error building plugin %s: %s\n%s", plugin.Name, err, std)
			}
		}
	}

	for _, plugin := range PluginList {
		root := fmt.Sprintf("%s/%s-sdk", root, plugin.Name)
		localProjects[plugin.Name] = map[string]string{
			NODEJS: filepath.Join(root, "nodejs", "bin"),
			PYTHON: filepath.Join(root, "python"),
			GO:     filepath.Join(root, "go"),
			DOTNET: filepath.Join(root, "dotnet"),
		}

		p := workspace.PluginOptions{
			Name:    plugin.Name,
			Path:    plugin.Bin,
			Version: plugin.Version.String(),
		}

		switch plugin.Kind {
		case workspace.AnalyzerPlugin:
			plugins.Analyzers = append(plugins.Analyzers, p)
		case workspace.LanguagePlugin:
			plugins.Languages = append(plugins.Languages, p)
		case workspace.ResourcePlugin:
			plugins.Providers = append(plugins.Providers, p)
		}
	}
	//Replace relative paths with absolute paths

	for i, provider := range plugins.Providers {
		if !filepath.IsAbs(provider.Path) {
			plugins.Providers[i].Path = filepath.Join(root, provider.Path)
		}
	}
	for i, language := range plugins.Languages {
		if !filepath.IsAbs(language.Path) {
			plugins.Languages[i].Path = filepath.Join(root, language.Path)
		}
	}

	for i, analyzer := range plugins.Analyzers {
		if !filepath.IsAbs(analyzer.Path) {
			plugins.Analyzers[i].Path = filepath.Join(root, analyzer.Path)
		}
	}

	//Generate SDKs
	pluginctx, err := plugin.NewContextWithRoot(cmdutil.Diag(), cmdutil.Diag(), nil, root, root,
		nil, false, nil, &plugins)
	if err != nil {
		return Tester{}, err
	}
	for _, plugin := range PluginList {
		if plugin.Kind != workspace.ResourcePlugin {
			continue
		}

		info, err := pluginctx.Host.ResolvePlugin(plugin.Kind, plugin.Name, &plugin.Version)
		if err != nil {
			return Tester{}, err
		}

		provider, err := pluginctx.Host.Provider(tokens.Package(info.Name), &plugin.Version)
		if err != nil {
			return Tester{}, err
		}
		schemaBytes, err := provider.GetSchema(0)
		if err != nil {
			return Tester{}, err
		}

		var spec schema.PackageSpec
		err = json.Unmarshal(schemaBytes, &spec)
		if err != nil {
			return Tester{}, err
		}

		pkg, _, err := schema.BindSpec(spec, nil)
		if err != nil {
			return Tester{}, err
		}

		pkg.Test = true

		pkgName := pkg.Name

		for _, langOpt := range Languages {
			lang := langOpt.Language

			var files map[string][]byte
			var err error
			switch lang {
			case GO:
				files, err = gogen.GeneratePackage(pkgName, pkg)
				if err != nil {
					return Tester{}, err
				}
			case PYTHON:
				err = PythonConfigurePkg(pkg)
				if err != nil {
					return Tester{}, err
				}
				files, err = pygen.GeneratePackage(pkgName, pkg, files)
				if err != nil {
					return Tester{}, err
				}
			case NODEJS:
				err = NodeConfigurePkg(pkg)
				if err != nil {
					return Tester{}, err
				}
				files, err = jsgen.GeneratePackage(pkgName, pkg, files)
				if err != nil {
					return Tester{}, err
				}
			case DOTNET:
				err = DotnetConfigurePkg(pkg)
				if err != nil {
					return Tester{}, err
				}
				files, err = dotnetgen.GeneratePackage(pkgName, pkg, files)
				if err != nil {
					return Tester{}, err
				}
			//In the future we should support java but I hava no idea where to even start with that.
			case YAML:
				continue
			default:
				fmt.Printf("Unknown language: '%s'", lang)
				continue
			}

			sdkDir := filepath.Join(root, fmt.Sprintf("%s-sdk", pkgName), lang)
			for p, file := range files {
				err = os.MkdirAll(filepath.Join(sdkDir, path.Dir(p)), 0700)
				if err != nil {
					return Tester{}, err
				}
				err = os.WriteFile(filepath.Join(sdkDir, p), file, 0600)
				if err != nil {
					return Tester{}, err
				}
			}
			if lang == NODEJS {
				//yarn install
				cmd := exec.Command("yarn", "install")
				cmd.Dir = sdkDir
				err = cmd.Run()
				if err != nil {
					return Tester{}, err
				}

				//yarn run tsc
				cmd = exec.Command("yarn", "run", "tsc")
				cmd.Dir = sdkDir
				err = cmd.Run()
				if err != nil {
					return Tester{}, err
				}

				//cp ../../README.md ../../LICENSE package.json yarn.lock ./bin/
				cmd = exec.Command("cp", "package.json", "yarn.lock", "./bin/")
				cmd.Dir = sdkDir
				err = cmd.Run()
				if err != nil {
					return Tester{}, err
				}

				//sed -i.bak -e "s/\$${VERSION}/$(VERSION)/g" ./bin/package.json
				replace := fmt.Sprintf("s/$${VERSION}/%s/g", pkg.Version)
				cmd = exec.Command("sed", "-i.bak", "-e", replace, "./bin/package.json")
				cmd.Dir = sdkDir
				err = cmd.Run()
				if err != nil {
					return Tester{}, err
				}
			}
			if lang == GO {
				//check if go.mod exists inside sdkdir
				_, err := os.Stat(filepath.Join(sdkDir, "go.mod"))
				//check if err is notExistError
				if os.IsNotExist(err) {
					//if not, create it
					cmd := exec.Command("go", "mod", "init")
					cmd.Dir = sdkDir
					std, err := cmd.CombinedOutput()
					if err != nil {
						fmt.Printf("%s\n", std)
						return Tester{}, err
					}
				}

				cmd := exec.Command("go", "mod", "tidy")
				cmd.Dir = sdkDir
				std, err := cmd.CombinedOutput()
				if err != nil {
					fmt.Printf("%s\n", std)
					return Tester{}, err
				}
			}
		}
	}

	localProjects["pulumi"] = PulumiSDKs

	return Tester{
		Plugins:       plugins,
		Languages:     Languages,
		LocalProjects: localProjects,
	}, nil
}

func (mTester Tester) TestLang(t *testing.T, opts *i.ProgramTestOptions, language string) {
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
	proj.Plugins = &mTester.Plugins

	assert.NoError(t, err)

	//Assert runtime is YAML
	assert.Equal(t, projinfo.Proj.Runtime.Name(), "yaml")

	pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil)
	assert.NoError(t, err)

	var pclProgram *pcl.Program

	host, err := plugin.NewDefaultHost(ctx, nil, false, &mTester.Plugins)
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
			program, diags, err := pcl.BindProgram(parser.Files, pcl.PluginHost(host))
			assert.NoError(t, err)
			assert.Empty(t, diags)
			pclProgram = program

		}
	}
	if pclProgram == nil {
		loader := schema.NewPluginLoader(host)
		_, pclProgram, err = yamlgen.Eject(pwd, loader)
		assert.NoError(t, err)
	}
	assert.NotNil(t, proj)
	assert.NotNil(t, pclProgram)

	//Instantiate new directories and run pulumi convert on each language with new directories as output.
	for _, langOpt := range mTester.Languages {
		if langOpt.Language == language {
			var projectGenerator projectGeneratorFunc
			switch langOpt.Language {
			case DOTNET:
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {

					return dotnetgen.GenerateProject(directory, project, p, getLanguage(localProjects, "dotnet"))
				}
			case GO:
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {

					return gogen.GenerateProject(directory, project, p, getLanguage(localProjects, "go"))
				}
			case NODEJS:
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {

					return jsgen.GenerateProject(directory, project, p, getLanguage(localProjects, "nodejs"))
				}
			case PYTHON:
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {

					return pygen.GenerateProject(directory, project, p, getLanguage(localProjects, "python"))
				}
			case JAVA:
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {
					return javagen.GenerateProject(directory, project, p)
				}
			case YAML: // nolint: goconst
				projectGenerator = func(directory string, project workspace.Project,
					p *pcl.Program, localProjects map[string]map[string]string) error {
					return yamlgen.GenerateProject(directory, project, p)
				}
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

			//generate project
			err = projectGenerator(subdir, *proj, pclProgram, mTester.LocalProjects)
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
			i.ProgramTest(t, &langPtOpts)
		}
	}
}

func PythonConfigurePkg(pkg *schema.Package) error {
	pkg.Version = &semver.Version{
		Major: pkg.Version.Major,
		Minor: pkg.Version.Minor,
		Patch: pkg.Version.Patch,
	} // Prune patch and build versions, since semver and python don't seem to agree on formatting.

	raw, notnil := pkg.Language["python"]
	message, ismessage := raw.(json.RawMessage)
	var pyPkg pygen.PackageInfo
	if notnil && ismessage {
		err := json.Unmarshal(message, &pyPkg)
		if err != nil {
			return err
		}
	} else if notnil && !ismessage {
		info, ok := raw.(pygen.PackageInfo)
		if !ok {
			return fmt.Errorf("invalid python package info")
		}
		pyPkg = info
	} else if !notnil {
		pyPkg = pygen.PackageInfo{}
	}
	pyPkg.RespectSchemaVersion = true
	pkg.Language["python"] = pyPkg
	return nil
}

func NodeConfigurePkg(pkg *schema.Package) error {
	raw, notnil := pkg.Language["nodejs"]
	message, ismessage := raw.(json.RawMessage)
	var nodePkg jsgen.NodePackageInfo
	if notnil && ismessage {
		err := json.Unmarshal(message, &nodePkg)
		if err != nil {
			return err
		}
	} else if notnil && !ismessage {
		info, ok := raw.(jsgen.NodePackageInfo)
		if !ok {
			return fmt.Errorf("invalid nodejs package info")
		}
		nodePkg = info
	} else if !notnil {
		nodePkg = jsgen.NodePackageInfo{}
	}
	nodePkg.RespectSchemaVersion = true
	pkg.Language["nodejs"] = nodePkg
	return nil
}

func DotnetConfigurePkg(pkg *schema.Package) error {
	raw, notnil := pkg.Language["csharp"]
	message, ismessage := raw.(json.RawMessage)
	var csPkg dotnetgen.CSharpPackageInfo
	if notnil && ismessage {
		err := json.Unmarshal(message, &csPkg)
		if err != nil {
			return err
		}
	} else if notnil && !ismessage {
		info, ok := raw.(dotnetgen.CSharpPackageInfo)
		if !ok {
			return fmt.Errorf("invalid csharp package info")
		}
		csPkg = info
	} else if !notnil {
		csPkg = dotnetgen.CSharpPackageInfo{}
	}
	csPkg.RespectSchemaVersion = true
	pkg.Language["csharp"] = csPkg
	return nil
}

func getLanguage(localProjects map[string]map[string]string, language string) map[string]string {
	langProjects := map[string]string{}
	for name, sdks := range localProjects {
		sdk, ok := sdks[language]
		if ok {
			langProjects[name] = sdk
		}
	}
	return langProjects
}
