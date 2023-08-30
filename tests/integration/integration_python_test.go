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

//go:build (python || all) && !xplatform_acceptance

package ints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// This checks that error logs are not being emitted twice
func TestMissingMainDoesNotEmitStackTrace(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "missing-main"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Stdout:        stdout,
		Stderr:        stderr,
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// ensure `  error: ` is only being shown once by the program
			assert.NotContains(t, stdout.String()+stderr.String(), "Traceback")
		},
	})
}

// TestPrintfPython tests that we capture stdout and stderr streams properly, even when the last line lacks an \n.
func TestPrintfPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("printf", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick:                  true,
		ExtraRuntimeValidation: printfTestValidation,
	})
}

func TestStackOutputsPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_outputs", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure the checkpoint contains a single resource, the Stack, with two outputs.
			fmt.Printf("Deployment: %v", stackInfo.Deployment)
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 1, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				assert.Equal(t, 0, len(stackRes.Inputs))
				assert.Equal(t, 2, len(stackRes.Outputs))
				assert.Equal(t, "ABC", stackRes.Outputs["xyz"])
				assert.Equal(t, float64(42), stackRes.Outputs["foo"])
			}
		},
	})
}

// Tests basic configuration from the perspective of a Pulumi program.
func TestConfigBasicPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_basic", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a Pythonic value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super Pythonic secret is encrypted",
		},
		OrderedConfig: []integration.ConfigValue{
			{Key: "outer.inner", Value: "value", Path: true},
			{Key: "names[0]", Value: "a", Path: true},
			{Key: "names[1]", Value: "b", Path: true},
			{Key: "names[2]", Value: "c", Path: true},
			{Key: "names[3]", Value: "super secret name", Path: true, Secret: true},
			{Key: "servers[0].port", Value: "80", Path: true},
			{Key: "servers[0].host", Value: "example", Path: true},
			{Key: "a.b[0].c", Value: "true", Path: true},
			{Key: "a.b[1].c", Value: "false", Path: true},
			{Key: "tokens[0]", Value: "shh", Path: true, Secret: true},
			{Key: "foo.bar", Value: "don't tell", Path: true, Secret: true},
		},
	})
}

// Tests configuration error from the perspective of a Pulumi program.
func TestConfigMissingPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_missing", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotEmpty(t, stackInfo.Events)
			text1 := "Missing required configuration variable 'config_missing_py:notFound'"
			text2 := "\tplease set a value using the command `pulumi config set --secret config_missing_py:notFound <value>`"
			var found1, found2 bool
			for _, event := range stackInfo.Events {
				if event.DiagnosticEvent != nil && strings.Contains(event.DiagnosticEvent.Message, text1) {
					found1 = true
				}
				if event.DiagnosticEvent != nil && strings.Contains(event.DiagnosticEvent.Message, text2) {
					found2 = true
				}
			}
			assert.True(t, found1, "expected error %q", text1)
			assert.True(t, found2, "expected error %q", text2)
		},
	})
}

