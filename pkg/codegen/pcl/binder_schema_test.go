package pcl

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func BenchmarkLoadPackage(b *testing.B) {
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(loader, "aws")
		contract.AssertNoError(err)
	}
}

//func TestAliasResolution(t *testing.T) {
//	t.Parallel()
//	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))
//
//	cache, err := NewPackageCache().loadPackageSchema(loader, "azure-native")
//	require.NoError(t, err)
//	_, tk, ok := cache.LookupResource("azure-native:web/v20210101:AppServiceEnvironment")
//	assert.True(t, ok, "could not find token")
//	assert.Equal(t, "azure-native:web:AppServiceEnvironment", tk)
//
//	token := "azure-native:vmwarecloudsimple:VirtualMachine" //nolint:gosec
//	_, tk, ok = cache.LookupResource(token)
//	assert.True(t, ok, "could not find token")
//	assert.Equal(t, token, tk)
//}
