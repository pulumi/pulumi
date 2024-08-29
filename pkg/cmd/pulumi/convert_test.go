package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYamlConvert is an entrypoint for debugging `pulumi convert“. To use this with an editor such as
// VS Code, drop a Pulumi.yaml in the convert_testdata folder and with the VS Code Go extension, the
// code lens (grayed out text above TestConvert) should display an option to "debug test".
//
// This is ideal for debugging panics in the convert command, as the debugger will break on the
// panic.
//
// See: https://github.com/golang/vscode-go/wiki/debugging
//
// Your mileage may vary with other tooling.
func TestYamlConvert(t *testing.T) {
	t.Parallel()

	if info, err := os.Stat("convert_testdata/Pulumi.yaml"); err != nil && os.IsNotExist(err) {
		t.Skip("skipping test, no Pulumi.yaml found")
	} else if err != nil {
		t.Fatalf("failed to stat Pulumi.yaml: %v", err)
	} else if info.IsDir() {
		t.Fatalf("Pulumi.yaml is a directory, not a file")
	}

	result := runConvert(
		pkgWorkspace.Instance, env.Global(), []string{}, "convert_testdata", []string{},
		"yaml", "go", "convert_testdata/go", true, true, "")
	require.Nil(t, result, "convert failed: %v", result)
}

func TestPclConvert(t *testing.T) {
	t.Parallel()

	// Check that we can run convert from PCL to PCL
	tmp := t.TempDir()

	result := runConvert(
		pkgWorkspace.Instance, env.Global(), []string{}, "pcl_convert_testdata",
		[]string{}, "pcl", "pcl", tmp, true, true, "")
	assert.Nil(t, result)

	// Check that we made one file
	pclBytes, err := os.ReadFile(filepath.Join(tmp, "main.pp"))
	assert.NoError(t, err)
	// On Windows, we need to replace \r\n with \n to match the expected string below
	pclCode := string(pclBytes)
	if runtime.GOOS == "windows" {
		pclCode = strings.ReplaceAll(pclCode, "\r\n", "\n")
	}
	expectedPclCode := `key = readFile("key.pub")

output result {
    __logicalName = "result"
    value = key
}`
	assert.Equal(t, expectedPclCode, pclCode)
}

// Tests that project names default to the directory of the source project.
func TestProjectNameDefaults(t *testing.T) {
	t.Parallel()

	// Arrange.
	outDir := t.TempDir()

	// Act.
	err := runConvert(
		pkgWorkspace.Instance,
		env.Global(),
		[]string{},             /*args*/
		"pcl_convert_testdata", /*cwd*/
		[]string{},             /*mappings*/
		"pcl",                  /*from*/
		"yaml",                 /*language*/
		outDir,
		true, /*generateOnly*/
		true, /*strict*/
		"",   /*name*/
	)
	assert.NoError(t, err)

	// Assert.
	yamlBytes, err := os.ReadFile(filepath.Join(outDir, "Pulumi.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(yamlBytes), "name: pcl_convert_testdata")
}

// Tests that project names can be overridden by the user.
func TestProjectNameOverrides(t *testing.T) {
	t.Parallel()

	// Arrange.
	outDir := t.TempDir()
	name := "test-project-name"

	// Act.
	err := runConvert(
		pkgWorkspace.Instance,
		env.Global(),
		[]string{},             /*args*/
		"pcl_convert_testdata", /*cwd*/
		[]string{},             /*mappings*/
		"pcl",                  /*from*/
		"yaml",                 /*language*/
		outDir,
		true, /*generateOnly*/
		true, /*strict*/
		name,
	)
	assert.NoError(t, err)

	// Assert.
	yamlBytes, err := os.ReadFile(filepath.Join(outDir, "Pulumi.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(yamlBytes), "name: "+name)
}
