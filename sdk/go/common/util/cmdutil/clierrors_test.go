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

package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestExitCodeFor(t *testing.T) {
	t.Parallel()

	t.Run("nil returns success", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, ExitSuccess, ExitCodeFor(nil))
	})

	t.Run("plain error returns general error code", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, ExitCodeError, ExitCodeFor(errors.New("something broke")))
	})

	t.Run("context.Canceled returns cancelled code", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, ExitCancelled, ExitCodeFor(context.Canceled))
	})

	t.Run("wrapped context.Canceled returns cancelled code", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("operation failed: %w", context.Canceled)
		assert.Equal(t, ExitCancelled, ExitCodeFor(err))
	})

	t.Run("ExitCoder wins over context.Canceled", func(t *testing.T) {
		t.Parallel()
		inner := fmt.Errorf("timed out waiting: %w", context.Canceled)
		err := WrapWithExitCode(ExitTimeout, inner)
		assert.Equal(t, ExitTimeout, ExitCodeFor(err))
	})

	t.Run("exit code survives deep wrapping", func(t *testing.T) {
		t.Parallel()
		base := WrapWithExitCode(ExitAuthenticationError, errors.New("bad token"))
		layer1 := fmt.Errorf("checking credentials: %w", base)
		layer2 := fmt.Errorf("running pre-flight: %w", layer1)
		layer3 := fmt.Errorf("pulumi up: %w", layer2)
		assert.Equal(t, ExitAuthenticationError, ExitCodeFor(layer3))
	})

	t.Run("BailError preserves inner exit code", func(t *testing.T) {
		t.Parallel()
		inner := WrapWithExitCode(ExitStackNotFound, errors.New("no stack named 'dev' found"))
		bail := result.BailError(inner)
		assert.Equal(t, ExitStackNotFound, ExitCodeFor(bail))
	})

	t.Run("BailError with plain error defaults to general error", func(t *testing.T) {
		t.Parallel()
		bail := result.BailError(errors.New("something failed"))
		assert.Equal(t, ExitCodeError, ExitCodeFor(bail))
	})

	t.Run("BailError with wrapped ExitCoder", func(t *testing.T) {
		t.Parallel()
		base := WrapWithExitCode(ExitConfigurationError, errors.New("invalid flag"))
		wrapped := fmt.Errorf("parsing args: %w", base)
		bail := result.BailError(wrapped)
		assert.Equal(t, ExitConfigurationError, ExitCodeFor(bail))
	})
}

func TestCancellationError(t *testing.T) {
	t.Parallel()

	operations := []string{"update", "destroy", "refresh", "import"}
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			err := CancellationError{Operation: op}
			assert.Equal(t, op+" cancelled", err.Error())
			assert.Equal(t, ExitCancelled, ExitCodeFor(err))

			var ec ExitCoder
			assert.ErrorAs(t, err, &ec)
			assert.Equal(t, ExitCancelled, ec.ExitCode())
		})
	}
}

func TestCLIError(t *testing.T) {
	t.Parallel()

	t.Run("WrapWithExitCode nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, WrapWithExitCode(ExitAuthenticationError, nil))
	})

	t.Run("preserves original error message", func(t *testing.T) {
		t.Parallel()
		err := WrapWithExitCode(ExitResourceError, errors.New("resource timed out"))
		assert.Equal(t, "resource timed out", err.Error())
	})

	t.Run("Unwrap exposes inner error for errors.Is", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("root cause")
		err := WrapWithExitCode(ExitConfigurationError, sentinel)
		assert.ErrorIs(t, err, sentinel)
	})

	t.Run("errors.As finds CLIError through wrapping", func(t *testing.T) {
		t.Parallel()
		inner := WrapWithExitCode(ExitPolicyViolation, errors.New("blocked"))
		wrapped := fmt.Errorf("deploy: %w", inner)

		var cliErr *CLIError
		assert.ErrorAs(t, wrapped, &cliErr)
		assert.Equal(t, ExitPolicyViolation, cliErr.ExitCode())
	})

	t.Run("outermost exit code wins when nested", func(t *testing.T) {
		t.Parallel()
		inner := WrapWithExitCode(ExitResourceError, errors.New("cloud API failed"))
		outer := WrapWithExitCode(ExitConfigurationError, inner)
		assert.Equal(t, ExitConfigurationError, ExitCodeFor(outer))
	})
}

type exitCoderImpl struct {
	code int
	msg  string
}

func (e exitCoderImpl) Error() string { return e.msg }
func (e exitCoderImpl) ExitCode() int { return e.code }

func TestExitCoderInterface(t *testing.T) {
	t.Parallel()

	t.Run("direct ExitCoder implementation", func(t *testing.T) {
		t.Parallel()
		err := exitCoderImpl{code: ExitAuthenticationError, msg: "login required"}
		assert.Equal(t, ExitAuthenticationError, ExitCodeFor(err))
	})

	t.Run("wrapped ExitCoder found through error chain", func(t *testing.T) {
		t.Parallel()
		inner := exitCoderImpl{code: ExitResourceError, msg: "resource failed"}
		wrapped := fmt.Errorf("deploy failed: %w", inner)
		assert.Equal(t, ExitResourceError, ExitCodeFor(wrapped))
	})
}
