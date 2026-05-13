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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockEnvWebhookClient records every call the cobra command makes so tests can
// assert flag propagation, and returns either canned fixture data or a canned
// error. A single struct backs every test (one field per recorded call) so a
// single mock instance can be passed to commands that chain GET-then-PATCH.
type mockEnvWebhookClient struct {
	// list
	listHooks []apitype.EnvironmentWebhook
	listErr   error
	listCall  *struct{ org, project, env string }

	// get
	getHook apitype.EnvironmentWebhook
	getErr  error
	getCall *struct{ org, project, env, name string }

	// create
	createResp apitype.EnvironmentWebhook
	createErr  error
	createCall *struct {
		org, project, env string
		req               apitype.CreateEnvironmentWebhookRequest
	}

	// update
	updateResp apitype.EnvironmentWebhook
	updateErr  error
	updateCall *struct {
		org, project, env, name string
		req                     apitype.UpdateEnvironmentWebhookRequest
	}

	// delete
	deleteErr  error
	deleteCall *struct{ org, project, env, name string }

	// ping
	pingResp apitype.EnvironmentWebhookDelivery
	pingErr  error
	pingCall *struct{ org, project, env, name string }

	// deliveries
	deliveries     []apitype.EnvironmentWebhookDelivery
	deliveriesErr  error
	deliveriesCall *struct{ org, project, env, name string }
}

func (m *mockEnvWebhookClient) ListEnvironmentWebhooks(
	_ context.Context, org, project, env string,
) ([]apitype.EnvironmentWebhook, error) {
	if m.listCall != nil {
		*m.listCall = struct{ org, project, env string }{org, project, env}
	}
	return m.listHooks, m.listErr
}

func (m *mockEnvWebhookClient) GetEnvironmentWebhook(
	_ context.Context, org, project, env, name string,
) (apitype.EnvironmentWebhook, error) {
	if m.getCall != nil {
		*m.getCall = struct{ org, project, env, name string }{org, project, env, name}
	}
	return m.getHook, m.getErr
}

func (m *mockEnvWebhookClient) CreateEnvironmentWebhook(
	_ context.Context, org, project, env string, req apitype.CreateEnvironmentWebhookRequest,
) (apitype.EnvironmentWebhook, error) {
	if m.createCall != nil {
		*m.createCall = struct {
			org, project, env string
			req               apitype.CreateEnvironmentWebhookRequest
		}{org, project, env, req}
	}
	return m.createResp, m.createErr
}

func (m *mockEnvWebhookClient) UpdateEnvironmentWebhook(
	_ context.Context, org, project, env, name string, req apitype.UpdateEnvironmentWebhookRequest,
) (apitype.EnvironmentWebhook, error) {
	if m.updateCall != nil {
		*m.updateCall = struct {
			org, project, env, name string
			req                     apitype.UpdateEnvironmentWebhookRequest
		}{org, project, env, name, req}
	}
	return m.updateResp, m.updateErr
}

func (m *mockEnvWebhookClient) DeleteEnvironmentWebhook(
	_ context.Context, org, project, env, name string,
) error {
	if m.deleteCall != nil {
		*m.deleteCall = struct{ org, project, env, name string }{org, project, env, name}
	}
	return m.deleteErr
}

func (m *mockEnvWebhookClient) PingEnvironmentWebhook(
	_ context.Context, org, project, env, name string,
) (apitype.EnvironmentWebhookDelivery, error) {
	if m.pingCall != nil {
		*m.pingCall = struct{ org, project, env, name string }{org, project, env, name}
	}
	return m.pingResp, m.pingErr
}

func (m *mockEnvWebhookClient) ListEnvironmentWebhookDeliveries(
	_ context.Context, org, project, env, name string,
) ([]apitype.EnvironmentWebhookDelivery, error) {
	if m.deliveriesCall != nil {
		*m.deliveriesCall = struct{ org, project, env, name string }{org, project, env, name}
	}
	return m.deliveries, m.deliveriesErr
}

// stubEnvWebhookFactory returns a factory that always yields the supplied
// client and (overridable) org.
func stubEnvWebhookFactory(c envWebhookClient, defaultOrg string) envWebhookFactory {
	return func(_ context.Context, override string) (envWebhookClient, string, error) {
		org := override
		if org == "" {
			org = defaultOrg
		}
		return c, org, nil
	}
}

// failingEnvWebhookFactory returns a factory that always errors.
func failingEnvWebhookFactory(err error) envWebhookFactory {
	return func(_ context.Context, _ string) (envWebhookClient, string, error) {
		return nil, "", err
	}
}

// --- list --------------------------------------------------------------------

