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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
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

// newMockBackendForAnalyze returns a wired-up MockBackend and MockStack. Tests
// must pass "--stack", "<name>" to take the simpler named-stack code path.
// Set stk.SnapshotF before running the command to control snapshot behavior.
func newMockBackendForAnalyze() (*backend.MockBackend, *backend.MockStack) {
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
// dependencies, attaches extra args, runs it, and returns stdout, stderr, and
// the error.
// All tests that reach stack loading must pass "--stack", "<name>" in extraArgs.
func runAnalyzeCmd(
	t *testing.T,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	loadAnalyzers func(context.Context, []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error),
	extraArgs ...string,
) (string, string, error) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newPolicyAnalyzeCmd(ws, lm, nil, loadAnalyzers)
	cmd.SilenceUsage = true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	args := append([]string{"--policy-pack", "./pack"}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

func TestPolicyAnalyzeCmd_RequiresPolicyPackFlag(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil, nil)
	cmd.SetArgs([]string{}) // no --policy-pack
	err := cmd.ExecuteContext(t.Context())
	assert.ErrorContains(t, err, "--policy-pack")
}

func TestPolicyAnalyzeCmd_ConfigCountMustMatchPackCount(t *testing.T) {
	t.Parallel()

	cmd := newPolicyAnalyzeCmd(nil, nil, nil, nil)
	cmd.SetArgs([]string{"--policy-pack", "./pack", "--policy-pack-config", "a.json", "--policy-pack-config", "b.json"})
	err := cmd.ExecuteContext(t.Context())
	assert.ErrorContains(t, err, "--policy-pack-config")
}

func TestPolicyAnalyzeCmd_ErrorOnSnapshotFailure(t *testing.T) {
	t.Parallel()

	snapErr := errors.New("cannot load snapshot")
	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return nil, snapErr
	}
	ws, lm := newMockWsAndLm(be)
	_, _, err := runAnalyzeCmd(t, ws, lm, nil, "--stack", "my-stack")
	assert.ErrorIs(t, err, snapErr)
	assert.ErrorContains(t, err, "loading stack snapshot")
}

func TestPolicyAnalyzeCmd_EmptySnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return &deploy.Snapshot{}, nil
	}
	ws, lm := newMockWsAndLm(be)
	_, stderr, err := runAnalyzeCmd(t, ws, lm, nil, "--stack", "my-stack")
	require.NoError(t, err)
	assert.Contains(t, stderr, "no resources")
}

func TestPolicyAnalyzeCmd_NilSnapshotPrintsMessage(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return nil, nil
	}
	ws, lm := newMockWsAndLm(be)
	_, stderr, err := runAnalyzeCmd(t, ws, lm, nil, "--stack", "my-stack")
	require.NoError(t, err)
	assert.Contains(t, stderr, "no resources")
}

func TestPolicyAnalyzeCmd_ErrorOnLoadAnalyzersFailure(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)
	loadErr := errors.New("pack not found")
	loadAnalyzers := func(_ context.Context, _ []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
		return nil, nil, loadErr
	}
	_, _, err := runAnalyzeCmd(t, ws, lm, loadAnalyzers, "--stack", "my-stack")
	assert.ErrorIs(t, err, loadErr)
	assert.ErrorContains(t, err, "loading policy packs")
}

func TestPolicyAnalyzeCmd_MandatoryViolationReturnsError(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)
	analyzer := &fakeAnalyzer{mandatory: true}
	stdout, _, err := runAnalyzeCmd(t, ws, lm,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack", "--diff")
	assert.ErrorContains(t, err, "mandatory policy violations")
	expected := "    test-pack@v [mandatory]  test-policy  (pkg:index:MyResource: res)test violation\n"
	assert.Equal(t, expected, stdout)
}

func TestPolicyAnalyzeCmd_NoViolationsSucceeds(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)
	stdout, _, err := runAnalyzeCmd(t, ws, lm, stubLoadAnalyzers(nil), "--stack", "my-stack", "--diff")
	require.NoError(t, err)
	assert.Empty(t, stdout)
}

func TestPolicyAnalyzeCmd_RemediationWritesToOutputStream(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)

	analyzer := &fakeAnalyzer{remediate: true}
	stdout, _, err := runAnalyzeCmd(t, ws, lm,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack", "--diff")
	require.NoError(t, err)
	expected := "    test-pack@v1.0.0 [remediate]  test-remediation  (pkg:index:MyResource: res)\n" +
		"      + k: \"fixed\"\n\n"
	assert.Equal(t, expected, stdout)
}

func TestPolicyAnalyzeCmd_ProgressDisplayGroupsPolicyOutput(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)

	analyzer := &fakeAnalyzer{mandatory: true}
	stdout, _, err := runAnalyzeCmd(t, ws, lm,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack")
	assert.ErrorContains(t, err, "mandatory policy violations")
	assert.Contains(t, stdout, "Policies:")
	assert.Contains(t, stdout, "test-pack@v")
	assert.Contains(t, stdout, "test-policy")
}

// fakeAnalyzer is a minimal plugin.Analyzer for use in command-level tests.
type fakeAnalyzer struct {
	mandatory bool
	remediate bool
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
	if a.remediate {
		return plugin.RemediateResponse{
			Remediations: []plugin.Remediation{{
				PolicyName:        "test-remediation",
				PolicyPackName:    "test-pack",
				PolicyPackVersion: "1.0.0",
				Description:       "fixes a property",
				Properties:        resource.PropertyMap{"k": resource.NewStringProperty("fixed")},
			}},
		}, nil
	}
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
