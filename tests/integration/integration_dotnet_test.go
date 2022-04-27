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

//go:build (dotnet || all) && !smoke

package ints

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

// TestPrintfDotNet tests that we capture stdout and stderr streams properly, even when the last line lacks an \n.
func TestPrintfDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    filepath.Join("printf", "dotnet"),
		Dependencies:           []string{"Pulumi"},
		Quick:                  true,
		ExtraRuntimeValidation: printfTestValidation,
	})
}

func TestStackOutputsDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_outputs", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
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

// TestStackComponentDotNet tests the programming model of defining a stack as an explicit top-level component.
func TestStackComponentDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_component", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
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
				assert.Equal(t, "ABC", stackRes.Outputs["abc"])
				assert.Equal(t, float64(42), stackRes.Outputs["Foo"])
			}
		},
	})
}

// TestStackComponentServiceProviderDotNet tests the creation of the stack using IServiceProvider.
func TestStackComponentServiceProviderDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_component", "dotnet_service_provider"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
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
				assert.Equal(t, "ABC", stackRes.Outputs["abc"])
				assert.Equal(t, float64(42), stackRes.Outputs["Foo"])
			}
		},
	})
}

// Tests basic configuration from the perspective of a Pulumi .NET program.
func TestConfigBasicDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_basic", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
		Config: map[string]string{
			"aConfigValue": "this value is a value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super secret is encrypted",
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

// Tests that accessing config secrets using non-secret APIs results in warnings being logged.
func TestConfigSecretsWarnDotNet(t *testing.T) {
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	t.Skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_secrets_warn", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
		Config: map[string]string{
			"plainstr1":  "1",
			"plainstr2":  "2",
			"plainstr3":  "3",
			"plainstr4":  "4",
			"plainbool1": "true",
			"plainbool2": "true",
			"plainbool3": "true",
			"plainbool4": "true",
			"plainint1":  "1",
			"plainint2":  "2",
			"plainint3":  "3",
			"plainint4":  "4",
			"plainobj1":  "{}",
			"plainobj2":  "{}",
			"plainobj3":  "{}",
			"plainobj4":  "{}",
		},
		Secrets: map[string]string{
			"str1":  "1",
			"str2":  "2",
			"str3":  "3",
			"str4":  "4",
			"bool1": "true",
			"bool2": "true",
			"bool3": "true",
			"bool4": "true",
			"int1":  "1",
			"int2":  "2",
			"int3":  "3",
			"int4":  "4",
			"obj1":  "{}",
			"obj2":  "{}",
			"obj3":  "{}",
			"obj4":  "{}",
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
				"Configuration 'config_secrets_dotnet:str1' value is a secret; use `GetSecret` instead of `Get`",
				"Configuration 'config_secrets_dotnet:str2' value is a secret; use `RequireSecret` instead of `Require`",
				"Configuration 'config_secrets_dotnet:bool1' value is a secret; use `GetSecretBoolean` instead of `GetBoolean`",
				"Configuration 'config_secrets_dotnet:bool2' value is a secret; use `RequireSecretBoolean` instead of `RequireBoolean`",
				"Configuration 'config_secrets_dotnet:int1' value is a secret; use `GetSecretInt32` instead of `GetInt32`",
				"Configuration 'config_secrets_dotnet:int2' value is a secret; use `RequireSecretInt32` instead of `RequireInt32`",
				"Configuration 'config_secrets_dotnet:obj1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_dotnet:obj2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_dotnet:parent1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_dotnet:parent2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_dotnet:names1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_dotnet:names2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
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

func TestStackReferenceSecretsDotnet(t *testing.T) {
	owner := os.Getenv("PULUMI_TEST_OWNER")
	if owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	d := "stack_reference_secrets"

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		RequireService: true,

		Dir:          filepath.Join(d, "dotnet", "step1"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join(d, "dotnet", "step2"),
				Additive:        true,
				ExpectNoChanges: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					_, isString := stackInfo.Outputs["refNormal"].(string)
					assert.Truef(t, isString, "referenced non-secret output was not a string")

					secretPropValue, ok := stackInfo.Outputs["refSecret"].(map[string]interface{})
					assert.Truef(t, ok, "secret output was not serialized as a secret")
					assert.Equal(t, resource.SecretSig, secretPropValue[resource.SigKey].(string))
				},
			},
		},
	})
}

// Tests a resource with a large (>4mb) string prop in .Net
func TestLargeResourceDotNet(t *testing.T) {
	t.Skip() // TODO[pulumi/pulumi#7832]
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{"Pulumi"},
		Dir:          filepath.Join("large_resource", "dotnet"),
	})
}

// Test remote component construction with a child resource that takes a long time to be created, ensuring it's created.
func TestConstructSlowDotnet(t *testing.T) {
	localProvider := testComponentSlowLocalProvider(t)

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the .NET program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	testDir := "construct_component_slow"
	runComponentSetup(t, testDir)

	opts := &integration.ProgramTestOptions{
		Env:            []string{testYarnLinkPulumiEnv},
		Dir:            filepath.Join(testDir, "dotnet"),
		Dependencies:   []string{"Pulumi"},
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
func TestConstructPlainDotnet(t *testing.T) {
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
			localProviders :=
				[]integration.LocalDependency{
					{Package: "testprovider", Path: buildTestProvider(t, filepath.Join("..", "testprovider"))},
					{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
				}
			integration.ProgramTest(t,
				optsForConstructPlainDotnet(t, test.expectedResourceCount, localProviders, test.env...))
		})
	}
}

func optsForConstructPlainDotnet(t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency,
	env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env:            env,
		Dir:            filepath.Join("construct_component_plain", "dotnet"),
		Dependencies:   []string{"Pulumi"},
		LocalProviders: localProviders,
		Quick:          true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources))
		},
	}
}

// Test remote component inputs properly handle unknowns.
func TestConstructUnknownDotnet(t *testing.T) {
	testConstructUnknown(t, "dotnet", "Pulumi")
}

// Test methods on remote components.
func TestConstructMethodsDotnet(t *testing.T) {
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
				Package: "testcomponent",
				Path:    filepath.Join(testDir, test.componentDir),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, "dotnet"),
				Dependencies:   []string{"Pulumi"},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestConstructMethodsUnknownDotnet(t *testing.T) {
	testConstructMethodsUnknown(t, "dotnet", "Pulumi")
}

func TestConstructMethodsErrorsDotnet(t *testing.T) {
	testConstructMethodsErrors(t, "dotnet", "Pulumi")
}

func TestConstructProviderDotnet(t *testing.T) {
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
				Dir:            filepath.Join(testDir, "dotnet"),
				Dependencies:   []string{"Pulumi"},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "hello world", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestGetResourceDotnet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies:             []string{"Pulumi"},
		Dir:                      filepath.Join("get_resource", "dotnet"),
		AllowEmptyPreviewChanges: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, float64(2), stack.Outputs["getPetLength"])

			out, ok := stack.Outputs["secret"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
		},
	})
}

// Test that the about command works as expected. Because about parses the
// results of each runtime independently, we have an integration test in each
// language.
func TestAboutDotnet(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("about", "dotnet")

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironmentFallible()
		}
	}()
	e.ImportDirectory(dir)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	_, stderr := e.RunCommand("pulumi", "about")
	// This one doesn't have a current stack. Assert that we caught it.
	assert.Contains(t, stderr, "No current stack")
}

func TestProviderDotnet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{"Pulumi"},
		Dir:          filepath.Join("provider", "dotnet"),
	})
}
