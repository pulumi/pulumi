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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCodeFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: ExitSuccess,
		},
		{
			name:     "CLIError returns its exit code",
			err:      NewCLIError(ExitStackNotFound, "stack not found"),
			expected: ExitStackNotFound,
		},
		{
			name:     "wrapped CLIError returns its exit code",
			err:      fmt.Errorf("outer: %w", NewCLIError(ExitAuthenticationError, "auth failed")),
			expected: ExitAuthenticationError,
		},
		{
			name:     "unknown error returns general error code",
			err:      errors.New("some error"),
			expected: ExitCodeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ExitCodeFor(tt.err))
		})
	}
}

func TestCLIError(t *testing.T) {
	t.Parallel()

	t.Run("NewCLIError creates error with message", func(t *testing.T) {
		t.Parallel()
		err := NewCLIError(ExitConfigurationError, "invalid config")
		assert.Equal(t, "invalid config", err.Error())
		assert.Equal(t, ExitConfigurationError, err.ExitCode())
	})

	t.Run("WrapError wraps existing error", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("inner error")
		err := WrapError(ExitResourceError, inner)
		assert.Equal(t, "inner error", err.Error())
		assert.Equal(t, ExitResourceError, err.ExitCode())
		assert.True(t, errors.Is(err, inner))
	})

	t.Run("CLIError with no message uses wrapped error", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("wrapped message")
		err := &CLIError{Err: inner, Code: ExitTimeout}
		assert.Equal(t, "wrapped message", err.Error())
	})

	t.Run("CLIError with no message or wrapped error shows code", func(t *testing.T) {
		t.Parallel()
		err := &CLIError{Code: 42}
		assert.Equal(t, "exit code 42", err.Error())
	})
}

type customExitCoder struct {
	code int
}

func (c customExitCoder) Error() string { return "custom error" }
func (c customExitCoder) ExitCode() int { return c.code }

func TestExitCoderInterface(t *testing.T) {
	t.Parallel()

	custom := customExitCoder{code: ExitPolicyViolation}
	assert.Equal(t, ExitPolicyViolation, ExitCodeFor(custom))

	wrapped := fmt.Errorf("wrapped: %w", custom)
	assert.Equal(t, ExitPolicyViolation, ExitCodeFor(wrapped))
}
