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

package cloud

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestIndex(ops ...*Operation) *Index {
	idx := &Index{
		Operations: ops,
		ByKey:      make(map[string]*Operation, len(ops)),
		router:     mux.NewRouter(),
	}
	// Same per-segment static-first ordering as parseIndex.
	sortedOps := append([]*Operation(nil), ops...)
	slices.SortStableFunc(sortedOps, compareOps)
	for _, op := range ops {
		idx.ByKey[op.Key()] = op
	}
	for _, op := range sortedOps {
		idx.router.Path(op.Path).Methods(op.Method).Name(op.Key())
	}
	return idx
}

func TestMatchPath_Literal(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user"},
	)
	mr, err := MatchPath(idx, "GET", "/api/user")
	require.NoError(t, err)
	assert.Equal(t, "/api/user", mr.Op.Path)
	assert.Empty(t, mr.Bindings)
}

func TestMatchPath_TemplateCapture(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/stacks/{orgName}/{projectName}"},
	)
	mr, err := MatchPath(idx, "GET", "/api/stacks/acme/widgets")
	require.NoError(t, err)
	assert.Equal(t, Binding{Literal: "acme"}, mr.Bindings["orgName"])
	assert.Equal(t, Binding{Literal: "widgets"}, mr.Bindings["projectName"])
}

func TestMatchPath_UserAliasPlaceholder(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/stacks/{orgName}"},
	)
	mr, err := MatchPath(idx, "GET", "/api/stacks/{org}")
	require.NoError(t, err)
	assert.Equal(t, Binding{Placeholder: "org"}, mr.Bindings["orgName"])
}

func TestMatchPath_MixedLiteralAndPlaceholder(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/stacks/{orgName}/{projectName}"},
	)
	mr, err := MatchPath(idx, "GET", "/api/stacks/acme/{project}")
	require.NoError(t, err)
	assert.Equal(t, Binding{Literal: "acme"}, mr.Bindings["orgName"])
	assert.Equal(t, Binding{Placeholder: "project"}, mr.Bindings["projectName"])
}

func TestMatchPath_MethodMismatch(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "POST", Path: "/api/user"},
	)
	_, err := MatchPath(idx, "GET", "/api/user")
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, cmdutil.ExitCodeError, apiErr.ExitCode)
	assert.Equal(t, ErrNoMatch, apiErr.Envelope.Error.Code)
}

func TestMatchPath_SegmentLengthMismatch(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/stacks/{orgName}"},
	)
	_, err := MatchPath(idx, "GET", "/api/stacks/acme/extra")
	assert.Error(t, err)
}

func TestMatchPath_PrefersMoreSpecific(t *testing.T) {
	t.Parallel()
	// Both could structurally match but the literal one wins.
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/users/{userLogin}"},
		&Operation{Method: "GET", Path: "/api/users/me"},
	)
	mr, err := MatchPath(idx, "GET", "/api/users/me")
	require.NoError(t, err)
	assert.Equal(t, "/api/users/me", mr.Op.Path)
}

func TestMatchPath_UserTemplateAgainstLiteralSegmentFails(t *testing.T) {
	t.Parallel()
	// User types {x} in a position where the spec has a literal "me".
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/users/me"},
	)
	_, err := MatchPath(idx, "GET", "/api/users/{x}")
	assert.Error(t, err)
}

func TestMatchByOperationID_Exact(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user", OperationID: "GetCurrentUser"},
		&Operation{Method: "GET", Path: "/api/orgs", OperationID: "ListOrgs"},
	)
	mr, err := MatchByOperationID(idx, "GetCurrentUser")
	require.NoError(t, err)
	assert.Equal(t, "/api/user", mr.Op.Path)
}

func TestMatchByOperationID_CaseInsensitive(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/orgs", OperationID: "ListOrgs"},
	)
	mr, err := MatchByOperationID(idx, "listorgs")
	require.NoError(t, err)
	assert.Equal(t, "ListOrgs", mr.Op.OperationID)
}

func TestMatchByOperationID_NoMatch(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user", OperationID: "GetCurrentUser"},
	)
	_, err := MatchByOperationID(idx, "Nonexistent")
	assert.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrNoMatch, apiErr.Envelope.Error.Code)
}

func TestMatchByOperationID_Ambiguous(t *testing.T) {
	t.Parallel()
	// Shouldn't happen in a real spec, but guard against duplicates anyway.
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/a", OperationID: "Dup"},
		&Operation{Method: "POST", Path: "/api/b", OperationID: "Dup"},
	)
	_, err := MatchByOperationID(idx, "Dup")
	assert.Error(t, err)
}

func TestLooksLikeOperationID(t *testing.T) {
	t.Parallel()
	assert.True(t, looksLikeOperationID("ListAccounts"))
	assert.True(t, looksLikeOperationID("get-user"))
	assert.False(t, looksLikeOperationID("/api/user"))
	assert.False(t, looksLikeOperationID(""))
}

func TestSplitLeadingHTTPMethod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		wantVerb string
		wantRest string
		wantOK   bool
	}{
		{"GET /api/user", "GET", "/api/user", true},
		{"get /api/user", "GET", "/api/user", true},
		{"POST   /api/stacks", "POST", "/api/stacks", true},
		{"/api/user", "", "", false},
		{"ListAccounts", "", "", false},
		{"BOGUS /api/user", "", "", false},
		{"", "", "", false},
		{"GET", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			v, r, ok := splitLeadingHTTPMethod(tc.in)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantVerb, v)
			assert.Equal(t, tc.wantRest, r)
		})
	}
}

