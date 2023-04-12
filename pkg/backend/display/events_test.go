package display

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/stretchr/testify/assert"
)

// This test checks that the ANSI control codes are removed from EngineEvents
// converted to be sent to the Pulumi Service API.
func TestRemoveANSI(t *testing.T) {
	t.Parallel()
	input := "\033[31mHello, World!\033[0m"
	expected := "Hello, World!"
	e := engine.NewEvent(
		engine.DiagEvent,
		engine.DiagEventPayload{
			Message: input,
		},
	)

	res, err := ConvertEngineEvent(e, false /* showSecrets */)
	assert.NoError(t, err, "unable to convert engine event")
	assert.Equal(t, expected, res.DiagnosticEvent.Message)
}
