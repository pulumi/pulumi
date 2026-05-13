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

package neo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestNeoUpgradeMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		caps           apitype.Capabilities
		currentVersion string
		wantUpgrade    bool
		wantContains   []string
	}{
		{
			name:           "no capability advertised",
			caps:           apitype.Capabilities{},
			currentVersion: "3.100.0",
		},
		{
			name: "empty MinCLIVersion",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: ""},
			},
			currentVersion: "3.100.0",
		},
		{
			name: "current equals required",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.0"},
			},
			currentVersion: "3.250.0",
		},
		{
			name: "current newer than required",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.0"},
			},
			currentVersion: "3.260.5",
		},
		{
			name: "current older than required",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.0"},
			},
			currentVersion: "3.233.0",
			wantUpgrade:    true,
			wantContains:   []string{"3.250.0", "3.233.0", "upgrade"},
		},
		{
			name: "dev build with empty version falls through",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.0"},
			},
			currentVersion: "",
		},
		{
			name: "unparseable MinCLIVersion falls through",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "not-a-semver"},
			},
			currentVersion: "3.100.0",
		},
		{
			name: "patch difference enforced",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.5"},
			},
			currentVersion: "3.250.4",
			wantUpgrade:    true,
			wantContains:   []string{"3.250.5", "3.250.4"},
		},
		{
			name: "prerelease handled by ParseTolerant",
			caps: apitype.Capabilities{
				NeoCLIMode: &apitype.NeoCLIModeConfig{MinCLIVersion: "3.250.0"},
			},
			currentVersion: "3.250.0-alpha.1",
			wantUpgrade:    true,
			wantContains:   []string{"3.250.0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := neoUpgradeMessage(tc.caps, tc.currentVersion)
			if !tc.wantUpgrade {
				require.Empty(t, msg)
				return
			}
			require.NotEmpty(t, msg)
			for _, sub := range tc.wantContains {
				assert.Contains(t, msg, sub)
			}
		})
	}
}
