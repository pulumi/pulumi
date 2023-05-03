package pcl_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/spf13/afero"

	"github.com/hashicorp/hcl/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestBindProgram(t *testing.T) {
	t.Parallel()

	testdata, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	bindOptions := map[string][]pcl.BindOption{}
	for _, r := range test.PulumiPulumiProgramTests {
		bindOptions[r.Directory+"-pp"] = r.BindOptions
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, v := range testdata {
		v := v
		if !v.IsDir() {
			continue
		}
		folderPath := filepath.Join(testdataPath, v.Name())
		files, err := os.ReadDir(folderPath)
		if err != nil {
			t.Fatalf("could not read test data: %v", err)
		}
		for _, fileName := range files {
			fileName := fileName.Name()
			if filepath.Ext(fileName) != ".pp" {
				continue
			}

			t.Run(fileName, func(t *testing.T) {
				t.Parallel()

				path := filepath.Join(folderPath, fileName)
				contents, err := os.ReadFile(path)
				require.NoErrorf(t, err, "could not read %v", path)

				parser := syntax.NewParser()
				err = parser.ParseFile(bytes.NewReader(contents), fileName)
				require.NoErrorf(t, err, "could not read %v", path)
				require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")

				var bindError error
				var diags hcl.Diagnostics
				loader := pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath)))
				absoluteFolderPath, err := filepath.Abs(folderPath)
				if err != nil {
					t.Fatalf("failed to bind program: unable to find the absolute path of %v", folderPath)
				}
				options := append(
					bindOptions[v.Name()],
					loader,
					pcl.DirPath(absoluteFolderPath),
					pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))
				// PCL binder options are taken from program_driver.go
				program, diags, bindError := pcl.BindProgram(parser.Files, options...)

				assert.NoError(t, bindError)
				if diags.HasErrors() || program == nil {
					t.Fatalf("failed to bind program: %v", diags)
				}
			})
		}
	}
}

func TestWritingProgramSource(t *testing.T) {
	t.Parallel()
	// STEP 1: Bind the program from {test-data}/components
	componentsDir := "components-pp"
	folderPath := filepath.Join(testdataPath, componentsDir)
	files, err := os.ReadDir(folderPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}
	parser := syntax.NewParser()
	for _, fileName := range files {
		fileName := fileName.Name()
		if filepath.Ext(fileName) != ".pp" {
			continue
		}

		path := filepath.Join(folderPath, fileName)
		contents, err := os.ReadFile(path)
		require.NoErrorf(t, err, "could not read %v", path)

		err = parser.ParseFile(bytes.NewReader(contents), fileName)
		require.NoErrorf(t, err, "could not read %v", path)
		require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")
	}

	var bindError error
	var diags hcl.Diagnostics
	absoluteProgramPath, err := filepath.Abs(folderPath)
	if err != nil {
		t.Fatalf("failed to bind program: unable to find the absolute path of %v", folderPath)
	}

	program, diags, bindError := pcl.BindProgram(parser.Files,
		pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath))),
		pcl.DirPath(absoluteProgramPath),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	assert.NoError(t, bindError)
	if diags.HasErrors() || program == nil {
		t.Fatalf("failed to bind program: %v", diags)
	}

	// STEP 2: assert the resulting files
	fs := afero.NewMemMapFs()
	writingFilesError := program.WriteSource(fs)
	assert.NoError(t, writingFilesError, "failed to write source files")

	// Assert main file exists
	mainFileExists, err := afero.Exists(fs, "/components.pp")
	assert.NoError(t, err, "failed to get the main file")
	assert.True(t, mainFileExists, "main program file should exist at the root")

	// Assert directories "simpleComponent" and "exampleComponent" are present
	simpleComponentDirExists, err := afero.DirExists(fs, "/simpleComponent")
	assert.NoError(t, err, "failed to get the simple component dir")
	assert.True(t, simpleComponentDirExists, "simple component dir exists")

	exampleComponentDirExists, err := afero.DirExists(fs, "/exampleComponent")
	assert.NoError(t, err, "failed to get the example component dir")
	assert.True(t, exampleComponentDirExists, "example component dir exists")

	// Assert simpleComponent/main.pp and exampleComponent/main.pp exist
	simpleMainExists, err := afero.Exists(fs, "/simpleComponent/main.pp")
	assert.NoError(t, err, "failed to get the main file of simple component")
	assert.True(t, simpleMainExists, "main program file of simple component should exist")

	exampleMainExists, err := afero.Exists(fs, "/exampleComponent/main.pp")
	assert.NoError(t, err, "failed to get the main file of example component")
	assert.True(t, exampleMainExists, "main program file of example component should exist")
}

func parseAndBindProgram(t *testing.T, text, name string, options ...pcl.BindOption) (*pcl.Program, hcl.Diagnostics) {
	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(text), name)
	if err != nil {
		t.Fatalf("could not read %v: %v", name, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	options = append(options, pcl.PluginHost(utils.NewHost(testdataPath)))

	program, diags, err := pcl.BindProgram(parser.Files, options...)
	if err != nil {
		t.Fatalf("could not bind program: %v", err)
	}
	return program, diags
}

func TestConfigNodeTypedString(t *testing.T) {
	t.Parallel()
	source := "config cidrBlock string { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "cidrBlock")
	assert.Equal(t, config.Type(), model.StringType, "the type is a string")
}

func TestConfigNodeTypedOptionalString(t *testing.T) {
	t.Parallel()
	source := "config cidrBlock string { default = null }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "cidrBlock")
	assert.True(t, model.IsOptionalType(config.Type()), "the type is optional")
	elementType := pcl.UnwrapOption(config.Type())
	assert.Equal(t, elementType, model.StringType, "element type is a string")
	assert.True(t, config.Nullable, "The config variable is nullable")
}

func TestConfigNodeTypedInt(t *testing.T) {
	t.Parallel()
	source := "config count int { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "count")
	assert.Equal(t, config.Type(), model.IntType, "the type is a string")
}

func TestConfigNodeTypedStringList(t *testing.T) {
	t.Parallel()
	source := "config names \"list(string)\" { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	listType, ok := config.Type().(*model.ListType)
	assert.True(t, ok, "the type of config is a list type")
	assert.Equal(t, listType.ElementType, model.StringType, "the element type is a string")
}

func TestConfigNodeTypedIntList(t *testing.T) {
	t.Parallel()
	source := "config names \"list(int)\" { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	listType, ok := config.Type().(*model.ListType)
	assert.True(t, ok, "the type of config is a list type")
	assert.Equal(t, listType.ElementType, model.IntType, "the element type is an int")
}

func TestConfigNodeTypedStringMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(string)\" { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.StringType, "the element type is a string")
}

func TestConfigNodeTypedIntMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(int)\" { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.IntType, "the element type is an int")
}

func TestConfigNodeTypedAnyMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(any)\" { }"
	program, diags := parseAndBindProgram(t, source, "config.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.DynamicType, "the element type is a dynamic")
}
