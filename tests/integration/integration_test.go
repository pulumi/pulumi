// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// TestEmptyNodeJS simply tests that we can run an empty NodeJS project.
func TestEmptyNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("empty", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestEmptyPython simply tests that we can run an empty Python project.
func TestEmptyPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("empty", "python"),
		Quick: true,
	})
}

// TestEmptyGo simply tests that we can run an empty Go project.
func TestEmptyGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("empty", "go"),
		Quick: true,
	})
}

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
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
		stdout, stderr := e.RunCommandExpectError("pulumi", "up", "--non-interactive", "--skip-preview")
		assert.Equal(t, "Updating (main-abs):\n", stdout)
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
		stdout, stderr := e.RunCommandExpectError("pulumi", "up", "--non-interactive", "--skip-preview")
		assert.Equal(t, "Updating (main-parent):\n", stdout)
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

// TestStackTagValidation verifies various error scenarios related to stack names and tags.
func TestStackTagValidation(t *testing.T) {
	t.Run("Error_StackName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "invalid name (spaces, parens, etc.)")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack name may only contain alphanumeric, hyphens, underscores, or periods")
	})

	t.Run("Error_DescriptionLength", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		prefix := "lorem ipsum dolor sit amet"     // 26
		prefix = prefix + prefix + prefix + prefix // 104
		prefix = prefix + prefix + prefix + prefix // 416 + the current Pulumi.yaml's description

		// Change the contents of the Description property of Pulumi.yaml.
		yamlPath := path.Join(e.CWD, "Pulumi.yaml")
		err := integration.ReplaceInFile("description: ", "description: "+prefix, yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "valid-name")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack tag \"pulumi:description\" value is too long (max length 256 characters)")
	})
}

// TestStackOutputs ensures we can export variables from a stack and have them get recorded as outputs.
func TestStackOutputs(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_outputs",
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
	e.ImportDirectory("stack_outputs")
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "stack-outs")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview")
	stdout, stderr := e.RunCommand("pulumi", "stack", "output", "--json")
	assert.Equal(t, `{
  "foo": 42,
  "xyz": "ABC"
}
`, stdout)
	assert.Equal(t, "", stderr)
}

// TestStackOutputsDisplayed ensures that outputs are printed at the end of an update
func TestStackOutputsDisplayed(t *testing.T) {
	stdout := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_outputs",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        false,
		Verbose:      true,
		Stdout:       stdout,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stdout.String()

			// ensure we get the outputs info both for the normal update, and for the no-change update.
			assert.Contains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n\nResources:\n    1 change")
			assert.Contains(t, output, "Outputs:\n    foo: 42\n    xyz: \"ABC\"\n\nResources:\n    0 changes")
		},
	})
}

// TestStackOutputsSuppressed ensures that outputs whose values are intentionally suppresses don't show.
func TestStackOutputsSuppressed(t *testing.T) {
	stdout := &bytes.Buffer{}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    "stack_outputs",
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
			fmt.Println(latest.Resources)
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

// TestConfigSave ensures that config commands in the Pulumi CLI work as expected.
func TestConfigSave(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Initialize an empty stack.
	path := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := (&workspace.Project{
		Name:    "testing-config",
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}).Save(path)
	assert.NoError(t, err)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "testing-2")
	e.RunCommand("pulumi", "stack", "init", "testing-1")

	// Now configure and save a few different things:
	e.RunCommand("pulumi", "config", "set", "configA", "value1")
	e.RunCommand("pulumi", "config", "set", "configB", "value2", "--stack", "testing-2")

	e.RunCommand("pulumi", "stack", "select", "testing-2")

	e.RunCommand("pulumi", "config", "set", "configD", "value4")
	e.RunCommand("pulumi", "config", "set", "configC", "value3", "--stack", "testing-1")

	// Now read back the config using the CLI:
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configB")
		assert.Equal(t, "value2\n", stdout)
	}
	{
		// the config in a different stack, so this should error.
		stdout, stderr := e.RunCommandExpectError("pulumi", "config", "get", "configA")
		assert.Equal(t, "", stdout)
		assert.NotEqual(t, "", stderr)
	}
	{
		// but selecting the stack should let you see it
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configA", "--stack", "testing-1")
		assert.Equal(t, "value1\n", stdout)
	}

	// Finally, check that the stack file contains what we expected.
	validate := func(k string, v string, cfg config.Map) {
		key, err := config.ParseKey("testing-config:config:" + k)
		assert.NoError(t, err)
		d, ok := cfg[key]
		assert.True(t, ok, "config key %v should be set", k)
		dv, err := d.Value(nil)
		assert.NoError(t, err)
		assert.Equal(t, v, dv)
	}

	testStack1, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-1.yaml"))
	assert.NoError(t, err)
	testStack2, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-2.yaml"))
	assert.NoError(t, err)

	assert.Equal(t, 2, len(testStack1.Config))
	assert.Equal(t, 2, len(testStack2.Config))

	validate("configA", "value1", testStack1.Config)
	validate("configC", "value3", testStack1.Config)

	validate("configB", "value2", testStack2.Config)
	validate("configD", "value4", testStack2.Config)

	e.RunCommand("pulumi", "stack", "rm", "--yes")
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
	})
}

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

func TestInvalidVersionInPackageJson(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("invalid_package_json"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config:       map[string]string{},
	})
}

// Tests basic configuration from the perspective of a Pulumi program.
func TestConfigBasicPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("config_basic", "python"),
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a Pythonic value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super Pythonic secret is encrypted",
		},
	})
}

// Tests basic configuration from the perspective of a Pulumi Go program.
func TestConfigBasicGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("config_basic", "go"),
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super secret is encrypted",
		},
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

			var defaultProvider *apitype.ResourceV2
			var explicitProvider *apitype.ResourceV2
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
func TestGetCreated(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "get_created",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}
