// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"testing"

	utilenv "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // This test modifies utilenv.Global and cannot run in parallel
func TestGetRefreshOption(t *testing.T) {
	tests := []struct {
		name                 string
		refresh              string
		project              workspace.Project
		envVars              map[string]string
		expectedRefreshState bool
	}{
		{
			"No options specified means no refresh",
			"",
			workspace.Project{},
			nil,
			false,
		},
		{
			"Passing --refresh=true causes a refresh",
			"true",
			workspace.Project{},
			nil,
			true,
		},
		{
			"Passing --refresh=false causes no refresh",
			"false",
			workspace.Project{},
			nil,
			false,
		},
		{
			"Setting Refresh at a project level via Pulumi.yaml and no CLI args",
			"",
			workspace.Project{
				Name:    "auto-refresh",
				Runtime: workspace.ProjectRuntimeInfo{},
				Options: &workspace.ProjectOptions{
					Refresh: "always",
				},
			},
			nil,
			true,
		},
		{
			"Setting Refresh at a project level via Pulumi.yaml and --refresh=false",
			"false",
			workspace.Project{
				Name:    "auto-refresh",
				Runtime: workspace.ProjectRuntimeInfo{},
				Options: &workspace.ProjectOptions{
					Refresh: "always",
				},
			},
			nil,
			false,
		},
		{
			"Environment variable PULUMI_REFRESH=true causes a refresh",
			"",
			workspace.Project{},
			map[string]string{"PULUMI_REFRESH": "true"},
			true,
		},
		{
			"Environment variable PULUMI_REFRESH=1 causes a refresh",
			"",
			workspace.Project{},
			map[string]string{"PULUMI_REFRESH": "1"},
			true,
		},
		{
			"Environment variable PULUMI_REFRESH=false causes no refresh",
			"",
			workspace.Project{},
			map[string]string{"PULUMI_REFRESH": "false"},
			false,
		},
		{
			"CLI flag --refresh=false overrides PULUMI_REFRESH=true",
			"false",
			workspace.Project{},
			map[string]string{"PULUMI_REFRESH": "true"},
			false,
		},
		{
			"CLI flag --refresh=true overrides PULUMI_REFRESH=false",
			"true",
			workspace.Project{},
			map[string]string{"PULUMI_REFRESH": "false"},
			true,
		},
		{
			"Project config overrides environment variable",
			"",
			workspace.Project{
				Name:    "auto-refresh",
				Runtime: workspace.ProjectRuntimeInfo{},
				Options: &workspace.ProjectOptions{
					Refresh: "always",
				},
			},
			map[string]string{"PULUMI_REFRESH": "false"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables if specified
			if tt.envVars != nil {
				// Save and restore the global environment
				oldGlobal := utilenv.Global
				utilenv.Global = utilenv.MapStore(tt.envVars)
				defer func() { utilenv.Global = oldGlobal }()
			}

			shouldRefresh, err := getRefreshOption(&tt.project, tt.refresh)
			if err != nil {
				t.Errorf("getRefreshOption() error = %v", err)
			}
			if shouldRefresh != tt.expectedRefreshState {
				t.Errorf("getRefreshOption got = %t, expected %t", shouldRefresh, tt.expectedRefreshState)
			}
		})
	}
}
