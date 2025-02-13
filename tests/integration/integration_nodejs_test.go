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

//go:build (nodejs || all) && !xplatform_acceptance

package ints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrintfNodeJS tests that we capture stdout and stderr streams properly, even when the last line lacks an \n.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPrintfNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    filepath.Join("printf", "nodejs"),
		Dependencies:           []string{"@pulumi/pulumi"},
		Quick:                  true,
		ExtraRuntimeValidation: printfTestValidation,
	})
}

// Tests emitting many engine events doesn't result in a performance problem.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEngineEventPerf(t *testing.T) {
	t.Skip() // TODO[pulumi/pulumi#7883]

	// Prior to pulumi/pulumi#2303, a preview or update would take ~40s.
	// Since then, it should now be down to ~4s, with additional padding,
	// since some Travis machines (especially the macOS ones) seem quite slow
	// to begin with.
	benchmarkEnforcer := &integration.AssertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 8 * time.Second,
		MaxUpdateDuration:  8 * time.Second,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "ee_perf",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ReportStats:  benchmarkEnforcer,
	})
}

// TestEngineEvents ensures that the test framework properly records and reads engine events.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEngineEvents(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "single_resource",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure that we have a non-empty list of events.
			assert.NotEmpty(t, stackInfo.Events)

			// Ensure that we have three "ResourcePre" events: one for the stack and one for our resource
			// and one for the provider
			preEventResourceTypes := []string{}
			for _, e := range stackInfo.Events {
				if e.ResourcePreEvent != nil {
					fmt.Println(e.ResourcePreEvent.Metadata.Type)
					preEventResourceTypes = append(preEventResourceTypes, e.ResourcePreEvent.Metadata.Type)
				}
			}

			assert.Equal(t, 3, len(preEventResourceTypes))
			assert.Contains(t, preEventResourceTypes, "pulumi:pulumi:Stack")
			assert.Contains(t, preEventResourceTypes, "pulumi-nodejs:dynamic:Resource")
			assert.Contains(t, preEventResourceTypes, "pulumi:providers:pulumi-nodejs")
		},
	})
}

// TestProjectMainNodejs tests out the ability to override the main entrypoint.
func TestProjectMainNodejs(t *testing.T) {
	test := integration.ProgramTestOptions{
		Dir:          "project_main/nodejs",
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.NotNil(t, stackInfo.Deployment)
		},
	}
	integration.ProgramTest(t, &test)

	t.Run("AbsolutePath", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.ImportDirectory("project_main_abs")

		// write a new Pulumi.yaml using the absolute path of the environment as "main"
		yamlPath := filepath.Join(e.RootPath, "Pulumi.yaml")
		absYamlContents := fmt.Sprintf(
			"name: project_main_abs\ndescription: A program with an absolute entry point\nruntime: nodejs\nmain: %s\n",
			e.RootPath,
		)
		t.Logf("writing new Pulumi.yaml: \npath: %s\ncontents:%s", yamlPath, absYamlContents)
		if err := os.WriteFile(yamlPath, []byte(absYamlContents), 0o600); err != nil {
			t.Error(err)
			return
		}

		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-abs")
		e.RunCommand("pulumi", "preview")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	t.Run("ParentFolder", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.ImportDirectory("project_main_parent")

		// yarn link first
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		// then virtually change directory to the location of the nested Pulumi.yaml
		e.CWD = filepath.Join(e.RootPath, "foo", "bar")

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-parent")
		e.RunCommand("pulumi", "preview")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})
}

// TestStackProjectName ensures we can read the Pulumi stack and project name from within the program.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackProjectName(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		RequireService: true,

		Dir:          "stack_project_name",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

func TestRemoveWithResourcesBlocked(t *testing.T) {
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	stackName, err := resource.NewUniqueHex("rm-test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	e.ImportDirectory("single_resource")
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
	_, stderr := e.RunCommandExpectError("pulumi", "stack", "rm", "--yes")
	assert.Contains(t, stderr, "--force")
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--non-interactive", "--yes")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

// TestStackOutputs ensures we can export variables from a stack and have them get recorded as outputs.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_outputs", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
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

// TestStackOutputsProgramErrorNodeJS tests that when a program error occurs, we update any
// updated stack outputs, but otherwise leave others untouched.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsProgramErrorNodeJS(t *testing.T) {
	d := filepath.Join("stack_outputs_program_error", "nodejs")

	validateOutputs := func(
		expected map[string]interface{},
	) func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
		return func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Equal(t, expected, stackInfo.RootResource.Outputs)
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(d, "step1"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
			"xyz": "ABC",
			"foo": float64(42),
		}),
		EditDirs: []integration.EditDir{
			{
				Dir:           filepath.Join(d, "step2"),
				Additive:      true,
				ExpectFailure: true,
				// A program error in TypeScript means we won't get any new stack outputs from the module exports,
				// so we expect the values to remain the same.
				ExtraRuntimeValidation: validateOutputs(map[string]interface{}{
					"xyz": "ABC",
					"foo": float64(42),
				}),
			},
		},
	})
}

// TestStackOutputsResourceErrorNodeJS ensures that prior stack outputs aren't overwritten when a
// resource operation error occurs during an update.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsResourceErrorNodeJS(t *testing.T) {
	d := filepath.Join("stack_outputs_resource_error", "nodejs")

	validateOutputs := func(
		expected map[string]interface{},
	) func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
		return func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Equal(t, expected, stackInfo.RootResource.Outputs)
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(d, "step1"),
		Dependencies: []string{"@pulumi/pulumi"},
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

// TestStackOutputsJSON ensures the CLI properly formats stack outputs as JSON when requested.
func TestStackOutputsJSON(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(filepath.Join("stack_outputs", "nodejs"))
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "stack-outs")
	e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
	stdout, _ := e.RunCommand("pulumi", "stack", "output", "--json")
	assert.Equal(t, `{
  "foo": 42,
  "xyz": "ABC"
}
`, stdout)
}

// TestStackOutputsDisplayed ensures that outputs are printed at the end of an update
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsDisplayed(t *testing.T) {
	stdout := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_outputs", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        false,
		Verbose:      true,
		Stdout:       stdout,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stdout.String()

			// ensure we get the outputs info both for the normal update, and for the no-change update.
			assert.Contains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n\nResources:\n    + 1 created")
			assert.Contains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n\nResources:\n    1 unchanged")
		},
	})
}

// TestStackOutputsSuppressed ensures that outputs whose values are intentionally suppresses don't show.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackOutputsSuppressed(t *testing.T) {
	stdout := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    filepath.Join("stack_outputs", "nodejs"),
		Dependencies:           []string{"@pulumi/pulumi"},
		Quick:                  false,
		Verbose:                true,
		Stdout:                 stdout,
		UpdateCommandlineFlags: []string{"--suppress-outputs"},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stdout.String()
			assert.NotContains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n")
			assert.NotContains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n")
		},
	})
}

