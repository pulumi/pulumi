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

package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// mockRequiredPolicy implements engine.RequiredPolicy for testing.
type mockRequiredPolicy struct {
	name      string
	version   string
	config    map[string]*json.RawMessage
	installed bool
	downloadF func(ctx context.Context, wrapper func(io.ReadCloser, int64) io.ReadCloser) (io.ReadCloser, int64, error)
	installF  func(ctx *plugin.Context, content io.ReadCloser, stdout, stderr io.Writer) error
}

var _ engine.RequiredPolicy = (*mockRequiredPolicy)(nil)

func (m *mockRequiredPolicy) Name() string                        { return m.name }
func (m *mockRequiredPolicy) Version() string                     { return m.version }
func (m *mockRequiredPolicy) Config() map[string]*json.RawMessage { return m.config }
func (m *mockRequiredPolicy) Installed() bool                     { return m.installed }
func (m *mockRequiredPolicy) LocalPath() (string, error)          { return "/path/to/" + m.name, nil }

func (m *mockRequiredPolicy) Download(
	ctx context.Context,
	wrapper func(io.ReadCloser, int64) io.ReadCloser,
) (io.ReadCloser, int64, error) {
	if m.downloadF != nil {
		return m.downloadF(ctx, wrapper)
	}
	content := io.NopCloser(bytes.NewReader([]byte("mock-tarball")))
	size := int64(len("mock-tarball"))
	return wrapper(content, size), size, nil
}

func (m *mockRequiredPolicy) Install(
	ctx *plugin.Context,
	content io.ReadCloser,
	stdout, stderr io.Writer,
) error {
	if m.installF != nil {
		return m.installF(ctx, content, stdout, stderr)
	}
	return nil
}

// newMockStack creates a MockStack wired to the given MockBackend.
func newMockStack(be *backend.MockBackend) *backend.MockStack {
	return &backend.MockStack{
		BackendF: func() backend.Backend { return be },
		RefF:     func() backend.StackReference { return &backend.MockStackReference{StringV: "test-stack"} },
	}
}

func TestPolicyInstallCmd_Run(t *testing.T) {
	t.Parallel()

	t.Run("uses current stack when no stack specified", func(t *testing.T) {
		t.Parallel()

		var requestedStackName string
		cmd := policyInstallCmd{
			getwd: func() (string, error) { return t.TempDir(), nil },
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				requestedStackName = stackName
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "")
		require.NoError(t, err)
		assert.Empty(t, requestedStackName, "expected empty stack name to use current stack")
	})

	t.Run("uses specified stack", func(t *testing.T) {
		t.Parallel()

		var requestedStackName string
		cmd := policyInstallCmd{
			getwd: func() (string, error) { return t.TempDir(), nil },
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				requestedStackName = stackName
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		require.NoError(t, err)
		assert.Equal(t, "my-stack", requestedStackName)
	})

	t.Run("returns error when stack not found", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("stack not found")
		cmd := policyInstallCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return nil, expectedErr
			},
		}

		err := cmd.Run(t.Context(), "nonexistent-stack")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns friendly error when no stack specified and no project found", func(t *testing.T) {
		t.Parallel()

		cmd := policyInstallCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return nil, fmt.Errorf("loading stack: %w", workspace.ErrProjectNotFound)
			},
		}

		err := cmd.Run(t.Context(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find a Pulumi project")
		assert.Contains(t, err.Error(), "--stack flag")
		assert.NotErrorIs(t, err, workspace.ErrProjectNotFound,
			"friendly error should not wrap the original error")
	})

	t.Run("returns original error when stack specified and no project found", func(t *testing.T) {
		t.Parallel()

		cmd := policyInstallCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return nil, workspace.ErrProjectNotFound
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		assert.ErrorIs(t, err, workspace.ErrProjectNotFound)
	})

	t.Run("returns error when GetStackPolicyPacks fails", func(t *testing.T) {
		t.Parallel()

		policyErr := errors.New("policy service unavailable")
		cmd := policyInstallCmd{
			getwd: func() (string, error) { return t.TempDir(), nil },
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, policyErr
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		assert.ErrorIs(t, err, policyErr)
		assert.ErrorContains(t, err, "getting stack policy packs")
	})

	t.Run("returns error when getwd fails", func(t *testing.T) {
		t.Parallel()

		getwdErr := errors.New("filesystem error")
		cmd := policyInstallCmd{
			getwd: func() (string, error) { return "", getwdErr },
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return []engine.RequiredPolicy{
							&mockRequiredPolicy{name: "some-pack"},
						}, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		assert.ErrorIs(t, err, getwdErr)
		assert.ErrorContains(t, err, "getting current working directory")
	})

	t.Run("succeeds with no policy packs", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		require.NoError(t, err)
	})

	t.Run("no policy packs prints message to stderr", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "")
		require.NoError(t, err)
		assert.Equal(t, "No policy packs to install for stack test-stack\n", stderr.String())
	})

	t.Run("installs all policy packs", func(t *testing.T) {
		t.Parallel()

		packs := []engine.RequiredPolicy{
			&mockRequiredPolicy{name: "pack-a", installed: true},
			&mockRequiredPolicy{name: "pack-b", installed: true},
			&mockRequiredPolicy{name: "pack-c", installed: true},
		}

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return packs, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "Successfully installed 3 policy packs for stack test-stack")
	})

	t.Run("prints singular success message for one policy pack", func(t *testing.T) {
		t.Parallel()

		packs := []engine.RequiredPolicy{
			&mockRequiredPolicy{name: "my-pack", installed: true},
		}

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return packs, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "")
		require.NoError(t, err)
		assert.Equal(t, "Successfully installed 1 policy pack for stack test-stack\n", stderr.String())
	})

	t.Run("prints plural success message for multiple policy packs", func(t *testing.T) {
		t.Parallel()

		packs := []engine.RequiredPolicy{
			&mockRequiredPolicy{name: "pack-a", installed: true},
			&mockRequiredPolicy{name: "pack-b", installed: true},
		}

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return packs, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "")
		require.NoError(t, err)
		assert.Equal(t, "Successfully installed 2 policy packs for stack test-stack\n", stderr.String())
	})

	t.Run("returns error when install fails", func(t *testing.T) {
		t.Parallel()

		installErr := errors.New("install failed")
		packs := []engine.RequiredPolicy{
			&mockRequiredPolicy{name: "good-pack", installed: true},
			&mockRequiredPolicy{
				name: "bad-pack",
				downloadF: func(
					ctx context.Context, wrapper func(io.ReadCloser, int64) io.ReadCloser,
				) (io.ReadCloser, int64, error) {
					return nil, 0, installErr
				},
			},
		}

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return packs, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		assert.ErrorIs(t, err, installErr)
		assert.ErrorContains(t, err, "policy pack bad-pack@v")
	})

	t.Run("returns first install error", func(t *testing.T) {
		t.Parallel()

		installErr := errors.New("install failed")
		packs := []engine.RequiredPolicy{
			&mockRequiredPolicy{
				name: "failing-pack",
				downloadF: func(
					ctx context.Context, wrapper func(io.ReadCloser, int64) io.ReadCloser,
				) (io.ReadCloser, int64, error) {
					return nil, 0, installErr
				},
			},
			&mockRequiredPolicy{name: "other-pack", installed: true},
		}

		var stderr bytes.Buffer
		cmd := policyInstallCmd{
			getwd:  func() (string, error) { return t.TempDir(), nil },
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return packs, nil
					},
				}), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack")
		assert.ErrorIs(t, err, installErr)
	})
}
