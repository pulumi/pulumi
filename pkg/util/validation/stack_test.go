package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
)

func TestValidateStackName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		title     string
		stackName string
		error     string
	}{
		{"empty", "", "a stack name may not be empty"},
		{"valid", "foo", ""},
		{
			title:     "slash",
			stackName: "foo/bar",
			error: "a stack name may only contain alphanumeric, hyphens, " +
				"underscores, or periods: invalid character '/'" +
				" at position 3",
		},
		{"long", strings.Repeat("a", 100), ""},
		{
			title:     "too-long",
			stackName: strings.Repeat("a", 101),
			error:     "a stack name cannot exceed 100 characters",
		},
		{
			title:     "first-char-invalid",
			stackName: "@stack-name",
			error: "a stack name may only contain alphanumeric, hyphens, " +
				"underscores, or periods: invalid character '@' " +
				"at position 0",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()
			err := ValidateStackName(tt.stackName)
			if tt.error == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.error)
			}
		})
	}
}

func TestValidateStackTag(t *testing.T) {
	t.Parallel()

	t.Run("valid tags", func(t *testing.T) {
		t.Parallel()

		names := []string{
			"tag-name",
			"-",
			"..",
			"foo:bar:baz",
			"__underscores__",
			"AaBb123",
		}

		for _, name := range names {
			name := name
			t.Run(name, func(t *testing.T) {
				tags := map[apitype.StackTagName]string{
					name: "tag-value",
				}

				err := ValidateStackTags(tags)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("invalid stack tag names", func(t *testing.T) {
		t.Parallel()

		names := []string{
			"tag!",
			"something with spaces",
			"escape\nsequences\there",
			"😄",
			"foo***bar",
		}

		for _, name := range names {
			name := name
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				tags := map[apitype.StackTagName]string{
					name: "tag-value",
				}

				err := ValidateStackTags(tags)
				assert.Error(t, err)
				msg := "stack tag names may only contain alphanumerics, hyphens, underscores, periods, or colons"
				assert.Equal(t, err.Error(), msg)
			})
		}
	})

	t.Run("too long tag name", func(t *testing.T) {
		t.Parallel()

		tags := map[apitype.StackTagName]string{
			strings.Repeat("v", 41): "tag-value",
		}

		err := ValidateStackTags(tags)
		assert.Error(t, err)
		msg := fmt.Sprintf("stack tag %q is too long (max length %d characters)", strings.Repeat("v", 41), 40)
		assert.Equal(t, err.Error(), msg)
	})

	t.Run("too long tag value", func(t *testing.T) {
		t.Parallel()

		tags := map[apitype.StackTagName]string{
			"tag-name": strings.Repeat("v", 257),
		}

		err := ValidateStackTags(tags)
		assert.Error(t, err)
		msg := fmt.Sprintf("stack tag %q value is too long (max length %d characters)", "tag-name", 256)
		assert.Equal(t, err.Error(), msg)
	})
}
