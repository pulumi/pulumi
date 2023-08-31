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

//go:build (go || all) && !xplatform_acceptance

package ints

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/grapl-security/pulumi-hcp/sdk/go/hcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sourcegraph.com/sourcegraph/appdash"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// This checks that the buildTarget option for Pulumi Go programs does build a binary.
func TestBuildTarget(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironmentFallible()
		}
	}()
	e.ImportDirectory(filepath.Join("go", "go-build-target"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "go-build-target-test-stack")
	e.RunCommand("pulumi", "stack", "select", "go-build-target-test-stack")
	e.RunCommand("pulumi", "preview")
	_, err := os.Stat(filepath.Join(e.RootPath, "a.out"))
	assert.NoError(t, err)
}

// This checks that the Exit Status artifact from Go Run is not being produced
func TestNoEmitExitStatus(t *testing.T) {
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "go-exit-5"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Stderr:        stderr,
		ExpectFailure: true,
		Quick:         true,
		SkipRefresh:   true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// ensure exit status is not emitted by the program
			assert.NotContains(t, stderr.String(), "exit status")
		},
	})
}

func TestPanickingProgram(t *testing.T) {
	var stderr bytes.Buffer
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "program-panic"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Stderr:        &stderr,
		ExpectFailure: true,
		Quick:         true,
		SkipRefresh:   true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Contains(t, stderr.String(), "panic: great sadness\n")
		},
	})
}

func TestPanickingComponentConfigure(t *testing.T) {
	var (
		testDir      = filepath.Join("go", "component-configure-panic")
		componentDir = "testcomponent-go"
	)
	runComponentSetup(t, testDir)

	var stderr bytes.Buffer
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "component-configure-panic", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{
				Package: "testcomponent",
				Path:    filepath.Join(testDir, componentDir),
			},
		},
		Stderr:        &stderr,
		ExpectFailure: true,
		Quick:         true,
		SkipRefresh:   true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Contains(t, stderr.String(), "panic: great sadness\n")
		},
	})
}

// This checks that error logs are not being emitted twice
func TestNoLogError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "go-exit-error"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Stdout:        stdout,
		Stderr:        stderr,
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			errorCount := strings.Count(stderr.String()+stdout.String(), "  error: ")

			// ensure `  error: ` is only being shown once by the program
			assert.Equal(t, 1, errorCount)
		},
	})
}

// This checks that the PULUMI_GO_USE_RUN=true flag is triggering go run by checking the `exit status`
// string is being emitted. This is a temporary fallback measure in case it breaks users and should
// not be assumed to be stable.
func TestGoRunEnvFlag(t *testing.T) {
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env: []string{"PULUMI_GO_USE_RUN=true"},
		Dir: filepath.Join("go", "go-exit-5"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Stderr:        stderr,
		ExpectFailure: true,
		Quick:         true,
		SkipRefresh:   true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// ensure exit status IS emitted by the program as it indicates `go run` was used
			assert.Contains(t, stderr.String(), "exit status")
		},
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

// TestPrintfGo tests that we capture stdout and stderr streams properly, even when the last line lacks an \n.
func TestPrintfGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("printf", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick:                  true,
		ExtraRuntimeValidation: printfTestValidation,
	})
}

// Tests basic configuration from the perspective of a Pulumi Go program.
func TestConfigBasicGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_basic", "go"),
		Dependencies: []string{"github.com/pulumi/pulumi/sdk/v3"},
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

// Tests configuration error from the perspective of a Pulumi Go program.
func TestConfigMissingGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_missing", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotEmpty(t, stackInfo.Events)
			text1 := "Missing required configuration variable 'config_missing_go:notFound'"
			text2 := "\tplease set a value using the command `pulumi config set --secret config_missing_go:notFound <value>`"
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

// Tests a resource with a large (>4mb) string prop in Go
func TestLargeResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Dir: filepath.Join("large_resource", "go"),
	})
}

// Test remote component construction with a child resource that takes a long time to be created, ensuring it's created.
func TestConstructSlowGo(t *testing.T) {
	localProvider := testComponentSlowLocalProvider(t)

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the Go program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	testDir := "construct_component_slow"
	runComponentSetup(t, testDir)

	opts := &integration.ProgramTestOptions{
		Env: []string{testYarnLinkPulumiEnv},
		Dir: filepath.Join(testDir, "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
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
func TestConstructPlainGo(t *testing.T) {
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
				optsForConstructPlainGo(t, test.expectedResourceCount, localProviders, test.env...))
		})
	}
}

