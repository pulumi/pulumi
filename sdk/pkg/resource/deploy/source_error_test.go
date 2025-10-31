// Copyright 2016-2023, Pulumi Corporation.
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

package deploy

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorSource(t *testing.T) {
	t.Parallel()
	t.Run("Close is nil", func(t *testing.T) {
		t.Parallel()
		s := &errorSource{}
		require.NoError(t, s.Close())
		// Ensure idempotent.
		require.NoError(t, s.Close())
	})
	t.Run("Iterate panics", func(t *testing.T) {
		t.Parallel()
		s := &errorSource{}
		assert.Panics(t, func() {
			_, err := s.Iterate(context.Background(), nil)
			contract.Ignore(err)
		})
	})
}