// TestStackParenting tests out that stacks and components are parented correctly.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackParenting(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_parenting",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure the checkpoint contains resources parented correctly.  This should look like this:
			//
			//     A      F
			//    / \      \
			//   B   C      G
			//      / \
			//     D   E
			//
			// with the caveat, of course, that A and F will share a common parent, the implicit stack.

			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 9, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))

				urns := make(map[string]resource.URN)
				for _, res := range stackInfo.Deployment.Resources[1:] {
					assert.NotNil(t, res)

					urns[res.URN.Name()] = res.URN
					switch res.URN.Name() {
					case "a", "f":
						assert.NotEqual(t, "", res.Parent)
						assert.Equal(t, stackRes.URN, res.Parent)
					case "b", "c":
						assert.Equal(t, urns["a"], res.Parent)
					case "d", "e":
						assert.Equal(t, urns["c"], res.Parent)
					case "g":
						assert.Equal(t, urns["f"], res.Parent)
					case "default":
						// Default providers should have the stack as a parent, but auto-parenting has been
						// disabled so they won't have a parent for now.
						assert.Equal(t, resource.URN(""), res.Parent)
					default:
						t.Fatalf("unexpected name %s", res.URN.Name())
					}
				}
			}
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackBadParenting(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           "stack_bad_parenting",
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		ExpectFailure: true,
	})
}

// TestStackDependencyGraph tests that the dependency graph of a stack is saved
// in the checkpoint file.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackDependencyGraph(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_dependencies",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			latest := stackInfo.Deployment
			assert.True(t, len(latest.Resources) >= 2)
			sawFirst := false
			sawSecond := false
			for _, res := range latest.Resources {
				urn := string(res.URN)
				if strings.Contains(urn, "dynamic:Resource::first") {
					// The first resource doesn't depend on anything.
					assert.Equal(t, 0, len(res.Dependencies))
					sawFirst = true
				} else if strings.Contains(urn, "dynamic:Resource::second") {
					// The second resource uses an Output property of the first resource, so it
					// depends directly on first.
					assert.Equal(t, 1, len(res.Dependencies))
					assert.True(t, strings.Contains(string(res.Dependencies[0]), "dynamic:Resource::first"))
					sawSecond = true
				}
			}

			assert.True(t, sawFirst && sawSecond)
		},
	})
}

// Tests basic configuration from the perspective of a Pulumi program.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestConfigBasicNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_basic", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
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

// Tests configuration error from the perspective of a Pulumi program.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestConfigMissingJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("config_missing", "nodejs"),
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotEmpty(t, stackInfo.Events)
			text1 := "Missing required configuration variable 'config_missing_js:notFound'"
			text2 := "\tplease set a value using the command `pulumi config set --secret config_missing_js:notFound <value>`"
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

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestConfigCaptureNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_capture_e2e", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config: map[string]string{
			"value": "it works",
		},
	})
}

// Tests that accessing config secrets using non-secret APIs results in warnings being logged.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestConfigSecretsWarnNodeJS(t *testing.T) {
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	t.Skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_secrets_warn", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
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
			"plainnum1":  "1",
			"plainnum2":  "2",
			"plainnum3":  "3",
			"plainnum4":  "4",
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
			"num1":  "1",
			"num2":  "2",
			"num3":  "3",
			"num4":  "4",
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
				"Configuration 'config_secrets_node:str1' value is a secret; use `getSecret` instead of `get`",
				"Configuration 'config_secrets_node:str2' value is a secret; use `requireSecret` instead of `require`",
				"Configuration 'config_secrets_node:bool1' value is a secret; use `getSecretBoolean` instead of `getBoolean`",
				"Configuration 'config_secrets_node:bool2' value is a secret; use `requireSecretBoolean` instead of `requireBoolean`",
				"Configuration 'config_secrets_node:num1' value is a secret; use `getSecretNumber` instead of `getNumber`",
				"Configuration 'config_secrets_node:num2' value is a secret; use `requireSecretNumber` instead of `requireNumber`",
				"Configuration 'config_secrets_node:obj1' value is a secret; use `getSecretObject` instead of `getObject`",
				"Configuration 'config_secrets_node:obj2' value is a secret; use `requireSecretObject` instead of `requireObject`",
				"Configuration 'config_secrets_node:parent1' value is a secret; use `getSecretObject` instead of `getObject`",
				"Configuration 'config_secrets_node:parent2' value is a secret; use `requireSecretObject` instead of `requireObject`",
				"Configuration 'config_secrets_node:names1' value is a secret; use `getSecretObject` instead of `getObject`",
				"Configuration 'config_secrets_node:names2' value is a secret; use `requireSecretObject` instead of `requireObject`",
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
				"plainnum1",
				"plainnum2",
				"plainnum3",
				"plainnum4",
				"plainobj1",
				"plainobj2",
				"plainobj3",
				"plainobj4",
				"str3",
				"str4",
				"bool3",
				"bool4",
				"num3",
				"num4",
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

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestInvalidVersionInPackageJson(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("invalid_package_json"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config:       map[string]string{},
	})
}

// Tests an explicit provider instance.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestExplicitProvider(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "explicit_provider",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			latest := stackInfo.Deployment

			// Expect one stack resource, two provider resources, and two custom resources.
			assert.True(t, len(latest.Resources) == 5)

			var defaultProvider *apitype.ResourceV3
			var explicitProvider *apitype.ResourceV3
			for _, res := range latest.Resources {
				urn := res.URN
				switch urn.Name() {
				case "default":
					assert.True(t, providers.IsProviderType(res.Type))
					assert.Nil(t, defaultProvider)
					prov := res
					defaultProvider = &prov

				case "p":
					assert.True(t, providers.IsProviderType(res.Type))
					assert.Nil(t, explicitProvider)
					prov := res
					explicitProvider = &prov

				case "a":
					prov, err := providers.ParseReference(res.Provider)
					assert.NoError(t, err)
					assert.NotNil(t, defaultProvider)
					defaultRef, err := providers.NewReference(defaultProvider.URN, defaultProvider.ID)
					assert.NoError(t, err)
					assert.Equal(t, defaultRef.String(), prov.String())

				case "b":
					prov, err := providers.ParseReference(res.Provider)
					assert.NoError(t, err)
					assert.NotNil(t, explicitProvider)
					explicitRef, err := providers.NewReference(explicitProvider.URN, explicitProvider.ID)
					assert.NoError(t, err)
					assert.Equal(t, explicitRef.String(), prov.String())
				}
			}

			assert.NotNil(t, defaultProvider)
			assert.NotNil(t, explicitProvider)
		},
	})
}

