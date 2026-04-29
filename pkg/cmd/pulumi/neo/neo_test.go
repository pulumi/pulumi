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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
