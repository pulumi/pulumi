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

package neo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHandler struct {
	wantMethod string
	result     any
	err        error
}

func (f *fakeHandler) Invoke(_ context.Context, method string, _ json.RawMessage) (any, error) {
	if method != f.wantMethod {
		return nil, errors.New("unexpected method " + method)
	}
	return f.result, f.err
}

func TestExecutor_DispatchesToHandlerBySplittingName(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	exec.Register("filesystem", &fakeHandler{
		wantMethod: "read",
		result:     map[string]any{"content": "hello"},
	})

	items := exec.Execute(t.Context(), []ToolCall{
		{ToolCallID: "c1", Name: "filesystem__read", Args: json.RawMessage(`{}`)},
	})

	require.Len(t, items, 1)
	assert.Equal(t, "c1", items[0].ToolCallID)
	assert.Equal(t, "filesystem__read", items[0].Name)
	assert.False(t, items[0].IsError)
	assert.Equal(t, map[string]any{"content": "hello"}, items[0].Content)
}

func TestExecutor_UnknownServerReturnsErrorItem(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	items := exec.Execute(t.Context(), []ToolCall{
		{ToolCallID: "c1", Name: "vcs__commit"},
	})

	require.Len(t, items, 1)
	assert.True(t, items[0].IsError)
	assert.Contains(t, items[0].Content.(map[string]string)["error"], `tool "vcs" is not available`)
}

func TestExecutor_NameWithoutSeparatorIsAnError(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	items := exec.Execute(t.Context(), []ToolCall{
		{ToolCallID: "c1", Name: "bare_name"},
	})

	require.Len(t, items, 1)
	assert.True(t, items[0].IsError)
	assert.Contains(t, items[0].Content.(map[string]string)["error"], "missing the server prefix")
}

func TestExecutor_HandlerErrorBecomesErrorItem(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	exec.Register("shell", &fakeHandler{
		wantMethod: "shell_execute",
		err:        errors.New("boom"),
	})
	items := exec.Execute(t.Context(), []ToolCall{
		{ToolCallID: "c", Name: "shell__shell_execute"},
	})
	require.Len(t, items, 1)
	assert.True(t, items[0].IsError)
	assert.Equal(t, "boom", items[0].Content.(map[string]string)["error"])
}
