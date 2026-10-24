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

package pulumiconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func parse(t *testing.T, text string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(text), &doc))
	return &doc
}

func TestValueNodeObject(t *testing.T) {
	t.Parallel()

	key := config.MustMakeKey("proj", "a")

	node, err := ValueNode(key, config.NewObjectValue(`{"b":["${x}",1]}`), false)
	require.NoError(t, err)
	out, err := yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "b:\n    - $${x}\n    - 1\n", string(out))

	_, err = ValueNode(key, config.NewObjectValue(`{"fn::toJSON":"x"}`), false)
	require.ErrorContains(t, err, "keys starting with fn:: are reserved in environments")

	_, err = ValueNode(key, config.NewObjectValue(`{"b":{"fn::open":"x"}}`), false)
	require.ErrorContains(t, err, "keys starting with fn:: are reserved in environments")

	_, err = ValueNode(key, config.NewObjectValue(`{"Fn::GetAtt":["a","b"]}`), false)
	require.ErrorIs(t, err, errReservedName)

	_, err = ValueNode(key, config.NewSecureObjectValue(`{"b":"x"}`), false)
	require.ErrorContains(t, err, "secret object values are not supported")

	for _, object := range []string{`{"secure":"x"}`, `{"b":{"secure":"x"}}`, `{"b":[{"secure":"x"}]}`} {
		_, err = ValueNode(key, config.NewObjectValue(object), false)
		require.ErrorIs(t, err, errSecureReprReserved, object)
	}
	_, err = ValueNode(config.MustMakeKey("proj", "a.b"), config.NewObjectValue(`{"secure":"x"}`), true)
	require.ErrorIs(t, err, errSecureReprReserved)

	node, err = ValueNode(key, config.NewObjectValue(`{"b":{"secure":1},"c":{"secure":"x","d":1}}`), false)
	require.NoError(t, err)
	out, err = yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "b:\n    secure: 1\nc:\n    d: 1\n    secure: x\n", string(out))
}

func TestSecretNode(t *testing.T) {
	t.Parallel()

	node := secretNode("abc")
	out, err := yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "fn::secret:\n    ciphertext: abc\n", string(out))

	viaValue, err := ValueNode(config.MustMakeKey("proj", "a"), config.NewSecureValue("abc"), false)
	require.NoError(t, err)
	assert.Equal(t, node, viaValue)
}

func TestImportsDefinition(t *testing.T) {
	t.Parallel()

	out, ok := ImportsDefinition(parse(t, "imports:\n  - a/b\n  - c\nvalues:\n  pulumiConfig:\n    x: 1\n"))
	require.True(t, ok)
	assert.Equal(t, "imports:\n    - a/b\n    - c\n", string(out))

	_, ok = ImportsDefinition(parse(t, "values:\n  pulumiConfig:\n    x: 1\n"))
	assert.False(t, ok)

	_, ok = ImportsDefinition(parse(t, "imports: []\n"))
	assert.False(t, ok)
}
