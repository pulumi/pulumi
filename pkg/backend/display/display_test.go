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
)

func TestShowEvents(t *testing.T) {
	// Test that internal events are filtered out.
	events := make(chan engine.Event)
	done := make(chan bool)
	stack, err := tokens.ParseStackName("stack")
	assert.NoError(t, err)

	eventLog, err := os.CreateTemp("", "event-log-")

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
	assert.NoError(t, err)
	assert.Contains(t, string(read), "not-filtered")
	assert.Contains(t, string(read), "this-is-filtered-from-display")
}
