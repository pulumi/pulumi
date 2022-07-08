// Copyright 2016-2022, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestRebuildBaseState(t *testing.T) {
	t.Parallel()
	a := &resource.State{URN: "A", Delete: true}
	b := &resource.State{URN: "B", Parent: a.URN}
	bNoParent := &resource.State{URN: "B"}
	previous := []*resource.State{a, b}
	resourcesToStep := map[*resource.State]Step{
		a: &RefreshStep{
			old: a,
			new: a,
		},
		b: &RefreshStep{
			old: b,
			new: b,
		},
	}
	expectedOlds := map[resource.URN]*resource.State{b.URN: bNoParent}
	ex := &deploymentExecutor{
		deployment: &Deployment{
			prev: &Snapshot{
				Resources: previous,
			},
		},
	}

	ex.rebuildBaseState(resourcesToStep, true)

	assert.EqualValues(t, expectedOlds, ex.deployment.olds)
}