// Tests that reads of unknown IDs do not fail.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestGetCreated(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "get_created",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestProviderSecretConfig that a first class provider can be created when it has secrets as part of its config.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestProviderSecretConfig(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "provider_secret_config",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestResourceWithSecretSerializationNodejs(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("secret_outputs", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// The program exports three resources:
			//   1. One named `withSecret` who's prefix property should be secret, specified via `pulumi.secret()`.
			//   2. One named `withSecretAdditional` who's prefix property should be a secret, specified via
			//      additionalSecretOutputs.
			//   3. One named `withoutSecret` which should not be a secret.
			// We serialize both of the these as POJO objects, so they appear as maps in the output.
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

//nolint:paralleltest // mutates environment variables
func TestPasswordlessPassphraseSecretsProvider(t *testing.T) {
	testOptions := integration.ProgramTestOptions{
		Dir:             "cloud_secrets_provider",
		Dependencies:    []string{"@pulumi/pulumi"},
		SecretsProvider: "passphrase",
		Env:             []string{"PULUMI_CONFIG_PASSPHRASE=\"\""},
		Secrets: map[string]string{
			"mysecret": "THISISASECRET",
		},
		CloudURL:   integration.MakeTempBackend(t),
		NoParallel: true, // mutates environment variables
	}

	workingTestOptions := testOptions.With(integration.ProgramTestOptions{
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "passphrase")

			_, err := passphrase.NewPromptingPassphraseSecretsManagerFromState(secretsProvider.State)
			assert.NoError(t, err)

			out, ok := stackInfo.Outputs["out"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
		},
	})

	brokenTestOptions := testOptions.With(integration.ProgramTestOptions{
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "passphrase")

			_, err := passphrase.NewPromptingPassphraseSecretsManagerFromState(secretsProvider.State)
			assert.Error(t, err)
		},
	})

	t.Run("works-when-passphrase-set", func(t *testing.T) {
		integration.ProgramTest(t, &workingTestOptions)
	})

	t.Run("error-when-passphrase-not-set", func(t *testing.T) {
		integration.ProgramTest(t, &brokenTestOptions)
	})
}

func TestCloudSecretProvider(t *testing.T) {
	t.Parallel()

	awsKmsKeyAlias := os.Getenv("PULUMI_TEST_KMS_KEY_ALIAS")
	if awsKmsKeyAlias == "" {
		t.Skipf("Skipping: PULUMI_TEST_KMS_KEY_ALIAS is not set")
	}

	azureKeyVault := os.Getenv("PULUMI_TEST_AZURE_KEY")
	if azureKeyVault == "" {
		t.Skipf("Skipping: PULUMI_TEST_AZURE_KEY is not set")
	}

	gcpKmsKey := os.Getenv("PULUMI_TEST_GCP_KEY")
	if gcpKmsKey == "" {
		t.Skipf("Skipping: PULUMI_TEST_GCP_KEY is not set")
	}

	// Generic test options for all providers
	testOptions := integration.ProgramTestOptions{
		Dir:             "cloud_secrets_provider",
		Dependencies:    []string{"@pulumi/pulumi"},
		SecretsProvider: "awskms://alias/" + awsKmsKeyAlias,
		Secrets: map[string]string{
			"mysecret": "THISISASECRET",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "cloud")

			_, err := cloud.NewCloudSecretsManagerFromState(secretsProvider.State)
			assert.NoError(t, err)

			out, ok := stackInfo.Outputs["out"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
		},
	}

	localTestOptions := testOptions.With(integration.ProgramTestOptions{
		CloudURL: integration.MakeTempBackend(t),
	})

	azureTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: "azurekeyvault://" + azureKeyVault,
	})

	gcpTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: "gcpkms://projects/" + gcpKmsKey,
	})

	// Run with default Pulumi service backend
	//
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("service", func(t *testing.T) { integration.ProgramTest(t, &testOptions) })

	// Check Azure secrets provider
	//
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("azure", func(t *testing.T) { integration.ProgramTest(t, &azureTestOptions) })

	// Check gcloud secrets provider
	//
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("gcp", func(t *testing.T) { integration.ProgramTest(t, &gcpTestOptions) })

	// Also run with local diy backend
	//
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("local", func(t *testing.T) { integration.ProgramTest(t, &localTestOptions) })
}

// Tests a resource with a large (>4mb) string prop in Node.js
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestLargeResourceNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("large_resource", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
	})
}

// Tests enum outputs
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEnumOutputNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("enums", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, "Burgundy", stack.Outputs["myTreeType"])
			assert.Equal(t, "Pulumi Planters Inc.foo", stack.Outputs["myTreeFarmChanged"])
			assert.Equal(t, "My Burgundy Rubber tree is from Pulumi Planters Inc.", stack.Outputs["mySentence"])
		},
	})
}

// Test remote component construction with a child resource that takes a long time to be created, ensuring it's created.
func TestConstructSlowNode(t *testing.T) {
	t.Parallel()

	localProvider := testComponentSlowLocalProvider(t)

	var opts *integration.ProgramTestOptions

	testDir := "construct_component_slow"
	runComponentSetup(t, testDir)

	opts = &integration.ProgramTestOptions{
		Dir:            filepath.Join(testDir, "nodejs"),
		Dependencies:   []string{"@pulumi/pulumi"},
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
func TestConstructPlainNode(t *testing.T) {
	t.Parallel()

	testDir := "construct_component_plain"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir          string
		expectedResourceCount int
	}{
		{
			componentDir:          "testcomponent",
			expectedResourceCount: 9,
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
				optsForConstructPlainNode(t, test.expectedResourceCount, localProviders))
		})
	}
}

func optsForConstructPlainNode(
	t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency,
) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Dir:            filepath.Join("construct_component_plain", "nodejs"),
		Dependencies:   []string{"@pulumi/pulumi"},
		LocalProviders: localProviders,
		Quick:          true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources))
		},
	}
}

// Test remote component inputs properly handle unknowns.
func TestConstructUnknownNode(t *testing.T) {
	t.Parallel()
	testConstructUnknown(t, "nodejs", "@pulumi/pulumi")
}

