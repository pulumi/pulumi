// nolint: lll
package nodejs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "nodejs", GeneratePackage)
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
