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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	dap "github.com/google/go-dap"
	"github.com/grapl-security/pulumi-hcp/sdk/go/hcp"
	"github.com/pulumi/appdash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// This checks that the buildTarget option for Pulumi Go programs does build a binary.
func TestBuildTarget(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(filepath.Join("go", "go-build-target"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "go-build-target-test-stack")
	e.RunCommand("pulumi", "stack", "select", "go-build-target-test-stack")
	e.RunCommand("pulumi", "preview")
	_, err := os.Stat(filepath.Join(e.RootPath, "a.out"))
	assert.NoError(t, err)
}

// This checks that the Exit Status artifact from Go Run is not being produced
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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

//nolint:paralleltest // ProgramTest calls t.Parallel()
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
	t.Parallel()

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
		NoParallel:    true,
		ExtraRuntimeValidation: func(t *testing.T, _ integration.RuntimeValidationStackInfo) {
			const needle = "panic: great sadness\n"
			haystack := stderr.String()
			// one instances of needle in the returned error:
			assert.Equal(t, 1, strings.Count(haystack, needle),
				"Expected only one instance of %q in:\n%s", needle, haystack)
		},
	})
}

// This checks that error logs are not being emitted twice
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
	t.Parallel()

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
		NoParallel:     true,
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

	//nolint:paralleltest // ProgramTest calls t.Parallel()
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

func optsForConstructPlainGo(
	t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency, env ...string,
) *integration.ProgramTestOptions {
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
	t.Parallel()
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

	//nolint:paralleltest // ProgramTest calls t.Parallel()
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
	t.Parallel()
	testConstructMethodsUnknown(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsResourcesGo(t *testing.T) {
	t.Parallel()
	testConstructMethodsResources(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsErrorsGo(t *testing.T) {
	t.Parallel()
	testConstructMethodsErrors(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestConstructMethodsProviderGo(t *testing.T) {
	t.Parallel()
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

	//nolint:paralleltest // ProgramTest calls t.Parallel()
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
	// This uses the random plugin so needs to be able to download it
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		NoParallel: true,
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
	t.Parallel()
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
	t.Parallel()

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
		Tracing:                "file:" + filepath.Join(dir, "{command}.trace"),
		RequireService:         true,
		NoParallel:             true,
	}

	integration.ProgramTest(t, opts)

	store, err := ReadMemoryStoreFromFile(filepath.Join(dir, "pulumi-update-initial.trace"))
	assert.NoError(t, err)
	assert.NotNil(t, store)

	t.Run("traced `go list -m -json`", func(t *testing.T) {
		t.Parallel()

		isGoListTrace := func(t *appdash.Trace) bool {
			m := t.StringMap()

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
		t.Parallel()

		exportStackCounter := 0
		err := WalkTracesWithDescendants(store, func(tr *appdash.Trace) error {
			name := tr.Name()
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
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(dir)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "about-stack")
	e.RunCommand("pulumi", "stack", "select", "about-stack")
	stdout, _ := e.RunCommand("pulumi", "about", "-t")

	// Assert we parsed the dependencies
	assert.Contains(t, stdout, "github.com/pulumi/pulumi/sdk/v3")
	// Assert we parsed the language plugin, we don't assert against the minor version number
	assert.Regexp(t, regexp.MustCompile(`language\W+go\W+3\.`), stdout)
}

func TestConstructOutputValuesGo(t *testing.T) {
	t.Parallel()
	testConstructOutputValues(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

// TestProjectMainGo tests out the ability to override the main entrypoint.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
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
	t.Parallel()

	// Context scoped to the lifetime of the test.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
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

// TestStackOutputsProgramErrorGo tests that when a program error occurs, we update any
// updated stack outputs, but otherwise leave others untouched.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsProgramErrorGo(t *testing.T) {
	d := filepath.Join("stack_outputs_program_error", "go")

	validateOutputs := func(
		expected map[string]interface{},
	) func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
		return func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Equal(t, expected, stackInfo.RootResource.Outputs)
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join(d, "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
		ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
			"xyz": "ABC",
			"foo": float64(42),
		}),
		EditDirs: []integration.EditDir{
			{
				Dir:           filepath.Join(d, "step2"),
				Additive:      true,
				ExpectFailure: true,
				ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
					"xyz": "DEF",       // Expected to be updated
					"foo": float64(42), // Expected to remain the same
				}),
			},
		},
	})
}

// TestStackOutputsResourceErrorGo tests that when a resource error occurs, we update any
// updated stack outputs, but otherwise leave others untouched.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsResourceErrorGo(t *testing.T) {
	d := filepath.Join("stack_outputs_resource_error", "go")

	validateOutputs := func(
		expected map[string]interface{},
	) func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
		return func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Equal(t, expected, stackInfo.RootResource.Outputs)
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join(d, "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
			"xyz": "ABC",
			"foo": float64(42),
		}),
		EditDirs: []integration.EditDir{
			{
				Dir:           filepath.Join(d, "step2"),
				Additive:      true,
				ExpectFailure: true,
				// Expect the values to remain the same because the deployment ends before RegisterResourceOutputs is
				// called for the stack.
				ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
					"xyz": "ABC",
					"foo": float64(42),
				}),
			},
			{
				Dir:           filepath.Join(d, "step3"),
				Additive:      true,
				ExpectFailure: true,
				// Expect the values to be updated.
				ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
					"xyz": "DEF",
					"foo": float64(1),
				}),
			},
		},
	})
}

