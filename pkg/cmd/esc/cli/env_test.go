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

package cli

import (
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEnvRef(t *testing.T) {
	t.Parallel()
	defaultOrg := "default-org"
	account := Account{DefaultOrg: defaultOrg}
	esc := &escCommand{account: account}
	cmd := &envCommand{esc: esc}

	t.Run("1 identifier", func(t *testing.T) {
		t.Parallel()
		refString := "abc@v1"

		ref, isRelative := cmd.getEnvRef(refString, nil)

		assert.Equal(t, ref.orgName, defaultOrg)
		assert.Equal(t, ref.projectName, client.DefaultProject)
		assert.Equal(t, ref.envName, "abc")
		assert.Equal(t, ref.version, "v1")
		assert.Equal(t, ref.hasAmbiguousPath, false)
		assert.Equal(t, isRelative, false)
	})

	t.Run("2 identifiers", func(t *testing.T) {
		t.Parallel()
		refString := "a/b@v1"

		ref, isRelative := cmd.getEnvRef(refString, nil)

		assert.Equal(t, ref.orgName, defaultOrg)
		assert.Equal(t, ref.projectName, "a")
		assert.Equal(t, ref.envName, "b")
		assert.Equal(t, ref.version, "v1")
		assert.Equal(t, ref.hasAmbiguousPath, true)
		assert.Equal(t, isRelative, false)
	})

	t.Run("3 identifiers", func(t *testing.T) {
		t.Parallel()
		refString := "a/b/c@v1"

		ref, isRelative := cmd.getEnvRef(refString, nil)

		assert.Equal(t, ref.orgName, "a")
		assert.Equal(t, ref.projectName, "b")
		assert.Equal(t, ref.envName, "c")
		assert.Equal(t, ref.version, "v1")
		assert.Equal(t, ref.hasAmbiguousPath, false)
		assert.Equal(t, isRelative, false)
	})

	t.Run("with relative env", func(t *testing.T) {
		t.Parallel()
		refString := "@v1"
		rel := &environmentRef{
			orgName:     "rel-org",
			projectName: "rel-project",
			envName:     "rel-env",
			version:     "rel-version",
		}

		ref, isRelative := cmd.getEnvRef(refString, rel)

		assert.Equal(t, ref.orgName, "rel-org")
		assert.Equal(t, ref.projectName, "rel-project")
		assert.Equal(t, ref.envName, "rel-env")
		assert.Equal(t, ref.version, "v1")
		assert.Equal(t, isRelative, true)
	})
}

func TestDraftConflictError(t *testing.T) {
	t.Parallel()
	esc := &escCommand{command: "pulumi", client: &testPulumiClient{}}
	ref := environmentRef{orgName: "org", projectName: "project", envName: "env"}

	t.Run("draft conflict", func(t *testing.T) {
		t.Parallel()
		err := esc.draftConflictError(ref, &client.EnvironmentErrorResponse{
			Code: 400,
			Message: "Bad Request: a draft already exists for this environment (change request cr-id); " +
				"submit it for approval via POST /api/change-requests/org/cr-id/submit, or close it via " +
				"POST /api/change-requests/org/cr-id/close to clear the lock",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "org/project/env already has a draft (change request cr-id)")
		assert.Contains(t, err.Error(), "--draft=cr-id")
		assert.Contains(t, err.Error(), "pulumi api /api/change-requests/org/cr-id/submit -X POST --body '{}'")
		assert.Contains(t, err.Error(), "pulumi api /api/change-requests/org/cr-id/close -X POST --body '{}'")
		assert.NotContains(t, err.Error(), "via POST")
	})

	t.Run("unrelated environment error", func(t *testing.T) {
		t.Parallel()
		err := esc.draftConflictError(ref, &client.EnvironmentErrorResponse{
			Code:    409,
			Message: `"org" does not support approvals. An organization is required.`,
		})

		require.NoError(t, err)
	})

	t.Run("unrelated error", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, esc.draftConflictError(ref, errors.New("boom")))
	})
}
