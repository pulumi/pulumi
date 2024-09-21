// Copyright 2023-2024, Pulumi Corporation.
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

package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"gopkg.in/yaml.v2"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
	// make sure we can download needed plugins
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local url means we have a
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
//
//nolint:paralleltest // sets envvars
func TestYamlConvertSmoke(t *testing.T) {
	// make sure we can download the yaml converter plugin
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	e.ImportDirectory("testdata/random_yaml")

	// Make sure random is installed
	e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")

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
			e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand(
				"pulumi", "convert", "--strict",
				"--language", Languages[runtime], "--from", "pcl", "--out", "out")
			e.CWD = filepath.Join(e.RootPath, "out")
			e.RunCommand("pulumi", "stack", "init", "test")

			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "up", "--yes")
			e.RunCommand("pulumi", "destroy", "--yes")
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
			e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")

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
				t.Skip("java doesn't support components")
			}

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/component_pp")

			// Make sure random is installed
			e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "convert", "--language", Languages[runtime], "--from", "pcl", "--out", "out")
			e.CWD = filepath.Join(e.RootPath, "out")
			e.RunCommand("pulumi", "stack", "init", "test")

			// TODO(https://github.com/pulumi/pulumi/issues/14339): This doesn't work for Go yet because the
			// source code convert emits is not valid
			if runtime != "go" {
				e.RunCommand("pulumi", "install")
				e.RunCommand("pulumi", "up", "--yes")
				e.RunCommand("pulumi", "destroy", "--yes")
			}
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
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)
	removeRandomFromLocalPlugins := func() {
		e.RunCommand("pulumi", "plugin", "rm", "resource", "random", "--all", "--yes")
	}

	bindSchema := func(pkg, schemaJson string) *schema.Package {
		var schemaSpec *schema.PackageSpec
		err := json.Unmarshal([]byte(schemaJson), &schemaSpec)
		require.NoError(t, err, "Unmarshalling schema specs from %s should work", pkg)
		require.NotNil(t, schemaSpec, "Specification should be non-nil")
		schema, diags, err := schema.BindSpec(*schemaSpec, nil)
		require.NoError(t, err, "Binding the schema spec should work")
		require.False(t, diags.HasErrors(), "Binding schema spec should have no errors")
		require.NotNil(t, schema)
		return schema
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
	bindSchema("random", schemaJSON)

	// try again using a specific version
	removeRandomFromLocalPlugins()
	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", "random@4.13.0")
	bindSchema("random", schemaJSON)

	// Now that the random provider is installed, run the command again without removing random from plugins
	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", "random")
	bindSchema("random", schemaJSON)

	// Now try to get the schema from the path to the binary
	binaryPath := filepath.Join(
		e.HomePath,
		"plugins",
		"resource-random-v4.13.0",
		"pulumi-resource-random")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", binaryPath)
	bindSchema("random", schemaJSON)

	// Now try and get the parameterized schema from the test-provider
	providerDir, err := filepath.Abs("testprovider")
	require.NoError(t, err)
	schemaJSON, _ = e.RunCommand("pulumi", "package", "get-schema", providerDir, "parameter")
	schema := bindSchema("testprovider", schemaJSON)
	// Sub-schema is a very simple empty schema with the name set from the argument given
	assert.Equal(t, "parameter", schema.Name)
}

//nolint:paralleltest // disabled parallel because we change the plugins cache
func TestPackageGetMappingToFile(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

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

	stdout, result := e.RunCommand("pulumi",
		"package", "get-mapping", "terraform", "random@4.13.0",
		"--out", "mapping.json")
	require.Empty(t, stdout)
	require.Contains(t, result, "random@4.13.0 maps to provider random")
	contents, err := os.ReadFile(filepath.Join(e.RootPath, "mapping.json"))
	require.NoError(t, err, "Reading the generated tf mapping from file should work")
	require.NotNil(t, contents, "mapping contents should be non-empty")
}

//nolint:paralleltest // disabled parallel because we change the plugins cache
func TestPackageGetMapping(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

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

	schema, result := e.RunCommand("pulumi", "package", "get-mapping", "terraform", "random@4.13.0")
	require.Contains(t, result, "random@4.13.0 maps to provider random")
	require.NotEmpty(t, schema, "mapping contents should be non-empty")
}

