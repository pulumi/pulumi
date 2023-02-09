package pcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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

func TestAzureLoaderConfusion(t *testing.T) {
	// To reproduce a panic a very specific condition is required here. This exact set of plugins installed:
	//
	// t0yv0@Antons-MacBook-Pro> pulumi plugin ls
	//
	// NAME     KIND      VERSION  SIZE   INSTALLED      LAST USED
	// azuread  resource  5.33.0   38 MB  5 minutes ago  5 minutes ago
	// random   resource  4.8.2    34 MB  5 minutes ago  5 minutes ago
	//
	// Moreover, one has to have pulumi-resource-azure in PATH and that provider needs to have the version v5.33.0.
	//
	// If these conditions are met you should see:
	//
	// - FAIL: TestAzureLoaderConfusion (0.20s)
	// panic: fatal: req: azuread@<nil>: entries: map[azure::0xc00053e3c0 azuread::0xc00053f640] (returned azure@5.33.0) [recovered]

	cwd, err := os.Getwd()
	contract.AssertNoError(err)
	sink := cmdutil.Diag()

	ctx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	contract.AssertNoError(err)
	loader := schema.NewPluginLoader(ctx.Host)

	cache := NewPackageCache()

	_, err = cache.loadPackageSchema(loader, "azure", "v5.33.0")
	contract.AssertNoError(err)

	_, err = cache.loadPackageSchema(loader, "azuread", "")
	contract.AssertNoError(err)
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
