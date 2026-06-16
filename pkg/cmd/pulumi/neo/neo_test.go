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

package neo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	displaytypes "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakeHTTPBackend embeds a generic MockBackend and adds the few extra methods the
// httpstate.Backend interface requires. resolveTaskTarget only dips into the base
// backend.Backend surface (ParseStackReference, GetDefaultOrg, CurrentUser), so the
// cloud-only hooks are no-ops.
type fakeHTTPBackend struct {
	*backend.MockBackend
	// ClientV is returned from Client(); leave nil for tests that don't need a
	// live client (the integration test wires a real *client.Client here).
	ClientV *client.Client
	// GetLatestStackPreviewF backs the stackPreviewLister capability used by
	// `pulumi neo debug`; leave nil to report no previews.
	GetLatestStackPreviewF func(context.Context, backend.StackReference) (*apitype.StackPreview, error)
}

func (f *fakeHTTPBackend) CloudURL() string                                       { return "" }
func (f *fakeHTTPBackend) StackConsoleURL(backend.StackReference) (string, error) { return "", nil }
func (f *fakeHTTPBackend) Client() *client.Client                                 { return f.ClientV }

func (f *fakeHTTPBackend) GetLatestStackPreview(
	ctx context.Context, stackRef backend.StackReference,
) (*apitype.StackPreview, error) {
	if f.GetLatestStackPreviewF != nil {
		return f.GetLatestStackPreviewF(ctx, stackRef)
	}
	return nil, nil
}

func (f *fakeHTTPBackend) RunDeployment(
	context.Context, backend.StackReference, apitype.CreateDeploymentRequest,
	display.Options, string, bool,
) error {
	return nil
}

func (f *fakeHTTPBackend) Search(
	context.Context, string, *apitype.PulumiQueryRequest,
) (*apitype.ResourceSearchResponse, error) {
	return nil, nil
}

func (f *fakeHTTPBackend) NaturalLanguageSearch(
	context.Context, string, string,
) (*apitype.ResourceSearchResponse, error) {
	return nil, nil
}

func (f *fakeHTTPBackend) PromptAI(context.Context, httpstate.AIPromptRequestBody) (*http.Response, error) {
	return nil, nil
}

func (f *fakeHTTPBackend) Capabilities(context.Context) apitype.Capabilities {
	return apitype.Capabilities{}
}

func newFakeBackend() *fakeHTTPBackend {
	return &fakeHTTPBackend{MockBackend: &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(s),
				FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
	}}
}

// isolateWorkspace neutralizes global state that resolveTaskTarget reads
// transitively: PULUMI_STACK (consulted by state.CurrentStack) and PULUMI_HOME
// (consulted by GetBackendConfigDefaultOrg, which reads ~/.pulumi/config.json for
// a user-configured default org and would otherwise leak a value from the
// developer's shell into these tests).
func isolateWorkspace(t *testing.T) {
	t.Helper()
	t.Setenv("PULUMI_STACK", "")
	t.Setenv("PULUMI_HOME", t.TempDir())
}

