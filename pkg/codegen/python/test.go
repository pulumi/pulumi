package python

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

func Check(t *testing.T, path string, _ codegen.StringSet) {
	ex, _, err := python.CommandPath()
	require.NoError(t, err)
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	err = integration.RunCommand(t, "python syntax check",
		[]string{ex, "-m", "py_compile", name}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
}
