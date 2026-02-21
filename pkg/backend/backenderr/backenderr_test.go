// Copyright 2019-2025, Pulumi Corporation.
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

package backenderr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
)

func TestNotFoundErrorIs(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ErrNotFound, registry.ErrNotFound)
}

func TestStackNotFoundErrorExitCode(t *testing.T) {
	t.Parallel()

	err := StackNotFoundError{StackName: "my-stack"}

	// Verify it returns the correct exit code
	assert.Equal(t, cmdutil.ExitStackNotFound, err.ExitCode())
	assert.Equal(t, 6, err.ExitCode())

	// Verify error message
	assert.Equal(t, "no stack named 'my-stack' found", err.Error())
}

func TestStackNotFoundErrorWithExitCodeFor(t *testing.T) {
	t.Parallel()

	err := StackNotFoundError{StackName: "test-stack"}

	// ExitCodeFor should extract the exit code
	assert.Equal(t, cmdutil.ExitStackNotFound, cmdutil.ExitCodeFor(err))

	// Should work when wrapped
	wrapped := fmt.Errorf("failed to select stack: %w", err)
	assert.Equal(t, cmdutil.ExitStackNotFound, cmdutil.ExitCodeFor(wrapped))
}

func TestCancelledErrorExitCode(t *testing.T) {
	t.Parallel()

	err := CancelledError{Operation: "destroy"}

	// Verify it returns the correct exit code
	assert.Equal(t, cmdutil.ExitCancelled, err.ExitCode())
	assert.Equal(t, 8, err.ExitCode())

	// Verify error message
	assert.Equal(t, "destroy cancelled", err.Error())

	// Should work when wrapped
	wrapped := fmt.Errorf("operation failed: %w", err)
	assert.Equal(t, cmdutil.ExitCancelled, cmdutil.ExitCodeFor(wrapped))
}

func TestNoChangesExpectedErrorExitCode(t *testing.T) {
	t.Parallel()

	err := NoChangesExpectedError{}

	// Verify it returns the correct exit code
	assert.Equal(t, cmdutil.ExitNoChanges, err.ExitCode())
	assert.Equal(t, 7, err.ExitCode())

	// Verify error message
	assert.Equal(t, "no changes were expected but changes occurred", err.Error())

	// Should work when wrapped
	wrapped := fmt.Errorf("operation failed: %w", err)
	assert.Equal(t, cmdutil.ExitNoChanges, cmdutil.ExitCodeFor(wrapped))
}

func TestBackendErrorExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "StackNotFoundError returns ExitStackNotFound",
			err:          StackNotFoundError{StackName: "test"},
			expectedCode: cmdutil.ExitStackNotFound,
		},
		{
			name:         "LoginRequiredError returns ExitAuthenticationError",
			err:          LoginRequiredError{},
			expectedCode: cmdutil.ExitAuthenticationError,
		},
		{
			name:         "ForbiddenError returns ExitAuthenticationError",
			err:          ForbiddenError{Err: errors.New("access denied")},
			expectedCode: cmdutil.ExitAuthenticationError,
		},
		{
			name:         "CancelledError returns ExitCancelled",
			err:          CancelledError{Operation: "update"},
			expectedCode: cmdutil.ExitCancelled,
		},
		{
			name:         "NoChangesExpectedError returns ExitNoChangesExpected",
			err:          NoChangesExpectedError{},
			expectedCode: cmdutil.ExitNoChanges,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Check the error implements ExitCoder
			exitCoder, ok := tt.err.(cmdutil.ExitCoder)
			assert.True(t, ok, "error should implement ExitCoder")
			assert.Equal(t, tt.expectedCode, exitCoder.ExitCode())

			// Check ExitCodeFor returns correct code
			assert.Equal(t, tt.expectedCode, cmdutil.ExitCodeFor(tt.err))
		})
	}
}

// Edge case tests for ExitCodeFor behavior

func TestExitCodeForNilError(t *testing.T) {
	t.Parallel()
	// nil error should return success (0)
	assert.Equal(t, cmdutil.ExitSuccess, cmdutil.ExitCodeFor(nil))
}

func TestExitCodeForPlainError(t *testing.T) {
	t.Parallel()
	// Plain error without ExitCoder should return generic error code (1)
	err := errors.New("some random error")
	assert.Equal(t, cmdutil.ExitCodeError, cmdutil.ExitCodeFor(err))
}

func TestExitCodeForDeeplyWrappedError(t *testing.T) {
	t.Parallel()
	// Error wrapped multiple times should still extract the correct exit code
	inner := StackNotFoundError{StackName: "deeply-nested"}
	wrapped1 := fmt.Errorf("layer 1: %w", inner)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped1)
	wrapped3 := fmt.Errorf("layer 3: %w", wrapped2)

	assert.Equal(t, cmdutil.ExitStackNotFound, cmdutil.ExitCodeFor(wrapped3))
}

func TestCancelledErrorAllOperations(t *testing.T) {
	t.Parallel()
	// Test all operation types that use CancelledError produce correct messages
	operations := []string{"update", "destroy", "refresh", "import", "state edit"}
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			err := CancelledError{Operation: op}
			assert.Equal(t, op+" cancelled", err.Error())
			assert.Equal(t, cmdutil.ExitCancelled, err.ExitCode())
			assert.Equal(t, cmdutil.ExitCancelled, cmdutil.ExitCodeFor(err))
		})
	}
}
