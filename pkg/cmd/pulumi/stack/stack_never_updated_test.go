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

package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgstack "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestRunStackText_NeverUpdatedOmitsLastUpdated(t *testing.T) {
	t.Parallel()

	// Shape of a checkpoint written by `pulumi stack init` on a self-managed backend:
	// a manifest time and CLI version, a secrets provider, and no resources. History
	// is empty because init is not an update.
	updatedAt := time.Date(2026, 10, 7, 23, 19, 10, 0, time.UTC)
	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    updatedAt,
			Version: "v3.265.0",
		},
		SecretsManager: &secrets.MockSecretsManager{
			TypeF:  func() string { return "passphrase" },
			StateF: func() json.RawMessage { return json.RawMessage(`{}`) },
		},
	}

	var buf bytes.Buffer
	err := runStackText(t.Context(), newMockStack(snap, nil), &buf, stackArgs{})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "No updates yet; run `pulumi up`")
	assert.Contains(t, out, "Secrets provider: passphrase")
	assert.NotContains(t, out, "Last updated:")
	assert.NotContains(t, out, "Pulumi version used:")
}

func TestRunStackText_DeployedStackStillShowsLastUpdated(t *testing.T) {
	t.Parallel()

	updatedAt := time.Now().Add(-2 * time.Hour)
	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    updatedAt,
			Version: "v3.265.0",
		},
		Resources: []*pkgresource.State{
			{
				Type: resource.RootStackType,
				URN:  resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev"),
				Outputs: resource.PropertyMap{
					"greeting": resource.NewProperty("hello"),
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runStackText(t.Context(), newMockStack(snap, nil), &buf, stackArgs{})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Last updated:")
	assert.Contains(t, out, "Pulumi version used: v3.265.0")
	assert.NotContains(t, out, "No updates yet")
	assert.Contains(t, out, "hello")
}

func TestRunStackText_DestroyedStackStillShowsLastUpdated(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    time.Now().Add(-time.Hour),
			Version: "v3.265.0",
		},
	}
	s := newMockStack(snap, nil).(*backend.MockStack)
	s.BackendF = func() backend.Backend {
		return &backend.MockBackend{
			NameF: func() string { return "mock://test" },
			GetHistoryF: func(context.Context, backend.StackReference, int, int) ([]backend.UpdateInfo, error) {
				return []backend.UpdateInfo{{Kind: apitype.DestroyUpdate}}, nil
			},
		}
	}

	var buf bytes.Buffer
	err := runStackText(t.Context(), s, &buf, stackArgs{})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Last updated:")
	assert.Contains(t, out, "Pulumi version used: v3.265.0")
	assert.NotContains(t, out, "No updates yet")
}

func TestRunStackText_NilSnapshotSaysNoUpdates(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runStackText(t.Context(), newMockStack(nil, nil), &buf, stackArgs{})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "No updates yet; run `pulumi up`")
	assert.NotContains(t, out, "Last updated:")
}

func TestStackOutput_NeverDeployedNamesDeployment(t *testing.T) {
	t.Parallel()

	// Cloud never-deployed stacks have no snapshot. Asking for one output must not
	// claim the property is missing from a deployment.
	_, err := runStackOutput(t, newMockStack(nil, nil), "greeting")
	require.EqualError(t, err, neverDeployedOutputsMessage)
	assert.NotContains(t, err.Error(), "does not have output property")
}

func TestStackOutput_InitCheckpointNamesDeployment(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    time.Now(),
			Version: "v3.265.0",
		},
	}
	_, err := runStackOutput(t, newMockStack(snap, nil), "greeting")
	require.EqualError(t, err, neverDeployedOutputsMessage)
}

func TestStackOutput_DeployedStackKeepsMissingPropertyError(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{Time: time.Now().Add(-time.Hour), Version: "v3.265.0"},
		Resources: []*pkgresource.State{
			{
				Type: resource.RootStackType,
				URN:  resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev"),
				Outputs: resource.PropertyMap{
					"other": resource.NewProperty("value"),
				},
			},
		},
	}

	stdout, err := runStackOutput(t, newMockStack(snap, nil), "greeting")
	require.EqualError(t, err, "current stack does not have output property 'greeting'")
	assert.Empty(t, stdout)

	stdout, err = runStackOutput(t, newMockStack(snap, nil), "other")
	require.NoError(t, err)
	assert.Equal(t, "value\n", stdout)
}

func TestStackOutput_DeployedStackWithNoOutputsKeepsMissingPropertyError(t *testing.T) {
	t.Parallel()

	// A successful update that exported nothing has a stack resource and no outputs.
	// That is not the same as a stack that has never been deployed.
	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{Time: time.Now().Add(-time.Hour)},
		Resources: []*pkgresource.State{
			{
				Type:    resource.RootStackType,
				URN:     resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev"),
				Outputs: resource.PropertyMap{},
			},
		},
	}

	_, err := runStackOutput(t, newMockStack(snap, nil), "greeting")
	require.EqualError(t, err, "current stack does not have output property 'greeting'")
}

func TestStackOutput_DestroyedStackKeepsMissingPropertyError(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{Time: time.Now().Add(-time.Hour)},
	}
	s := newMockStack(snap, nil).(*backend.MockStack)
	s.BackendF = func() backend.Backend {
		return &backend.MockBackend{
			NameF: func() string { return "mock://test" },
			GetHistoryF: func(context.Context, backend.StackReference, int, int) ([]backend.UpdateInfo, error) {
				return []backend.UpdateInfo{{Kind: apitype.DestroyUpdate}}, nil
			},
		}
	}

	_, err := runStackOutput(t, s, "greeting")
	require.EqualError(t, err, "current stack does not have output property 'greeting'")
}

