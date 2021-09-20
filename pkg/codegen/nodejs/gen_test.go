// nolint: lll
package nodejs

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "nodejs", GeneratePackage, typeCheckGeneratedPackage)
}

func typeCheckGeneratedPackage(t *testing.T, pwd string) {
	var err error
	var npm string
	npm, err = executable.FindExecutable("npm")
	require.NoError(t, err)

	var stdout, stderr bytes.Buffer
	cmdOptions := integration.ProgramTestOptions{
		Verbose: true,
		Stderr:  &stderr,
		Stdout:  &stdout,
	}

	// TODO remove when https://github.com/pulumi/pulumi/pull/7938 lands
	file := filepath.Join(pwd, "package.json")
	oldFile, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	newFile := strings.ReplaceAll(string(oldFile), "${VERSION}", "0.0.1")
	err = ioutil.WriteFile(file, []byte(newFile), 0600)
	require.NoError(t, err)

	err = integration.RunCommand(t, "npm install", []string{npm, "i"}, pwd, &cmdOptions)
	require.NoError(t, err)

	// We increase the amount of memory node can use. We get OOM otherwise.
	nodeOptions := []string{os.Getenv("NODE_OPTIONS"), "--max_old_space_size=4096"}
	err = os.Setenv("NODE_OPTIONS", strings.Join(nodeOptions, " "))
	require.NoError(t, err)
	err = integration.RunCommand(t, "typecheck ts",
		[]string{filepath.Join(".", "node_modules", ".bin", "tsc"), "--noEmit"}, pwd, &cmdOptions)
	if err != nil {
		stderr := stderr.String()
		if len(stderr) > 0 {
			t.Logf("stderr: %s", stderr)
		}
		stdout := stdout.String()
		if len(stdout) > 0 {
			t.Logf("stdout: %s", stdout)
		}

	}
	require.NoError(t, err)
}

func TestGenerateTypeNames(t *testing.T) {
	test.TestTypeNameCodegen(t, "nodejs", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		modules, info, err := generateModuleContextMap("test", pkg, nil)
		require.NoError(t, err)

		pkg.Language["nodejs"] = info

		root, ok := modules[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t, false, nil)
		}
	})
}

func TestPascalCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "hi",
			expected: "Hi",
		},
		{
			input:    "NothingChanges",
			expected: "NothingChanges",
		},
		{
			input:    "everything-changed",
			expected: "EverythingChanged",
		},
	}
	for _, tt := range tests {
		result := pascal(tt.input)
		require.Equal(t, tt.expected, result)
	}
}
