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

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

var awsEmulator, _ = LookupEmulator("aws")

// fakeFloci serves the AWS emulator's health endpoint, reporting healthy once started.
func fakeFloci(t *testing.T, healthy bool) (*httptest.Server, *atomic.Bool) {
	var up atomic.Bool
	up.Store(healthy)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != awsEmulator.HealthPath || !up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &up
}

type fakeTools struct {
	installed map[string]bool
	onRun     func(name string, args []string) error

	mu   sync.Mutex
	runs []string
}

func (f *fakeTools) lookPath(file string) (string, error) {
	if f.installed[file] {
		return "/usr/bin/" + file, nil
	}
	return "", exec.ErrNotFound
}

func (f *fakeTools) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.runs = append(f.runs, name+" "+strings.Join(args, " "))
	f.mu.Unlock()
	if f.onRun != nil {
		return nil, f.onRun(name, args)
	}
	return nil, nil
}

func port(t *testing.T, endpoint string) string {
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	return u.Port()
}

func newTestFloci(t *testing.T, endpoint string, tools *fakeTools) (*Floci, *bytes.Buffer) {
	return newTestEmulator(t, awsEmulator, endpoint, tools)
}

func newTestEmulator(t *testing.T, emulator Emulator, endpoint string, tools *fakeTools) (*Floci, *bytes.Buffer) {
	var progress bytes.Buffer
	return &Floci{
		Emulator:      emulator,
		Endpoint:      endpoint,
		DataDir:       t.TempDir(),
		Progress:      &progress,
		lookPath:      tools.lookPath,
		run:           tools.run,
		client:        &http.Client{Timeout: time.Second},
		pollInterval:  10 * time.Millisecond,
		pullTimeout:   5 * time.Second,
		launchTimeout: 5 * time.Second,
		startTimeout:  5 * time.Second,
		now:           time.Now,
	}, &progress
}

func TestEnsureRunningAlreadyHealthy(t *testing.T) {
	t.Parallel()

	srv, _ := fakeFloci(t, true)
	tools := &fakeTools{installed: map[string]bool{"floci": true, "docker": true}}
	f, _ := newTestFloci(t, srv.URL, tools)

	require.NoError(t, f.EnsureRunning(t.Context(), true))
	assert.Empty(t, tools.runs, "nothing is started when floci is already running")
}

func TestEnsureRunningStartsWithFlociCLI(t *testing.T) {
	t.Parallel()

	srv, up := fakeFloci(t, false)
	tools := &fakeTools{installed: map[string]bool{"floci": true, "docker": true}}
	tools.onRun = func(name string, _ []string) error {
		up.Store(name == "floci")
		return nil
	}
	f, progress := newTestFloci(t, srv.URL, tools)

	require.NoError(t, f.EnsureRunning(t.Context(), true))
	require.Equal(t, []string{
		"docker image inspect --format {{.Id}} floci/floci:2.1.0",
		"floci aws start --detach --image floci/floci:2.1.0 --pull missing --port " + port(t, srv.URL) +
			" --persist " + f.DataDir,
	}, tools.runs, "the image is present, so it isn't pulled, and the floci CLI starts the pinned release")
	assert.Contains(t, progress.String(), "Starting floci")
	assert.NotContains(t, progress.String(), "Pulling")
}

func TestEnsureRunningStartsWithDocker(t *testing.T) {
	t.Parallel()

	srv, up := fakeFloci(t, false)
	tools := &fakeTools{installed: map[string]bool{"docker": true}}
	tools.onRun = func(name string, args []string) error {
		switch args[0] {
		case "start":
			return errors.New("no such container")
		case "run":
			up.Store(true)
		}
		return nil
	}
	f, _ := newTestFloci(t, srv.URL, tools)

	require.NoError(t, f.EnsureRunning(t.Context(), true))
	require.Len(t, tools.runs, 4)
	assert.True(t, strings.HasPrefix(tools.runs[0], "docker info"))
	assert.True(t, strings.HasPrefix(tools.runs[1], "docker image inspect"))
	assert.Equal(t, "docker start floci", tools.runs[2])
	assert.Contains(t, tools.runs[3], "docker run --detach --name floci")
	assert.Contains(t, tools.runs[3], awsEmulator.Image)
}

