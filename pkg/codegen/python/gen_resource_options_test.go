package python

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/stretchr/testify/require"
)

func TestGenResourceOptions(t *testing.T) {
	f, err := (&ResourceOptionsGenerator{}).GenerateResourceOptions(codegen.PulumiResourceOptions)
	require.NoError(t, err)

	t.Log(string(f))
	t.Fail()
}
