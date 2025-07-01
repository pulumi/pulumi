// Copyright 2016-2024, Pulumi Corporation.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestParseTagFilter(t *testing.T) {
	t.Parallel()

	p := func(s string) *string {
		return &s
	}

	tests := []struct {
		Filter    string
		WantName  string
		WantValue *string
	}{
		// Just tag name
		{Filter: "", WantName: ""},
		{Filter: ":", WantName: ":"},
		{Filter: "just tag name", WantName: "just tag name"},
		{Filter: "tag-name123", WantName: "tag-name123"},

		// Tag name and value
		{Filter: "tag-name123=tag value", WantName: "tag-name123", WantValue: p("tag value")},
		{Filter: "tag-name123=tag value:with-colon", WantName: "tag-name123", WantValue: p("tag value:with-colon")},
		{Filter: "tag-name123=tag value=with-equal", WantName: "tag-name123", WantValue: p("tag value=with-equal")},

		// Degenerate cases
		{Filter: "=", WantName: "", WantValue: p("")},
		{Filter: "no tag value=", WantName: "no tag value", WantValue: p("")},
		{Filter: "=no tag name", WantName: "", WantValue: p("no tag name")},
	}

	for _, test := range tests {
		name, value := parseTagFilter(test.Filter)
		assert.Equal(t, test.WantName, name, "parseTagFilter(%q) name", test.Filter)
		if test.WantValue == nil {
			assert.Nil(t, value, "parseTagFilter(%q) value", test.Filter)
		} else {
			if value == nil {
				t.Errorf("parseTagFilter(%q) expected %q tag name, but got nil", test.Filter, *test.WantValue)
			} else {
				assert.Equal(t, *test.WantValue, *value)
			}
		}
	}
}

func newContToken(s string) backend.ContinuationToken {
	return &s
}

// mockStackSummary implements the backend.StackSummary interface.
type mockStackSummary struct {
	name        string
	LastUpdateF func() *time.Time
}

func (mss *mockStackSummary) Name() backend.StackReference {
	name := tokens.MustParseStackName(mss.name)
	return &backend.MockStackReference{
		NameV:               name,
		FullyQualifiedNameV: name.Q(),
		StringV:             name.String(),
	}
}

func (mss *mockStackSummary) LastUpdate() *time.Time {
	if mss.LastUpdateF != nil {
		return mss.LastUpdateF()
	}
	return nil
}

func (mss *mockStackSummary) ResourceCount() *int {
	return nil
}

type stackLSInputs struct {
	filter      backend.ListStacksFilter
	inContToken backend.ContinuationToken
}

