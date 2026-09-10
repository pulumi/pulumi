// Copyright 2016, Pulumi Corporation.
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
	"io"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configEnvLsProjectYAML = `name: test
runtime: yaml`

func runConfigEnvLs(t *testing.T, stackYAML string, env *esc.Environment, args ...string) string {
	var stdout bytes.Buffer
	parent := newConfigEnvCmdForTest(
		strings.NewReader(""), &stdout, configEnvLsProjectYAML, stackYAML, env, nil, nil)
	cmd := newConfigEnvListCmd(parent)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	return cleanStdout(stdout.String())
}

func TestConfigEnvLsCmd(t *testing.T) {
	t.Parallel()

	t.Run("no imports", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		const expectedOut = "This stack configuration has no environments listed. " +
			"Try adding one with `pulumi config env add <projectName>/<envName>`.\n"

		assert.Equal(t, expectedOut, runConfigEnvLs(t, "", env))
	})

	t.Run("no imports, json", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		const expectedOut = "[]\n"

		assert.Equal(t, expectedOut, runConfigEnvLs(t, "", env, "--output", "json"))
	})

	t.Run("with imports", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region":   esc.NewValue("us-west-2"),
					"app:password": esc.NewSecret("hunter2"),
				}),
			},
		}

		const stackYAML = `environment:
    - env
    - otherEnv
    - thirdEnv
`

		const expectedOut = `ENVIRONMENTS
env
otherEnv
thirdEnv
`

		assert.Equal(t, expectedOut, runConfigEnvLs(t, stackYAML, env))
	})

	t.Run("with imports, json", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region":   esc.NewValue("us-west-2"),
					"app:password": esc.NewSecret("hunter2"),
				}),
			},
		}

		const stackYAML = `environment:
    - env
    - otherEnv
    - thirdEnv
`

		const expectedOut = `[
  "env",
  "otherEnv",
  "thirdEnv"
]
`

		assert.Equal(t, expectedOut, runConfigEnvLs(t, stackYAML, env, "--output", "json"))
	})

	t.Run("with imports", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region":   esc.NewValue("us-west-2"),
					"app:password": esc.NewSecret("hunter2"),
				}),
			},
		}

		const stackYAML = `environment:
    - env
    - otherEnv
    - thirdEnv
`

		const expectedOut = `ENVIRONMENTS
env
otherEnv
thirdEnv
`

		assert.Equal(t, expectedOut, runConfigEnvLs(t, stackYAML, env))
	})

	t.Run("repeated imports", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region":   esc.NewValue("us-west-2"),
					"app:password": esc.NewSecret("hunter2"),
				}),
			},
		}

		const stackYAML = `environment:
    - env
    - otherEnv
    - env
    - thirdEnv
`

		const expectedOut = `[
  "env",
  "otherEnv",
  "env",
  "thirdEnv"
]
`

		assert.Equal(t, expectedOut, runConfigEnvLs(t, stackYAML, env, "--output", "json"))
	})
}
