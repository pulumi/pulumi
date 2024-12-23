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

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type base64EvalCrypter struct{}

func newBase64EvalCrypter() (evalCrypter, error) {
	return base64EvalCrypter{}, nil
}

func (c base64EvalCrypter) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	ciphertext, err := config.Base64Crypter.EncryptValue(ctx, string(plaintext))
	if err != nil {
		return nil, err
	}
	return []byte(ciphertext), nil
}

func (c base64EvalCrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	plaintext, err := config.Base64Crypter.DecryptValue(ctx, string(ciphertext))
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}

func TestConfigEnvInit(t *testing.T) {
	t.Parallel()

	projectYAML := `name: test
runtime: yaml`

	t.Run("no config", func(t *testing.T) {
		t.Parallel()

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, "", &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, yes: true}
		ctx := context.Background()
		err := init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {}\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig: {}\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("some config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		plaintext := map[string]config.Plaintext{
			"aws:region":   config.NewPlaintext("us-west-2"),
			"app:password": config.NewSecurePlaintext("hunter2"),
			"app:tags": config.NewPlaintext(map[string]config.Plaintext{
				"env": config.NewPlaintext("testing"),
				"owners": config.NewPlaintext([]config.Plaintext{
					config.NewPlaintext("alice"),
					config.NewPlaintext("bob"),
				}),
			}),
		}
		cfg := make(config.Map)
		for k, v := range plaintext {
			cv, err := v.Encrypt(ctx, config.Base64Crypter)
			require.NoError(t, err)
			ns, name, _ := strings.Cut(k, ":")
			cfg[config.MustMakeKey(ns, name)] = cv
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"[secret]\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret:\n" +
			"        ciphertext: ZXNjeAAAAAFhSFZ1ZEdWeU1nPT2+gKwa\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("some config, show secrets", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		plaintext := map[string]config.Plaintext{
			"aws:region":   config.NewPlaintext("us-west-2"),
			"app:password": config.NewSecurePlaintext("hunter2"),
			"app:tags": config.NewPlaintext(map[string]config.Plaintext{
				"env": config.NewPlaintext("testing"),
				"owners": config.NewPlaintext([]config.Plaintext{
					config.NewPlaintext("alice"),
					config.NewPlaintext("bob"),
				}),
			}),
		}
		cfg := make(config.Map)
		for k, v := range plaintext {
			cv, err := v.Encrypt(ctx, config.Base64Crypter)
			require.NoError(t, err)
			ns, name, _ := strings.Cut(k, ":")
			cfg[config.MustMakeKey(ns, name)] = cv
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, showSecrets: true, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"hunter2\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret: hunter2\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("other env, some config, show secrets", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		plaintext := map[string]config.Plaintext{
			"aws:region":   config.NewPlaintext("us-west-2"),
			"app:password": config.NewSecurePlaintext("hunter2"),
			"app:tags": config.NewPlaintext(map[string]config.Plaintext{
				"env": config.NewPlaintext("testing"),
				"owners": config.NewPlaintext([]config.Plaintext{
					config.NewPlaintext("alice"),
					config.NewPlaintext("bob"),
				}),
			}),
		}
		cfg := make(config.Map)
		for k, v := range plaintext {
			cv, err := v.Encrypt(ctx, config.Base64Crypter)
			require.NoError(t, err)
			ns, name, _ := strings.Cut(k, ":")
			cfg[config.MustMakeKey(ns, name)] = cv
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{
			Environment: workspace.NewEnvironment([]string{"env"}),
			Config:      cfg,
		})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{
			"env": `{"values": {"pulumiConfig": {"app:tags": {"name": "project"}}}}`,
		})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, showSecrets: true, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"hunter2\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret: hunter2\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})
}
