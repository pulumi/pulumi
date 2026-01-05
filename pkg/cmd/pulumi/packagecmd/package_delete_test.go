// Copyright 2025, Pulumi Corporation.
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

package packagecmd

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackageDeleteCmd_Run(t *testing.T) {
	tests := []struct {
		name              string
		packageVersion    string
		resolveErr        error
		deleteErr         error
		registryErr       error
		expectedErr       string
		expectedDeleteArg struct {
			source    string
			publisher string
			name      string
			version   semver.Version
		}
	}{
		{
			name:           "successful delete",
			packageVersion: "private/myorg/my-package@1.0.0",
			expectedDeleteArg: struct {
				source    string
				publisher string
				name      string
				version   semver.Version
			}{
				source:    "private",
				publisher: "myorg",
				name:      "my-package",
				version:   semver.MustParse("1.0.0"),
			},
		},
		{
			name:           "delete fails with backend error",
			packageVersion: "private/myorg/my-package@1.0.0",
			deleteErr:      errors.New("permission denied"),
			expectedErr:    "failed to delete package version",
		},
		{
			name:           "invalid semantic version",
			packageVersion: "private/myorg/my-package@not-semver",
			expectedErr:    "invalid version",
		},
		{
			name:           "package not found",
			packageVersion: "private/myorg/nonexistent@1.0.0",
			resolveErr:     errors.New("package not found"),
			expectedErr:    "package not found",
		},
		{
			name:           "registry backend error",
			packageVersion: "private/myorg/my-package@1.0.0",
			registryErr:    errors.New("failed to connect to registry"),
			expectedErr:    "failed to get the registry backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deletedSource, deletedPublisher, deletedName string
			var deletedVersion semver.Version

			mockCloudRegistry := &backend.MockCloudRegistry{
				Mock: registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if tt.resolveErr != nil {
							return apitype.PackageMetadata{}, tt.resolveErr
						}
						return apitype.PackageMetadata{
							Source:    "private",
							Publisher: "myorg",
							Name:      "my-package",
							Version:   semver.MustParse("1.0.0"),
						}, nil
					},
				},
				DeletePackageVersionF: func(
					ctx context.Context, source, publisher, name string, version semver.Version,
				) error {
					deletedSource = source
					deletedPublisher = publisher
					deletedName = name
					deletedVersion = version
					return tt.deleteErr
				},
			}

			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					if tt.registryErr != nil {
						return nil, tt.registryErr
					}
					return mockCloudRegistry, nil
				},
				GetReadOnlyCloudRegistryF: func() registry.Registry { return mockCloudRegistry },
			})

			cmd := newPackageDeleteCmd()
			cmd.SetArgs([]string{tt.packageVersion, "--yes" /* Skip confirmation for tests */})
			err := cmd.ExecuteContext(t.Context())

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedDeleteArg.source, deletedSource)
				assert.Equal(t, tt.expectedDeleteArg.publisher, deletedPublisher)
				assert.Equal(t, tt.expectedDeleteArg.name, deletedName)
				assert.Equal(t, tt.expectedDeleteArg.version, deletedVersion)
			}
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackageDeleteCmd_NonInteractiveRequiresYes(t *testing.T) {
	mockCloudRegistry := &backend.MockCloudRegistry{
		Mock: registry.Mock{
			GetPackageF: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				return apitype.PackageMetadata{
					Source:    "private",
					Publisher: "myorg",
					Name:      "my-package",
					Version:   semver.MustParse("1.0.0"),
				}, nil
			},
		},
		DeletePackageVersionF: func(
			ctx context.Context, source, publisher, name string, version semver.Version,
		) error {
			require.Fail(t, "DeletePackageVersion should not be called without --yes in non-interactive mode")
			return nil
		},
	}

	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return mockCloudRegistry, nil
		},
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockCloudRegistry },
	})

	defer func(old bool) { cmdutil.DisableInteractive = old }(cmdutil.DisableInteractive)
	cmdutil.DisableInteractive = true
	cmd := newPackageDeleteCmd()
	cmd.SetArgs([]string{"private/myorg/my-package@1.0.0"})
	err := cmd.ExecuteContext(t.Context())
	// In non-interactive mode without --yes, should fail
	assert.ErrorContains(t, err, "non-interactive mode requires --yes flag")
}