// Test a paramaterized provider with go.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestParameterizedGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "parameterized"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		PrePrepareProject: func(info *engine.Projinfo) error {
			e := ptesting.NewEnvironment(t)
			e.CWD = info.Root

			// We have a bare-bones main.go program checked-in that does _not_ depend on the generated SDK.
			// This allows the `make tidy` script and other tools like dependabot to run successfully in this
			// directory. When running the test, overwrite the bare-bones main.go with the actual test
			// program that makes use of the generated SDK.
			actualProgram, err := os.ReadFile(filepath.Join(e.CWD, "actual_program.txt"))
			require.NoError(t, err)
			e.WriteTestFile("main.go", string(actualProgram))

			actualProgram, err = os.ReadFile(filepath.Join(e.CWD, "actual_program_test.txt"))
			require.NoError(t, err)
			e.WriteTestFile("main_test.go", string(actualProgram))

			// Generate the SDK for the provider.
			path := info.Proj.Plugins.Providers[0].Path
			_, _ = e.RunCommand("pulumi", "package", "gen-sdk", path, "pkg", "--language", "go")

			// Add a reference to the generated SDK in go.mod.
			err = appendLines(filepath.Join(e.CWD, "go.mod"), []string{
				"require example.com/pulumi-pkg/sdk/go v1.0.0",
				"replace example.com/pulumi-pkg/sdk/go => ./sdk/go",
			})
			require.NoError(t, err)

			return nil
		},
		PostPrepareProject: func(info *engine.Projinfo) error {
			e := ptesting.NewEnvironment(t)
			e.CWD = info.Root

			e.RunCommand("go", "test", "-v", "./...")

			return nil
		},
	})
}

func appendLines(name string, lines []string) error {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

//nolint:paralleltest // mutates environment
func TestPackageAddGo(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	var err error
	templatePath, err := filepath.Abs("go/packageadd")
	require.NoError(t, err)
	err = fsutil.CopyFile(e.CWD, templatePath, nil)
	require.NoError(t, err)

	_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "random")
	randomVersion := getPluginVersion(e, "random")
	assert.NotEmpty(t, randomVersion)
	_, _ = e.RunCommand("pulumi", "package", "add", "random")

	modBytes, err := os.ReadFile(filepath.Join(e.CWD, "go.mod"))
	assert.NoError(t, err)
	_, err = modfile.Parse("go.mod", modBytes, nil)
	assert.NoError(t, err)

	// Verify that the Pulumi.yaml file contains the random package with correct settings
	yamlContent, err := os.ReadFile(filepath.Join(e.CWD, "Pulumi.yaml"))
	require.NoError(t, err)
	yamlString := string(yamlContent)
	require.Contains(t, yamlString, "packages:")
	require.Contains(t, yamlString, "random: random@"+randomVersion)

	// Currently package add does not work correctly for non parameterized
	// packages, once they add the go.mod as expected we can parse it and check
	// if it contains a rename as the parameterized version of this test does.
}

