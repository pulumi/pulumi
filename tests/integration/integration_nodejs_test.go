// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build nodejs all

package ints

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

// TestEmptyNodeJS simply tests that we can run an empty NodeJS project.
func TestEmptyNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("empty", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// Tests emitting many engine events doesn't result in a performance problem.
func TestEngineEventPerf(t *testing.T) {
	// Prior to pulumi/pulumi#2303, a preview or update would take ~40s.
	// Since then, it should now be down to ~4s, with additional padding,
	// since some Travis machines (especially the macOS ones) seem quite slow
	// to begin with.
	benchmarkEnforcer := &assertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 8 * time.Second,
		MaxUpdateDuration:  8 * time.Second,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "ee_perf",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ReportStats:  benchmarkEnforcer,
		// Don't run in parallel since it is sensitive to system resources.
		NoParallel: true,
	})
}

// TestEngineEvents ensures that the test framework properly records and reads engine events.
func TestEngineEvents(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "single_resource",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		NoParallel:   true, // avoid contention for Dir
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure that we have a non-empty list of events.
			assert.NotEmpty(t, stackInfo.Events)

			// Ensure that we have two "ResourcePre" events: one for the stack and one for our resource.
			preEventResourceTypes := []string{}
			for _, e := range stackInfo.Events {
				if e.ResourcePreEvent != nil {
					preEventResourceTypes = append(preEventResourceTypes, e.ResourcePreEvent.Metadata.Type)
				}
			}

			assert.Equal(t, 2, len(preEventResourceTypes))
			assert.Contains(t, preEventResourceTypes, "pulumi:pulumi:Stack")
			assert.Contains(t, preEventResourceTypes, "pulumi-nodejs:dynamic:Resource")
		},
	})

}

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	test := integration.ProgramTestOptions{
		Dir:          "project_main",
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.NotNil(t, stackInfo.Deployment)
		},
	}
	integration.ProgramTest(t, &test)

	t.Run("AbsolutePath", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.ImportDirectory("project_main_abs")

		// write a new Pulumi.yaml using the absolute path of the environment as "main"
		yamlPath := filepath.Join(e.RootPath, "Pulumi.yaml")
		absYamlContents := fmt.Sprintf(
			"name: project_main_abs\ndescription: A program with an absolute entry point\nruntime: nodejs\nmain: %s\n",
			e.RootPath,
		)
		t.Logf("writing new Pulumi.yaml: \npath: %s\ncontents:%s", yamlPath, absYamlContents)
		if err := os.WriteFile(yamlPath, []byte(absYamlContents), 0644); err != nil {
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
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
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
func TestStackProjectName(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_project_name",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

func TestRemoveWithResourcesBlocked(t *testing.T) {
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

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

// TestStackOutputsJSON ensures the CLI properly formats stack outputs as JSON when requested.
func TestStackOutputsJSON(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()
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

					urns[string(res.URN.Name())] = res.URN
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
						// Default providers are not parented.
						assert.Equal(t, "", string(res.Parent))
					default:
						t.Fatalf("unexpected name %s", res.URN.Name())
					}
				}
			}
		},
	})
}

func TestStackBadParenting(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           "stack_bad_parenting",
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		ExpectFailure: true,
	})
}

// TestStackDependencyGraph tests that the dependency graph of a stack is saved
// in the checkpoint file.
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

func TestConfigCaptureNodeJS(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
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

func TestInvalidVersionInPackageJson(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("invalid_package_json"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config:       map[string]string{},
	})
}

// Tests an explicit provider instance.
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

