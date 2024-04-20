package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func CheckGo(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	// We remove go.mod to ensure tests are reproducible.
	goMod := filepath.Join(dir, "go.mod")
	if err = os.Remove(goMod); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "generate go.mod",
		[]string{ex, "mod", "init", "main"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "go tidy",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	if pulumiSDKPath != "" {
		err = integration.RunCommand(t, "point towards local Go SDK",
			[]string{
				ex, "mod", "edit",
				fmt.Sprintf("--replace=%s=%s",
					"github.com/pulumi/pulumi/sdk/v3",
					pulumiSDKPath),
			},
			dir, &integration.ProgramTestOptions{})
		require.NoError(t, err)
	}
	TypeCheckGo(t, path, deps, pulumiSDKPath)
}

func TypeCheckGo(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	err = integration.RunCommand(t, "go tidy after replace",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)

	err = integration.RunCommand(t, "test build", []string{ex, "build", "-v", "all"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	os.Remove(filepath.Join(dir, "main"))
	assert.NoError(t, err)
}

func generateGoBatchTest(t *testing.T, generator GenProgram, testCases []ProgramTest) {
	pulumiSdkPath, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk"))
	require.NoError(t, err)
	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				CheckGo(t, path, dependencies, pulumiSdkPath)
			},
			GenProgram: generator,
			TestCases:  testCases,
		})
}