// Tests that accessing config secrets using non-secret APIs results in warnings being logged.
func TestConfigSecretsWarnPython(t *testing.T) {
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	t.Skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_secrets_warn", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"plainstr1":   "1",
			"plainstr2":   "2",
			"plainstr3":   "3",
			"plainstr4":   "4",
			"plainbool1":  "true",
			"plainbool2":  "true",
			"plainbool3":  "true",
			"plainbool4":  "true",
			"plainint1":   "1",
			"plainint2":   "2",
			"plainint3":   "3",
			"plainint4":   "4",
			"plainfloat1": "1.1",
			"plainfloat2": "2.2",
			"plainfloat3": "3.3",
			"plainfloat4": "4.4",
			"plainobj1":   "{}",
			"plainobj2":   "{}",
			"plainobj3":   "{}",
			"plainobj4":   "{}",
		},
		Secrets: map[string]string{
			"str1":   "1",
			"str2":   "2",
			"str3":   "3",
			"str4":   "4",
			"bool1":  "true",
			"bool2":  "true",
			"bool3":  "true",
			"bool4":  "true",
			"int1":   "1",
			"int2":   "2",
			"int3":   "3",
			"int4":   "4",
			"float1": "1.1",
			"float2": "2.2",
			"float3": "3.3",
			"float4": "4.4",
			"obj1":   "{}",
			"obj2":   "{}",
			"obj3":   "{}",
			"obj4":   "{}",
		},
		OrderedConfig: []integration.ConfigValue{
			{Key: "parent1.foo", Value: "plain1", Path: true},
			{Key: "parent1.bar", Value: "secret1", Path: true, Secret: true},
			{Key: "parent2.foo", Value: "plain2", Path: true},
			{Key: "parent2.bar", Value: "secret2", Path: true, Secret: true},
			{Key: "names1[0]", Value: "plain1", Path: true},
			{Key: "names1[1]", Value: "secret1", Path: true, Secret: true},
			{Key: "names2[0]", Value: "plain2", Path: true},
			{Key: "names2[1]", Value: "secret2", Path: true, Secret: true},
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotEmpty(t, stackInfo.Events)
			//nolint:lll
			expectedWarnings := []string{
				"Configuration 'config_secrets_python:str1' value is a secret; use `get_secret` instead of `get`",
				"Configuration 'config_secrets_python:str2' value is a secret; use `require_secret` instead of `require`",
				"Configuration 'config_secrets_python:bool1' value is a secret; use `get_secret_bool` instead of `get_bool`",
				"Configuration 'config_secrets_python:bool2' value is a secret; use `require_secret_bool` instead of `require_bool`",
				"Configuration 'config_secrets_python:int1' value is a secret; use `get_secret_int` instead of `get_int`",
				"Configuration 'config_secrets_python:int2' value is a secret; use `require_secret_int` instead of `require_int`",
				"Configuration 'config_secrets_python:float1' value is a secret; use `get_secret_float` instead of `get_float`",
				"Configuration 'config_secrets_python:float2' value is a secret; use `require_secret_float` instead of `require_float`",
				"Configuration 'config_secrets_python:obj1' value is a secret; use `get_secret_object` instead of `get_object`",
				"Configuration 'config_secrets_python:obj2' value is a secret; use `require_secret_object` instead of `require_object`",
				"Configuration 'config_secrets_python:parent1' value is a secret; use `get_secret_object` instead of `get_object`",
				"Configuration 'config_secrets_python:parent2' value is a secret; use `require_secret_object` instead of `require_object`",
				"Configuration 'config_secrets_python:names1' value is a secret; use `get_secret_object` instead of `get_object`",
				"Configuration 'config_secrets_python:names2' value is a secret; use `require_secret_object` instead of `require_object`",
			}
			for _, warning := range expectedWarnings {
				var found bool
				for _, event := range stackInfo.Events {
					if event.DiagnosticEvent != nil && event.DiagnosticEvent.Severity == "warning" &&
						strings.Contains(event.DiagnosticEvent.Message, warning) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected warning %q", warning)
			}

			// These keys should not be in any warning messages.
			unexpectedWarnings := []string{
				"plainstr1",
				"plainstr2",
				"plainstr3",
				"plainstr4",
				"plainbool1",
				"plainbool2",
				"plainbool3",
				"plainbool4",
				"plainint1",
				"plainint2",
				"plainint3",
				"plainint4",
				"plainfloat1",
				"plainfloat2",
				"plainfloat3",
				"plainfloat4",
				"plainobj1",
				"plainobj2",
				"plainobj3",
				"plainobj4",
				"str3",
				"str4",
				"bool3",
				"bool4",
				"int3",
				"int4",
				"float3",
				"float4",
				"obj3",
				"obj4",
			}
			for _, warning := range unexpectedWarnings {
				for _, event := range stackInfo.Events {
					if event.DiagnosticEvent != nil {
						assert.NotContains(t, event.DiagnosticEvent.Message, warning)
					}
				}
			}
		},
	})
}

