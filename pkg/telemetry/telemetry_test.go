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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

type request struct {
	UserAgent string
	Events    []event
}

type fakeService struct {
	*httptest.Server
	status  int
	hang    bool
	release chan struct{}

	mu       sync.Mutex
	requests []request
}

func newFakeService(t *testing.T, status int) *fakeService {
	s := &fakeService{status: status}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/telemetry/v1", r.URL.Path)
		var body struct{ Events []event }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		s.mu.Lock()
		s.requests = append(s.requests, request{UserAgent: r.UserAgent(), Events: body.Events})
		hang, release := s.hang, s.release
		s.mu.Unlock()
		if hang {
			<-r.Context().Done()
			return
		}
		if release != nil {
			<-release
		}
		w.WriteHeader(s.status)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fakeService) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request{}, s.requests...)
}

type fixture struct {
	args    InitArgs
	diags   *bytes.Buffer
	service *fakeService
}

func newFixture(t *testing.T, vars map[string]string) *fixture {
	var diags bytes.Buffer
	service := newFakeService(t, http.StatusNoContent)
	return &fixture{
		args: InitArgs{
			HomeDir:   t.TempDir(),
			Diags:     diag.DefaultSink(io.Discard, &diags, diag.FormatOptions{Color: colors.Never}),
			Pulse:     time.Hour,
			Env:       env.NewEnv(env.MapStore(vars)),
			URL:       service.URL,
			UserAgent: "pulumi-cli/1 (v3.275.0; linux)",
		},
		diags:   &diags,
		service: service,
	}
}

func (f *fixture) configPath() string { return filepath.Join(f.args.HomeDir, configFile) }

func (f *fixture) walDir() string { return filepath.Join(f.args.HomeDir, "telemetry") }

func (f *fixture) readConfig(t *testing.T) config {
	data, err := os.ReadFile(f.configPath())
	require.NoError(t, err)
	var cfg config
	require.NoError(t, json.Unmarshal(data, &cfg))
	return cfg
}

func (f *fixture) writeConfig(t *testing.T, cfg string) {
	require.NoError(t, os.WriteFile(f.configPath(), []byte(cfg), 0o600))
}

func TestDisabledByEnv(t *testing.T) {
	t.Parallel()

	for name, vars := range map[string]map[string]string{
		"DO_NOT_TRACK":           {"DO_NOT_TRACK": "1"},
		"PULUMI_TELEMETRY=0":     {"PULUMI_TELEMETRY": "0"},
		"PULUMI_TELEMETRY=false": {"PULUMI_TELEMETRY": "false"},
		"PULUMI_TELEMETRY=maybe": {"PULUMI_TELEMETRY": "maybe"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, vars)

			ctx := Init(t.Context(), f.args)

			require.Nil(t, fromContext(ctx))
			require.NoFileExists(t, f.configPath())
		})
	}
}

func TestPrecedence(t *testing.T) {
	t.Parallel()

	const installation = "0b3b5a4e-6c1f-4c55-9d0a-2a7f6f0d8c11"
	setting := func(value string) string {
		return `{"version": 1, "installation": "` + installation + `", "notified": true` + value + `}`
	}
	cases := map[string]struct {
		vars   map[string]string
		config string
		send   bool
	}{
		"PULUMI_TELEMETRY beats DO_NOT_TRACK": {
			vars: map[string]string{"DO_NOT_TRACK": "1", "PULUMI_TELEMETRY": "1"},
			send: true,
		},
		"PULUMI_TELEMETRY beats config": {
			vars:   map[string]string{"PULUMI_TELEMETRY": "0"},
			config: setting(`, "telemetry": "enabled"`),
		},
		"config enabled beats DO_NOT_TRACK": {
			vars:   map[string]string{"DO_NOT_TRACK": "1"},
			config: setting(`, "telemetry": "enabled"`),
			send:   true,
		},
		"config disabled": {
			config: setting(`, "telemetry": "disabled"`),
		},
		"DO_NOT_TRACK without explicit setting": {
			vars:   map[string]string{"DO_NOT_TRACK": "1"},
			config: setting(""),
		},
		"default": {
			config: setting(""),
			send:   true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, c.vars)
			if c.config != "" {
				f.writeConfig(t, c.config)
			}

			ctx := Init(t.Context(), f.args)
			defer Shutdown(ctx)

			tel := fromContext(ctx)
			if c.send {
				require.NotNil(t, tel)
				require.True(t, tel.send)
			} else {
				require.Nil(t, tel)
			}
			require.NotContains(t, f.diags.String(), "collects anonymous usage telemetry")
		})
	}
}