func optsForConstructPlainGo(t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component_plain", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
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
func TestConstructUnknownGo(t *testing.T) {
	testConstructUnknown(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsGo(t *testing.T) {
	t.Parallel()

	testDir := "construct_component_methods"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
		env          []string
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
				Env: test.env,
				Dir: filepath.Join(testDir, "go"),
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
				},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])

					// TODO[pulumi/pulumi#12471]: Only the Go SDK has been fixed such that rehydrated
					// components are kept as dependencies. So only check this for the provider written
					// in Go. Once the other SDKs are fixed, we can test the other providers as well.
					if test.componentDir == "testcomponent-go" {
						var componentURN string
						for _, res := range stackInfo.Deployment.Resources {
							if res.URN.Name() == "component" {
								componentURN = string(res.URN)
							}
						}
						assert.Contains(t, stackInfo.Outputs["messagedeps"], componentURN)
					}
				},
			})
		})
	}
}

func TestConstructMethodsUnknownGo(t *testing.T) {
	testConstructMethodsUnknown(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsResourcesGo(t *testing.T) {
	testConstructMethodsResources(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsErrorsGo(t *testing.T) {
	testConstructMethodsErrors(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsProviderGo(t *testing.T) {
	testConstructMethodsProvider(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructProviderGo(t *testing.T) {
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
				Dir: filepath.Join(testDir, "go"),
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
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

func TestGetResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Dir:                      filepath.Join("get_resource", "go"),
		AllowEmptyPreviewChanges: true,
		Secrets: map[string]string{
			"bar": "this super secret is encrypted",
		},
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

func TestComponentProviderSchemaGo(t *testing.T) {
	// TODO[https://github.com/pulumi/pulumi/issues/12365] We no longer build the go-component in
	// component_setup.sh so there's no native binary for the testComponentProviderSchema to just exec. It
	// _ought_ to be rewritten to use the plugin host framework so that it starts the component up the same as
	// all the other tests are doing (via shimless).
	t.Skip("testComponentProviderSchema needs to be updated to use a plugin host to deal with non-native-binary providers")

	path := filepath.Join("component_provider_schema", "testcomponent-go", "pulumi-resource-testcomponent")
	if runtime.GOOS == WindowsOS {
		path += ".exe"
	}
	testComponentProviderSchema(t, path)
}

// TestTracePropagationGo checks that --tracing flag lets golang sub-process to emit traces.
func TestTracePropagationGo(t *testing.T) {
	dir := t.TempDir()

	opts := &integration.ProgramTestOptions{
		Dir:                    filepath.Join("empty", "go"),
		Dependencies:           []string{"github.com/pulumi/pulumi/sdk/v3"},
		SkipRefresh:            true,
		SkipPreview:            true,
		SkipUpdate:             false,
		SkipExportImport:       true,
		SkipEmptyPreviewUpdate: true,
		Quick:                  false,
		Tracing:                fmt.Sprintf("file:%s", filepath.Join(dir, "{command}.trace")),
		RequireService:         true,
	}

	integration.ProgramTest(t, opts)

	store, err := ReadMemoryStoreFromFile(filepath.Join(dir, "pulumi-update-initial.trace"))
	assert.NoError(t, err)
	assert.NotNil(t, store)

	t.Run("traced `go list -m -json`", func(t *testing.T) {
		isGoListTrace := func(t *appdash.Trace) bool {
			m := t.Span.Annotations.StringMap()

			isGoCmd := strings.HasSuffix(m["command"], "go") ||
				strings.HasSuffix(m["command"], "go.exe")

			return isGoCmd &&
				m["component"] == "exec.Command" &&
				strings.Contains(m["args"], "list -m -json")
		}
		tr, err := FindTrace(store, isGoListTrace)
		assert.NoError(t, err)
		assert.NotNil(t, tr)
	})

	t.Run("traced api/exportStack exactly once", func(t *testing.T) {
		exportStackCounter := 0
		err := WalkTracesWithDescendants(store, func(tr *appdash.Trace) error {
			name := tr.Span.Name()
			if name == "api/exportStack" {
				exportStackCounter++
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, exportStackCounter)
	})
}

// Test that the about command works as expected. Because about parses the
// results of each runtime independently, we have an integration test in each
// language.
func TestAboutGo(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("about", "go")

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironmentFallible()
		}
	}()
	e.ImportDirectory(dir)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "about-stack")
	e.RunCommand("pulumi", "stack", "select", "about-stack")
	stdout, _ := e.RunCommand("pulumi", "about", "-t")

	// Assert we parsed the dependencies
	assert.Contains(t, stdout, "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructOutputValuesGo(t *testing.T) {
	testConstructOutputValues(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

// TestProjectMainGo tests out the ability to override the main entrypoint.
func TestProjectMainGo(t *testing.T) {
	test := integration.ProgramTestOptions{
		Dir:          "project_main/go",
		Dependencies: []string{"github.com/pulumi/pulumi/sdk/v3"},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.NotNil(t, stackInfo.Deployment)
		},
	}
	integration.ProgramTest(t, &test)
}

// TestRefreshGo simply tests that we can build and run an empty Go project with the `refresh` option set.
func TestRefreshGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("refresh", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
	})
}

// TestResourceRefsGetResourceGo tests that invoking the built-in 'pulumi:pulumi:getResource' function
// returns resource references for any resource reference in a resource's state.
func TestResourceRefsGetResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("resource_refs_get_resource", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
	})
}

// TestDeletedWithGo tests the DeletedWith resource option.
func TestDeletedWithGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("deleted_with", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		Quick: true,
	})
}

func TestConstructProviderPropagationGo(t *testing.T) {
	t.Parallel()

	testConstructProviderPropagation(t, "go", []string{"github.com/pulumi/pulumi/sdk/v3"})
}

func TestConstructResourceOptionsGo(t *testing.T) {
	t.Parallel()

	testConstructResourceOptions(t, "go", []string{"github.com/pulumi/pulumi/sdk/v3"})
}

// Regression test for https://github.com/pulumi/pulumi/issues/13301.
// The reproduction is a bit involved:
//
//   - Set up a fake Pulumi Go project that imports a specific non-Pulumi plugin.
//     Specifically, the plugin MUST NOT be imported by any Go file in the project.
//   - Install that plugin with 'pulumi plugin install'.
//   - Run a Go Automation program that uses that plugin.
//
// The issue in #13301 was that this plugin would not be downloaded by `pulumi plugin install`,
// causing a failure when the Automation program tried to use it.
func TestAutomation_externalPluginDownload_issue13301(t *testing.T) {
	// Context scoped to the lifetime of the test.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironmentFallible()
		}
	}()
	e.ImportDirectory(filepath.Join("go", "regress-13301"))

	// Rename go.mod.bad to go.mod so that the Go toolchain uses it.
	require.NoError(t, os.Rename(
		filepath.Join(e.CWD, "go.mod.bad"),
		filepath.Join(e.CWD, "go.mod"),
	))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	// Plugins are installed globally in PULUMI_HOME.
	// We will set that to a temporary directory,
	// so this is not polluted by other tests.
	pulumiHome := filepath.Join(e.RootPath, ".pulumi-home")
	require.NoError(t, os.MkdirAll(pulumiHome, 0o700))

	// The commands that follow will make gRPC requests that
	// we don't have a lot of visibility into.
	// If we run the Pulumi CLI with PULUMI_DEBUG_GRPC set to a path,
	// it will log the gRPC requests and responses to that file.
	//
	// Capture these and print them if the test fails.
	grpcLog := filepath.Join(e.RootPath, "debug-grpc.log")
	defer func() {
		if !t.Failed() {
			return
		}

		if bs, err := os.ReadFile(grpcLog); err == nil {
			t.Logf("grpc debug log:\n%s", bs)
		}
	}()

	e.Env = append(e.Env,
		"PULUMI_HOME="+pulumiHome,
		"PULUMI_DEBUG_GRPC="+grpcLog)
	e.RunCommand("pulumi", "plugin", "install")

	ws, err := auto.NewLocalWorkspace(ctx,
		auto.Project(workspace.Project{
			Name:    "issue-13301",
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		}),
		auto.WorkDir(e.CWD),
		auto.PulumiHome(pulumiHome),
		auto.EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "not-a-real-passphrase",
			"PULUMI_DEBUG_COMMANDS":    "true",
			"PULUMI_CREDENTIALS_PATH":  e.RootPath,
			"PULUMI_DEBUG_GRPC":        grpcLog,
		}),
	)
	require.NoError(t, err)

	ws.SetProgram(func(ctx *pulumi.Context) error {
		provider, err := hcp.NewProvider(ctx, "hcp", &hcp.ProviderArgs{})
		if err != nil {
			return err
		}

		_ = provider // unused
		return nil
	})

	stack, err := auto.UpsertStack(ctx, "foo", ws)
	require.NoError(t, err)

	_, err = stack.Preview(ctx)
	require.NoError(t, err)
}

func TestConstructProviderExplicitGo(t *testing.T) {
	t.Parallel()

	testConstructProviderExplicit(t, "go", []string{"github.com/pulumi/pulumi/sdk/v3"})
}
