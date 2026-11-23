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
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
		json.RawMessage(`{"title": "unknown condition", "identifier": "d", "where": {"command": "new"}, "schemaVersion": 1}`),
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
		"\nWARNING [90a]: CLI v3.100.0 is no longer actively maintained\n\n  You are running v3.100.0.\n  Please upgrade.\n",
		colors.Never.Colorize(render(n, "v3.100.0", "90a")))

	assert.NotContains(t, render(survey, "v3.100.0", "90a"), "90a")
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

	_, err := refresh(t.Context(), sources(url), readCache())
	require.ErrorContains(t, err, "500 Internal Server Error")
	assert.Equal(t, stale, readCache())
}

func TestDismiss(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	other := Notification{
		Title: "Upgrade", ID: "90abffff-0000-0000-0000-000000000000", Severity: diag.Warning, SchemaVersion: 1,
	}
	third := Notification{
		Title: "Third", ID: "12345678-0000-0000-0000-000000000000", Severity: diag.Error, SchemaVersion: 1,
	}
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
	assert.Contains(t, stderr.String(), "WARNING [90abf]: Upgrade")
	assert.Contains(t, stderr.String(), "ERROR [123]: Third")
	assert.Equal(t, 1, strings.Count(stderr.String(), "pulumi notifications dismiss <id>"))
}

func TestCloudNotifications(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_CONSOLE_DOMAIN", "app.example.com")
	static, err := json.Marshal([]Notification{survey})
	require.NoError(t, err)

	var dismissed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/static":
			_, _ = w.Write(static)
		case "/api/user/organizations/default":
			_, _ = w.Write([]byte(`{"GitHubLogin": "acme"}`))
		case "/api/preview/orgs/acme/notifications":
			assert.Equal(t, "token secret", r.Header.Get("Authorization"))
			assert.Equal(t, "true", r.URL.Query().Get("unreadOnly"))
			_, _ = w.Write([]byte(`{"notifications": [
				{"id": "b1111111", "title": "Payment failed", "body": "Update your card.",
				 "deepLinkUrl": "/acme/settings/billing", "dismissible": false},
				{"id": "c2222222", "title": "Task complete", "dismissible": true}
			], "unreadCount": 2}`))
		case "/api/preview/orgs/acme/notifications/dismiss":
			b, _ := io.ReadAll(r.Body)
			dismissed = string(b)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
		Current:  srv.URL,
		Accounts: map[string]workspace.Account{srv.URL: {AccessToken: "secret"}},
	}))

	billing := Notification{
		Title: "Payment failed", ID: "b1111111", Severity: diag.Error, SchemaVersion: 1,
		Overview: "Update your card.\nhttp://app.example.com/acme/settings/billing",
	}
	task := Notification{Title: "Task complete", ID: "c2222222", Severity: diag.Warning, SchemaVersion: 1}

	pending := Start(t.Context(), srv.URL+"/static")
	var got []Notification
	require.Eventually(t, func() bool {
		got = pending()
		return len(got) == 3
	}, 5*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, []Notification{survey, billing, task}, got)

	n, err := Dismiss(t.Context(), srv.URL+"/static", "c22")
	require.NoError(t, err)
	assert.Equal(t, task, n)
	assert.JSONEq(t, `{"ids": ["c2222222"]}`, dismissed)
	assert.Equal(t, []string{task.ID}, readDismissed()[srv.URL+"/"])
}

func TestPrint_Frequency(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	outage := Notification{Title: "Outage", ID: "e", Severity: diag.Error, SchemaVersion: 1}
	upgrade := Notification{Title: "Upgrade", ID: "w", Severity: diag.Warning, SchemaVersion: 1}
	ns := []Notification{survey, outage, upgrade}
	require.NoError(t, writeJSON(cacheFile, map[string]cacheEntry{
		staticSource: {FetchedAt: time.Now(), Notifications: ns},
	}))
	require.NoError(t, writeJSON(shownFile, map[string]time.Time{"no-longer-sent": time.Now()}))

	show := func() string {
		var stderr bytes.Buffer
		Print(diag.DefaultSink(io.Discard, &stderr, diag.FormatOptions{Color: colors.Never}), ns, "v3.200.0")
		return stderr.String()
	}

	first := show()
	assert.Contains(t, first, survey.Title)
	assert.Contains(t, first, outage.Title)
	assert.Contains(t, first, upgrade.Title)
	assert.Contains(t, first, "dismiss <id>")
	assert.Equal(t, []string{survey.ID, upgrade.ID}, slices.Sorted(maps.Keys(readShown())))

	second := show()
	assert.NotContains(t, second, survey.Title)
	assert.Contains(t, second, outage.Title)
	assert.NotContains(t, second, upgrade.Title)

	shown := readShown()
	shown[upgrade.ID] = time.Now().Add(-25 * time.Hour)
	require.NoError(t, writeJSON(shownFile, shown))
	third := show()
	assert.NotContains(t, third, survey.Title)
	assert.Contains(t, third, upgrade.Title)

	ns = []Notification{{Title: "Tip", ID: "i", Severity: diag.Info, SchemaVersion: 1}}
	infoOnly := show()
	assert.Contains(t, infoOnly, "Tip")
	assert.NotContains(t, infoOnly, "dismiss")
}
