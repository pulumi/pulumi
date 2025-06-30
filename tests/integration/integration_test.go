// Copyright 2016-2023, Pulumi Corporation.
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

//go:build all

package ints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

// TestStackTagValidation verifies various error scenarios related to stack names and tags.
func TestStackTagValidation(t *testing.T) {
	t.Parallel()

	t.Run("Error_StackName", func(t *testing.T) {
		t.Parallel()
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "invalid name (spaces, parens, etc.)")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr,
			"a stack name may only contain alphanumeric, hyphens, underscores, or periods")
	})

	t.Run("Error_DescriptionLength", func(t *testing.T) {
		t.Parallel()

		// This test requires the service, as only the service supports stack tags.
		if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
			t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
		}
		if os.Getenv("PULUMI_TEST_OWNER") == "" {
			t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
		}

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		stackName, err := resource.NewUniqueHex("test-", 8, -1)
		contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

		e.RunCommand("git", "init")
		e.ImportDirectory("stack_project_name")

		prefix := "lorem ipsum dolor sit amet"     // 26
		prefix = prefix + prefix + prefix + prefix // 104
		prefix = prefix + prefix + prefix + prefix // 416 + the current Pulumi.yaml's description

		// Change the contents of the Description property of Pulumi.yaml.
		yamlPath := filepath.Join(e.CWD, "Pulumi.yaml")
		err = integration.ReplaceInFile("description: ", "description: "+prefix, yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", stackName)
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack tag \"pulumi:description\" value is too long (max length 256 characters)")
	})
}

// TestStackInitValidation verifies various error scenarios related to init'ing a stack.
func TestStackInitValidation(t *testing.T) {
	t.Parallel()

	t.Run("Error_InvalidStackYaml", func(t *testing.T) {
		t.Parallel()
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		// Starting a yaml value with a quote string and then more data is invalid
		invalidYaml := "\"this is invalid\" yaml because of trailing data after quote string"

		// Change the contents of the Description property of Pulumi.yaml.
		yamlPath := filepath.Join(e.CWD, "Pulumi.yaml")
		err := integration.ReplaceInFile("description: ", "description: "+invalidYaml, yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "valid-name")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "invalid YAML file")
	})
}

