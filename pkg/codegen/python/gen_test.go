package python

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

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
	for _, tt := range pathTests {
		t.Run(tt.input, func(t *testing.T) {
			result := relPathToRelImport(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "python", GeneratePackage)
}

func TestGenerateTypeNames(t *testing.T) {
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
			return root.typeString(t, false, false)
		}
	})
}
