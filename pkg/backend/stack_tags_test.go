// Copyright 2026, Pulumi Corporation.
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

package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// TestUpdateStackTagsValidation ensures the shared UpdateStackTags entrypoint
// validates tags for every backend, not just the Pulumi Cloud (httpstate)
// client. See https://github.com/pulumi/pulumi/issues/24428.
func TestUpdateStackTagsValidation(t *testing.T) {
	t.Parallel()

	// A mock backend that records whether it was reached, so we can assert
	// that invalid tags never make it past validation.
	called := false
	be := &MockBackend{
		UpdateStackTagsF: func(
			ctx context.Context, stack Stack, tags map[apitype.StackTagName]string,
		) error {
			called = true
			return nil
		},
	}
	stack := &MockStack{
		BackendF: func() Backend { return be },
	}

	t.Run("rejects an over-long tag name", func(t *testing.T) {
		called = false
		tags := map[apitype.StackTagName]string{
			"this tag name is way over forty characters long!!!": "value",
		}
		err := UpdateStackTags(context.Background(), stack, tags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too long")
		assert.False(t, called, "backend must not be called for invalid tags")
	})

	t.Run("rejects an invalid tag name character set", func(t *testing.T) {
		called = false
		tags := map[apitype.StackTagName]string{
			"not valid!": "value",
		}
		err := UpdateStackTags(context.Background(), stack, tags)
		require.Error(t, err)
		assert.False(t, called, "backend must not be called for invalid tags")
	})

	t.Run("passes valid tags through to the backend", func(t *testing.T) {
		called = false
		tags := map[apitype.StackTagName]string{
			"environment": "development",
		}
		err := UpdateStackTags(context.Background(), stack, tags)
		require.NoError(t, err)
		assert.True(t, called, "backend must be called for valid tags")
	})
}
