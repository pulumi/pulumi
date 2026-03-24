// Copyright 2016, Pulumi Corporation.
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

package env

import (
	"testing"

	utilenv "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest
func TestAccessTokenIsRedacted(t *testing.T) {
	// Not parallel: temporarily modifies the global env store.
	old := utilenv.Global
	defer func() { utilenv.Global = old }()

	utilenv.Global = utilenv.MapStore{
		"PULUMI_ACCESS_TOKEN": "super-secret-token",
	}

	// The raw value should be retrievable for actual use.
	assert.Equal(t, "super-secret-token", AccessToken.Value())

	// But it must be redacted in string representations.
	assert.Equal(t, "[secret]", AccessToken.String())

	// And it must be redacted in ConfiguredVariables output.
	vars := utilenv.ConfiguredVariables()
	assert.Equal(t, "[secret]", vars["PULUMI_ACCESS_TOKEN"])
}
