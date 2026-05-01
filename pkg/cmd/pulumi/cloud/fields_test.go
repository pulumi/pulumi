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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseField_TypeInference(t *testing.T) {
	t.Parallel()
	cases := []struct {
		spec string
		want any
	}{
		{"name=pulumi", "pulumi"},
		{"count=42", float64(42)},
		{"rate=3.14", 3.14},
		{"enabled=true", true},
		{"enabled=false", false},
		{"val=null", nil},
		{"s=12abc", "12abc"},
	}
	for _, tc := range cases {
		f, err := parseField(tc.spec, true, nil)
		require.NoError(t, err, "spec=%q", tc.spec)
		assert.Equal(t, tc.want, f.Value, "spec=%q", tc.spec)
	}
}

func TestParseField_RawNoCoercion(t *testing.T) {
	t.Parallel()
	f, err := parseField("count=42", false, nil)
	require.NoError(t, err)
	assert.Equal(t, "42", f.Value)
	assert.True(t, f.Raw)
}

func TestParseField_AtStdin(t *testing.T) {
	t.Parallel()
	stdin := strings.NewReader("hello from stdin")
	f, err := parseField("note=@-", true, stdin)
	require.NoError(t, err)
	assert.Equal(t, "hello from stdin", f.Value)
}

func TestParseField_EmptyKey(t *testing.T) {
	t.Parallel()
	_, err := parseField("=value", true, nil)
	assert.Error(t, err)
}

func TestParseField_NoEquals(t *testing.T) {
	t.Parallel()
	_, err := parseField("nokvhere", true, nil)
	assert.Error(t, err)
}

func TestParseHeaders(t *testing.T) {
	t.Parallel()
	hs, err := parseHeaders([]string{
		"Accept: application/json",
		"X-Custom:value",
		"Idempotency-Key: abc:def",
	})
	require.NoError(t, err)
	require.Len(t, hs, 3)
	assert.Equal(t, ParsedHeader{"Accept", "application/json"}, hs[0])
	assert.Equal(t, ParsedHeader{"X-Custom", "value"}, hs[1])
	// Only the first colon splits; later ones stay in the value.
	assert.Equal(t, ParsedHeader{"Idempotency-Key", "abc:def"}, hs[2])
}

func TestParseHeaders_Invalid(t *testing.T) {
	t.Parallel()
	_, err := parseHeaders([]string{"NoColon"})
	assert.Error(t, err)
	_, err = parseHeaders([]string{": value"})
	assert.Error(t, err)
}

func TestMethodDefaultsToPost(t *testing.T) {
	t.Parallel()
	assert.False(t, methodDefaultsToPost(0, false, false))
	assert.True(t, methodDefaultsToPost(1, false, false))
	assert.True(t, methodDefaultsToPost(0, true, false))
	assert.True(t, methodDefaultsToPost(0, false, true))
}

func TestParseField_JSONObject(t *testing.T) {
	t.Parallel()
	f, err := parseField(`tags={"env":"prod","team":"platform"}`, true, nil)
	require.NoError(t, err)
	m, ok := f.Value.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", f.Value)
	assert.Equal(t, "prod", m["env"])
	assert.Equal(t, "platform", m["team"])
}

func TestParseField_JSONArray(t *testing.T) {
	t.Parallel()
	f, err := parseField(`members=["alice","bob"]`, true, nil)
	require.NoError(t, err)
	a, ok := f.Value.([]any)
	require.True(t, ok, "expected []any, got %T", f.Value)
	assert.Equal(t, []any{"alice", "bob"}, a)
}

func TestParseField_JSONMalformedFallsThroughToString(t *testing.T) {
	t.Parallel()
	// Starts with `{` but isn't valid JSON — preserved as the literal string
	// rather than erroring, so users can send braces in plain values.
	f, err := parseField(`note={literal}`, true, nil)
	require.NoError(t, err)
	assert.Equal(t, "{literal}", f.Value)
}

func TestParseField_RawKeepsJSONAsString(t *testing.T) {
	t.Parallel()
	// `-f` disables all coercion, including JSON parsing.
	f, err := parseField(`tags={"env":"prod"}`, false, nil)
	require.NoError(t, err)
	assert.Equal(t, `{"env":"prod"}`, f.Value)
	assert.True(t, f.Raw)
}
