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

var aliasTestCases = []struct {
	name        string
	alias       func(t *testing.T) Alias
	expectedURN string
}{
	{
		"plain",
		func(*testing.T) Alias {
			return Alias{
				Type: String("kubernetes:storage.k8s.io/v1beta1:CSIDriver"),
			}
		},
		"AnUrn$kubernetes:storage.k8s.io/v1beta1:CSIDriver::defName",
	},
	{
		"noParent",
		func(*testing.T) Alias {
			return Alias{
				Type:     String("kubernetes:storage.k8s.io/v1beta1:CSIDriver"),
				NoParent: Bool(true),
			}
		}, "urn:pulumi:defStack::defProject::kubernetes:storage.k8s.io/v1beta1:CSIDriver::defName",
	},
	{
		"parent",
		func(t *testing.T) Alias {
			return Alias{
				Type:   String("kubernetes:storage.k8s.io/v1beta1:CSIDriver"),
				Parent: newResource(t, URN("AParent::AParent"), ID("theParent")),
			}
		}, "AParent$kubernetes:storage.k8s.io/v1beta1:CSIDriver::defName",
	},
	{
		"parentURN",
		func(*testing.T) Alias {
			return Alias{
				Type:      String("kubernetes:storage.k8s.io/v1beta1:CSIDriver"),
				ParentURN: URN("AParent::AParent"),
			}
		}, "AParent$kubernetes:storage.k8s.io/v1beta1:CSIDriver::defName",
	},
}

func TestAliasResolution(t *testing.T) {
	t.Parallel()

	for _, tt := range aliasTestCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parent := newResource(t, URN("AnUrn::ASegment"), ID("hello"))
			out, err := tt.alias(t).collapseToURN("defName", "defType", parent, "defProject", "defStack")
			assert.NoError(t, err)
			urn, _, _, err := out.awaitURN(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, URN(tt.expectedURN), urn)
		})
	}
}

func newResource(t *testing.T, urn URN, id ID) Resource {
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)
	return newSimpleCustomResource(ctx, urn, id)
}