// getPluginVersion finds the highest version of a plugin by name
func getPluginVersion(e *ptesting.Environment, pluginName string) string {
	stdout, _ := e.RunCommand("pulumi", "plugin", "ls", "--json")

	type Plugin struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Version string `json:"version"`
	}

	var plugins []Plugin
	err := json.Unmarshal([]byte(stdout), &plugins)
	if err != nil {
		return ""
	}

	for _, plugin := range plugins {
		if plugin.Name == pluginName && plugin.Kind == "resource" {
			// Even if multiple versions are installed, the entries are ordered descending.
			return plugin.Version
		}
	}

	return ""
}

//nolint:paralleltest // mutates environment
func TestPackageAddGoParameterized(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	var err error
	templatePath, err := filepath.Abs("go/packageadd")
	require.NoError(t, err)
	err = fsutil.CopyFile(e.CWD, templatePath, nil)
	require.NoError(t, err)

	// Install terraform-provider and note its version
	_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "terraform-provider")
	terraformProviderVersion := getPluginVersion(e, "terraform-provider")
	assert.NotEmpty(t, terraformProviderVersion)
	_, _ = e.RunCommand("pulumi", "package", "add", "terraform-provider", "NetApp/netapp-cloudmanager", "25.1.0")

	// Verify that the Pulumi.yaml file contains the netapp-cloudmanage package with correct settings
	yamlContent, err := os.ReadFile(filepath.Join(e.CWD, "Pulumi.yaml"))
	require.NoError(t, err)
	yamlString := string(yamlContent)
	require.Contains(t, yamlString, "packages:")
	require.Contains(t, yamlString, "netapp-cloudmanager:")
	require.Contains(t, yamlString, "source: terraform-provider")
	require.Contains(t, yamlString, "version: "+terraformProviderVersion)
	require.Contains(t, yamlString, "parameters:")
	require.Contains(t, yamlString, "- NetApp/netapp-cloudmanager")
	require.Contains(t, yamlString, "- 25.1.0")

	assert.True(t, e.PathExists("sdks/netapp-cloudmanager/go.mod"))
	packageModBytes, err := os.ReadFile(filepath.Join(e.CWD, "sdks/netapp-cloudmanager/go.mod"))
	assert.NoError(t, err)
	packageMod, err := modfile.Parse("package.mod", packageModBytes, nil)
	assert.NoError(t, err)
	assert.Equal(t, "github.com/pulumi/pulumi-terraform-provider/sdks/go/netapp-cloudmanager/v25",
		packageMod.Module.Mod.Path)

	modBytes, err := os.ReadFile(filepath.Join(e.CWD, "go.mod"))
	assert.NoError(t, err)
	gomod, err := modfile.Parse("go.mod", modBytes, nil)
	assert.NoError(t, err)

	containsRename := false
	containedRenames := make([]string, len(gomod.Replace))
	for _, r := range gomod.Replace {
		if r.New.Path == "./sdks/netapp-cloudmanager" &&
			r.Old.Path == "github.com/pulumi/pulumi-terraform-provider/sdks/go/netapp-cloudmanager/v25" {
			containsRename = true
		}
		containedRenames = append(containedRenames, r.Old.Path+" => "+r.New.Path)
	}

	assert.True(t, containsRename,
		fmt.Sprintf("expected go.mod to contain a replace for the package.  Contains: %v", containedRenames))
}