func TestFirstRunShowsNoticeWithoutSending(t *testing.T) {
	t.Parallel()

	f := newFixture(t, nil)

	ctx := Init(t.Context(), f.args)

	require.Nil(t, fromContext(ctx))
	require.Contains(t, f.diags.String(), "notice: The Pulumi CLI collects anonymous usage telemetry")
	cfg := f.readConfig(t)
	require.Equal(t, configVersion, cfg.Version)
	require.True(t, cfg.Notified)
	require.NotEmpty(t, cfg.Installation)

	f.diags.Reset()
	ctx = Init(t.Context(), f.args)

	tel := fromContext(ctx)
	require.NotNil(t, tel)
	require.True(t, tel.send)
	require.Equal(t, cfg.Installation, tel.installation)
	require.Empty(t, f.diags.String())
	Shutdown(ctx)
}

func TestExplicitOptInSkipsNotice(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})

	ctx := Init(t.Context(), f.args)

	require.NotNil(t, fromContext(ctx))
	require.Empty(t, f.diags.String())
	require.False(t, f.readConfig(t).Notified)
	Shutdown(ctx)
}

func TestConfigSetting(t *testing.T) {
	t.Parallel()

	const installation = "0b3b5a4e-6c1f-4c55-9d0a-2a7f6f0d8c11"
	cases := map[string]struct {
		config  string
		send    bool
		log     bool
		warning string
	}{
		"disabled": {
			config: `{"version": 1, "installation": "` + installation + `", "telemetry": "disabled", "notified": true}`,
		},
		"disabled with log": {
			config: `{"version": 1, "installation": "` + installation + `", "telemetry": "disabled", "log": true}`,
			log:    true,
		},
		"enabled": {
			config: `{"version": 1, "installation": "` + installation + `", "telemetry": "enabled"}`,
			send:   true,
		},
		"missing version": {
			config:  `{"installation": "` + installation + `"}`,
			warning: "unsupported version 0",
		},
		"newer version": {
			config:  `{"version": 2, "installation": "` + installation + `"}`,
			warning: "unsupported version 2",
		},
		"bad installation": {
			config:  `{"version": 1, "installation": "my-laptop"}`,
			warning: "invalid installation ID",
		},
		"bad setting": {
			config:  `{"version": 1, "installation": "` + installation + `", "telemetry": "log"}`,
			warning: `unrecognized telemetry setting "log"`,
		},
		"not json": {
			config:  `version: 1`,
			warning: "could not parse",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, nil)
			f.writeConfig(t, c.config)

			ctx := Init(t.Context(), f.args)
			defer Shutdown(ctx)

			tel := fromContext(ctx)
			if !c.send && !c.log {
				require.Nil(t, tel)
			} else {
				require.NotNil(t, tel)
				require.Equal(t, c.send, tel.send)
				require.Equal(t, c.log, tel.logPath != "")
				require.Equal(t, installation, tel.installation)
			}
			if c.warning != "" {
				require.Contains(t, f.diags.String(), c.warning)
			} else {
				require.Empty(t, f.diags.String())
			}
		})
	}
}

func (f *fixture) readLog(t *testing.T) []logEntry {
	data, err := os.ReadFile(filepath.Join(f.walDir(), "log", "events.jsonl"))
	require.NoError(t, err)
	var entries []logEntry
	for line := range bytes.Lines(data) {
		var entry logEntry
		require.NoError(t, json.Unmarshal(line, &entry))
		entries = append(entries, entry)
	}
	return entries
}

