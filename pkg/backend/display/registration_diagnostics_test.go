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

package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceDiagnosticContextFromJSON(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, message string }{
		{"plain", "invalid port\n"},
		{"trailing reset", colors.SpecNote + "invalid port\n\n" + colors.Reset + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var jsonEvent apitype.EngineEvent
			require.NoError(t, json.Unmarshal([]byte(`{
        "diagnosticEvent": {
            "urn": "urn:pulumi:stack::project::example:index:Service::site",
            "message": "invalid port\n", "severity": "error", "color": "raw",
            "sourceContext": {
                "sourcePosition": "project:///main.py#2",
                "stackTrace": [{"sourcePosition": "project:///main.py#2"}],
                "rendered": "  ┌─ main.py:2\n>2│ site = Service(port=0)\n  └─"
            }
        }
    }`), &jsonEvent))
			jsonEvent.DiagnosticEvent.Message = test.message
			event, err := ConvertJSONEvent(jsonEvent)
			require.NoError(t, err)
			payload := event.Payload().(engine.DiagEventPayload)
			assert.Equal(t, test.message, payload.Message)
			apiEvent, err := ConvertEngineEvent(event, false)
			require.NoError(t, err)
			assert.Equal(t, (*apitype.DiagnosticSourceContext)(payload.SourceContext), apiEvent.DiagnosticEvent.SourceContext)
			assert.Equal(t, payload.Message, apiEvent.DiagnosticEvent.Message)

			opts := defaultOpts()
			opts.Color = colors.Never
			diff := renderDiffDiagEvent(payload, opts)
			assert.Contains(t, diff, "invalid port\n\n  ┌─ main.py:2")
			assert.NotContains(t, diff, "invalid port\n\n\n")
			assert.Contains(t, diff, ">2│ site = Service(port=0)")

			var stdout, stderr bytes.Buffer
			opts.Stdout, opts.Stderr = &stdout, &stderr
			events := sliceToBufferedChan([]engine.Event{event, engine.NewCancelEvent()})
			done := make(chan bool)
			go ShowProgressEvents("test", "update", tokens.MustParseStackName("stack"), "project", "", events, done, opts, false)
			<-done
			assert.Equal(t, 1, strings.Count(stdout.String(), "┌─ main.py:2"))
			assert.Contains(t, stdout.String(), ">2│ site = Service(port=0)")
			assert.NotRegexp(t, `invalid port\n[ \t]*\n[ \t]*\n`, stdout.String())
		})
	}
}
