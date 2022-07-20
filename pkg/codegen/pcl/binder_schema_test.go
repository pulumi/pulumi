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
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath, nil))

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(loader, "aws", "")
		contract.AssertNoError(err)
	}
}
