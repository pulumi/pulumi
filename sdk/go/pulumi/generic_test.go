package pulumi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputTElementType(t *testing.T) {
	t.Parallel()

	t.Run("OutputT[int]", func(t *testing.T) {
		e, ok := inputTElementType(typeOf[OutputT[int]]())
		assert.True(t, ok)
		assert.Equal(t, typeOf[int](), e)
	})
}