func TestEnsureRunningWithoutTools(t *testing.T) {
	t.Parallel()

	srv, _ := fakeFloci(t, false)
	tools := &fakeTools{}
	f, _ := newTestFloci(t, srv.URL, tools)

	err := f.EnsureRunning(t.Context(), true)
	require.ErrorIs(t, err, ErrNotRunning)
	assert.Contains(t, err.Error(), "floci aws start")
	assert.Contains(t, err.Error(), "docker run")
	assert.Empty(t, tools.runs)
}

func TestEnsureRunningWithoutAutostart(t *testing.T) {
	t.Parallel()

	srv, _ := fakeFloci(t, false)
	tools := &fakeTools{installed: map[string]bool{"floci": true}}
	f, _ := newTestFloci(t, srv.URL, tools)

	err := f.EnsureRunning(t.Context(), false)
	require.ErrorIs(t, err, ErrNotRunning)
	assert.Contains(t, err.Error(), "floci aws start")
	assert.Empty(t, tools.runs)
}

func TestEnsureRunningRemoteEndpoint(t *testing.T) {
	t.Parallel()

	tools := &fakeTools{installed: map[string]bool{"floci": true, "docker": true}}
	f, _ := newTestFloci(t, "http://floci.example.invalid:4566", tools)
	f.client.Timeout = 100 * time.Millisecond

	err := f.EnsureRunning(t.Context(), true)
	require.ErrorIs(t, err, ErrNotRunning)
	assert.Contains(t, err.Error(), "FLOCI_ENDPOINT")
	assert.Contains(t, err.Error(), "floci (AWS) is not running")
	assert.Empty(t, tools.runs, "floci is never started for an endpoint on another machine")
}

func mockStack(tags map[apitype.StackTagName]string) backend.Stack {
	return &backend.MockStack{
		TagsF: func() map[apitype.StackTagName]string { return tags },
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev"), StringV: "dev"}
		},
	}
}

func TestGCPEmulator(t *testing.T) {
	t.Parallel()

	gcp, ok := LookupEmulator("gcp")
	require.True(t, ok)

	for _, installed := range []string{"floci", "docker"} {
		t.Run(installed, func(t *testing.T) {
			t.Parallel()

			var up atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/_floci-gcp/health" && up.Load() {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			t.Cleanup(srv.Close)

			tools := &fakeTools{installed: map[string]bool{installed: true}}
			tools.onRun = func(name string, args []string) error {
				switch {
				case name == "floci" && args[1] == "start", name == "docker" && args[0] == "run":
					up.Store(true)
				case name == "docker" && args[0] == "start":
					return errors.New("no such container")
				}
				return nil
			}
			f, _ := newTestEmulator(t, gcp, srv.URL, tools)

			require.NoError(t, f.EnsureRunning(t.Context(), true))
			last := tools.runs[len(tools.runs)-1]
			if installed == "floci" {
				assert.True(t, strings.HasPrefix(last, "floci gcp start --detach --image floci/floci-gcp:"), last)
			} else {
				assert.Contains(t, last, "docker run --detach --name floci-gcp")
				assert.Contains(t, last, ":4588")
				assert.Contains(t, last, "FLOCI_GCP_STORAGE_MODE=persistent")
				assert.Contains(t, last, "floci/floci-gcp:")
			}
		})
	}
}

