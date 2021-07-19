// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build dotnet all

package ints

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// TestEmptyDotNet simply tests that we can run an empty .NET project.
func TestEmptyDotNet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("empty", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
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

// Tests that stack references work in .NET.
func TestStackReferenceDotnet(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	opts := &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_reference", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
		Config: map[string]string{
			"org": os.Getenv("PULUMI_TEST_OWNER"),
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step1",
				Additive: true,
			},
			{
				Dir:      "step2",
				Additive: true,
			},
		},
	}
	integration.ProgramTest(t, opts)
}

func TestStackReferenceSecretsDotnet(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	owner := os.Getenv("PULUMI_TEST_OWNER")
	if owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	d := "stack_reference_secrets"

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(d, "dotnet", "step1"),
		Dependencies: []string{"Pulumi"},
		Config: map[string]string{
			"org": owner,
		},
		Quick: true,
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
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{"Pulumi"},
		Dir:          filepath.Join("large_resource", "dotnet"),
	})
}

// Test remote component construction in .NET.
func TestConstructDotnet(t *testing.T) {
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
			env:                   []string{pulumiRuntimeVirtualEnv(t, filepath.Join("..", ".."))},
		},
		{
			componentDir:          "testcomponent-go",
			expectedResourceCount: 8, // One less because no dynamic provider.
		},
	}

	for _, test := range tests {
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t, filepath.Join("construct_component", test.componentDir))
			integration.ProgramTest(t,
				optsForConstructDotnet(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructDotnet(t *testing.T, expectedResourceCount int, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env:          env,
		Dir:          filepath.Join("construct_component", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Secrets: map[string]string{
			"secret": "this super secret is encrypted",
		},
		Quick:      true,
		NoParallel: true, // avoid contention for Dir
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))

				// Check that dependencies flow correctly between the originating program and the remote component
				// plugin.
				urns := make(map[string]resource.URN)
				for _, res := range stackInfo.Deployment.Resources[1:] {
					assert.NotNil(t, res)

					urns[string(res.URN.Name())] = res.URN
					switch res.URN.Name() {
					case "child-a":
						for _, deps := range res.PropertyDependencies {
							assert.Empty(t, deps)
						}
					case "child-b":
						expected := []resource.URN{urns["a"]}
						assert.ElementsMatch(t, expected, res.Dependencies)
						assert.ElementsMatch(t, expected, res.PropertyDependencies["echo"])
					case "child-c":
						expected := []resource.URN{urns["a"], urns["child-a"]}
						assert.ElementsMatch(t, expected, res.Dependencies)
						assert.ElementsMatch(t, expected, res.PropertyDependencies["echo"])
					case "a", "b", "c":
						secretPropValue, ok := res.Outputs["secret"].(map[string]interface{})
						assert.Truef(t, ok, "secret output was not serialized as a secret")
						assert.Equal(t, resource.SecretSig, secretPropValue[resource.SigKey].(string))
					}
				}
			}
		},
	}
}

// Test remote component construction with a child resource that takes a long time to be created, ensuring it's created.
func TestConstructSlowDotnet(t *testing.T) {
	pathEnv := testComponentSlowPathEnv(t)

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the .NET program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	opts := &integration.ProgramTestOptions{
		Env:          []string{pathEnv, testYarnLinkPulumiEnv},
		Dir:          filepath.Join("construct_component_slow", "dotnet"),
		Dependencies: []string{"Pulumi"},
		Quick:        true,
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
			env:                   []string{pulumiRuntimeVirtualEnv(t, filepath.Join("..", ".."))},
		},
		{
			componentDir:          "testcomponent-go",
			expectedResourceCount: 8, // One less because no dynamic provider.
		},
	}

	for _, test := range tests {
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t, filepath.Join("construct_component_plain", test.componentDir))
			integration.ProgramTest(t,
				optsForConstructPlainDotnet(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructPlainDotnet(t *testing.T, expectedResourceCount int,
	env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env:          env,
		Dir:          filepath.Join("construct_component_plain", "dotnet"),
		Dependencies: []string{"Pulumi"},
		NoParallel:   true, // avoid contention for Dir
		Quick:        true,
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

func TestGetResourceDotnet(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies:             []string{"Pulumi"},
		Dir:                      filepath.Join("get_resource", "dotnet"),
		AllowEmptyPreviewChanges: true,
	})
}