func TestMultiStackReferencePython(t *testing.T) {
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}
	t.Parallel()

	// build a stack with an export
	exporterOpts := &integration.ProgramTestOptions{
		RequireService: true,

		Dir: filepath.Join("stack_reference_multi", "python", "exporter"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"org": os.Getenv("PULUMI_TEST_OWNER"),
		},
		DestroyOnCleanup: true,
	}
	exporterStackName := exporterOpts.GetStackName().String()
	integration.ProgramTest(t, exporterOpts)

	importerOpts := &integration.ProgramTestOptions{
		RequireService: true,

		Dir: filepath.Join("stack_reference_multi", "python", "importer"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"org":                 os.Getenv("PULUMI_TEST_OWNER"),
			"exporter_stack_name": exporterStackName,
		},
		DestroyOnCleanup: true,
	}
	integration.ProgramTest(t, importerOpts)
}

func TestResourceWithSecretSerializationPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("secret_outputs", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// The program exports three resources:
			//   1. One named `withSecret` who's prefix property should be secret, specified via `pulumi.secret()`.
			//   2. One named `withSecretAdditional` who's prefix property should be a secret, specified via
			//      additionalSecretOutputs.
			//   3. One named `withoutSecret` which should not be a secret.
			// We serialize both of the these as plain old objects, so they appear as maps in the output.
			withSecretProps, ok := stackInfo.Outputs["withSecret"].(map[string]interface{})
			assert.Truef(t, ok, "POJO output was not serialized as a map")

			withSecretAdditionalProps, ok := stackInfo.Outputs["withSecretAdditional"].(map[string]interface{})
			assert.Truef(t, ok, "POJO output was not serialized as a map")

			withoutSecretProps, ok := stackInfo.Outputs["withoutSecret"].(map[string]interface{})
			assert.Truef(t, ok, "POJO output was not serialized as a map")

			// The secret prop should have been serialized as a secret
			secretPropValue, ok := withSecretProps["prefix"].(map[string]interface{})
			assert.Truef(t, ok, "secret output was not serialized as a secret")
			assert.Equal(t, resource.SecretSig, secretPropValue[resource.SigKey].(string))

			// The other secret prop should have been serialized as a secret
			secretAdditionalPropValue, ok := withSecretAdditionalProps["prefix"].(map[string]interface{})
			assert.Truef(t, ok, "secret output was not serialized as a secret")
			assert.Equal(t, resource.SecretSig, secretAdditionalPropValue[resource.SigKey].(string))

			// And here, the prop was not set, it should just be a string value
			_, isString := withoutSecretProps["prefix"].(string)
			assert.Truef(t, isString, "non-secret output was not a string")
		},
	})
}

// Tests that we issue an error if we fail to locate the Python command when running
// a Python example.
func TestPython3NotInstalled(t *testing.T) {
	// TODO[pulumi/pulumi#6304]
	t.Skip("Temporarily skipping failing test - pulumi/pulumi#6304")
	stderr := &bytes.Buffer{}
	badPython := "python3000"
	expectedError := fmt.Sprintf(
		"Failed to locate any of %q on your PATH.  Have you installed Python 3.6 or greater?",
		[]string{badPython})
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Env: []string{
			// Note: we use PULUMI_PYTHON_CMD to override the default behavior of searching
			// for Python 3, since anyone running tests surely already has Python 3 installed on their
			// machine. The code paths are functionally the same.
			fmt.Sprintf("PULUMI_PYTHON_CMD=%s", badPython),
		},
		ExpectFailure: true,
		Stderr:        stderr,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stderr.String()
			assert.Contains(t, output, expectedError)
		},
	})
}

// Tests custom resource type name of dynamic provider in Python.
func TestCustomResourceTypeNameDynamicPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("dynamic", "python-resource-type-name"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			urnOut := stack.Outputs["urn"].(string)
			urn := resource.URN(urnOut)
			typ := urn.Type().String()
			assert.Equal(t, "pulumi-python:dynamic/custom-provider:CustomResource", typ)
		},
	})
}

func TestPartialValuesPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("partial_values", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		AllowEmptyPreviewChanges: true,
	})
}

