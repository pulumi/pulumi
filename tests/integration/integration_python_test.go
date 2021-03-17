// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build python all

package ints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

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

func TestStackOutputsPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_outputs", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
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

// Tests basic configuration from the perspective of a Pulumi program.
func TestConfigBasicPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_basic", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a Pythonic value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super Pythonic secret is encrypted",
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

func TestStackReferencePython(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_reference", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
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

func TestMultiStackReferencePython(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	// build a stack with an export
	exporterOpts := &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_reference_multi", "python", "exporter"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"org": os.Getenv("PULUMI_TEST_OWNER"),
		},
		NoParallel: true,
	}

	// we're going to manually initialize and then defer the deletion of this stack
	exporterPt := integration.ProgramTestManualLifeCycle(t, exporterOpts)
	exporterPt.TestFinished = false
	err := exporterPt.TestLifeCyclePrepare()
	assert.NoError(t, err)
	err = exporterPt.TestLifeCycleInitialize()
	assert.NoError(t, err)

	defer func() {
		destroyErr := exporterPt.TestLifeCycleDestroy()
		assert.NoError(t, destroyErr)
		exporterPt.TestFinished = true
		exporterPt.TestCleanUp()
	}()

	err = exporterPt.TestPreviewUpdateAndEdits()
	assert.NoError(t, err)

	exporterStackName := exporterOpts.GetStackName().String()

	importerOpts := &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_reference_multi", "python", "importer"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Config: map[string]string{
			"org":                 os.Getenv("PULUMI_TEST_OWNER"),
			"exporter_stack_name": exporterStackName,
		},
		NoParallel: true,
	}
	integration.ProgramTest(t, importerOpts)
}

func TestResourceWithSecretSerializationPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("secret_outputs", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// The program exports three resources:
			//   1. One named `withSecret` who's prefix property should be secret, specified via `pulumi.secret()`.
			//   2. One named `withSecretAdditional` who's prefix property should be a secret, specified via
			//      additionalSecretOutputs.
			//   3. One named `withoutSecret` which should not be a secret.
			// We serialize both of the these as plain old objects, so they appear as maps in the output.
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

// Tests that we issue an error if we fail to locate the Python command when running
// a Python example.
func TestPython3NotInstalled(t *testing.T) {
	t.Skip("Temporarily skipping failing test - pulumi/pulumi#6304")
	stderr := &bytes.Buffer{}
	badPython := "python3000"
	expectedError := fmt.Sprintf(
		"Failed to locate any of %q on your PATH.  Have you installed Python 3.6 or greater?",
		[]string{badPython})
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		Env: []string{
			// Note: we use PULUMI_PYTHON_CMD to override the default behavior of searching
			// for Python 3, since anyone running tests surely already has Python 3 installed on their
			// machine. The code paths are functionally the same.
			fmt.Sprintf("PULUMI_PYTHON_CMD=%s", badPython),
		},
		ExpectFailure: true,
		Stderr:        stderr,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			output := stderr.String()
			assert.Contains(t, output, expectedError)
		},
	})
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
			},
		}},
	})
}

func TestPartialValuesPython(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("partial_values", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		AllowEmptyPreviewChanges: true,
	})
}

// Tests a resource with a large (>4mb) string prop in Python
func TestLargeResourcePython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("large_resource", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
	})
}

// Test enum outputs
func TestEnumOutputsPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("enums", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)

			assert.Equal(t, "Burgundy", stack.Outputs["myTreeType"])
			assert.Equal(t, "Pulumi Planters Inc.foo", stack.Outputs["myTreeFarmChanged"])
			assert.Equal(t, "My Burgundy Rubber tree is from Pulumi Planters Inc.", stack.Outputs["mySentence"])
		},
	})
}

// Test to ensure Pylint is clean.
func TestPythonPylint(t *testing.T) {
	t.Skip("Temporarily skipping test - pulumi/pulumi#4849")
	var opts *integration.ProgramTestOptions
	opts = &integration.ProgramTestOptions{
		Dir: filepath.Join("python", "pylint"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			randomURN := stack.Outputs["random_urn"].(string)
			assert.NotEmpty(t, randomURN)

			randomID := stack.Outputs["random_id"].(string)
			randomVal := stack.Outputs["random_val"].(string)
			assert.Equal(t, randomID, randomVal)

			cwd := stack.Outputs["cwd"].(string)
			assert.NotEmpty(t, cwd)

			pylint := filepath.Join("venv", "bin", "pylint")
			if runtime.GOOS == WindowsOS {
				pylint = filepath.Join("venv", "Scripts", "pylint")
			}

			err := integration.RunCommand(t, "pylint", []string{pylint, "__main__.py"}, cwd, opts)
			assert.NoError(t, err)
		},
		Quick: true,
	}
	integration.ProgramTest(t, opts)
}

// Test Python SDK codegen to ensure <Resource>Args and traditional keyword args work.
func TestPythonResourceArgs(t *testing.T) {
	testdir := filepath.Join("python", "resource_args")

	// Generate example library from schema.
	schemaBytes, err := os.ReadFile(filepath.Join(testdir, "schema.json"))
	assert.NoError(t, err)
	var spec schema.PackageSpec
	assert.NoError(t, json.Unmarshal(schemaBytes, &spec))
	pkg, err := schema.ImportSpec(spec, nil)
	assert.NoError(t, err)
	files, err := pygen.GeneratePackage("test", pkg, map[string][]byte{})
	assert.NoError(t, err)
	outdir := filepath.Join(testdir, "lib")
	assert.NoError(t, os.RemoveAll(outdir))
	for f, contents := range files {
		outfile := filepath.Join(outdir, f)
		assert.NoError(t, os.MkdirAll(filepath.Dir(outfile), 0755))
		if outfile == filepath.Join(outdir, "setup.py") {
			contents = []byte(strings.ReplaceAll(string(contents), "${VERSION}", "0.0.1"))
		}
		assert.NoError(t, os.WriteFile(outfile, contents, 0600))
	}
	assert.NoError(t, os.WriteFile(filepath.Join(outdir, "README.md"), []byte(""), 0600))

	// Test the program.
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: testdir,
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
			filepath.Join(testdir, "lib"),
		},
		Quick: true,
	})
}

