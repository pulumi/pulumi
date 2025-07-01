// Copyright 2020-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			//nolint:paralleltest // golangci-lint v2 upgrade
			t.Run(name, func(t *testing.T) {
				tags := map[apitype.StackTagName]string{
					name: "tag-value",
				}

				err := ValidateStackTags(tags)
				require.NoError(t, err)
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
				msg := "stack tag names may only contain alphanumerics, hyphens, underscores, periods, or colons"
				assert.EqualError(t, err, msg)
			})
		}
	})

	t.Run("too long tag name", func(t *testing.T) {
		t.Parallel()

		tags := map[apitype.StackTagName]string{
			strings.Repeat("v", 41): "tag-value",
		}

		err := ValidateStackTags(tags)
		msg := fmt.Sprintf("stack tag %q is too long (max length %d characters)", strings.Repeat("v", 41), 40)
		assert.EqualError(t, err, msg)
	})

	t.Run("too long tag value", func(t *testing.T) {
		t.Parallel()

		tags := map[apitype.StackTagName]string{
			"tag-name": strings.Repeat("v", 257),
		}

		err := ValidateStackTags(tags)
		msg := fmt.Sprintf("stack tag %q value is too long (max length %d characters)", "tag-name", 256)
		assert.EqualError(t, err, msg)
	})
}