// TestConfigPaths ensures that config commands with paths work as expected.
func TestConfigPaths(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Initialize an empty stack.
	path := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := (&workspace.Project{
		Name:    "testing-config",
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}).Save(path)
	assert.NoError(t, err)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "testing")

	namespaces := []string{"", "my:"}

	tests := []struct {
		Key                   string
		Value                 string
		Secret                bool
		Path                  bool
		TopLevelKey           string
		TopLevelExpectedValue string
	}{
		{
			Key:                   "aConfigValue",
			Value:                 "this value is a value",
			TopLevelKey:           "aConfigValue",
			TopLevelExpectedValue: "this value is a value",
		},
		{
			Key:                   "anotherConfigValue",
			Value:                 "this value is another value",
			TopLevelKey:           "anotherConfigValue",
			TopLevelExpectedValue: "this value is another value",
		},
		{
			Key:                   "bEncryptedSecret",
			Value:                 "this super secret is encrypted",
			Secret:                true,
			TopLevelKey:           "bEncryptedSecret",
			TopLevelExpectedValue: "this super secret is encrypted",
		},
		{
			Key:                   "anotherEncryptedSecret",
			Value:                 "another encrypted secret",
			Secret:                true,
			TopLevelKey:           "anotherEncryptedSecret",
			TopLevelExpectedValue: "another encrypted secret",
		},
		{
			Key:                   "[]",
			Value:                 "square brackets value",
			TopLevelKey:           "[]",
			TopLevelExpectedValue: "square brackets value",
		},
		{
			Key:                   "x.y",
			Value:                 "x.y value",
			TopLevelKey:           "x.y",
			TopLevelExpectedValue: "x.y value",
		},
		{
			Key:                   "0",
			Value:                 "0 value",
			Path:                  true,
			TopLevelKey:           "0",
			TopLevelExpectedValue: "0 value",
		},
		{
			Key:                   "true",
			Value:                 "value",
			Path:                  true,
			TopLevelKey:           "true",
			TopLevelExpectedValue: "value",
		},
		{
			Key:                   `["test.Key"]`,
			Value:                 "test key value",
			Path:                  true,
			TopLevelKey:           "test.Key",
			TopLevelExpectedValue: "test key value",
		},
		{
			Key:                   `nested["test.Key"]`,
			Value:                 "nested test key value",
			Path:                  true,
			TopLevelKey:           "nested",
			TopLevelExpectedValue: `{"test.Key":"nested test key value"}`,
		},
		{
			Key:                   "outer.inner",
			Value:                 "value",
			Path:                  true,
			TopLevelKey:           "outer",
			TopLevelExpectedValue: `{"inner":"value"}`,
		},
		{
			Key:                   "names[0]",
			Value:                 "a",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a"]`,
		},
		{
			Key:                   "names[1]",
			Value:                 "b",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b"]`,
		},
		{
			Key:                   "names[2]",
			Value:                 "c",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b","c"]`,
		},
		{
			Key:                   "names[3]",
			Value:                 "super secret name",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b","c","super secret name"]`,
		},
		{
			Key:                   "servers[0].port",
			Value:                 "80",
			Path:                  true,
			TopLevelKey:           "servers",
			TopLevelExpectedValue: `[{"port":80}]`,
		},
		{
			Key:                   "servers[0].host",
			Value:                 "example",
			Path:                  true,
			TopLevelKey:           "servers",
			TopLevelExpectedValue: `[{"host":"example","port":80}]`,
		},
		{
			Key:                   "a.b[0].c",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "a",
			TopLevelExpectedValue: `{"b":[{"c":true}]}`,
		},
		{
			Key:                   "a.b[1].c",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "a",
			TopLevelExpectedValue: `{"b":[{"c":true},{"c":false}]}`,
		},
		{
			Key:                   "tokens[0]",
			Value:                 "shh",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "tokens",
			TopLevelExpectedValue: `["shh"]`,
		},
		{
			Key:                   "foo.bar",
			Value:                 "don't tell",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "foo",
			TopLevelExpectedValue: `{"bar":"don't tell"}`,
		},
		{
			Key:                   "semiInner.a.b.c.d",
			Value:                 "1",
			Path:                  true,
			TopLevelKey:           "semiInner",
			TopLevelExpectedValue: `{"a":{"b":{"c":{"d":1}}}}`,
		},
		{
			Key:                   "wayInner.a.b.c.d.e.f.g.h.i.j.k",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "wayInner",
			TopLevelExpectedValue: `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":false}}}}}}}}}}}`,
		},
		{
			Key:                   "foo1[0]",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "foo1",
			TopLevelExpectedValue: `[false]`,
		},
		{
			Key:                   "foo2[0]",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "foo2",
			TopLevelExpectedValue: `[true]`,
		},
		{
			Key:                   "foo3[0]",
			Value:                 "10",
			Path:                  true,
			TopLevelKey:           "foo3",
			TopLevelExpectedValue: `[10]`,
		},
		{
			Key:                   "foo4[0]",
			Value:                 "0",
			Path:                  true,
			TopLevelKey:           "foo4",
			TopLevelExpectedValue: `[0]`,
		},
		{
			Key:                   "foo5[0]",
			Value:                 "00",
			Path:                  true,
			TopLevelKey:           "foo5",
			TopLevelExpectedValue: `["00"]`,
		},
		{
			Key:                   "foo6[0]",
			Value:                 "01",
			Path:                  true,
			TopLevelKey:           "foo6",
			TopLevelExpectedValue: `["01"]`,
		},
		{
			Key:                   "foo7[0]",
			Value:                 "0123456",
			Path:                  true,
			TopLevelKey:           "foo7",
			TopLevelExpectedValue: `["0123456"]`,
		},
		{
			Key:                   "bar1.inner",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "bar1",
			TopLevelExpectedValue: `{"inner":false}`,
		},
		{
			Key:                   "bar2.inner",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "bar2",
			TopLevelExpectedValue: `{"inner":true}`,
		},
		{
			Key:                   "bar3.inner",
			Value:                 "10",
			Path:                  true,
			TopLevelKey:           "bar3",
			TopLevelExpectedValue: `{"inner":10}`,
		},
		{
			Key:                   "bar4.inner",
			Value:                 "0",
			Path:                  true,
			TopLevelKey:           "bar4",
			TopLevelExpectedValue: `{"inner":0}`,
		},
		{
			Key:                   "bar5.inner",
			Value:                 "00",
			Path:                  true,
			TopLevelKey:           "bar5",
			TopLevelExpectedValue: `{"inner":"00"}`,
		},
		{
			Key:                   "bar6.inner",
			Value:                 "01",
			Path:                  true,
			TopLevelKey:           "bar6",
			TopLevelExpectedValue: `{"inner":"01"}`,
		},
		{
			Key:                   "bar7.inner",
			Value:                 "0123456",
			Path:                  true,
			TopLevelKey:           "bar7",
			TopLevelExpectedValue: `{"inner":"0123456"}`,
		},

		// Overwriting a top-level string value is allowed.
		{
			Key:                   "aConfigValue.inner",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "aConfigValue",
			TopLevelExpectedValue: `{"inner":"new value"}`,
		},
		{
			Key:                   "anotherConfigValue[0]",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "anotherConfigValue",
			TopLevelExpectedValue: `["new value"]`,
		},
		{
			Key:                   "bEncryptedSecret.inner",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "bEncryptedSecret",
			TopLevelExpectedValue: `{"inner":"new value"}`,
		},
		{
			Key:                   "anotherEncryptedSecret[0]",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "anotherEncryptedSecret",
			TopLevelExpectedValue: `["new value"]`,
		},
	}

	validateConfigGet := func(key string, value string, path bool) {
		args := []string{"config", "get", key}
		if path {
			args = append(args, "--path")
		}
		stdout, stderr := e.RunCommand("pulumi", args...)
		assert.Equal(t, value+"\n", stdout)
		assert.Equal(t, "", stderr)
	}

	for _, ns := range namespaces {
		for _, test := range tests {
			key := fmt.Sprintf("%s%s", ns, test.Key)
			topLevelKey := fmt.Sprintf("%s%s", ns, test.TopLevelKey)

			// Set the value.
			args := []string{"config", "set"}
			if test.Secret {
				args = append(args, "--secret")
			}
			if test.Path {
				args = append(args, "--path")
			}
			args = append(args, key, test.Value)
			stdout, stderr := e.RunCommand("pulumi", args...)
			assert.Equal(t, "", stdout)
			assert.Equal(t, "", stderr)

			// Get the value and validate it.
			validateConfigGet(key, test.Value, test.Path)

			// Get the top-level value and validate it.
			validateConfigGet(topLevelKey, test.TopLevelExpectedValue, false /*path*/)
		}
	}

	badKeys := []string{
		// Syntax errors.
		"root[",
		`root["nested]`,
		"root.array[abc]",

		// First path segment must be a non-empty string.
		`[""]`,
		"[0]",
		".foo",
		".[0]",

		// Index out of range.
		"names[-1]",
		"names[5]",

		// A "secure" key that is a map with a single string value is reserved by the system.
		"key.secure",
		"super.nested.map.secure",

		// Type mismatch.
		"outer[0]",
		"names.nested",
		"outer.inner.nested",
		"outer.inner[0]",
	}

	for _, ns := range namespaces {
		for _, badKey := range badKeys {
			key := fmt.Sprintf("%s%s", ns, badKey)
			stdout, stderr := e.RunCommandExpectError("pulumi", "config", "set", "--path", key, "value")
			assert.Equal(t, "", stdout)
			assert.NotEqual(t, "", stderr)
		}
	}

	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