// These tests mutate process-wide env (PULUMI_STACK, PULUMI_HOME) so they can't
// run with t.Parallel — the paralleltest lint rule is suppressed on each one.

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_UsesStackFlag(t *testing.T) {
	isolateWorkspace(t)

	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "default-org", nil }

	ws := &pkgWorkspace.MockContext{}
	project := &workspace.Project{Name: tokens.PackageName("my-proj")}

	target, err := resolveTaskTarget(t.Context(), ws, be, project, "prod", "")
	require.NoError(t, err)
	assert.Equal(t, "default-org", target.org)
	assert.Equal(t, "my-proj", target.project)
	assert.Equal(t, "prod", target.stackName())
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_OrgFlagOverridesDefault(t *testing.T) {
	isolateWorkspace(t)

	// The explicit --org flag should win over any backend default, and the
	// backend's GetDefaultOrg hook must not be consulted at all.
	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) {
		t.Fatal("GetDefaultOrg should not be called when --org is provided")
		return "", nil
	}

	ws := &pkgWorkspace.MockContext{}
	target, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "explicit")
	require.NoError(t, err)
	assert.Equal(t, "explicit", target.org)
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_FallsBackToBackendDefaultOrg(t *testing.T) {
	isolateWorkspace(t)

	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "backend-default", nil }

	ws := &pkgWorkspace.MockContext{}
	target, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, "backend-default", target.org)
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_ErrorsWhenOrgUnresolvable(t *testing.T) {
	isolateWorkspace(t)

	// No flag, no project-configured default, and the backend has no opinion →
	// we must not create a task against an empty org; surface a clear error
	// directing the user to pass --org.
	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "", nil }

	ws := &pkgWorkspace.MockContext{}
	_, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass --org")
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_DefaultOrgLookupErrorIsWrapped(t *testing.T) {
	isolateWorkspace(t)

	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) {
		return "", errors.New("boom")
	}

	ws := &pkgWorkspace.MockContext{}
	_, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "determining default organization")
	assert.Contains(t, err.Error(), "boom")
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_InvalidStackReferenceErrors(t *testing.T) {
	isolateWorkspace(t)

	be := newFakeBackend()
	be.ParseStackReferenceF = func(string) (backend.StackReference, error) {
		return nil, errors.New("invalid stack")
	}

	ws := &pkgWorkspace.MockContext{}
	_, err := resolveTaskTarget(t.Context(), ws, be, nil, "bad/stack/name/here", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stack")
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_OmitsProjectNameWhenProjectNil(t *testing.T) {
	isolateWorkspace(t)

	// `pulumi neo` can be run outside a project — resolveTaskTarget must tolerate
	// a nil project and return an empty projectName rather than panicking.
	be := newFakeBackend()
	target, err := resolveTaskTarget(t.Context(), ws(), be, nil, "", "explicit")
	require.NoError(t, err)
	assert.Equal(t, "explicit", target.org)
	assert.Empty(t, target.project)
}

// parseQualifiedStackRefF builds a MockStackReference from `owner/project/name`,
// `owner/name`, or `name`, mirroring the real cloud backend's split rules.
// Tests use it to exercise the org-resolution paths in resolveTaskTarget that
// depend on the parsed reference carrying an owner.
func parseQualifiedStackRefF(s string) (backend.StackReference, error) {
	parts := strings.Split(s, "/")
	ref := &backend.MockStackReference{StringV: s, FullyQualifiedNameV: tokens.QName(s)}
	switch len(parts) {
	case 3:
		ref.OrganizationV = parts[0]
		ref.ProjectV = tokens.Name(parts[1])
		ref.NameV = tokens.MustParseStackName(parts[2])
	case 2:
		ref.OrganizationV = parts[0]
		ref.NameV = tokens.MustParseStackName(parts[1])
	default:
		ref.NameV = tokens.MustParseStackName(s)
	}
	return ref, nil
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_StackFlagOwnerWinsOverDefaultOrg(t *testing.T) {
	isolateWorkspace(t)

	// Regression: previously the owner embedded in --stack was discarded and the
	// Neo task was created in the user's default org, leaving the stack name
	// pointing at a different org. The parsed reference's owner must win.
	be := newFakeBackend()
	be.ParseStackReferenceF = parseQualifiedStackRefF
	be.GetDefaultOrgF = func(context.Context) (string, error) {
		t.Fatal("GetDefaultOrg must not be consulted when the stack reference has an owner")
		return "", nil
	}

	target, err := resolveTaskTarget(t.Context(), ws(), be, nil, "otherorg/proj/dev", "")
	require.NoError(t, err)
	assert.Equal(t, "otherorg", target.org)
	assert.Equal(t, "dev", target.stackName())
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_WorkspaceStackOwnerWinsOverDefaultOrg(t *testing.T) {
	isolateWorkspace(t)
	// isolateWorkspace sets PULUMI_STACK="" (still present in env), which makes
	// state.CurrentStack short-circuit via os.LookupEnv before consulting the
	// mocked workspace. For this test we need the workspace path to run, so
	// fully unset PULUMI_STACK; isolateWorkspace's t.Setenv cleanup still
	// restores the original after the test.
	require.NoError(t, os.Unsetenv("PULUMI_STACK"))

	// Regression: same bug as the --stack path but via the workspace-selected
	// stack. If `.pulumi/workspace.json` selects `otherorg/proj/dev`, Neo must
	// attach to `otherorg`, not the user's default org.
	be := newFakeBackend()
	be.ParseStackReferenceF = parseQualifiedStackRefF
	be.GetStackF = func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
		return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
	}
	be.GetDefaultOrgF = func(context.Context) (string, error) {
		t.Fatal("GetDefaultOrg must not be consulted when the workspace stack has an owner")
		return "", nil
	}

	wsCtx := &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "otherorg/proj/dev"}
				},
			}, nil
		},
	}

	target, err := resolveTaskTarget(t.Context(), wsCtx, be, nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, "otherorg", target.org)
	assert.Equal(t, "dev", target.stackName())
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_OrgFlagOverridesStackOwner(t *testing.T) {
	isolateWorkspace(t)

	// --org is an explicit override — when both are present it wins over the
	// stack reference's owner. The stack name is still taken from --stack;
	// the caller is responsible for ensuring the named stack exists in the
	// org they pass.
	be := newFakeBackend()
	be.ParseStackReferenceF = parseQualifiedStackRefF

	target, err := resolveTaskTarget(t.Context(), ws(), be, nil, "otherorg/proj/dev", "explicit")
	require.NoError(t, err)
	assert.Equal(t, "explicit", target.org)
	assert.Equal(t, "dev", target.stackName())
}

