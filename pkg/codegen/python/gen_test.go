// Copyright 2016-2021, Pulumi Corporation.
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

package python

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

const venvRelDir = "venv"

var pathTests = []struct {
	input    string
	expected string
}{
	{".", "."},
	{"", "."},
	{"../", ".."},
	{"../..", "..."},
	{"../../..", "...."},
	{"something", ".something"},
	{"../parent", "..parent"},
	{"../../module", "...module"},
}

func TestRelPathToRelImport(t *testing.T) {
	t.Parallel()

	for _, tt := range pathTests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := relPathToRelImport(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}

func TestGeneratePackage(t *testing.T) {
	t.Parallel()

	var virtualEnvLock sync.Mutex
	// If we are running without checks, we mark the env as already built so we don't
	// build it again.
	virtualEnvBuilt := test.NoSDKCodegenChecks()

	// To speed up these tests, we will generate one common virtual environment for all of
	// them to run in, rather than having one per test. We want to make sure that we only
	// build the virtual env if we are going to run one of the tests. We thus build the
	// environment lazily
	needsEnv := func(testFn test.CodegenCheck) test.CodegenCheck {
		return func(t *testing.T, codedir string) {
			func() {
				virtualEnvLock.Lock()
				defer virtualEnvLock.Unlock()
				if !virtualEnvBuilt {
					err := buildVirtualEnv(context.Background())
					if err != nil {
						t.Error(err)
						t.FailNow()
					}
					virtualEnvBuilt = true
				}
			}()
			testFn(t, codedir)
		}
	}

	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "python",
		GenPackage: GeneratePackage,
		Checks: map[string]test.CodegenCheck{
			"python/py_compile": needsEnv(test.CompilePython),
			"python/test":       needsEnv(pyTestCheck),
		},
		TestCases: test.PulumiPulumiSDKTests,
	})
}

func absTestsPath() (string, error) {
	hereDir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	return hereDir, nil
}

func virtualEnvPath() (string, error) {
	hereDir, err := absTestsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(hereDir, venvRelDir), nil
}

// To serialize shared `venv` operations; without the lock running
// tests with `-parallel` causes sproadic failure.
var venvMutex = &sync.Mutex{}

func buildVirtualEnv(ctx context.Context) error {
	hereDir, err := absTestsPath()
	if err != nil {
		return err
	}
	venvDir, err := virtualEnvPath()
	if err != nil {
		return err
	}

	gotVenv, err := test.PathExists(venvDir)
	if err != nil {
		return err
	}

	if gotVenv {
		err := os.RemoveAll(venvDir)
		if err != nil {
			return err
		}
	}

	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain:  toolchain.Pip,
		Root:       hereDir,
		Virtualenv: venvRelDir,
	})
	if err != nil {
		return err
	}

	err = tc.InstallDependencies(ctx, hereDir, false, /*useLanguageVersionTools */
		false /*showOutput*/, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python"))
	if err != nil {
		return err
	}

	gotSdk, err := test.PathExists(sdkDir)
	if err != nil {
		return err
	}

	if !gotSdk {
		return errors.New("This test requires Python SDK to be built; please `cd sdk/python && make ensure build install`")
	}

	// install Pulumi Python SDK from the current source tree, -e means no-copy, ref directly
	pyCmd, err := tc.ModuleCommand(ctx, "pip", "install", "-e", sdkDir)
	if err != nil {
		contract.Failf("failed to create pip install command: %v", err)
	}
	pyCmd.Dir = hereDir
	output, err := pyCmd.CombinedOutput()
	if err != nil {
		contract.Failf("failed to link venv against in-source pulumi: %v\nstdout/stderr:\n%s",
			err, output)
	}

	return nil
}

func pyTestCheck(t *testing.T, codeDir string) {
	extraDir := filepath.Join(filepath.Dir(codeDir), "python-extras")
	if _, err := os.Stat(extraDir); os.IsNotExist(err) {
		// We won't run any tests since no extra tests were included.
		return
	}
	hereDir, err := absTestsPath()
	if err != nil {
		t.Error(err)
		return
	}

	moduleCmd := func(module string, args ...string) error {
		t.Logf("cd %s && %s", codeDir, strings.Join(append([]string{module}, args...), " "))
		tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       hereDir,
			Virtualenv: venvRelDir,
		})
		if err != nil {
			return err
		}

		cmd, err := tc.ModuleCommand(context.Background(), module, args...)
		if err != nil {
			return err
		}
		cmd.Dir = codeDir

		outw := iotest.LogWriter(t)
		cmd.Stderr = outw
		cmd.Stdout = outw
		return cmd.Run()
	}

	installPackage := func() error {
		venvMutex.Lock()
		defer venvMutex.Unlock()
		return moduleCmd("pip", "install", "-e", ".")
	}

	if err = installPackage(); err != nil {
		t.Error(err)
		return
	}

	if err = moduleCmd("pytest", "."); err != nil {
		exitError, isExitError := err.(*exec.ExitError)
		if isExitError && exitError.ExitCode() == 5 {
			t.Logf("Could not find any pytest tests in %s", codeDir)
		} else {
			t.Error(err)
		}
		return
	}
}

