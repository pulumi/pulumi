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

package pulumi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAliasResolution(t *testing.T) {
	k8Alias := Alias{
		Type: String("kubernetes:storage.k8s.io/v1beta1:CSIDriver"),
	}
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)
	parent := newSimpleCustomResource(ctx, URN("AnUrn::ASegment"), ID("hello"))
	out, err := k8Alias.collapseToURN("defName", "defType", parent, "defProject", "defStack")
	assert.NoError(t, err)
	urn, _, _, err := out.awaitURN(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, URN("AnUrn$kubernetes:storage.k8s.io/v1beta1:CSIDriver::defName"), urn)
}