func TestLogging(t *testing.T) {
	t.Parallel()

	t.Run("sending", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1", "PULUMI_TELEMETRY_LOG": "1"})
		f.args.CI = true

		ctx := Init(t.Context(), f.args)
		Emit(ctx, boolFact("ci", true))
		Shutdown(ctx)

		entries := f.readLog(t)
		require.Len(t, entries, 1)
		require.True(t, entries[0].Sending)
		require.Contains(t, string(entries[0].Event), `"facts":{"ci":true}`)
		require.Len(t, f.service.received(), 1)
	})

	t.Run("not sending", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "0", "PULUMI_TELEMETRY_LOG": "1"})
		f.args.CI = true

		ctx := Init(t.Context(), f.args)
		Emit(ctx, boolFact("ci", true), enumFact("backend", "pulumi"))
		Shutdown(ctx)

		entries := f.readLog(t)
		require.Len(t, entries, 1)
		require.False(t, entries[0].Sending)
		require.Contains(t, string(entries[0].Event), `"facts":{"backend":"pulumi","ci":true}`)
		require.Empty(t, Headers(ctx))
		require.Empty(t, f.service.received())
		require.NoFileExists(t, filepath.Join(f.walDir(), fromContext(ctx).invocation+walExtension))
	})

	t.Run("discarded", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1", "PULUMI_TELEMETRY_LOG": "1"})

		ctx := Init(t.Context(), f.args)
		Emit(ctx, boolFact("ci", false))
		UseBackend(ctx, "https://app.pulumi.example.com")
		Shutdown(ctx)

		entries := f.readLog(t)
		require.Len(t, entries, 2)
		require.Equal(t, fromContext(ctx).invocation, entries[1].Discarded)
	})
}

func TestSendOnShutdownInCI(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
	f.args.CI = true

	ctx := Init(t.Context(), f.args)
	tel := fromContext(ctx)
	Emit(ctx, enumFact("command", "pulumi:stack:new"), numberFact("exit-code", 0), setFact("flags", "yes", "stack"))
	Emit(ctx, boolFact("ci", true))
	Emit(ctx)
	Shutdown(ctx)

	received := f.service.received()
	require.Len(t, received, 1)
	require.Equal(t, f.args.UserAgent, received[0].UserAgent)
	require.Len(t, received[0].Events, 2)
	for _, e := range received[0].Events {
		require.Equal(t, tel.installation, e.Installation)
		require.Equal(t, tel.invocation, e.Invocation)
		require.WithinDuration(t, time.Now(), e.Timestamp, time.Minute)
	}
	require.Equal(t, map[string]any{
		"command":   "pulumi:stack:new",
		"exit-code": float64(0),
		"flags":     []any{"yes", "stack"},
	}, received[0].Events[0].Facts)
	require.Equal(t, map[string]any{"ci": true}, received[0].Events[1].Facts)
	require.NoDirExists(t, f.walDir())
}

func TestHeaders(t *testing.T) {
	t.Parallel()

	require.Empty(t, Headers(t.Context()))

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
	ctx := Init(t.Context(), f.args)
	defer Shutdown(ctx)
	tel := fromContext(ctx)

	headers := Headers(ctx)
	require.Equal(t, tel.installation, headers.Get("X-Pulumi-Installation-Id"))
	require.Equal(t, tel.invocation, headers.Get("X-Pulumi-Invocation-Id"))
}

func TestPulseSendsAndClearsLog(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
	f.args.Pulse = 10 * time.Millisecond

	ctx := Init(t.Context(), f.args)
	tel := fromContext(ctx)
	wal := filepath.Join(f.walDir(), tel.invocation+walExtension)
	Emit(ctx, boolFact("ci", false))
	UseBackend(ctx, "https://app.pulumi.com")

	require.Eventually(t, func() bool { return len(f.service.received()) == 1 }, 10*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		info, err := os.Stat(wal)
		return err == nil && info.Size() == 0
	}, 10*time.Second, 10*time.Millisecond)

	Shutdown(ctx)
	require.NoFileExists(t, wal)
}

func TestUnsentEventsAreSentByAnotherProcess(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})

	ctx := Init(t.Context(), f.args)
	tel := fromContext(ctx)
	Emit(ctx, boolFact("ci", false))
	Emit(ctx, boolFact("ci", true))
	Shutdown(ctx)

	require.Empty(t, f.service.received())
	wal := filepath.Join(f.walDir(), tel.invocation+walExtension)
	require.FileExists(t, wal)

	other := newQueue(f.walDir(), "other", f.args.URL, f.args.UserAgent, false, time.Hour)
	other.sendOrphans(t.Context())

	received := f.service.received()
	require.Len(t, received, 1)
	require.Len(t, received[0].Events, 2)
	require.Equal(t, tel.invocation, received[0].Events[0].Invocation)
	require.NoFileExists(t, wal)
}

