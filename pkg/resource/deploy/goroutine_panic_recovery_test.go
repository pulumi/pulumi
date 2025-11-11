// Copyright 2025, Pulumi Corporation.
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

package deploy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPanicRecovery(t *testing.T) {
	t.Setenv("PULUMI_GOROUTINE_PANIC_RECOVERY", "true")
	t.Run("panic is caught and sent to error channel", func(t *testing.T) {
		t.Parallel()
		panicErrs := make(chan error, 1)
		done := make(chan bool)

		go PanicRecovery(panicErrs, func() {
			defer close(done)
			panic("test panic")
		})

		<-done

		select {
		case err := <-panicErrs:
			require.ErrorContains(t, err, "panic in goroutine: test panic")
			require.ErrorContains(t, err, "Stack trace:")
		case <-time.After(1 * time.Second):
			require.Fail(t, "Timeout waiting for panic error")
		}
	})

	t.Run("no panic results in no error", func(t *testing.T) {
		t.Parallel()
		panicErrs := make(chan error, 1)
		done := make(chan bool)

		go PanicRecovery(panicErrs, func() {
			defer close(done)
		})

		<-done

		select {
		case err := <-panicErrs:
			require.Fail(t, "Expected no error but got one", "error: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("nil channel causes panic to be re-raised", func(t *testing.T) {
		t.Parallel()

		require.Panics(t, func() {
			PanicRecovery(nil, func() {
				panic("test panic with nil channel")
			})
		})
	})
}

func TestPanicRecoveryDisabled(t *testing.T) {
	t.Parallel()
	panicErrs := make(chan error, 1)

	require.Panics(t, func() {
		PanicRecovery(panicErrs, func() {
			panic("test panic without recovery")
		})
	})
}