func ws() pkgWorkspace.Context { return &pkgWorkspace.MockContext{} }

// -----------------------------------------------------------------------------
// NewNeoCmd
// -----------------------------------------------------------------------------

func TestNewNeoCmd_RegistersFlags(t *testing.T) {
	t.Parallel()
	// Flag surface is part of the public CLI contract — renaming or dropping any
	// of these breaks users' scripts. Pin the shape.
	cmd := NewNeoCmd()

	assert.Equal(t, "neo [prompt]", cmd.Use)
	assert.False(t, cmd.Hidden, "the command must be visible")

	stack := cmd.Flags().Lookup("stack")
	require.NotNil(t, stack, "--stack flag must be registered")
	assert.Equal(t, "s", stack.Shorthand)

	require.NotNil(t, cmd.Flags().Lookup("org"), "--org flag must be registered")
	require.NotNil(t, cmd.Flags().Lookup("cwd"), "--cwd flag must be registered")

	print := cmd.Flags().Lookup("print")
	require.NotNil(t, print, "--print flag must be registered")
	assert.Equal(t, "p", print.Shorthand)

	disableIntegrations := cmd.Flags().Lookup("disable-integrations")
	require.NotNil(t, disableIntegrations, "--disable-integrations flag must be registered")
	assert.Equal(t, "false", disableIntegrations.DefValue, "--disable-integrations must default to off")

	// cobra.MaximumNArgs(1): 0 or 1 prompt args OK, 2+ rejected.
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"hello"}))
	require.Error(t, cmd.Args(cmd, []string{"a", "b"}), "more than one positional arg must be rejected")
}

