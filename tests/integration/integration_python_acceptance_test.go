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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

func boolPointer(b bool) *bool {
	return &b
}

// TestEmptyPython simply tests that we can run an empty Python project.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEmptyPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		Quick: true,
	})
}

// Tests dynamic provider in Python.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDynamicPython(t *testing.T) {
	var randomVal string
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("dynamic", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			randomVal = stack.Outputs["random_val"].(string)
		},
		EditDirs: []integration.EditDir{{
			Dir:      filepath.Join("dynamic", "python", "step1"),
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

				// Ensure there are no diagnostic events other than debug.
				for _, event := range stack.Events {
					if event.DiagnosticEvent != nil {
						assert.Equal(t, "debug", event.DiagnosticEvent.Severity,
							"unexpected diagnostic event: %#v", event.DiagnosticEvent)
					}
				}
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

	//nolint:paralleltest // ProgramTest calls t.Parallel()
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

func optsForConstructPython(
	t *testing.T, expectedResourceCount int, localProviders []integration.LocalDependency, env ...string,
) *integration.ProgramTestOptions {
	return &integration.ProgramTestOptions{
		Env: env,
		Dir: filepath.Join("construct_component", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
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

//nolint:paralleltest // Sets env vars
func TestConstructComponentConfigureProviderPython(t *testing.T) {
	// This uses the tls plugin so needs to be able to download it
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	const testDir = "construct_component_configure_provider"
	runComponentSetup(t, testDir)
	pulumiRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	pulumiPySDK := filepath.Join("..", "..", "sdk", "python")
	componentSDK := filepath.Join(pulumiRoot, "pkg/codegen/testing/test/testdata/methods-return-plain-resource/python")
	opts := testConstructComponentConfigureProviderCommonOptions()
	opts = opts.With(integration.ProgramTestOptions{
		Dir:          filepath.Join(testDir, "python"),
		Dependencies: []string{pulumiPySDK, componentSDK},
		NoParallel:   true,
	})
	integration.ProgramTest(t, &opts)
}

// Regresses https://github.com/pulumi/pulumi/issues/6471
func TestAutomaticVenvCreation(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")
	t.Parallel()

	// Do not use integration.ProgramTest to avoid automatic venv
	// handling by test harness; we actually are testing venv
	// handling by the pulumi CLI itself.

	check := func(t *testing.T, venvPathTemplate string, dir string) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

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
			"virtualenv: >-\n      "+venvPath))

		if err := os.WriteFile(pulumiYaml, newYaml, 0o600); err != nil {
			t.Error(err)
			return
		}

		t.Logf("Wrote Pulumi.yaml:\n%s\n", string(newYaml))

		// Make a subdir and change to it to ensure paths aren't just relative to the working directory.
		subdir := filepath.Join(e.RootPath, "subdir")
		err = os.Mkdir(subdir, 0o755)
		require.NoError(t, err)
		e.CWD = subdir

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "teststack")
		e.RunCommand("pulumi", "preview")

		var absVenvPath string
		if filepath.IsAbs(venvPath) {
			absVenvPath = venvPath
		} else {
			absVenvPath = filepath.Join(e.RootPath, venvPath)
		}

		if !toolchain.IsVirtualEnv(absVenvPath) {
			t.Errorf("Expected a virtual environment to be created at %s but it is not there",
				absVenvPath)
		}
	}

	t.Run("RelativePath", func(t *testing.T) {
		t.Parallel()
		check(t, "venv", filepath.Join("python", "venv"))
	})

	t.Run("AbsolutePath", func(t *testing.T) {
		t.Parallel()
		check(t, filepath.Join("${root}", "absvenv"), filepath.Join("python", "venv"))
	})

	t.Run("RelativePathWithMain", func(t *testing.T) {
		t.Parallel()
		check(t, "venv", filepath.Join("python", "venv-with-main"))
	})

	t.Run("AbsolutePathWithMain", func(t *testing.T) {
		t.Parallel()
		check(t, filepath.Join("${root}", "absvenv"), filepath.Join("python", "venv-with-main"))
	})

	t.Run("TestInitVirtualEnvBeforePythonVersionCheck", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

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

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestAutomaticVenvCreationPoetry(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")

	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory(filepath.Join("python", "poetry"))

	// Make a subdir and change to it to ensure paths aren't just relative to the working directory.
	subdir := filepath.Join(e.RootPath, "subdir")
	err := os.Mkdir(subdir, 0o755)
	require.NoError(t, err)
	e.CWD = subdir

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "teststack")
	e.RunCommand("pulumi", "preview")

	localPoetryVenv := filepath.Join(e.RootPath, ".venv")
	if !toolchain.IsVirtualEnv(localPoetryVenv) {
		t.Errorf("Expected a virtual environment to be created at %s but it is not there", localPoetryVenv)
	}
}

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestPoetryInstallParentDirectory(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory(filepath.Join("python", "poetry-parent"))
	// Run from the subdir with the Pulumi.yaml file
	e.CWD = filepath.Join(e.RootPath, "subfolder")

	e.RunCommand("pulumi", "install")

	localPoetryVenv := filepath.Join(e.RootPath, ".venv")
	if !toolchain.IsVirtualEnv(localPoetryVenv) {
		t.Errorf("Expected a virtual environment to be created at %s but it is not there", localPoetryVenv)
	}
}

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestPoetryInstallWithMain(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory(filepath.Join("python", "poetry-main"))

	e.RunCommand("pulumi", "install")

	localPoetryVenv := filepath.Join(e.RootPath, ".venv")
	if !toolchain.IsVirtualEnv(localPoetryVenv) {
		t.Errorf("Expected a virtual environment to be created at %s but it is not there", localPoetryVenv)
	}
}

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestPoetryInstallWithMainAndParent(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory(filepath.Join("python", "poetry-main-and-parent"))

	e.RunCommand("pulumi", "install")

	localPoetryVenv := filepath.Join(e.RootPath, "src", ".venv")
	if !toolchain.IsVirtualEnv(localPoetryVenv) {
		t.Errorf("Expected a virtual environment to be created at %s but it is not there", localPoetryVenv)
	}
}

func TestUv(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")

	t.Parallel()

	for _, test := range []struct {
		template     string
		cwd          string
		expectedVenv string
	}{
		{
			template:     "uv",
			expectedVenv: "my-venv",
		},
		{
			template:     "uv-main",
			expectedVenv: "my-venv",
		},
		{
			template:     "uv-parent",
			cwd:          "subfolder",
			expectedVenv: "subfolder/my-venv",
		},
		{
			template:     "uv-no-venv-option",
			expectedVenv: ".venv",
		},
		{
			template:     "uv-no-venv-option-parent",
			cwd:          "subfolder",
			expectedVenv: ".venv", // The virtualenv is relative to pyproject.toml
		},
	} {
		test := test
		// On windows, when running in parallel, we can run into issues when Uv tries
		// to write the same cache file concurrently. This is the same issue we see
		// for Poetry https://github.com/pulumi/pulumi/pull/17337
		//nolint:paralleltest
		t.Run(test.template, func(t *testing.T) {
			if runtime.GOOS != "windows" {
				t.Parallel()
			}
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			e.ImportDirectory(filepath.Join("python", test.template))

			if test.cwd != "" {
				e.CWD = filepath.Join(e.RootPath, test.cwd)
			}

			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", ptesting.RandomStackName())
			e.RunCommand("pulumi", "preview")

			venv := filepath.Join(e.RootPath, test.expectedVenv)
			if !toolchain.IsVirtualEnv(venv) {
				t.Errorf("Expected a virtual environment to be created at %s but it is not there", venv)
			}
		})
	}

	t.Run("convert requirements.txt", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Parallel()
		}
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		e.ImportDirectory(filepath.Join("python", "uv-convert-requirements"))

		e.RunCommand("pulumi", "install")

		venv := filepath.Join(e.RootPath, "my-venv")
		if !toolchain.IsVirtualEnv(venv) {
			t.Errorf("Expected a virtual environment to be created at %s but it is not there", venv)
		}

		require.True(t, e.PathExists("pyproject.toml"), "pyproject.toml should have been created")
		require.False(t, e.PathExists("requirements.txt"), "requirements.txt should have been deleted")
	})

	t.Run("convert requirements.txt subfolder", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Parallel()
		}
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		e.ImportDirectory(filepath.Join("python", "uv-convert-requirements-subfolder"))
		e.CWD = filepath.Join(e.RootPath, "subfolder")

		e.RunCommand("pulumi", "install")

		venv := filepath.Join(e.RootPath, "subfolder", "my-venv")
		if !toolchain.IsVirtualEnv(venv) {
			t.Errorf("Expected a virtual environment to be created at %s but it is not there", venv)
		}

		e.CWD = e.RootPath // Reset the CWD so we can use e.PathExists
		require.True(t, e.PathExists("pyproject.toml"), "pyproject.toml should have been created")
		require.False(t, e.PathExists("requirements.txt"), "requirements.txt should have been deleted")
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestMypySupport(t *testing.T) {
	validation := func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
		// Should get an event for the mypy failure.
		messages := []string{}
		for _, event := range stack.Events {
			if event.DiagnosticEvent != nil {
				messages = append(messages,
					strings.Replace(event.DiagnosticEvent.Message, "\r\n", "\n", -1))
			}
		}
		expected := "__main__.py:8: error: " +
			"Argument 1 to \"export\" has incompatible type \"int\"; expected \"str\"" +
			"  [arg-type]\n\n"
		assert.Contains(t, messages, expected, "Did not find expected mypy diagnostic event")
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "mypy"),
		// mypy doesn't really support editable packages, so we install pulumi normally from pip for this test.
		Quick:                  true,
		ExpectFailure:          true,
		ExtraRuntimeValidation: validation,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPyrightSupport(t *testing.T) {
	validation := func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
		// Should get an event for the pyright failure.
		found := false
		expected := "__main__.py:8:15 - error:" +
			" Argument of type \"Literal[42]\" cannot be assigned to parameter \"name\"" +
			" of type \"str\" in function \"export\"\n\n"
		for _, event := range stack.Events {
			if event.DiagnosticEvent != nil {
				message := strings.Replace(event.DiagnosticEvent.Message, "\r\n", "\n", -1)
				if strings.HasSuffix(message, expected) {
					found = true
				}
			}
		}
		assert.True(t, found, "Did not find expected pyright diagnostic event")
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "pyright"),
		// to match the mypy test we install pulumi normally from pip for this test.
		Quick:                  true,
		ExpectFailure:          true,
		ExtraRuntimeValidation: validation,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestTypecheckerMissingError(t *testing.T) {
	validation := func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
		// Should get an event for the pyright failure.
		found := false
		expected := "The typechecker option is set to pyright, but pyright is not installed." +
			" Please add an entry for pyright to requirements.txt"
		for _, event := range stack.Events {
			if event.DiagnosticEvent != nil {
				if strings.Contains(event.DiagnosticEvent.Message, expected) {
					found = true
				}
			}
		}
		assert.True(t, found, "Did not find expected pyright diagnostic event")
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    filepath.Join("python", "pyright-missing"),
		Quick:                  true,
		ExpectFailure:          true,
		ExtraRuntimeValidation: validation,
	})
}

