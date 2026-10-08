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

package credentialhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--credential-helper-test" {
		os.Exit(runTestHelper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

func runTestHelper(args []string) int {
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}
	switch args[0] {
	case "echo":
		err = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"version": 1,
			"env": map[string]string{
				"REQUEST": string(request), "ARGUMENT": args[1],
			},
		})
	case "reply":
		_, err = fmt.Fprintln(os.Stdout, args[1])
	case "stderr":
		if _, err = fmt.Fprintln(os.Stderr, "helper diagnostic"); err == nil {
			_, err = fmt.Fprintln(os.Stdout, `{"version":1,"accessToken":"pul-token"}`)
		}
	case "fail":
		fmt.Fprintln(os.Stdout, `{"version":1,"accessToken":"secret-value"}`)
		return 23
	case "wait":
		time.Sleep(time.Minute)
	default:
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func testRunner(t *testing.T, args ...string) Runner {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	return Runner{Path: executable, Args: append([]string{"--credential-helper-test"}, args...)}
}

func TestRunSendsRequestAndLiteralArguments(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	literal := "$(touch " + marker + ") with spaces; ${HOME}"
	runner := testRunner(t, "echo", literal)
	for _, selected := range []string{"", "https://api.example.com"} {
		t.Run(selected, func(t *testing.T) {
			t.Parallel()
			response, err := runner.Run(t.Context(), Request{Reason: Initial, SelectedBackendURL: selected})
			require.NoError(t, err)
			assert.Equal(t, literal, response.Env["ARGUMENT"])
			if selected == "" {
				assert.JSONEq(t, `{"version":1,"reason":"initial"}`, response.Env["REQUEST"])
			} else {
				assert.JSONEq(t,
					`{"version":1,"reason":"initial","selectedBackendUrl":"https://api.example.com"}`,
					response.Env["REQUEST"])
			}
			_, err = os.Stat(marker)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestRunForwardsStderr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	runner := testRunner(t, "stderr")
	runner.Stderr = &stderr
	response, err := runner.Run(t.Context(), Request{Reason: Initial})
	require.NoError(t, err)
	assert.Equal(t, "pul-token", response.AccessToken)
	assert.Equal(t, "helper diagnostic\n", stderr.String())
}

func TestRunDecline(t *testing.T) {
	t.Parallel()
	runner := testRunner(t, "reply", "{}")
	response, err := runner.Run(t.Context(), Request{Reason: Initial})
	require.NoError(t, err)
	assert.Nil(t, response)
}

func TestRunFailure(t *testing.T) {
	t.Parallel()
	response, err := testRunner(t, "fail").Run(t.Context(), Request{Reason: Initial})
	require.Error(t, err)
	assert.Nil(t, response)
	assert.NotContains(t, err.Error(), "secret-value")
}

func TestRunMissingExecutable(t *testing.T) {
	t.Parallel()
	runner := Runner{Path: filepath.Join(t.TempDir(), "missing")}
	_, err := runner.Run(t.Context(), Request{Reason: Initial})
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunRejectsRelativeExecutable(t *testing.T) {
	t.Parallel()
	runner := Runner{Path: "pulumi-credential-helper"}
	_, err := runner.Run(t.Context(), Request{Reason: Initial})
	require.ErrorContains(t, err, "path must be absolute")
}

func TestRunCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := testRunner(t, "wait")
	_, err := runner.Run(ctx, Request{Reason: Initial})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	runner := testRunner(t, "wait")
	_, err := runner.Run(ctx, Request{Reason: Initial})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
