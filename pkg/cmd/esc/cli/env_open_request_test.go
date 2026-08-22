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

package cli

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
)

func testOpenApprovalCommand(t *testing.T) (*envCommand, *testPulumiClient, *bytes.Buffer) {
	t.Helper()

	client := &testPulumiClient{
		user:         "test-user",
		environments: map[string]*testEnvironment{"test-org/test-project/test-env": {}},
		openEnvs:     map[string]*esc.Environment{},
	}
	var stderr bytes.Buffer
	env := &envCommand{
		esc:          &escCommand{client: client, stderr: &stderr},
		pollInterval: time.Millisecond,
	}
	return env, client, &stderr
}

func TestWithOpenApproval(t *testing.T) {
	t.Parallel()

	ref := environmentRef{orgName: "test-org", projectName: "test-project", envName: "test-env"}
	denied := errors.New("environment requires approval to open")

	t.Run("no request without the flag", func(t *testing.T) {
		t.Parallel()

		env, client, _ := testOpenApprovalCommand(t)
		err := env.withOpenApproval(t.Context(), ref, openApprovalOptions{}, func() error { return denied })
		assert.ErrorIs(t, err, denied)
		assert.Empty(t, client.submittedChangeRequests)
	})

	t.Run("retries until approved", func(t *testing.T) {
		t.Parallel()

		env, client, stderr := testOpenApprovalCommand(t)
		attempts := 0
		opts := openApprovalOptions{
			waitForApproval: true,
			reason:          "incident 1234",
			accessDuration:  time.Hour,
			waitTimeout:     10 * time.Millisecond,
		}
		err := env.withOpenApproval(t.Context(), ref, opts, func() error {
			attempts++
			if attempts < 3 {
				return denied
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 3, attempts)
		require.Len(t, client.submittedChangeRequests, 1)
		assert.Equal(t, "incident 1234", *client.submittedChangeRequests[0].description)
		assert.Equal(t, 3600, client.accessDurationSeconds)
		assert.Contains(t, stderr.String(), "for approval")
	})

	t.Run("submits without waiting", func(t *testing.T) {
		t.Parallel()

		env, client, stderr := testOpenApprovalCommand(t)
		attempts := 0
		err := env.withOpenApproval(t.Context(), ref, openApprovalOptions{requestApproval: true}, func() error {
			attempts++
			return denied
		})
		assert.ErrorIs(t, err, denied)
		assert.Equal(t, 1, attempts)
		require.Len(t, client.submittedChangeRequests, 1)
		assert.Equal(t, int(defaultAccessDuration.Seconds()), client.accessDurationSeconds)
		assert.NotContains(t, stderr.String(), "Waiting")
	})

	t.Run("times out", func(t *testing.T) {
		t.Parallel()

		env, _, _ := testOpenApprovalCommand(t)
		opts := openApprovalOptions{waitForApproval: true, waitTimeout: 10 * time.Millisecond}
		err := env.withOpenApproval(t.Context(), ref, opts, func() error { return denied })
		assert.ErrorIs(t, err, denied)
		assert.ErrorContains(t, err, "timed out waiting for the open request to be approved")
	})

	t.Run("reports the original error when no approval rule applies", func(t *testing.T) {
		t.Parallel()

		env, _, _ := testOpenApprovalCommand(t)
		missing := environmentRef{orgName: "test-org", projectName: "test-project", envName: "nope"}
		opts := openApprovalOptions{waitForApproval: true, waitTimeout: 10 * time.Millisecond}
		err := env.withOpenApproval(t.Context(), missing, opts, func() error { return denied })
		assert.ErrorIs(t, err, denied)
	})
}
