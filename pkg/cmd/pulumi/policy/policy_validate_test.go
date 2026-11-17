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

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

func writeValidateConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"rule": {"enforcementLevel": "mandatory"}}`), 0o600))
	return path
}

func TestPolicyValidate_JSON(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:      func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		ValidateF: func(context.Context, backend.PolicyPackOperation) error { return nil },
	}
	out, err := runPolicyPackCmd(t,
		newPolicyValidateCmd(newTestWsAndLm(newTestBackend(pack))),
		"acme/p", "1.0.0", "--config", writeValidateConfig(t), "--output", "json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, map[string]any{
		"organizationName": "acme",
		"name":             "p",
		"versionTag":       "1.0.0",
		"valid":            true,
	}, got)
}

func TestPolicyValidate_DefaultMessage(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:      func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		ValidateF: func(context.Context, backend.PolicyPackOperation) error { return nil },
	}
	out, err := runPolicyPackCmd(t,
		newPolicyValidateCmd(newTestWsAndLm(newTestBackend(pack))),
		"acme/p", "1.0.0", "--config", writeValidateConfig(t))
	require.NoError(t, err)
	assert.Equal(t, "Policy pack acme/p version 1.0.0 configuration is valid.\n", out)
}

func TestPolicyValidate_InvalidStillErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("bad config")
	pack := &backend.MockPolicyPack{
		RefF:      func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		ValidateF: func(context.Context, backend.PolicyPackOperation) error { return wantErr },
	}
	out, err := runPolicyPackCmdSplit(t,
		newPolicyValidateCmd(newTestWsAndLm(newTestBackend(pack))),
		"acme/p", "1.0.0", "--config", writeValidateConfig(t), "--output", "json")
	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, out)
}