// TestCompareOps_PerSegmentStaticFirst pins the routing-order contract
// that powers PrefersMoreSpecific: at each depth, literal segments sort
// before {template} segments; equal-kind segments fall back to lexical
// compare; shorter paths sort before longer ones with the same prefix.
// Mirrors pulumi-service's cmd/pulumi-codegen/cmd/go.go operationTree.walk.
func TestCompareOps_PerSegmentStaticFirst(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		a, b     string
		wantSign int // -1 a<b, 0 equal, +1 a>b
	}{
		{"literal_before_template_at_depth_2", "/api/users/me", "/api/users/{login}", -1},
		{"literal_before_template_at_depth_3", "/api/{org}/me", "/api/{org}/{login}", -1},
		{"lexical_among_literals", "/api/orgs", "/api/users", -1},
		{"lexical_among_templates", "/api/{a}", "/api/{b}", -1},
		{"shorter_before_longer_same_prefix", "/api/users", "/api/users/me", -1},
		{"equal", "/api/user", "/api/user", 0},
		{
			"literal_branch_wins_over_template_branch_at_root",
			"/api/foo/{a}", "/api/{x}/bar", -1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &Operation{Method: "GET", Path: tc.a}
			b := &Operation{Method: "GET", Path: tc.b}
			got := compareOps(a, b)
			switch {
			case tc.wantSign < 0:
				assert.Less(t, got, 0, "expected %s < %s", tc.a, tc.b)
			case tc.wantSign > 0:
				assert.Greater(t, got, 0, "expected %s > %s", tc.a, tc.b)
			default:
				assert.Equal(t, 0, got, "expected %s == %s", tc.a, tc.b)
			}
		})
	}
}

// TestMatchPath_DeepSpecificity exercises a 3-level template tree to
// verify that the per-segment static-first ordering still picks the
// most-specific match when literals and templates interleave deeper than
// the simpler PrefersMoreSpecific case.
func TestMatchPath_DeepSpecificity(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/orgs/{orgName}/members"},
		&Operation{Method: "GET", Path: "/api/orgs/{orgName}/{otherName}"},
	)
	// User input "members" should hit the literal-segment route, not the
	// pure-template one — even though the template route would also match.
	mr, err := MatchPath(idx, "GET", "/api/orgs/acme/members")
	require.NoError(t, err)
	assert.Equal(t, "/api/orgs/{orgName}/members", mr.Op.Path)
	assert.Equal(t, Binding{Literal: "acme"}, mr.Bindings["orgName"])
	// And conversely a non-"members" tail falls through to the template route.
	mr, err = MatchPath(idx, "GET", "/api/orgs/acme/teams")
	require.NoError(t, err)
	assert.Equal(t, "/api/orgs/{orgName}/{otherName}", mr.Op.Path)
	assert.Equal(t, Binding{Literal: "teams"}, mr.Bindings["otherName"])
}

// TestMatchPath_SamePathDifferentMethods verifies that two operations
// sharing a path but differing in HTTP method both register and route
// correctly; gorilla's per-route .Methods filter handles the dispatch.
func TestMatchPath_SamePathDifferentMethods(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/stacks/{orgName}", OperationID: "GetStack"},
		&Operation{Method: "POST", Path: "/api/stacks/{orgName}", OperationID: "CreateStack"},
	)
	mr, err := MatchPath(idx, "GET", "/api/stacks/acme")
	require.NoError(t, err)
	assert.Equal(t, "GetStack", mr.Op.OperationID)
	mr, err = MatchPath(idx, "POST", "/api/stacks/acme")
	require.NoError(t, err)
	assert.Equal(t, "CreateStack", mr.Op.OperationID)
}

// TestMatchByOperationID_PopulatesPlaceholderBindings verifies the
// MatchByOperationID symmetry trick: a templated op looked up by ID gets
// Bindings populated with one Placeholder per {var} in its path, so the
// dispatcher can hand them to resolveTemplateVar.
func TestMatchByOperationID_PopulatesPlaceholderBindings(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{
			Method: "GET", Path: "/api/orgs/{orgName}/teams/{teamName}/members",
			OperationID: "ListTeamMembers",
		},
	)
	mr, err := MatchByOperationID(idx, "ListTeamMembers")
	require.NoError(t, err)
	assert.Equal(t, Binding{Placeholder: "orgName"}, mr.Bindings["orgName"])
	assert.Equal(t, Binding{Placeholder: "teamName"}, mr.Bindings["teamName"])
}

// TestMatchByOperationID_LiteralPathHasNoBindings verifies that an op
// with no template segments returns an empty (non-nil) Bindings map.
func TestMatchByOperationID_LiteralPathHasNoBindings(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user", OperationID: "GetCurrentUser"},
	)
	mr, err := MatchByOperationID(idx, "GetCurrentUser")
	require.NoError(t, err)
	require.NotNil(t, mr.Bindings)
	assert.Empty(t, mr.Bindings)
}

// TestMatchPath_PathNormalization verifies that user input with extra
// slashes is normalised via path.Clean before matching, so /api//user
// resolves to the same op as /api/user.
func TestMatchPath_PathNormalization(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user"},
	)
	mr, err := MatchPath(idx, "GET", "/api//user")
	require.NoError(t, err)
	assert.Equal(t, "/api/user", mr.Op.Path)
}

func TestMatchPath_SuggestsOperationIDWhenInputLooksLikeOne(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(
		&Operation{Method: "GET", Path: "/api/user", OperationID: "GetCurrentUser"},
	)
	_, err := MatchPath(idx, "GET", "ListAccounts")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	// The suggestion mentioning operation-ID usage should be present.
	found := false
	for _, s := range apiErr.Envelope.Error.Suggestions {
		if strings.Contains(s, "describe ListAccounts") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected an operation-ID suggestion, got %v", apiErr.Envelope.Error.Suggestions)
}
