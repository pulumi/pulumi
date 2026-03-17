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

// makeAnalyzeSnapshot builds a minimal snapshot with a single resource.
func makeAnalyzeSnapshot() *deploy.Snapshot {
	return &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:    tokens.Type("pkg:index:MyResource"),
			URN:     "urn:pulumi:stack::project::pkg:index:MyResource::res",
			Custom:  true,
			Outputs: resource.PropertyMap{"k": resource.NewProperty("v")},
		}},
	}
}

// stubRequireStack returns a requireStack func that yields the given stack.
func stubRequireStack(s backend.Stack) func(context.Context, string) (backend.Stack, error) {
	return func(_ context.Context, _ string) (backend.Stack, error) { return s, nil }
}

// stubGetSnapshot returns a getSnapshot func that yields the given snapshot.
func stubGetSnapshot(snap *deploy.Snapshot) func(context.Context, backend.Stack) (*deploy.Snapshot, error) {
	return func(_ context.Context, _ backend.Stack) (*deploy.Snapshot, error) { return snap, nil }
}

// stubLoadAnalyzers returns a loadAnalyzers func that yields the given analyzers.
func stubLoadAnalyzers(analyzers []plugin.Analyzer) func(context.Context, []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
	return func(_ context.Context, _ []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
		return analyzers, func() {}, nil
	}
}

// runAnalyzeCmd is a helper that sets up a newPolicyAnalyzeCmd with the given
// injectable dependencies, attaches extra args, runs it, and returns the error.
func runAnalyzeCmd(
	t *testing.T,
	requireStack func(context.Context, string) (backend.Stack, error),
	getSnapshot func(context.Context, backend.Stack) (*deploy.Snapshot, error),
	loadAnalyzers func(context.Context, []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error),
	extraArgs ...string,
) (string, error) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := newPolicyAnalyzeCmd(requireStack, getSnapshot, loadAnalyzers)
	cmd.SetErr(&stderr)
	args := append([]string{"--policy-pack", "./pack"}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stderr.String(), err
}

func TestPolicyAnalyzeCmd_RequiresPolicyPackFlag(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil)
	cmd.SetArgs([]string{}) // no --policy-pack
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack")
}

