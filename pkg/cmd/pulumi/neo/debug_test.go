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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDebugSeedPromptWithID(t *testing.T) {
	t.Parallel()

	// The seed is a short trigger line; the debugging procedure lives in the
	// pulumi-debug-failed-operation skill, which Neo's evaluator loads from this text. The id
	// itself tells Neo whether it is an update version or a preview UUID, so the seed is the
	// same generic line for both — only the id varies.
	for _, id := range []string{"5", "7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d"} {
		got := debugSeedPrompt(id)
		assert.Equal(t,
			"Debug the failed Pulumi operation "+id+
				" of this stack and fix it directly in this working directory.\n",
			got)

		// The procedure moved to the skill, so the seed stays a one-liner with no embedded steps.
		assert.NotContains(t, got, "<details>")
		assert.NotContains(t, got, "/api/console")
	}
}

func TestDebugSeedPromptNoID(t *testing.T) {
	t.Parallel()

	// With no id, the seed targets the user's most recent operation; the skill confirms which one.
	assert.Equal(t,
		"Debug my most recent Pulumi operation on this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(""))
}

func TestDebugStackContext_FullContext(t *testing.T) {
	t.Parallel()

	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", []string{"acme"}, nil, nil
	}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return []backend.UpdateInfo{
			{Kind: apitype.UpdateUpdate, Version: 42, Result: backend.FailedResult, StartTime: 100},
			{Kind: apitype.UpdateUpdate, Version: 41, Result: backend.SucceededResult, StartTime: 50},
		}, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(t.Context(), be, ref, "acme", "my-proj")

	assert.Contains(t, got, "- Organization: acme\n")
	assert.Contains(t, got, "- User: alice\n")
	assert.Contains(t, got, "- Project: my-proj\n")
	assert.Contains(t, got, "- Stack: dev\n")
	// With no newer preview, the most recent history entry (newest-first) is surfaced.
	assert.Contains(t, got, "- Most recent operation: update (version 42, result: failed)\n")
	assert.NotContains(t, got, "version 41")
}

func TestDebugStackContext_NewerPreviewWins(t *testing.T) {
	t.Parallel()

	// Regression: a preview that ran more recently than the last deployment must be reported as
	// the most recent operation. Previews are not in GetHistory, so without the separate lookup
	// `neo debug` would wrongly target the older update.
	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", nil, nil, nil
	}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return []backend.UpdateInfo{
			{Kind: apitype.UpdateUpdate, Version: 42, Result: backend.SucceededResult, StartTime: 100},
		}, nil
	}
	be.GetLatestStackPreviewF = func(
		_ context.Context, _ backend.StackReference,
	) (*apitype.StackPreview, error) {
		return &apitype.StackPreview{
			UpdateID: "2e07637b-d20b-4d4f-9d29-a7bcb1631cf7",
			Info:     apitype.UpdateInfo{Result: apitype.FailedResult, StartTime: 200},
		}, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(t.Context(), be, ref, "acme", "my-proj")

	assert.Contains(t, got,
		"- Most recent operation: preview 2e07637b-d20b-4d4f-9d29-a7bcb1631cf7 (result: failed)\n")
	assert.NotContains(t, got, "version 42")
}

func TestDebugStackContext_OlderPreviewLosesToUpdate(t *testing.T) {
	t.Parallel()

	// When the last deployment is newer than the latest preview, the update wins.
	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", nil, nil, nil
	}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return []backend.UpdateInfo{
			{Kind: apitype.UpdateUpdate, Version: 42, Result: backend.FailedResult, StartTime: 300},
		}, nil
	}
	be.GetLatestStackPreviewF = func(
		_ context.Context, _ backend.StackReference,
	) (*apitype.StackPreview, error) {
		return &apitype.StackPreview{
			UpdateID: "2e07637b-d20b-4d4f-9d29-a7bcb1631cf7",
			Info:     apitype.UpdateInfo{Result: apitype.SucceededResult, StartTime: 200},
		}, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(t.Context(), be, ref, "acme", "my-proj")

	assert.Contains(t, got, "- Most recent operation: update (version 42, result: failed)\n")
	assert.NotContains(t, got, "preview")
}

func TestDebugStackContext_GracefulOmission(t *testing.T) {
	t.Parallel()

	// Every lookup is best-effort: a CurrentUser error, a nil stack reference, and no
	// resolved project/stack must all be omitted rather than break the debug prompt.
	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "", nil, nil, errors.New("not logged in")
	}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		t.Fatal("GetHistory must not be called when the stack reference is nil")
		return nil, nil
	}

	got := debugStackContext(t.Context(), be, nil, "acme", "")

	assert.Contains(t, got, "- Organization: acme\n")
	assert.NotContains(t, got, "- User:")
	assert.NotContains(t, got, "- Project:")
	assert.NotContains(t, got, "- Stack:")
	assert.NotContains(t, got, "Most recent operation")
}

func TestDebugStackContext_EmptyHistory(t *testing.T) {
	t.Parallel()

	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", nil, nil, nil
	}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return nil, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(t.Context(), be, ref, "acme", "my-proj")

	// A stack with no operation history shouldn't produce a dangling operation line.
	assert.NotContains(t, got, "Most recent operation")
}

func TestNeoDebugCmdRegistered(t *testing.T) {
	t.Parallel()

	cmd := newNeoDebugCmd()
	assert.Equal(t, "debug [update-or-preview-id]", cmd.Use)
	require.NotNil(t, cmd.Args)
	// The id is optional (0 or 1 args); two or more is an error.
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"5"}))
	assert.Error(t, cmd.Args(cmd, []string{"5", "6"}))

	// The shared flags are present so `debug` honors the same options as `pulumi neo`.
	for _, name := range []string{"stack", "org", "cwd", "approval-mode", "permission-mode", "print"} {
		assert.NotNilf(t, cmd.Flags().Lookup(name), "flag %q", name)
	}
}
