// Copyright 2016-2024, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/pulumi/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigEnvAddCmd(t *testing.T) {
	t.Parallel()

	projectYAML := `name: test
runtime: yaml`

	t.Run("no imports", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(stdin, &stdout, projectYAML, "", env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent}
		ctx := context.Background()
		err := add.run(ctx, []string{"env"})
		require.NoError(t, err)

		const expectedOut = `KEY         VALUE
aws:region  us-west-2

Save? Yes
`

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("no imports, yes", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{
			Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					"aws:region": esc.NewValue("us-west-2"),
				}),
			},
		}

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(stdin, &stdout, projectYAML, "", env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent, yes: true}
		ctx := context.Background()
		err := add.run(ctx, []string{"env"})
		require.NoError(t, err)

		const expectedOut = `KEY         VALUE
aws:region  us-west-2
`

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("no effects", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{}

		var newStackYAML string
		stdin := strings.NewReader("n")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(stdin, &stdout, projectYAML, "", env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent}
		ctx := context.Background()
		err := add.run(ctx, []string{"env"})
		require.Error(t, err)

		const expectedOut = "KEY  VALUE\n" +
			"The stack's environment does not define the `environmentVariables`, `files`, or `pulumiConfig` properties.\n" +
			"Without at least one of these properties, the environment will not affect the stack's behavior.\n\n\n" +
			"Save? No\n"

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		assert.Equal(t, "", newStackYAML)
	})

	t.Run("no effects, yes", func(t *testing.T) {
		t.Parallel()

		env := &esc.Environment{}

		var newStackYAML string
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(nil, &stdout, projectYAML, "", env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent, yes: true}
		ctx := context.Background()
		err := add.run(ctx, []string{"env"})
		require.NoError(t, err)

		const expectedOut = "KEY  VALUE\n" +
			"The stack's environment does not define the `environmentVariables`, `files`, or `pulumiConfig` properties.\n" +
			"Without at least one of these properties, the environment will not affect the stack's behavior.\n\n"

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("one import, secrets", func(t *testing.T) {
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
`

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(stdin, &stdout, projectYAML, stackYAML, env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent}
		ctx := context.Background()
		err := add.run(ctx, []string{"env2"})
		require.NoError(t, err)

		const expectedOut = `KEY           VALUE
app:password  [secret]
aws:region    us-west-2

Save? Yes
`

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
  - env2
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("one import, secrets", func(t *testing.T) {
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
`

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForTest(stdin, &stdout, projectYAML, stackYAML, env, nil, &newStackYAML)
		add := &configEnvAddCmd{parent: parent, showSecrets: true}
		ctx := context.Background()
		err := add.run(ctx, []string{"env2"})
		require.NoError(t, err)

		const expectedOut = `KEY           VALUE
app:password  hunter2
aws:region    us-west-2

Save? Yes
`

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
  - env2
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})
}
