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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:  "organization/project/stack",
				NameV:    tokens.MustParseStackName("stack"),
				ProjectV: "project",
			}
		},
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

// writeStateFile marshals the given resources into an UntypedDeployment JSON file
// (the shape `pulumi stack export` produces) and returns its path.
func writeStateFile(t *testing.T, resources []apitype.ResourceV3) string {
	t.Helper()
	return writeStateFileNamed(t, "state.json", apitype.DeploymentV3{Resources: resources})
}

// writeStateFileNamed marshals a DeploymentV3 into an UntypedDeployment JSON file with
// the given base name and returns its path.
func writeStateFileNamed(t *testing.T, name string, deployment apitype.DeploymentV3) string {
	t.Helper()
	raw, err := json.Marshal(deployment)
	require.NoError(t, err)
	data, err := json.Marshal(apitype.UntypedDeployment{Version: 3, Deployment: raw})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// encryptedSecret returns the serialized form `pulumi stack export` writes for an
// encrypted secret value.
func encryptedSecret(ciphertext string) apitype.SecretV1 {
	return apitype.SecretV1{Sig: resource.SecretSig, Ciphertext: ciphertext}
}

func TestPolicyAnalyzeCmd_File_NoViolationsSucceeds(t *testing.T) {
	t.Parallel()

	path := writeStateFile(t, []apitype.ResourceV3{{
		Type:    "pkg:index:MyResource",
		URN:     "urn:pulumi:stack::project::pkg:index:MyResource::res",
		Custom:  true,
		Outputs: map[string]any{"k": "v"},
	}})
	stdout, stderr, err := runAnalyzeCmd(t, nil, nil, stubLoadAnalyzers(nil), "--file", path, "--diff")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	// No secrets to redact: the fallback warning must not fire.
	assert.NotContains(t, stderr, "redacted")
}

func TestPolicyAnalyzeCmd_File_MandatoryViolationReturnsError(t *testing.T) {
	t.Parallel()

	path := writeStateFile(t, []apitype.ResourceV3{{
		Type:    "pkg:index:MyResource",
		URN:     "urn:pulumi:stack::project::pkg:index:MyResource::res",
		Custom:  true,
		Outputs: map[string]any{"k": "v"},
	}})
	analyzer := &fakeAnalyzer{mandatory: true}
	stdout, _, err := runAnalyzeCmd(t, nil, nil,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--file", path, "--diff")
	assert.ErrorContains(t, err, "mandatory policy violations")
	expected := "    test-pack@v [mandatory]  test-policy  (pkg:index:MyResource: res)test violation\n"
	assert.Equal(t, expected, stdout)
}

func TestPolicyAnalyzeCmd_File_MissingFile(t *testing.T) {
	t.Parallel()

	_, _, err := runAnalyzeCmd(t, nil, nil, stubLoadAnalyzers(nil),
		"--file", filepath.Join(t.TempDir(), "does-not-exist.json"), "--diff")
	assert.ErrorContains(t, err, "could not open")
}

func TestPolicyAnalyzeCmd_File_MalformedJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
	_, _, err := runAnalyzeCmd(t, nil, nil, stubLoadAnalyzers(nil), "--file", path, "--diff")
	assert.ErrorContains(t, err, "reading deployment from")
}

// A non-empty but structurally invalid resource URN is rejected with a clear error,
// rather than panicking deep in snapshot analysis where URNs are dereferenced.
func TestPolicyAnalyzeCmd_File_InvalidURNRejected(t *testing.T) {
	t.Parallel()

	path := writeStateFile(t, []apitype.ResourceV3{{
		Type:   "pkg:index:MyResource",
		URN:    "not-a-valid-urn",
		Custom: true,
	}})
	_, _, err := runAnalyzeCmd(t, nil, nil,
		stubLoadAnalyzers([]plugin.Analyzer{&fakeAnalyzer{}}),
		"--file", path, "--diff")
	assert.ErrorContains(t, err, "invalid URN")
}

func TestPolicyAnalyzeCmd_File_MutuallyExclusiveWithStack(t *testing.T) {
	t.Parallel()

	_, _, err := runAnalyzeCmd(t, nil, nil, stubLoadAnalyzers(nil),
		"--file", "state.json", "--stack", "my-stack")
	assert.ErrorContains(t, err, "[file stack]")
}

func TestPolicyAnalyzeCmd_File_EmptyDeploymentPrintsMessage(t *testing.T) {
	t.Parallel()

	path := writeStateFile(t, nil)
	_, stderr, err := runAnalyzeCmd(t, nil, nil, stubLoadAnalyzers(nil), "--file", path, "--diff")
	require.NoError(t, err)
	assert.Contains(t, stderr, "no resources")
}

// A state file exported from a query is a filtered subset: it can reference a
// provider, parent, or dependencies that are not themselves present. Analysis
// must tolerate these dangling references rather than fail.
func TestPolicyAnalyzeCmd_File_DanglingReferencesTolerated(t *testing.T) {
	t.Parallel()

	path := writeStateFile(t, []apitype.ResourceV3{{
		Type:         "pkg:index:MyResource",
		URN:          "urn:pulumi:stack::project::pkg:index:MyResource::res",
		Custom:       true,
		External:     true,
		Provider:     "urn:pulumi:stack::project::pulumi:providers:pkg::missing::00000000-0000-0000-0000-000000000000",
		Parent:       "urn:pulumi:stack::project::pkg:index:Missing::parent",
		Dependencies: []resource.URN{"urn:pulumi:stack::project::pkg:index:Missing::dep"},
		Outputs:      map[string]any{"k": "v"},
	}})
	_, _, err := runAnalyzeCmd(t, nil, nil,
		stubLoadAnalyzers([]plugin.Analyzer{&fakeAnalyzer{}}),
		"--file", path, "--diff")
	require.NoError(t, err)
}

// A file whose secrets can't be decrypted offline must still analyze, redacting them.
func TestPolicyAnalyzeCmd_File_UndecryptableSecretsRedacted(t *testing.T) {
	t.Parallel()

	path := writeStateFileNamed(t, "state.json", apitype.DeploymentV3{
		SecretsProviders: &apitype.SecretsProvidersV1{Type: "service"},
		Resources: []apitype.ResourceV3{{
			Type:   "pkg:index:MyResource",
			URN:    "urn:pulumi:stack::project::pkg:index:MyResource::res",
			Custom: true,
			Inputs: map[string]any{"password": encryptedSecret("v1:cannot-decrypt-offline")},
		}},
	})

	analyzer := &fakeAnalyzer{}
	_, stderr, err := runAnalyzeCmd(t, nil, nil,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--file", path, "--diff")
	require.NoError(t, err)
	assert.Contains(t, stderr, "secret values redacted")

	pw := analyzer.analyzeProperties["password"]
	require.True(t, pw.IsSecret())
	assert.Equal(t, "[secret]", pw.SecretValue().Element.StringValue())
}

// The progress (non-diff) display reads the synthetic ref's Name()/Project(); the other
// file-mode tests use --diff, which never touches it. Name and project come from the
// resource URN (prod/infra), not the file base (prod-export).
func TestPolicyAnalyzeCmd_File_ProgressDisplayUsesSyntheticRef(t *testing.T) {
	t.Parallel()

	path := writeStateFileNamed(t, "prod-export.json", apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{{
			Type:    "pkg:index:MyResource",
			URN:     "urn:pulumi:prod::infra::pkg:index:MyResource::res",
			Custom:  true,
			Outputs: map[string]any{"k": "v"},
		}},
	})

	analyzer := &fakeAnalyzer{mandatory: true}
	stdout, _, err := runAnalyzeCmd(t, nil, nil,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--file", path)
	assert.ErrorContains(t, err, "mandatory policy violations")
	assert.Contains(t, stdout, "infra-prod")
	assert.NotContains(t, stdout, "prod-export")
}

func TestStackReferenceForFile(t *testing.T) {
	t.Parallel()

	t.Run("derives the shared stack and project from the resource URNs", func(t *testing.T) {
		t.Parallel()

		ref := stackReferenceForFile(&deploy.Snapshot{
			Resources: []*resource.State{
				{URN: "urn:pulumi:prod::infra::pkg:index:MyResource::a"},
				{URN: "urn:pulumi:prod::infra::pkg:index:MyResource::b"},
			},
		})
		assert.Equal(t, "prod", ref.Name().String())
		project, ok := ref.Project()
		require.True(t, ok)
		assert.Equal(t, "infra", string(project))
	})

	t.Run("uses a neutral label for a heterogeneous selection", func(t *testing.T) {
		t.Parallel()

		// Resources spanning more than one project or stack — a common shape for a Pulumi
		// Insights export — must not be labeled with any single origin.
		for _, urns := range [][]resource.URN{
			{
				"urn:pulumi:prod::infra::pkg:index:MyResource::a",
				"urn:pulumi:prod::networking::pkg:index:MyResource::b", // differing project
			},
			{
				"urn:pulumi:prod::infra::pkg:index:MyResource::a",
				"urn:pulumi:dev::infra::pkg:index:MyResource::b", // differing stack
			},
		} {
			resources := make([]*resource.State, len(urns))
			for i, u := range urns {
				resources[i] = &resource.State{URN: u}
			}
			ref := stackReferenceForFile(&deploy.Snapshot{Resources: resources})
			assert.Equal(t, "multiple", ref.Name().String())
			project, ok := ref.Project()
			require.True(t, ok)
			assert.Equal(t, "multiple", string(project))
		}
	})

	t.Run("falls back to unknown when no valid URN is present", func(t *testing.T) {
		t.Parallel()

		ref := stackReferenceForFile(&deploy.Snapshot{
			Resources: []*resource.State{{URN: "not-a-urn"}},
		})
		assert.Equal(t, "unknown", ref.Name().String())
		assert.Equal(t, "<unknown>", ref.String())
		project, ok := ref.Project()
		require.True(t, ok)
		assert.Equal(t, "unknown", string(project))
	})
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
	// Message names the stack via its fully-qualified reference, not just the bare Name().
	assert.Contains(t, stderr, "Stack organization/project/stack has no resources")
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
	expected := "" +
		"    pulumi:pulumi:Stack project-stack  \n" +
		"Policies:\n" +
		"    ❌ test-pack@v\n" +
		"        - [mandatory]  test-policy  (pkg:index:MyResource: res)\n" +
		"          test violation\n\n"
	assert.Equal(t, expected, stdout)
}

func TestPolicyAnalyzeCmd_JSONDisplayWritesPolicyEvents(t *testing.T) {
	t.Parallel()

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return makeAnalyzeSnapshot(), nil
	}
	ws, lm := newMockWsAndLm(be)

	analyzer := &fakeAnalyzer{mandatory: true}
	stdout, _, err := runAnalyzeCmd(t, ws, lm,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack", "--json")
	assert.ErrorContains(t, err, "mandatory policy violations")

	events := decodeEngineEventsJSON(t, stdout)
	require.Len(t, events, 2)

	assert.Equal(t, 0, events[0].Sequence)
	require.NotNil(t, events[0].PolicyEvent)
	assert.Equal(t, "test-policy", events[0].PolicyEvent.PolicyName)
	assert.Equal(t, "test-pack", events[0].PolicyEvent.PolicyPackName)
	assert.Equal(t, "mandatory", events[0].PolicyEvent.EnforcementLevel)
	assert.Equal(t, "test violation", events[0].PolicyEvent.Message)

	assert.Equal(t, 1, events[1].Sequence)
	require.NotNil(t, events[1].SummaryEvent)
	version, ok := events[1].SummaryEvent.PolicyPacks["test-pack"]
	require.True(t, ok)
	assert.Equal(t, "", version)
}

func TestPolicyAnalyzeCmd_AnalyzeStackCalled_PrintsAndUsesOutputs(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:    tokens.Type("pkg:index:MyResource"),
			URN:     urn,
			Custom:  true,
			Inputs:  resource.PropertyMap{"origin": resource.NewProperty("input")},
			Outputs: resource.PropertyMap{"origin": resource.NewProperty("output")},
		}},
	}

	be, stk := newMockBackendForAnalyze()
	stk.SnapshotF = func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
		return snap, nil
	}
	ws, lm := newMockWsAndLm(be)

	analyzer := &fakeAnalyzer{
		stackDiagnostic: &plugin.AnalyzeDiagnostic{
			PolicyName:       "stack-policy",
			PolicyPackName:   "test-pack",
			EnforcementLevel: apitype.Mandatory,
			Message:          "stack-level violation",
			URN:              urn,
		},
	}

	stdout, _, err := runAnalyzeCmd(t, ws, lm,
		stubLoadAnalyzers([]plugin.Analyzer{analyzer}),
		"--stack", "my-stack", "--diff")
	assert.ErrorContains(t, err, "mandatory policy violations")
	assert.True(t, analyzer.analyzeStackCalled)
	expected := "    test-pack@v [mandatory]  stack-policy  (pkg:index:MyResource: res)stack-level violation\n"
	assert.Equal(t, expected, stdout)

	got, ok := analyzer.analyzeStackProperties["origin"]
	require.True(t, ok)
	assert.True(t, got.DeepEquals(resource.NewProperty("output")))
	assert.False(t, got.DeepEquals(resource.NewProperty("input")))
}