// Tests that stack references work in Node.
func TestStackReferenceNodeJS(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	opts := &integration.ProgramTestOptions{
		Dir:          filepath.Join("stack_reference", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
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

// Tests that reads of unknown IDs do not fail.
func TestGetCreated(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "get_created",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestProviderSecretConfig that a first class provider can be created when it has secrets as part of its config.
func TestProviderSecretConfig(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "provider_secret_config",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

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

func TestStackReferenceSecretsNodejs(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	owner := os.Getenv("PULUMI_TEST_OWNER")
	if owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	d := "stack_reference_secrets"

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join(d, "nodejs", "step1"),
		Dependencies: []string{"@pulumi/pulumi"},
		Config: map[string]string{
			"org": owner,
		},
		Quick: true,
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join(d, "nodejs", "step2"),
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

func TestPasswordlessPassphraseSecretsProvider(t *testing.T) {
	testOptions := integration.ProgramTestOptions{
		Dir:             "cloud_secrets_provider",
		Dependencies:    []string{"@pulumi/pulumi"},
		SecretsProvider: fmt.Sprintf("passphrase"),
		Env:             []string{"PULUMI_CONFIG_PASSPHRASE=\"\""},
		NoParallel:      true,
		Secrets: map[string]string{
			"mysecret": "THISISASECRET",
		},
		CloudURL: "file://~",
	}

	workingTestOptions := testOptions.With(integration.ProgramTestOptions{
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "passphrase")

			_, err := passphrase.NewPassphaseSecretsManagerFromState(secretsProvider.State)
			assert.NoError(t, err)

			out, ok := stackInfo.Outputs["out"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
			os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
		},
	})

	brokenTestOptions := testOptions.With(integration.ProgramTestOptions{
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "passphrase")

			_, err := passphrase.NewPassphaseSecretsManagerFromState(secretsProvider.State)
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
	awsKmsKeyAlias := os.Getenv("PULUMI_TEST_KMS_KEY_ALIAS")
	if awsKmsKeyAlias == "" {
		t.Skipf("Skipping: PULUMI_TEST_KMS_KEY_ALIAS is not set")
	}

	azureKeyVault := os.Getenv("PULUMI_TEST_AZURE_KEY")
	if azureKeyVault == "" {
		t.Skipf("Skipping: PULUMI_TEST_AZURE_KEY is not set")
	}

	gcpKmsKey := os.Getenv("PULUMI_TEST_GCP_KEY")
	if azureKeyVault == "" {
		t.Skipf("Skipping: PULUMI_TEST_GCP_KEY is not set")
	}

	// Generic test options for all providers
	testOptions := integration.ProgramTestOptions{
		Dir:             "cloud_secrets_provider",
		Dependencies:    []string{"@pulumi/pulumi"},
		SecretsProvider: fmt.Sprintf("awskms://alias/%s", awsKmsKeyAlias),
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
		CloudURL: "file://~",
	})

	azureTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: fmt.Sprintf("azurekeyvault://%s", azureKeyVault),
	})

	gcpTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: fmt.Sprintf("gcpkms://projects/%s", gcpKmsKey),
	})

	// Run with default Pulumi service backend
	t.Run("service", func(t *testing.T) {
		integration.ProgramTest(t, &testOptions)
	})

	// Check Azure secrets provider
	t.Run("azure", func(t *testing.T) { integration.ProgramTest(t, &azureTestOptions) })

	// Check gcloud secrets provider
	t.Run("gcp", func(t *testing.T) { integration.ProgramTest(t, &gcpTestOptions) })

	// Also run with local backend
	t.Run("local", func(t *testing.T) { integration.ProgramTest(t, &localTestOptions) })

}

// Tests a resource with a large (>4mb) string prop in Node.js
func TestLargeResourceNode(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("large_resource", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
	})
}

// Tests enum outputs
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

// Test remote component construction in Node.
func TestConstructNode(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows")
	}

	tests := []struct {
		componentDir          string
		expectedResourceCount int
		env                   []string
	}{
		{
			componentDir:          "testcomponent",
			expectedResourceCount: 9,
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
				optsForConstructNode(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructNode(t *testing.T, expectedResourceCount int, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env:          env,
		Dir:          filepath.Join("construct_component", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Secrets: map[string]string{
			"secret": "this super secret is encrypted",
		},
		Quick:      true,
		NoParallel: true,
		// verify that additional flags don't cause the component provider hang
		UpdateCommandlineFlags: []string{"--logflow", "--logtostderr"},
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
func TestConstructSlowNode(t *testing.T) {
	pathEnv := testComponentSlowPathEnv(t)

	var opts *integration.ProgramTestOptions
	opts = &integration.ProgramTestOptions{
		Env:          []string{pathEnv},
		Dir:          filepath.Join("construct_component_slow", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
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
func TestConstructPlainNode(t *testing.T) {
	tests := []struct {
		componentDir          string
		expectedResourceCount int
		env                   []string
	}{
		{
			componentDir:          "testcomponent",
			expectedResourceCount: 9,
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
				optsForConstructPlainNode(t, test.expectedResourceCount, append(test.env, pathEnv)...))
		})
	}
}

func optsForConstructPlainNode(t *testing.T, expectedResourceCount int, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env:          env,
		Dir:          filepath.Join("construct_component_plain", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		NoParallel:   true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, expectedResourceCount, len(stackInfo.Deployment.Resources))
		},
	}
}

// Test remote component inputs properly handle unknowns.
func TestConstructUnknownNode(t *testing.T) {
	testConstructUnknown(t, "nodejs", "@pulumi/pulumi")
}

// Test methods on remote components.
func TestConstructMethodsNode(t *testing.T) {
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
				Env:          []string{pathEnv},
				Dir:          filepath.Join("construct_component_methods", "nodejs"),
				Dependencies: []string{"@pulumi/pulumi"},
				Quick:        true,
				NoParallel:   true, // avoid contention for Dir
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message"])
				},
			})
		})
	}
}

func TestGetResourceNode(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                      filepath.Join("get_resource", "nodejs"),
		Dependencies:             []string{"@pulumi/pulumi"},
		AllowEmptyPreviewChanges: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, "foo", stack.Outputs["foo"])
		},
	})
}

func TestComponentProviderSchemaNode(t *testing.T) {
	path := filepath.Join("component_provider_schema", "testcomponent", "pulumi-resource-testcomponent")
	if runtime.GOOS == WindowsOS {
		path += ".cmd"
	}
	testComponentProviderSchema(t, path)
}

// Test throwing an error within an apply in a remote component written in nodejs.
// The provider should return the error and shutdown gracefully rather than hanging.
func TestConstructNodeErrorApply(t *testing.T) {
	dir := "construct_component_error_apply"
	componentDir := "testcomponent"

	stderr := &bytes.Buffer{}
	expectedError := "intentional error from within an apply"

	opts := &integration.ProgramTestOptions{
		Env:           []string{pathEnv(t, filepath.Join(dir, componentDir))},
		Dir:           filepath.Join(dir, "nodejs"),
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		NoParallel:    true,
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
