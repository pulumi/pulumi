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

// Package notifications fetches, caches and prints the messages Pulumi publishes to CLI users.
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// StaticURL is where the notifications sent to every CLI user are published.
const StaticURL = "https://docs.pulumi.com/cli-notifications.json"

const (
	staticSource  = "static"
	cacheFile     = "notifications.json"
	dismissedFile = "dismissed-notifications.json"
	shownFile     = "shown-notifications.json"
	cacheTTL      = 24 * time.Hour
	fetchTimeout  = 3 * time.Second
	minPrefix     = 3
	// warningInterval is how long to wait before showing a warning again.
	warningInterval = 24 * time.Hour
)

// Notification is a single message, along with the conditions under which it should be shown.
type Notification struct {
	Title string `json:"title"`
	// ID is a UUID, so that IDs from the static file and from Pulumi Cloud never collide.
	ID       string        `json:"identifier"`
	Overview string        `json:"overview"`
	Severity diag.Severity `json:"severity"`
	Where    Where         `json:"where"`
	// SchemaVersion is the version of this Notification format. Entries with any version other
	// than 1 are skipped, so that the format can change without older CLIs misreading it.
	SchemaVersion uint `json:"schemaVersion"`
}

// Where holds the version ranges a user's environment must match for a notification to apply. Each
// range uses github.com/blang/semver range syntax, such as ">=3.100.0 <3.200.0".
type Where struct {
	// Providers maps provider names to the range of versions the notification applies to.
	Providers map[string]string `json:"providers,omitempty"`
	CLI       string            `json:"cli,omitempty"`
}

// UnmarshalJSON rejects conditions this CLI doesn't know, so that it skips a notification rather than
// showing it to users the condition would have excluded.
func (w *Where) UnmarshalJSON(b []byte) error {
	type where Where
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode((*where)(w))
}

type cacheEntry struct {
	FetchedAt     time.Time      `json:"fetchedAt"`
	Notifications []Notification `json:"notifications"`
}

type source struct {
	key   string
	fetch func(context.Context) ([]Notification, error)
}

// sources returns where to fetch notifications from: the static file, and the default org of the
// current Pulumi Cloud login, if there is one.
func sources(staticURL string) []source {
	static := source{staticSource, func(ctx context.Context) ([]Notification, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, staticURL, nil)
		if err != nil {
			return nil, err
		}
		return decodeResponse(http.DefaultClient.Do(req))
	}}

	// ponytail: like the update check, this only knows the stored current login, so
	// PULUMI_BACKEND_URL, project backend URLs and PULUMI_ACCESS_TOKEN-only logins get no Cloud
	// notifications. Resolve the backend as the commands do if those users need them.
	creds, err := workspace.GetStoredCredentials()
	if err != nil || creds.Current == "" || diy.IsDIYBackendURL(creds.Current) {
		return []source{static}
	}
	cloudURL, token := creds.Current, creds.Accounts[creds.Current].AccessToken

	org := env.DefaultOrg.Value()
	if org == "" {
		if config, err := workspace.GetPulumiConfig(); err == nil {
			org = config.BackendConfig[cloudURL].DefaultOrg
		}
	}

	cloud := source{cloudURL + "/" + org, func(ctx context.Context) ([]Notification, error) {
		c := client.NewClient(cloudURL, token, false, cmdutil.Diag())
		org := org
		if org == "" {
			resp, err := c.GetDefaultOrg(ctx)
			if err != nil {
				return nil, err
			}
			org = resp.GitHubLogin
		}
		if org == "" {
			return []Notification{}, nil
		}
		return decodeResponse(c.RawCall(
			ctx, http.MethodGet, "/api/orgs/"+url.PathEscape(org)+"/notifications", nil, nil, nil, false))
	}}
	return []source{static, cloud}
}

// stale returns the sources without a fresh cache entry.
func stale(ss []source, cache map[string]cacheEntry) []source {
	return slices.DeleteFunc(slices.Clone(ss), func(s source) bool {
		entry, ok := cache[s.key]
		return ok && time.Since(entry.FetchedAt) < cacheTTL
	})
}

// Start returns the notifications cached for every source, and fetches those that are missing or
// stale in the background. The returned function yields them all if the fetches have finished by
// the time it is called, and only the fresh cached ones otherwise, so it never delays the command.
func Start(ctx context.Context, staticURL string) func() []Notification {
	ss := sources(staticURL)
	cache := readCache()
	toFetch := stale(ss, cache)

	var cached []Notification
	for _, s := range ss {
		if !slices.ContainsFunc(toFetch, func(f source) bool { return f.key == s.key }) {
			cached = append(cached, cache[s.key].Notifications...)
		}
	}
	if len(toFetch) == 0 {
		return func() []Notification { return cached }
	}

	result := make(chan []Notification, 1)
	go func() {
		fetched, err := refresh(ctx, toFetch, cache)
		if err != nil {
			slog.InfoContext(ctx, "error fetching notifications", "err", err)
		}
		result <- append(slices.Clone(cached), fetched...)
	}()
	return func() []Notification {
		select {
		case ns := <-result:
			return ns
		default:
			return cached
		}
	}
}