func testDestroyStackRef(e *ptesting.Environment, organization string) {
	e.ImportDirectory("large_resource/nodejs")

	stackName, err := resource.NewUniqueHex("rm-test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	if organization != "" {
		qualifiedStackName := fmt.Sprintf("%s/%s", organization, stackName)
		e.RunCommand("pulumi", "stack", "init", qualifiedStackName)
	} else {
		e.RunCommand("pulumi", "stack", "init", stackName)
	}

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")
	e.CWD = os.TempDir()
	stackRef := stackName
	if organization != "" {
		stackRef = organization + "/large_resource_js/" + stackName
	}

	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "-s", stackRef)
	e.RunCommand("pulumi", "stack", "rm", "--yes", "-s", stackRef)
}

func TestDestroyStackRef_LocalProject(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	testDestroyStackRef(e, "organization")
}

func TestDestroyStackRef_LocalNonProject_NewEnv(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.Env = []string{"PULUMI_DIY_BACKEND_LEGACY_LAYOUT=true"}
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	testDestroyStackRef(e, "")
}

func TestDestroyStackRef_LocalNonProject_OldEnv(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.Env = []string{"PULUMI_SELF_MANAGED_STATE_LEGACY_LAYOUT=true"}
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	testDestroyStackRef(e, "")
}

func TestDestroyStackRef_Cloud(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	output, _ := e.RunCommand("pulumi", "whoami")
	organization := strings.TrimSpace(output)
	testDestroyStackRef(e, organization)
}

//nolint:paralleltest // uses parallel programtest
func TestJSONOutput(t *testing.T) {
	stdout := &bytes.Buffer{}

	// Test without env var for streaming preview (should print previewSummary).
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_outputs", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Stdout:       stdout,
		Verbose:      true,
		JSONOutput:   true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			output := stdout.String()

			// Check that the previewSummary is present.
			assert.Regexp(t, previewSummaryRegex, output)

			// Check that each event present in the event stream is also in stdout.
			for _, evt := range stack.Events {
				assertOutputContainsEvent(t, evt, output)
			}
		},
	})
}

func TestProviderDownloadURL(t *testing.T) {
	t.Parallel()

	validate := func(t *testing.T, stdout []byte) {
		deployment := &apitype.UntypedDeployment{}
		err := json.Unmarshal(stdout, deployment)
		assert.NoError(t, err)
		data := &apitype.DeploymentV3{}
		err = json.Unmarshal(deployment.Deployment, data)
		assert.NoError(t, err)
		urlKey := "pluginDownloadURL"
		getPluginDownloadURL := func(inputs map[string]interface{}) string {
			internal, ok := inputs["__internal"].(map[string]interface{})
			if ok {
				pluginDownloadURL, _ := internal[urlKey].(string)
				return pluginDownloadURL
			}
			return ""
		}
		for _, resource := range data.Resources {
			switch {
			case providers.IsDefaultProvider(resource.URN):
				assert.Equalf(t, "get.example.test", getPluginDownloadURL(resource.Inputs), "Inputs")
				assert.Equalf(t, "", getPluginDownloadURL(resource.Outputs), "Outputs")
				assert.Equalf(t, nil, resource.Inputs[urlKey], "Outputs")
				assert.Equalf(t, nil, resource.Outputs[urlKey], "Outputs")
			case providers.IsProviderType(resource.Type):
				assert.Equalf(t, "get.pulumi.test/providers", getPluginDownloadURL(resource.Inputs), "Inputs")
				assert.Equalf(t, "", getPluginDownloadURL(resource.Outputs), "Outputs")
				assert.Equalf(t, nil, resource.Inputs[urlKey], "Outputs")
				assert.Equalf(t, nil, resource.Outputs[urlKey], "Outputs")
			default:
				_, hasURL := resource.Inputs[urlKey]
				assert.False(t, hasURL)
				_, hasURL = resource.Outputs[urlKey]
				assert.False(t, hasURL)
			}
		}
		assert.Greater(t, len(data.Resources), 1, "We should construct more then just the stack")
	}

	languages := []struct {
		name       string
		dependency string
	}{
		{"python", filepath.Join("..", "..", "sdk", "python")},
		{"nodejs", "@pulumi/pulumi"},
		{"go", "github.com/pulumi/pulumi/sdk/v3"},
	}

	//nolint:paralleltest // uses parallel programtest
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			dir := filepath.Join("gather_plugin", lang.name)
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:                    dir,
				ExportStateValidator:   validate,
				SkipPreview:            true,
				SkipEmptyPreviewUpdate: true,
				Dependencies:           []string{lang.dependency},
			})
		})
	}
}

func TestExcludeProtected(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory("exclude_protected")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "stack", "init", "dev")

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	stdout, _ := e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "--exclude-protected")
	assert.Contains(t, stdout, "All unprotected resources were destroyed. There are still 7 protected resources")
	// We run the command again, but this time there are not unprotected resources to destroy.
	stdout, _ = e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "--exclude-protected")
	assert.Contains(t, stdout, "There were no unprotected resources to destroy. There are still 7")
}

func TestUnprotect(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory("protect_resources/step1")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "stack", "init", "dev")

	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy")
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "exit status 0xffffffff")
	} else {
		assert.ErrorContains(t, err, "exit status 255")
	}

	e.RunCommand("pulumi", "state", "unprotect", "--all", "--yes")
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}

func TestUnprotectProtect(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory("protect_resources/step1")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "stack", "init", "dev")

	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy")
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "exit status 0xffffffff")
	} else {
		assert.ErrorContains(t, err, "exit status 255")
	}

	e.RunCommand("pulumi", "state", "unprotect", "--all", "--yes")
	e.RunCommand("pulumi", "state", "protect", "--all", "--yes")

	_, _, err = e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy")
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "exit status 0xffffffff")
	} else {
		assert.ErrorContains(t, err, "exit status 255")
	}
}