// TestNewNeoCmd_PrintRejectsManualApproval pins the RunE pre-check that refuses
// `--print --approval-mode=manual` before any network call: there is no UI to
// approve from, so the agent would deadlock on the first approval prompt.
func TestNewNeoCmd_PrintRejectsManualApproval(t *testing.T) {
	t.Parallel()
	cmd := NewNeoCmd()
	cmd.SetContext(t.Context())
	require.NoError(t, cmd.Flags().Set("print", "true"))
	require.NoError(t, cmd.Flags().Set("approval-mode", "manual"))

	err := cmd.RunE(cmd, []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--approval-mode=manual is incompatible with --print")
}

func TestDedupeExistingRoots(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	missing := tmp + "/does-not-exist"

	got := dedupeExistingRoots("", tmp, tmp, missing, t.TempDir())
	require.Len(t, got, 2, "empties, missing paths, and duplicates must be dropped")
	assert.Equal(t, tmp, got[0], "first occurrence of a canonical path is preserved verbatim")
}

// -----------------------------------------------------------------------------
// newPulumiSinkForUI
//
// The sink is a thin translation layer between tools.PulumiSink callbacks and
// the UIEvent channel that drives the TUI. Each callback maps to one UIEvent
// variant; these tests exercise that mapping in isolation so the runNeo
// errgroup plumbing doesn't have to be staged for a unit test.
// -----------------------------------------------------------------------------

func TestNewPulumiSinkForUI_OnStart(t *testing.T) {
	t.Parallel()

	uiCh := make(chan UIEvent, 1)
	sink := newPulumiSinkForUI(uiCh)
	require.NotNil(t, sink.OnStart)

	sink.OnStart("pulumi__pulumi_preview", "dev", true)

	got := <-uiCh
	start, ok := got.(UIPulumiStart)
	require.True(t, ok, "OnStart must emit UIPulumiStart, got %T", got)
	assert.Equal(t, "pulumi__pulumi_preview", start.ToolName)
	assert.Equal(t, "dev", start.StackName)
	assert.True(t, start.IsPreview)
}

func TestNewPulumiSinkForUI_OnResource(t *testing.T) {
	t.Parallel()

	uiCh := make(chan UIEvent, 1)
	sink := newPulumiSinkForUI(uiCh)
	require.NotNil(t, sink.OnResource)

	sink.OnResource("pulumi__pulumi_up", deploy.OpCreate,
		"urn:pulumi:dev::p::aws:s3/Bucket::b", "aws:s3/Bucket", "running")

	got := <-uiCh
	res, ok := got.(UIPulumiResource)
	require.True(t, ok, "OnResource must emit UIPulumiResource, got %T", got)
	assert.Equal(t, "pulumi__pulumi_up", res.ToolName)
	assert.Equal(t, deploy.OpCreate, res.Op)
	assert.Equal(t, "urn:pulumi:dev::p::aws:s3/Bucket::b", res.URN)
	assert.Equal(t, "aws:s3/Bucket", res.Type)
	assert.Equal(t, "running", res.Status)
}

func TestNewPulumiSinkForUI_OnDiag(t *testing.T) {
	t.Parallel()

	uiCh := make(chan UIEvent, 1)
	sink := newPulumiSinkForUI(uiCh)
	require.NotNil(t, sink.OnDiag)

	sink.OnDiag("pulumi__pulumi_up", "warning", "deprecated foo", "urn:x")

	got := <-uiCh
	d, ok := got.(UIPulumiDiag)
	require.True(t, ok, "OnDiag must emit UIPulumiDiag, got %T", got)
	assert.Equal(t, "pulumi__pulumi_up", d.ToolName)
	assert.Equal(t, "warning", d.Severity)
	assert.Equal(t, "deprecated foo", d.Message)
	assert.Equal(t, "urn:x", d.URN)
}

func TestNewPulumiSinkForUI_OnEnd(t *testing.T) {
	t.Parallel()

	uiCh := make(chan UIEvent, 1)
	sink := newPulumiSinkForUI(uiCh)
	require.NotNil(t, sink.OnEnd)

	counts := displaytypes.ResourceChanges{deploy.OpCreate: 2}
	sink.OnEnd("pulumi__pulumi_up", "boom", counts, "5s")

	got := <-uiCh
	e, ok := got.(UIPulumiEnd)
	require.True(t, ok, "OnEnd must emit UIPulumiEnd, got %T", got)
	assert.Equal(t, "pulumi__pulumi_up", e.ToolName)
	assert.Equal(t, "boom", e.Err)
	assert.Equal(t, counts, e.Counts)
	assert.Equal(t, "5s", e.Elapsed)
}

func TestNewPulumiSinkForUI_NonBlockingOnFullChannel(t *testing.T) {
	t.Parallel()

	// sendUI is a non-blocking send (buffered + default branch), so a full
	// channel must drop the event rather than deadlock the engine drain
	// goroutine. Verify by overflowing a 1-buffered channel.
	uiCh := make(chan UIEvent, 1)
	uiCh <- UIWarning{Message: "filler"}

	sink := newPulumiSinkForUI(uiCh)
	require.NotPanics(t, func() {
		sink.OnStart("pulumi__pulumi_preview", "dev", true)
	}, "OnStart must not block when the UI channel is full")
}

// -----------------------------------------------------------------------------
// runWithTUI
//
// Regression tests for the Ctrl+C double-press hang: when the bubbletea program
// exits, the shared errgroup context must be cancelled so the dispatcher and
// any active session.Run unblock. errgroup itself only cancels its derived
// context on a non-nil error, but tea.Quit returns nil from p.Run, so without
// the explicit cancel inside runWithTUI the helper would hang on g.Wait.
// -----------------------------------------------------------------------------

// awaitClose returns once ch is closed or the deadline elapses, failing the
// test if the channel never closes. Tests use it to assert that worker
// goroutines actually exit instead of relying on test-timeout to catch a hang.
func awaitClose(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting: %s", msg)
	}
}

func TestRunWithTUI_CleanTUIExitCancelsWorkers(t *testing.T) {
	t.Parallel()

	// The bug: tea.Quit returns nil from p.Run, errgroup keeps gctx alive on
	// nil errors, so workers blocked on gctx.Done would never exit. Simulate
	// the clean exit and verify the worker is unblocked.
	workerExited := make(chan struct{})

	err := runWithTUI(
		t.Context(),
		func() error { return nil }, // tea.Quit-style clean exit
		func(g *errgroup.Group, gctx context.Context) {
			g.Go(func() error {
				<-gctx.Done()
				close(workerExited)
				return nil
			})
		},
	)
	require.NoError(t, err)
	awaitClose(t, workerExited, "worker should unblock when TUI exits cleanly")
}

func TestRunWithTUI_PropagatesTUIError(t *testing.T) {
	t.Parallel()

	// A non-nil error from runTUI must surface through g.Wait, and workers
	// must still get cancelled on the way out so g.Wait can return.
	boom := errors.New("tui boom")
	workerExited := make(chan struct{})

	err := runWithTUI(
		t.Context(),
		func() error { return boom },
		func(g *errgroup.Group, gctx context.Context) {
			g.Go(func() error {
				<-gctx.Done()
				close(workerExited)
				return nil
			})
		},
	)
	require.ErrorIs(t, err, boom)
	awaitClose(t, workerExited, "worker should unblock when TUI exits with error")
}

