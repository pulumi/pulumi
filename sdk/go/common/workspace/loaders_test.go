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

package workspace

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshallNil(t *testing.T) {
	t.Parallel()
	modifiedProjectStack := []byte(`
config:
  aws:region: us-east-1
  aws:id:
    -
`)

	marshaller, err := marshallerForPath(".yaml")
	require.NoError(t, err)
	var projectStack ProjectStack
	err = marshaller.Unmarshal(modifiedProjectStack, &projectStack)
	assert.NoError(t, err)

	_, err = projectStack.Config.Decrypt(config.Base64Crypter)
	assert.NoError(t, err)
}

func TestUnmarshalTime(t *testing.T) {
	t.Parallel()
	modifiedProjectStack := []byte(`
config:
  aws:region: us-east-1
  aws:time: 2020-01-01T00:00:00Z
`)
	marshaller, err := marshallerForPath(".yaml")
	require.NoError(t, err)
	var projectStack ProjectStack
	err = marshaller.Unmarshal(modifiedProjectStack, &projectStack)
	assert.NoError(t, err)

	_, err = projectStack.Config.Decrypt(config.Base64Crypter)
	assert.NoError(t, err)
}

func TestNoEmptyValueWarning(t *testing.T) {
	t.Parallel()
	b := []byte(`
config:
  project:a: a
`)
	marshaller, err := marshallerForPath(".yaml")
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	var p *Project
	projectStack, err := LoadProjectStackBytes(sink, p, b, "Pulumi.stack.yaml", marshaller)
	require.NoError(t, err)
	require.NotContains(t, stderr.String(), "warning: No value for configuration keys")
	require.Len(t, projectStack.Config, 1)
	require.Equal(t, projectStack.Config[config.MustMakeKey("project", "a")], config.NewValue("a"))
}

func TestEmptyValueWarning(t *testing.T) {
	t.Parallel()
	b := []byte(`
config:
  project:a:
  project:b: null
  project:c: ~
  project:d: ""
`)
	marshaller, err := marshallerForPath(".yaml")
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	var p *Project
	projectStack, err := LoadProjectStackBytes(sink, p, b, "Pulumi.stack.yaml", marshaller)
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "warning: No value for configuration keys")
	require.Contains(t, stderr.String(), "project:a")
	require.Contains(t, stderr.String(), "project:b")
	require.Contains(t, stderr.String(), "project:c")
	require.NotContains(t, stderr.String(), "project:d")
	require.Len(t, projectStack.Config, 4)
	require.Equal(t, projectStack.Config[config.MustMakeKey("project", "a")], config.NewValue(""))
	require.Equal(t, projectStack.Config[config.MustMakeKey("project", "b")], config.NewValue(""))
	require.Equal(t, projectStack.Config[config.MustMakeKey("project", "c")], config.NewValue(""))
	require.Equal(t, projectStack.Config[config.MustMakeKey("project", "d")], config.NewValue(""))
}
