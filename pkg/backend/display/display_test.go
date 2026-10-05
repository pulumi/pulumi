// Copyright 2024, Pulumi Corporation.
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

package display

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/auth/authtest"
	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if code, ran := authtest.RunHelper(); ran {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

type helperTestTransport struct{}

func (helperTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

func TestCredentialHelperDiagnosticsDuringRendering(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	helper := authtest.NewHelper(t, map[string]any{
		"initial": authtest.Reply{
			Response: map[string]any{
				"version": 1, "headers": map[string][]string{"X-Gate": {"old"}}, "expiresAt": "2000-01-01T00:00:00Z",
			},
			Stderr: "helper initial diagnostic\n",
		},
		"expired": authtest.Reply{
			Response: map[string]any{"version": 1, "headers": map[string][]string{"X-Gate": {"new"}}},
			Stderr:   "helper expired diagnostic\n",
		},
	})
	session := auth.NewSession(&stderr)
	require.NoError(t, session.UseHelper(workspace.CredentialHelper{Path: helper.Path, Args: helper.Args}))
	ctx := t.Context()
	const backendURL = "https://api.example.com"
	_, err := session.PrepareBackend(ctx, backendURL)
	require.NoError(t, err)
	httpAuth := session.HTTPAuth(backendURL)

	var jsonEvent apitype.EngineEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"stdoutEvent":{"message":"existing system message\n","color":"never"}
	}`), &jsonEvent))
	event, err := ConvertJSONEvent(jsonEvent)
	require.NoError(t, err)
	events, done := make(chan engine.Event), make(chan bool)
	go ShowProgressEvents("test", apitype.UpdateUpdate, tokens.MustParseStackName("stack"), "project", "",
		events, done, Options{
			IsInteractive: true, Color: colors.Never, Stdout: &stdout, Stderr: &stderr,
			HelperDiagnostics: httpAuth.CaptureHelperStderr,
			term:              terminal.NewMockTerminal(&stdout, 100, 40, true), DeterministicOutput: true,
		}, false)
	events <- event
	request, err := http.NewRequestWithContext(auth.WithHelperAuth(ctx), http.MethodGet, backendURL+"/api/user", nil)
	require.NoError(t, err)
	response, err := httpAuth.Transport(helperTestTransport{}).RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	events <- engine.NewCancelEvent()
	<-done
	assert.Equal(t, "helper initial diagnostic\n", stderr.String())
	assert.Contains(t, stdout.String(), "existing system message")
	assert.Contains(t, stdout.String(), "helper expired diagnostic")
}

func TestShowEvents(t *testing.T) {
	t.Parallel()

	// Test that internal events are filtered out.
	events := make(chan engine.Event)
	done := make(chan bool)
	stack, err := tokens.ParseStackName("stack")
	require.NoError(t, err)

	eventLog, err := os.CreateTemp(t.TempDir(), "event-log-")
	require.NoError(t, err)

	go func() {
		events <- engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN: resource.NewURN(stack.Q(), "proj", "parent", "base", "not-filtered"),
				Op:  deploy.OpCreate,
			},
			Internal: false,
		})
		events <- engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN: resource.NewURN(stack.Q(), "proj", "parent", "base", "this-is-filtered-from-display"),
				Op:  deploy.OpCreate,
			},
			Internal: true,
		})
		events <- engine.NewCancelEvent()
		close(events)
	}()

	var stdout bytes.Buffer
	ShowEvents("op", apitype.UpdateUpdate, stack, "proj", "permalink", events, done, Options{
		EventLogPath: eventLog.Name(),
		Stdout:       &stdout,
		Color:        colors.Never,
	}, false)
	<-done

	assert.Contains(t, stdout.String(), "not-filtered")
	assert.NotContains(t, stdout.String(), "this-is-filtered-from-display")

	read, err := os.ReadFile(eventLog.Name())
	require.NoError(t, err)
	assert.Contains(t, string(read), "not-filtered")
	assert.Contains(t, string(read), "this-is-filtered-from-display")
}

func TestStartEventLogger_ClosesOutputOnInputClose(t *testing.T) {
	t.Parallel()

	// startEventLogger must close its output channel when the input is exhausted,
	// so consumers can rely on standard `for range` semantics rather than having
	// to break on CancelEvent.
	eventLog, err := os.CreateTemp(t.TempDir(), "event-log-")
	require.NoError(t, err)

	events := make(chan engine.StampedEvent, 1)
	events <- engine.StampedEvent{Event: engine.NewCancelEvent()}
	close(events)

	done := make(chan bool)
	outEvents, outDone := startEventLogger(events, done, Options{
		EventLogPath: eventLog.Name(),
	})

	drained := make(chan struct{})
	go func() {
		for range outEvents {
		}
		close(drained)
	}()

	select {
	case <-drained:
	case <-time.After(2 * time.Second):
		t.Fatal("outEvents was not closed after the input was drained")
	}

	close(outDone)
	<-done
}

func TestStartEventLogger_TCPFallsBackOnInvalidTarget(t *testing.T) {
	t.Parallel()

	// If the TCP target is unusable, startEventLogger must fall back to the
	// original (events, done) so the rest of the display pipeline still runs.
	// "tcp://" with an empty address makes grpc.NewClient fail synchronously.
	events := make(chan engine.StampedEvent)
	done := make(chan bool)

	outEvents, outDone := startEventLogger(events, done, Options{
		EventLogPath: "tcp://",
	})

	// The originals are returned unchanged — same channel identity.
	assert.Equal(t, (<-chan engine.StampedEvent)(events), outEvents)
	assert.Equal(t, (chan<- bool)(done), outDone)
}

func TestEscapeURN(t *testing.T) {
	t.Parallel()

	// Most characters can safely be displayed as is, including double quotes.
	require.Equal(t, "\"double quotes\"", escapeURN("\"double quotes\""))
	require.Equal(t, "escaped double quote \\\"", escapeURN("escaped double quote \\\""))
	require.Equal(t, "backslashes\\\\\\\\", escapeURN("backslashes\\\\\\\\"))
	require.Equal(t, "C:\\windows\\paths", escapeURN("C:\\windows\\paths"))
	require.Equal(t, "Emoji 🦄", escapeURN("Emoji 🦄"))
	require.Equal(t, "Non breaking space: <\u00a0>", escapeURN("Non breaking space: <\u00a0>"))

	// Non graphic characters need to be escaped, as they can mess up the display layout.
	require.Equal(t, "newline\\n", escapeURN("newline\n"))
	require.Equal(t, "tab\\t", escapeURN("tab\t"))
	require.Equal(t, "ZWJ: <\\u200d>", escapeURN("ZWJ: <\u200d>"))
}
