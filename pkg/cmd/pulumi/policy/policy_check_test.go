// Copyright 2016-2026, Pulumi Corporation.
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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newMockCheckStack creates a MockStack with a snapshot wired to the given MockBackend.
func newMockCheckStack(be *backend.MockBackend, snap *deploy.Snapshot) *backend.MockStack {
	return &backend.MockStack{
		BackendF: func() backend.Backend { return be },
		RefF:     func() backend.StackReference { return &backend.MockStackReference{StringV: "test-stack"} },
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return snap, nil
		},
	}
}

func TestPolicyCheckCmd_Run(t *testing.T) {
	t.Parallel()

	bucketState := &resource.State{
		Type:    "aws:s3:Bucket",
		URN:     "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
		Inputs:  resource.PropertyMap{"bucketName": resource.NewStringProperty("my-bucket")},
		Outputs: resource.PropertyMap{"bucketName": resource.NewStringProperty("my-bucket"), "arn": resource.NewStringProperty("arn:aws:s3:::my-bucket")},
	}
	nonEmptySnap := &deploy.Snapshot{
		Resources: []*resource.State{bucketState},
	}

	t.Run("returns error when stack not found", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("stack not found")
		cmd := policyCheckCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return nil, expectedErr
			},
		}

		err := cmd.Run(t.Context(), "nonexistent-stack", nil, nil)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns friendly error when no stack specified and no project found", func(t *testing.T) {
		t.Parallel()

		cmd := policyCheckCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return nil, fmt.Errorf("loading stack: %w", workspace.ErrProjectNotFound)
			},
		}

		err := cmd.Run(t.Context(), "", nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find a Pulumi project")
		assert.Contains(t, err.Error(), "--stack flag")
	})

	t.Run("uses specified stack name", func(t *testing.T) {
		t.Parallel()

		var requestedStackName string
		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				requestedStackName = stackName
				return newMockCheckStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}, nonEmptySnap), nil
			},
		}

		err := cmd.Run(t.Context(), "my-stack", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "my-stack", requestedStackName)
	})

	t.Run("no policy packs prints message", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}, nonEmptySnap), nil
			},
		}

		err := cmd.Run(t.Context(), "", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "No policy packs to check for stack test-stack\n", stderr.String())
	})

	t.Run("empty snapshot prints message", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, nil
					},
				}, &deploy.Snapshot{Resources: nil}), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				t.Fatal("check should not be called for empty snapshot")
				return engine.CheckResult{}, nil
			},
		}

		// Need at least one policy pack to get past the "no policy packs" check.
		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "has no resources to check")
	})

	t.Run("check passes with no violations", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				return engine.CheckResult{Passed: true}, nil
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "Policy check passed")
	})

	t.Run("check passes with advisory violations", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				// Emit an advisory violation event.
				events <- engine.NewEvent(engine.PolicyViolationEventPayload{
					ResourceURN:      bucketState.URN,
					Message:          "consider adding tags",
					PolicyName:       "prefer-tags",
					PolicyPackName:   "advisory-pack",
					EnforcementLevel: apitype.Advisory,
					Prefix:           "advisory: ",
				})
				return engine.CheckResult{
					Passed:          true,
					AdvisoryViolations: 1,
				}, nil
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "Policy check passed with 1 advisory violation")
	})

	t.Run("check fails with mandatory violations", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				events <- engine.NewEvent(engine.PolicyViolationEventPayload{
					ResourceURN:      bucketState.URN,
					Message:          "bucket must not be public",
					PolicyName:       "no-public-buckets",
					PolicyPackName:   "strict-pack",
					EnforcementLevel: apitype.Mandatory,
					Prefix:           "mandatory: ",
				})
				return engine.CheckResult{
					Passed:              false,
					MandatoryViolations: 1,
				}, nil
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy check failed")
		assert.Contains(t, stdout.String(), "1 mandatory violation")
	})

	t.Run("check fails with mixed violations", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				events <- engine.NewEvent(engine.PolicyViolationEventPayload{
					EnforcementLevel: apitype.Mandatory,
					Prefix:           "mandatory: ",
					Message:          "violation 1",
				})
				events <- engine.NewEvent(engine.PolicyViolationEventPayload{
					EnforcementLevel: apitype.Mandatory,
					Prefix:           "mandatory: ",
					Message:          "violation 2",
				})
				events <- engine.NewEvent(engine.PolicyViolationEventPayload{
					EnforcementLevel: apitype.Advisory,
					Prefix:           "advisory: ",
					Message:          "warning 1",
				})
				return engine.CheckResult{
					Passed:              false,
					MandatoryViolations: 2,
					AdvisoryViolations:  1,
				}, nil
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		require.Error(t, err)
		assert.Contains(t, stdout.String(), "2 mandatory violations")
		assert.Contains(t, stdout.String(), "1 advisory violation")
	})

	t.Run("returns error from GetStackPolicyPacks", func(t *testing.T) {
		t.Parallel()

		policyErr := errors.New("policy service unavailable")
		cmd := policyCheckCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{
					GetStackPolicyPacksF: func(
						ctx context.Context, stackRef backend.StackReference,
					) ([]engine.RequiredPolicy, error) {
						return nil, policyErr
					},
				}, nonEmptySnap), nil
			},
		}

		err := cmd.Run(t.Context(), "", nil, nil)
		assert.ErrorIs(t, err, policyErr)
	})

	t.Run("returns error when snapshot load fails", func(t *testing.T) {
		t.Parallel()

		snapErr := errors.New("snapshot load failed")
		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return &backend.MockStack{
					BackendF: func() backend.Backend { return &backend.MockBackend{} },
					RefF:     func() backend.StackReference { return &backend.MockStackReference{StringV: "test-stack"} },
					SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
						return nil, snapErr
					},
				}, nil
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		assert.ErrorIs(t, err, snapErr)
		assert.ErrorContains(t, err, "loading stack snapshot")
	})

	t.Run("policy-pack-config without policy-pack errors", func(t *testing.T) {
		t.Parallel()

		cmd := policyCheckCmd{
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
		}

		err := cmd.Run(t.Context(), "", nil, []string{"/path/to/config.json"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--policy-pack-config must be specified for each --policy-pack")
	})

	t.Run("local policy packs passed to check", func(t *testing.T) {
		t.Parallel()

		var receivedOpts engine.UpdateOptions
		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				receivedOpts = opts
				return engine.CheckResult{Passed: true}, nil
			},
		}

		err := cmd.Run(t.Context(), "",
			[]string{"/path/to/pack-a", "/path/to/pack-b"},
			[]string{"/path/to/config-a.json", "/path/to/config-b.json"})
		require.NoError(t, err)
		require.Len(t, receivedOpts.LocalPolicyPacks, 2)
		assert.Equal(t, "/path/to/pack-a", receivedOpts.LocalPolicyPacks[0].Path)
		assert.Equal(t, "/path/to/config-a.json", receivedOpts.LocalPolicyPacks[0].Config)
		assert.Equal(t, "/path/to/pack-b", receivedOpts.LocalPolicyPacks[1].Path)
		assert.Equal(t, "/path/to/config-b.json", receivedOpts.LocalPolicyPacks[1].Config)
	})

	t.Run("returns error from check engine", func(t *testing.T) {
		t.Parallel()

		checkErr := errors.New("engine error")
		var stdout, stderr bytes.Buffer
		cmd := policyCheckCmd{
			stdout: &stdout,
			stderr: &stderr,
			requireStack: func(ctx context.Context, stackName string) (backend.Stack, error) {
				return newMockCheckStack(&backend.MockBackend{}, nonEmptySnap), nil
			},
			checkFunc: func(ctx context.Context, u engine.UpdateInfo, opts engine.UpdateOptions,
				snap *deploy.Snapshot, events chan<- engine.Event,
			) (engine.CheckResult, error) {
				return engine.CheckResult{}, checkErr
			},
		}

		err := cmd.Run(t.Context(), "", []string{"/path/to/pack"}, nil)
		assert.ErrorIs(t, err, checkErr)
		assert.ErrorContains(t, err, "policy check failed")
	})

	t.Run("pluralize helper", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "violation", pluralize("violation", 1))
		assert.Equal(t, "violations", pluralize("violation", 0))
		assert.Equal(t, "violations", pluralize("violation", 2))
		assert.Equal(t, "violations", pluralize("violation", 100))
	})
}