// outputSnapshotStack serves stack outputs independently of Snapshot, matching a cloud
// backend whose outputs API succeeds when exporting the deployment fails.
type outputSnapshotStack struct {
	backend.Stack
	snapErr error
}

func (s outputSnapshotStack) SnapshotStackOutputs(context.Context, secrets.Provider) (property.Map, error) {
	return property.Map{}, nil
}

func (s outputSnapshotStack) Snapshot(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
	return nil, s.snapErr
}

func TestStackOutput_UnreadableSnapshotUsesHistory(t *testing.T) {
	t.Parallel()

	neverUpdated := outputSnapshotStack{Stack: newMockStack(nil, nil), snapErr: assert.AnError}
	_, err := runStackOutput(t, neverUpdated, "greeting")
	require.EqualError(t, err, neverDeployedOutputsMessage)

	deployed := newMockStack(nil, nil).(*backend.MockStack)
	deployed.BackendF = func() backend.Backend {
		return &backend.MockBackend{
			GetHistoryF: func(context.Context, backend.StackReference, int, int) ([]backend.UpdateInfo, error) {
				return []backend.UpdateInfo{{Kind: apitype.UpdateUpdate}}, nil
			},
		}
	}
	_, err = runStackOutput(t, outputSnapshotStack{Stack: deployed, snapErr: assert.AnError}, "greeting")
	require.EqualError(t, err, "current stack does not have output property 'greeting'")
}

func TestStackOutput_NeverDeployedListsNoOutputs(t *testing.T) {
	t.Parallel()

	// Listing every output (no property name) stays a successful empty result.
	stdout, err := runStackOutput(t, newMockStack(nil, nil))
	require.NoError(t, err)
	assert.Contains(t, stdout, "No output values currently in this stack")
}

func TestFileBackendNeverUpdatedStackAgreesWithHistory(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	s := createFileBackendStack(t, initDeployment(t, time.Date(2026, 10, 7, 23, 19, 10, 0, time.UTC)))

	history, err := s.Backend().GetHistory(ctx, s.Ref(), 1, 1)
	require.NoError(t, err)
	require.Empty(t, history)

	var buf bytes.Buffer
	require.NoError(t, runStackText(ctx, s, &buf, stackArgs{}))
	out := buf.String()
	assert.Contains(t, out, "No updates yet; run `pulumi up`")
	assert.NotContains(t, out, "Last updated:")
	assert.NotContains(t, out, "Pulumi version used:")

	_, err = runStackOutput(t, s, "greeting")
	require.EqualError(t, err, neverDeployedOutputsMessage)
}

func TestFileBackendDeployedStackUnchanged(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	s := createFileBackendStack(t, nil)

	manifest := deploy.Manifest{
		Time:    time.Now().Add(-2 * time.Hour),
		Version: "v3.265.0",
	}
	manifest.Magic = manifest.NewMagic()
	urn := resource.NewURN("dev", "repro", "", resource.RootStackType, "repro-dev")
	snap := deploy.NewSnapshot(manifest, nil, []*pkgresource.State{
		{
			Type: resource.RootStackType,
			URN:  urn,
			Outputs: resource.PropertyMap{
				"greeting": resource.NewProperty("hello"),
			},
		},
	}, nil, deploy.SnapshotMetadata{}, nil, nil)
	deployment, err := pkgstack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)
	require.NoError(t, s.Backend().ImportDeployment(ctx, s, deployment))

	var buf bytes.Buffer
	require.NoError(t, runStackText(ctx, s, &buf, stackArgs{}))
	out := buf.String()
	assert.Contains(t, out, "Last updated:")
	assert.Contains(t, out, "Pulumi version used: v3.265.0")
	assert.Contains(t, out, "hello")
	assert.NotContains(t, out, "No updates yet")

	stdout, err := runStackOutput(t, s, "greeting")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", stdout)

	_, err = runStackOutput(t, s, "missing")
	require.EqualError(t, err, "current stack does not have output property 'missing'")
}

func runStackOutput(t *testing.T, s backend.Stack, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := stackOutputCmd{
		Stdout: &buf,
		requireStack: func(
			context.Context, diag.Sink, pkgWorkspace.Context, cmdBackend.LoginManager,
			string, LoadOption, display.Options, string,
		) (backend.Stack, error) {
			return s, nil
		},
	}
	err := cmd.Run(t.Context(), args)
	return buf.String(), err
}

func createFileBackendStack(t *testing.T, initial *apitype.UntypedDeployment) backend.Stack {
	t.Helper()
	ctx := t.Context()
	stateDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir), &workspace.Project{
		Name: tokens.PackageName("repro"),
	})
	require.NoError(t, err)

	stackRef, err := b.ParseStackReference("dev")
	require.NoError(t, err)
	s, err := b.CreateStack(ctx, stackRef, "", initial, nil)
	require.NoError(t, err)
	return s
}

func initDeployment(t *testing.T, updatedAt time.Time) *apitype.UntypedDeployment {
	t.Helper()
	manifest := deploy.Manifest{
		Time:    updatedAt,
		Version: "v3.265.0",
	}
	manifest.Magic = manifest.NewMagic()
	body, err := json.Marshal(&apitype.DeploymentV3{Manifest: manifest.Serialize()})
	require.NoError(t, err)
	return &apitype.UntypedDeployment{
		Version:    3,
		Deployment: body,
	}
}
