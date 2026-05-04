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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPage_Array(t *testing.T) {
	t.Parallel()
	items, field, cursor, param, err := ExtractPage([]byte(`[1,2,3]`), nil)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Empty(t, field)
	assert.Empty(t, cursor)
	assert.Empty(t, param)
}

func TestExtractPage_ObjectWithContinuationToken(t *testing.T) {
	t.Parallel()
	body := []byte(`{"items":[{"a":1},{"a":2}],"continuationToken":"abc"}`)
	items, field, cursor, param, err := ExtractPage(body, nil)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "items", field)
	assert.Equal(t, "abc", cursor)
	assert.Equal(t, "continuationToken", param)

	// Item preserved as RawMessage
	var first map[string]int
	require.NoError(t, json.Unmarshal(items[0], &first))
	assert.Equal(t, 1, first["a"])
}

func TestExtractPage_ObjectWithTopLevelCursor(t *testing.T) {
	t.Parallel()
	body := []byte(`{"items":[{"a":1}],"cursor":"xyz"}`)
	items, field, cursor, param, err := ExtractPage(body, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "items", field)
	assert.Equal(t, "xyz", cursor)
	assert.Equal(t, "cursor", param)
}

func TestExtractPage_ObjectWithPaginationNextLink(t *testing.T) {
	t.Parallel()
	// v2 shape: "next" carries a link; extract its cursor query param.
	// "cursor" in the response is an opaque bookmark of the *current*
	// page and must NOT be used to advance.
	body := []byte(`{"resources":[{"id":"r1"}],"pagination":{"cursor":"bookmark",` +
		`"next":"/api/orgs/acme/search/resourcesv2?query=foo&cursor=next-token&size=1"}}`)
	items, field, cursor, param, err := ExtractPage(body, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "resources", field)
	assert.Equal(t, "next-token", cursor)
	assert.Equal(t, "cursor", param)
}

func TestExtractPage_ObjectWithPaginationLastPage(t *testing.T) {
	t.Parallel()
	body := []byte(`{"resources":[{"id":"r1"}],"pagination":{"cursor":"bookmark","next":""}}`)
	items, field, cursor, param, err := ExtractPage(body, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "resources", field)
	assert.Empty(t, cursor)
	assert.Empty(t, param)
}

func TestExtractPage_ObjectWithCursorButNoArray(t *testing.T) {
	t.Parallel()
	// A cursor field with no accompanying array is a server envelope bug —
	// we'd loop forever if we kept sending the cursor back. Error out.
	_, _, _, _, err := ExtractPage([]byte(`{"continuationToken":"x","status":"ok"}`), nil)
	assert.Error(t, err)
}

// TestExtractPage_EmptyObject covers the case where the server returns
// `{}` to signal "no results, no more pages" (seen on Insights endpoints
// when the caller's org has no accounts). This must terminate pagination
// gracefully rather than erroring.
func TestExtractPage_EmptyObject(t *testing.T) {
	t.Parallel()
	items, field, cursor, param, err := ExtractPage([]byte(`{}`), nil)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Empty(t, field)
	assert.Empty(t, cursor)
	assert.Empty(t, param)
}

// TestExtractPage_NullArrayField: a server that serialises an empty
// collection as `null` instead of `[]` should be treated the same as an
// empty array — no more pages.
func TestExtractPage_NullArrayField(t *testing.T) {
	t.Parallel()
	items, _, cursor, _, err := ExtractPage([]byte(`{"accounts":null}`), nil)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Empty(t, cursor)
}

func TestExtractPage_ObjectMultipleArrays(t *testing.T) {
	t.Parallel()
	_, _, _, _, err := ExtractPage([]byte(`{"items":[1],"other":[2]}`), nil)
	assert.Error(t, err)
}

