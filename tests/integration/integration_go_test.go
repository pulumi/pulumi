// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build go all

package ints

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sourcegraph.com/sourcegraph/appdash"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// TestEmptyGo simply tests that we can build and run an empty Go project.
func TestEmptyGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
	})
}

// TestEmptyGoRun exercises the 'go run' invocation path that doesn't require an explicit build step.
func TestEmptyGoRun(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "gorun"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
	})
}

// TestEmptyGoRunMain exercises the 'go run' invocation path with a 'main' entrypoint specified in Pulumi.yml
func TestEmptyGoRunMain(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "gorun_main"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
	})
}

// Tests basic configuration from the perspective of a Pulumi Go program.
func TestConfigBasicGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_basic", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
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
func TestConfigSecretsWarnGo(t *testing.T) {
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	t.Skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_secrets_warn", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
		Config: map[string]string{
			"plainstr1":    "1",
			"plainstr2":    "2",
			"plainstr3":    "3",
			"plainstr4":    "4",
			"plainstr5":    "5",
			"plainstr6":    "6",
			"plainstr7":    "7",
			"plainstr8":    "8",
			"plainstr9":    "9",
			"plainstr10":   "10",
			"plainstr11":   "11",
			"plainstr12":   "12",
			"plainbool1":   "true",
			"plainbool2":   "true",
			"plainbool3":   "true",
			"plainbool4":   "true",
			"plainbool5":   "true",
			"plainbool6":   "true",
			"plainbool7":   "true",
			"plainbool8":   "true",
			"plainbool9":   "true",
			"plainbool10":  "true",
			"plainbool11":  "true",
			"plainbool12":  "true",
			"plainint1":    "1",
			"plainint2":    "2",
			"plainint3":    "3",
			"plainint4":    "4",
			"plainint5":    "5",
			"plainint6":    "6",
			"plainint7":    "7",
			"plainint8":    "8",
			"plainint9":    "9",
			"plainint10":   "10",
			"plainint11":   "11",
			"plainint12":   "12",
			"plainfloat1":  "1.1",
			"plainfloat2":  "2.2",
			"plainfloat3":  "3.3",
			"plainfloat4":  "4.4",
			"plainfloat5":  "5.5",
			"plainfloat6":  "6.6",
			"plainfloat7":  "7.7",
			"plainfloat8":  "8.8",
			"plainfloat9":  "9.9",
			"plainfloat10": "10.1",
			"plainfloat11": "11.11",
			"plainfloat12": "12.12",
			"plainobj1":    "{}",
			"plainobj2":    "{}",
			"plainobj3":    "{}",
			"plainobj4":    "{}",
			"plainobj5":    "{}",
			"plainobj6":    "{}",
			"plainobj7":    "{}",
			"plainobj8":    "{}",
			"plainobj9":    "{}",
			"plainobj10":   "{}",
			"plainobj11":   "{}",
			"plainobj12":   "{}",
		},
		Secrets: map[string]string{
			"str1":    "1",
			"str2":    "2",
			"str3":    "3",
			"str4":    "4",
			"str5":    "5",
			"str6":    "6",
			"str7":    "7",
			"str8":    "8",
			"str9":    "9",
			"str10":   "10",
			"str11":   "11",
			"str12":   "12",
			"bool1":   "true",
			"bool2":   "true",
			"bool3":   "true",
			"bool4":   "true",
			"bool5":   "true",
			"bool6":   "true",
			"bool7":   "true",
			"bool8":   "true",
			"bool9":   "true",
			"bool10":  "true",
			"bool11":  "true",
			"bool12":  "true",
			"int1":    "1",
			"int2":    "2",
			"int3":    "3",
			"int4":    "4",
			"int5":    "5",
			"int6":    "6",
			"int7":    "7",
			"int8":    "8",
			"int9":    "9",
			"int10":   "10",
			"int11":   "11",
			"int12":   "12",
			"float1":  "1.1",
			"float2":  "2.2",
			"float3":  "3.3",
			"float4":  "4.4",
			"float5":  "5.5",
			"float6":  "6.6",
			"float7":  "7.7",
			"float8":  "8.8",
			"float9":  "9.9",
			"float10": "10.1",
			"float11": "11.11",
			"float12": "12.12",
			"obj1":    "{}",
			"obj2":    "{}",
			"obj3":    "{}",
			"obj4":    "{}",
			"obj5":    "{}",
			"obj6":    "{}",
			"obj7":    "{}",
			"obj8":    "{}",
			"obj9":    "{}",
			"obj10":   "{}",
			"obj11":   "{}",
			"obj12":   "{}",
		},
		OrderedConfig: []integration.ConfigValue{
			{Key: "parent1.foo", Value: "plain1", Path: true},
			{Key: "parent1.bar", Value: "secret1", Path: true, Secret: true},
			{Key: "parent2.foo", Value: "plain2", Path: true},
			{Key: "parent2.bar", Value: "secret2", Path: true, Secret: true},
			{Key: "parent3.foo", Value: "plain2", Path: true},
			{Key: "parent3.bar", Value: "secret2", Path: true, Secret: true},
			{Key: "names1[0]", Value: "plain1", Path: true},
			{Key: "names1[1]", Value: "secret1", Path: true, Secret: true},
			{Key: "names2[0]", Value: "plain2", Path: true},
			{Key: "names2[1]", Value: "secret2", Path: true, Secret: true},
			{Key: "names3[0]", Value: "plain2", Path: true},
			{Key: "names3[1]", Value: "secret2", Path: true, Secret: true},
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotEmpty(t, stackInfo.Events)
			//nolint:lll
			expectedWarnings := []string{
				"Configuration 'config_secrets_go:str1' value is a secret; use `GetSecret` instead of `Get`",
				"Configuration 'config_secrets_go:str2' value is a secret; use `RequireSecret` instead of `Require`",
				"Configuration 'config_secrets_go:str3' value is a secret; use `TrySecret` instead of `Try`",
				"Configuration 'config_secrets_go:str7' value is a secret; use `GetSecret` instead of `Get`",
				"Configuration 'config_secrets_go:str8' value is a secret; use `RequireSecret` instead of `Require`",
				"Configuration 'config_secrets_go:str9' value is a secret; use `TrySecret` instead of `Try`",

				"Configuration 'config_secrets_go:bool1' value is a secret; use `GetSecretBool` instead of `GetBool`",
				"Configuration 'config_secrets_go:bool2' value is a secret; use `RequireSecretBool` instead of `RequireBool`",
				"Configuration 'config_secrets_go:bool3' value is a secret; use `TrySecretBool` instead of `TryBool`",
				"Configuration 'config_secrets_go:bool7' value is a secret; use `GetSecretBool` instead of `GetBool`",
				"Configuration 'config_secrets_go:bool8' value is a secret; use `RequireSecretBool` instead of `RequireBool`",
				"Configuration 'config_secrets_go:bool9' value is a secret; use `TrySecretBool` instead of `TryBool`",

				"Configuration 'config_secrets_go:int1' value is a secret; use `GetSecretInt` instead of `GetInt`",
				"Configuration 'config_secrets_go:int2' value is a secret; use `RequireSecretInt` instead of `RequireInt`",
				"Configuration 'config_secrets_go:int3' value is a secret; use `TrySecretInt` instead of `TryInt`",
				"Configuration 'config_secrets_go:int7' value is a secret; use `GetSecretInt` instead of `GetInt`",
				"Configuration 'config_secrets_go:int8' value is a secret; use `RequireSecretInt` instead of `RequireInt`",
				"Configuration 'config_secrets_go:int9' value is a secret; use `TrySecretInt` instead of `TryInt`",

				"Configuration 'config_secrets_go:float1' value is a secret; use `GetSecretFloat64` instead of `GetFloat64`",
				"Configuration 'config_secrets_go:float2' value is a secret; use `RequireSecretFloat64` instead of `RequireFloat64`",
				"Configuration 'config_secrets_go:float3' value is a secret; use `TrySecretFloat64` instead of `TryFloat64`",
				"Configuration 'config_secrets_go:float7' value is a secret; use `GetSecretFloat64` instead of `GetFloat64`",
				"Configuration 'config_secrets_go:float8' value is a secret; use `RequireSecretFloat64` instead of `RequireFloat64`",
				"Configuration 'config_secrets_go:float9' value is a secret; use `TrySecretFloat64` instead of `TryFloat64`",

				"Configuration 'config_secrets_go:obj1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_go:obj2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_go:obj3' value is a secret; use `TrySecretObject` instead of `TryObject`",
				"Configuration 'config_secrets_go:obj7' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_go:obj8' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_go:obj9' value is a secret; use `TrySecretObject` instead of `TryObject`",

				"Configuration 'config_secrets_go:parent1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_go:parent2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_go:parent3' value is a secret; use `TrySecretObject` instead of `TryObject`",

				"Configuration 'config_secrets_go:names1' value is a secret; use `GetSecretObject` instead of `GetObject`",
				"Configuration 'config_secrets_go:names2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
				"Configuration 'config_secrets_go:names3' value is a secret; use `TrySecretObject` instead of `TryObject`",
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
				"plainstr5",
				"plainstr6",
				"plainstr7",
				"plainstr8",
				"plainstr9",
				"plainstr10",
				"plainstr11",
				"plainstr12",
				"plainbool1",
				"plainbool2",
				"plainbool3",
				"plainbool4",
				"plainbool5",
				"plainbool6",
				"plainbool7",
				"plainbool8",
				"plainbool9",
				"plainbool10",
				"plainbool11",
				"plainbool12",
				"plainint1",
				"plainint2",
				"plainint3",
				"plainint4",
				"plainint5",
				"plainint6",
				"plainint7",
				"plainint8",
				"plainint9",
				"plainint10",
				"plainint11",
				"plainint12",
				"plainfloat1",
				"plainfloat2",
				"plainfloat3",
				"plainfloat4",
				"plainfloat5",
				"plainfloat6",
				"plainfloat7",
				"plainfloat8",
				"plainfloat9",
				"plainfloat10",
				"plainfloat11",
				"plainfloat12",
				"plainobj1",
				"plainobj2",
				"plainobj3",
				"plainobj4",
				"plainobj5",
				"plainobj6",
				"plainobj7",
				"plainobj8",
				"plainobj9",
				"plainobj10",
				"plainobj11",
				"plainobj12",
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

// Tests that stack references work in Go.
func TestStackReferenceGo(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_reference", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
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

// Tests a resource with a large (>4mb) string prop in Go
func TestLargeResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Dir: filepath.Join("large_resource", "go"),
	})
}

// Test remote component construction in Go.
func TestConstructGo(t *testing.T) {
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
			integration.ProgramTest(t, optsForConstructGo(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructGo(t *testing.T, expectedResourceCount int, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
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
func TestConstructSlowGo(t *testing.T) {
	pathEnv := testComponentSlowPathEnv(t)

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the Go program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	opts := &integration.ProgramTestOptions{
		Env: []string{pathEnv, testYarnLinkPulumiEnv},
		Dir: filepath.Join("construct_component_slow", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
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
func TestConstructPlainGo(t *testing.T) {
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
				optsForConstructPlainGo(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructPlainGo(t *testing.T, expectedResourceCount int, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component_plain", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick:      true,
		NoParallel: true, // avoid contention for Dir
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources))
		},
	}
}

// Test remote component inputs properly handle unknowns.
func TestConstructUnknownGo(t *testing.T) {
	testConstructUnknown(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsGo(t *testing.T) {
	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t, filepath.Join("construct_component_methods", test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env: []string{pathEnv},
				Dir: filepath.Join("construct_component_methods", "go"),
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
				},
				Quick:      true,
				NoParallel: true, // avoid contention for Dir
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestGetResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Dir:                      filepath.Join("get_resource", "go"),
		AllowEmptyPreviewChanges: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, float64(2), stack.Outputs["getPetLength"])
		},
	})
}

func TestComponentProviderSchemaGo(t *testing.T) {
	path := filepath.Join("component_provider_schema", "testcomponent-go", "pulumi-resource-testcomponent")
	if runtime.GOOS == WindowsOS {
		path += ".exe"
	}
	testComponentProviderSchema(t, path)
}

// TestTracePropagationGo checks that --tracing flag lets golang sub-process to emit traces.
func TestTracePropagationGo(t *testing.T) {

	// Detect a special trace coming from Go language plugin.
	isGoListTrace := func(t *appdash.Trace) bool {
		m := t.Span.Annotations.StringMap()

		isGoCmd := strings.HasSuffix(m["command"], "go") ||
			strings.HasSuffix(m["command"], "go.exe")

		if m["component"] == "exec.Command" &&
			m["args"] == "[list -m -json -mod=mod all]" &&
			isGoCmd {
			return true
		}

		return false
	}

	var foundTrace *appdash.Trace

	// Look for trace mathching `isGoListTrace` in the trace file
	// and store to `foundTrace`.
	searchForGoListTrace := func(dir string) error {

		store, err := ReadMemoryStoreFromFile(filepath.Join(dir, "pulumi.trace"))
		if err != nil {
			return err
		}

		foundTrace, err = FindTrace(store, isGoListTrace)
		if err != nil {
			return err
		}

		return nil
	}

	opts := &integration.ProgramTestOptions{
		Dir:                    filepath.Join("empty", "go"),
		Dependencies:           []string{"github.com/pulumi/pulumi/sdk/v3"},
		SkipRefresh:            true,
		SkipPreview:            false,
		SkipUpdate:             true,
		SkipExportImport:       true,
		SkipEmptyPreviewUpdate: true,
		Quick:                  false,
		Tracing:                fmt.Sprintf("file:./pulumi.trace"),
		PreviewCompletedHook:   searchForGoListTrace,
	}

	integration.ProgramTest(t, opts)

	if foundTrace == nil {
		t.Errorf("Did not find a trace for `go list -m -json -mod=mod all` command")
	}
}
