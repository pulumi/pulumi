// Copyright 2024, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscEnvironmentConsoleURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cloudURL   string
		orgName    string
		envProject string
		envName    string
		want       string
	}{
		{
			name:       "api.pulumi.com becomes app.pulumi.com",
			cloudURL:   "https://api.pulumi.com",
			orgName:    "myorg",
			envProject: "myproject",
			envName:    "dev",
			want:       "https://app.pulumi.com/myorg/esc/myproject/dev",
		},
		{
			name:       "already app. prefix is unchanged",
			cloudURL:   "https://app.pulumi.com",
			orgName:    "myorg",
			envProject: "myproject",
			envName:    "dev",
			want:       "https://app.pulumi.com/myorg/esc/myproject/dev",
		},
		{
			name:       "non-standard host is preserved as-is",
			cloudURL:   "https://pulumi.acme.internal",
			orgName:    "myorg",
			envProject: "myproject",
			envName:    "staging",
			want:       "https://pulumi.acme.internal/myorg/esc/myproject/staging",
		},
		{
			name:       "invalid URL returns empty string",
			cloudURL:   "not-a-url :// bad",
			orgName:    "myorg",
			envProject: "myproject",
			envName:    "dev",
			want:       "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := escEnvironmentConsoleURL(tc.cloudURL, tc.orgName, tc.envProject, tc.envName)
			assert.Equal(t, tc.want, got)
		})
	}
}
