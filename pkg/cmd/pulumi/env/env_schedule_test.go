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

package env

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// scheduleCall captures one call to the mock client.
type scheduleCall struct {
	method  string
	org     string
	project string
	env     string
	id      string
	req     apitype.CreateEnvironmentScheduleRequest
}

// mockScheduleClient is a stub for envScheduleClient that records every call.
// Per-method `*Result` / `*Err` fields let tests configure the return values.
type mockScheduleClient struct {
	listResult   apitype.ListScheduledActionsResponse
	listErr      error
	createResult apitype.ScheduledAction
	createErr    error
	pauseErr     error
	resumeErr    error
	deleteErr    error
	calls        []scheduleCall
}

func (m *mockScheduleClient) ListEnvironmentSchedules(
	_ context.Context, org, project, env string,
) (apitype.ListScheduledActionsResponse, error) {
	m.calls = append(m.calls, scheduleCall{method: "list", org: org, project: project, env: env})
	return m.listResult, m.listErr
}

func (m *mockScheduleClient) CreateEnvironmentSchedule(
	_ context.Context, org, project, env string, req apitype.CreateEnvironmentScheduleRequest,
) (apitype.ScheduledAction, error) {
	m.calls = append(m.calls, scheduleCall{
		method: "create", org: org, project: project, env: env, req: req,
	})
	return m.createResult, m.createErr
}

func (m *mockScheduleClient) PauseEnvironmentSchedule(_ context.Context, org, project, env, id string) error {
	m.calls = append(m.calls, scheduleCall{method: "pause", org: org, project: project, env: env, id: id})
	return m.pauseErr
}

func (m *mockScheduleClient) ResumeEnvironmentSchedule(_ context.Context, org, project, env, id string) error {
	m.calls = append(m.calls, scheduleCall{method: "resume", org: org, project: project, env: env, id: id})
	return m.resumeErr
}

func (m *mockScheduleClient) DeleteEnvironmentSchedule(_ context.Context, org, project, env, id string) error {
	m.calls = append(m.calls, scheduleCall{method: "delete", org: org, project: project, env: env, id: id})
	return m.deleteErr
}

// stubScheduleFactory builds an envScheduleFactory that always returns the
// supplied client and defaultOrg unless overridden by --org.
func stubScheduleFactory(c envScheduleClient, defaultOrg string) envScheduleFactory {
	return func(_ context.Context, orgOverride string) (envScheduleClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return c, org, nil
	}
}

func failingScheduleFactory(err error) envScheduleFactory {
	return func(_ context.Context, _ string) (envScheduleClient, string, error) {
		return nil, "", err
	}
}