// TestExtractPage_NextTokenMapsToContinuationToken covers the Insights-style
// shape where the response uses "nextToken" but the operation's pagination
// query parameter is "continuationToken". We must resolve the cursor param
// from the operation's parameters, not from the response field name.
func TestExtractPage_NextTokenMapsToContinuationToken(t *testing.T) {
	t.Parallel()
	op := &Operation{
		Method: "GET",
		Path:   "/api/preview/insights/{orgName}/accounts",
		Params: []ParamSpec{
			{Name: "orgName", In: "path"},
			{Name: "continuationToken", In: "query"},
			{Name: "count", In: "query"},
		},
	}
	body := []byte(`{"accounts":[{"id":"a"}],"nextToken":"abc123"}`)
	items, field, cursor, param, err := ExtractPage(body, op)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "accounts", field)
	assert.Equal(t, "abc123", cursor)
	assert.Equal(t, "continuationToken", param)
}

// TestExtractPage_NextTokenWithNoKnownQueryParam: the response carries a
// next-page cursor but the op exposes no recognised pagination query
// parameter. That is a structural mismatch we cannot recover from — we
// prefer to error loudly rather than silently drop the cursor or loop.
func TestExtractPage_NextTokenWithNoKnownQueryParam(t *testing.T) {
	t.Parallel()
	op := &Operation{
		Method: "GET",
		Path:   "/api/foo",
		Params: []ParamSpec{
			{Name: "other", In: "query"},
		},
	}
	body := []byte(`{"items":[{"a":1}],"nextToken":"abc"}`)
	_, _, _, _, err := ExtractPage(body, op)
	require.Error(t, err)
}

// TestExtractPage_NextTokenEmptyIsLastPage: an empty nextToken means no
// more pages — match the behavior for continuationToken/cursor.
func TestExtractPage_NextTokenEmptyIsLastPage(t *testing.T) {
	t.Parallel()
	op := &Operation{
		Params: []ParamSpec{{Name: "continuationToken", In: "query"}},
	}
	body := []byte(`{"items":[{"a":1}],"nextToken":""}`)
	_, _, cursor, param, err := ExtractPage(body, op)
	require.NoError(t, err)
	assert.Empty(t, cursor)
	assert.Empty(t, param)
}

// TestPickCursorParam covers the query-param selection order.
func TestPickCursorParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params []ParamSpec
		want   string
	}{
		{
			name: "continuationToken preferred",
			params: []ParamSpec{
				{Name: "pageToken", In: "query"},
				{Name: "continuationToken", In: "query"},
				{Name: "cursor", In: "query"},
			},
			want: "continuationToken",
		},
		{
			name:   "cursor when continuationToken absent",
			params: []ParamSpec{{Name: "cursor", In: "query"}, {Name: "pageToken", In: "query"}},
			want:   "cursor",
		},
		{
			name:   "pageToken fallback",
			params: []ParamSpec{{Name: "pageToken", In: "query"}},
			want:   "pageToken",
		},
		{
			name:   "ignore path params with matching name",
			params: []ParamSpec{{Name: "continuationToken", In: "path"}},
			want:   "",
		},
		{
			name:   "no candidates",
			params: []ParamSpec{{Name: "filter", In: "query"}},
			want:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := &Operation{Params: tc.params}
			assert.Equal(t, tc.want, pickCursorParam(op))
		})
	}
}

func TestExtractPage_EmptyBody(t *testing.T) {
	t.Parallel()
	items, field, cursor, param, err := ExtractPage(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Empty(t, field)
	assert.Empty(t, cursor)
	assert.Empty(t, param)
}

func TestExtractPage_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, _, _, _, err := ExtractPage([]byte(`not json at all`), nil)
	assert.Error(t, err)
}

func TestMarshalAccumulated_BareArray(t *testing.T) {
	t.Parallel()
	// When the server returned a bare array, output stays a bare array.
	items := []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}
	out, err := marshalAccumulated(items, "")
	require.NoError(t, err)
	assert.JSONEq(t, `[1,2]`, string(out))
}

func TestMarshalAccumulated_PreservesEnvelope(t *testing.T) {
	t.Parallel()
	// When the server returned an object envelope, we rewrap so downstream
	// `| jq` filters like '.accounts[]' keep working across --paginate.
	items := []json.RawMessage{json.RawMessage(`{"id":"a"}`), json.RawMessage(`{"id":"b"}`)}
	out, err := marshalAccumulated(items, "accounts")
	require.NoError(t, err)
	assert.JSONEq(t, `{"accounts":[{"id":"a"},{"id":"b"}]}`, string(out))
}

