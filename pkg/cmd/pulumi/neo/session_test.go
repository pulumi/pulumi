// Copyright 2016-2025, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateApprovalMode tests ---

func TestValidateApprovalMode_Valid(t *testing.T) {
	for _, mode := range ValidApprovalModes {
		err := validateApprovalMode(mode)
		assert.NoError(t, err, "mode %q should be valid", mode)
	}
}

func TestValidateApprovalMode_Invalid(t *testing.T) {
	err := validateApprovalMode("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid approval mode")
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidateApprovalMode_Empty(t *testing.T) {
	err := validateApprovalMode("")
	require.Error(t, err)
}

// --- handleApproval tests ---

func TestHandleApproval_AutoAlwaysApproves(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "high",
	}

	result := handleApproval(d, parsed, "auto")
	assert.True(t, result)
}

func TestHandleApproval_BalancedApprovesLow(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	result := handleApproval(d, parsed, "balanced")
	assert.True(t, result)
}

func TestHandleApproval_BalancedApprovesEmpty(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "",
	}

	result := handleApproval(d, parsed, "balanced")
	assert.True(t, result)
}

func TestHandleApproval_BalancedPromptsHigh(t *testing.T) {
	// Non-interactive display, so PromptApproval will return an error → false.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "high",
	}

	result := handleApproval(d, parsed, "balanced")
	assert.False(t, result) // Non-interactive can't approve
}

func TestHandleApproval_BalancedPromptsDestructive(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "destructive",
	}

	result := handleApproval(d, parsed, "balanced")
	assert.False(t, result)
}

func TestHandleApproval_ManualAlwaysPrompts(t *testing.T) {
	// Non-interactive display → prompt fails → returns false.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	result := handleApproval(d, parsed, "manual")
	assert.False(t, result)
}

func TestHandleApproval_ManualPromptsEvenForLow(t *testing.T) {
	// Verify manual mode doesn't auto-approve low sensitivity.
	// With interactive mode but input "n", should deny.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	// Manual mode always prompts - non-interactive means it returns false.
	result := handleApproval(d, parsed, "manual")
	assert.False(t, result, "manual mode should prompt even for low sensitivity")
}

// --- ValidApprovalModes ---

func TestValidApprovalModes(t *testing.T) {
	assert.Contains(t, ValidApprovalModes, "auto")
	assert.Contains(t, ValidApprovalModes, "balanced")
	assert.Contains(t, ValidApprovalModes, "manual")
	assert.Len(t, ValidApprovalModes, 3)
}
