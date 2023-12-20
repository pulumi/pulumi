// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/pulumi/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestConfigEnvLsCmd(t *testing.T) {
	t.Parallel()

	projectYAML := `name: test
runtime: yaml`

	t.Run("no imports", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, "", env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(false)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = "This stack configuration has no environments listed. " +
			"Try adding one with `pulumi config env add [envName]`.\n"

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})

	t.Run("no imports, json", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, "", env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(true)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = "[]\n"

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})

	t.Run("with imports", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

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

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, stackYAML, env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(false)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = `ENVIRONMENTS
env
otherEnv
thirdEnv
`

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})

	t.Run("with imports, json", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

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

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, stackYAML, env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(true)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = `[
  "env",
  "otherEnv",
  "thirdEnv"
]
`

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})

	t.Run("with imports", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

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

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, stackYAML, env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(false)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = `ENVIRONMENTS
env
otherEnv
thirdEnv
`

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})

	t.Run("repeated imports", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

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

		stdin := strings.NewReader("")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(ctx, stdin, &stdout, projectYAML, stackYAML, env, nil, nil)
		ls := &configEnvLsCmd{parent: parent, jsonOut: boolPtr(true)}
		err := ls.run(nil, nil)
		require.NoError(t, err)

		const expectedOut = `[
  "env",
  "otherEnv",
  "env",
  "thirdEnv"
]
`

		assert.Equal(t, expectedOut, cleanStdout(stdout.String()))
	})
}