func TestEnvWebhookList(t *testing.T) {
	t.Parallel()

	t.Run("text output with multiple webhooks", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			listHooks: []apitype.EnvironmentWebhook{
				{Name: "a", DisplayName: "Alpha", PayloadURL: "https://a.example", Active: true},
				{Name: "b", DisplayName: "Beta", PayloadURL: "https://b.example", Active: false},
			},
			listCall: &struct{ org, project, env string }{},
		}

		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "myproj", "myenv"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		assert.Equal(t, "acme", mock.listCall.org)
		assert.Equal(t, "myproj", mock.listCall.project)
		assert.Equal(t, "myenv", mock.listCall.env)
		assert.Contains(t, out.String(), "a")
		assert.Contains(t, out.String(), "Alpha")
		assert.Contains(t, out.String(), "2 webhook(s)")
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			listHooks: []apitype.EnvironmentWebhook{
				{Name: "a", DisplayName: "Alpha", PayloadURL: "https://a.example", Active: true},
			},
		}

		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "myproj", "myenv", "--output", "json"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		var got struct {
			Webhooks []apitype.EnvironmentWebhook `json:"webhooks"`
			Count    int                          `json:"count"`
		}
		require.NoError(t, json.Unmarshal(out.Bytes(), &got))
		assert.Equal(t, 1, got.Count)
		assert.Equal(t, "a", got.Webhooks[0].Name)
	})

	t.Run("empty list prints a friendly line", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "p", "e"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Contains(t, out.String(), "No webhooks configured")
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "p", "e"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- new ---------------------------------------------------------------------

func TestEnvWebhookNew(t *testing.T) {
	t.Parallel()

	t.Run("forwards all flags to the create request", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			createResp: apitype.EnvironmentWebhook{
				Name: "hook1", DisplayName: "Hook One", PayloadURL: "https://x", Active: true,
			},
			createCall: &struct {
				org, project, env string
				req               apitype.CreateEnvironmentWebhookRequest
			}{},
		}

		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"new", "p", "e", "hook1",
			"--url", "https://x",
			"--display-name", "Hook One",
			"--format", "slack",
			"--filter", "added",
			"--filter", "removed",
			"--active=false",
			"--secret", "shh",
		})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		assert.Equal(t, "acme", mock.createCall.org)
		assert.Equal(t, "p", mock.createCall.project)
		assert.Equal(t, "e", mock.createCall.env)
		assert.Equal(t, apitype.CreateEnvironmentWebhookRequest{
			Name:        "hook1",
			DisplayName: "Hook One",
			PayloadURL:  "https://x",
			Active:      false,
			Format:      "slack",
			Filters:     []string{"added", "removed"},
			Secret:      "shh",
		}, mock.createCall.req)
	})

	t.Run("defaults display name to webhook name", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			createCall: &struct {
				org, project, env string
				req               apitype.CreateEnvironmentWebhookRequest
			}{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"new", "p", "e", "hook1", "--url", "https://x"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, "hook1", mock.createCall.req.DisplayName)
		assert.Equal(t, "raw", mock.createCall.req.Format)
		assert.True(t, mock.createCall.req.Active)
	})

	t.Run("--url is required", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"new", "p", "e", "hook1"})
		err := cmd.ExecuteContext(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"new", "p", "e", "h", "--url", "https://x"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- edit --------------------------------------------------------------------

func TestEnvWebhookEdit(t *testing.T) {
	t.Parallel()

	t.Run("only --active=false sends just that field", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			updateCall: &struct {
				org, project, env, name string
				req                     apitype.UpdateEnvironmentWebhookRequest
			}{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"edit", "p", "e", "h", "--active=false"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		got := mock.updateCall.req
		require.NotNil(t, got.Active)
		assert.False(t, *got.Active)
		assert.Nil(t, got.PayloadURL)
		assert.Nil(t, got.DisplayName)
		assert.Nil(t, got.Format)
		assert.Nil(t, got.Filters)
		assert.Nil(t, got.Secret)
	})

	t.Run("no flags sends an empty PATCH body but still hits server", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			updateCall: &struct {
				org, project, env, name string
				req                     apitype.UpdateEnvironmentWebhookRequest
			}{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"edit", "p", "e", "h"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, "h", mock.updateCall.name)
		assert.Equal(t, apitype.UpdateEnvironmentWebhookRequest{}, mock.updateCall.req)
	})

	t.Run("--add-filter triggers GET-then-PATCH and merges filters", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			getHook: apitype.EnvironmentWebhook{
				Name: "h", Filters: []string{"existing-1", "existing-2"},
			},
			getCall: &struct{ org, project, env, name string }{},
			updateCall: &struct {
				org, project, env, name string
				req                     apitype.UpdateEnvironmentWebhookRequest
			}{},
		}

		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"edit", "p", "e", "h",
			"--add-filter", "added-1",
			"--remove-filter", "existing-2",
		})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, "h", mock.getCall.name)
		require.NotNil(t, mock.updateCall.req.Filters)
		assert.Equal(t, []string{"existing-1", "added-1"}, *mock.updateCall.req.Filters)
	})

	t.Run("--filter replaces the list outright with no GET", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			updateCall: &struct {
				org, project, env, name string
				req                     apitype.UpdateEnvironmentWebhookRequest
			}{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"edit", "p", "e", "h", "--filter", "only-one"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		// GET should not be invoked: getCall is nil so we can't assert; just
		// check Filters was set to the literal flag value.
		require.NotNil(t, mock.updateCall.req.Filters)
		assert.Equal(t, []string{"only-one"}, *mock.updateCall.req.Filters)
	})

	t.Run("--filter together with --add-filter is rejected", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"edit", "p", "e", "h", "--filter", "x", "--add-filter", "y"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "cannot be combined")
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"edit", "p", "e", "h"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- remove ------------------------------------------------------------------

