//go:build nodejs || go || python || all

package ints

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

func requirePrinted(
	t *testing.T,
	stack integration.RuntimeValidationStackInfo,
	severity string,
	text string,
) {
	found := false
	for _, event := range stack.Events {
		if event.DiagnosticEvent != nil &&
			event.DiagnosticEvent.Severity == severity && strings.Contains(event.DiagnosticEvent.Message, text) {
			found = true
			break
		}
	}
	b, err := json.Marshal(stack.Events)
	require.NoError(t, err)
	require.True(t, found, "Expected to find printed message: %s, got %s", text, b)
}