func TestInvalidPluginError(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: invalid-plugin
runtime: yaml
description: A Pulumi program referencing an invalid plugin.
plugins:
  providers:
    - name: fakeplugin
      bin: ./does/not/exist/bin # key should be 'path'
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	{
		_, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "invalid-resources")
		assert.NotContains(t, stderr, "panic: ")
		assert.Contains(t, stderr, "error: ")
	}
	{
		_, stderr := e.RunCommandExpectError("pulumi", "pre")
		assert.NotContains(t, stderr, "panic: ")
		assert.Contains(t, stderr, "error: ")
	}
}

// Regression test for https://github.com/pulumi/pulumi/issues/12632.
func TestPassphraseSetAllGet(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	e.Passphrase = "test-passphrase"
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: passphrase-test
runtime: yaml
description: A Pulumi program testing passphrase config.
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	// Init a new stack, then set a secret config value, then try to get it.
	e.RunCommand("pulumi", "stack", "init", "passphrase-test")
	// Clear the config file so that "config set-all" has to re-initialize the passphrase config.
	err := os.Remove(filepath.Join(e.RootPath, "Pulumi.passphrase-test.yaml"))
	require.NoError(t, err)
	// Set a secret config value, then try to get it.
	e.RunCommand("pulumi", "config", "set-all", "--secret", "foo=bar")
	stdout, _ := e.RunCommand("pulumi", "config", "get", "foo")
	assert.Contains(t, stdout, "bar")
}

// Similar to TestPassphraseSetAllGet but covering for "set" instead of "set-all".
func TestPassphraseSetGet(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	e.Passphrase = "test-passphrase"
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: passphrase-test
runtime: yaml
description: A Pulumi program testing passphrase config.
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	// Init a new stack, then set a secret config value, then try to get it.
	e.RunCommand("pulumi", "stack", "init", "passphrase-test")
	// Clear the config file so that "config set" has to re-initialize the passphrase config.
	err := os.Remove(filepath.Join(e.RootPath, "Pulumi.passphrase-test.yaml"))
	require.NoError(t, err)
	// Set a secret config value, then try to get it.
	e.RunCommand("pulumi", "config", "set", "--secret", "foo", "bar")
	stdout, _ := e.RunCommand("pulumi", "config", "get", "foo")
	assert.Contains(t, stdout, "bar")
}

// Tests that "config rm" and "config rm-all" work as expected.
func TestConfigRmRmAll(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	e.Passphrase = "test-passphrase"
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: config-rm-test
runtime: yaml
description: A Pulumi program testing config rm and config rm-all.
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "config-rm-test")
	// Clear the config file so that "config set" has to re-initialize the passphrase config.
	err := os.Remove(filepath.Join(e.RootPath, "Pulumi.config-rm-test.yaml"))
	require.NoError(t, err)

	configCmd := func(cmd string) func(args ...string) []string {
		return func(args ...string) []string {
			return append([]string{"config", cmd}, args...)
		}
	}

	configSet := configCmd("set")
	configSetAll := configCmd("set-all")
	configGet := configCmd("get")
	configRm := configCmd("rm")
	configRmAll := configCmd("rm-all")

	var stdout, stderr string

	// Set a single plaintext value, then try to remove it.
	e.RunCommand("pulumi", configSet("foo", "bar")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")

	// Set a single secret value, then try to remove it.
	e.RunCommand("pulumi", configSet("--secret", "foo", "bar")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")

	// Set multiple values, then remove one plaintext value.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")
	stdout, _ = e.RunCommand("pulumi", configGet("baz")...)
	assert.Contains(t, stdout, "quux")

	// Set multiple values, then remove one secret value.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRm("baz")...)
	stdout, _ = e.RunCommand("pulumi", configGet("foo")...)
	assert.Contains(t, stdout, "bar")
	_, stderr = e.RunCommandExpectError("pulumi", configGet("baz")...)
	assert.Contains(t, stderr, "'baz' not found")

	// Set multiple values, then remove them all.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRmAll("foo", "baz")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")
	_, stderr = e.RunCommandExpectError("pulumi", configGet("baz")...)
	assert.Contains(t, stderr, "'baz' not found")
}

// Tests that "config rm" and "config rm-all" work as expected with an explicit --config-file.
func TestConfigRmRmAllExplicitConfig(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	e.Passphrase = "test-passphrase"
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: config-rm-test
runtime: yaml
description: A Pulumi program testing config rm and config rm-all.
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "config-rm-test")
	configFile := t.TempDir() + "/Pulumi.config-rm-test.yaml"

	configCmd := func(cmd string) func(args ...string) []string {
		return func(args ...string) []string {
			return append([]string{"config", "--config-file", configFile, cmd}, args...)
		}
	}

	configSet := configCmd("set")
	configSetAll := configCmd("set-all")
	configGet := configCmd("get")
	configRm := configCmd("rm")
	configRmAll := configCmd("rm-all")

	var stdout, stderr string

	// Set a single plaintext value, then try to remove it.
	e.RunCommand("pulumi", configSet("foo", "bar")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")

	// Set a single secret value, then try to remove it.
	e.RunCommand("pulumi", configSet("--secret", "foo", "bar")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")

	// Set multiple values, then remove one plaintext value.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRm("foo")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")
	stdout, _ = e.RunCommand("pulumi", configGet("baz")...)
	assert.Contains(t, stdout, "quux")

	// Set multiple values, then remove one secret value.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRm("baz")...)
	stdout, _ = e.RunCommand("pulumi", configGet("foo")...)
	assert.Contains(t, stdout, "bar")
	_, stderr = e.RunCommandExpectError("pulumi", configGet("baz")...)
	assert.Contains(t, stderr, "'baz' not found")

	// Set multiple values, then remove them all.
	e.RunCommand("pulumi", configSetAll("--plaintext", "foo=bar", "--secret", "baz=quux")...)

	e.RunCommand("pulumi", configRmAll("foo", "baz")...)
	_, stderr = e.RunCommandExpectError("pulumi", configGet("foo")...)
	assert.Contains(t, stderr, "'foo' not found")
	_, stderr = e.RunCommandExpectError("pulumi", configGet("baz")...)
	assert.Contains(t, stderr, "'baz' not found")
}

// Regression test for https://github.com/pulumi/pulumi/issues/12593.
//
// Verifies that a "provider" option passed to a remote component
// is properly propagated to the component's children.
//
// Language-specific tests should call this function with the
// appropriate parameters.
func testConstructProviderPropagation(t *testing.T, lang string, deps []string) {
	const (
		testDir      = "construct_component_provider_propagation"
		componentDir = "testcomponent-go"
	)
	runComponentSetup(t, testDir)

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(testDir, lang),
		Dependencies: deps,
		LocalProviders: []integration.LocalDependency{
			{
				Package: "testcomponent",
				Path:    filepath.Join(testDir, componentDir),
			},
		},
		Quick:      true,
		NoParallel: true, // already called by tests
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			gotProviders := make(map[string]string) // resource name => provider name

			for _, res := range stackInfo.Deployment.Resources {
				if res.URN.Type() == "testprovider:index:Random" {
					ref, err := providers.ParseReference(res.Provider)
					assert.NoError(t, err)
					if err == nil {
						gotProviders[res.URN.Name()] = ref.URN().Name()
					}
				}
			}

			assert.Equal(t, map[string]string{
				"uses_default":       "default",
				"uses_provider":      "explicit",
				"uses_providers":     "explicit",
				"uses_providers_map": "explicit",
			}, gotProviders)
		},
	})
}