// Tests a resource with a large (>4mb) string prop in Python
func TestLargeResourcePython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("large_resource", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
	})
}

// Test enum outputs
func TestEnumOutputsPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("enums", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)

			assert.Equal(t, "Burgundy", stack.Outputs["myTreeType"])
			assert.Equal(t, "Pulumi Planters Inc.foo", stack.Outputs["myTreeFarmChanged"])
			assert.Equal(t, "My Burgundy Rubber tree is from Pulumi Planters Inc.", stack.Outputs["mySentence"])
		},
	})
}

// Test to ensure Pylint is clean.
func TestPythonPylint(t *testing.T) {
	t.Skip("Temporarily skipping test - pulumi/pulumi#4849")
	var opts *integration.ProgramTestOptions
	opts = &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "pylint"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			randomURN := stack.Outputs["random_urn"].(string)
			assert.NotEmpty(t, randomURN)

			randomID := stack.Outputs["random_id"].(string)
			randomVal := stack.Outputs["random_val"].(string)
			assert.Equal(t, randomID, randomVal)

			cwd := stack.Outputs["cwd"].(string)
			assert.NotEmpty(t, cwd)

			pylint := filepath.Join("venv", "bin", "pylint")
			if runtime.GOOS == WindowsOS {
				pylint = filepath.Join("venv", "Scripts", "pylint")
			}

			err := integration.RunCommand(t, "pylint", []string{pylint, "__main__.py"}, cwd, opts)
			assert.NoError(t, err)
		},
		Quick: true,
	}
	integration.ProgramTest(t, opts)
}

// Test Python SDK codegen to ensure <Resource>Args and traditional keyword args work.
func TestPythonResourceArgs(t *testing.T) {
	testdir := filepath.Join("python", "resource_args")

	// Generate example library from schema.
	schemaBytes, err := os.ReadFile(filepath.Join(testdir, "schema.json"))
	assert.NoError(t, err)
	var spec schema.PackageSpec
	assert.NoError(t, json.Unmarshal(schemaBytes, &spec))
	pkg, err := schema.ImportSpec(spec, nil)
	assert.NoError(t, err)
	files, err := pygen.GeneratePackage("test", pkg, map[string][]byte{})
	assert.NoError(t, err)
	outdir := filepath.Join(testdir, "lib")
	assert.NoError(t, os.RemoveAll(outdir))
	for f, contents := range files {
		outfile := filepath.Join(outdir, f)
		assert.NoError(t, os.MkdirAll(filepath.Dir(outfile), 0o755))
		if outfile == filepath.Join(outdir, "setup.py") {
			contents = []byte(strings.ReplaceAll(string(contents), "${VERSION}", "0.0.1"))
		}
		assert.NoError(t, os.WriteFile(outfile, contents, 0o600))
	}
	assert.NoError(t, os.WriteFile(filepath.Join(outdir, "README.md"), []byte(""), 0o600))

	// Test the program.
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: testdir,
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
			filepath.Join(testdir, "lib"),
		},
		Quick: true,
	})
}

// Test to ensure that internal stacks are hidden
func TestPythonStackTruncate(t *testing.T) {
	cases := []string{
		"normal",
		"main_specified",
		"main_dir_specified",
	}

	for _, name := range cases {
		// Test the program.
		t.Run(name, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join("python", "stack_truncate", name),
				Dependencies: []string{
					filepath.Join("..", "..", "sdk", "python", "env", "src"),
				},
				Quick: true,
				// This test should fail because it raises an exception
				ExpectFailure: true,
				// We need to validate that the failure has a truncated stack trace
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Ensure that we have a non-empty list of events.
					assert.NotEmpty(t, stackInfo.Events)

					const stacktraceLinePrefix = "  File "

					// get last DiagnosticEvent containing python stack trace
					stackTraceMessage := ""
					for _, e := range stackInfo.Events {
						if e.DiagnosticEvent == nil {
							continue
						}
						msg := e.DiagnosticEvent.Message
						if !strings.Contains(msg, "Traceback") {
							continue
						}
						if !strings.Contains(msg, stacktraceLinePrefix) {
							continue
						}
						stackTraceMessage = msg
					}
					assert.NotEqual(t, "", stackTraceMessage)

					// make sure that the stack trace contains 2 frames as per the
					// program we're running
					numStackTraces := strings.Count(stackTraceMessage, stacktraceLinePrefix)
					assert.Equal(t, 2, numStackTraces)
				},
			})
		})
	}
}