func TestPolicyAnalyzeCmd_ConfigCountMustMatchPackCount(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil)
	cmd.SetArgs([]string{"--policy-pack", "./pack", "--policy-pack-config", "a.json", "--policy-pack-config", "b.json"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack-config")
}

func TestPolicyAnalyzeCmd_UsesCurrentStackWhenNoneSpecified(t *testing.T) {
	t.Parallel()

	var requestedStack string
	requireStack := func(_ context.Context, name string) (backend.Stack, error) {
		requestedStack = name
		return newMockStack(&backend.MockBackend{}), nil
	}
	_, err := runAnalyzeCmd(t, requireStack, stubGetSnapshot(makeAnalyzeSnapshot()), stubLoadAnalyzers(nil))
	require.NoError(t, err)
	assert.Empty(t, requestedStack, "empty stack name means use current stack")
}

func TestPolicyAnalyzeCmd_UsesSpecifiedStack(t *testing.T) {
	t.Parallel()

	var requestedStack string
	requireStack := func(_ context.Context, name string) (backend.Stack, error) {
		requestedStack = name
		return newMockStack(&backend.MockBackend{}), nil
	}
	_, err := runAnalyzeCmd(t, requireStack, stubGetSnapshot(makeAnalyzeSnapshot()), stubLoadAnalyzers(nil), "--stack", "my-stack")
	require.NoError(t, err)
	assert.Equal(t, "my-stack", requestedStack)
}

func TestPolicyAnalyzeCmd_FriendlyErrorWhenNoProjectAndNoStack(t *testing.T) {
	t.Parallel()

	requireStack := func(_ context.Context, _ string) (backend.Stack, error) {
		return nil, fmt.Errorf("loading: %w", workspace.ErrProjectNotFound)
	}
	_, err := runAnalyzeCmd(t, requireStack, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--stack flag")
	assert.NotErrorIs(t, err, workspace.ErrProjectNotFound)
}

func TestPolicyAnalyzeCmd_OriginalErrorWhenStackSpecifiedAndNotFound(t *testing.T) {
	t.Parallel()

	requireStack := func(_ context.Context, _ string) (backend.Stack, error) {
		return nil, workspace.ErrProjectNotFound
	}
	_, err := runAnalyzeCmd(t, requireStack, nil, nil, "--stack", "my-stack")
	assert.ErrorIs(t, err, workspace.ErrProjectNotFound)
}

func TestPolicyAnalyzeCmd_ErrorOnStackNotFound(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("stack not found")
	requireStack := func(_ context.Context, _ string) (backend.Stack, error) { return nil, expectedErr }
	_, err := runAnalyzeCmd(t, requireStack, nil, nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestPolicyAnalyzeCmd_ErrorOnSnapshotFailure(t *testing.T) {
	t.Parallel()

	snapErr := errors.New("cannot load snapshot")
	getSnapshot := func(_ context.Context, _ backend.Stack) (*deploy.Snapshot, error) { return nil, snapErr }
	_, err := runAnalyzeCmd(t, stubRequireStack(newMockStack(&backend.MockBackend{})), getSnapshot, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, snapErr)
	assert.ErrorContains(t, err, "loading stack snapshot")
}

func TestPolicyAnalyzeCmd_EmptySnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	out, err := runAnalyzeCmd(t,
		stubRequireStack(newMockStack(&backend.MockBackend{})),
		stubGetSnapshot(&deploy.Snapshot{}),
		nil)
	require.NoError(t, err)
	assert.Contains(t, out, "no resources")
}

func TestPolicyAnalyzeCmd_NilSnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	out, err := runAnalyzeCmd(t,
		stubRequireStack(newMockStack(&backend.MockBackend{})),
		stubGetSnapshot(nil),
		nil)
	require.NoError(t, err)
	assert.Contains(t, out, "no resources")
}

func TestPolicyAnalyzeCmd_ErrorOnLoadAnalyzersFailure(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("pack not found")
	loadAnalyzers := func(_ context.Context, _ []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
		return nil, nil, loadErr
	}
	_, err := runAnalyzeCmd(t,
		stubRequireStack(newMockStack(&backend.MockBackend{})),
		stubGetSnapshot(makeAnalyzeSnapshot()),
		loadAnalyzers)
	require.Error(t, err)
	assert.ErrorIs(t, err, loadErr)
	assert.ErrorContains(t, err, "loading policy packs")
}

func TestPolicyAnalyzeCmd_MandatoryViolationReturnsError(t *testing.T) {
	t.Parallel()

	// Use a real deploytest.Analyzer that returns a mandatory violation so that
	// the command runs through deploy.AnalyzeSnapshot end-to-end.
	analyzer := &fakeAnalyzer{mandatory: true}
	_, err := runAnalyzeCmd(t,
		stubRequireStack(newMockStack(&backend.MockBackend{})),
		stubGetSnapshot(makeAnalyzeSnapshot()),
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mandatory policy violations")
}

func TestPolicyAnalyzeCmd_NoViolationsSucceeds(t *testing.T) {
	t.Parallel()

	_, err := runAnalyzeCmd(t,
		stubRequireStack(newMockStack(&backend.MockBackend{})),
		stubGetSnapshot(makeAnalyzeSnapshot()),
		stubLoadAnalyzers(nil))
	require.NoError(t, err)
}

// fakeAnalyzer is a minimal plugin.Analyzer for use in command-level tests.
type fakeAnalyzer struct {
	mandatory bool
}

func (a *fakeAnalyzer) Analyze(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	if a.mandatory {
		return plugin.AnalyzeResponse{
			Diagnostics: []plugin.AnalyzeDiagnostic{{
				PolicyName:       "test-policy",
				PolicyPackName:   "test-pack",
				EnforcementLevel: "mandatory",
				Message:          "test violation",
				URN:              r.URN,
			}},
		}, nil
	}
	return plugin.AnalyzeResponse{}, nil
}

func (a *fakeAnalyzer) AnalyzeStack(_ []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
	return plugin.AnalyzeResponse{}, nil
}
func (a *fakeAnalyzer) Remediate(_ plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
	return plugin.RemediateResponse{}, nil
}
func (a *fakeAnalyzer) GetAnalyzerInfo() (plugin.AnalyzerInfo, error) {
	return plugin.AnalyzerInfo{Name: "test-pack"}, nil
}
func (a *fakeAnalyzer) Name() tokens.QName                       { return "test-pack" }
func (a *fakeAnalyzer) GetPluginInfo() (plugin.PluginInfo, error) { return plugin.PluginInfo{}, nil }
func (a *fakeAnalyzer) Configure(_ map[string]plugin.AnalyzerPolicyConfig) error { return nil }
func (a *fakeAnalyzer) Cancel(_ context.Context) error                           { return nil }
func (a *fakeAnalyzer) Close() error                                             { return nil }
