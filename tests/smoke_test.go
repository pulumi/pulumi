package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var Runtimes = []string{"python", "java", "go", "yaml", "nodejs", "dotnet"}

// Mapping from the language runtime names to the common language name used by templates and the like.
var Languages = map[string]string{
	"python": "python",
	"java":   "java",
	"go":     "go",
	"yaml":   "yaml",
	"nodejs": "typescript",
	"dotnet": "csharp",
}

// Quick sanity tests for each downstream language to check that a minimal example can be created and run.
//
//nolint:paralleltest // pulumi new is not parallel safe
func TestLanguageNewSmoke(t *testing.T) {
	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local filestate means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "new", "random-"+Languages[runtime], "--yes")
			e.RunCommand("pulumi", "up", "--yes")
			e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// Quick sanity tests that YAML convert works.
func TestYamlConvertSmoke(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	e.ImportDirectory("testdata/random_yaml")

	// Make sure random is installed
	out, _ := e.RunCommand("pulumi", "plugin", "ls")
	if !strings.Contains(out, "random  resource  4.13.0") {
		e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")
	}

	e.RunCommand(
		"pulumi", "convert", "--strict",
		"--language", "pcl", "--from", "yaml", "--out", "out")

	actualPcl, err := os.ReadFile(filepath.Join(e.RootPath, "out", "program.pp"))
	require.NoError(t, err)
	assert.Equal(t, `resource pet "random:index/randomPet:RandomPet" {
	__logicalName = "pet"
}

output name {
	__logicalName = "name"
	value = pet.id
}
`, string(actualPcl))
}

// Quick sanity tests for each downstream language to check that convert works.
func TestLanguageConvertSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/random_pp")

			// Make sure random is installed
			out, _ := e.RunCommand("pulumi", "plugin", "ls")
			if !strings.Contains(out, "random  resource  4.13.0") {
				e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")
			}

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand(
				"pulumi", "convert", "--strict",
				"--language", Languages[runtime], "--from", "pcl", "--out", "out")
			e.CWD = filepath.Join(e.RootPath, "out")
			e.RunCommand("pulumi", "stack", "init", "test")

			// TODO[pulumi/pulumi#13075]: Skipping `up` until we have a way to tell the language host to
			// install dependencies.
			// e.RunCommand("pulumi", "up", "--yes")
			// e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// Quick sanity tests for each downstream language to check that non-strict convert works.
func TestLanguageConvertLenientSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/bad_random_pp")

			// Make sure random is installed
			out, _ := e.RunCommand("pulumi", "plugin", "ls")
			if !strings.Contains(out, "random  resource  4.13.0") {
				e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")
			}

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand(
				"pulumi", "convert", "--generate-only",
				"--language", Languages[runtime], "--from", "pcl", "--out", "out")
			// We don't want care about running this program because it _will_ be incorrect.
		})
	}
}

// Quick sanity tests for each downstream language to check that convert with components works.
func TestLanguageConvertComponentSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			if runtime == "yaml" {
				t.Skip("yaml doesn't support components")
			}
			if runtime == "java" {
				t.Skip("yaml doesn't support components")
			}

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/component_pp")

			// Make sure random is installed
			out, _ := e.RunCommand("pulumi", "plugin", "ls")
			if !strings.Contains(out, "random  resource  4.13.0") {
				e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")
			}

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "convert", "--language", Languages[runtime], "--from", "pcl", "--out", "out")
			e.CWD = filepath.Join(e.RootPath, "out")
			e.RunCommand("pulumi", "stack", "init", "test")

			// TODO[pulumi/pulumi#13075]: Skipping `up` until we have a way to tell the language host to
			// install dependencies.
			// e.RunCommand("pulumi", "up", "--yes")
			// e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// Quick sanity tests for each downstream language to check that sdk-gen works.
func TestLanguageGenerateSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		if runtime == "yaml" {
			// yaml doesn't support sdks
			continue
		}

		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/simple_schema")
			e.RunCommand("pulumi", "package", "gen-sdk", "--language", runtime, "schema.json")
		})
	}
}

//nolint:paralleltest // disabled parallel because we change the plugins cache
func TestPackageGetSchema(t *testing.T) {
	// Skipping because it gets rate limited in CI
	t.Skip()
	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)
	removeRandomFromLocalPlugins := func() {
		e.RunCommand("pulumi", "plugin", "rm", "resource", "random", "--all", "--yes")
	}

	bindSchema := func(schemaJson string) {
		var schemaSpec *schema.PackageSpec
		err := json.Unmarshal([]byte(schemaJson), &schemaSpec)
		require.NoError(t, err, "Unmarshalling schema specs from random should work")
		require.NotNil(t, schemaSpec, "Specification should be non-nil")
		schema, diags, err := schema.BindSpec(*schemaSpec, nil)
		require.NoError(t, err, "Binding the schema spec should work")
		require.False(t, diags.HasErrors(), "Binding schema spec should have no errors")
		require.NotNil(t, schema)
	}

	// Make sure the random provider is not installed locally
	// So that we can test the `package get-schema` command works if the plugin
	// is not installed locally on first run.
	out, _ := e.RunCommand("pulumi", "plugin", "ls")
	if strings.Contains(out, "random  resource") {
		removeRandomFromLocalPlugins()
	}

	// get the schema and bind it
	schemaJSON, _ := e.RunCommand("pulumi", "package", "get-schema", "random")
	bindSchema(schemaJSON)

	// try again using a specific version
	removeRandomFromLocalPlugins()
	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", "random@4.13.0")
	bindSchema(schemaJSON)

	// Now that the random provider is installed, run the command again without removing random from plugins
	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", "random")
	bindSchema(schemaJSON)

	// Now try to get the schema from the path to the binary
	binaryPath := filepath.Join(
		os.Getenv("HOME"),
		".pulumi",
		"plugins",
		"resource-random-v4.13.0",
		"pulumi-resource-random")

	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", binaryPath)
	bindSchema(schemaJSON)
}

//nolint:paralleltest // disabled parallel because we change the plugins cache
func TestPackageGetMapping(t *testing.T) {
	// Skipping because it gets rate limited in CI
	t.Skip()
	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)
	removeRandomFromLocalPlugins := func() {
		e.RunCommand("pulumi", "plugin", "rm", "resource", "random", "--all", "--yes")
	}

	// Make sure the random provider is not installed locally
	// So that we can test the `package get-mapping` command works if the plugin
	// is not installed locally on first run.
	out, _ := e.RunCommand("pulumi", "plugin", "ls")
	if strings.Contains(out, "random  resource") {
		removeRandomFromLocalPlugins()
	}

	result, _ := e.RunCommand("pulumi", "package", "get-mapping", "terraform", "random@4.13.0", "--out", "mapping.json")
	require.Contains(t, result, "random@4.13.0 maps to provider random")
	contents, err := os.ReadFile(filepath.Join(e.RootPath, "mapping.json"))
	require.NoError(t, err, "Reading the generated tf mapping from file should work")
	require.NotNil(t, contents, "mapping contents should be non-empty")
}

// Quick sanity tests for each downstream language to check that import works.
//
//nolint:paralleltest // pulumi new is not parallel safe
func TestLanguageImportSmoke(t *testing.T) {
	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local filestate means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "new", Languages[runtime], "--yes")
			e.RunCommand("pulumi", "import", "--yes", "random:index/randomId:RandomId", "identifier", "p-9hUg")
		})
	}
}