// Test to validate that various resource options are propagated for MLCs.
func testConstructResourceOptions(t *testing.T, dir string, deps []string) {
	const (
		testDir      = "construct_component_resource_options"
		componentDir = "testcomponent-go"
	)
	runComponentSetup(t, testDir)

	validate := func(t *testing.T, resources []apitype.ResourceV3) {
		urns := make(map[string]resource.URN) // name => URN
		for _, res := range resources {
			urns[res.URN.Name()] = res.URN
		}

		for _, res := range resources {
			switch name := res.URN.Name(); name {
			case "Protect":
				assert.True(t, res.Protect, "Protect(%s)", name)

			case "DependsOn":
				wantDeps := []resource.URN{urns["Dep1"], urns["Dep2"]}
				assert.ElementsMatch(t, wantDeps, res.Dependencies,
					"DependsOn(%s)", name)

			case "AdditionalSecretOutputs":
				assert.Equal(t,
					[]resource.PropertyKey{"foo"}, res.AdditionalSecretOutputs,
					"AdditionalSecretOutputs(%s)", name)

			case "CustomTimeouts":
				if ct := res.CustomTimeouts; assert.NotNil(t, ct, "CustomTimeouts(%s)", name) {
					assert.Equal(t, float64(60), ct.Create, "CustomTimeouts.Create(%s)", name)
					assert.Equal(t, float64(120), ct.Update, "CustomTimeouts.Update(%s)", name)
					assert.Equal(t, float64(180), ct.Delete, "CustomTimeouts.Delete(%s)", name)
				}

			case "DeletedWith":
				assert.Equal(t, urns["getDeletedWithMe"], res.DeletedWith, "DeletedWith(%s)", name)

			case "RetainOnDelete":
				assert.True(t, res.RetainOnDelete, "RetainOnDelete(%s)", name)
			}
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(testDir, dir),
		Dependencies: deps,
		LocalProviders: []integration.LocalDependency{
			{
				Package: "testcomponent",
				Path:    filepath.Join(testDir, componentDir),
			},
		},
		Quick:                   true,
		NoParallel:              true, // already called by tests
		DestroyExcludeProtected: true, // test contains protected resources
		SkipStackRemoval:        true, // protected resources prevent stack removal
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			validate(t, stackInfo.Deployment.Resources)
		},
	})
}

func testProjectRename(e *ptesting.Environment, organization string) {
	e.ImportDirectory("large_resource/nodejs")

	stackName, err := resource.NewUniqueHex("rm-test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	if organization != "" {
		qualifiedStackName := fmt.Sprintf("%s/%s", organization, stackName)
		e.RunCommand("pulumi", "stack", "init", qualifiedStackName)
	} else {
		e.RunCommand("pulumi", "stack", "init", stackName)
	}

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")
	newProjectName := "new_large_resource_js"
	stackRef := organization + "/" + newProjectName + "/" + stackName

	e.RunCommand("pulumi", "stack", "rename", stackRef)

	// Rename the project name in the yaml file
	projFilename := filepath.Join(e.CWD, "Pulumi.yaml")
	proj, err := workspace.LoadProject(projFilename)
	require.NoError(e, err)
	proj.Name = tokens.PackageName(newProjectName)
	err = proj.Save(projFilename)
	require.NoError(e, err)

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes", "--expect-no-changes", "-s", stackRef)
	e.RunCommand("pulumi", "stack", "rm", "--force", "--yes", "-s", stackRef)
}

func TestProjectRename_LocalProject(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	testProjectRename(e, "organization")
}

func TestProjectRename_Cloud(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	output, _ := e.RunCommand("pulumi", "whoami")
	organization := strings.TrimSpace(output)
	testProjectRename(e, organization)
}

//nolint:paralleltest // uses parallel programtest
func TestParentRename_issue13179(t *testing.T) {
	// This test is a reproduction of the issue reported in
	// https://github.com/pulumi/pulumi/issues/13179.
	//
	// It creates a stack with a resource that has a parent
	// and then renames the parent resource with 'pulumi state rename'.

	var parentURN resource.URN
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir: "state_rename_parent",
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		// Only run up:
		SkipRefresh: true,
		Quick:       true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			for _, res := range stackInfo.Deployment.Resources {
				if res.URN.Name() == "parent" {
					parentURN = res.URN
				}
			}
		},
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	t.Cleanup(pt.TestCleanUp)

	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")

	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")

	// PreviewUpdateAndEdits calls ExtraRuntimeValidation,
	// so we should have captured the parent URN.
	require.NotEmpty(t, parentURN, "no parent URN captured")

	// Rename the parent resource.
	require.NoError(t,
		pt.RunPulumiCommand("state", "rename", "-y", string(parentURN), "newParent"),
		"rename failed")
}

