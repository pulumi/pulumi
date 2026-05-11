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

package neo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUIEventSealedInterface locks the sealed-variant contract: each concrete
// UIEvent type must implement the unexported uiEvent() marker. Calling the
// marker via the interface proves the implementation exists. Adding a new
// variant requires adding an entry here (otherwise it silently fails to be
// accepted as a UIEvent at call sites like sendUI).
func TestUIEventSealedInterface(t *testing.T) {
	t.Parallel()

	events := []UIEvent{
		UIAssistantMessage{Content: "hi", IsFinal: true},
		UIToolStarted{Name: "filesystem__read", Args: json.RawMessage(`{}`)},
		UIToolProgress{Name: "filesystem__read", Message: "reading"},
		UIToolCompleted{Name: "filesystem__read", Args: json.RawMessage(`{}`), IsError: false},
		UIError{Message: "boom"},
		UIWarning{Message: "careful"},
		UICancelled{},
		UITaskIdle{},
		UISessionURL{URL: "https://example"},
		UIUserMessage{Content: "hello"},
		UIApprovalRequest{ApprovalID: "appr_1"},
		UIAwaitingApprovals{},
		UIContextCompression{},
		UIPulumiStart{ToolName: "pulumi__pulumi_preview"},
		UIPulumiResource{ToolName: "pulumi__pulumi_preview"},
		UIPulumiDiag{ToolName: "pulumi__pulumi_preview"},
		UIPulumiEnd{ToolName: "pulumi__pulumi_preview"},
	}

	// Calling uiEvent() through the interface dispatches to each concrete impl —
	// that's what lifts tui_events.go from 0% to full coverage of the markers.
	for _, e := range events {
		e.uiEvent()
	}
	require.Len(t, events, 17, "bumped this when adding a new UIEvent variant")
}