func TestNewPythonUsesPip(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	stdout, _ := e.RunCommand("pulumi", "new", "python", "--force", "--non-interactive", "--yes", "--generate-only")

	require.Contains(t, stdout, "pulumi install")

	expected := map[string]interface{}{
		"toolchain":  "pip",
		"virtualenv": "venv",
	}
	integration.CheckRuntimeOptions(t, e.RootPath, expected)
}

//nolint:paralleltest // Modifies env
func TestNewPythonUsesPipNonInteractive(t *testing.T) {
	// Force interactive mode to properly test `--yes`.
	t.Setenv("PULUMI_TEST_INTERACTIVE", "1")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	stdout, _ := e.RunCommand("pulumi", "new", "python", "--force", "--yes", "--generate-only")

	require.Contains(t, stdout, "pulumi install")

	expected := map[string]interface{}{
		"toolchain":  "pip",
		"virtualenv": "venv",
	}
	integration.CheckRuntimeOptions(t, e.RootPath, expected)
}

//nolint:paralleltest // Modifies env
func TestNewPythonChoosePoetry(t *testing.T) {
	// The windows acceptance tests are run using git bash, but the survey library does not support this
	// https://github.com/AlecAivazis/survey/issues/148
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: survey library does not support git bash on Windows")
	}

	t.Setenv("PULUMI_TEST_INTERACTIVE", "1")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.Stdin = strings.NewReader("poetry\n")
	e.RunCommand("pulumi", "new", "python", "--force", "--generate-only",
		"--name", "test_project",
		"--description", "A python test using poetry as toolchain",
		"--stack", "test",
	)

	expected := map[string]interface{}{
		"toolchain": "poetry",
	}
	integration.CheckRuntimeOptions(t, e.RootPath, expected)
}