func TestRunWithTUI_PropagatesWorkerError(t *testing.T) {
	t.Parallel()

	// Worker errors must propagate; the TUI goroutine should still complete
	// (errgroup cancels gctx on the worker's non-nil return, but runTUI is
	// not bound to gctx in production — here we make it watch ctx so the
	// test terminates without relying on the test-timeout).
	boom := errors.New("worker boom")

	err := runWithTUI(
		t.Context(),
		func() error {
			// Real bubbletea programs aren't ctx-aware; in production the
			// errgroup waits for tea.Quit. For the test we just exit
			// immediately so g.Wait can collect the worker's error.
			return nil
		},
		func(g *errgroup.Group, _ context.Context) {
			g.Go(func() error { return boom })
		},
	)
	require.ErrorIs(t, err, boom)
}

func TestRunWithTUI_RegisterRunsBeforeTUI(t *testing.T) {
	t.Parallel()

	// register must run synchronously before the TUI goroutine starts so the
	// dispatcher (which the register callback installs) is already listening
	// on outCh by the time the TUI begins emitting events. If runWithTUI ever
	// inverted the order, an early TUI message could race past the dispatcher.
	var registerDone atomic.Bool
	tuiStarted := make(chan struct{})

	err := runWithTUI(
		t.Context(),
		func() error {
			close(tuiStarted)
			// Assert the ordering invariant from inside the TUI goroutine.
			assert.True(t, registerDone.Load(), "register must complete before runTUI starts")
			return nil
		},
		func(g *errgroup.Group, gctx context.Context) {
			g.Go(func() error {
				<-gctx.Done()
				return nil
			})
			registerDone.Store(true)
		},
	)
	require.NoError(t, err)
	awaitClose(t, tuiStarted, "TUI goroutine should run")
}

func TestRunWithTUI_LazilySpawnedWorkersJoin(t *testing.T) {
	t.Parallel()

	// The dispatcher captures g and calls g.Go from inside its loop when the
	// first user message arrives (lazy task creation). g.Wait must include
	// those late-spawned goroutines, so the helper can't return until they
	// also finish. Verify by spawning a delayed worker and checking it ran.
	var lateWorkerRan atomic.Bool

	err := runWithTUI(
		t.Context(),
		func() error { return nil },
		func(g *errgroup.Group, gctx context.Context) {
			g.Go(func() error {
				// Spawn a sibling after this worker starts. In production this
				// is the dispatcher reacting to its first outbound user_message.
				g.Go(func() error {
					lateWorkerRan.Store(true)
					return nil
				})
				<-gctx.Done()
				return nil
			})
		},
	)
	require.NoError(t, err)
	assert.True(t, lateWorkerRan.Load(), "late-spawned worker must run before runWithTUI returns")
}

func TestRunWithTUI_ParentContextCancellationStopsWorkers(t *testing.T) {
	t.Parallel()

	// External cancellation (Ctrl+C delivered to the parent ctx) must
	// propagate to workers via gctx. The TUI is not ctx-aware in production
	// — we simulate that by having it block until the test releases it via
	// tuiRelease, mimicking a TUI that exits in response to its own input.
	parentCtx, cancelParent := context.WithCancel(t.Context())
	tuiRelease := make(chan struct{})
	workerExited := make(chan struct{})

	go func() {
		// Cancel the parent shortly after starting; the worker should observe
		// it via gctx.Done and exit, after which we release the TUI so the
		// helper can return.
		time.Sleep(20 * time.Millisecond)
		cancelParent()
	}()

	go func() {
		<-workerExited
		close(tuiRelease)
	}()

	err := runWithTUI(
		parentCtx,
		func() error {
			<-tuiRelease
			return nil
		},
		func(g *errgroup.Group, gctx context.Context) {
			g.Go(func() error {
				<-gctx.Done()
				close(workerExited)
				return nil
			})
		},
	)
	// errgroup returns the first non-nil worker error, but workers here return
	// nil after observing cancellation. The helper itself returns nil — the
	// caller of runNeo would surface ctx.Err() via session.Run separately.
	require.NoError(t, err)
}

// -----------------------------------------------------------------------------
// dispatchUserEvents
//
// The dispatcher used to live as an inline goroutine inside runNeo, where its
// branches couldn't be exercised without driving the bubbletea model. It was
// extracted so each branch (context cancel, channel close, lazy task
// creation, taskID-not-ready warning, post-event success and failure) is
// directly testable.
// -----------------------------------------------------------------------------

// noopPostEvent is the postEvent stub for dispatcher tests that aren't
// exercising the post path. Always succeeds; receiving a call here means the
// test reached the post branch when it shouldn't have.
func noopPostEvent(context.Context, string, any) error { return nil }

