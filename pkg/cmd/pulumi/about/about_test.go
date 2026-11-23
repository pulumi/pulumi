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

package about

import (
	"regexp"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI(t *testing.T) {
	t.Parallel()

	cli := getCLIAbout()
	assert.Equal(t, cli.GoVersion, runtime.Version())
	assert.Equal(t, cli.GoCompiler, runtime.Compiler)
}

func TestBackend(t *testing.T) {
	t.Parallel()

	stats, err := host.Info()
	if err != nil {
		t.Skipf("Underlying stats call failed: %s", err)
	}
	backend, err := getHostAbout()
	require.NoError(t, err, "We should be able to get stats here")
	display := backend.String()
	assert.Contains(t, display, stats.Platform)
	assert.Contains(t, display, stats.PlatformVersion)
	assert.Contains(t, display, stats.KernelArch)
}

func TestBackendAboutTokenType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		tokenInfo *workspace.TokenInformation
		expected  string
	}{
		{"personal", nil, "personal"},
		{"organization", &workspace.TokenInformation{Name: "t", Organization: "My Org"}, "organization: My Org"},
		{"team", &workspace.TokenInformation{Name: "t", Team: "My Team"}, "team: My Team"},
		{
			"team with organization",
			&workspace.TokenInformation{Name: "t", Organization: "My Org", Team: "My Team"},
			"team: My Team",
		},
		{
			"service principal",
			&workspace.TokenInformation{Name: "t", Organization: "My Org", ServicePrincipal: "CI Bot"},
			"service principal: CI Bot (organization: My Org)",
		},
		{
			"service principal without organization",
			&workspace.TokenInformation{Name: "t", ServicePrincipal: "CI Bot"},
			"service principal: CI Bot",
		},
		{"unknown", &workspace.TokenInformation{Name: "t"}, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			display := backendAbout{
				Name:             "pulumi.com",
				URL:              "https://pulumi.example.com",
				User:             "user1",
				TokenInformation: c.tokenInfo,
			}.String()
			assert.Regexp(t, `Token type\s+`+regexp.QuoteMeta(c.expected)+` *\n`, display)
		})
	}
}
