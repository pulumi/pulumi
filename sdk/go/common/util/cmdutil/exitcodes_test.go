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

package cmdutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCodeConstants(t *testing.T) {
	t.Parallel()

	// Verify exit codes have expected values
	assert.Equal(t, 0, ExitSuccess)
	assert.Equal(t, 1, ExitCodeError)
	assert.Equal(t, 2, ExitConfigurationError)
	assert.Equal(t, 3, ExitAuthenticationError)
	assert.Equal(t, 4, ExitResourceError)
	assert.Equal(t, 5, ExitPolicyViolation)
	assert.Equal(t, 6, ExitStackNotFound)
	assert.Equal(t, 7, ExitNoChanges)
	assert.Equal(t, 8, ExitCancelled)
	assert.Equal(t, 9, ExitTimeout)
	assert.Equal(t, 255, ExitInternalError)
}

func TestExitCodesMetadata(t *testing.T) {
	t.Parallel()

	// Verify all exit codes have metadata
	expectedCodes := []int{
		ExitSuccess,
		ExitCodeError,
		ExitConfigurationError,
		ExitAuthenticationError,
		ExitResourceError,
		ExitPolicyViolation,
		ExitStackNotFound,
		ExitNoChanges,
		ExitCancelled,
		ExitTimeout,
		ExitInternalError,
	}

	for _, code := range expectedCodes {
		info, ok := ExitCodes[code]
		assert.True(t, ok, "Exit code %d should have metadata", code)
		assert.Equal(t, code, info.Code, "ExitCodeInfo.Code should match the map key")
		assert.NotEmpty(t, info.Name, "Exit code %d should have a name", code)
		assert.NotEmpty(t, info.Description, "Exit code %d should have a description", code)
	}
}

func TestRetryableExitCodes(t *testing.T) {
	t.Parallel()

	// These exit codes should be marked as retryable
	retryable := []int{ExitCodeError, ExitResourceError, ExitTimeout}
	for _, code := range retryable {
		info := ExitCodes[code]
		assert.True(t, info.Retryable, "Exit code %d (%s) should be retryable", code, info.Name)
	}

	// These exit codes should not be retryable
	notRetryable := []int{
		ExitSuccess,
		ExitConfigurationError,
		ExitAuthenticationError,
		ExitPolicyViolation,
		ExitStackNotFound,
		ExitNoChanges,
		ExitCancelled,
		ExitInternalError,
	}
	for _, code := range notRetryable {
		info := ExitCodes[code]
		assert.False(t, info.Retryable, "Exit code %d (%s) should not be retryable", code, info.Name)
	}
}
