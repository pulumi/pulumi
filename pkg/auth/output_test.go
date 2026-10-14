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
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--helper-stderr" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		fmt.Fprintln(os.Stderr, "refreshing credentials")
		fmt.Fprintln(os.Stdout, `{}`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCaptureHelperStderr(t *testing.T) {
	t.Parallel()
	var terminal bytes.Buffer
	session := NewSession(&terminal)
	messages, restore := session.stderr.capture()
	finished := make(chan struct{})
	go func() {
		for range helperMessageBuffer + 3 {
			fmt.Fprint(session.stderr, "diagnostic\n")
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("helper blocked on an unread display channel")
	}
	assert.Empty(t, terminal.String(), "overflow must not write synchronously to the terminal")
	assert.Equal(t, "diagnostic\n", <-messages)
	restore()
	assert.Equal(t, helperMessageBuffer-1, strings.Count(terminal.String(), "diagnostic\n"))
	assert.Contains(t, terminal.String(), "omitted 3 diagnostic messages")
	_, err := fmt.Fprint(session.stderr, "after rendering")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(terminal.String(), "after rendering"))
}

func TestHelperLines(t *testing.T) {
	t.Parallel()
	var terminal bytes.Buffer
	output := &helperOutput{terminal: &terminal}
	messages, restore := output.capture()
	defer restore()
	first, second := &helperLines{output: output}, &helperLines{output: output}
	fmt.Fprint(first, "par")
	fmt.Fprint(second, "another\n")
	fmt.Fprint(first, "tial\nnext")
	first.flush()
	assert.Equal(t, "another\n", <-messages)
	assert.Equal(t, "partial\n", <-messages)
	assert.Equal(t, "next", <-messages)

	fmt.Fprint(first, strings.Repeat("secret", helperLineLimit), "\nend\n")
	assert.Equal(t, "[credential helper: omitted diagnostic line exceeding 4096 bytes]\n", <-messages)
	assert.Equal(t, "end\n", <-messages)
}

func TestCapturedHelperProcess(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	require.NoError(t, err)
	session := NewSession(io.Discard)
	require.NoError(t, session.UseHelper(workspace.CredentialHelper{Path: executable, Args: []string{"--helper-stderr"}}))
	messages, restore := session.stderr.capture()
	defer restore()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := session.PrepareBackend(ctx, testBackendURL)
		finished <- err
	}()
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("helper did not finish while its diagnostics were unread")
	}
	assert.Equal(t, "refreshing credentials\n", <-messages)
}