// Test remote component construction in Python.
func TestConstructPython(t *testing.T) {
	pathEnv, err := testComponentPathEnv()
	if err != nil {
		t.Fatalf("failed to build test component PATH: %v", err)
	}

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the Python program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	var opts *integration.ProgramTestOptions
	opts = &integration.ProgramTestOptions{
		Env: []string{pathEnv, testYarnLinkPulumiEnv},
		Dir: filepath.Join("construct_component", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 9, len(stackInfo.Deployment.Resources)) {
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
						assert.Equal(t, []resource.URN{urns["a"]}, res.PropertyDependencies["echo"])
					case "child-c":
						assert.ElementsMatch(t, []resource.URN{urns["child-a"], urns["a"]},
							res.PropertyDependencies["echo"])
					}
				}
			}
		},
	}
	integration.ProgramTest(t, opts)
}

func TestGetResourcePython(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("get_resource", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python", "env", "src"),
		},
		AllowEmptyPreviewChanges: true,
	})
}

// Regresses https://github.com/pulumi/pulumi/issues/6471
func TestAutomaticVenvCreation(t *testing.T) {
	// Do not use integration.ProgramTest to avoid automatic venv
	// handling by test harness; we actually are testing venv
	// handling by the pulumi CLI itself.

	check := func(t *testing.T, venvPathTemplate string) {

		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		venvPath := strings.ReplaceAll(venvPathTemplate, "${root}", e.RootPath)
		t.Logf("venvPath = %s (IsAbs = %v)", venvPath, filepath.IsAbs(venvPath))

		e.ImportDirectory(filepath.Join("python", "venv"))

		// replace "virtualenv: venv" with "virtualenv: ${venvPath}" in Pulumi.yaml
		pulumiYaml := filepath.Join(e.RootPath, "Pulumi.yaml")

		oldYaml, err := ioutil.ReadFile(pulumiYaml)
		if err != nil {
			t.Error(err)
			return
		}
		newYaml := []byte(strings.ReplaceAll(string(oldYaml),
			"virtualenv: venv",
			fmt.Sprintf("virtualenv: >-\n      %s", venvPath)))
		if err := ioutil.WriteFile(pulumiYaml, newYaml, 0644); err != nil {
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
		check(t, "venv")
	})

	t.Run("AbsolutePath", func(t *testing.T) {
		check(t, filepath.Join("${root}", "absvenv"))
	})
}

func TestPythonAwaitOutputs(t *testing.T) {
	t.Run("SuccessSimple", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "success"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicStringMessage := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "magic string") {
							sawMagicStringMessage = true
						}
					}
				}
				assert.True(t, sawMagicStringMessage, "Did not see printed message from unexported output")
			},
		})
	})

	t.Run("SuccessMultipleOutputs", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "multiple_outputs"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicString := false
				sawFoo := false
				sawBar := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "magic string") {
							sawMagicString = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "bar") {
							sawBar = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				msg := "Did not see printed message from unexported output"
				assert.True(t, sawMagicString, msg)
				assert.True(t, sawFoo, msg)
				assert.True(t, sawBar, msg)
			},
		})
	})

	t.Run("CreateWithinApply", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "create_inside_apply"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawUrn := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "pulumi-python:dynamic:Resource::magic_string") {
							sawUrn = true
						}
					}
				}
				assert.True(t, sawUrn)
			},
		})
	})

	t.Run("ErrorHandlingSuccess", func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "error_handling"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			Quick:                    true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				sawMagicStringMessage := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "oh yeah") {
							sawMagicStringMessage = true
						}
					}
				}
				assert.True(t, sawMagicStringMessage, "Did not see printed message from unexported output")
			},
		})
	})

	t.Run("FailureSimple", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
			},
		})
	})

	t.Run("FailureWithExportedOutput", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure_exported_output"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
				sawFoo := false
				sawPrinted := false
				sawNotPrinted := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "not printed") {
							sawNotPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "printed") {
							sawPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				assert.True(t, sawPrinted)
				assert.True(t, sawFoo)
				assert.False(t, sawNotPrinted)
			},
		})
	})

	t.Run("FailureMultipleOutputs", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		expectedError := "IndexError: list index out of range"
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join("python_await", "failure_multiple_unexported_outputs"),
			Dependencies: []string{
				filepath.Join("..", "..", "sdk", "python", "env", "src"),
			},
			AllowEmptyPreviewChanges: true,
			ExpectFailure:            true,
			Quick:                    true,
			Stderr:                   stderr,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				output := stderr.String()
				assert.Contains(t, output, expectedError)
				sawFoo := false
				sawPrinted := false
				sawNotPrinted := false
				for _, evt := range stack.Events {
					if evt.DiagnosticEvent != nil {
						if strings.Contains(evt.DiagnosticEvent.Message, "not printed") {
							sawNotPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "printed") {
							sawPrinted = true
						}
						if strings.Contains(evt.DiagnosticEvent.Message, "foo") {
							sawFoo = true
						}
					}
				}
				assert.True(t, sawPrinted)
				assert.True(t, sawFoo)
				assert.False(t, sawNotPrinted)
			},
		})
	})
}