// runCmd executes the parent `env schedule` cobra command with the supplied
// args, capturing stdout/stderr.
func runScheduleCmd(t *testing.T, factory envScheduleFactory, args ...string) (string, error) {
	t.Helper()
	cmd := newEnvScheduleCmdWith(factory)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestEnvScheduleList(t *testing.T) {
	t.Parallel()

	t.Run("default output renders all schedules", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{listResult: apitype.ListScheduledActionsResponse{
			Schedules: []apitype.ScheduledAction{
				{ID: "s1", Kind: "environment_rotation", ScheduleCron: "0 9 * * *", NextExecution: "2030-01-01T09:00:00Z"},
				{ID: "s2", Kind: "environment_rotation", ScheduleOnce: "2030-06-01T00:00:00Z", Paused: true},
			},
		}}
		out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"), "list", "proj", "env")
		require.NoError(t, err)
		assert.Contains(t, out, "ID:        s1")
		assert.Contains(t, out, "Cron:      0 9 * * *")
		assert.Contains(t, out, "Next run:  2030-01-01T09:00:00Z")
		assert.Contains(t, out, "ID:        s2")
		assert.Contains(t, out, "Once:      2030-06-01T00:00:00Z")
		assert.Contains(t, out, "Paused:    true")

		require.Len(t, c.calls, 1)
		assert.Equal(t, "list", c.calls[0].method)
		assert.Equal(t, "acme", c.calls[0].org)
		assert.Equal(t, "proj", c.calls[0].project)
		assert.Equal(t, "env", c.calls[0].env)
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"), "list", "proj", "env")
		require.NoError(t, err)
		assert.Contains(t, out, "No schedules configured")
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{listResult: apitype.ListScheduledActionsResponse{
			Schedules: []apitype.ScheduledAction{{ID: "s1", Kind: "environment_rotation"}},
		}}
		out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"list", "proj", "env", "--output", "json")
		require.NoError(t, err)
		assert.Contains(t, out, `"id": "s1"`)
		assert.Contains(t, out, `"schedules"`)
	})

	t.Run("invalid output rejected before call", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"list", "proj", "env", "--output", "yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --output")
		assert.Empty(t, c.calls, "expected no client call when output validation fails")
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()
		_, err := runScheduleCmd(t, failingScheduleFactory(errors.New("not logged in")),
			"list", "proj", "env")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("--org override wins", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "default-org"),
			"list", "proj", "env", "--org", "override-org")
		require.NoError(t, err)
		require.Len(t, c.calls, 1)
		assert.Equal(t, "override-org", c.calls[0].org)
	})

	t.Run("list http error wraps", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{listErr: errors.New("boom")}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"list", "proj", "env")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing schedules")
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestEnvScheduleNew(t *testing.T) {
	t.Parallel()

	t.Run("cron schedule with rotate-secrets action", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{createResult: apitype.ScheduledAction{
			ID: "s1", Kind: "environment_rotation", ScheduleCron: "0 9 * * *",
		}}
		out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "0 9 * * *")
		require.NoError(t, err)
		assert.Contains(t, out, "ID:        s1")

		require.Len(t, c.calls, 1)
		got := c.calls[0]
		assert.Equal(t, "create", got.method)
		assert.Equal(t, "0 9 * * *", got.req.ScheduleCron)
		assert.Empty(t, got.req.ScheduleOnce)
		require.NotNil(t, got.req.SecretRotationRequest)
	})

	t.Run("once schedule", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{createResult: apitype.ScheduledAction{ID: "s1"}}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--once", "2030-01-01T00:00:00Z")
		require.NoError(t, err)
		require.Len(t, c.calls, 1)
		assert.Equal(t, "2030-01-01T00:00:00Z", c.calls[0].req.ScheduleOnce)
		assert.Empty(t, c.calls[0].req.ScheduleCron)
	})

	t.Run("rejects both --cron and --once", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "* * * * *", "--once", "2030-01-01T00:00:00Z")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one of --cron or --once")
		assert.Empty(t, c.calls)
	})

	t.Run("rejects neither --cron nor --once", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one of --cron or --once")
		assert.Empty(t, c.calls)
	})

	t.Run("rejects unsupported --action", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "* * * * *", "--action", "deploy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported --action")
		assert.Empty(t, c.calls)
	})

	t.Run("invalid output rejected", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "* * * * *", "--output", "yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --output")
		assert.Empty(t, c.calls)
	})

	t.Run("create error wraps", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{createErr: errors.New("conflict")}
		_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "* * * * *")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating schedule")
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()
		c := &mockScheduleClient{createResult: apitype.ScheduledAction{ID: "s1"}}
		out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
			"new", "proj", "env", "--cron", "* * * * *", "--output", "json")
		require.NoError(t, err)
		assert.Contains(t, out, `"id": "s1"`)
	})
}

func TestEnvScheduleSimpleCommands(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		cmd, method, successWord string
	}{
		{"pause", "pause", "Paused"},
		{"resume", "resume", "Resumed"},
		{"remove", "delete", "Deleted"},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			c := &mockScheduleClient{}
			out, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
				tc.cmd, "proj", "env", "sched-1")
			require.NoError(t, err)
			require.Len(t, c.calls, 1)
			got := c.calls[0]
			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "acme", got.org)
			assert.Equal(t, "proj", got.project)
			assert.Equal(t, "env", got.env)
			assert.Equal(t, "sched-1", got.id)
			assert.True(t, strings.HasPrefix(out, tc.successWord),
				"expected output prefix %q, got %q", tc.successWord, out)
			assert.Contains(t, out, "sched-1")
			assert.Contains(t, out, "acme/proj/env")
		})

		t.Run(tc.cmd+" error wraps", func(t *testing.T) {
			t.Parallel()
			c := &mockScheduleClient{
				pauseErr:  errors.New("boom"),
				resumeErr: errors.New("boom"),
				deleteErr: errors.New("boom"),
			}
			_, err := runScheduleCmd(t, stubScheduleFactory(c, "acme"),
				tc.cmd, "proj", "env", "sched-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.cmd+" sched-1")
			assert.Contains(t, err.Error(), "boom")
		})
	}
}

func TestEnvScheduleCmd_FactoryDefaults(t *testing.T) {
	t.Parallel()

	// Constructing the command tree with a nil factory must not panic;
	// the default factory is only resolved at RunE time.
	cmd := newEnvScheduleCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "schedule", cmd.Name())

	subs := cmd.Commands()
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		names = append(names, sub.Name())
	}
	assert.ElementsMatch(t, []string{"list", "new", "pause", "resume", "remove"}, names)
}
