// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
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

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/v2/testing/integration"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v2/go/common/testing"
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

	t.Run("Error_AbsolutePath", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.ImportDirectory("project_main_abs")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-abs")
		stdout, stderr := e.RunCommandExpectError("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		assert.Equal(t, "Updating (main-abs):\n \n", stdout)
		assert.Contains(t, stderr, "project 'main' must be a relative path")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	t.Run("Error_ParentFolder", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.ImportDirectory("project_main_parent")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-parent")
		stdout, stderr := e.RunCommandExpectError("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		assert.Equal(t, "Updating (main-parent):\n \n", stdout)
		assert.Contains(t, stderr, "project 'main' must be a subfolder")
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
	t.Run("service", func(t *testing.T) { integration.ProgramTest(t, &testOptions) })

	// Check Azure secrets provider
	t.Run("azure", func(t *testing.T) { integration.ProgramTest(t, &azureTestOptions) })

	// Check gcloud secrets provider
	t.Run("gcp", func(t *testing.T) { integration.ProgramTest(t, &gcpTestOptions) })

	// Also run with local backend
	t.Run("local", func(t *testing.T) { integration.ProgramTest(t, &localTestOptions) })

}

//Tests a resource with a large (>4mb) string prop in Node.js
func TestLargeResourceNode(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("large_resource", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
	})
}
