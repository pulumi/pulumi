// Copyright 2024, Pulumi Corporation.
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

func TestExitCodeForNil(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ExitSuccess, ExitCodeFor(nil))
}

func TestExitCodeForGenericError(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ExitCodeError, ExitCodeFor(errors.New("something went wrong")))
}

func TestExitCodeForCLIError(t *testing.T) {
	t.Parallel()
	err := WrapWithExitCode(ExitAuthenticationError, errors.New("bad credentials"))
	assert.Equal(t, ExitAuthenticationError, ExitCodeFor(err))
}

func TestExitCodeForWrappedCLIError(t *testing.T) {
	t.Parallel()
	inner := WrapWithExitCode(ExitStackNotFound, errors.New("no stack"))
	wrapped := fmt.Errorf("operation failed: %w", inner)
	assert.Equal(t, ExitStackNotFound, ExitCodeFor(wrapped))
}

func TestExitCodeForContextCanceled(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ExitCancelled, ExitCodeFor(context.Canceled))
}

func TestExitCodeForWrappedContextCanceled(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("operation failed: %w", context.Canceled)
	assert.Equal(t, ExitCancelled, ExitCodeFor(err))
}

func TestExitCodeForCancellationError(t *testing.T) {
	t.Parallel()
	err := CancellationError{Operation: "update"}
	assert.Equal(t, ExitCancelled, ExitCodeFor(err))
	assert.Equal(t, "update cancelled", err.Error())
}

func TestWrapWithExitCodeNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, WrapWithExitCode(ExitAuthenticationError, nil))
}

func TestCLIErrorUnwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("root cause")
	err := WrapWithExitCode(ExitConfigurationError, inner)
	assert.ErrorIs(t, err, inner)
}

func TestExitCodeForBailErrorWithExitCoder(t *testing.T) {
	t.Parallel()
	inner := WrapWithExitCode(ExitStackNotFound, errors.New("no stack"))
	bail := result.BailError(inner)
	assert.Equal(t, ExitStackNotFound, ExitCodeFor(bail))
}

func TestExitCodeForBailErrorWithPlainError(t *testing.T) {
	t.Parallel()
	bail := result.BailError(errors.New("something failed"))
	assert.Equal(t, ExitCodeError, ExitCodeFor(bail))
}

// exitCoderImpl is a test helper that implements ExitCoder directly.
type exitCoderImpl struct {
	code int
	msg  string
}

func (e exitCoderImpl) Error() string { return e.msg }
func (e exitCoderImpl) ExitCode() int { return e.code }

func TestExitCodeForDirectExitCoder(t *testing.T) {
	t.Parallel()
	err := exitCoderImpl{code: ExitPolicyViolation, msg: "policy denied"}
	assert.Equal(t, ExitPolicyViolation, ExitCodeFor(err))
}

func TestExitCodeForWrappedExitCoder(t *testing.T) {
	t.Parallel()
	inner := exitCoderImpl{code: ExitResourceError, msg: "resource failed"}
	wrapped := fmt.Errorf("deploy failed: %w", inner)
	assert.Equal(t, ExitResourceError, ExitCodeFor(wrapped))
}