func TestLiveLogIsNotClaimed(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})

	ctx := Init(t.Context(), f.args)
	defer Shutdown(ctx)
	tel := fromContext(ctx)
	Emit(ctx, boolFact("ci", false))
	UseBackend(ctx, "file:///tmp/state")

	other := newQueue(f.walDir(), "other", f.args.URL, f.args.UserAgent, false, time.Hour)
	other.sendOrphans(t.Context())

	require.Empty(t, f.service.received())
	info, err := os.Stat(filepath.Join(f.walDir(), tel.invocation+walExtension))
	require.NoError(t, err)
	require.NotZero(t, info.Size())
}

func TestEligible(t *testing.T) {
	t.Parallel()

	for backendURL, eligible := range map[string]bool{
		"":                           true,
		"file:///home/me/.pulumi":    true,
		"s3://my-bucket":             true,
		"https://api.pulumi.com":     true,
		"https://app.pulumi.com":     true,
		"https://API.pulumi.com/":    true,
		"https://api.eu.pulumi.com":  false,
		"https://app.eu.pulumi.com":  false,
		"https://pulumi.example.com": false,
		"pulumi.example.com":         false,
		"mem://state":                false,
		"http://localhost:8080":      false,
	} {
		require.Equal(t, eligible, Eligible(backendURL), backendURL)
	}
}

func TestEventsHeldUntilBackend(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
	f.args.Pulse = 10 * time.Millisecond

	ctx := Init(t.Context(), f.args)
	defer Shutdown(ctx)
	tel := fromContext(ctx)
	wal := filepath.Join(f.walDir(), tel.invocation+walExtension)
	Emit(ctx, boolFact("ci", false))

	time.Sleep(100 * time.Millisecond)
	require.Empty(t, f.service.received())
	info, err := os.Stat(wal)
	require.NoError(t, err)
	require.Zero(t, info.Size())

	UseBackend(ctx, "https://app.pulumi.com")

	require.Eventually(t, func() bool { return len(f.service.received()) == 1 }, 10*time.Second, 10*time.Millisecond)
	events := f.service.received()[0].Events
	require.Len(t, events, 2)
	require.Equal(t, map[string]any{"backend": "pulumi"}, events[1].Facts)
}

func TestIneligibleBackendDiscards(t *testing.T) {
	t.Parallel()

	for name, ci := range map[string]bool{"local": false, "CI": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
			f.args.CI = ci

			ctx := Init(t.Context(), f.args)
			Emit(ctx, boolFact("ci", ci))
			UseBackend(ctx, "https://app.pulumi.example.com")
			Emit(ctx, numberFact("exit-code", 0))
			UseBackend(ctx, "https://app.pulumi.com")
			Shutdown(ctx)

			require.Empty(t, f.service.received())
			require.Empty(t, Headers(ctx))
			entries, err := os.ReadDir(f.walDir())
			if err == nil {
				require.Empty(t, entries)
			}
		})
	}
}

func TestOrphanSkipsCorruptLines(t *testing.T) {
	t.Parallel()

	f := newFixture(t, nil)
	require.NoError(t, os.MkdirAll(f.walDir(), 0o700))
	wal := filepath.Join(f.walDir(), "crashed"+walExtension)
	require.NoError(t, os.WriteFile(wal, []byte(`{"facts":{"ci":true}}`+"\n"+`{"facts":{"ci":`), 0o600))

	q := newQueue(f.walDir(), "other", f.args.URL, f.args.UserAgent, false, time.Hour)
	q.sendOrphans(t.Context())

	received := f.service.received()
	require.Len(t, received, 1)
	require.Len(t, received[0].Events, 1)
	require.NoFileExists(t, wal)
}

func TestOrphanEmptyLogs(t *testing.T) {
	t.Parallel()

	f := newFixture(t, nil)
	require.NoError(t, os.MkdirAll(f.walDir(), 0o700))
	fresh := filepath.Join(f.walDir(), "fresh"+walExtension)
	stale := filepath.Join(f.walDir(), "stale"+walExtension)
	require.NoError(t, os.WriteFile(fresh, nil, 0o600))
	require.NoError(t, os.WriteFile(stale, nil, 0o600))
	old := time.Now().Add(-2 * staleEmptyAge)
	require.NoError(t, os.Chtimes(stale, old, old))

	q := newQueue(f.walDir(), "other", f.args.URL, f.args.UserAgent, false, time.Hour)
	q.sendOrphans(t.Context())

	require.Empty(t, f.service.received())
	require.FileExists(t, fresh)
	require.NoFileExists(t, stale)
}

