package pcl

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/codegen/v3/schema"
	"github.com/pulumi/pulumi/codegen/v3/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func BenchmarkLoadPackage(b *testing.B) {
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(loader, "aws", "")
		contract.AssertNoError(err)
	}
}
