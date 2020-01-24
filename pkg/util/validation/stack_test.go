package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/stretchr/testify/assert"
)

func TestValidateStackTag(t *testing.T) {
	t.Run("valid tag", func(t *testing.T) {
		tags := map[apitype.StackTagName]string{
			"tag-name": "tag-value",
		}

		err := ValidateStackTags(tags)
		assert.NoError(t, err)
	})

	t.Run("invalid stack tag name", func(t *testing.T) {
		tags := map[apitype.StackTagName]string{
			"hello!": "tag-value",
		}

		err := ValidateStackTags(tags)
		assert.Error(t, err)
		msg := "invalid stack tag name: " +
			"a stack tag name may only contain alphanumerics, hyphens, underscores, or periods"
		assert.Equal(t, err.Error(), msg)
	})

	t.Run("too long tag name", func(t *testing.T) {
		tags := map[apitype.StackTagName]string{
			strings.Repeat("v", 41): "tag-value",
		}

		err := ValidateStackTags(tags)
		assert.Error(t, err)
		msg := fmt.Sprintf("stack tag name %q is too long (max length %d characters)", strings.Repeat("v", 41), 40)
		assert.Equal(t, err.Error(), msg)
	})

	t.Run("too long tag value", func(t *testing.T) {
		tags := map[apitype.StackTagName]string{
			"tag-name": strings.Repeat("v", 257),
		}

		err := ValidateStackTags(tags)
		assert.Error(t, err)
		msg := fmt.Sprintf("stack tag value %q is too long (max length %d characters)", strings.Repeat("v", 257), 256)
		assert.Equal(t, err.Error(), msg)
	})
}