//nolint:paralleltest // mutates environment
func TestConvertTerraformProviderGo(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	var err error
	templatePath, err := filepath.Abs("convertfromterraform")
	require.NoError(t, err)
	err = fsutil.CopyFile(e.CWD, templatePath, nil)
	require.NoError(t, err)

	_, _ = e.RunCommand("pulumi", "plugin", "install", "converter", "terraform")
	_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "terraform-provider", "0.8.0")
	_, _ = e.RunCommand("pulumi", "convert", "--from", "terraform", "--language", "go", "--out", "godir")

	assert.True(t, e.PathExists("godir/go.mod"))
	assert.True(t, e.PathExists("godir/sdks/supabase/go.mod"))

	modBytes, err := os.ReadFile(filepath.Join(e.CWD, "godir", "go.mod"))
	assert.NoError(t, err)
	gomod, err := modfile.Parse("go.mod", modBytes, nil)
	assert.NoError(t, err)

	containsRename := false
	for _, r := range gomod.Replace {
		if r.New.Path == "./sdks/supabase" && r.Old.Path ==
			"github.com/pulumi/pulumi-terraform-provider/sdks/go/supabase" {
			containsRename = true
		}
	}

	assert.True(t, containsRename)
}

//nolint:paralleltest // mutates environment
func TestConvertMultipleTerraformProviderGo(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	var err error
	templatePath, err := filepath.Abs("convertmultiplefromterraform")
	require.NoError(t, err)
	err = fsutil.CopyFile(e.CWD, templatePath, nil)
	require.NoError(t, err)

	_, _ = e.RunCommand("pulumi", "plugin", "install", "converter", "terraform")
	_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "terraform-provider")
	_, _ = e.RunCommand("pulumi", "convert", "--from", "terraform", "--language", "go", "--out", "godir")

	assert.True(t, e.PathExists("godir/go.mod"))
	assert.True(t, e.PathExists("godir/sdks/supabase/go.mod"))
	assert.True(t, e.PathExists("godir/sdks/b2/go.mod"))

	modBytes, err := os.ReadFile(filepath.Join(e.CWD, "godir", "go.mod"))
	assert.NoError(t, err)
	gomod, err := modfile.Parse("go.mod", modBytes, nil)
	assert.NoError(t, err)

	containsRenameSupabase := false
	containsRenameBB := false
	for _, r := range gomod.Replace {
		if r.New.Path == "./sdks/supabase" && r.Old.Path ==
			"github.com/pulumi/pulumi-terraform-provider/sdks/go/supabase" {
			containsRenameSupabase = true
		}
		if r.New.Path == "./sdks/b2" && r.Old.Path ==
			"github.com/pulumi/pulumi-terraform-provider/sdks/go/b2" {
			containsRenameBB = true
		}
	}

	assert.True(t, containsRenameSupabase)
	assert.True(t, containsRenameBB)
}

func readUpdateEventLog(logfile string) ([]apitype.EngineEvent, error) {
	events := make([]apitype.EngineEvent, 0)
	eventsFile, err := os.Open(logfile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("expected to be able to open event log file %s: %w",
			logfile, err)
	}

	defer contract.IgnoreClose(eventsFile)

	decoder := json.NewDecoder(eventsFile)
	for {
		var event apitype.EngineEvent
		if err = decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed decoding engine event from log file %s: %w",
				logfile, err)
		}
		events = append(events, event)
	}
	return events, nil
}

func newDAPRequest(seq int, command string) dap.Request {
	request := dap.Request{}
	request.Type = "request"
	request.Command = command
	request.Seq = seq
	return request
}

