package internals

import (
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

func await(out pulumi.Output) (interface{}, bool, bool, []pulumi.Resource, error) {
	return awaitWithContext(context.Background(), out)
}

func TestBasicOutputs(t *testing.T) {
	t.Parallel()

	// Just test basic resolve and reject functionality.
	{
		out, resolve, _ := pulumi.NewOutput()
		go func() {
			resolve(42)
		}()
		v, known, secret, deps, err := await(out)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Nil(t, deps)
		assert.NotNil(t, v)
		assert.Equal(t, 42, v.(int))
	}
	{
		out, _, reject := pulumi.NewOutput()
		go func() {
			reject(errors.New("boom"))
		}()
		v, _, _, _, err := await(out)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}
