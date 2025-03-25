// Copyright 2016-2022, Pulumi Corporation.
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

package tests

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

const remoteTestRepo = "https://github.com/pulumi/test-repo.git"

func TestInvalidRemoteFlags(t *testing.T) {
	t.Parallel()

	commands := []string{"preview", "up", "refresh", "destroy"}

	tests := map[string]struct {
		args []string
		err  string
	}{
		"no url and no inherit-settings": {
			err: "error: the url arg must be specified if not passing --remote-inherit-settings",
		},
		"no branch or commit": {
			args: []string{remoteTestRepo},
			err:  "error: either `--remote-git-branch` or `--remote-git-commit` is required",
		},
		"both branch and commit": {
			args: []string{remoteTestRepo, "--remote-git-branch", "branch", "--remote-git-commit", "commit"},
			err:  "error: `--remote-git-branch` and `--remote-git-commit` cannot both be specified",
		},
		"both ssh private key and path": {
			args: []string{
				remoteTestRepo, "--remote-git-branch", "branch", "--remote-git-auth-ssh-private-key", "key",
				"--remote-git-auth-ssh-private-key-path", "path",
			},
			err: "error: `--remote-git-auth-ssh-private-key` and `--remote-git-auth-ssh-private-key-path` " +
				"cannot both be specified",
		},
		"ssh private key path doesn't exist": {
			args: []string{
				remoteTestRepo, "--remote-git-branch", "branch", "--remote-git-auth-ssh-private-key-path",
				"doesntexist",
			},
			err: "error: reading SSH private key path",
		},
		"invalid env": {
			args: []string{remoteTestRepo, "--remote-git-branch", "branch", "--remote-env", "invalid"},
			err:  `expected value of the form "NAME=value": missing "=" in "invalid"`,
		},
		"empty env name": {
			args: []string{remoteTestRepo, "--remote-git-branch", "branch", "--remote-env", "=value"},
			err:  `error: expected non-empty environment name in "=value"`,
		},
		"invalid secret env": {
			args: []string{remoteTestRepo, "--remote-git-branch", "branch", "--remote-env-secret", "blah"},
			err:  `expected value of the form "NAME=value": missing "=" in "blah"`,
		},
		"empty secret env name": {
			args: []string{remoteTestRepo, "--remote-git-branch", "branch", "--remote-env-secret", "=value"},
			err:  `error: expected non-empty environment name in "=value"`,
		},
	}

	for _, command := range commands {
		command := command
		for name, tc := range tests {
			tc := tc
			t.Run(command+"_"+name, func(t *testing.T) {
				t.Parallel()

				e := ptesting.NewEnvironment(t)
				defer e.DeleteIfNotFailed()

				// Remote flags currently require PULUMI_EXPERIMENTAL.
				e.Env = append(e.Env, "PULUMI_EXPERIMENTAL=true")

				args := []string{command, "--remote"}
				_, err := e.RunCommandExpectError("pulumi", append(args, tc.args...)...)
				assert.NotEmpty(t, tc.err)
				assert.Contains(t, err, tc.err)
			})
		}
	}
}

func TestRemoteLifecycle(t *testing.T) {
	// This test requires the service with access to Pulumi Deployments.
	// Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments,
	// set PULUMI_TEST_OWNER to the organization to use for the fully qualified stack,
	// and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	if os.Getenv("PULUMI_TEST_OWNER") == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}
	if os.Getenv("PULUMI_TEST_DEPLOYMENTS_API") == "" {
		t.Skipf("Skipping: PULUMI_TEST_DEPLOYMENTS_API is not set")
	}
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Remote flags currently require PULUMI_EXPERIMENTAL.
	e.Env = append(e.Env, "PULUMI_EXPERIMENTAL=true")

	randomSuffix := func() string {
		b := make([]byte, 4)
		_, err := rand.Read(b)
		assert.NoError(t, err)
		return hex.EncodeToString(b)
	}

	owner := os.Getenv("PULUMI_TEST_OWNER")
	proj := "go_remote_proj"
	stack := strings.ToLower("p-t-remotelifecycle-" + randomSuffix())
	fullyQualifiedStack := fmt.Sprintf("%s/%s/%s", owner, proj, stack)

	e.RunCommand("pulumi", "stack", "init", "--no-select", "--stack", fullyQualifiedStack)

	args := func(command string) []string {
		return []string{
			command, remoteTestRepo, "--stack", fullyQualifiedStack,
			"--remote", "--remote-git-branch", "refs/heads/master", "--remote-git-repo-dir", "goproj",
		}
	}

	e.RunCommand("pulumi", args("preview")...)
	e.RunCommand("pulumi", args("up")...)
	e.RunCommand("pulumi", args("refresh")...)
	e.RunCommand("pulumi", args("destroy")...)

	e.RunCommand("pulumi", "stack", "rm", "--stack", fullyQualifiedStack, "--yes")
}
