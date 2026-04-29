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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestIndex(ops ...*Operation) *Index {
	idx := &Index{
		Operations: ops,
		ByKey:      make(map[string]*Operation, len(ops)),
	}
	for _, op := range ops {
		idx.ByKey[op.Key()] = op
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

func TestCountTemplateParams(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, countTemplateParams("/api/user"))
	assert.Equal(t, 2, countTemplateParams("/api/stacks/{org}/{project}"))
	assert.Equal(t, 1, countTemplateParams("/api/orgs/{orgName}/members"))
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
