package testing

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func RandomStackName() string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	contract.AssertNoErrorf(err, "failed to generate random stack name")
	return "test" + hex.EncodeToString(b)
}