func TestMarshalAccumulated_StripsCursorFields(t *testing.T) {
	t.Parallel()
	// The rewrapped envelope only contains the items field — cursor
	// metadata is meaningless post-pagination and must not leak through.
	items := []json.RawMessage{json.RawMessage(`1`)}
	out, err := marshalAccumulated(items, "resources")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	_, hasCursor := got["cursor"]
	_, hasContToken := got["continuationToken"]
	_, hasPagination := got["pagination"]
	assert.False(t, hasCursor)
	assert.False(t, hasContToken)
	assert.False(t, hasPagination)
	assert.Contains(t, got, "resources")
}

// TestFlushAccumulated_SilentSuppressesStdout pins the contract that --silent
// suppresses the partial-failure body flush, not just the success-path output.
func TestFlushAccumulated_SilentSuppressesStdout(t *testing.T) {
	t.Parallel()
	items := []json.RawMessage{json.RawMessage(`{"id":"a"}`)}
	var buf bytes.Buffer
	flushAccumulated(&buf, items, "accounts", &apiCommand{silent: true})
	assert.Empty(t, buf.String(), "--silent must suppress the partial-failure flush")
}

// TestFlushAccumulated_DefaultWritesEnvelope covers the baseline (no flags).
func TestFlushAccumulated_DefaultWritesEnvelope(t *testing.T) {
	t.Parallel()
	items := []json.RawMessage{json.RawMessage(`{"id":"a"}`)}
	var buf bytes.Buffer
	flushAccumulated(&buf, items, "accounts", &apiCommand{})
	assert.JSONEq(t, `{"accounts":[{"id":"a"}]}`, buf.String())
}

// TestRunPaginate_TruncationEmitsPartialPaginationError pins the contract
// that hitting PaginationLimit with a still-active cursor produces an
// ErrPartialPagination envelope and a non-zero exit, not a silent "complete".
//
//nolint:paralleltest // mutates PaginationLimit
func TestRunPaginate_TruncationEmitsPartialPaginationError(t *testing.T) {
	// Always hand back a new cursor so the loop can never exit naturally.
	var pagesServed int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pagesServed++
		fmt.Fprintf(w, `{"items":[{"p":%d}],"continuationToken":"c%d"}`, pagesServed, pagesServed)
	}))
	t.Cleanup(srv.Close)

	origLimit := PaginationLimit
	PaginationLimit = 3
	t.Cleanup(func() { PaginationLimit = origLimit })

	apiClient := client.NewClient(srv.URL, "", false, nil)
	req := paginateRequest{Method: "GET", Path: "/list", BaseQuery: url.Values{}, Accept: "application/json"}
	var buf bytes.Buffer
	err := runPaginate(t.Context(), &buf, apiClient, req, &apiCommand{})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, cmdutil.ExitCodeError, apiErr.ExitCode)
	assert.Equal(t, ErrPartialPagination, apiErr.Envelope.Error.Code)
	assert.True(t, apiErr.Silent, "truncation error must be Silent so stderr is not double-written")
	assert.Contains(t, apiErr.Envelope.Error.Message, "truncated at 3 pages")
	assert.Equal(t, 3, pagesServed)
}

// TestHTTPErrorEnvelopeBytes_ExitCodes pins the exit-code contract:
// 4xx and 5xx both exit 1, only 401/403 exit 3. The error code field still
// distinguishes 4xx from 5xx for JSON consumers.
func TestHTTPErrorEnvelopeBytes_ExitCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status   int
		wantExit int
		wantCode string
	}{
		{400, cmdutil.ExitCodeError, ErrHTTP4xx},
		{401, cmdutil.ExitAuthenticationError, ErrHTTP4xx},
		{403, cmdutil.ExitAuthenticationError, ErrHTTP4xx},
		{404, cmdutil.ExitCodeError, ErrHTTP4xx},
		{409, cmdutil.ExitCodeError, ErrHTTP4xx},
		{429, cmdutil.ExitCodeError, ErrHTTP4xx},
		{500, cmdutil.ExitCodeError, ErrHTTP5xx},
		{502, cmdutil.ExitCodeError, ErrHTTP5xx},
		{503, cmdutil.ExitCodeError, ErrHTTP5xx},
		{504, cmdutil.ExitCodeError, ErrHTTP5xx},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{StatusCode: tc.status, Header: http.Header{}}
			apiErr := httpErrorEnvelopeBytes(resp, []byte(`{}`))
			assert.Equal(t, tc.wantExit, apiErr.ExitCode, "exit code for %d", tc.status)
			assert.Equal(t, tc.wantCode, apiErr.Envelope.Error.Code, "error code for %d", tc.status)
		})
	}
}

