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
	"testing"

	"github.com/stretchr/testify/assert"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
	"github.com/pulumi/pulumi/sdk/v3/go/esc/workspace"
)

func TestGetEnvRef(t *testing.T) {
	defaultOrg := "default-org"
	account := workspace.Account{DefaultOrg: defaultOrg}
	esc := &escCommand{account: account}
	cmd := &envCommand{esc: esc}

	t.Run("1 identifier", func(t *testing.T) {
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
