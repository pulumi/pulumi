// Copyright 2016-2021, Pulumi Corporation.
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

//nolint:lll
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexops/autogold"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestProjectRoundtripComments(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	pulumiProject := `
# 🔴 header comment
name: pulumi-test
runtime: yaml
config:
  first-value:
    type: string
    default: first
  second-value:
    type: string
  third-value:
    type: array
    items:
      type: string
    default: [third] # 🟠 comment after array
# 🟡 comment before resources
resources:
  my-bucket:
            type: aws:s3:bucket
            # 🟢 comment before props, note the indentation is excessive, will change to 2 spaces
            properties:
              # 🔵 comment before prop
              bucket: test-123 # 🟣 comment after prop
# 🟥 footer comment
`

	integration.CreatePulumiRepo(e, pulumiProject)
	projFilename := filepath.Join(e.CWD, workspace.ProjectFile+".yaml")
	// TODO: Replace this with config set --project after implemented.
	proj, err := workspace.LoadProject(projFilename)
	require.NoError(t, err)
	ty := "string"
	proj.Config["new-value"] = workspace.ProjectConfigType{
		Type:        &ty,
		Description: "💜 a new value added to config, expect unicode to be escaped",
	}
	err = proj.Save(projFilename)
	require.NoError(t, err)

	projData, err := os.ReadFile(projFilename)
	require.NoError(t, err)

	// Project file:
	want := autogold.Want("roundtrip-project", `# 🔴 header comment
name: pulumi-test
runtime: yaml
config:
  first-value:
    type: string
    default: first
  second-value:
    type: string
  third-value:
    type: array
    items:
      type: string
    default: [third] # 🟠 comment after array
  new-value:
    type: string
    description: "\U0001F49C a new value added to config, expect unicode to be escaped"
# 🟡 comment before resources
resources:
  my-bucket:
    type: aws:s3:bucket
    # 🟢 comment before props, note the indentation is excessive, will change to 2 spaces
    properties:
      # 🔵 comment before prop
      bucket: test-123 # 🟣 comment after prop
# 🟥 footer comment
`)
	want.Equal(t, string(projData))
}

func TestConfigRoundtripComments(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	pulumiProject := `
name: foo
runtime: yaml
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "test")
	e.Passphrase = "TestConfigRoundtripComments"
	configFilename := filepath.Join(e.CWD, workspace.ProjectFile+".test.yaml")

	err := os.WriteFile(configFilename, []byte(`
encryptionsalt: v1:ThS5UPxP9qc=:v1:UZYAXF+ylaJ0rGhv:9OTvBnOEDFgxs7btjzSu+LZ470vLpg==
# 🔴 header comment
config:
  foo:a: some-value # 🟠 comment after value
  # 🟡 comment before key
  foo:b: some-value
  foo:c:
    a: A
    b: B
    c:
      - 1
      - 2
      - 3 # 🟢 comment in array
      # 🔵 comment after array
  foo:d:
    secure: v1:T1ftqhY0hqr+EJK6:+jvd5PMecFx80tcavzuZY4tLatgIfoe/xR72GA== # 🟣 comment on secret

# 🟥 footer comment`), 0o600)
	require.NoError(t, err)
	e.RunCommand("pulumi", "config", "set", "e", "E")
	e.RunCommand("pulumi", "config", "set", "--path", "c.c[2]", "three")

	projData, err := os.ReadFile(configFilename)
	require.NoError(t, err)

	// Project file:
	want := autogold.Want("roundtrip-config", `encryptionsalt: v1:ThS5UPxP9qc=:v1:UZYAXF+ylaJ0rGhv:9OTvBnOEDFgxs7btjzSu+LZ470vLpg==
# 🔴 header comment
config:
  foo:a: some-value # 🟠 comment after value
  # 🟡 comment before key
  foo:b: some-value
  foo:c:
    a: A
    b: B
    c:
      - 1
      - 2
      - three # 🟢 comment in array
      # 🔵 comment after array
  foo:d:
    secure: v1:T1ftqhY0hqr+EJK6:+jvd5PMecFx80tcavzuZY4tLatgIfoe/xR72GA== # 🟣 comment on secret
  foo:e: E

# 🟥 footer comment
`)
	want.Equal(t, string(projData))
}