func TestOrphanKeptWhenServiceUnavailable(t *testing.T) {
	t.Parallel()

	f := newFixture(t, nil)
	f.service.status = http.StatusServiceUnavailable
	require.NoError(t, os.MkdirAll(f.walDir(), 0o700))
	wal := filepath.Join(f.walDir(), "crashed"+walExtension)
	require.NoError(t, os.WriteFile(wal, []byte(`{"facts":{"ci":true}}`+"\n"), 0o600))

	q := newQueue(f.walDir(), "other", f.args.URL, f.args.UserAgent, false, time.Hour)
	q.sendOrphans(t.Context())

	require.Len(t, f.service.received(), 1)
	data, err := os.ReadFile(wal)
	require.NoError(t, err)
	require.Equal(t, `{"facts":{"ci":true}}`+"\n", string(data))
}

func TestFlushOutcome(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		status  int
		err     bool
		pending int
	}{
		"accepted":    {status: http.StatusNoContent},
		"rejected":    {status: http.StatusBadRequest},
		"rate limit":  {status: http.StatusTooManyRequests, err: true, pending: 1},
		"unavailable": {status: http.StatusServiceUnavailable, err: true, pending: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, nil)
			f.service.status = c.status
			q := newQueue(f.walDir(), "self", f.args.URL, f.args.UserAgent, true, time.Hour)
			q.add([]byte(`{"facts":{"ci":true}}`))

			err := q.flush(t.Context())

			if c.err {
				require.ErrorContains(t, err, "telemetry endpoint returned")
			} else {
				require.NoError(t, err)
			}
			require.Len(t, q.pending, c.pending)
		})
	}
}

func TestFlushAfterDiscard(t *testing.T) {
	t.Parallel()

	f := newFixture(t, nil)
	f.service.release = make(chan struct{})
	q := newQueue(f.walDir(), "self", f.args.URL, f.args.UserAgent, false, time.Hour)
	q.add([]byte(`{"facts":{"ci":true}}`))

	flushed := make(chan error, 1)
	go func() { flushed <- q.flush(t.Context()) }()
	require.Eventually(t, func() bool { return len(f.service.received()) == 1 }, 10*time.Second, 10*time.Millisecond)

	q.discard()
	close(f.service.release)

	require.NoError(t, <-flushed)
	require.Empty(t, q.pending)
}

func TestShutdownDoesNotWaitForInFlightSend(t *testing.T) {
	t.Parallel()

	f := newFixture(t, map[string]string{"PULUMI_TELEMETRY": "1"})
	f.args.CI = true
	f.args.Pulse = 10 * time.Millisecond
	f.service.hang = true

	ctx := Init(t.Context(), f.args)
	fromContext(ctx).queue.shutdownTimeout = 100 * time.Millisecond
	Emit(ctx, boolFact("ci", true))
	UseBackend(ctx, "https://app.pulumi.com")
	require.Eventually(t, func() bool { return len(f.service.received()) == 1 }, 10*time.Second, 10*time.Millisecond)

	start := time.Now()
	Shutdown(ctx)

	require.Less(t, time.Since(start), 2*time.Second)
}

func TestOptOutSkipsConfigWarnings(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		vars map[string]string
		warn bool
	}{
		"PULUMI_TELEMETRY=0": {vars: map[string]string{"PULUMI_TELEMETRY": "0"}},
		"DO_NOT_TRACK":       {vars: map[string]string{"DO_NOT_TRACK": "1"}},
		"logging requested":  {vars: map[string]string{"PULUMI_TELEMETRY": "0", "PULUMI_TELEMETRY_LOG": "1"}, warn: true},
		"no opt-out":         {warn: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, c.vars)
			f.writeConfig(t, `{"version": 2}`)

			require.Nil(t, fromContext(Init(t.Context(), f.args)))

			if c.warn {
				require.Contains(t, f.diags.String(), "unsupported version 2")
			} else {
				require.Empty(t, f.diags.String())
			}
		})
	}
}