// Test methods on remote components.
func TestConstructMethodsNode(t *testing.T) {
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

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProvider := integration.LocalDependency{
				Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, "nodejs"),
				Dependencies:   []string{"@pulumi/pulumi"},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestConstructMethodsUnknownNode(t *testing.T) {
	t.Parallel()
	testConstructMethodsUnknown(t, "nodejs", "@pulumi/pulumi")
}

func TestConstructMethodsResourcesNode(t *testing.T) {
	t.Parallel()
	testConstructMethodsResources(t, "nodejs", "@pulumi/pulumi")
}

func TestConstructMethodsErrorsNode(t *testing.T) {
	t.Parallel()
	testConstructMethodsErrors(t, "nodejs", "@pulumi/pulumi")
}

func TestConstructMethodsProviderNode(t *testing.T) {
	t.Parallel()
	testConstructMethodsProvider(t, "nodejs", "@pulumi/pulumi")
}

func TestConstructProviderNode(t *testing.T) {
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
				Dir:            filepath.Join(testDir, "nodejs"),
				Dependencies:   []string{"@pulumi/pulumi"},
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "hello world", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestGetResourceNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                      filepath.Join("get_resource", "nodejs"),
		Dependencies:             []string{"@pulumi/pulumi"},
		AllowEmptyPreviewChanges: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, "foo", stack.Outputs["foo"])

			out, ok := stack.Outputs["secret"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
		},
	})
}

func TestComponentProviderSchemaNode(t *testing.T) {
	t.Parallel()
	path := filepath.Join("component_provider_schema", "testcomponent", "pulumi-resource-testcomponent")
	if runtime.GOOS == WindowsOS {
		path += ".cmd"
	}
	testComponentProviderSchema(t, path)
}

// Test throwing an error within an apply in a remote component written in nodejs.
// The provider should return the error and shutdown gracefully rather than hanging.
func TestConstructNodeErrorApply(t *testing.T) {
	t.Parallel()

	dir := "construct_component_error_apply"
	componentDir := "testcomponent"

	runComponentSetup(t, dir)

	stderr := &bytes.Buffer{}
	expectedError := "intentional error from within an apply"

	opts := &integration.ProgramTestOptions{
		Dir:          filepath.Join(dir, "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testcomponent", Path: filepath.Join(dir, componentDir)},
		},
		Quick:         true,
		Stderr:        stderr,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stderr.String()
			assert.Contains(t, output, expectedError)
		},
	}

	t.Run(componentDir, func(t *testing.T) {
		integration.ProgramTest(t, opts)
	})
}

// Test to ensure that internal stacks are hidden
func TestNodejsStackTruncate(t *testing.T) {
	t.Parallel()

	cases := []string{
		"syntax-error",
		"ts-error",
	}

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, name := range cases {
		// Test the program.
		t.Run(name, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:          filepath.Join("nodejs", "omit-stacktrace", name),
				Dependencies: []string{"@pulumi/pulumi"},
				Quick:        true,
				// This test should fail because it raises an exception
				ExpectFailure: true,
				// We need to validate that the failure has a truncated stack trace
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Ensure that we have a non-empty list of events.
					assert.NotEmpty(t, stackInfo.Events)

					const stacktraceLinePrefix = "    at "

					// get last DiagnosticEvent containing python stack trace
					stackTraceMessage := ""
					for _, e := range stackInfo.Events {
						if e.DiagnosticEvent == nil {
							continue
						}
						msg := e.DiagnosticEvent.Message
						if !strings.Contains(msg, stacktraceLinePrefix) {
							continue
						}
						stackTraceMessage = msg
					}
					assert.Equal(t, "", stackTraceMessage)
				},
			})
		})
	}
}

// Test targeting `es2016` in `tsconfig.json` works.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCompilerOptionsNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "compiler_options"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-js"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMJSMain(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-js-main"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMTS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-ts"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestTSWithPackageJsonInParentDir(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "ts-with-package-json-in-parent-dir"),
		RelativeWorkDir: filepath.Join("myprogram"),
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMWithPackageJsonInParentDir(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "esm-with-package-json-in-parent-dir"),
		RelativeWorkDir: filepath.Join("myprogram"),
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMWithoutPackageJsonInParentDir(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "esm-package-json-in-parent-dir-without-main"),
		RelativeWorkDir: filepath.Join("myprogram"),
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPackageJsonInParentDirWithoutMain(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "package-json-in-parent-dir-without-main"),
		RelativeWorkDir: filepath.Join("myprogram"),
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMTSNestedSrc(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-ts-nested-src"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config: map[string]string{
			"test": "hello world",
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Len(t, stack.Outputs, 1)
			test, ok := stack.Outputs["test"]
			assert.True(t, ok)
			assert.Equal(t, "hello world", test)
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMTSDefaultExport(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-ts-default-export"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Len(t, stack.Outputs, 1)
			helloWorld, ok := stack.Outputs["helloWorld"]
			assert.True(t, ok)
			assert.Equal(t, helloWorld, 123.0)
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMTSSpecifierResolutionNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-ts-specifier-resolution-node"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestESMTSCompiled(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "esm-ts-compiled"),
		Dependencies: []string{"@pulumi/pulumi"},
		RunBuild:     true,
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestMainOverridesPackageJSON(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "main-overrides-package-json"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, "This is the entrypoint from Pulumi.yaml", stack.Outputs["text"])
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNpmWorkspace(t *testing.T) {
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works with npm
		// workspaces.
		return nil
	}
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "npm-and-yarn-workspaces"),
		Quick:           true,
		RelativeWorkDir: "infra",
		PrepareProject:  preparePropject,
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")
	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

//nolint:paralleltest // mutates environment variables
func TestYarnWorkspace(t *testing.T) {
	t.Setenv("PULUMI_PREFER_YARN", "true")
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works with yarn
		// workspaces.
		return nil
	}
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "npm-and-yarn-workspaces"),
		Quick:           true,
		RelativeWorkDir: "infra",
		PrepareProject:  preparePropject,
		NoParallel:      true, // mutates environment variables
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")
	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

//nolint:paralleltest // mutates environment variables
func TestYarnWorkspaceNoHoist(t *testing.T) {
	t.Setenv("PULUMI_PREFER_YARN", "true")
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works with yarn
		// workspaces.
		return nil
	}
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "yarn-workspaces-nohoist"),
		Quick:           true,
		RelativeWorkDir: "infra",
		PrepareProject:  preparePropject,
		NoParallel:      true, // mutates environment variables
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")
	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNestedPackageJSON(t *testing.T) {
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works with npm
		// workspaces.
		return nil
	}
	dir := filepath.Join("nodejs", "npm-and-yarn-not-a-workspace")
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:             dir,
		Quick:           true,
		RelativeWorkDir: "infra",
		PrepareProject:  preparePropject,
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")
	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")

	// There is no node_modules directory in the test directory (parent of program dir)
	_, err := os.Stat(filepath.Join(pt.GetTmpDir(), "node_modules"))
	require.True(t, os.IsNotExist(err))

	// node_modules was created inside the program directory
	_, err = os.Stat(filepath.Join(pt.GetTmpDir(), "infra", "node_modules"))
	require.NoError(t, err)

	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPnpmWorkspace(t *testing.T) {
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works with pnpm
		// workspaces.
		return nil
	}
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "pnpm-workspace"),
		Quick:           true,
		RelativeWorkDir: "infra",
		PrepareProject:  preparePropject,
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")

	_, err := os.Stat(filepath.Join(pt.GetTmpDir(), "node_modules", ".pnpm"))
	require.NoError(t, err)

	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestInstallWithMain(t *testing.T) {
	preparePropject := func(projinfo *engine.Projinfo) error {
		// The default nodejs prepare uses yarn to link dependencies.
		// For this test we don't want to test the current SDK, instead we
		// want to test `pulumi install` and ensure that it works for projects
		// with a `main` property.
		return nil
	}
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:            filepath.Join("nodejs", "pulumi-main"),
		Quick:          true,
		PrepareProject: preparePropject,
	})

	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})

	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.RunPulumiCommand("install"), "install")

	_, err := os.Stat(filepath.Join(pt.GetTmpDir(), "node_modules"))
	require.NoError(t, err)

	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	require.NoError(t, pt.TestPreviewUpdateAndEdits(), "update")
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}

