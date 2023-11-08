// Copyright 2016-2022, Pulumi Corporation.
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

package deploy

import (
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestRoundTripManifest(t *testing.T) {
	t.Parallel()
	ver := semver.MustParse("1.1.1")
	original := &Manifest{
		Plugins: []workspace.PluginInfo{
			{
				Name:    "foo-plug",
				Version: &ver,
			},
			{
				Name:    "bar-plug",
				Version: &ver,
			},
		},
	}
	s := original.Serialize()
	roundtripped, err := DeserializeManifest(s)
	assert.NoError(t, err)
	assert.Equal(t, original, roundtripped)
}