// noopSpawn is the spawnCreateTask stub for dispatcher tests that aren't
// exercising lazy task creation. Discards everything.
func noopSpawn(string, client.NeoApprovalMode, client.NeoPermissionMode, bool) {}

// noopUpdateTask is the updateTask stub for dispatcher tests that aren't
// exercising the mid-session update path. Always succeeds.
func noopUpdateTask(context.Context, string, client.UpdateNeoTaskOptions) error { return nil }

func TestDispatchUserEvents_ContextCancelExits(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	out := make(chan outboundEvent)
	done := make(chan error, 1)
	go func() {
		done <- dispatchUserEvents(ctx, out, nil, true,
			func() string { return "task-1" },
			noopSpawn, noopPostEvent, noopUpdateTask)
	}()

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "ctx-cancel exit must return nil, not ctx.Err()")
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not exit after ctx cancel")
	}
}

func TestDispatchUserEvents_ChannelCloseExits(t *testing.T) {
	t.Parallel()

	out := make(chan outboundEvent)
	done := make(chan error, 1)
	go func() {
		done <- dispatchUserEvents(t.Context(), out, nil, true,
			func() string { return "task-1" },
			noopSpawn, noopPostEvent, noopUpdateTask)
	}()

	close(out)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not exit after outCh close")
	}
}

func TestDispatchUserEvents_LazyTaskCreationOnFirstUserMessage(t *testing.T) {
	t.Parallel()

	// initialTaskCreated=false: no CLI prompt was passed, so the first
	// user_message must trigger CreateNeoTask via spawnCreateTask. Subsequent
	// events should NOT spawn again — the dispatcher tracks taskCreated.
	var spawned []string
	var spawnedPlanMode []bool
	var spawnedApproval []client.NeoApprovalMode
	var spawnedPermission []client.NeoPermissionMode
	spawn := func(msg string, am client.NeoApprovalMode, pm client.NeoPermissionMode, planMode bool) {
		spawned = append(spawned, msg)
		spawnedPlanMode = append(spawnedPlanMode, planMode)
		spawnedApproval = append(spawnedApproval, am)
		spawnedPermission = append(spawnedPermission, pm)
	}

	postCalls := 0
	post := func(_ context.Context, _ string, _ any) error {
		postCalls++
		return nil
	}

	out := make(chan outboundEvent, 4)
	out <- outboundEvent{
		event:          apitype.AgentUserEventUserMessage{Type: userEventUserMessage, Content: "hello"},
		planMode:       true,
		approvalMode:   client.NeoApprovalModeBalanced,
		permissionMode: client.NeoPermissionModeReadOnly,
	}
	// Second event arrives after taskCreated is true and a taskID is set —
	// must take the post branch, not the spawn branch.
	out <- outboundEvent{
		event: apitype.AgentUserEventCancel{Type: userEventUserCancel},
	}
	close(out)

	err := dispatchUserEvents(t.Context(), out, nil, false,
		func() string { return "task-1" },
		spawn, post, noopUpdateTask)
	require.NoError(t, err)

	require.Len(t, spawned, 1, "only the first user_message must trigger lazy task creation")
	assert.Equal(t, "hello", spawned[0])
	assert.Equal(t, []bool{true}, spawnedPlanMode,
		"planMode must be forwarded from the outboundEvent envelope")
	assert.Equal(t, []client.NeoApprovalMode{client.NeoApprovalModeBalanced}, spawnedApproval,
		"approvalMode must be forwarded from the outboundEvent envelope")
	assert.Equal(t, []client.NeoPermissionMode{client.NeoPermissionModeReadOnly}, spawnedPermission,
		"permissionMode must be forwarded from the outboundEvent envelope")
	assert.Equal(t, 1, postCalls, "the second event must be posted to the live task")
}

func TestDispatchUserEvents_WarnsWhenTaskNotReady(t *testing.T) {
	t.Parallel()

	// Non-user_message event arrives before any task exists. The dispatcher
	// must surface a UIWarning instead of silently dropping; the comment in
	// runNeo notes this is unreachable in normal TUI flow but defends against
	// future event types or bugs in the busy-state gate.
	uiCh := make(chan UIEvent, 4)

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, uiCh, false,
		func() string { return "" }, // no task yet
		func(string, client.NeoApprovalMode, client.NeoPermissionMode, bool) {
			t.Fatal("spawn must not fire for non-user_message")
		},
		func(context.Context, string, any) error {
			t.Fatal("post must not fire when taskID is empty")
			return nil
		},
		noopUpdateTask)
	require.NoError(t, err)

	close(uiCh)
	var got []UIWarning
	for evt := range uiCh {
		if w, ok := evt.(UIWarning); ok {
			got = append(got, w)
		}
	}
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "task not ready")
}