// TestRunPaginate_SafetyValveBreaksAreNotTruncation pins that the no-progress
// safety valves (server echoes the cursor, or returns a non-empty cursor with
// zero items) exit the loop without producing ErrPartialPagination.
func TestRunPaginate_SafetyValveBreaksAreNotTruncation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "server echoes the last cursor",
			handler: func(w http.ResponseWriter, r *http.Request) {
				tok := r.URL.Query().Get("continuationToken")
				if tok == "" {
					fmt.Fprintln(w, `{"items":[{"p":1}],"continuationToken":"stuck"}`)
					return
				}
				fmt.Fprintln(w, `{"items":[{"p":2}],"continuationToken":"stuck"}`)
			},
		},
		{
			name: "non-empty cursor with empty page",
			handler: func(w http.ResponseWriter, r *http.Request) {
				tok := r.URL.Query().Get("continuationToken")
				if tok == "" {
					fmt.Fprintln(w, `{"items":[{"p":1}],"continuationToken":"next"}`)
					return
				}
				fmt.Fprintln(w, `{"items":[],"continuationToken":"still-next"}`)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)

			apiClient := client.NewClient(srv.URL, "", false, nil)
			req := paginateRequest{Method: "GET", Path: "/list", BaseQuery: url.Values{}, Accept: "application/json"}
			var buf bytes.Buffer
			require.NoError(t, runPaginate(t.Context(), &buf, apiClient, req, &apiCommand{}))
		})
	}
}

// TestRunPaginate_TruncationRespectsSilent pins that the accumulated-body
// flush on truncation still respects --silent (no stdout payload).
//
//nolint:paralleltest // mutates os.Stdout and PaginationLimit
func TestRunPaginate_TruncationRespectsSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"items":[{"a":1}],"continuationToken":"keep-going"}`)
	}))
	t.Cleanup(srv.Close)

	origLimit := PaginationLimit
	PaginationLimit = 2
	t.Cleanup(func() { PaginationLimit = origLimit })

	apiClient := client.NewClient(srv.URL, "", false, nil)
	req := paginateRequest{Method: "GET", Path: "/list", BaseQuery: url.Values{}, Accept: "application/json"}
	var buf bytes.Buffer
	_ = runPaginate(t.Context(), &buf, apiClient, req, &apiCommand{silent: true})
	assert.Empty(t, strings.TrimSpace(buf.String()), "--silent must suppress truncation flush")
}

// TestCursorFromLink exercises the four recognised advance-marker names
// plus the non-matching and malformed cases.
func TestCursorFromLink(t *testing.T) {
	t.Parallel()
	cases := []struct {
		link       string
		wantCursor string
		wantParam  string
	}{
		{"https://api.example/list?cursor=abc", "abc", "cursor"},
		{"https://api.example/list?continuationToken=xyz", "xyz", "continuationToken"},
		{"/list?page=3", "3", "page"},
		{"/list?offset=50", "50", "offset"},
		{"/list?cursor=first&continuationToken=second", "first", "cursor"}, // cursor wins
		{"/list?other=foo", "", ""},
		{"", "", ""},
		{"://bad", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.link, func(t *testing.T) {
			t.Parallel()
			cursor, param := cursorFromLink(tc.link)
			assert.Equal(t, tc.wantCursor, cursor)
			assert.Equal(t, tc.wantParam, param)
		})
	}
}
