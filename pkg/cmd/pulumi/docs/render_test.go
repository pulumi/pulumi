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

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebURL(t *testing.T) {
	t.Parallel()

	t.Run("docs path", func(t *testing.T) {
		t.Parallel()
		url := webURL("https://www.pulumi.com", "iac/concepts/stacks")
		assert.Equal(t, "https://www.pulumi.com/docs/iac/concepts/stacks/", url)
	})

	t.Run("registry path", func(t *testing.T) {
		t.Parallel()
		url := webURL("https://www.pulumi.com", "registry/packages/aws")
		assert.Equal(t, "https://www.pulumi.com/registry/packages/aws/", url)
	})
}