//nolint:paralleltest // Modifies env
func TestNewPythonChooseUv(t *testing.T) {
	// The windows acceptance tests are run using git bash, but the survey library does not support this
	// https://github.com/AlecAivazis/survey/issues/148
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: survey library does not support git bash on Windows")
	}

	t.Setenv("PULUMI_TEST_INTERACTIVE", "1")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.Stdin = strings.NewReader("uv\n")
	e.RunCommand("pulumi", "new", "python", "--force", "--generate-only",
		"--name", "test_project",
		"--description", "A python test using uv as toolchain",
	)

	expected := map[string]interface{}{
		"toolchain": "uv",
	}
	integration.CheckRuntimeOptions(t, e.RootPath, expected)

	e.RunCommand("pulumi", "install")

	require.True(t, e.PathExists(".venv"))
	require.True(t, e.PathExists("uv.lock"))
	require.True(t, e.PathExists("pyproject.toml"))
	require.False(t, e.PathExists("requirements.txt"))
}

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestNewPythonRuntimeOptions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "new", "python", "--force", "--non-interactive", "--yes", "--generate-only",
		"--name", "test_project",
		"--description", "A python test using poetry as toolchain",
		"--stack", "test",
		"--runtime-options", "toolchain=pip,virtualenv=mytestenv",
	)

	expected := map[string]interface{}{
		"toolchain":  "pip",
		"virtualenv": "mytestenv",
	}
	integration.CheckRuntimeOptions(t, e.RootPath, expected)
}

