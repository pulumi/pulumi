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

package telemetry

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

const defaultPulse = 2 * time.Second

// InitArgs configures Init.
type InitArgs struct {
	// HomeDir is $PULUMI_HOME, which holds the telemetry config, pending events and the local log.
	HomeDir string
	// Diags receives the first-run notice and warnings about unusable telemetry state.
	Diags diag.Sink
	// Pulse is how often pending events are sent; zero uses the default.
	Pulse time.Duration
	// Env is the environment telemetry settings are read from.
	Env env.Env
	// CI makes Shutdown send pending events before returning, since a CI machine is unlikely to run the CLI again.
	CI bool
	// URL is the Pulumi Cloud API endpoint events are sent to.
	URL string
	// UserAgent is sent with every request, so the service can record the CLI version.
	UserAgent string
}

// KV is a single named telemetry fact. Facts can only be built in this package, so that every value sent is one
// the package knows about.
type KV interface {
	fact() (string, any)
}

type kv struct {
	name  string
	value any
}

func (f kv) fact() (string, any) { return f.name, f.value }

func numberFact(name string, value float64) KV { return kv{name, value} }

func boolFact(name string, value bool) KV { return kv{name, value} }

func enumFact(name string, value string) KV { return kv{name, value} }

func setFact(name string, values ...string) KV { return kv{name, append([]string{}, values...)} }

type event struct {
	Installation string         `json:"installation"`
	Invocation   string         `json:"invocation"`
	Timestamp    time.Time      `json:"timestamp"`
	Facts        map[string]any `json:"facts"`
}

type telemetry struct {
	installation string
	invocation   string
	send         bool
	logPath      string
	queue        *queue
	disabled     atomic.Bool
}

type logEntry struct {
	Sending   bool            `json:"sending"`
	Discarded string          `json:"discarded,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
}

type contextKey struct{}

// Init sets up telemetry for this invocation and returns a context carrying it. If telemetry is disabled, ctx is
// returned unchanged and the other functions in this package do nothing. Events are held until UseBackend says
// whether they may be sent.
func Init(ctx context.Context, args InitArgs) context.Context {
	t := newTelemetry(args)
	if t == nil {
		return ctx
	}
	if t.queue != nil {
		runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		t.queue.cancel = cancel
		go t.queue.run(runCtx)
	}
	return context.WithValue(ctx, contextKey{}, t)
}

func fromContext(ctx context.Context) *telemetry {
	t, _ := ctx.Value(contextKey{}).(*telemetry)
	return t
}

// Emit records an event made of the given facts.
func Emit(ctx context.Context, info ...KV) {
	t := fromContext(ctx)
	if t == nil || t.disabled.Load() || len(info) == 0 {
		return
	}

	facts := make(map[string]any, len(info))
	for _, i := range info {
		name, value := i.fact()
		facts[name] = value
	}
	line, err := json.Marshal(event{
		Installation: t.installation,
		Invocation:   t.invocation,
		Timestamp:    time.Now().UTC(),
		Facts:        facts,
	})
	if err != nil {
		slog.DebugContext(ctx, "could not marshal telemetry event", "err", err)
		return
	}

	t.log(logEntry{Sending: t.send, Event: line})
	if t.send {
		t.queue.add(line)
	}
}

func (t *telemetry) log(entry logEntry) {
	if t.logPath == "" {
		return
	}
	line, err := json.Marshal(entry)
	if err != nil {
		slog.Debug("could not marshal telemetry log entry", "err", err)
		return
	}
	f, err := os.OpenFile(t.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		slog.Debug("could not open telemetry log", "err", err)
		return
	}
	defer func() { logCleanupErr(f.Close()) }()
	if _, err := f.Write(append(line, '\n')); err != nil {
		slog.Debug("could not write telemetry log", "err", err)
	}
}

// Headers returns the installation and invocation IDs to attach to requests to Pulumi Cloud, or no headers if
// telemetry is not being sent.
func Headers(ctx context.Context) http.Header {
	header := http.Header{}
	if t := fromContext(ctx); t != nil && t.send && !t.disabled.Load() {
		header.Set("X-Pulumi-Installation-Id", t.installation)
		header.Set("X-Pulumi-Invocation-Id", t.invocation)
	}
	return header
}

// Shutdown stops telemetry for this invocation. In CI it first tries to send pending events; otherwise they stay on
// disk for a later invocation to send.
func Shutdown(ctx context.Context) {
	if t := fromContext(ctx); t != nil && t.queue != nil {
		t.queue.shutdown(ctx)
	}
}

var (
	eligibleHosts = []string{"api.pulumi.com", "app.pulumi.com"}
	diySchemes    = []string{"file", "s3", "azblob", "gs"}
)

// Eligible reports whether telemetry may be sent for an invocation using backendURL: DIY backends, Pulumi Cloud,
// and no backend at all are eligible, while self-hosted and other regional Pulumi Cloud backends are not.
func Eligible(backendURL string) bool {
	if backendURL == "" {
		return true
	}
	u, err := url.Parse(backendURL)
	if err != nil {
		return false
	}
	switch {
	case slices.Contains(diySchemes, u.Scheme):
		return true
	case u.Scheme == "http" || u.Scheme == "https":
		return slices.Contains(eligibleHosts, strings.ToLower(u.Hostname()))
	default:
		return false
	}
}

// UseBackend records which backend the invocation uses. Held events are released for sending if the backend is
// eligible and discarded otherwise.
func UseBackend(ctx context.Context, backendURL string) {
	t := fromContext(ctx)
	if t == nil || t.disabled.Load() {
		return
	}
	if !Eligible(backendURL) {
		t.disabled.Store(true)
		if t.queue != nil {
			t.queue.discard()
		}
		t.log(logEntry{Discarded: t.invocation})
		return
	}
	Emit(ctx, backendFact(backendURL))
	if t.queue != nil {
		t.queue.release()
	}
}
