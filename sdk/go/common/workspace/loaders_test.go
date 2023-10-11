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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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
