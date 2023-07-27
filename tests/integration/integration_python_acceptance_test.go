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

//go:build python || all

package ints

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

func boolPointer(b bool) *bool {
	return &b
}

// TestEmptyPython simply tests that we can run an empty Python project.
func TestEmptyPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
	})
}

func TestStackReferencePython(t *testing.T) {
	opts := &integration.ProgramTestOptions{
		RequireService: true,

		Dir: filepath.Join("stack_reference", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
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

// Tests dynamic provider in Python.
func TestDynamicPython(t *testing.T) {
	var randomVal string
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("dynamic", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			randomVal = stack.Outputs["random_val"].(string)
		},
		EditDirs: []integration.EditDir{{
			Dir:      "step1",
			Additive: true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				assert.Equal(t, randomVal, stack.Outputs["random_val"].(string))

				// Regression testing the workaround for https://github.com/pulumi/pulumi/issues/8265
				// Ensure the __provider input and output was marked secret
				assertIsSecret := func(v interface{}) {
					switch v := v.(type) {
					case string:
						assert.Fail(t, "__provider was not a secret")
					case map[string]interface{}:
						assert.Equal(t, resource.SecretSig, v[resource.SigKey])
					}
				}

				dynRes := stack.Deployment.Resources[2]
				assertIsSecret(dynRes.Inputs["__provider"])
				assertIsSecret(dynRes.Outputs["__provider"])
			},
		}},
		UseSharedVirtualEnv: boolPointer(false),
	})
}

// Test remote component construction in Python.
func TestConstructPython(t *testing.T) {
	t.Parallel()

	testDir := "construct_component"
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
				optsForConstructPython(t, test.expectedResourceCount, localProviders, test.env...))
		})
	}
}

func optsForConstructPython(t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency, env ...string) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		LocalProviders: localProviders,
		Secrets: map[string]string{
			"secret": "this super secret is encrypted",
		},
		Quick:               true,
		UseSharedVirtualEnv: boolPointer(false),
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

func TestConstructComponentConfigureProviderPython(t *testing.T) {
	const testDir = "construct_component_configure_provider"
	runComponentSetup(t, testDir)
	pulumiRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	pulumiPySDK := filepath.Join("..", "..", "sdk", "python", "env", "src")
	componentSDK := filepath.Join(pulumiRoot, "pkg/codegen/testing/test/testdata/methods-return-plain-resource/python")
	opts := testConstructComponentConfigureProviderCommonOptions()
	opts = opts.With(integration.ProgramTestOptions{
		Dir:          filepath.Join(testDir, "python"),
		Dependencies: []string{pulumiPySDK, componentSDK},
	})
	integration.ProgramTest(t, &opts)
}

// Regresses https://github.com/pulumi/pulumi/issues/6471
func TestAutomaticVenvCreation(t *testing.T) {
	t.Parallel()

	// Do not use integration.ProgramTest to avoid automatic venv
	// handling by test harness; we actually are testing venv
	// handling by the pulumi CLI itself.

	check := func(t *testing.T, venvPathTemplate string, dir string) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		venvPath := strings.ReplaceAll(venvPathTemplate, "${root}", e.RootPath)
		t.Logf("venvPath = %s (IsAbs = %v)", venvPath, filepath.IsAbs(venvPath))

		e.ImportDirectory(dir)

		// replace "virtualenv: venv" with "virtualenv: ${venvPath}" in Pulumi.yaml
		pulumiYaml := filepath.Join(e.RootPath, "Pulumi.yaml")

		oldYaml, err := os.ReadFile(pulumiYaml)
		if err != nil {
			t.Error(err)
			return
		}
		newYaml := []byte(strings.ReplaceAll(string(oldYaml),
			"virtualenv: venv",
			fmt.Sprintf("virtualenv: >-\n      %s", venvPath)))

		if err := os.WriteFile(pulumiYaml, newYaml, 0o644); err != nil {
			t.Error(err)
			return
		}

		t.Logf("Wrote Pulumi.yaml:\n%s\n", string(newYaml))

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "teststack")
		e.RunCommand("pulumi", "preview")

		var absVenvPath string
		if filepath.IsAbs(venvPath) {
			absVenvPath = venvPath
		} else {
			absVenvPath = filepath.Join(e.RootPath, venvPath)
		}

		if !python.IsVirtualEnv(absVenvPath) {
			t.Errorf("Expected a virtual environment to be created at %s but it is not there",
				absVenvPath)
		}
	}

	t.Run("RelativePath", func(t *testing.T) {
		check(t, "venv", filepath.Join("python", "venv"))
	})

	t.Run("AbsolutePath", func(t *testing.T) {
		check(t, filepath.Join("${root}", "absvenv"), filepath.Join("python", "venv"))
	})

	t.Run("RelativePathWithMain", func(t *testing.T) {
		check(t, "venv", filepath.Join("python", "venv-with-main"))
	})

	t.Run("AbsolutePathWithMain", func(t *testing.T) {
		check(t, filepath.Join("${root}", "absvenv"), filepath.Join("python", "venv-with-main"))
	})

	t.Run("TestInitVirtualEnvBeforePythonVersionCheck", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		dir := filepath.Join("python", "venv")
		e.ImportDirectory(dir)

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "teststack")
		stdout, stderr, _ := e.GetCommandResults("pulumi", "preview")
		// pulumi/pulumi#9175
		// Ensures this error message doesn't show up for uninitialized
		// virtualenv
		//     `Failed to resolve python version command: ` +
		//     `fork/exec <path>/venv/bin/python: ` +
		//     `no such file or directory`
		assert.NotContains(t, stdout, "fork/exec")
		assert.NotContains(t, stderr, "fork/exec")
	})
}