func testStackRmConfig(e *ptesting.Environment, organization string) {
	// We need to create two projects for this test
	goDir := filepath.Join(e.RootPath, "large_resource_go")
	err := os.Mkdir(goDir, 0o700)
	require.NoError(e, err)

	jsDir := filepath.Join(e.RootPath, "large_resource_js")
	err = os.Mkdir(jsDir, 0o700)
	require.NoError(e, err)

	stackName, err := resource.NewUniqueHex("rm-test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	qualifiedStackName := fmt.Sprintf("%s/%s", organization, stackName)
	// Create a stack in the go project
	e.CWD = goDir
	e.ImportDirectory("large_resource/go")
	e.RunCommand("pulumi", "stack", "init", qualifiedStackName)
	// Create a config value to ensure there's a Pulumi.<name>.yaml file.
	e.RunCommand("pulumi", "config", "set", "key", "value")

	// Now create the js project
	e.CWD = jsDir
	e.ImportDirectory("large_resource/nodejs")
	e.RunCommand("pulumi", "stack", "init", qualifiedStackName)
	// Create a config value to ensure there's a Pulumi.<name>.yaml file.
	e.RunCommand("pulumi", "config", "set", "key", "value")

	// Now try and remove the go stack while still in the js directory
	stackRef := organization + "/large_resource_go/" + stackName
	e.RunCommand("pulumi", "stack", "rm", "--yes", "-s", stackRef)

	// And check that Pulumi.<name>.yaml file is still there for the js project
	_, err = os.Stat(filepath.Join(jsDir, "Pulumi."+stackName+".yaml"))
	assert.NoError(e, err)
}

func TestStackRmConfig_LocalProject(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	testStackRmConfig(e, "organization")
}

func TestStackRmConfig_Cloud(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	output, _ := e.RunCommand("pulumi", "whoami")
	organization := strings.TrimSpace(output)
	testStackRmConfig(e, organization)
}

func TestAdvisoryPolicyPack(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("single_resource")
	e.ImportDirectory("policy")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	stackName, err := resource.NewUniqueHex("advisory-policy-pack", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	e.RunCommand("pulumi", "stack", "init", stackName)

	_, _, err = e.GetCommandResultsIn(filepath.Join(e.CWD, "advisory_policy_pack"), "npm", "install")
	assert.NoError(t, err)

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	stdout, _, err := e.GetCommandResults(
		"pulumi", "up", "--skip-preview", "--yes", "--policy-pack", "advisory_policy_pack")
	assert.NoError(t, err)
	assert.Contains(t, stdout, "Failing advisory policy pack for testing\n          foobar")
}

func TestMandatoryPolicyPack(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("single_resource")
	e.ImportDirectory("policy")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	stackName, err := resource.NewUniqueHex("mandatory-policy-pack", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	e.RunCommand("pulumi", "stack", "init", stackName)

	_, _, err = e.GetCommandResultsIn(filepath.Join(e.CWD, "mandatory_policy_pack"), "npm", "install")
	assert.NoError(t, err)

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	stdout, _, err := e.GetCommandResults(
		"pulumi", "up", "--skip-preview", "--yes", "--policy-pack", "mandatory_policy_pack")
	assert.Error(t, err)
	assert.Contains(t, stdout, "error: update failed")
	assert.Contains(t, stdout, "❌ typescript@v0.0.1 (local: mandatory_policy_pack)")
}

func TestMultiplePolicyPacks(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("single_resource")
	e.ImportDirectory("policy")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	stackName, err := resource.NewUniqueHex("multiple-policy-pack", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	e.RunCommand("pulumi", "stack", "init", stackName)

	_, _, err = e.GetCommandResultsIn(filepath.Join(e.CWD, "advisory_policy_pack"), "npm", "install")
	assert.NoError(t, err)
	_, _, err = e.GetCommandResultsIn(filepath.Join(e.CWD, "mandatory_policy_pack"), "npm", "install")
	assert.NoError(t, err)

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	stdout, _, err := e.GetCommandResults("pulumi", "up", "--skip-preview", "--yes",
		"--policy-pack", "advisory_policy_pack",
		"--policy-pack", "mandatory_policy_pack")
	assert.Error(t, err)
	assert.Contains(t, stdout, "Failing advisory policy pack for testing\n          foobar")
	assert.Contains(t, stdout, "error: update failed")
	assert.Contains(t, stdout, "❌ typescript@v0.0.1 (local: advisory_policy_pack; mandatory_policy_pack)")
}

// regression test for https://github.com/pulumi/pulumi/issues/11092
func TestPolicyPluginExtraArguments(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("single_resource")
	e.ImportDirectory("policy")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	stackName, err := resource.NewUniqueHex("policy-plugin-extra-args", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	assert.NoError(t, err)
	// Create a venv for the policy package and install the current python SDK into it
	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain:  toolchain.Pip,
		Root:       filepath.Join(e.CWD, "python_policy_pack"),
		Virtualenv: "venv",
	})
	require.NoError(t, err)
	require.NoError(t, tc.InstallDependencies(context.Background(),
		filepath.Join(e.CWD, "python_policy_pack"), false /*useLanguageVersionTools*/, false, /*showOutput */
		os.Stdout, os.Stderr))
	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "sdk", "python"))
	require.NoError(t, err)
	gotSdk, err := test.PathExists(sdkDir)
	require.NoError(t, err)
	if !gotSdk {
		t.Log("This test requires Python SDK to be built; please `cd sdk/python && make ensure build install`")
		t.FailNow()
	}
	cmd, err := tc.ModuleCommand(context.Background(), "pip", "install", "-e", sdkDir)
	require.NoError(t, err)
	require.NoError(t, cmd.Run())

	// Run with extra arguments
	_, _, err = e.GetCommandResults("pulumi", "--logflow", "preview", "--logtostderr",
		"--policy-pack", "python_policy_pack", "--tracing", "file:/trace.log")
	require.NoError(t, err)
}

func TestPolicyPackNewGenerateOnly(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	require.False(t, e.PathExists("venv"))
	stdout, _ := e.RunCommand("pulumi", "policy", "new", "aws-python", "--force", "--generate-only")
	require.Contains(t, stdout, "To install dependencies for the Policy Pack, run `pulumi install`")
	require.False(t, e.PathExists("venv"))
}

func TestPolicyPackNew(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	require.False(t, e.PathExists("venv"))
	stdout, _ := e.RunCommand("pulumi", "policy", "new", "aws-python", "--force")
	require.NotContains(t, stdout, "To install dependencies for the Policy Pack, run `pulumi install`")
	require.Contains(t, stdout, "Finished creating virtual environment")
	require.Contains(t, stdout, "Finished installing dependencies")
	require.True(t, e.PathExists("venv"))
}

func TestPolicyPackPublish(t *testing.T) {
	t.Parallel()
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	testOrg := os.Getenv("PULUMI_TEST_ORG")
	if testOrg == "" {
		t.Skipf("Skipping: PULUMI_TEST_ORG is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.RunCommand("pulumi", "policy", "new", "aws-typescript", "--force")
	// Change the name of the policy in case `aws-typescript` is already published
	// and so that we have a clear indication that this is coming from a test.
	// We do our best to clean up the policy after the test completes;
	name := fmt.Sprintf("test-policy-pack-publish-%d", time.Now().UnixNano())
	policyIndexPath := filepath.Join(e.RootPath, "index.ts")
	policyContent, err := os.ReadFile(policyIndexPath)
	require.NoError(t, err)
	newContent := strings.Replace(string(policyContent),
		`new PolicyPack("aws-typescript"`,
		fmt.Sprintf(`new PolicyPack("%s"`, name),
		1)
	err = os.WriteFile(policyIndexPath, []byte(newContent), 0o600)
	require.NoError(t, err)

	stdout, stderr := e.RunCommand("pulumi", "policy", "publish", testOrg)
	t.Logf("stdout: %s\nstderr: %s", stdout, stderr)

	stdout, stderr = e.RunCommand("pulumi", "policy", "rm", testOrg+"/"+name, "all", "--yes")
	t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
}

func TestPolicyPackInstallDependencies(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("policy/python_policy_pack")
	require.False(t, e.PathExists("venv"))
	stdout, _ := e.RunCommand("pulumi", "install")
	require.Contains(t, stdout, "Finished creating virtual environment")
	require.Contains(t, stdout, "Finished installing dependencies")
	require.True(t, e.PathExists("venv"))
}

func TestProjectInstallDependencies(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("single_resource")
	require.False(t, e.PathExists("node_modules"))
	stdout, _ := e.RunCommand("pulumi", "install")
	require.Contains(t, stdout, "Finished installing dependencies")
	require.True(t, e.PathExists("node_modules"))
}

func TestInstallDependenciesForPolicyPackWithParentProject(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	// create a directory structure with a pulumi project and a nested policy pack
	require.NoError(t, fsutil.CopyFile(e.RootPath, "nodejs/esm-ts", nil))
	policyPackPath := filepath.Join(e.RootPath, "policy")
	require.NoError(t, os.Mkdir(policyPackPath, 0o700))
	require.NoError(t, fsutil.CopyFile(policyPackPath, "policy/mandatory_policy_pack", nil))

	// chdir to the policy pack directory and run pulumi install
	e.CWD = policyPackPath
	stdout, _ := e.RunCommand("pulumi", "install")

	e.CWD = e.RootPath
	require.Contains(t, stdout, "Finished installing dependencies")
	require.False(t, e.PathExists("node_modules"), "node_modules should not exist in the root path")
	require.True(t, e.PathExists(filepath.Join("policy", "node_modules")), "node_modules should exist in the policy pack")
}

func TestInstallDependenciesProjectWithParentPolicyPack(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	// create a directory structure with a policy pack and a nested project
	require.NoError(t, fsutil.CopyFile(e.RootPath, "policy/mandatory_policy_pack", nil))
	projectPath := filepath.Join(e.RootPath, "project")
	require.NoError(t, os.Mkdir(projectPath, 0o700))
	require.NoError(t, fsutil.CopyFile(projectPath, "nodejs/esm-ts", nil))

	// chdir to the project directory and run pulumi install
	e.CWD = projectPath
	stdout, _ := e.RunCommand("pulumi", "install")

	e.CWD = e.RootPath
	require.Contains(t, stdout, "Finished installing dependencies")
	require.False(t, e.PathExists("node_modules"), "node_modules should not exist in the root path")
	require.True(t, e.PathExists(filepath.Join("project", "node_modules")),
		"node_modules should exist in the project path")
}

// Regression test for https://github.com/pulumi/pulumi/issues/17862
func TestPulumiNewLocalTemplatePath(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Clone the `python` folder from https://github.com/pulumi/templates into a temporary directory
	templateDir := e.TempDir()
	cwd := e.CWD
	e.CWD = templateDir
	e.RunCommand("git", "init")
	e.RunCommand("git", "remote", "add", "origin", "https://github.com/pulumi/templates")
	e.RunCommand("git", "config", "core.sparseCheckout", "true")
	e.RunCommand("git", "sparse-checkout", "set", "python")
	e.RunCommand("git", "pull", "origin", "master")
	e.CWD = cwd

	templatePath := filepath.Join(templateDir, "python")
	stdout, stderr := e.RunCommand("pulumi", "new", templatePath, "--generate-only", "--yes", "--force")
	require.Empty(t, stderr)
	require.Contains(t, stdout, "Your new project is ready to go")
}

func TestPulumiInstallInstallsPackagesIntoTheCorrectDirectory(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)

	e.ImportDirectory("packageadd-remote")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.Env = append(e.Env, "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false")
	e.RunCommand("pulumi", "stack", "select", "organization/packageadd-remote", "--create")

	e.RunCommand("pulumi", "package", "add",
		"github.com/pulumi/component-test-providers/test-provider@b39e20e4e33600e33073ccb2df0ddb46388641dc")

	// Remove the plugin from the local cache and try to install it using `pulumi install`
	e.RunCommand("pulumi", "plugin", "rm", "--all", "--yes")
	stdout, _ := e.RunCommand("pulumi", "plugin", "ls")
	require.NotContains(t, stdout, "github.com_pulumi_component-test-providers")

	e.RunCommand("pulumi", "install")
	stdout, _ = e.RunCommand("pulumi", "plugin", "ls")
	require.Contains(t, stdout, "github.com_pulumi_component-test-providers")
	require.Contains(t, stdout, "0.0.0-xb39e20e4e33600e33073ccb2df0ddb46388641dc")

	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview")
}

func TestOverrideComponentNamespace(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
	stdout, _ := e.RunCommand("pulumi", "package", "get-schema",
		"github.com/pulumi/component-test-providers/test-provider@b39e20e4e33600e33073ccb2df0ddb46388641dc")
	var packageSpec schema.PackageSpec
	err := json.Unmarshal([]byte(stdout), &packageSpec)
	require.NoError(t, err)
	// Without the override in the `package` command we'd expect the namespace to be empty here. Check that
	// it is 'pulumi', which we get from `github.com/*pulumi*/....
	require.Equal(t, "pulumi", packageSpec.Namespace)
}

func TestOverrideNamespaceLowercase(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
	stdout, _ := e.RunCommand("pulumi", "package", "get-schema",
		"github.com/Pulumi/component-test-providers/test-provider@b39e20e4e33600e33073ccb2df0ddb46388641dc")
	var packageSpec schema.PackageSpec
	err := json.Unmarshal([]byte(stdout), &packageSpec)
	require.NoError(t, err)
	// Without the override in the `package` command we'd expect the namespace to be empty here. Check that
	// it is 'pulumi', which we get from `github.com/*Pulumi*/....; note that namespaces only allow lowercasee
	// characters, so we lowercase it automatically.
	require.Equal(t, "pulumi", packageSpec.Namespace)
}

func TestTaggedComponent(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory(filepath.Join("external_tagged_component"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "stack", "init", "organization/external_tagged_component/test")
	e.Env = []string{"PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false"}
	e.RunCommand("pulumi", "install")
	stdout, _ := e.RunCommand("pulumi", "plugin", "ls")
	// Make sure we have exactly the plugin we expect installed
	assert.Equal(t, 1, strings.Count(stdout, "github.com_pulumi_pulumi-yaml.git"))

	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")

	stdout, _ = e.RunCommand("pulumi", "plugin", "ls")
	// Make sure we have no additional plugins after the up
	assert.Equal(t, 1, strings.Count(stdout, "github.com_pulumi_pulumi-yaml.git"))

	stdout, _ = e.RunCommand("pulumi", "stack", "output", "randomPet")
	// We expect 4 words separated by dashes.
	require.Equal(t, 4, len(strings.Split(stdout, "-")))
	require.Equal(t, "test-", stdout[:5])

	stdout, _ = e.RunCommand("pulumi", "stack", "output", "randomString")
	require.Len(t, strings.TrimSuffix(stdout, "\n"), 8, fmt.Sprintf("expected %s to have 8 characters", stdout))
}

func TestGetSchemaUsesCorrectVersion(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
	stdout, _ := e.RunCommand("pulumi", "package", "get-schema", "terraform-provider@0.10.0", "hashicorp/random", "3.6.0")
	var packageSpec schema.PackageSpec
	err := json.Unmarshal([]byte(stdout), &packageSpec)
	require.NoError(t, err)
	require.Equal(t, "3.6.0", packageSpec.Version)
	require.Equal(t, "0.10.0", packageSpec.Parameterization.BaseProvider.Version)

	stdout, _ = e.RunCommand("pulumi", "package", "get-schema", "terraform-provider", "hashicorp/random", "3.6.0")
	err = json.Unmarshal([]byte(stdout), &packageSpec)
	require.NoError(t, err)
	require.Equal(t, "3.6.0", packageSpec.Version)
}

// Regression test for https://github.com/pulumi/pulumi/issues/19905
func TestComponentProviderErrorInResourceRegistration(t *testing.T) {
	// We want to install the command provider
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	// The regression caused a hang where a remote component construct would
	// never return.
	timeout := time.After(3 * time.Minute)
	done := make(chan bool)
	go func() {
		pulumiHome := t.TempDir()
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			NoParallel:      true, // We're modifying the env above
			PulumiHomeDir:   pulumiHome,
			Dir:             "component-error-resource",
			RelativeWorkDir: "program",
			PrepareProject: func(info *engine.Projinfo) error {
				providerPath := filepath.Join(info.Root, "..", "provider")

				// Install command provider
				t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
				cmd := exec.Command("pulumi", "plugin", "install", "resource", "command", "1.0.4")
				cmd.Env = append(cmd.Environ(), "PULUMI_HOME="+pulumiHome)
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "%s failed with: %s", cmd.String(), string(out))

				// Install the provider's dependencies
				installNodejsProviderDependencies(t, providerPath)

				// Add the provider to our project
				cmd = exec.Command("pulumi", "package", "add", providerPath)
				cmd.Dir = info.Root
				cmd.Env = append(cmd.Environ(), "PULUMI_HOME="+pulumiHome)
				out, err = cmd.CombinedOutput()
				require.NoError(t, err, "%s failed with: %s", cmd.String(), string(out))

				return nil
			},
			Quick:         true,
			ExpectFailure: true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				foundError := false
				for _, event := range stack.Events {
					if event.DiagnosticEvent != nil && event.DiagnosticEvent.Severity == "error" {
						t.Logf("DiagnosticEvent.Message: %s", event.DiagnosticEvent.Message)
						if strings.Contains(event.DiagnosticEvent.Message, "exiting with error") {
							foundError = true
						}
					}
				}
				events, err := json.Marshal(stack.Events)
				require.NoError(t, err, "failed to marshal stack events")
				require.True(t, foundError, "expected to find an error in the stack events, got %s", events)
			},
		})

		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test didn't finish in time")
	case <-done:
	}
}
