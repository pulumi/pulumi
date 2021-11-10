// nolint: lll
package nodejs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "nodejs",
		GenPackage: GeneratePackage,
		Checks: map[string]test.CodegenCheck{
			"nodejs/compile": typeCheckGeneratedPackage,
			"nodejs/test":    testGeneratedPackage,
		},
	})
}

func typeCheckGeneratedPackage(t *testing.T, pwd string) {
	// TODO: previous attempt used npm. It may be more popular and
	// better target than yarn, however our build uses yarn in
	// other places at the moment, and yarn does not run into the
	// ${VERSION} problem; use yarn for now.
	//
	// var npm string
	// npm, err = executable.FindExecutable("npm")
	// require.NoError(t, err)
	// // TODO remove when https://github.com/pulumi/pulumi/pull/7938 lands
	// file := filepath.Join(pwd, "package.json")
	// oldFile, err := ioutil.ReadFile(file)
	// require.NoError(t, err)
	// newFile := strings.ReplaceAll(string(oldFile), "${VERSION}", "0.0.1")
	// err = ioutil.WriteFile(file, []byte(newFile), 0600)
	// require.NoError(t, err)
	// err = integration.RunCommand(t, "npm install", []string{npm, "i"}, pwd, &cmdOptions)
	// require.NoError(t, err)

	test.RunCommand(t, "yarn_link", pwd, "yarn", "link", "@pulumi/pulumi")
	test.RunCommand(t, "yarn_install", pwd, "yarn", "install")
	tscOptions := &integration.ProgramTestOptions{
		// Avoid Out of Memory error on CI:
		Env: []string{"NODE_OPTIONS=--max_old_space_size=4096"},
	}
	test.RunCommandWithOptions(t, tscOptions, "tsc", pwd, "yarn", "run", "tsc", "--noEmit")
}

// Runs unit tests against the generated code.
func testGeneratedPackage(t *testing.T, pwd string) {
	test.RunCommand(t, "mocha", pwd,
		"yarn", "run", "mocha", "-r", "ts-node/register", "tests/**/*.spec.ts")
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

func Test_isStringType(t *testing.T) {
	tests := []struct {
		name     string
		input    schema.Type
		expected bool
	}{
		{"string", schema.StringType, true},
		{"int", schema.IntType, false},
		{"Input[string]", &schema.InputType{ElementType: schema.StringType}, true},
		{"Input[int]", &schema.InputType{ElementType: schema.IntType}, false},
		{"StrictStringEnum", &schema.EnumType{ElementType: schema.StringType}, true},
		{"StrictIntEnum", &schema.EnumType{ElementType: schema.IntType}, false},
		{"RelaxedStringEnum", &schema.UnionType{
			ElementTypes: []schema.Type{&schema.EnumType{ElementType: schema.StringType}, schema.StringType},
		}, true},
		{"RelaxedIntEnum", &schema.UnionType{
			ElementTypes: []schema.Type{&schema.EnumType{ElementType: schema.IntType}, schema.IntType},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStringType(tt.input); got != tt.expected {
				t.Errorf("isStringType() = %v, want %v", got, tt.expected)
			}
		})
	}
}