func TestTranspileOnly(t *testing.T) {
	t.Parallel()

	for _, test := range []string{"tsconfig-no-check", "swc"} {
		test := test
		t.Run(test, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join("nodejs", test)
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			e.ImportDirectory(dir)

			stackName := ptesting.RandomStackName()

			// For this test we need to properly install the core SDK instead of yarn
			// linkining, because yarn link breaks the typescript version detection, and
			// causes us to use the vendored typescript 3.8.3, which does not support
			// the `noCheck` option.
			coreSDK, err := filepath.Abs(filepath.Join("..", "..", "sdk", "nodejs", "bin"))
			require.NoError(t, err)
			e.RunCommand("yarn", "install")
			e.RunCommand("yarn", "add", coreSDK)
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", stackName)
			e.RunCommand("pulumi", "stack", "select", stackName)
			e.RunCommand("pulumi", "up", "--yes")
			_, _ = e.RunCommand("pulumi", "destroy", "--skip-preview", "--refresh=true")
		})
	}
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePaths(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "codepaths"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePathsTSC(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "codepaths-tsc"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		RunBuild:     true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePathsNested(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "codepaths-nested"),
		Dependencies:    []string{"@pulumi/pulumi"},
		RelativeWorkDir: "nested",
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePathsWorkspace(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "codepaths-workspaces"),
		Dependencies:    []string{"@pulumi/pulumi"},
		RelativeWorkDir: "infra",
		Quick:           true,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePathsWorkspaceTSC(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             filepath.Join("nodejs", "codepaths-workspaces-tsc"),
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
		RunBuild:        true,
		RelativeWorkDir: "infra",
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCodePathsNoDependencies(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "codepaths-no-dependencies"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// Test that the resource stopwatch doesn't contain a negative time.
func TestNoNegativeTimingsOnRefresh(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Skip on windows because we lack yarn")
	}
	t.Parallel()

	dir := filepath.Join("empty", "nodejs")
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(dir)

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "negative-timings")
	e.RunCommand("pulumi", "stack", "select", "negative-timings")
	e.RunCommand("pulumi", "up", "--yes")
	stdout, _ := e.RunCommand("pulumi", "destroy", "--skip-preview", "--refresh=true")
	// Assert there are no negative times in the output.
	assert.NotContainsf(t, stdout, " (-",
		"`pulumi destroy --skip-preview --refresh=true` contains a negative time")
}

// Test that the about command works as expected. Because about parses the
// results of each runtime independently, we have an integration test in each
// language.
func TestAboutNodeJS(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Skip on windows because we lack yarn")
	}
	t.Parallel()

	dir := filepath.Join("about", "nodejs")
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(dir)

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "about-nodejs")
	e.RunCommand("pulumi", "stack", "select", "about-nodejs")
	stdout, stderr := e.RunCommand("pulumi", "about")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
	// Assert we parsed the dependencies
	assert.Containsf(t, stdout, "@types/node",
		"Did not contain expected output. stderr: \n%q", stderr)
	// Assert we parsed the language plugin, we don't assert against the minor version number
	assert.Regexp(t, regexp.MustCompile(`language\W+nodejs\W+3\.`), stdout)
}

func TestConstructOutputValuesNode(t *testing.T) {
	t.Parallel()
	testConstructOutputValues(t, "nodejs", "@pulumi/pulumi")
}

func TestTSConfigOption(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Skip on windows because we lack yarn")
	}
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("tsconfig")

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "select", "tsconfg", "--create")
	e.RunCommand("pulumi", "preview")
}

// This tests that despite an exception, that the snapshot is still written.
func TestUnsafeSnapshotManagerRetainsResourcesOnError(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("Check with experimental flag", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir:          filepath.Join("unsafe_snapshot_tests", "bad_resource"),
			Dependencies: []string{"@pulumi/pulumi"},
			Env: []string{
				"PULUMI_EXPERIMENTAL=1",
				"PULUMI_SKIP_CHECKPOINTS=1",
			},
			Quick: true,
			// The program throws an exception and 1 resource fails to be created.
			ExpectFailure: true,
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Ensure the checkpoint contains the 1003 other resources that were created
				// - stack
				// - provider
				// - `base` resource
				// - 1000 resources(via a for loop)
				// - NOT a resource that failed to be created dependent on the `base` resource output
				assert.NotNil(t, stackInfo.Deployment)
				assert.Equal(t, 3+1000, len(stackInfo.Deployment.Resources))
			},
		})
	})

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	t.Run("Check without experimental flag", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir:          filepath.Join("unsafe_snapshot_tests", "bad_resource"),
			Dependencies: []string{"@pulumi/pulumi"},
			Env: []string{
				"PULUMI_EXPERIMENTAL=0",
				"PULUMI_SKIP_CHECKPOINTS=1",
			},
			Quick: true,
			// The program throws an exception and 1 resource fails to be created.
			ExpectFailure: true,
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Ensure the checkpoint contains the 1003 other resources that were created
				// - stack
				// - provider
				// - `base` resource
				// - 1000 resources(via a for loop)
				// - NOT a resource that failed to be created dependent on the `base` resource output
				assert.NotNil(t, stackInfo.Deployment)
				assert.Equal(t, 3+1000, len(stackInfo.Deployment.Resources))
			},
		})
	})
}

// TestResourceRefsGetResourceNode tests that invoking the built-in 'pulumi:pulumi:getResource' function
// returns resource references for any resource reference in a resource's state.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestResourceRefsGetResourceNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("resource_refs_get_resource", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestDeletedWithNode tests the DeletedWith resource option.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDeletedWithNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("deleted_with", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		Quick: true,
	})
}

// Tests custom resource type name of dynamic provider.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestCustomResourceTypeNameDynamicNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("dynamic", "nodejs-resource-type-name"),
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			urnOut := stack.Outputs["urn"].(string)
			urn := resource.URN(urnOut)
			typ := urn.Type().String()
			assert.Equal(t, "pulumi-nodejs:dynamic/custom-provider:CustomResource", typ)
		},
	})
}

