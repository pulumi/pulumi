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
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func wsWithCurrent(url, storedToken string) *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{
				Current:      url,
				AccessTokens: map[string]string{url: storedToken},
			}, nil
		},
	}
}

//nolint:paralleltest // mutates PULUMI_BACKEND_URL and PULUMI_ACCESS_TOKEN
func TestResolveResourceProviderEnv(t *testing.T) {
	const cloud = "https://api.pulumi.com"

	t.Run("cloud login prefers the access token from the environment", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "")
		t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")
		got := ResolveResourceProviderEnv(wsWithCurrent(cloud, "stored-token"))
		assert.Equal(t, map[string]string{
			"PULUMI_API":          cloud,
			"PULUMI_ACCESS_TOKEN": "env-token",
		}, got)
	})

	t.Run("cloud login falls back to stored credentials", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "")
		t.Setenv("PULUMI_ACCESS_TOKEN", "")
		got := ResolveResourceProviderEnv(wsWithCurrent(cloud, "stored-token"))
		assert.Equal(t, map[string]string{
			"PULUMI_API":          cloud,
			"PULUMI_ACCESS_TOKEN": "stored-token",
		}, got)
	})

	t.Run("DIY backend injects nothing", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "")
		t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")
		assert.Nil(t, ResolveResourceProviderEnv(wsWithCurrent("file:///tmp/state", "")))
	})

	t.Run("logged out injects nothing", func(t *testing.T) {
		t.Setenv("PULUMI_BACKEND_URL", "")
		t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")
		assert.Nil(t, ResolveResourceProviderEnv(&pkgWorkspace.MockContext{}))
	})
}