type stackLSOutputs struct {
	summaries    []backend.StackSummary
	outContToken backend.ContinuationToken
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestListStacksPagination(t *testing.T) {
	// We mock out the ListStacks call so that it will return 4x well-known responses, and
	// keep track of the parameters used for validation.
	var requestsMade []stackLSInputs
	cannedResponses := []stackLSOutputs{
		// Page 1.
		{
			summaries: []backend.StackSummary{
				&mockStackSummary{name: "stack-in-page-1"},
			},
			outContToken: newContToken("first-cont-token-response"),
		},

		// Pages 2 and 3. We don't expect a backend to return a nil result of StackSummary objects,
		// but we do expect the situation to be handled gracefully by the CLI.
		{nil, newContToken("second-cont-token-response")},
		{[]backend.StackSummary{}, newContToken("third-cont-token-response")},

		// Page 4.
		{
			summaries: []backend.StackSummary{
				&mockStackSummary{name: "stack-in-page-4"},
				&mockStackSummary{name: "stack-in-page-4"},
			},
			outContToken: nil,
		},
	}

	testutil.MockBackendInstance(t, &backend.MockBackend{
		ListStacksF: func(ctx context.Context, filter backend.ListStacksFilter, inContToken backend.ContinuationToken) (
			[]backend.StackSummary, backend.ContinuationToken, error,
		) {
			requestsMade = append(requestsMade, stackLSInputs{filter, inContToken})
			requestIdx := len(requestsMade) - 1
			response := cannedResponses[requestIdx]
			return response.summaries, response.outContToken, nil
		},
	})

	const testOrgName, testProjName = "comprehendingdevice", "website"

	// Execute the command, which will use our mocked backend. Confirm the expected number of
	// backend calls were made.
	ctx := context.Background()
	args := stackLSArgs{
		orgFilter:  testOrgName,
		projFilter: testProjName,
	}
	if err := runStackLS(ctx, args); err != nil {
		t.Fatalf("runStackLS returned an error: %v", err)
	}
	if len(requestsMade) != 4 {
		t.Fatalf("runStackLS didn't call backend::ListStacks the expected number of times (%d vs 4.)", len(requestsMade))
	}

	assertFilterIsAsExpected := func(filter backend.ListStacksFilter) {
		assert.Equal(t, testOrgName, *filter.Organization)
		assert.Equal(t, testProjName, *filter.Project)
		assert.Nil(t, filter.TagName)
		assert.Nil(t, filter.TagValue)
	}

	// Confirm the first request to ListStacks was made with a nil continuation token.
	call1 := requestsMade[0]
	assertFilterIsAsExpected(call1.filter)
	assert.Nil(t, call1.inContToken)

	// Confirm subsequent calls were all using the continuation token returned from
	// the previous call to backend::ListStacks.
	for callIdx := 1; callIdx < len(requestsMade); callIdx++ {
		call := requestsMade[callIdx]
		assertFilterIsAsExpected(call.filter)
		assert.Equal(t, *cannedResponses[callIdx-1].outContToken, *call.inContToken,
			"Continuation token for request %d was not the same token returned from call %d.",
			callIdx, callIdx-1)
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestListStacksJsonProgress(t *testing.T) {
	mockTime := time.Unix(1, 0)

	testutil.MockBackendInstance(t, &backend.MockBackend{
		ListStacksF: func(ctx context.Context, filter backend.ListStacksFilter, inContToken backend.ContinuationToken) (
			[]backend.StackSummary, backend.ContinuationToken, error,
		) {
			return []backend.StackSummary{
				&mockStackSummary{
					name: "stack-in-page-1",
					LastUpdateF: func() *time.Time {
						t := mockTime
						return &t
					},
				},
				&mockStackSummary{
					name: "stack-in-page-2",
					LastUpdateF: func() *time.Time {
						t := time.Unix(0, 0)
						return &t
					},
				},
				&mockStackSummary{
					name: "stack-in-page-3",
					LastUpdateF: func() *time.Time {
						return nil
					},
				},
			}, nil, nil
		},
		SupportsProgressF: func() bool {
			return true
		},
	})

	var buff bytes.Buffer
	ctx := context.Background()
	args := stackLSArgs{
		jsonOut:   true,
		allStacks: true,
		stdout:    &buff,
	}
	err := runStackLS(ctx, args)
	require.NoError(t, err)

	assert.JSONEq(t, `[
			{
				"name": "stack-in-page-1",
				"updateInProgress": false,
				"lastUpdate": "1970-01-01T00:00:01.000Z",
				"current": false
			},
			{
				"name": "stack-in-page-2",
				"updateInProgress": true,
				"current": false
			},
			{
				"name": "stack-in-page-3",
				"current": false
			}
		]`, buff.String())
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestListStacksJsonNoProgress(t *testing.T) {
	mockTime := time.Unix(1, 0)

	testutil.MockBackendInstance(t, &backend.MockBackend{
		ListStacksF: func(ctx context.Context, filter backend.ListStacksFilter, inContToken backend.ContinuationToken) (
			[]backend.StackSummary, backend.ContinuationToken, error,
		) {
			return []backend.StackSummary{
				&mockStackSummary{
					name: "stack-in-page-1",
					LastUpdateF: func() *time.Time {
						t := mockTime
						return &t
					},
				},
				&mockStackSummary{
					name: "stack-in-page-2",
					LastUpdateF: func() *time.Time {
						return nil
					},
				},
			}, nil, nil
		},
		SupportsProgressF: func() bool {
			return false
		},
	})

	var buff bytes.Buffer
	ctx := context.Background()
	args := stackLSArgs{
		jsonOut:   true,
		allStacks: true,
		stdout:    &buff,
	}
	err := runStackLS(ctx, args)
	require.NoError(t, err)

	assert.JSONEq(t, `[
			{
				"name": "stack-in-page-1",
				"lastUpdate": "1970-01-01T00:00:01.000Z",
				"current": false
			},
			{
				"name": "stack-in-page-2",
				"current": false
			}
		]`, buff.String())
}