// Tests errors in dynamic provider methods
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestErrorCreateDynamicNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("dynamic", "nodejs-error-create"),
		Dependencies:  []string{"@pulumi/pulumi"},
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			foundError := false
			for _, event := range stack.Events {
				if event.ResOpFailedEvent != nil {
					foundError = true
					assert.Equal(t, apitype.OpType("create"), event.ResOpFailedEvent.Metadata.Op)
				}
			}
			assert.True(t, foundError, "Did not see create error")
		},
	})
}

// Tests configuration for dynamic providers
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsDynamicProviderConfig(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("dynamic", "nodejs-config"),
		Dependencies: []string{"@pulumi/pulumi"},
		Secrets: map[string]string{
			"password":      "s3cret",
			"colors:banana": "yellow",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure the resulting output had the expected value
			code, ok := stackInfo.Outputs["authenticated"].(string)
			assert.True(t, ok)
			assert.Equal(t, "200", code)

			color, ok := stackInfo.Outputs["color"].(string)
			assert.True(t, ok)
			assert.Equal(t, "yellow", color)
		},
	})
}

// Regression test for https://github.com/pulumi/pulumi/issues/12301
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestRegression12301Node(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "regression-12301"),
		Dependencies: []string{"@pulumi/pulumi"},
		PostPrepareProject: func(project *engine.Projinfo) error {
			// Move the bad JSON file up one directory
			jsonPath := filepath.Join(project.Root, "regression-12301.json")
			dirName := filepath.Base(project.Root)
			newPath := filepath.Join(project.Root, "..", dirName+".json")
			return os.Rename(jsonPath, newPath)
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Len(t, stack.Outputs, 1)
			assert.Contains(t, stack.Outputs, "bar")
			assert.Equal(t, 3.0, stack.Outputs["bar"].(float64))
		},
	})
}

// Tests provider config is passed through to provider processes.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPulumiConfig(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("dynamic", "nodejs-pulumi-config"),
		Dependencies: []string{"@pulumi/pulumi"},
		Config: map[string]string{
			"pulumi-nodejs:id": "testing123",
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Len(t, stack.Outputs, 1)
			assert.Contains(t, stack.Outputs, "rid")
			assert.Equal(t, "testing123", stack.Outputs["rid"].(string))
		},
	})
}

func TestConstructProviderPropagationNode(t *testing.T) {
	t.Parallel()

	testConstructProviderPropagation(t, "nodejs", []string{"@pulumi/pulumi"})
}

func TestConstructProviderExplicitNode(t *testing.T) {
	t.Parallel()

	testConstructProviderExplicit(t, "nodejs", []string{"@pulumi/pulumi"})
}

// Regression test for https://github.com/pulumi/pulumi/issues/7376
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestUndefinedStackOutputNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "undefined-stack-output"),
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.Equal(t, nil, stack.Outputs["nil"])
			assert.Equal(t, []interface{}{0.0, nil, nil}, stack.Outputs["list"])
			assert.Equal(t, map[string]interface{}{
				"nil2":    nil,
				"number2": 0.0,
			}, stack.Outputs["map"])

			var found bool
			for _, event := range stack.Events {
				if event.DiagnosticEvent != nil {
					if event.DiagnosticEvent.Severity == "warning" &&
						strings.Contains(event.DiagnosticEvent.Message, "will not show as a stack output") {
						assert.Equal(t,
							"Undefined value (undef) will not show as a stack output.\n",
							event.DiagnosticEvent.Message)
						found = true
					}
				}
			}
			assert.True(t, found, "Did not see undef warning")
		},
	})
}

// Tests basic environments from the perspective of a Pulumi program.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEnvironmentsBasicNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:            filepath.Join("environments_basic"),
		Dependencies:   []string{"@pulumi/pulumi"},
		Quick:          true,
		RequireService: true,
		CreateEnvironments: []integration.Environment{{
			Name: "basic",
			Definition: map[string]any{
				"values": map[string]any{
					"pulumiConfig": map[string]any{
						"aConfigValue": "this value is a value",
						"bEncryptedSecret": map[string]any{
							"fn::secret": "this super secret is encrypted",
						},
						"outer": map[string]any{
							"inner": "value",
						},
						"names": []any{"a", "b", "c", map[string]any{"fn::secret": "super secret name"}},
						"servers": []any{
							map[string]any{
								"port": 80,
								"host": "example",
							},
						},
						"a": map[string]any{
							"b": []any{
								map[string]any{"c": true},
								map[string]any{"c": false},
							},
						},
						"tokens": []any{
							map[string]any{
								"fn::secret": "shh",
							},
						},
						"foo": map[string]any{
							"bar": map[string]any{
								"fn::secret": "don't tell",
							},
						},
					},
				},
			},
		}},
		Environments: []string{"basic"},
	})
}

// Tests merged environments from the perspective of a Pulumi program.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEnvironmentsMergeNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:            filepath.Join("environments_merge"),
		Dependencies:   []string{"@pulumi/pulumi"},
		Quick:          true,
		RequireService: true,
		CreateEnvironments: []integration.Environment{
			{
				Name: "merge-0",
				Definition: map[string]any{
					"values": map[string]any{
						"pulumiConfig": map[string]any{
							"outer": map[string]any{
								"inner": "not-a-value",
							},
							"names": []any{"a", "b", "c", map[string]any{"fn::secret": "super secret name"}},
							"servers": []any{
								map[string]any{
									"port": 80,
									"host": "example",
								},
							},
						},
					},
				},
			},
			{
				Name: "merge-1",
				Definition: map[string]any{
					"values": map[string]any{
						"pulumiConfig": map[string]any{
							"a": map[string]any{
								"b": []any{
									map[string]any{"c": true},
									map[string]any{"c": false},
								},
							},
							"tokens": []any{
								map[string]any{
									"fn::secret": "shh",
								},
							},
							"foo": map[string]any{
								"bar": "not so secret",
							},
						},
					},
				},
			},
		},
		Environments: []string{"merge-0", "merge-1"},
		Config: map[string]string{
			"aConfigValue": "this value is a value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super secret is encrypted",
		},
		OrderedConfig: []integration.ConfigValue{
			{Key: "outer.inner", Value: "value", Path: true},
			{Key: "foo.bar", Value: "don't tell", Path: true, Secret: true},
		},
	})
}