// Test remote component construction with a child resource that takes a long time to be created, ensuring it's created.
func TestConstructSlowPython(t *testing.T) {
	localProvider := testComponentSlowLocalProvider(t)

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the Python program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	testDir := "construct_component_slow"
	runComponentSetup(t, testDir)

	opts := &integration.ProgramTestOptions{
		Env: []string{testYarnLinkPulumiEnv},
		Dir: filepath.Join(testDir, "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		LocalProviders: []integration.LocalDependency{localProvider},
		Quick:          true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 5, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))
			}
		},
	}
	integration.ProgramTest(t, opts)
}

// Test remote component construction with prompt inputs.
func TestConstructPlainPython(t *testing.T) {
	t.Parallel()

	testDir := "construct_component_plain"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir          string
		expectedResourceCount int
		env                   []string
	}{
		{
			componentDir:          "testcomponent",
			expectedResourceCount: 9,
			// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
			// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
			// module to run `yarn install && yarn link @pulumi/pulumi` in the Go program's directory, allowing
			// the Node.js dynamic provider plugin to load.
			// When the underlying issue has been fixed, the use of this environment variable inside the integration
			// test module should be removed.
			env: []string{"PULUMI_TEST_YARN_LINK_PULUMI=true"},
		},
		{
			componentDir:          "testcomponent-python",
			expectedResourceCount: 9,
		},
		{
			componentDir:          "testcomponent-go",
			expectedResourceCount: 8, // One less because no dynamic provider.
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProviders := []integration.LocalDependency{
				{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
			}
			integration.ProgramTest(t,
				optsForConstructPlainPython(t, test.expectedResourceCount, localProviders, test.env...))
		})
	}
}

func optsForConstructPlainPython(t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency,
	env ...string,
) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component_plain", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		LocalProviders: localProviders,
		Quick:          true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources))
		},
	}
}

// Test remote component inputs properly handle unknowns.
func TestConstructUnknownPython(t *testing.T) {
	testConstructUnknown(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

// Test methods on remote components.
func TestConstructMethodsPython(t *testing.T) {
	t.Parallel()

	testDir := "construct_component_methods"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProvider := integration.LocalDependency{
				Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join(testDir, "python"),
				Dependencies: []string{
					filepath.Join("..", "..", "sdk", "python", "env", "src"),
				},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestConstructMethodsUnknownPython(t *testing.T) {
	testConstructMethodsUnknown(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

func TestConstructMethodsResourcesPython(t *testing.T) {
	testConstructMethodsResources(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

func TestConstructMethodsErrorsPython(t *testing.T) {
	testConstructMethodsErrors(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

func TestConstructMethodsProviderPython(t *testing.T) {
	testConstructMethodsProvider(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

func TestConstructProviderPython(t *testing.T) {
	t.Parallel()

	const testDir = "construct_component_provider"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProvider := integration.LocalDependency{
				Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join(testDir, "python"),
				Dependencies: []string{
					filepath.Join("..", "..", "sdk", "python", "env", "src"),
				},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "hello world", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestGetResourcePython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("get_resource", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		AllowEmptyPreviewChanges: true,
	})
}

func TestPythonAwaitOutputs(t *testing.T) {
	t.Parallel()

	t.Run("SuccessSimple", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "success"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicStringMessage := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "magic string") {
							sawMagicStringMessage = true
						}
					}
				}
				assert.True(t, sawMagicStringMessage, "Did not see printed message from unexported output")
			},
		})
	})

	t.Run("SuccessMultipleOutputs", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "multiple_outputs"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicString := false
				sawFoo := false
				sawBar := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "magic string") {
							sawMagicString = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "bar") {
							sawBar = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				msg := "Did not see printed message from unexported output"
				assert.True(t, sawMagicString, msg)
				assert.True(t, sawFoo, msg)
				assert.True(t, sawBar, msg)
			},
		})
	})

	t.Run("CreateWithinApply", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "create_inside_apply"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawUrn := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "pulumi-python:dynamic:Resource::magic_string") {
							sawUrn = true
						}
					}
				}
				assert.True(t, sawUrn)
			},
		})
	})

	t.Run("ErrorHandlingSuccess", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "error_handling"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicStringMessage := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "oh yeah") {
							sawMagicStringMessage = true
						}
					}
				}
				assert.True(t, sawMagicStringMessage, "Did not see printed message from unexported output")
			},
		})
	})

	t.Run("FailureSimple", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
			},
		})
	})

	t.Run("FailureWithExportedOutput", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure_exported_output"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
				sawFoo := false
				sawPrinted := false
				sawNotPrinted := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "not printed") {
							sawNotPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "printed") {
							sawPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				assert.True(t, sawPrinted)
				assert.True(t, sawFoo)
				assert.False(t, sawNotPrinted)
			},
		})
	})

	t.Run("FailureMultipleOutputs", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure_multiple_unexported_outputs"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
				sawFoo := false
				sawPrinted := false
				sawNotPrinted := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "not printed") {
							sawNotPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "printed") {
							sawPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				assert.True(t, sawPrinted)
				assert.True(t, sawFoo)
				assert.False(t, sawNotPrinted)
			},
		})
	})
}

