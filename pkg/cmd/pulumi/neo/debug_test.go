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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDebugSeedPromptWithID(t *testing.T) {
	t.Parallel()

	// The seed is a short trigger line; the debugging procedure lives in the
	// pulumi-debug-failed-operation skill, which Neo's evaluator loads from this text. The kind
	// selects the noun (update vs preview) and the id pins the specific run.
	assert.Equal(t,
		"Debug the failed Pulumi update 42 of this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(debugUpdate, "42"))
	assert.Equal(t,
		"Debug the failed Pulumi preview 7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d "+
			"of this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(debugPreview, "7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d"))

	// The procedure moved to the skill, so the seed stays a one-liner with no embedded steps.
	got := debugSeedPrompt(debugUpdate, "42")
	assert.NotContains(t, got, "<details>")
	assert.NotContains(t, got, "/api/console")
}

func TestDebugSeedPromptNoID(t *testing.T) {
	t.Parallel()

	// With no id, the seed targets the user's most recent operation of the given kind; the skill
	// confirms which one.
	assert.Equal(t,
		"Debug my most recent Pulumi update on this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(debugUpdate, ""))
	assert.Equal(t,
		"Debug my most recent Pulumi preview on this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(debugPreview, ""))
}

func TestLatestOperationID(t *testing.T) {
	t.Parallel()

	be := newFakeBackend()
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return []backend.UpdateInfo{
			{Kind: apitype.UpdateUpdate, Version: 42, Result: backend.FailedResult, StartTime: 100},
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

	// An update resolves to its history version; a preview to its opaque UpdateID. The two are
	// looked up independently, so each kind ignores the other.
	assert.Equal(t, "42", debugUpdate.latestID(t.Context(), be, ref))
	assert.Equal(t, "2e07637b-d20b-4d4f-9d29-a7bcb1631cf7", debugPreview.latestID(t.Context(), be, ref))
}

func TestLatestOperationID_BestEffort(t *testing.T) {
	t.Parallel()

	// A nil stack ref, an error, or empty history all resolve to "" rather than panicking.
	be := newFakeBackend()
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		t.Fatal("GetHistory must not be called when the stack reference is nil")
		return nil, nil
	}
	assert.Equal(t, "", debugUpdate.latestID(t.Context(), be, nil))

	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return nil, errors.New("boom")
	}
	assert.Equal(t, "", debugUpdate.latestID(t.Context(), be, ref))

	be.GetHistoryF = func(
		_ context.Context, _ backend.StackReference, _ int, _ int,
	) ([]backend.UpdateInfo, error) {
		return nil, nil
	}
	assert.Equal(t, "", debugUpdate.latestID(t.Context(), be, ref))
}

func TestDebugStackContext_FullContext(t *testing.T) {
	t.Parallel()

	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", []string{"acme"}, nil, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(be, taskTarget{org: "acme", project: "my-proj", ref: ref}, debugUpdate, "42")

	assert.Contains(t, got, "- Organization: acme\n")
	assert.Contains(t, got, "- User: alice\n")
	assert.Contains(t, got, "- Project: my-proj\n")
	assert.Contains(t, got, "- Stack: dev\n")
	// The context phrasing matches the seed prompt's "<kind> <id>".
	assert.Contains(t, got, "- Debugging: update 42\n")
}

func TestDebugStackContext_Preview(t *testing.T) {
	t.Parallel()

	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "alice", nil, nil, nil
	}
	ref := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	got := debugStackContext(be, taskTarget{org: "acme", project: "my-proj", ref: ref}, debugPreview,
		"2e07637b-d20b-4d4f-9d29-a7bcb1631cf7")

	assert.Contains(t, got, "- Debugging: preview 2e07637b-d20b-4d4f-9d29-a7bcb1631cf7\n")
}

func TestDebugStackContext_GracefulOmission(t *testing.T) {
	t.Parallel()

	// Every lookup is best-effort: a CurrentUser error, a nil stack reference, no resolved
	// project/stack, and no resolved id must all be omitted rather than break the debug prompt.
	be := newFakeBackend()
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "", nil, nil, errors.New("not logged in")
	}

	got := debugStackContext(be, taskTarget{org: "acme"}, debugUpdate, "")

	assert.Contains(t, got, "- Organization: acme\n")
	assert.NotContains(t, got, "- User:")
	assert.NotContains(t, got, "- Project:")
	assert.NotContains(t, got, "- Stack:")
	assert.NotContains(t, got, "- Debugging:")
}