func TestDispatchUserEvents_PostsEventToBackend(t *testing.T) {
	t.Parallel()

	// Once a task is live, non-user_message events flow through to postEvent.
	type postCall struct {
		taskID string
		body   any
	}
	var calls []postCall

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, nil, true,
		func() string { return "task-1" },
		noopSpawn,
		func(_ context.Context, taskID string, body any) error {
			calls = append(calls, postCall{taskID: taskID, body: body})
			return nil
		},
		noopUpdateTask)
	require.NoError(t, err)

	require.Len(t, calls, 1)
	assert.Equal(t, "task-1", calls[0].taskID)
	cancelEvt, ok := calls[0].body.(apitype.AgentUserEventCancel)
	require.True(t, ok, "body must round-trip the original AgentUserEvent type")
	assert.Equal(t, userEventUserCancel, cancelEvt.Type)
}

func TestDispatchUserEvents_WarnsOnPostFailure(t *testing.T) {
	t.Parallel()

	// A backend error during PostNeoTaskUserEvent must surface as a UIWarning
	// — losing a single user event shouldn't tear down the dispatcher loop.
	uiCh := make(chan UIEvent, 4)

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, uiCh, true,
		func() string { return "task-1" },
		noopSpawn,
		func(context.Context, string, any) error {
			return errors.New("network down")
		},
		noopUpdateTask)
	require.NoError(t, err, "post errors must be reported via UIWarning, not propagated")

	close(uiCh)
	var got []UIWarning
	for evt := range uiCh {
		if w, ok := evt.(UIWarning); ok {
			got = append(got, w)
		}
	}
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "failed to send event")
	assert.Contains(t, got[0].Message, "network down")
}

func TestCreateNeoTaskWithEntityRetry(t *testing.T) {
	t.Parallel()

	t.Run("RetriesWithoutEntityOnInvalidEntities", func(t *testing.T) {
		t.Parallel()

		// First call carries entity_diff and gets rejected; the wrapper must retry
		// once with no stack so the task is still created.
		var calls []map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			calls = append(calls, body)
			if len(calls) == 1 {
				rw.WriteHeader(http.StatusBadRequest)
				_, _ = rw.Write([]byte(`{"code":400,"message":"invalid entities: unable to access stack"}`))
				return
			}
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_retry"}`))
		}))
		defer server.Close()

		pc := client.NewClient(server.URL, "", false, nil)
		var droppedErrs []error
		resp, err := createNeoTaskWithEntityRetry(
			t.Context(), pc, "my-org", "hello", "my-stack", "my-project",
			client.CreateNeoTaskOptions{ToolExecutionMode: "cli"},
			func(originalErr error) {
				droppedErrs = append(droppedErrs, originalErr)
			})
		require.NoError(t, err)
		assert.Equal(t, "t_retry", resp.TaskID)
		require.Len(t, calls, 2)

		firstMsg, _ := calls[0]["message"].(map[string]any)
		require.NotNil(t, firstMsg)
		assert.Contains(t, firstMsg, "entity_diff")

		retryMsg, _ := calls[1]["message"].(map[string]any)
		require.NotNil(t, retryMsg)
		assert.NotContains(t, retryMsg, "entity_diff")

		require.Len(t, droppedErrs, 1)
		assert.Contains(t, droppedErrs[0].Error(), "invalid entities")
	})

	t.Run("DoesNotRetryOnUnrelatedError", func(t *testing.T) {
		t.Parallel()

		// Non-entity errors must surface to the caller without a second POST.
		var calls int
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			calls++
			rw.WriteHeader(http.StatusForbidden)
			_, _ = rw.Write([]byte(`{"code":403,"message":"forbidden"}`))
		}))
		defer server.Close()

		pc := client.NewClient(server.URL, "", false, nil)
		_, err := createNeoTaskWithEntityRetry(
			t.Context(), pc, "my-org", "hello", "my-stack", "my-project",
			client.CreateNeoTaskOptions{},
			func(error) { t.Fatal("onEntityDropped should not fire on unrelated errors") })
		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("DoesNotRetryWhenStackMissing", func(t *testing.T) {
		t.Parallel()

		// With no stack to attach there's nothing to drop, so an "invalid entities"
		// response must propagate as-is rather than triggering a pointless retry.
		var calls int
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			calls++
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(`{"code":400,"message":"invalid entities"}`))
		}))
		defer server.Close()

		pc := client.NewClient(server.URL, "", false, nil)
		_, err := createNeoTaskWithEntityRetry(
			t.Context(), pc, "my-org", "hello", "", "", client.CreateNeoTaskOptions{},
			func(error) { t.Fatal("onEntityDropped should not fire when no stack is attached") })
		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})
}

