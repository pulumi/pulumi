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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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

// newMockBackendForAnalyze returns a wired-up MockBackend and MockStack for use in
// policy analyze tests. Tests must pass "--stack", "<name>" to take the simpler
// named-stack code path, which only requires ParseStackReferenceF and GetStackF.
func newMockBackendForAnalyze() (*backend.MockBackend, backend.Stack) {
	var be *backend.MockBackend
	stk := &backend.MockStack{
		BackendF: func() backend.Backend { return be },
		RefF:     func() backend.StackReference { return &backend.MockStackReference{} },
	}
	be = &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{}, nil
		},
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return stk, nil
		},
	}
	return be, stk
}

// newMockWsAndLm returns a MockContext and MockLoginManager that route through be.
func newMockWsAndLm(be backend.Backend) (pkgWorkspace.Context, cmdBackend.LoginManager) {
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			_ string, _ *workspace.Project, _ bool, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	return ws, lm
}

// stubGetSnapshot returns a getSnapshot func that yields the given snapshot.
func stubGetSnapshot(snap *deploy.Snapshot) func(context.Context, backend.Stack) (*deploy.Snapshot, error) {
	return func(_ context.Context, _ backend.Stack) (*deploy.Snapshot, error) { return snap, nil }
}

// stubLoadAnalyzers returns a loadAnalyzers func that yields the given analyzers.
func stubLoadAnalyzers(
	analyzers []plugin.Analyzer,
) func(context.Context, []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
	return func(
		_ context.Context, _ []engine.LocalPolicyPack,
	) ([]plugin.Analyzer, func(), error) {
		return analyzers, func() {}, nil
	}
}

// runAnalyzeCmd is a helper that builds a newPolicyAnalyzeCmd with the given
// dependencies, attaches extra args, runs it, and returns stderr and the error.
// All tests that reach stack loading must pass "--stack", "<name>" in extraArgs.
func runAnalyzeCmd(
	t *testing.T,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	getSnapshot func(context.Context, backend.Stack) (*deploy.Snapshot, error),
	loadAnalyzers func(context.Context, []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error),
	extraArgs ...string,
) (string, error) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := newPolicyAnalyzeCmd(ws, lm, getSnapshot, loadAnalyzers)
	cmd.SetErr(&stderr)
	args := append([]string{"--policy-pack", "./pack"}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stderr.String(), err
}

func TestPolicyAnalyzeCmd_RequiresPolicyPackFlag(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil, nil)
	cmd.SetArgs([]string{}) // no --policy-pack
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack")
}

func TestPolicyAnalyzeCmd_ConfigCountMustMatchPackCount(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil, nil)
	cmd.SetArgs([]string{"--policy-pack", "./pack", "--policy-pack-config", "a.json", "--policy-pack-config", "b.json"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy-pack-config")
}

func TestPolicyAnalyzeCmd_ErrorOnSnapshotFailure(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	snapErr := errors.New("cannot load snapshot")
	getSnapshot := func(_ context.Context, _ backend.Stack) (*deploy.Snapshot, error) { return nil, snapErr }
	_, err := runAnalyzeCmd(t, ws, lm, getSnapshot, nil, "--stack", "my-stack")
	require.Error(t, err)
	assert.ErrorIs(t, err, snapErr)
	assert.ErrorContains(t, err, "loading stack snapshot")
}

func TestPolicyAnalyzeCmd_EmptySnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	out, err := runAnalyzeCmd(t, ws, lm, stubGetSnapshot(&deploy.Snapshot{}), nil, "--stack", "my-stack")
	require.NoError(t, err)
	assert.Contains(t, out, "no resources")
}

func TestPolicyAnalyzeCmd_NilSnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	out, err := runAnalyzeCmd(t, ws, lm, stubGetSnapshot(nil), nil, "--stack", "my-stack")
	require.NoError(t, err)
	assert.Contains(t, out, "no resources")
}

func TestPolicyAnalyzeCmd_ErrorOnLoadAnalyzersFailure(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	loadErr := errors.New("pack not found")
	loadAnalyzers := func(_ context.Context, _ []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
		return nil, nil, loadErr
	}
	_, err := runAnalyzeCmd(t, ws, lm, stubGetSnapshot(makeAnalyzeSnapshot()), loadAnalyzers, "--stack", "my-stack")
	require.Error(t, err)
	assert.ErrorIs(t, err, loadErr)
	assert.ErrorContains(t, err, "loading policy packs")
}

func TestPolicyAnalyzeCmd_MandatoryViolationReturnsError(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	analyzer := &fakeAnalyzer{mandatory: true}
	_, err := runAnalyzeCmd(t, ws, lm,
		stubGetSnapshot(makeAnalyzeSnapshot()),
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mandatory policy violations")
}

func TestPolicyAnalyzeCmd_NoViolationsSucceeds(t *testing.T) {
	t.Parallel()

	be, _ := newMockBackendForAnalyze()
	ws, lm := newMockWsAndLm(be)
	_, err := runAnalyzeCmd(t, ws, lm,
		stubGetSnapshot(makeAnalyzeSnapshot()),
		stubLoadAnalyzers(nil),
		"--stack", "my-stack")
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
func (a *fakeAnalyzer) Name() tokens.QName                                       { return "test-pack" }
func (a *fakeAnalyzer) GetPluginInfo() (plugin.PluginInfo, error)                { return plugin.PluginInfo{}, nil }
func (a *fakeAnalyzer) Configure(_ map[string]plugin.AnalyzerPolicyConfig) error { return nil }
func (a *fakeAnalyzer) Cancel(_ context.Context) error                           { return nil }
func (a *fakeAnalyzer) Close() error                                             { return nil }
