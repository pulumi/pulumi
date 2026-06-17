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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // these subtests use t.Setenv
func TestPulumiCloudCredentialEnv(t *testing.T) {
	t.Run("cloud login injects api address and token", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")
		t.Setenv("PULUMI_ACCESS_TOKEN", "secret")

		assert.Equal(t, map[string]string{
			"PULUMI_API":          "https://api.example.com",
			"PULUMI_ACCESS_TOKEN": "secret",
		}, pulumiCloudCredentialEnv(nil))
	})

	t.Run("diy backend gets nothing", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "file:///tmp/state")
		t.Setenv("PULUMI_ACCESS_TOKEN", "secret")

		assert.Nil(t, pulumiCloudCredentialEnv(nil))
	})

	t.Run("logged out gets nothing", func(t *testing.T) {
		t.Setenv("PULUMI_CREDENTIALS_PATH", t.TempDir())
		t.Setenv("PULUMI_BACKEND_URL", "")
		t.Setenv("PULUMI_ACCESS_TOKEN", "")

		assert.Nil(t, pulumiCloudCredentialEnv(nil))
	})
}