// Tests errors that would occur when generating code that shadows reserved
// identifiers (e.g. "const exports = ...").
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodeJSReservedIdentifierShadowing(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("dynamic", "nodejs-reserved-identifier-shadowing"),
		Dependencies:  []string{"@pulumi/pulumi"},
		ExpectFailure: false,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			noError := true
			for _, event := range stack.Events {
				if event.ResOpFailedEvent != nil {
					noError = false
					assert.Equal(t, apitype.OpType("create"), event.ResOpFailedEvent.Metadata.Op)
				}
			}

			assert.True(t, noError, "An error occurred when testing shadowing of reserved identifiers")
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodeOOM(t *testing.T) {
	stderr := &bytes.Buffer{}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("nodejs", "oom"),
		Dependencies:  []string{"@pulumi/pulumi"},
		ExpectFailure: true,
		Stderr:        stderr,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			t.Logf("stdout: %s", stderr.String())
			assert.Contains(t, stderr.String(), "Detected a possible out of memory error")
		},
	})
}

// Test a parameterized provider with nodejs.
//
//nolint:paralleltest // mutates environment
func TestParameterizedNode(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	// Enable MultiArgumentInputs in the testprovider/echo schema
	t.Setenv("PULUMI_TEST_MULTI_ARGUMENT_INPUTS", "true")

	// We can't use ImportDirectory here because we need to run this in the right directory such that the relative paths
	// work.
	var err error
	e.CWD, err = filepath.Abs("nodejs/parameterized")
	require.NoError(t, err)

	err = os.RemoveAll(filepath.Join("nodejs", "parameterized", "sdk"))
	require.NoError(t, err)

	_, _ = e.RunCommand("pulumi", "package", "gen-sdk", "../../../testprovider", "pkg", "--language", "nodejs", "--local")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Verbose:       true,
		DebugLogLevel: 10,
		Dir:           filepath.Join("nodejs", "parameterized"),
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
		NoParallel: true,
		PrePrepareProject: func(project *engine.Projinfo) error {
			// Patch up the local SDK's package.json so its pulumi dependency points to the local core SDK
			coreSDK, err := filepath.Abs(filepath.Join("..", "..", "sdk", "nodejs", "bin"))
			if err != nil {
				return err
			}
			packageJSON := filepath.Join(project.Root, "sdk", "nodejs", "package.json")

			fmt.Println("coreSDK", coreSDK)
			fmt.Println("packagejson", packageJSON)

			data, err := os.ReadFile(packageJSON)
			if err != nil {
				return err
			}
			var pkgJSON map[string]interface{}
			err = json.Unmarshal(data, &pkgJSON)
			if err != nil {
				return err
			}
			deps := pkgJSON["dependencies"].(map[string]interface{})
			deps["@pulumi/pulumi"] = "file:" + coreSDK
			data, err = json.MarshalIndent(pkgJSON, "", "  ")
			if err != nil {
				return err
			}
			err = os.WriteFile(packageJSON, data, 0o600)
			if err != nil {
				return err
			}

			return nil
		},
	})
}

//nolint:paralleltest // mutates environment
func TestPackageAddNode(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	for _, packageManager := range []string{"npm", "yarn", "pnpm"} {
		t.Run(packageManager, func(t *testing.T) {
			var err error
			templatePath, err := filepath.Abs("nodejs/packageadd_" + packageManager)
			require.NoError(t, err)
			err = fsutil.CopyFile(e.CWD, templatePath, nil)
			require.NoError(t, err)

			_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "random")
			_, _ = e.RunCommand("pulumi", "package", "add", "random")
			assert.True(t, e.PathExists("sdks/random"))

			packagesJSONBytes, err := os.ReadFile(filepath.Join(e.CWD, "package.json"))
			assert.NoError(t, err)
			packagesJSON := make(map[string]any)
			err = json.Unmarshal(packagesJSONBytes, &packagesJSON)
			assert.NoError(t, err)

			dependencies, ok := packagesJSON["dependencies"].(map[string]any)
			assert.True(t, ok)
			cf, ok := dependencies["@pulumi/random"]
			assert.True(t, ok)
			cf, ok = cf.(string)
			assert.True(t, ok)

			assert.Equal(t, "file:sdks/random", cf)
		})
	}
}

//nolint:paralleltest // mutates environment
func TestConvertTerraformProviderNode(t *testing.T) {
	e := ptesting.NewEnvironment(t)

	var err error
	templatePath, err := filepath.Abs("convertfromterraform")
	require.NoError(t, err)
	err = fsutil.CopyFile(e.CWD, templatePath, nil)
	require.NoError(t, err)

	_, _ = e.RunCommand("pulumi", "plugin", "install", "converter", "terraform")
	_, _ = e.RunCommand("pulumi", "plugin", "install", "resource", "terraform-provider")
	_, _ = e.RunCommand("pulumi", "convert", "--from", "terraform", "--language", "typescript", "--out", "nodedir")

	packagesJSONBytes, err := os.ReadFile(filepath.Join(e.CWD, "nodedir/package.json"))
	assert.NoError(t, err)
	packagesJSON := make(map[string]any)
	err = json.Unmarshal(packagesJSONBytes, &packagesJSON)
	assert.NoError(t, err)

	dependencies, ok := packagesJSON["dependencies"].(map[string]any)
	assert.True(t, ok)
	cf, ok := dependencies["@pulumi/supabase"]
	assert.True(t, ok)
	cf, ok = cf.(string)
	assert.True(t, ok)

	assert.Equal(t, "file:sdks/supabase", cf)
}

func TestConstructFailuresNode(t *testing.T) {
	t.Parallel()
	testConstructFailures(t, "nodejs", "@pulumi/pulumi")
}

func TestCallFailuresNode(t *testing.T) {
	t.Parallel()
	testCallFailures(t, "nodejs", "@pulumi/pulumi")
}

// TestLogDebugNode tests that the amount of debug logs is reasonable.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestLogDebugNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("log_debug", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
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

// Test custom autonaming configurations.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestAutonaming(t *testing.T) {
	testCases := []struct {
		name     string
		autoName string
		config   map[string]string
	}{
		{
			name:     "no autonaming configured",
			autoName: "default-name",
		},
		{
			name: "autonaming configured globally to verbatim",
			config: map[string]string{
				"pulumi:autonaming.mode": "verbatim",
			},
			autoName: "test1",
		},
		{
			name: "autonaming configured on provider to a pattern",
			config: map[string]string{
				"pulumi:autonaming.providers.testprovider.pattern": "${config.foo}-${name}",
				"foo": "bar",
			},
			autoName: "bar-test1",
		},
	}

	for _, tc := range testCases {
		tc := tc
		orderedConfig := []integration.ConfigValue{}
		for k, v := range tc.config {
			orderedConfig = append(orderedConfig, integration.ConfigValue{Key: k, Value: v, Path: true})
		}
		env := []string{}
		t.Run(tc.name, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:           "autonaming",
				Dependencies:  []string{"@pulumi/pulumi"},
				Env:           env,
				OrderedConfig: orderedConfig,
				LocalProviders: []integration.LocalDependency{
					{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
				},
				Quick: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "explicit-name", stackInfo.RootResource.Outputs["explicitName"])
					assert.Equal(t, tc.autoName, stackInfo.RootResource.Outputs["autoName"])
					assert.True(t, strings.HasSuffix(stackInfo.RootResource.Outputs["componentUrn"].(string), "::test3"),
						"Expected componentUrn to end with '::test3', got %v", stackInfo.RootResource.Outputs["componentUrn"])
				},
			})
		})
	}
}

