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

type MatrixTestOption struct {
	Language string
	Version  *semver.Version //Array of versions?
	Opts     *i.ProgramTestOptions
}

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func MatrixTest(t *testing.T, opts *i.ProgramTestOptions, langOpts []MatrixTestOption) {

	dir := opts.Dir
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

	//name := projinfo.Proj.Name

	//Instantiate new directories and run pulumi convert on each language with new directories as output.
	for _, langOpt := range langOpts {

		var projectGenerator projectGeneratorFunc
		switch langOpt.Language {
		case "csharp":
			projectGenerator = dotnetgen.GenerateProject
		case "go":
			projectGenerator = gogen.GenerateProject
		case "typescript":
			projectGenerator = jsgen.GenerateProject
		case "python":
			projectGenerator = pygen.GenerateProject
		case "java":
			projectGenerator = javagen.GenerateProject
		case "yaml": // nolint: goconst
			projectGenerator = yamlgen.GenerateProject
		default:
			projectGenerator = nil
		}

		assert.NotEqual(t, projectGenerator, nil)

		subdir := filepath.Join(dir, langOpt.Language)

		//This feels a little sketchy to me but the alternative is to go in and modify it after it's been written to file.
		//Either that or just don't mutate the name.
		//projinfo.Proj.Name = tokens.PackageName(fmt.Sprintf("%s-%s", name, langOpt.Language))

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
		if err := os.RemoveAll(subdir); err != nil {
			t.Errorf("error removing subdir: %v", err)
		}
	}
}