func TestEnvWebhookRemove(t *testing.T) {
	t.Parallel()

	t.Run("delete and confirmation line", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			deleteCall: &struct{ org, project, env, name string }{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"remove", "p", "e", "h"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, "h", mock.deleteCall.name)
		assert.Contains(t, out.String(), "Removed webhook")
		assert.Contains(t, out.String(), "h")
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"remove", "p", "e", "h"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- ping --------------------------------------------------------------------

func TestEnvWebhookPing(t *testing.T) {
	t.Parallel()

	t.Run("text rendering", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			pingResp: apitype.EnvironmentWebhookDelivery{
				ID: "did-1", Kind: "ping", ResponseCode: 200, Duration: 42,
			},
			pingCall: &struct{ org, project, env, name string }{},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"ping", "p", "e", "h"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, "h", mock.pingCall.name)
		s := out.String()
		assert.Contains(t, s, "did-1")
		assert.Contains(t, s, "ping")
		assert.Contains(t, s, "200")
		assert.Contains(t, s, "42")
	})

	t.Run("json round-trips through apitype", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			pingResp: apitype.EnvironmentWebhookDelivery{
				ID: "did-1", Kind: "ping", Timestamp: 123, Duration: 42,
				Payload: "{}", RequestURL: "https://x", RequestHeaders: "{}",
				ResponseCode: 200, ResponseHeaders: "{}", ResponseBody: "{}",
			},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"ping", "p", "e", "h", "--output", "json"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		var got apitype.EnvironmentWebhookDelivery
		require.NoError(t, json.Unmarshal(out.Bytes(), &got))
		assert.Equal(t, mock.pingResp, got)
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"ping", "p", "e", "h"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- delivery list -----------------------------------------------------------

func TestEnvWebhookDeliveryList(t *testing.T) {
	t.Parallel()

	t.Run("renders rows", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			deliveries: []apitype.EnvironmentWebhookDelivery{
				{ID: "d1", Kind: "ping", Timestamp: 1, ResponseCode: 200, Duration: 10},
				{ID: "d2", Kind: "stack.updated", Timestamp: 2, ResponseCode: 500, Duration: 20},
			},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"delivery", "list", "p", "e", "h"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		s := out.String()
		assert.Contains(t, s, "d1")
		assert.Contains(t, s, "d2")
		assert.Contains(t, s, "2 delivery(ies)")
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"delivery", "list", "p", "e", "h"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Contains(t, out.String(), "No deliveries")
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()

		mock := &mockEnvWebhookClient{
			deliveries: []apitype.EnvironmentWebhookDelivery{
				{ID: "d1", Kind: "ping", ResponseCode: 200},
			},
		}
		cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"delivery", "list", "p", "e", "h", "--output", "json"})
		require.NoError(t, cmd.ExecuteContext(t.Context()))

		var env struct {
			Deliveries []apitype.EnvironmentWebhookDelivery `json:"deliveries"`
			Count      int                                  `json:"count"`
		}
		require.NoError(t, json.Unmarshal(out.Bytes(), &env))
		assert.Equal(t, 1, env.Count)
		assert.Equal(t, "d1", env.Deliveries[0].ID)
	})

	t.Run("factory error propagates", func(t *testing.T) {
		t.Parallel()

		cmd := newEnvWebhookCmdWith(failingEnvWebhookFactory(errors.New("nope")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"delivery", "list", "p", "e", "h"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "nope")
	})
}

// --- helpers -----------------------------------------------------------------

func TestMergeFilters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                          string
		existing, adds, removes, want []string
	}{
		{"no-op", []string{"a", "b"}, nil, nil, []string{"a", "b"}},
		{"add new", []string{"a"}, []string{"b"}, nil, []string{"a", "b"}},
		{"add dup", []string{"a"}, []string{"a"}, nil, []string{"a"}},
		{"remove existing", []string{"a", "b"}, nil, []string{"a"}, []string{"b"}},
		{"add and remove", []string{"a", "b"}, []string{"c"}, []string{"b"}, []string{"a", "c"}},
		{"remove wins over add", []string{}, []string{"a"}, []string{"a"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, mergeFilters(tc.existing, tc.adds, tc.removes))
		})
	}
}

func TestEnvWebhookInvalidOutput(t *testing.T) {
	t.Parallel()

	mock := &mockEnvWebhookClient{}
	cmd := newEnvWebhookCmdWith(stubEnvWebhookFactory(mock, "acme"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "p", "e", "--output", "yaml"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "invalid --output")
}
