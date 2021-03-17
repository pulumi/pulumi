// Copyright 2016-2018, Pulumi Corporation.
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

package migrate

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentV1ToV2(t *testing.T) {
	v1 := apitype.DeploymentV1{
		Manifest: apitype.ManifestV1{},
		Resources: []apitype.ResourceV1{
			{
				URN: resource.URN("a"),
			},
			{
				URN: resource.URN("b"),
			},
		},
	}

	v2 := UpToDeploymentV2(v1)
	assert.Equal(t, v1.Manifest, v2.Manifest)
	assert.Len(t, v1.Resources, 2)
	assert.Equal(t, resource.URN("a"), v1.Resources[0].URN)
	assert.Equal(t, resource.URN("b"), v1.Resources[1].URN)
}
