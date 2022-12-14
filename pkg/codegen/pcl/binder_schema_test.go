package pcl

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func BenchmarkLoadPackage(b *testing.B) {
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(loader, "aws", "")
		contract.AssertNoError(err)
	}
}

func TestGenEnum(t *testing.T) {
	t.Parallel()
	enum := &model.EnumType{
		Elements: []cty.Value{
			cty.StringVal("foo"),
			cty.StringVal("bar"),
		},
		Type:  model.StringType,
		Token: "my:enum",
		Annotations: []interface{}{
			enumSchemaType{
				Type: &schema.EnumType{Elements: []*schema.Enum{{Value: "foo"}, {Value: "bar"}}},
			},
		},
	}
	safeEnumFunc := func(member *schema.Enum) {}
	unsafeEnumFunc := func(from model.Expression) {}

	d := GenEnum(enum, &model.LiteralValueExpression{
		Value: cty.StringVal("foo"),
	}, safeEnumFunc, unsafeEnumFunc)
	assert.Nil(t, d)

	d = GenEnum(enum, &model.LiteralValueExpression{
		Value: cty.StringVal("Bar"),
	}, safeEnumFunc, unsafeEnumFunc)
	assert.Equal(t, d.Summary, `"Bar" is not a valid value of the enum "my:enum"`)
	assert.Equal(t, d.Detail, `Valid members are "foo", "bar"`)

}
