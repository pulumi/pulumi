// Copyright 2024, Pulumi Corporation.
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

package config

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func newTestRemoteStackForEdit(tb testing.TB, eb *backend.MockEnvironmentsBackend, escEnv string) *backend.MockStack {
	tb.Helper()
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend { return eb },
	}
	s.OrgNameF = func() string { return "myorg" }
	return s
}

// TestConfigEdit_RemoteStack_NoChanges verifies that when the editor makes no changes
// to the temp file, UpdateEnvironmentWithProject is never called.
func TestConfigEdit_RemoteStack_NoChanges(t *testing.T) {
	t.Parallel()

	const envYAML = "values:\n  pulumiConfig:\n    myproject:host: localhost\n"

	eb := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte(envYAML), "etag123", 1, nil
		},
		UpdateEnvironmentWithProjectF: func(_ context.Context, _, _, _ string, _ []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			t.Fatal("UpdateEnvironmentWithProject must not be called when there are no changes")
			return nil, nil
		},
	}

	stack := newTestRemoteStackForEdit(t, eb, "myproject/dev")
	cmd := &configEditCmd{stdout: io.Discard}

	// openEditor does nothing — file content remains identical to what was written.
	noopEditor := func(filename string) error { return nil }

	err := cmd.editRemote(context.Background(), stack, noopEditor)
	require.NoError(t, err)
}

// TestConfigEdit_RemoteStack_WithChanges verifies that modifications are uploaded
// with the correct etag for optimistic concurrency.
func TestConfigEdit_RemoteStack_WithChanges(t *testing.T) {
	t.Parallel()

	const origYAML = "values:\n  pulumiConfig:\n    myproject:host: localhost\n"
	const newYAML = "values:\n  pulumiConfig:\n    myproject:host: remotehost\n"

	var uploadedYAML []byte
	var uploadedEtag string

	eb := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte(origYAML), "etag123", 1, nil
		},
		UpdateEnvironmentWithProjectF: func(_ context.Context, _, _, _ string, yaml []byte, etag string) (apitype.EnvironmentDiagnostics, error) {
			uploadedYAML = yaml
			uploadedEtag = etag
			return nil, nil
		},
	}

	stack := newTestRemoteStackForEdit(t, eb, "myproject/dev")
	cmd := &configEditCmd{stdout: io.Discard}

	// Simulate user editing: overwrite the temp file with new content.
	editingEditor := func(filename string) error {
		return os.WriteFile(filename, []byte(newYAML), 0o600)
	}

	err := cmd.editRemote(context.Background(), stack, editingEditor)
	require.NoError(t, err)
	assert.Equal(t, []byte(newYAML), uploadedYAML)
	assert.Equal(t, "etag123", uploadedEtag)
}
