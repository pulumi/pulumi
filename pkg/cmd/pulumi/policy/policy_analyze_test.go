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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// noopCleanup is a no-op cleanup function for injecting into tests.
func noopCleanup() {}

// stubLoadAnalyzers returns a loadAnalyzers function that yields the given analyzers.
func stubLoadAnalyzers(analyzers []plugin.Analyzer) func(
	context.Context, []engine.LocalPolicyPack,
) ([]plugin.Analyzer, func(), error) {
	return func(_ context.Context, _ []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
		return analyzers, noopCleanup, nil
	}
}

// stubSnapshot returns a getSnapshot function yielding the given snapshot.
func stubSnapshot(snap *deploy.Snapshot) func(context.Context, backend.Stack) (*deploy.Snapshot, error) {
	return func(_ context.Context, _ backend.Stack) (*deploy.Snapshot, error) {
		return snap, nil
	}
}

// makeSnapshot builds a minimal snapshot containing a single resource.
func makeSnapshot() *deploy.Snapshot {
	return &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:    tokens.Type("pkg:index:MyResource"),
			URN:     "urn:pulumi:stack::project::pkg:index:MyResource::res",
			Custom:  true,
			Outputs: resource.PropertyMap{"k": resource.NewProperty("v")},
		}},
	}
}

// stubRunAnalysis returns a runAnalysis function that records calls and returns the given result.
func stubRunAnalysis(hasMandatory bool, err error) func(
	context.Context, *deploy.Snapshot, []plugin.Analyzer, deploy.PolicyEvents,
) (bool, error) {
	return func(_ context.Context, _ *deploy.Snapshot, _ []plugin.Analyzer, _ deploy.PolicyEvents) (bool, error) {
		return hasMandatory, err
	}
}

func TestPolicyAnalyzeCmd_RequiresPolicyPackFlag(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{stderr: &stderr}

	err := cmd.Run(t.Context(), "", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack")
}

func TestPolicyAnalyzeCmd_ConfigCountMustMatchPackCount(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{stderr: &stderr}

	err := cmd.Run(t.Context(), "", []string{"./my-pack"}, []string{"a.json", "b.json"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack-config")
}

func TestPolicyAnalyzeCmd_UsesCurrentStackWhenNoneSpecified(t *testing.T) {
	t.Parallel()

	var requestedStack string
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			requestedStack = name
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(makeSnapshot()),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis:   stubRunAnalysis(false, nil),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.NoError(t, err)
	assert.Empty(t, requestedStack, "empty stack name means use current stack")
}

func TestPolicyAnalyzeCmd_UsesSpecifiedStack(t *testing.T) {
	t.Parallel()

	var requestedStack string
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			requestedStack = name
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(makeSnapshot()),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis:   stubRunAnalysis(false, nil),
	}

	err := cmd.Run(t.Context(), "my-stack", []string{"./pack"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "my-stack", requestedStack)
}

func TestPolicyAnalyzeCmd_FriendlyErrorWhenNoProjectAndNoStack(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return nil, fmt.Errorf("loading: %w", workspace.ErrProjectNotFound)
		},
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--stack flag")
	assert.NotErrorIs(t, err, workspace.ErrProjectNotFound)
}

func TestPolicyAnalyzeCmd_OriginalErrorWhenStackSpecifiedAndNotFound(t *testing.T) {
	t.Parallel()

	expectedErr := workspace.ErrProjectNotFound
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return nil, expectedErr
		},
	}

	err := cmd.Run(t.Context(), "my-stack", []string{"./pack"}, nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestPolicyAnalyzeCmd_ErrorOnStackNotFound(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("stack not found")
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return nil, expectedErr
		},
	}

	err := cmd.Run(t.Context(), "bad-stack", []string{"./pack"}, nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestPolicyAnalyzeCmd_ErrorOnSnapshotFailure(t *testing.T) {
	t.Parallel()

	snapErr := errors.New("cannot load snapshot")
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot: func(ctx context.Context, s backend.Stack) (*deploy.Snapshot, error) {
			return nil, snapErr
		},
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, snapErr)
	assert.ErrorContains(t, err, "loading stack snapshot")
}

func TestPolicyAnalyzeCmd_EmptySnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot: stubSnapshot(&deploy.Snapshot{}),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "no resources")
}

func TestPolicyAnalyzeCmd_NilSnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot: stubSnapshot(nil),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "no resources")
}

func TestPolicyAnalyzeCmd_ErrorOnLoadAnalyzersFailure(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("pack not found")
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot: stubSnapshot(makeSnapshot()),
		loadAnalyzers: func(ctx context.Context, packs []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
			return nil, nil, loadErr
		},
	}

	err := cmd.Run(t.Context(), "", []string{"./bad-pack"}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, loadErr)
	assert.ErrorContains(t, err, "loading policy packs")
}

func TestPolicyAnalyzeCmd_ErrorOnAnalysisFailure(t *testing.T) {
	t.Parallel()

	analysisErr := errors.New("analysis failed")
	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(makeSnapshot()),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis:   stubRunAnalysis(false, analysisErr),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, analysisErr)
	assert.ErrorContains(t, err, "running policy analysis")
}

func TestPolicyAnalyzeCmd_MandatoryViolationReturnsError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(makeSnapshot()),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis:   stubRunAnalysis(true, nil),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mandatory policy violations")
}

func TestPolicyAnalyzeCmd_NoViolationsSucceeds(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(makeSnapshot()),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis:   stubRunAnalysis(false, nil),
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.NoError(t, err)
}

func TestPolicyAnalyzeCmd_SnapshotPassedToAnalysis(t *testing.T) {
	t.Parallel()

	snap := makeSnapshot()
	var gotSnap *deploy.Snapshot

	var stderr bytes.Buffer
	cmd := policyAnalyzeCmd{
		stderr: &stderr,
		requireStack: func(ctx context.Context, name string) (backend.Stack, error) {
			return newMockStack(&backend.MockBackend{}), nil
		},
		getSnapshot:   stubSnapshot(snap),
		loadAnalyzers: stubLoadAnalyzers(nil),
		runAnalysis: func(
			ctx context.Context,
			s *deploy.Snapshot,
			analyzers []plugin.Analyzer,
			events deploy.PolicyEvents,
		) (bool, error) {
			gotSnap = s
			return false, nil
		},
	}

	err := cmd.Run(t.Context(), "", []string{"./pack"}, nil)
	require.NoError(t, err)
	assert.Same(t, snap, gotSnap, "the snapshot returned by getSnapshot should be passed to runAnalysis")
}
