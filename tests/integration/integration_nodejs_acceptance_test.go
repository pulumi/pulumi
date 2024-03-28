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

//go:build nodejs || all

package ints

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// TestEmptyNodeJS simply tests that we can run an empty NodeJS project.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEmptyNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("empty", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// Tests that stack references work in Node.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestStackReferenceNodeJS(t *testing.T) {
	t.Skip("Temporarily skipping test - pulumi/pulumi#14765")
	opts := &integration.ProgramTestOptions{
		RequireService: true,

		Dir:          filepath.Join("stack_reference", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      filepath.Join("stack_reference", "nodejs", "step1"),
				Additive: true,
			},
			{
				Dir:      filepath.Join("stack_reference", "nodejs", "step2"),
				Additive: true,
			},
		},
	}
	integration.ProgramTest(t, opts)
}

// Test remote component construction in Node.
func TestConstructNode(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows")
	}
	t.Parallel()

	testDir := "construct_component"
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
				optsForConstructNode(t, test.expectedResourceCount, localProviders))
		})
	}
}

func optsForConstructNode(
	t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency,
) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Dir:            filepath.Join("construct_component", "nodejs"),
		Dependencies:   []string{"@pulumi/pulumi"},
		LocalProviders: localProviders,
		Secrets: map[string]string{
			"secret": "this super secret is encrypted",
		},
		Quick: true,
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

					urns[res.URN.Name()] = res.URN
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

func TestConstructComponentConfigureProviderNode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows")
	}

	// NOTE: this test can be quite awkward about dependencies. Specifically, the component, the
	// component's node SDK, and the test program need to agree on the version of pulumi-tls
	// dependency, and when there are discrepancies two versions of pulumi-tls may be installed
	// at the same time breaking assumptions. This is currently achieved as follows:
	//
	// ${componentSDK} has a direct node "x.y.z" reference not a floating one "^x.y.z".
	// ${testDir}/nodejs/package.json has the a direct reference also

	const testDir = "construct_component_configure_provider"
	runComponentSetup(t, testDir)
	pulumiRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	componentSDK := filepath.Join(pulumiRoot, "pkg/codegen/testing/test/testdata/methods-return-plain-resource/nodejs")

	// The test relies on artifacts (Node package) from a codegen test. Ensure the SDK is generated.
	cmd := exec.Command("go", "test", "-test.v", "-run", "TestGeneratePackage/methods-return-plain-resource")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = filepath.Join(pulumiRoot, "pkg", "codegen", "nodejs")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PULUMI_ACCEPT=1")
	err = cmd.Run()
	require.NoErrorf(t, err, "Failed to ensure that methods-return-plain-resource codegen"+
		" test has generated the Node SDK:\n%s\n%s\n",
		stdout.String(), stderr.String())

	t.Logf("yarn run tsc # precompile @pulumi/metaprovider")
	cmd2 := exec.Command("yarn", "run", "tsc")
	cmd2.Dir = filepath.Join(componentSDK)
	err = cmd2.Run()
	require.NoError(t, err)

	t.Logf("yarn link # prelink @pulumi/metaprovider")
	cmd3 := exec.Command("yarn", "link")
	cmd3.Dir = filepath.Join(componentSDK, "bin")
	err = cmd3.Run()
	require.NoError(t, err)

	opts := testConstructComponentConfigureProviderCommonOptions()
	opts = opts.With(integration.ProgramTestOptions{
		NoParallel: true,
		Dir:        filepath.Join(testDir, "nodejs"),
		Dependencies: []string{
			"@pulumi/pulumi",
			"@pulumi/metaprovider",
		},
	})
	integration.ProgramTest(t, &opts)
}
