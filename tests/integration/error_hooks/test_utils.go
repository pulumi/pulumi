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
