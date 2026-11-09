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
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
	staticSource = "static"
	cacheFile    = "notifications.json"
	cacheTTL     = 24 * time.Hour
	fetchTimeout = 3 * time.Second
)

type Severity string

const (
	Info    Severity = "info"
	Warning Severity = "warning"
	Error   Severity = "error"
)

// Notification is a single message, along with the conditions under which it should be shown.
type Notification struct {
	Title         string   `json:"title"`
	ID            string   `json:"identifier"`
	Overview      string   `json:"overview"`
	Severity      Severity `json:"severity"`
	Where         Where    `json:"where"`
	SchemaVersion uint     `json:"schemaVersion"`
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

	if cache == nil {
		cache = map[string]cacheEntry{}
	}
	cache[staticSource] = cacheEntry{FetchedAt: time.Now(), Notifications: ns}
	if err := writeCache(cache); err != nil {
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

func cachePath() (string, error) {
	return workspace.GetPulumiPath(cacheFile)
}

func readCache() map[string]cacheEntry {
	path, err := cachePath()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache map[string]cacheEntry
	if err := json.Unmarshal(b, &cache); err != nil {
		return nil
	}
	return cache
}

func writeCache(cache map[string]cacheEntry) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// Print writes the notifications that apply to the given CLI version to the sink's stderr.
func Print(sink diag.Sink, ns []Notification, cliVersion string) {
	cli, err := semver.ParseTolerant(cliVersion)
	for _, n := range ns {
		if applies(n, cli, err == nil) {
			sink.Infoerrf(diag.RawMessage("", render(n, cliVersion)))
		}
	}
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

func render(n Notification, cliVersion string) string {
	var prefix string
	switch n.Severity {
	case Warning:
		prefix = colors.SpecWarning + "WARNING:" + colors.Reset
	case Error:
		prefix = colors.SpecError + "ERROR:" + colors.Reset
	case Info:
		fallthrough
	default:
		prefix = "INFO:"
	}

	resolve := strings.NewReplacer("{cli}", cliVersion)
	overview := strings.ReplaceAll(resolve.Replace(n.Overview), "\n", "\n  ")
	return fmt.Sprintf("\n%s %s\n\n  %s\n", prefix, resolve.Replace(n.Title), overview)
}