// Test dict key translations.
func TestPythonTranslation(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "translation"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
	})
}

func TestComponentProviderSchemaPython(t *testing.T) {
	// TODO[https://github.com/pulumi/pulumi/issues/12365] we no longer have shim files so there's no native
	// binary for the testComponentProviderSchema to just exec. It _ought_ to be rewritten to use the plugin
	// host framework so that it starts the component up the same as all the other tests are doing (via
	// shimless).
	t.Skip("testComponentProviderSchema needs to be updated to use a plugin host to deal with non-native-binary providers")

	path := filepath.Join("component_provider_schema", "testcomponent-python", "pulumi-resource-testcomponent")
	if runtime.GOOS == WindowsOS {
		path += ".cmd"
	}
	testComponentProviderSchema(t, path)
}

// Test that the about command works as expected. Because about parses the
// results of each runtime independently, we have an integration test in each
// language.
func TestAboutPython(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("about", "python")

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironmentFallible()
		}
	}()
	e.ImportDirectory(dir)

	stdout, _ := e.RunCommand("pulumi", "about", "--json")
	// Assert we parsed the dependencies
	assert.Contains(t, stdout, "pulumi-kubernetes")
}

func TestConstructOutputValuesPython(t *testing.T) {
	testConstructOutputValues(t, "python", filepath.Join("..", "..", "sdk", "python", "env", "src"))
}

// TestResourceRefsGetResourcePython tests that invoking the built-in 'pulumi:pulumi:getResource' function
// returns resource references for any resource reference in a resource's state.
func TestResourceRefsGetResourcePython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("resource_refs_get_resource", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
	})
}

// TestDeletedWithPython tests the DeletedWith resource option.
func TestDeletedWithPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("deleted_with", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		Quick: true,
	})
}

func TestConstructProviderPropagationPython(t *testing.T) {
	t.Parallel()

	testConstructProviderPropagation(t, "python", []string{
		filepath.Join("..", "..", "sdk", "python", "env", "src"),
	})
}

// Regression test for https://github.com/pulumi/pulumi/issues/9411
func TestDuplicateOutputPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "duplicate-output"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			expected := []interface{}{float64(1), float64(2)}
			assert.Equal(t, expected, stack.Outputs["export1"])
			assert.Equal(t, expected, stack.Outputs["export2"])
		},
	})
}

func TestConstructProviderExplicitPython(t *testing.T) {
	t.Parallel()

	testConstructProviderExplicit(t, "python", []string{
		filepath.Join("..", "..", "sdk", "python", "env", "src"),
	})
}