// Quick sanity tests for each downstream language to check that import works.
//
//nolint:paralleltest // pulumi new is not parallel safe
func TestLanguageImportSmoke(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local url means we have a
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

// Test that PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION disables plugin acquisition in convert.
//
//nolint:paralleltest // changes env vars and plugin cache
func TestConvertDisableAutomaticPluginAcquisition(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	e.ImportDirectory("testdata/aws_tf")

	// Delete all cached plugins and disable plugin acquisition.
	e.RunCommand("pulumi", "plugin", "rm", "--all", "--yes")
	// Disable acquisition.
	e.SetEnvVars("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true")

	// This should fail because of no terraform converter
	_, stderr := e.RunCommandExpectError(
		"pulumi", "convert",
		"--language", "pcl", "--from", "terraform", "--out", "out")
	assert.Contains(t, stderr, "no converter plugin 'pulumi-converter-terraform' found")

	// Install a _specific_ version of the terraform converter (so this test doesn't change due to a new release)
	e.RunCommand("pulumi", "plugin", "install", "converter", "terraform", "v1.0.8")
	// This should now convert, but won't use our full aws tokens
	e.RunCommand(
		"pulumi", "convert",
		"--language", "pcl", "--from", "terraform", "--out", "out")

	output, err := os.ReadFile(filepath.Join(e.RootPath, "out", "main.pp"))
	require.NoError(t, err)
	// If we had an AWS plugin and mapping this would be "aws:ec2/instance:Instance"
	assert.Contains(t, string(output), "\"aws:index:instance\"")
}

// Small integration test for preview --import-file
func TestPreviewImportFile(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	e.ImportDirectory("testdata/import_node")

	// Make sure random is installed
	e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.12.0")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "preview", "--import-file", "import.json")

	expectedResources := []interface{}{
		map[string]interface{}{
			"id":      "<PLACEHOLDER>",
			"name":    "username",
			"type":    "random:index/randomPet:RandomPet",
			"version": "4.12.0",
		},
		map[string]interface{}{
			"name":      "component",
			"type":      "pkg:index:MyComponent",
			"component": true,
		},
		map[string]interface{}{
			"id":          "<PLACEHOLDER>",
			"logicalName": "username",
			// This isn't ideal, we don't really need to change the "name" here because it isn't used as a
			// parent, but currently we generate unique names for all resources rather than just unique names
			// for all parent resources.
			"name":    "usernameRandomPet",
			"type":    "random:index/randomPet:RandomPet",
			"version": "4.12.0",
			"parent":  "component",
		},
	}

	importBytes, err := os.ReadFile(filepath.Join(e.CWD, "import.json"))
	require.NoError(t, err)
	var actual map[string]interface{}
	err = json.Unmarshal(importBytes, &actual)
	require.NoError(t, err)
	assert.ElementsMatch(t, expectedResources, actual["resources"])
	_, has := actual["nameTable"]
	assert.False(t, has, "nameTable should not be present in import file")
}

// Small integration test for relative plugin paths. It's hard to do this test via the standard ProgramTest because that
// framework does it's own manipulation of plugin paths. Regression test for
// https://github.com/pulumi/pulumi/issues/15467.
func TestRelativePluginPath(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	// We can't use ImportDirectory here because we need to run this in the right directory such that the relative paths
	// work.
	var err error
	e.CWD, err = filepath.Abs("testdata/relative_plugin_node")
	require.NoError(t, err)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "preview")
}

// Quick sanity tests for https://github.com/pulumi/pulumi/issues/16248. Ensure we can run plugins and auto-fetch them.
//
//nolint:paralleltest // changes env vars
func TestPluginRun(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)

	removeRandomFromLocalPlugins := func() {
		e.RunCommand("pulumi", "plugin", "rm", "resource", "random", "--all", "--yes")
	}
	removeRandomFromLocalPlugins()

	_, stderr := e.RunCommandExpectError("pulumi", "plugin", "run", "--kind=resource", "random", "--", "--help")
	assert.Contains(t, stderr, "flag: help requested")
	_, stderr = e.RunCommandExpectError("pulumi", "plugin", "run", "--kind=resource", "random", "--", "--help")
	assert.Contains(t, stderr, "flag: help requested")
}

func TestInstall(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		// Reassign runtime before capture since it changes while looping.
		runtime := runtime

		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// Make sure the random provider is not installed locally
			// so that we can test the `install` command works.
			out, _ := e.RunCommand("pulumi", "plugin", "ls")
			if strings.Contains(out, "random  resource") {
				e.RunCommand("pulumi", "plugin", "rm", "resource", "random", "--all", "--yes")
			}

			// `new` wants to work in an empty directory but our use of local url means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			// Pass `--generate-only` so dependencies are not installed as part of the `new` command.
			e.RunCommand("pulumi", "new", "random-"+Languages[runtime], "--yes", "--generate-only")

			// Ensure `install` works and subsequent `up` and `destroy` operations work.
			_, stderr := e.RunCommand("pulumi", "install")
			assert.Regexp(t, regexp.MustCompile(`resource plugin random.+ installing`), stderr)
			e.RunCommand("pulumi", "stack", "init", "test")
			e.RunCommand("pulumi", "up", "--yes")
			e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// A smoke test to ensure that secrets providers that are persisted to state are
// used in favour of and to restore stack YAML configuration when it is absent
// or empty and the PULUMI_FALLBACK_TO_STATE_SECRETS_MANAGER environment variable
// is set.
//
//nolint:paralleltest // pulumi new is not parallel safe, and we set environment variables
func TestSecretsProvidersSmoke(t *testing.T) {
	// Make sure we can download needed plugins
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	// Ensure we have a passphrase set for the default secrets provider.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-passphrase")

	// Enable secrets manager fallback.
	t.Setenv("PULUMI_FALLBACK_TO_STATE_SECRETS_MANAGER", "true")

	operations := [][]string{
		{"up", "--yes"},
		{"preview"},
		{"refresh", "--yes"},
	}

	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local url means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "new", "random-"+Languages[runtime], "--yes")
			e.RunCommand("pulumi", "up", "--yes")

			stackJSONStr, _ := e.RunCommand("pulumi", "stack", "export")
			stackJSON := apitype.UntypedDeployment{}
			err = json.Unmarshal([]byte(stackJSONStr), &stackJSON)
			require.NoError(t, err)

			deployment := apitype.DeploymentV3{}
			err = json.Unmarshal(stackJSON.Deployment, &deployment)
			require.NoError(t, err)

			for _, operation := range operations {
				os.Remove(filepath.Join(projectDir, "Pulumi.dev.yaml"))
				e.RunCommand("pulumi", operation...)

				stackYamlStr, err := os.ReadFile(filepath.Join(projectDir, "Pulumi.dev.yaml"))
				require.NoError(t, err)

				stack := workspace.ProjectStack{}
				err = yaml.Unmarshal(stackYamlStr, &stack)
				require.NoError(t, err)

				require.NotEmpty(t, stack.EncryptionSalt)
				require.Contains(t, string(deployment.SecretsProviders.State), stack.EncryptionSalt)
			}

			e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}