//nolint:paralleltest // Poetry causes issues when run in parallel on windows. See pulumi/pulumi#17183
func TestNewPythonConvertRequirementsTxt(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Add a poetry.toml to make poetry create the virtualenv inside the temp
	// directory. That way it gets cleaned up with the test.
	poetryToml := `[virtualenvs]
in-project = true`
	err := os.WriteFile(filepath.Join(e.RootPath, "poetry.toml"), []byte(poetryToml), 0o600)
	require.NoError(t, err)

	template := "python"

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	out, _ := e.RunCommand("pulumi", "new", template, "--force", "--non-interactive", "--yes",
		"--name", "test_project",
		"--description", "A python test using poetry as toolchain",
		"--stack", "test",
		"--runtime-options", "toolchain=poetry",
	)

	require.Contains(t, out, "Deleted requirements.txt")
	require.True(t, e.PathExists("pyproject.toml"), "pyproject.toml was created")
	require.False(t, e.PathExists("requirements.txt"), "requirements.txt was removed")

	b, err := os.ReadFile(filepath.Join(e.RootPath, "pyproject.toml"))
	require.NoError(t, err)
	require.Equal(t, `[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"

[tool]
[tool.poetry]
package-mode = false
[tool.poetry.dependencies]
pulumi = ">=3.0.0,<4.0.0"
python = "^3.9"
`, string(b))
}

// Regression test for https://github.com/pulumi/pulumi/issues/17877
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestUvWindowsError(t *testing.T) {
	t.Skip("installs release version of pulumi -> no coverage")

	if runtime.GOOS != "windows" {
		t.Parallel()
	}
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// This test will hang on Windows without the fix for https://github.com/pulumi/pulumi/issues/17877
	done := make(chan struct{})
	var stdout string
	go func() {
		e.ImportDirectory(filepath.Join("python", "uv-with-error"))
		e.RunCommand("pulumi", "install")
		e.RunCommand("pulumi", "plugin", "install", "resource", "random")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", ptesting.RandomStackName())
		stdout, _ = e.RunCommandExpectError("pulumi", "preview")

		done <- struct{}{}
	}()
	select {
	case <-done:
		require.Contains(t, stdout, "Duplicate resource URN")
	case <-time.After(3 * time.Minute):
		t.Fatal("Timed out waiting for TestUvWindowsError")
	}
}