// ApplySandbox doesn't start anything itself: emulators start the first time a provider needs them.
func TestApplySandbox(t *testing.T) {
	srv, _ := fakeFloci(t, true)
	t.Setenv("FLOCI_ENDPOINT", srv.URL)
	t.Setenv("FLOCI_GCP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("PULUMI_SANDBOX_NO_AUTOSTART", "true")

	var opts engine.UpdateOptions
	var out bytes.Buffer
	require.NoError(t, ApplySandbox(t.Context(), &out, mockStack(nil), &opts))
	assert.Nil(t, opts.Sandbox, "non-sandbox stacks are untouched")
	assert.Empty(t, out.String())

	stack := mockStack(map[apitype.StackTagName]string{backend.SandboxStackTag: "true"})
	require.NoError(t, ApplySandbox(t.Context(), &out, stack, &opts))
	require.NotNil(t, opts.Sandbox)
	assert.Contains(t, out.String(), "is a sandbox stack")

	p, err := opts.Sandbox.Package(t.Context(), "aws", nil)
	require.NoError(t, err)
	endpoint, _ := p.Env().Raw("AWS_ENDPOINT_URL")
	assert.Equal(t, srv.URL, endpoint)

	_, err = opts.Sandbox.Package(t.Context(), "gcp", nil)
	require.ErrorIs(t, err, ErrNotRunning)
}

func TestEnsureRunningPullsMissingImage(t *testing.T) {
	t.Parallel()

	srv, up := fakeFloci(t, false)
	tools := &fakeTools{installed: map[string]bool{"floci": true, "docker": true}}
	tools.onRun = func(name string, args []string) error {
		switch {
		case name == "docker" && args[0] == "image":
			return errors.New("No such image")
		case name == "floci":
			up.Store(true)
		}
		return nil
	}
	f, progress := newTestFloci(t, srv.URL, tools)

	require.NoError(t, f.EnsureRunning(t.Context(), true))
	require.Len(t, tools.runs, 3)
	assert.Equal(t, "docker pull floci/floci:2.1.0", tools.runs[1])
	assert.Contains(t, progress.String(), "Pulling floci/floci:2.1.0 (first run")
	assert.Contains(t, progress.String(), "Pulled floci/floci:2.1.0 in")
}

func TestEnsureRunningLaunchTimeout(t *testing.T) {
	t.Parallel()

	srv, _ := fakeFloci(t, false)
	tools := &fakeTools{installed: map[string]bool{"floci": true}}
	f, _ := newTestFloci(t, srv.URL, tools)
	f.launchTimeout = 10 * time.Millisecond
	f.run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	err := f.EnsureRunning(t.Context(), true)
	require.ErrorContains(t, err, "starting floci (AWS) timed out")
	assert.Contains(t, err.Error(), "pulumi sandbox status")
}

func TestStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"version":"2.0.1","services":{"s3":"running","ec2":"available","eks":"disabled"}}`))
		assert.Nil(t, err)
	}))
	t.Cleanup(srv.Close)
	f, _ := newTestFloci(t, srv.URL, &fakeTools{})

	status, err := f.Status(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "2.0.1", status.Version)
	assert.Equal(t, map[string]bool{"s3": true, "ec2": true, "eks": false}, status.Services)

	assert.True(t, f.OlderThanPinned(status.Version), "2.0.1 is older than the pinned 2.1.0")
	assert.False(t, f.OlderThanPinned("2.1.0"))
	assert.False(t, f.OlderThanPinned("2.2.0"))
	assert.False(t, f.OlderThanPinned("nightly-09302026"), "unparseable versions are never older")
}

func TestDescribeEmulator(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		version string
		warns   bool
	}{{"2.0.0", true}, {"2.1.0", false}, {"nightly", false}} {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := w.Write([]byte(`{"version":"` + tc.version + `","services":{"s3":"running"}}`))
				assert.Nil(t, err)
			}))
			t.Cleanup(srv.Close)
			f, _ := newTestFloci(t, srv.URL, &fakeTools{})

			var stdout, stderr bytes.Buffer
			sink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never})
			described := describeEmulator(t.Context(), f, sink)

			assert.Equal(t, "floci "+tc.version, described.DisplayName)
			assert.Equal(t, map[string]bool{"s3": true}, described.Services)
			assert.Contains(t, stdout.String(), "Using floci "+tc.version+" at "+srv.URL)
			if tc.warns {
				assert.Contains(t, stderr.String(), "sandbox stacks are tested with 2.1.0")
				assert.Contains(t, stderr.String(), "docker rm floci")
			} else {
				assert.Empty(t, stderr.String())
			}
		})
	}
}

func TestSandboxCmdName(t *testing.T) {
	t.Parallel()

	cmd := NewSandboxCmd()
	assert.Equal(t, "sandbox", cmd.Name())
	names := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.ElementsMatch(t, []string{"env", "status", "stop"}, names)
}
