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

package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Create a default config in us-west-2.
	cfg1, err := getAWSConfig(ctx, "us-west-2", "", "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", cfg1.Region)

	// Create a config with explicit credentials and ensure they're set.
	cfg2, err := getAWSConfig(ctx, "us-west-2", "AKIA123", "456", "xyz", "", false)
	require.NoError(t, err)

	creds, err := cfg2.Credentials.Retrieve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "AKIA123", creds.AccessKeyID)
	assert.Equal(t, "456", creds.SecretAccessKey)
	assert.Equal(t, "xyz", creds.SessionToken)

	// Create a config with different creds and make sure they're different.
	cfg3, err := getAWSConfig(ctx, "us-west-2", "AKIA123", "456", "hij", "", false)
	require.NoError(t, err)

	creds, err = cfg3.Credentials.Retrieve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "AKIA123", creds.AccessKeyID)
	assert.Equal(t, "456", creds.SecretAccessKey)
	assert.Equal(t, "hij", creds.SessionToken)
}
