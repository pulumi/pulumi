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

package auth

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureHelperStderrRestoresTerminal(t *testing.T) {
	t.Parallel()
	var terminal bytes.Buffer
	session := NewSession(&terminal)
	messages, restore := session.stderr.capture()
	finished := make(chan error, 1)
	go func() {
		_, err := fmt.Fprint(session.stderr, "during rendering")
		finished <- err
	}()
	assert.Equal(t, "during rendering", <-messages)
	require.NoError(t, <-finished)
	assert.Empty(t, terminal.String())

	// A helper may still be writing when the renderer stops consuming messages.
	go func() {
		_, err := fmt.Fprint(session.stderr, "after rendering")
		finished <- err
	}()
	restore()
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("helper remained blocked after the renderer stopped")
	}
	assert.Equal(t, "after rendering", terminal.String())
}
