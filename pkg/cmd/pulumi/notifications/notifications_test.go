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

package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

var survey = Notification{
	Title:         "We want to hear from you!",
	ID:            "90abcdef-0123-4567-890a-bcdef0123456",
	Overview:      "Fill out a quick survey here: https://example.com",
	Severity:      diag.Info,
	SchemaVersion: 1,
}

func TestDecode(t *testing.T) {
	t.Parallel()

	raw := []json.RawMessage{
		json.RawMessage(`{"title": "ok", "identifier": "a", "schemaVersion": 1}`),
		json.RawMessage(`{"title": "newer", "identifier": "b", "schemaVersion": 2}`),
		json.RawMessage(`{"title": "malformed", "identifier": "c", "schemaVersion": "1"}`),
	}

	ns := decode(raw)
	require.Len(t, ns, 1)
	assert.Equal(t, "ok", ns[0].Title)
}

func TestApplies(t *testing.T) {
	t.Parallel()

	cli := semver.MustParse("3.200.0")
	cases := []struct {
		name     string
		where    Where
		cliKnown bool
		want     bool
	}{
		{"no conditions", Where{}, true, true},
		{"no conditions, unknown CLI", Where{}, false, true},
		{"CLI in range", Where{CLI: "< 3.201.0"}, true, true},
		{"CLI out of range", Where{CLI: ">= 3.201.0"}, true, false},
		{"CLI range, unknown CLI", Where{CLI: "< 3.201.0"}, false, false},
		{"invalid range", Where{CLI: "not a range"}, true, false},
		{"provider condition", Where{Providers: map[string]string{"aws": ">= 0.0.0"}}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, applies(Notification{Where: c.where}, cli, c.cliKnown))
		})
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	n := Notification{
		Title:    "CLI {cli} is no longer actively maintained",
		Overview: "You are running {cli}.\nPlease upgrade.",
		Severity: diag.Warning,
	}

	assert.Equal(t,
		"\nWARNING: CLI v3.100.0 is no longer actively maintained\n\n  You are running v3.100.0.\n  Please upgrade.\n"+
			"\n  To hide this, run pulumi notifications dismiss 90a.\n",
		colors.Never.Colorize(render(n, "v3.100.0", "90a")))
}

func TestShortPrefixes(t *testing.T) {
	t.Parallel()

	ns := []Notification{{ID: "ABC111"}, {ID: "abc222"}, {ID: "abd"}, {ID: "zzzzzz"}}
	assert.Equal(t, map[string]string{
		"abc111": "abc1",
		"abc222": "abc2",
		"abd":    "abd",
		"zzzzzz": "zzz",
	}, shortPrefixes(ns))
}

func serve(t *testing.T, body string, status int) (url string, calls *int) {
	calls = new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, calls
}

func TestStart_UsesFreshCache(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	require.NoError(t, writeJSON(cacheFile, map[string]cacheEntry{
		staticSource: {FetchedAt: time.Now(), Notifications: []Notification{survey}},
	}))
	url, calls := serve(t, `[]`, http.StatusOK)

	assert.Equal(t, []Notification{survey}, Start(t.Context(), url)())
	assert.Equal(t, 0, *calls)
}

func TestStart_RefreshesStaleCache(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	require.NoError(t, writeJSON(cacheFile, map[string]cacheEntry{
		staticSource: {FetchedAt: time.Now().Add(-25 * time.Hour)},
	}))
	body, err := json.Marshal([]Notification{survey})
	require.NoError(t, err)
	url, _ := serve(t, string(body), http.StatusOK)

	pending := Start(t.Context(), url)
	var got []Notification
	require.Eventually(t, func() bool {
		got = pending()
		return got != nil
	}, 5*time.Second, 10*time.Millisecond)

	assert.Equal(t, []Notification{survey}, got)
	assert.Equal(t, []Notification{survey}, readCache()[staticSource].Notifications)
}

func TestRefresh_FailureKeepsCache(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	stale := map[string]cacheEntry{
		staticSource: {FetchedAt: time.Now().Add(-25 * time.Hour).UTC(), Notifications: []Notification{survey}},
	}
	require.NoError(t, writeJSON(cacheFile, stale))
	url, _ := serve(t, `oops`, http.StatusInternalServerError)

	_, err := refresh(t.Context(), url, readCache())
	require.ErrorContains(t, err, "500 Internal Server Error")
	assert.Equal(t, stale, readCache())
}

func TestDismiss(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	other := Notification{Title: "Upgrade", ID: "90abffff-0000-0000-0000-000000000000", SchemaVersion: 1}
	third := Notification{Title: "Third", ID: "12345678-0000-0000-0000-000000000000", SchemaVersion: 1}
	require.NoError(t, writeJSON(cacheFile, map[string]cacheEntry{
		staticSource: {FetchedAt: time.Now(), Notifications: []Notification{survey, other, third}},
	}))
	require.NoError(t, writeJSON(dismissedFile, map[string][]string{
		staticSource: {"no-longer-sent"},
		"acme":       {"not-fetched"},
	}))

	_, err := Dismiss(t.Context(), "", "90")
	require.ErrorContains(t, err, "at least 3 characters")

	_, err = Dismiss(t.Context(), "", "777")
	require.ErrorContains(t, err, `no notification ID starts with "777"`)

	_, err = Dismiss(t.Context(), "", "90ab")
	require.EqualError(t, err, `"90ab" matches more than one notification:`+
		"\n  90abc  We want to hear from you!\n  90abf  Upgrade")

	n, err := Dismiss(t.Context(), "", "90ABC")
	require.NoError(t, err)
	assert.Equal(t, survey, n)
	assert.Equal(t, map[string][]string{
		staticSource: {survey.ID},
		"acme":       {"not-fetched"},
	}, readDismissed())

	var stderr bytes.Buffer
	Print(diag.DefaultSink(io.Discard, &stderr, diag.FormatOptions{Color: colors.Never}),
		[]Notification{survey, other, third}, "v3.200.0")
	assert.NotContains(t, stderr.String(), survey.Title)
	assert.Contains(t, stderr.String(), "pulumi notifications dismiss 90abf")
	assert.Contains(t, stderr.String(), "pulumi notifications dismiss 123")
}