// refresh fetches each source, caching and returning the notifications from those that succeed.
// Concurrent CLI processes may overwrite each other's cache; the losing process's entries are
// simply fetched again later.
func refresh(ctx context.Context, ss []source, cache map[string]cacheEntry) ([]Notification, error) {
	var ns []Notification
	var errs []error
	for _, s := range ss {
		fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
		fetched, err := s.fetch(fetchCtx)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.key, err))
			continue
		}
		cache[s.key] = cacheEntry{FetchedAt: time.Now(), Notifications: fetched}
		ns = append(ns, fetched...)
	}

	if len(errs) < len(ss) {
		if err := writeJSON(cacheFile, cache); err != nil {
			slog.InfoContext(ctx, "error caching notifications", "err", err)
		}
	}
	return ns, errors.Join(errs...)
}

func decodeResponse(resp *http.Response, err error) ([]Notification, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return decode(raw), nil
}

// decode skips entries it can't understand, so that one malformed or newer-format entry doesn't
// hide the others.
func decode(raw []json.RawMessage) []Notification {
	ns := []Notification{}
	for _, r := range raw {
		var n Notification
		if err := json.Unmarshal(r, &n); err != nil || n.SchemaVersion != 1 {
			continue
		}
		ns = append(ns, n)
	}
	return ns
}

func readJSON(name string, v any) bool {
	path, err := workspace.GetPulumiPath(name)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(b, v) == nil
}

