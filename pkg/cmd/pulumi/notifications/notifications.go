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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// StaticURL is where the notifications sent to every CLI user are published.
const StaticURL = "https://static.pulumi.com/notifications.json"

const (
	staticSource  = "static"
	cacheFile     = "notifications.json"
	dismissedFile = "dismissed-notifications.json"
	cacheTTL      = 24 * time.Hour
	fetchTimeout  = 3 * time.Second
	minPrefix     = 3
)

// Notification is a single message, along with the conditions under which it should be shown.
type Notification struct {
	Title         string        `json:"title"`
	ID            string        `json:"identifier"`
	Overview      string        `json:"overview"`
	Severity      diag.Severity `json:"severity"`
	Where         Where         `json:"where"`
	SchemaVersion uint          `json:"schemaVersion"`
}

// Where holds the version ranges a user's environment must match for a notification to apply.
type Where struct {
	Providers map[string]string `json:"providers,omitempty"`
	CLI       string            `json:"cli,omitempty"`
}

type cacheEntry struct {
	FetchedAt     time.Time      `json:"fetchedAt"`
	Notifications []Notification `json:"notifications"`
}

// Start returns cached notifications if they are fresh, and otherwise fetches them in the background.
// The returned function yields the notifications if they have arrived by the time it is called, and
// nil otherwise, so it never delays the command.
func Start(ctx context.Context, url string) func() []Notification {
	cache := readCache()
	if entry, ok := cache[staticSource]; ok && time.Since(entry.FetchedAt) < cacheTTL {
		return func() []Notification { return entry.Notifications }
	}

	result := make(chan []Notification, 1)
	go func() {
		ns, err := refresh(ctx, url, cache)
		if err != nil {
			slog.InfoContext(ctx, "error fetching notifications", "err", err)
		}
		result <- ns
	}()
	return func() []Notification {
		select {
		case ns := <-result:
			return ns
		default:
			return nil
		}
	}
}

// refresh fetches the notifications and, if that succeeds, caches them.
func refresh(ctx context.Context, url string, cache map[string]cacheEntry) ([]Notification, error) {
	ns, err := fetch(ctx, url)
	if err != nil {
		return nil, err
	}

	cache[staticSource] = cacheEntry{FetchedAt: time.Now(), Notifications: ns}
	if err := writeJSON(cacheFile, cache); err != nil {
		slog.InfoContext(ctx, "error caching notifications", "err", err)
	}
	return ns, nil
}

func fetch(ctx context.Context, url string) ([]Notification, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
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

// Print writes the notifications that apply to the given CLI version, and haven't been dismissed,
// to the sink's stderr.
func Print(sink diag.Sink, ns []Notification, cliVersion string) {
	dismissed := map[string]bool{}
	for _, ids := range readDismissed() {
		for _, id := range ids {
			dismissed[id] = true
		}
	}

	cli, err := semver.ParseTolerant(cliVersion)
	prefixes := shortPrefixes(ns)
	for _, n := range ns {
		if !dismissed[n.ID] && applies(n, cli, err == nil) {
			sink.Infoerrf(diag.RawMessage("", render(n, cliVersion, prefixes[strings.ToLower(n.ID)])))
		}
	}
}

// Dismiss stops the notification whose ID starts with prefix from being shown again, and returns it.
func Dismiss(ctx context.Context, url, prefix string) (Notification, error) {
	if len(prefix) < minPrefix {
		return Notification{}, fmt.Errorf("the ID prefix must be at least %d characters long", minPrefix)
	}

	cache := readCache()
	if entry, ok := cache[staticSource]; !ok || time.Since(entry.FetchedAt) >= cacheTTL {
		if _, err := refresh(ctx, url, cache); err != nil {
			slog.InfoContext(ctx, "error fetching notifications", "err", err)
		}
	}

	type match struct {
		source       string
		notification Notification
	}
	var all []Notification
	var matches []match
	for source, entry := range cache {
		for _, n := range entry.Notifications {
			all = append(all, n)
			if strings.HasPrefix(strings.ToLower(n.ID), strings.ToLower(prefix)) {
				matches = append(matches, match{source, n})
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

func applies(n Notification, cli semver.Version, cliKnown bool) bool {
	// We don't know the versions of the project's providers, so provider conditions never match.
	if len(n.Where.Providers) > 0 {
		return false
	}
	if n.Where.CLI == "" {
		return true
	}
	inRange, err := semver.ParseRange(n.Where.CLI)
	return err == nil && cliKnown && inRange(cli)
}

func render(n Notification, cliVersion, idPrefix string) string {
	prefix, ok := map[diag.Severity]string{
		diag.Warning: colors.SpecWarning + "WARNING:" + colors.Reset,
		diag.Error:   colors.SpecError + "ERROR:" + colors.Reset,
	}[n.Severity]
	if !ok {
		prefix = "INFO:"
	}

	resolve := strings.NewReplacer("{cli}", cliVersion)
	overview := strings.ReplaceAll(resolve.Replace(n.Overview), "\n", "\n  ")
	dismiss := colors.SpecCommand + "pulumi notifications dismiss " + idPrefix + colors.Reset
	return fmt.Sprintf("\n%s %s\n\n  %s\n\n  To hide this, run %s.\n",
		prefix, resolve.Replace(n.Title), overview, dismiss)
}
