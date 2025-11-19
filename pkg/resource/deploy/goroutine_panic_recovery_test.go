// Copyright 2016-2024, Pulumi Corporation.
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
	"strings"
	"testing"
	"time"
)

func TestPanicRecovery(t *testing.T) {
	t.Parallel()
	t.Run("panic is caught and sent to error channel", func(t *testing.T) {
		t.Parallel()
		panicErrs := make(chan error, 1)
		done := make(chan bool)

		go PanicRecovery(panicErrs, func() {
			defer close(done)
			panic("test panic")
		})

		// Wait for the goroutine to complete
		<-done

		// Check if we received the panic error
		select {
		case err := <-panicErrs:
			if err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !strings.Contains(err.Error(), "panic in goroutine: test panic") {
				t.Errorf("Expected panic message to contain 'test panic', got: %v", err)
			}
			if !strings.Contains(err.Error(), "Stack trace:") {
				t.Errorf("Expected error to contain stack trace, got: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for panic error")
		}
	})

	t.Run("no panic results in no error", func(t *testing.T) {
		t.Parallel()
		panicErrs := make(chan error, 1)
		done := make(chan bool)

		go PanicRecovery(panicErrs, func() {
			defer close(done)
			// Normal execution, no panic
		})

		// Wait for the goroutine to complete
		<-done

		// Check that no error was sent
		select {
		case err := <-panicErrs:
			t.Fatalf("Expected no error but got: %v", err)
		case <-time.After(100 * time.Millisecond):
			// Expected: no error was sent
		}
	})

	t.Run("nil channel does not cause panic recovery to fail", func(t *testing.T) {
		t.Parallel()
		done := make(chan bool)

		// This should not panic even with nil channel
		go PanicRecovery(nil, func() {
			defer close(done)
			panic("test panic with nil channel")
		})

		// Wait for the goroutine to complete
		select {
		case <-done:
			// Success: goroutine completed without crashing
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for goroutine to complete")
		}
	})
}