func TestGenerateTypeNames(t *testing.T) {
	t.Parallel()

	test.TestTypeNameCodegen(t, "python", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		// Decode python-specific info
		err := pkg.ImportLanguages(map[string]schema.Language{"python": Importer})
		require.NoError(t, err)

		info, _ := pkg.Language["python"].(PackageInfo)

		modules, err := generateModuleContextMap("test", pkg, info, nil)
		require.NoError(t, err)

		root, ok := modules[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t, typeStringOpts{})
		}
	})
}

func TestEscapeDocString(t *testing.T) {
	t.Parallel()
	lines := []string{
		`Active directory email address. Example: xyz@contoso.com or Contoso\xyz`,
		`Triple quotes """ are all escaped`,
		`But just quotes " are not`,
		`This \N should be escaped`,
		`Here \\N slashes should be escaped but not N`,
	}
	source := strings.Join(lines, "\n")
	expected := `"""
Active directory email address. Example: xyz@contoso.com or Contoso\\xyz
Triple quotes \"\"\" are all escaped
But just quotes " are not
This \\N should be escaped
Here \\\\N slashes should be escaped but not N
"""
`
	w := &bytes.Buffer{}
	printComment(w, source, "")
	assert.Equal(t, expected, w.String())
}

// This test evaluates the calculateDeps function, which takes a list of
// dependencies, and generates a slice of order pairs, where the first
// item is the name of the dep and the second item is the version constraint.
func TestCalculateDeps(t *testing.T) {
	t.Parallel()
	type TestCase struct {
		// This is the input to the calculate deps function, a list of
		// deps provided in the schema.
		inputDeps map[string]string
		// This is the set of ordered pairs.
		expected [][2]string
		// calculateDeps can error if the Pulumi version provided is
		// invalid. This field is used to check that condition.
		expectedErr error
	}
	cases := []TestCase{{
		// Test 1: Give no explicit deps.
		inputDeps: map[string]string{},
		expected: [][2]string{
			// We expect three alphabetized deps,
			// with semver and parver formatted differently from Pulumi.
			// Pulumi should not have a version.
			{"parver>=0.2.1", ""},
			{"pulumi", ">=3.142.0,<4.0.0"},
			{"semver>=2.8.1"},
		},
	}, {
		// Test 2: If you only one dep, we expect Pulumi to have a narrower
		//         constraint than if you had provided no deps.
		inputDeps: map[string]string{
			"foobar": "7.10.8",
		},
		expected: [][2]string{
			{"foobar", "7.10.8"},
			{"parver>=0.2.1", ""},
			{"pulumi", ">=3.142.0,<4.0.0"},
			{"semver>=2.8.1"},
		},
	}, {
		// Test 3: If you provide pulumi, we expect the constraint to
		// be respected.
		inputDeps: map[string]string{
			"pulumi": ">=3.0.0,<3.50.0",
		},
		expected: [][2]string{
			// We expect three alphabetized deps,
			// with semver and parver formatted differently from Pulumi.
			{"parver>=0.2.1", ""},
			{"pulumi", ">=3.0.0,<3.50.0"},
			{"semver>=2.8.1"},
		},
	}, {
		// Test 4: If you provide an illegal pulumi version, we expect an error.
		inputDeps: map[string]string{
			"pulumi": ">=0.16.0,<4.0.0",
		},
		expectedErr: fmt.Errorf("lower version bound must be at least %v", oldestAllowedPulumi),
	}}

	for i, tc := range cases {
		tc := tc
		name := fmt.Sprintf("CalculateDeps #%d", i+1)
		t.Run(name, func(tt *testing.T) {
			tt.Parallel()
			observedDeps, err := calculateDeps(false, tc.inputDeps)
			assert.Equal(tt, tc.expectedErr, err)
			for index := range observedDeps {
				observedDep := observedDeps[index]
				expectedDep := tc.expected[index]
				assert.ElementsMatch(tt, expectedDep, observedDep)
			}
		})
	}
}

// This function tests that setPythonRequires correctly sets the minimum
// Python version when generating pyproject metadata.
func TestPythonRequiresSuccessful(t *testing.T) {
	t.Parallel()
	expected := "3.1"
	pkg := schema.Package{
		Language: map[string]interface{}{
			"python": PackageInfo{
				PythonRequires: expected,
			},
		},
	}
	schema := new(PyprojectSchema)
	schema.Project = new(Project)

	setPythonRequires(schema, &pkg)
	observed := *schema.Project.RequiresPython
	assert.Equal(t, expected, observed, "Expected version %s but observed version %s", expected, observed)
}

// This function tests that setPythonRequires correctly selects the default
// Python version when generating pyproject metadata.
func TestPythonRequiresNotProvided(t *testing.T) {
	t.Parallel()
	expected := defaultMinPythonVersion
	pkg := schema.Package{
		Language: map[string]interface{}{
			"python": PackageInfo{
				// Don't set PythonRequires
			},
		},
	}
	schema := new(PyprojectSchema)
	schema.Project = new(Project)

	setPythonRequires(schema, &pkg)
	observed := *schema.Project.RequiresPython
	assert.Equal(t, expected, observed, "Expected version %s but observed version %s", expected, observed)
}