func decodeEngineEventsJSON(t *testing.T, out string) []apitype.EngineEvent {
	t.Helper()

	dec := json.NewDecoder(strings.NewReader(out))
	var events []apitype.EngineEvent
	for {
		var event apitype.EngineEvent
		err := dec.Decode(&event)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		events = append(events, event)
	}

	// Ensure there is no trailing non-whitespace junk.
	require.False(t, dec.More())

	return events
}

// fakeAnalyzer is a minimal plugin.Analyzer for use in command-level tests.
type fakeAnalyzer struct {
	mandatory              bool
	remediate              bool
	stackDiagnostic        *plugin.AnalyzeDiagnostic
	analyzeStackCalled     bool
	analyzeStackProperties resource.PropertyMap
	analyzeProperties      resource.PropertyMap
}

func (a *fakeAnalyzer) Analyze(_ context.Context, r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	a.analyzeProperties = r.Properties.Copy()
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

func (a *fakeAnalyzer) AnalyzeStack(
	_ context.Context, resources []plugin.AnalyzerStackResource,
) (plugin.AnalyzeResponse, error) {
	a.analyzeStackCalled = true
	if len(resources) > 0 {
		a.analyzeStackProperties = resources[0].Properties.Copy()
	}
	if a.stackDiagnostic != nil {
		return plugin.AnalyzeResponse{
			Diagnostics: []plugin.AnalyzeDiagnostic{*a.stackDiagnostic},
		}, nil
	}
	return plugin.AnalyzeResponse{}, nil
}

func (a *fakeAnalyzer) Remediate(_ context.Context, _ plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
	if a.remediate {
		return plugin.RemediateResponse{
			Remediations: []plugin.Remediation{{
				PolicyName:        "test-remediation",
				PolicyPackName:    "test-pack",
				PolicyPackVersion: "1.0.0",
				Description:       "fixes a property",
				Properties:        resource.PropertyMap{"k": resource.NewProperty("fixed")},
			}},
		}, nil
	}
	return plugin.RemediateResponse{}, nil
}

func (a *fakeAnalyzer) GetAnalyzerInfo(context.Context) (plugin.AnalyzerInfo, error) {
	return plugin.AnalyzerInfo{Name: "test-pack"}, nil
}
func (a *fakeAnalyzer) Name() tokens.QName { return "test-pack" }
func (a *fakeAnalyzer) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{}, nil
}

func (a *fakeAnalyzer) Configure(context.Context, map[string]plugin.AnalyzerPolicyConfig) error {
	return nil
}
func (a *fakeAnalyzer) Cancel(context.Context) error { return nil }
func (a *fakeAnalyzer) Close() error                 { return nil }