func TestDebuggerAttach(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(filepath.Join("go", "go-build-target"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.Env = append(e.Env, "PULUMI_DEBUG_COMMANDS=true")
		e.RunCommand("pulumi", "stack", "init", "debugger-test")
		e.RunCommand("pulumi", "stack", "select", "debugger-test")
		e.RunCommand("pulumi", "preview", "--attach-debugger",
			"--event-log", filepath.Join(e.RootPath, "debugger.log"))
	}()

	// Wait for the debugging event
	wait := 20 * time.Millisecond
	var debugEvent *apitype.StartDebuggingEvent
outer:
	for i := 0; i < 50; i++ {
		events, err := readUpdateEventLog(filepath.Join(e.RootPath, "debugger.log"))
		require.NoError(t, err)
		for _, event := range events {
			if event.StartDebuggingEvent != nil {
				debugEvent = event.StartDebuggingEvent
				break outer
			}
		}
		time.Sleep(wait)
		wait *= 2
	}
	require.NotNil(t, debugEvent)

	// We've attached a debugger, so we need to connect to it and let the program continue.
	conn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(int(debugEvent.Config["port"].(float64))))
	require.NoError(t, err)
	defer conn.Close()

	seq := 0
	err = dap.WriteProtocolMessage(conn, &dap.InitializeRequest{
		Request: newDAPRequest(seq, "initialize"),
		Arguments: dap.InitializeRequestArguments{
			ClientID:        "pulumi",
			ClientName:      "Pulumi",
			AdapterID:       "pulumi",
			Locale:          "en-us",
			LinesStartAt1:   true,
			ColumnsStartAt1: true,
		},
	})
	assert.NoError(t, err)
	seq++
	reader := bufio.NewReader(conn)
	// We need to read the response, but we don't actually care
	// about it.  It just includes the capabilities of the
	// debugger.
	resp, err := dap.ReadProtocolMessage(reader)
	assert.NoError(t, err)
	assert.IsType(t, &dap.InitializeResponse{}, resp)
	json, err := json.Marshal(debugEvent.Config)
	assert.NoError(t, err)
	err = dap.WriteProtocolMessage(conn, &dap.AttachRequest{
		Request:   newDAPRequest(seq, "attach"),
		Arguments: json,
	})
	assert.NoError(t, err)
	seq++
	// read the initialized event, and then the response to the attach request.
	resp, err = dap.ReadProtocolMessage(reader)
	assert.NoError(t, err)
	assert.IsType(t, &dap.InitializedEvent{}, resp)
	resp, err = dap.ReadProtocolMessage(reader)
	assert.NoError(t, err)
	assert.IsType(t, &dap.AttachResponse{}, resp)

	err = dap.WriteProtocolMessage(conn, &dap.ContinueRequest{
		Request: newDAPRequest(seq, "continue"),
	})
	assert.NoError(t, err)
	seq++
	resp, err = dap.ReadProtocolMessage(reader)
	assert.NoError(t, err)
	assert.IsType(t, &dap.ContinueResponse{}, resp)
	resp, err = dap.ReadProtocolMessage(reader)
	assert.NoError(t, err)
	assert.IsType(t, &dap.TerminatedEvent{}, resp)

	err = dap.WriteProtocolMessage(conn, &dap.DisconnectRequest{
		Request: newDAPRequest(seq, "disconnect"),
	})
	assert.NoError(t, err)

	// Make sure the program finished successfully.
	wg.Wait()
}