func TestNodejsSourcemapTest(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("nodejs/sourcemap-in-test")
	e.RunCommand("yarn", "install")
	coreSDK, err := filepath.Abs(filepath.Join("..", "..", "sdk", "nodejs", "bin"))
	require.NoError(t, err)
	e.RunCommand("yarn", "add", coreSDK)

	_, stderr := e.RunCommandExpectError("yarn", "test")

	expectedTrace := `a failing test so we can inspect the stacktrace reported by jest

    this is a test error

      1 | export function willThrow() {
      2 |     if (true) {
    > 3 |         throw new Error("this is a test error");
        |               ^
      4 |     }
      5 | }
      6 |

      at willThrow (index.ts:3:15)
`
	require.Contains(t, stripansi.Strip(stderr), expectedTrace)
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSourcemapProgramTypescript(t *testing.T) {
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("nodejs", "sourcemap-in-program"),
		Dependencies:  []string{"@pulumi/pulumi"},
		ExpectFailure: true,
		Stderr:        stderr,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			require.Regexp(t, "Error: this is a test error\n.*at willThrow.*index.ts:6:15", stderr.String())
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSourcemapProgramJavascript(t *testing.T) {
	stderr := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           filepath.Join("nodejs", "sourcemap-in-program-precompiled"),
		Dependencies:  []string{"@pulumi/pulumi"},
		ExpectFailure: true,
		Stderr:        stderr,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			require.Regexp(t, "Error: this is a test error\n.*at willThrow.*index.ts:6:15", stderr.String())
		},
	})
}

func TestPackageAddProviderFromRemoteSource(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)

	e.ImportDirectory("packageadd-remote")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.Env = append(e.Env, "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false")
	e.RunCommand("pulumi", "stack", "select", "organization/packageadd-remote", "--create")

	// The broken provider doesn't succeed in generating the SDK.  We still want to see it installed,
	// and want to make sure we can still install a different provider from a different subdirectory,
	// from the same repository and the same revision.
	e.RunCommandExpectError("pulumi", "package", "add",
		"github.com/pulumi/component-test-providers/broken-test-provider@d47cf0910e0450400775594609ee82566d1fb355")
	stdout, _ := e.RunCommand("pulumi", "plugin", "ls")
	require.Contains(t, stdout, "github.com_pulumi_component-test-providers")
	require.Contains(t, stdout, "0.0.0-xd47cf0910e0450400775594609ee82566d1fb355")

	e.RunCommand("pulumi", "package", "add",
		"github.com/pulumi/component-test-providers/test-provider@d47cf0910e0450400775594609ee82566d1fb355")

	e.RunCommand("yarn", "add", "tls-self-signed-cert@file:sdks/tls-self-signed-cert")

	e.Env = []string{"PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"}
	// Ensure the plugin our package needs is installed manually.  We want to turn off automatic
	// plugin acquisition here to show that the pulumi-tls-self-signed-cert from the package add
	// above is used.
	e.RunCommand("pulumi", "plugin", "install", "resource", "tls", "v4.11.1")
	stdout, _ = e.RunCommand("pulumi", "plugin", "ls")
	require.Contains(t, stdout, "github.com_pulumi_component-test-providers")
	require.Contains(t, stdout, "0.0.0-xd47cf0910e0450400775594609ee82566d1fb355")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview")
}

// Tests that we can get the schema for a Node.js component provider using component_provider_host.
func TestNodejsComponentProviderGetSchema(t *testing.T) {
	t.Parallel()
	dir, err := filepath.Abs(filepath.Join("component_provider", "nodejs", "component-provider-host", "provider"))
	require.NoError(t, err)
	installNodejsProviderDependencies(t, dir)

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Run the command from a different, sibling, directory. This ensures that
	// get-package does not rely on the current working directory.
	e.CWD = t.TempDir()
	stdout, stderr := e.RunCommand("pulumi", "package", "get-schema", dir)
	require.Empty(t, stderr)
	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &schema))
	require.Equal(t, "nodejs-component-provider", schema["name"].(string))
	require.Equal(t, "1.2.3", schema["version"].(string))
	require.Equal(t, "Node.js Sample Components", schema["description"].(string))

	// Check the component schema
	expectedJSON := `{
		"isComponent": true,
		"type": "object"
	}
	`
	expected := make(map[string]interface{})
	resources := schema["resources"].(map[string]interface{})
	component := resources["nodejs-component-provider:index:MyComponent"].(map[string]interface{})
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &expected))
	require.Equal(t, expected, component)
}

// Tests that we can run a Node.js component provider using component_provider_host
func TestNodejsComponentProviderRun(t *testing.T) {
	t.Parallel()

	testData, err := filepath.Abs(filepath.Join("component_provider", "nodejs", "component-provider-host"))
	require.NoError(t, err)
	providerDir := filepath.Join(testData, "provider")
	installNodejsProviderDependencies(t, providerDir)

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, runtime := range []string{"yaml", "python"} {
		t.Run(runtime, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				PrepareProject: func(info *engine.Projinfo) error {
					if runtime != "yaml" {
						cmd := exec.Command("pulumi", "package", "add", providerDir)
						cmd.Dir = info.Root
						out, err := cmd.CombinedOutput()
						require.NoError(t, err, "%s failed with: %s", cmd.String(), string(out))
					}
					return nil
				},
				Dir: filepath.Join(testData, runtime),
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					t.Logf("Outputs: %v", stack.Outputs)
					urn, err := resource.ParseURN(stack.Outputs["urn"].(string))
					require.NoError(t, err)
					require.Equal(t, tokens.Type("nodejs-component-provider:index:MyComponent"), urn.Type())
					require.Equal(t, "comp", urn.Name())
				},
			})
		})
	}
}

// lock to prevent concurrent installation of provider dependencies
var installNodejsProviderDependenciesLock sync.Mutex

func installNodejsProviderDependencies(t *testing.T, dir string) {
	t.Helper()

	installNodejsProviderDependenciesLock.Lock()
	defer installNodejsProviderDependenciesLock.Unlock()

	pm, err := npm.ResolvePackageManager(npm.YarnPackageManager, dir)
	require.NoError(t, err)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = pm.Install(context.Background(), dir, false /* production*/, stdout, stderr)
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	cmd := exec.Command("yarn", "link", "@pulumi/pulumi")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", out)
}