func writeJSON(name string, v any) error {
	path, err := workspace.GetPulumiPath(name)
	if err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func readCache() map[string]cacheEntry {
	var cache map[string]cacheEntry
	if !readJSON(cacheFile, &cache) || cache == nil {
		return map[string]cacheEntry{}
	}
	return cache
}

// readDismissed returns the dismissed notification IDs, keyed by the source they came from.
func readDismissed() map[string][]string {
	var dismissed map[string][]string
	if !readJSON(dismissedFile, &dismissed) || dismissed == nil {
		return map[string][]string{}
	}
	return dismissed
}

// readShown returns when each notification was last shown, keyed by ID.
func readShown() map[string]time.Time {
	var shown map[string]time.Time
	if !readJSON(shownFile, &shown) || shown == nil {
		return map[string]time.Time{}
	}
	return shown
}

// Print writes the notifications that apply to the given CLI and provider versions, are due, and
// haven't been dismissed, to the sink's stderr. Errors are due every time, warnings once a day, and
// anything else only once.
func Print(sink diag.Sink, ns []Notification, cliVersion string, providers map[string][]semver.Version) {
	dismissed := map[string]bool{}
	for _, ids := range readDismissed() {
		for _, id := range ids {
			dismissed[id] = true
		}
	}
	shown := readShown()
	changed := false

	// Builds without a version stamped in at link time have an empty version, which won't parse.
	cli, err := semver.ParseTolerant(cliVersion)
	prefixes := shortPrefixes(ns)
	for _, n := range ns {
		if dismissed[n.ID] || !applies(n, cli, err == nil, providers) || !due(n, shown) {
			continue
		}
		sink.Infoerrf(diag.RawMessage("", render(n, cliVersion, providers, prefixes[strings.ToLower(n.ID)])))
		if n.Severity != diag.Error {
			shown[n.ID] = time.Now()
			changed = true
		}
	}
	if !changed {
		return
	}

	// Forget notifications that are no longer cached, which a source drops only once it has been
	// fetched without them.
	cached := map[string]bool{}
	for _, entry := range readCache() {
		for _, n := range entry.Notifications {
			cached[n.ID] = true
		}
	}
	if len(cached) > 0 {
		maps.DeleteFunc(shown, func(id string, _ time.Time) bool { return !cached[id] })
	}
	if err := writeJSON(shownFile, shown); err != nil {
		slog.Info("error recording shown notifications", "err", err)
	}
}

func due(n Notification, shown map[string]time.Time) bool {
	last, ok := shown[n.ID]
	if n.Severity == diag.Error || !ok {
		return true
	}
	return n.Severity == diag.Warning && time.Since(last) >= warningInterval
}

// Dismiss stops the notification whose ID starts with prefix from being shown again, and returns it.
func Dismiss(ctx context.Context, staticURL, prefix string) (Notification, error) {
	if len(prefix) < minPrefix {
		return Notification{}, fmt.Errorf("the ID prefix must be at least %d characters long", minPrefix)
	}

	ss := sources(staticURL)
	cache := readCache()
	if _, err := refresh(ctx, stale(ss, cache), cache); err != nil {
		slog.InfoContext(ctx, "error fetching notifications", "err", err)
	}

	type match struct {
		source       string
		notification Notification
	}
	var all []Notification
	var matches []match
	for _, s := range ss {
		for _, n := range cache[s.key].Notifications {
			all = append(all, n)
			if strings.HasPrefix(strings.ToLower(n.ID), strings.ToLower(prefix)) {
				matches = append(matches, match{s.key, n})
			}
		}
	}

	switch len(matches) {
	case 0:
		return Notification{}, fmt.Errorf("no notification ID starts with %q", prefix)
	case 1:
	default:
		prefixes := shortPrefixes(all)
		slices.SortFunc(matches, func(a, b match) int {
			return strings.Compare(a.notification.ID, b.notification.ID)
		})
		var msg strings.Builder
		fmt.Fprintf(&msg, "%q matches more than one notification:", prefix)
		for _, m := range matches {
			fmt.Fprintf(&msg, "\n  %s  %s", prefixes[strings.ToLower(m.notification.ID)], m.notification.Title)
		}
		return Notification{}, errors.New(msg.String())
	}

	m := matches[0]
	dismissed := readDismissed()
	if !slices.Contains(dismissed[m.source], m.notification.ID) {
		dismissed[m.source] = append(dismissed[m.source], m.notification.ID)
	}

	// Only prune against sources we hold a successful fetch for, so that a failed fetch, or a
	// different org, never brings a dismissed notification back.
	for source, ids := range dismissed {
		entry, ok := cache[source]
		if !ok {
			continue
		}
		dismissed[source] = slices.DeleteFunc(ids, func(id string) bool {
			return !slices.ContainsFunc(entry.Notifications, func(n Notification) bool { return n.ID == id })
		})
	}

	return m.notification, writeJSON(dismissedFile, dismissed)
}

// shortPrefixes maps each lowercased ID to its shortest prefix, of at least minPrefix characters,
// that no other ID shares.
func shortPrefixes(ns []Notification) map[string]string {
	ids := make([]string, len(ns))
	for i, n := range ns {
		ids[i] = strings.ToLower(n.ID)
	}
	slices.Sort(ids)

	prefixes := make(map[string]string, len(ids))
	for i, id := range ids {
		n := minPrefix
		if i > 0 {
			n = max(n, commonPrefix(id, ids[i-1])+1)
		}
		if i+1 < len(ids) {
			n = max(n, commonPrefix(id, ids[i+1])+1)
		}
		prefixes[id] = id[:min(n, len(id))]
	}
	return prefixes
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// applies reports whether the CLI version, and some version of each named provider, are in range.
func applies(n Notification, cli semver.Version, cliKnown bool, providers map[string][]semver.Version) bool {
	for name, r := range n.Where.Providers {
		inRange, err := semver.ParseRange(r)
		if err != nil || !slices.ContainsFunc(providers[name], inRange) {
			return false
		}
	}
	if n.Where.CLI == "" {
		return true
	}
	inRange, err := semver.ParseRange(n.Where.CLI)
	return err == nil && cliKnown && inRange(cli)
}

func render(n Notification, cliVersion string, providers map[string][]semver.Version, idPrefix string) string {
	prefix, ok := map[diag.Severity]string{
		diag.Warning: colors.SpecWarning + "WARNING:" + colors.Reset,
		diag.Error:   colors.SpecError + "ERROR:" + colors.Reset,
	}[n.Severity]
	if !ok {
		prefix = "INFO:"
	}

	vars := make([]string, 0, 2+2*len(providers))
	vars = append(vars, "{cli}", cliVersion)
	for name, versions := range providers {
		versions = slices.SortedFunc(slices.Values(versions), semver.Version.Compare)
		vs := make([]string, len(versions))
		for i, v := range versions {
			vs[i] = v.String()
		}
		vars = append(vars, "{providers."+name+"}", strings.Join(vs, ", "))
	}
	resolve := strings.NewReplacer(vars...)
	overview := strings.ReplaceAll(resolve.Replace(n.Overview), "\n", "\n  ")
	msg := fmt.Sprintf("\n%s %s\n\n  %s\n", prefix, resolve.Replace(n.Title), overview)
	// Infos are only shown once, so there is nothing to dismiss.
	if ok {
		msg += "\n  To hide this, run " +
			colors.SpecCommand + "pulumi notifications dismiss " + idPrefix + colors.Reset + ".\n"
	}
	return msg
}
