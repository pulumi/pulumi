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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureNeoTaskOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		neoTaskOnFailureFlag    bool
		isDIYBackend            bool
		expectedStartNeoTaskErr bool
	}{
		{
			name:                    "flag enabled with cloud backend sets option",
			neoTaskOnFailureFlag:    true,
			isDIYBackend:            false,
			expectedStartNeoTaskErr: true,
		},
		{
			name:                    "flag disabled with cloud backend leaves option false",
			neoTaskOnFailureFlag:    false,
			isDIYBackend:            false,
			expectedStartNeoTaskErr: false,
		},
		{
			name:                    "flag enabled with DIY backend does not set option",
			neoTaskOnFailureFlag:    true,
			isDIYBackend:            true,
			expectedStartNeoTaskErr: false,
		},
		{
			name:                    "flag disabled with DIY backend leaves option false",
			neoTaskOnFailureFlag:    false,
			isDIYBackend:            true,
			expectedStartNeoTaskErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := display.Options{}
			configureNeoTaskOption(tt.neoTaskOnFailureFlag, nil, &opts, tt.isDIYBackend)
			require.Equal(t, tt.expectedStartNeoTaskErr, opts.StartNeoTaskOnError)
		})
	}
}

func TestGetRefreshOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		refresh              string
		project              workspace.Project
		expectedRefreshState bool
	}{
		{
			"No options specified means no refresh",
			"",
			workspace.Project{},
			false,
		},
		{
			"Passing --refresh=true causes a refresh",
			"true",
			workspace.Project{},
			true,
		},
		{
			"Passing --refresh=false causes no refresh",
			"false",
			workspace.Project{},
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
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			shouldRefresh, err := GetRefreshOption(&tt.project, tt.refresh)
			if err != nil {
				t.Errorf("GetRefreshOption() error = %v", err)
			}
			if shouldRefresh != tt.expectedRefreshState {
				t.Errorf("GetRefreshOption got = %t, expected %t", shouldRefresh, tt.expectedRefreshState)
			}
		})
	}
}

func TestRejectOnManagedStack(t *testing.T) {
	t.Parallel()

	managed := &backend.MockStack{ConfigLocationF: func() backend.StackConfigLocation {
		return backend.StackConfigLocation{ManagedEnvironment: "p/env"}
	}}
	unmanaged := &backend.MockStack{}

	assert.EqualError(t, rejectConfigFlagsOnManagedStack(managed, []string{"a=b"}, ""),
		"--config is not supported on stacks with an ESC-managed environment yet")
	assert.EqualError(t, rejectConfigFlagsOnManagedStack(managed, nil, "Pulumi.other.yaml"),
		"--config-file is not supported on stacks with an ESC-managed environment yet")
	require.NoError(t, rejectConfigFlagsOnManagedStack(managed, nil, ""))
	require.NoError(t, rejectConfigFlagsOnManagedStack(unmanaged, []string{"a=b"}, "Pulumi.other.yaml"))

	assert.EqualError(t, rejectTemplateUpOnManagedStack(managed),
		"pulumi up <template> is not supported on stacks with an ESC-managed environment yet")
	require.NoError(t, rejectTemplateUpOnManagedStack(unmanaged))
}
