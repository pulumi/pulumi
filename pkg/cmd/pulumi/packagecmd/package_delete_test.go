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
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    PackageVersion
		expectedErr error
	}{
		{
			name:  "valid package version",
			input: "private/myorg/my-package@1.0.0",
			expected: PackageVersion{
				source:    "private",
				publisher: "myorg",
				name:      "my-package",
				version:   semver.MustParse("1.0.0"),
			},
		},
		{
			name:  "valid package version with prerelease",
			input: "pulumi/pulumi/aws@6.0.0-alpha.1",
			expected: PackageVersion{
				source:    "pulumi",
				publisher: "pulumi",
				name:      "aws",
				version:   semver.MustParse("6.0.0-alpha.1"),
			},
		},
		{
			name:  "valid package version with build metadata",
			input: "private/org/pkg@1.2.3+build.456",
			expected: PackageVersion{
				source:    "private",
				publisher: "org",
				name:      "pkg",
				version:   semver.MustParse("1.2.3+build.456"),
			},
		},
		{
			name:        "missing version",
			input:       "private/myorg/my-package",
			expectedErr: errors.New("invalid package version format"),
		},
		{
			name:        "empty version after @",
			input:       "private/myorg/my-package@",
			expectedErr: errors.New("invalid package version format"),
		},
		{
			name:        "too few path components",
			input:       "myorg/my-package@1.0.0",
			expectedErr: errors.New("invalid package name format"),
		},
		{
			name:        "too many path components",
			input:       "private/extra/myorg/my-package@1.0.0",
			expectedErr: errors.New("invalid package name format"),
		},
		{
			name:        "invalid semantic version",
			input:       "private/myorg/my-package@invalid",
			expectedErr: errors.New("invalid semantic version"),
		},
		{
			name:        "empty input",
			input:       "",
			expectedErr: errors.New("invalid package version format"),
		},
		{
			name:        "only @",
			input:       "@1.0.0",
			expectedErr: errors.New("invalid package name format"),
		},
		{
			name:        "empty source",
			input:       "/myorg/my-package@1.0.0",
			expectedErr: errors.New("source, publisher, and name cannot be empty"),
		},
		{
			name:        "empty publisher",
			input:       "private//my-package@1.0.0",
			expectedErr: errors.New("source, publisher, and name cannot be empty"),
		},
		{
			name:        "empty name",
			input:       "private/myorg/@1.0.0",
			expectedErr: errors.New("source, publisher, and name cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			packageVersion, err := parsePackageVersion(tt.input)

			if tt.expectedErr != nil {
				require.ErrorContains(t, err, tt.expectedErr.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.expected, packageVersion)
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackageDeleteCmd_Run(t *testing.T) {
	tests := []struct {
		name              string
		packageVersion    string
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
			name:           "invalid package version format",
			packageVersion: "invalid-format",
			expectedErr:    "invalid package version format",
		},
		{
			name:           "invalid semantic version",
			packageVersion: "private/myorg/my-package@not-semver",
			expectedErr:    "invalid semantic version",
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

	cmd := newPackageDeleteCmd()
	cmd.SetArgs([]string{"private/myorg/my-package@1.0.0"})
	err := cmd.ExecuteContext(t.Context())
	// In non-interactive mode without --yes, should fail
	// Note: This test assumes the test environment is non-interactive.
	// The actual behavior depends on cmdutil.Interactive() which checks if stdin is a terminal.
	assert.ErrorContains(t, err, "non-interactive mode requires --yes flag")
}
