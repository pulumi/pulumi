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

package config

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func newTestRemoteStackForRestore(
	tb testing.TB, eb *backend.MockEnvironmentsBackend, escEnv string,
) *backend.MockStack {
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

func TestConfigRestore_CreatesNewRevision(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForRestore(t, eb, "myproject/dev")

	oldContent := []byte("values:\n  pulumiConfig:\n    key: oldvalue\n")
	var writtenYAML []byte
	var writtenEtag string

	var stdout bytes.Buffer
	cmd := &configRestoreCmd{
		stdout: &stdout,
		getEnvironment: func(
			_ context.Context, _ backend.EnvironmentsBackend,
			_, _, _, version string, _ bool,
		) ([]byte, string, int, error) {
			if version == "3" {
				return oldContent, "etag-old", 3, nil
			}
			return []byte("values:\n  pulumiConfig:\n    key: currentvalue\n"), "etag-current", 5, nil
		},
		updateEnvironmentWithProject: func(
			_ context.Context, _ backend.EnvironmentsBackend,
			_, _, _ string, yaml []byte, etag string,
		) error {
			writtenYAML = yaml
			writtenEtag = etag
			return nil
		},
	}

	err := cmd.restoreRevision(context.Background(), stack, "myproject/dev", "3")
	require.NoError(t, err)
	assert.Equal(t, oldContent, writtenYAML)
	assert.Equal(t, "etag-current", writtenEtag)
	assert.Contains(t, stdout.String(), "Restored config to revision 3")
}

func TestConfigRestore_EtagConflict(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForRestore(t, eb, "myproject/dev")

	cmd := &configRestoreCmd{
		stdout: &bytes.Buffer{},
		getEnvironment: func(
			_ context.Context, _ backend.EnvironmentsBackend,
			_, _, _, version string, _ bool,
		) ([]byte, string, int, error) {
			if version == "3" {
				return []byte("old"), "etag-old", 3, nil
			}
			return []byte("current"), "etag-current", 5, nil
		},
		updateEnvironmentWithProject: func(
			_ context.Context, _ backend.EnvironmentsBackend,
			_, _, _ string, _ []byte, _ string,
		) error {
			return &apitype.ErrorResponse{Code: http.StatusConflict, Message: "conflict"}
		},
	}

	err := cmd.restoreRevision(context.Background(), stack, "myproject/dev", "3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified concurrently")
}

func TestConfigRestore_LocalStack(t *testing.T) {
	t.Parallel()

	localEnv := "myproject/dev"
	localBackend := &backend.MockBackend{
		NameF: func() string { return "file" },
	}
	stack := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: false, EscEnv: &localEnv}
		},
		BackendF: func() backend.Backend { return localBackend },
	}

	cmd := &configRestoreCmd{
		stdout: &bytes.Buffer{},
	}

	err := cmd.restoreRevision(context.Background(), stack, "myproject/dev", "3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support ESC environments")
}

func TestConfigRestore_RevisionNotFound(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForRestore(t, eb, "myproject/dev")

	cmd := &configRestoreCmd{
		stdout: &bytes.Buffer{},
		getEnvironment: func(
			_ context.Context, _ backend.EnvironmentsBackend,
			_, _, _, version string, _ bool,
		) ([]byte, string, int, error) {
			if version == "999" {
				return nil, "", 0, &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
			}
			return []byte("current"), "etag-current", 5, nil
		},
	}

	err := cmd.restoreRevision(context.Background(), stack, "myproject/dev", "999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision 999 not found")
}
