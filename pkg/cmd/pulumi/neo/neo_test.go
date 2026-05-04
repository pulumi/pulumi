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
	"errors"
	"net/http"
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
}

func (f *fakeHTTPBackend) CloudURL() string                                       { return "" }
func (f *fakeHTTPBackend) StackConsoleURL(backend.StackReference) (string, error) { return "", nil }
func (f *fakeHTTPBackend) Client() *client.Client                                 { return nil }

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

	org, proj, stack, err := resolveTaskTarget(t.Context(), ws, be, project, "prod", "")
	require.NoError(t, err)
	assert.Equal(t, "default-org", org)
	assert.Equal(t, "my-proj", proj)
	assert.Equal(t, "prod", stack)
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
	org, _, _, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "explicit")
	require.NoError(t, err)
	assert.Equal(t, "explicit", org)
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_FallsBackToBackendDefaultOrg(t *testing.T) {
	isolateWorkspace(t)

	be := newFakeBackend()
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "backend-default", nil }

	ws := &pkgWorkspace.MockContext{}
	org, _, _, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, "backend-default", org)
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
	_, _, _, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
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
	_, _, _, err := resolveTaskTarget(t.Context(), ws, be, nil, "", "")
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
	_, _, _, err := resolveTaskTarget(t.Context(), ws, be, nil, "bad/stack/name/here", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stack")
}

//nolint:paralleltest // uses t.Setenv
func TestResolveTaskTarget_OmitsProjectNameWhenProjectNil(t *testing.T) {
	isolateWorkspace(t)

	// `pulumi neo` can be run outside a project — resolveTaskTarget must tolerate
	// a nil project and return an empty projectName rather than panicking.
	be := newFakeBackend()
	org, proj, _, err := resolveTaskTarget(t.Context(), ws(), be, nil, "", "explicit")
	require.NoError(t, err)
	assert.Equal(t, "explicit", org)
	assert.Empty(t, proj)
}

func ws() pkgWorkspace.Context { return &pkgWorkspace.MockContext{} }

// -----------------------------------------------------------------------------
// NewNeoCmd
// -----------------------------------------------------------------------------

//nolint:paralleltest // uses t.Setenv to toggle PULUMI_EXPERIMENTAL
func TestNewNeoCmd_RegistersFlags(t *testing.T) {
	// Flag surface is part of the public CLI contract — renaming or dropping any
	// of these breaks users' scripts. Pin the shape.
	t.Setenv("PULUMI_EXPERIMENTAL", "true")
	cmd := NewNeoCmd()

	assert.Equal(t, "neo [prompt]", cmd.Use)
	assert.False(t, cmd.Hidden, "with PULUMI_EXPERIMENTAL=true the command must be visible")

	stack := cmd.Flags().Lookup("stack")
	require.NotNil(t, stack, "--stack flag must be registered")
	assert.Equal(t, "s", stack.Shorthand)

	require.NotNil(t, cmd.Flags().Lookup("org"), "--org flag must be registered")
	require.NotNil(t, cmd.Flags().Lookup("cwd"), "--cwd flag must be registered")

	// cobra.MaximumNArgs(1): 0 or 1 prompt args OK, 2+ rejected.
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"hello"}))
	require.Error(t, cmd.Args(cmd, []string{"a", "b"}), "more than one positional arg must be rejected")
}

//nolint:paralleltest // uses t.Setenv to toggle PULUMI_EXPERIMENTAL
func TestNewNeoCmd_HiddenWhenNotExperimental(t *testing.T) {
	// env.Experimental.Value() is read at command construction; clearing the
	// env var and re-constructing must hide the command so users on stable
	// channels don't see it in `pulumi --help`.
	t.Setenv("PULUMI_EXPERIMENTAL", "")
	cmd := NewNeoCmd()
	assert.True(t, cmd.Hidden, "without PULUMI_EXPERIMENTAL the command must be hidden")
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