func TestPluginDebuggerAttach(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.ImportDirectory(filepath.Join("debug-plugin"))
	e.CWD = filepath.Join(e.CWD, "program")

	e.RunCommand("pulumi", "package", "add", "../go-plugin")

	wg := sync.WaitGroup{}
	wg.Add(1)
	eventLogPath := filepath.Join(e.RootPath, "plugin_debugger.log")
	go func() {
		defer wg.Done()
		e.RunCommand("pulumi", "stack", "init", "plugin-debugger-test")
		e.RunCommand("pulumi", "stack", "select", "plugin-debugger-test")
		// We're disconnecting the debugger from the plugin, and it exits immediately.
		// Therefore we expect a EOF error.
		stdout, _ := e.RunCommandExpectError("pulumi", "preview", "--attach-debugger=plugins",
			"--event-log", eventLogPath)
		require.Regexp(t, "error: could not read plugin \\[.*/go-plugin/pulumi-resource-debugplugin\\]: EOF", stdout)
	}()

	wait := 20 * time.Millisecond
	var debugEvent *apitype.StartDebuggingEvent
outer:
	for i := 0; i < 50; i++ {
		events, err := readUpdateEventLog(eventLogPath)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		for _, event := range events {
			if event.StartDebuggingEvent != nil {
				debugEvent = event.StartDebuggingEvent
				break outer
			}
		}
		time.Sleep(wait)
		wait *= 2
	}
	require.NotNil(t, debugEvent, "did not receive start debugging event for plugin")

	conn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(int(debugEvent.Config["port"].(float64))))
	require.NoError(t, err)
	defer conn.Close()

	seq := 0
	err = dap.WriteProtocolMessage(conn, &dap.InitializeRequest{
		Request: newDAPRequest(seq, "initialize"),
		Arguments: dap.InitializeRequestArguments{
			ClientID:        "pulumi-test-plugin",
			ClientName:      "Pulumi Test Plugin",
			AdapterID:       "pulumi",
			Locale:          "en-us",
			LinesStartAt1:   true,
			ColumnsStartAt1: true,
		},
	})
	require.NoError(t, err)
	seq++
	reader := bufio.NewReader(conn)
	// We need to read the response, but we don't actually care
	// about it.  It just includes the capabilities of the
	// debugger.
	resp, err := dap.ReadProtocolMessage(reader)
	require.NoError(t, err)
	require.IsType(t, &dap.InitializeResponse{}, resp)
	json, err := json.Marshal(debugEvent.Config)
	require.NoError(t, err)
	err = dap.WriteProtocolMessage(conn, &dap.AttachRequest{
		Request:   newDAPRequest(seq, "attach"),
		Arguments: json,
	})
	require.NoError(t, err)
	seq++
	// read the initialized event, and then the response to the attach request.
	resp, err = dap.ReadProtocolMessage(reader)
	require.NoError(t, err)
	require.IsType(t, &dap.InitializedEvent{}, resp)
	resp, err = dap.ReadProtocolMessage(reader)
	require.NoError(t, err)
	require.IsType(t, &dap.AttachResponse{}, resp)

	err = dap.WriteProtocolMessage(conn, &dap.ContinueRequest{
		Request: newDAPRequest(seq, "continue"),
	})
	require.NoError(t, err)
	seq++
	resp, err = dap.ReadProtocolMessage(reader)
	require.NoError(t, err)
	require.IsType(t, &dap.ContinueResponse{}, resp)
	err = dap.WriteProtocolMessage(conn, &dap.DisconnectRequest{
		Request: newDAPRequest(seq, "disconnect"),
	})
	assert.NoError(t, err)

	// Wait for the pulumi preview command to finish.
	wg.Wait()
}

func TestConstructFailuresGo(t *testing.T) {
	t.Parallel()
	testConstructFailures(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

func TestCallFailuresGo(t *testing.T) {
	t.Parallel()
	testCallFailures(t, "go", "github.com/pulumi/pulumi/sdk/v3")
}

// TestLogDebugGo tests that the amount of debug logs is reasonable.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestLogDebugGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("log_debug", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			var count int
			for _, ev := range stack.Events {
				if de := ev.DiagnosticEvent; de != nil && de.Severity == "debug" {
					count++
				}
			}
			t.Logf("Found %v debug log events", count)

			// Ensure at least 1 debug log events are emitted, confirming debug logs are working as expected.
			assert.Greaterf(t, count, 0, "%v is not enough debug log events", count)

			// More than 25 debug log events on such a simple program is very likely unintended.
			assert.LessOrEqual(t, count, 25, "%v is too many debug log events", count)
		},
	})
}

// Test that we can successfully start up a shimless provider and it gets the right environment
// variables set.
func TestRunPlugin(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(filepath.Join("run_plugin"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.CWD = filepath.Join(e.RootPath, "provider-nodejs")
	e.RunCommand("pulumi", "install")

	e.CWD = filepath.Join(e.RootPath, "provider-python")
	e.RunCommand("pulumi", "install")

	e.CWD = filepath.Join(e.RootPath, "go")
	sdkPath, err := filepath.Abs("../../sdk/")
	require.NoError(t, err)

	e.RunCommand("go", "mod", "edit", "-replace=github.com/pulumi/pulumi/sdk/v3="+sdkPath)
	e.RunCommand("go", "mod", "tidy")
	e.RunCommand("pulumi", "stack", "init", "runplugin-test")
	e.RunCommand("pulumi", "stack", "select", "runplugin-test")
	e.RunCommand("pulumi", "preview")
}

// This checks that we provide a useful error message if the user tries to run a
// program without a main package.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestErrorNoMainPackage(t *testing.T) {
	stderr := &bytes.Buffer{}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "go-no-main-package"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		Stderr:        stderr,
		ExpectFailure: true,
		Quick:         true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Contains(t, stderr.String(), "does your program have a 'main' package?")
		},
	})
}
