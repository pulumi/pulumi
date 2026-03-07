// Copyright 2016-2025, Pulumi Corporation.
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

package config

import (
	"context"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/require"
)

func TestParseEnvironmentImportsMetadata(t *testing.T) {
	t.Parallel()

	t.Run("no metadata", func(t *testing.T) {
		t.Parallel()

		imports, err := parseEnvironmentImportsMetadata(nil)
		require.NoError(t, err)
		require.Nil(t, imports)
	})

	t.Run("missing stack environments key", func(t *testing.T) {
		t.Parallel()

		imports, err := parseEnvironmentImportsMetadata(map[string]string{"other": "value"})
		require.NoError(t, err)
		require.Nil(t, imports)
	})

	t.Run("valid metadata", func(t *testing.T) {
		t.Parallel()

		imports, err := parseEnvironmentImportsMetadata(map[string]string{
			backend.StackEnvironments: `[{"id":"default/providers.aws"},{"id":"team/common"}]`,
		})
		require.NoError(t, err)
		require.Equal(t, []string{"default/providers.aws", "team/common"}, imports)
	})

	t.Run("invalid metadata", func(t *testing.T) {
		t.Parallel()

		_, err := parseEnvironmentImportsMetadata(map[string]string{
			backend.StackEnvironments: `{`,
		})
		require.Error(t, err)
	})
}

func TestOmitEnvironmentConfigValues(t *testing.T) {
	t.Parallel()

	stackConfig := config.Map{
		config.MustMakeKey("aws", "region"): config.NewValue("us-west-2"),
		config.MustMakeKey("proj", "name"):  config.NewValue("from-env"),
		config.MustMakeKey("proj", "mode"):  config.NewValue("from-stack"),
	}

	pulumiEnv := esc.NewValue(map[string]esc.Value{
		"aws:region": esc.NewValue("us-west-2"),
		"name":       esc.NewValue("from-env"),
		"mode":       esc.NewValue("from-env"),
	})

	err := omitEnvironmentConfigValues(context.Background(), "dev", "proj", pulumiEnv, stackConfig)
	require.NoError(t, err)

	_, hasRegion := stackConfig[config.MustMakeKey("aws", "region")]
	_, hasName := stackConfig[config.MustMakeKey("proj", "name")]
	mode, hasMode := stackConfig[config.MustMakeKey("proj", "mode")]

	require.False(t, hasRegion)
	require.False(t, hasName)
	require.True(t, hasMode)
	require.Equal(t, config.NewValue("from-stack"), mode)
}