func TestDispatchUserEvents_RoutesUpdateToUpdateTask(t *testing.T) {
	t.Parallel()

	// An outboundEvent.update entry carries a mid-session approval/permission
	// toggle. The dispatcher must route it to updateTask (not postEvent) and
	// pass through the Options struct verbatim, so the cloud PATCH lands.
	type updateCall struct {
		taskID string
		opts   client.UpdateNeoTaskOptions
	}
	var updates []updateCall
	mode := client.NeoApprovalModeBalanced

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{update: &client.UpdateNeoTaskOptions{ApprovalMode: &mode}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, nil, true,
		func() string { return "task-1" },
		noopSpawn,
		func(context.Context, string, any) error {
			t.Fatal("postEvent must not fire for an update-only outboundEvent")
			return nil
		},
		func(_ context.Context, taskID string, opts client.UpdateNeoTaskOptions) error {
			updates = append(updates, updateCall{taskID: taskID, opts: opts})
			return nil
		})
	require.NoError(t, err)

	require.Len(t, updates, 1)
	assert.Equal(t, "task-1", updates[0].taskID)
	require.NotNil(t, updates[0].opts.ApprovalMode)
	assert.Equal(t, client.NeoApprovalModeBalanced, *updates[0].opts.ApprovalMode)
}

func TestDispatchUserEvents_DropsUpdateSilentlyWhenTaskNotReady(t *testing.T) {
	t.Parallel()

	// A pre-task-creation toggle is the expected path when the user presses
	// Ctrl+A or Ctrl+R before sending any message. The dispatcher must drop
	// the update without warning — the next CreateNeoTask will pick up the
	// snapshotted value from the user_message envelope.
	uiCh := make(chan UIEvent, 4)
	mode := client.NeoApprovalModeAuto

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{update: &client.UpdateNeoTaskOptions{ApprovalMode: &mode}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, uiCh, false,
		func() string { return "" }, // no task yet
		noopSpawn,
		noopPostEvent,
		func(context.Context, string, client.UpdateNeoTaskOptions) error {
			t.Fatal("updateTask must not fire when taskID is empty")
			return nil
		})
	require.NoError(t, err)

	close(uiCh)
	for evt := range uiCh {
		if w, ok := evt.(UIWarning); ok {
			t.Fatalf("pre-task update must drop silently, got UIWarning: %s", w.Message)
		}
	}
}

func TestDispatchUserEvents_WarnsOnUpdateFailure(t *testing.T) {
	t.Parallel()

	// PATCH errors must surface as a UIWarning so the user knows the toggle
	// didn't take effect, but the dispatcher itself must keep running — a
	// transient cloud blip shouldn't tear the session down.
	uiCh := make(chan UIEvent, 4)
	mode := client.NeoApprovalModeAuto

	out := make(chan outboundEvent, 1)
	out <- outboundEvent{update: &client.UpdateNeoTaskOptions{ApprovalMode: &mode}}
	close(out)

	err := dispatchUserEvents(t.Context(), out, uiCh, true,
		func() string { return "task-1" },
		noopSpawn,
		noopPostEvent,
		func(context.Context, string, client.UpdateNeoTaskOptions) error {
			return errors.New("network down")
		})
	require.NoError(t, err)

	close(uiCh)
	var got []UIWarning
	for evt := range uiCh {
		if w, ok := evt.(UIWarning); ok {
			got = append(got, w)
		}
	}
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "failed to update Neo task")
	assert.Contains(t, got[0].Message, "network down")
}

func TestParseApprovalMode(t *testing.T) {
	t.Parallel()

	t.Run("ValidValues", func(t *testing.T) {
		t.Parallel()
		for _, s := range []string{"manual", "balanced", "auto"} {
			got, err := parseApprovalMode(s)
			require.NoErrorf(t, err, "value %q must parse", s)
			assert.Equal(t, client.NeoApprovalMode(s), got)
		}
	})

	t.Run("RejectsUnknown", func(t *testing.T) {
		t.Parallel()
		_, err := parseApprovalMode("yolo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "manual, balanced, auto")
	})
}

func TestParsePermissionMode(t *testing.T) {
	t.Parallel()

	t.Run("ValidValues", func(t *testing.T) {
		t.Parallel()
		for _, s := range []string{"default", "read-only"} {
			got, err := parsePermissionMode(s)
			require.NoErrorf(t, err, "value %q must parse", s)
			assert.Equal(t, client.NeoPermissionMode(s), got)
		}
	})

	t.Run("RejectsUnknown", func(t *testing.T) {
		t.Parallel()
		_, err := parsePermissionMode("admin")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "default, read-only")
	})
}
