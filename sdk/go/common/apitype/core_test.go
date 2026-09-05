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

package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PreserveEnvironmentOnDelete intentionally omits the omitempty JSON tag so that a
// false value serializes explicitly; an absent key must stay distinguishable from
// an explicit false on the wire. Guard that here so a future edit can't silently
// reintroduce omitempty.
func TestStackConfig_PreserveEnvironmentOnDeleteSerializesFalse(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(StackConfig{Environment: "acme/prod"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"environment":"acme/prod","preserveEnvironmentOnDelete":false}`, string(b))

	var rt StackConfig
	require.NoError(t, json.Unmarshal([]byte(`{"preserveEnvironmentOnDelete":true}`), &rt))
	assert.True(t, rt.PreserveEnvironmentOnDelete)
}
