package tests

import (
	"os"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSchemaParameterized(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	e := ptesting.NewEnvironment(t)
	defer deleteIfNotFailed(e)
	e.CWD = cwd

	// First get the schema not parameterized
	stdout, _ := e.RunCommand("pulumi", "package", "get-schema", "./testprovider")
	assert.Contains(t, string(stdout), "\"name\": \"testprovider\"")

	// Then again with a parameter to change the package name
	stdout, _ = e.RunCommand("pulumi", "package", "get-schema", "./testprovider", "hello")
	assert.Contains(t, string(stdout), "\"name\": \"hello\"")

	// Run again and expect an error because of too many args
	_, stderr := e.RunCommandExpectError("pulumi", "package", "get-schema", "./testprovider", "hello", "world")
	assert.Contains(t, string(stderr), "unexpected args count, got 2, expected 1")
}
