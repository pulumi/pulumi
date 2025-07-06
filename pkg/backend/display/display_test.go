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
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
