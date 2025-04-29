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

package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates environment variables
func TestCurrentStack(t *testing.T) {
	ctx := context.Background()

	// Earlier versions of the Pulumi CLI did not always store the current selected stack with the fully qualified
	// name. For backwards compatibility, ensure that users upgrading to newer versions that do qualify with the org name,
	// picks the right org name.
	t.Run("qualifies with correct org name", func(t *testing.T) {
		user := "my-test-user"
		org := "my-test-org"
		project := "my-test-project"
		stack := "my-test-stack"

		t.Run("`$org/$project/$stack` qualifies with specified org", func(t *testing.T) {
			fullyQualifiedName := fmt.Sprintf("%s/%s/%s", org, project, stack)
			t.Setenv("PULUMI_STACK", fullyQualifiedName)

			backend := &backend.MockBackend{
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, fullyQualifiedName, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, backend)
			assert.NoError(t, err)
		})

		t.Run("`$org/$stack` qualifies with specified org", func(t *testing.T) {
			orgQualifiedName := fmt.Sprintf("%s/%s", project, stack)
			t.Setenv("PULUMI_STACK", orgQualifiedName)

			backend := &backend.MockBackend{
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, orgQualifiedName, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, backend)
			assert.NoError(t, err)
		})

		t.Run("`$stack` qualifies with individual org if default org not configured", func(t *testing.T) {
			individualOrgQualifiedName := fmt.Sprintf("%s/%s", user, stack)
			t.Setenv("PULUMI_STACK", stack)
			tempdir := tempProjectDir(t)
			t.Setenv("PULUMI_CREDENTIALS_PATH", tempdir)
			backendURL := "test-backend-url"
			t.Setenv("PULUMI_BACKEND_URL", backendURL)

			writeConfig(t, tempdir, []byte("{}"))

			backend := &backend.MockBackend{
				SupportsOrganizationsF: func() bool {
					return true
				},
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return user, nil, nil, nil
				},
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, individualOrgQualifiedName, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, backend)
			assert.NoError(t, err)
		})

		t.Run("`$stack` does not qualify with org if default org configured", func(t *testing.T) {
			t.Setenv("PULUMI_STACK", stack)
			tempdir := tempProjectDir(t)
			t.Setenv("PULUMI_CREDENTIALS_PATH", tempdir)
			backendURL := "test-backend-url"
			t.Setenv("PULUMI_BACKEND_URL", backendURL)

			stub := fmt.Sprintf(`{
				"backends": {
					"%s": {
						"defaultOrg": "%s"
					}
				}
			}`, backendURL, org)
			writeConfig(t, tempdir, []byte(stub))

			backend := &backend.MockBackend{
				SupportsOrganizationsF: func() bool {
					return true
				},
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, stack, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, backend)
			assert.NoError(t, err)
		})

		t.Run("`$stack` does not qualify with org if backend does not support orgs", func(t *testing.T) {
			t.Setenv("PULUMI_STACK", stack)

			backend := &backend.MockBackend{
				SupportsOrganizationsF: func() bool {
					return false
				},
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, stack, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, backend)
			assert.NoError(t, err)
		})
	})
}

func writeConfig(t *testing.T, dir string, config []byte) {
	assert.NoError(t, os.WriteFile(dir+"/config.json", config, 0o600))
}

func tempProjectDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), genUniqueName(t))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return dir
}

func genUniqueName(t *testing.T) string {
	t.Helper()

	var bs [8]byte
	_, err := rand.Read(bs[:])
	require.NoError(t, err)

	return "test-" + hex.EncodeToString(bs[:])
}
