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

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

const (
	managedEnvOrgVar      = "PULUMI_TEST_MANAGED_ENV_ORG"
	managedEnvKeyPrefix   = "pulumi.esc.managedEnvironment"
	managedEnvProject     = "managed-env-it"
	managedEnvProjectYAML = "name: " + managedEnvProject + "\nruntime: yaml\nresources: {}\n"

	managedEnvTemplateYAML = `name: ${PROJECT}
runtime: yaml
template:
  config:
    plain:
      default: one
    token:
      default: hunter2
      secret: true
resources: {}
`
)

func createdManagedEnvironment(t *testing.T, command string, stdout string, stderr string) string {
	t.Helper()
	created := regexp.MustCompile(`Created environment (\S+) for stack configuration`).FindStringSubmatch(stdout + stderr)
	require.Len(t, created, 2, "%s did not report a managed environment:\n%s%s", command, stdout, stderr)
	return created[1]
}

func latestUpdateEnvironment(t *testing.T, e *ptesting.Environment) map[string]string {
	t.Helper()
	out, _ := e.RunCommand("pulumi", "stack", "history", "--json")
	var history []struct {
		Environment map[string]string `json:"environment"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &history))
	require.NotEmpty(t, history)
	return history[0].Environment
}

// TestManagedEnvironment needs a Pulumi Cloud organization that has stack-managed ESC environments enabled, on a
// service that includes the managed-environment draft and publish changes. It is skipped unless
// PULUMI_TEST_MANAGED_ENV_ORG names that organization. PULUMI_API selects the service (default
// https://api.pulumi.com). The organization also needs the Approvals feature: the draft read and edit routes the
// edited-draft steps call keep that gate. It also relies on a CLI that includes the stack-managed environment
// evaluate, send and config-editing changes, and `pulumi new --esc-config`.
//
//nolint:paralleltest // the steps run in order against one stack
func TestManagedEnvironment(t *testing.T) {
	org := os.Getenv(managedEnvOrgVar)
	token := os.Getenv("PULUMI_ACCESS_TOKEN")
	if org == "" || token == "" {
		t.Skipf("Skipping: %s and PULUMI_ACCESS_TOKEN must be set", managedEnvOrgVar)
	}
	apiURL := os.Getenv("PULUMI_API")
	if apiURL == "" {
		apiURL = "https://api.pulumi.com"
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.SetBackend(apiURL)
	e.SetEnvVars("PULUMI_EXPERIMENTAL=true")
	integration.CreatePulumiRepo(e, managedEnvProjectYAML)

	stackName := ptesting.RandomStackName()
	stdout, stderr := e.RunCommand("pulumi", "stack", "init", org+"/"+stackName, "--esc-config")
	managedEnv := createdManagedEnvironment(t, "stack init", stdout, stderr)
	envProject, envName, _ := strings.Cut(managedEnv, "/")
	defer e.RunCommand("pulumi", "stack", "rm", "--yes", "--force")

	client := &http.Client{Timeout: time.Minute}
	send := func(method string, path string, body []byte, ifMatch string) (int, string, http.Header) {
		req, err := http.NewRequestWithContext(t.Context(), method, apiURL+path, bytes.NewReader(body))
		require.NoError(t, err)
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.pulumi+8")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		payload, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(payload), resp.Header
	}

	call := func(method string, path string, body any) (int, string) {
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			require.NoError(t, err)
		}
		status, response, _ := send(method, path, payload, "")
		return status, response
	}

	baseRevision := func(env map[string]string) int {
		revision, err := strconv.Atoi(env[managedEnvKeyPrefix+".baseRevision"])
		require.NoError(t, err)
		return revision
	}

	definition := func(value string) []byte {
		return fmt.Appendf(nil, "values:\n  pulumiConfig:\n    plain: %s\n", value)
	}

	// `pulumi env edit --draft` submits the draft it creates, which the service refuses on a managed environment,
	// so drafts are created and edited through the API.
	draftsPath := fmt.Sprintf("/api/esc/environments/%s/%s/%s/drafts", org, envProject, envName)

	createDraft := func(value string) string {
		status, body, _ := send(http.MethodPost, draftsPath, definition(value), "")
		require.Equal(t, http.StatusOK, status, body)
		var created struct {
			ChangeRequestID string `json:"changeRequestId"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &created))
		require.NotEmpty(t, created.ChangeRequestID, body)
		return created.ChangeRequestID
	}

	editDraft := func(id string, value string) {
		status, body, header := send(http.MethodGet, draftsPath+"/"+id, nil, "")
		require.Equal(t, http.StatusOK, status, body)
		etag := header.Get("ETag")
		require.NotEmpty(t, etag, "draft read returned no ETag")
		status, body, _ = send(http.MethodPatch, draftsPath+"/"+id, definition(value), etag)
		require.Equal(t, http.StatusOK, status, body)
	}

	startRawUpdate := func(environment apitype.UpdateProgramEnvironment) (int, string) {
		stackPath := fmt.Sprintf("/api/stacks/%s/%s/%s", org, managedEnvProject, stackName)
		status, body := call(http.MethodPost, stackPath+"/update", apitype.UpdateProgramRequest{
			Name:        managedEnvProject,
			Runtime:     "yaml",
			Config:      map[string]apitype.ConfigValue{},
			Environment: &environment,
		})
		require.Equal(t, http.StatusOK, status, body)
		var created apitype.UpdateProgramResponse
		require.NoError(t, json.Unmarshal([]byte(body), &created))
		return call(http.MethodPost, stackPath+"/update/"+created.UpdateID, apitype.StartUpdateRequest{})
	}

	e.RunCommand("pulumi", "config", "set", "plain", "one")
	e.RunCommand("pulumi", "config", "set", "--secret", "token", "hunter2")

	t.Run("PlainConfigListsSecretAsSecret", func(t *testing.T) {
		out, _ := e.RunCommand("pulumi", "config")
		assert.Regexp(t, `token\s+\[secret\]`, out)
		assert.NotContains(t, out, "hunter2")
	})

	var changedBase int
	t.Run("ChangedAndApplied", func(t *testing.T) {
		e.RunCommand("pulumi", "up", "--yes", "--skip-preview")
		env := latestUpdateEnvironment(t, e)
		changedBase = baseRevision(env)
		assert.Equal(t, managedEnv, env[managedEnvKeyPrefix])
		assert.NotEmpty(t, env[managedEnvKeyPrefix+".changeRequestID"])
		assert.Equal(t, "0", env[managedEnvKeyPrefix+".draftRevision"])
	})

	var publishedRevision int
	t.Run("Unchanged", func(t *testing.T) {
		e.RunCommand("pulumi", "up", "--yes", "--skip-preview")
		env := latestUpdateEnvironment(t, e)
		publishedRevision = baseRevision(env)
		assert.Greater(t, publishedRevision, changedBase)
		assert.NotContains(t, env, managedEnvKeyPrefix+".changeRequestID")
		assert.NotContains(t, env, managedEnvKeyPrefix+".draftRevision")
	})

	t.Run("StaleBase", func(t *testing.T) {
		status, body := startRawUpdate(apitype.UpdateProgramEnvironment{BaseRevision: 0})
		assert.Equal(t, http.StatusConflict, status, body)
		assert.Contains(t, body, "changed since it was evaluated")
	})

	t.Run("EditedDraft", func(t *testing.T) {
		require.NotZero(t, publishedRevision, "Unchanged must publish a revision first")
		id := createDraft("edited-one")
		editDraft(id, "edited-two")
		status, body := startRawUpdate(apitype.UpdateProgramEnvironment{
			BaseRevision:    publishedRevision,
			ChangeRequestID: id,
			DraftRevision:   0,
		})
		assert.Equal(t, http.StatusConflict, status, body)
		assert.Contains(t, body, "edited after evaluation")
	})

	t.Run("OpenByRevisionIgnoresLaterEdit", func(t *testing.T) {
		id := createDraft("pinned-one")
		editDraft(id, "pinned-two")
		status, body := call(http.MethodPost, draftsPath+"/"+id+"/open?duration=5m&revision=0", nil)
		require.Equal(t, http.StatusOK, status, body)
		var opened struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &opened))
		status, values := call(http.MethodGet,
			fmt.Sprintf("/api/esc/environments/%s/%s/%s/open/%s", org, envProject, envName, opened.ID), nil)
		require.Equal(t, http.StatusOK, status, values)
		assert.Contains(t, values, "pinned-one")
		assert.NotContains(t, values, "pinned-two")
	})

	t.Run("NewFromTemplate", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.SetBackend(apiURL)
		e.SetEnvVars("PULUMI_EXPERIMENTAL=true")

		templateDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(templateDir, "Pulumi.yaml"), []byte(managedEnvTemplateYAML), 0o600))

		// A unique project, since non-interactive `pulumi new` refuses a project name that already exists.
		stackName := ptesting.RandomStackName()
		projectName := managedEnvProject + "-new-" + stackName
		stackRef := org + "/" + projectName + "/" + stackName
		// Registered before `pulumi new`, which can fail after it has created the stack and its environment.
		defer func() {
			_, rmErr, err := e.GetCommandResults("pulumi", "stack", "rm", "--yes", "--force", "--stack", stackRef)
			if err != nil {
				t.Errorf("removing stack %s: %v\n%s", stackRef, err, rmErr)
			}
		}()
		stdout, stderr := e.RunCommand("pulumi", "new", templateDir, "--name", projectName,
			"--stack", org+"/"+stackName, "--esc-config", "--yes", "--force")
		managedEnv := createdManagedEnvironment(t, "pulumi new", stdout, stderr)

		raw, err := os.ReadFile(filepath.Join(e.RootPath, "Pulumi."+stackName+".yaml"))
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "hunter2")
		var stackFile struct {
			Config      map[string]any `yaml:"config"`
			Environment struct {
				Values struct {
					PulumiConfig map[string]any `yaml:"pulumiConfig"`
				} `yaml:"values"`
			} `yaml:"environment"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &stackFile))
		assert.Empty(t, stackFile.Config)
		pulumiConfig := stackFile.Environment.Values.PulumiConfig
		assert.Equal(t, "one", pulumiConfig[projectName+":plain"])
		token, ok := pulumiConfig[projectName+":token"].(map[any]any)
		require.True(t, ok, "token is not an fn::secret: %v", pulumiConfig)
		secret, ok := token["fn::secret"].(map[any]any)
		require.True(t, ok, "token is not an fn::secret: %v", pulumiConfig)
		assert.NotEmpty(t, secret["ciphertext"])

		e.RunCommand("pulumi", "up", "--yes", "--skip-preview")
		env := latestUpdateEnvironment(t, e)
		assert.Equal(t, managedEnv, env[managedEnvKeyPrefix])
		assert.NotEmpty(t, env[managedEnvKeyPrefix+".changeRequestID"])

		// ESC can only decrypt the token if `pulumi new` encrypted it with the environment's key.
		out, _ := e.RunCommand("pulumi", "env", "open", org+"/"+managedEnv)
		var opened struct {
			PulumiConfig map[string]any `json:"pulumiConfig"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &opened))
		assert.Equal(t, "one", opened.PulumiConfig[projectName+":plain"])
		assert.Equal(t, "hunter2", opened.PulumiConfig[projectName+":token"])
	})
}
