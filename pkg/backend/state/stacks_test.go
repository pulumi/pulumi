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
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentStack(t *testing.T) {
	ctx := t.Context()

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

			ws := &pkgWorkspace.MockContext{}
			backend := &backend.MockBackend{
				URLF: func() string { return "https://api.pulumi.com" },
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, fullyQualifiedName, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, ws, backend)
			require.NoError(t, err)
		})

		t.Run("`$org/$stack` qualifies with specified org", func(t *testing.T) {
			orgQualifiedName := fmt.Sprintf("%s/%s", project, stack)
			t.Setenv("PULUMI_STACK", orgQualifiedName)

			ws := &pkgWorkspace.MockContext{}
			backend := &backend.MockBackend{
				URLF: func() string { return "https://api.pulumi.com" },
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, orgQualifiedName, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, ws, backend)
			require.NoError(t, err)
		})

		t.Run("`$stack` qualifies with individual org if default org not configured", func(t *testing.T) {
			individualOrgQualifiedName := fmt.Sprintf("%s/%s", user, stack)
			t.Setenv("PULUMI_STACK", stack)
			tempdir := t.TempDir()
			t.Setenv("PULUMI_CREDENTIALS_PATH", tempdir)
			backendURL := "test-backend-url"
			t.Setenv("PULUMI_BACKEND_URL", backendURL)

			writeConfig(t, tempdir, []byte("{}"))

			ws := &pkgWorkspace.MockContext{}
			backend := &backend.MockBackend{
				URLF: func() string { return backendURL },
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

			_, err := CurrentStack(ctx, ws, backend)
			require.NoError(t, err)
		})

		t.Run("`$stack` does not qualify with org if default org configured", func(t *testing.T) {
			t.Setenv("PULUMI_STACK", stack)
			tempdir := t.TempDir()
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

			ws := &pkgWorkspace.MockContext{}
			backend := &backend.MockBackend{
				URLF: func() string { return backendURL },
				SupportsOrganizationsF: func() bool {
					return true
				},
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, stack, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, ws, backend)
			require.NoError(t, err)
		})

		t.Run("`$stack` does not qualify with org if backend does not support orgs", func(t *testing.T) {
			t.Setenv("PULUMI_STACK", stack)

			ws := &pkgWorkspace.MockContext{}
			backend := &backend.MockBackend{
				URLF: func() string { return "file://~" },
				SupportsOrganizationsF: func() bool {
					return false
				},
				GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					assert.Equal(t, stack, ref.FullyQualifiedName().String())
					return &backend.MockStack{}, nil
				},
			}

			_, err := CurrentStack(ctx, ws, backend)
			require.NoError(t, err)
		})
	})

	t.Run("legacy cloud selection is ignored on local backend", func(t *testing.T) {
		settings := &pkgWorkspace.Settings{
			Stack: "cloud-org/my-project/my-stack",
		}
		ws := &pkgWorkspace.MockContext{
			NewF: func(string) (pkgWorkspace.W, error) {
				return &pkgWorkspace.MockW{
					SettingsF: func() *pkgWorkspace.Settings { return settings },
				}, nil
			},
		}
		be := &backend.MockBackend{
			URLF: func() string { return "file://~" },
			SupportsOrganizationsF: func() bool {
				return false
			},
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				assert.Equal(t, "cloud-org/my-project/my-stack", s)
				return nil, fmt.Errorf("organization name must be 'organization'")
			},
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
				t.Fatal("GetStack should not be called for a legacy selection from another backend")
				return nil, nil
			},
		}

		got, err := CurrentStack(ctx, ws, be)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("per-backend selections do not leak across backends", func(t *testing.T) {
		settings := &pkgWorkspace.Settings{
			Stacks: map[string]string{
				"https://api.pulumi.com": "cloud-org/my-project/dev",
				"file://~":               "organization/my-project/local",
			},
		}
		ws := &pkgWorkspace.MockContext{
			NewF: func(string) (pkgWorkspace.W, error) {
				return &pkgWorkspace.MockW{
					SettingsF: func() *pkgWorkspace.Settings { return settings },
				}, nil
			},
		}

		localBe := &backend.MockBackend{
			URLF: func() string { return "file://~" },
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				assert.Equal(t, "organization/my-project/local", s)
				return &backend.MockStackReference{
					FullyQualifiedNameV: tokens.QName(s),
				}, nil
			},
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{}, nil
			},
		}
		got, err := CurrentStack(ctx, ws, localBe)
		require.NoError(t, err)
		assert.NotNil(t, got)

		cloudBe := &backend.MockBackend{
			URLF: func() string { return "https://api.pulumi.com" },
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				assert.Equal(t, "cloud-org/my-project/dev", s)
				return &backend.MockStackReference{
					FullyQualifiedNameV: tokens.QName(s),
				}, nil
			},
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{}, nil
			},
		}
		got, err = CurrentStack(ctx, ws, cloudBe)
		require.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("invalid scoped selection still returns parse error", func(t *testing.T) {
		settings := &pkgWorkspace.Settings{
			Stacks: map[string]string{
				"file://~": "cloud-org/my-project/dev",
			},
		}
		ws := &pkgWorkspace.MockContext{
			NewF: func(string) (pkgWorkspace.W, error) {
				return &pkgWorkspace.MockW{
					SettingsF: func() *pkgWorkspace.Settings { return settings },
				}, nil
			},
		}
		be := &backend.MockBackend{
			URLF: func() string { return "file://~" },
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				return nil, fmt.Errorf("organization name must be 'organization'")
			},
		}

		_, err := CurrentStack(ctx, ws, be)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "organization name must be 'organization'")
	})
}

func writeConfig(t *testing.T, dir string, config []byte) {
	require.NoError(t, os.WriteFile(dir+"/config.json", config, 0o600))
}
