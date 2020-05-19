package hcl2

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/codegen/internal/test"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

func BenchmarkLoadPackage(b *testing.B) {
	host := test.NewHost(testdataPath)

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(host, "aws")
		contract.AssertNoError(err)
	}
}
