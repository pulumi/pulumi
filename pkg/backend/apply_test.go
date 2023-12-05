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

package backend

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

// TestComputeUpdateStats tests that the number of non-stack resources and the number of retained resources is correct.
func TestComputeUpdateStats(t *testing.T) {
	t.Parallel()

	events := []engine.Event{
		makeResourcePreEvent("res1", "pulumi:pulumi:Stack", deploy.OpCreate, false),
		makeResourcePreEvent("res2", "custom:resource:Type", deploy.OpCreate, false),
		makeResourcePreEvent("res3", "custom:resource:Type", deploy.OpDelete, true),
		makeResourcePreEvent("res4", "custom:resource:Type", deploy.OpReplace, true),
		makeResourcePreEvent("res5", "custom:resource:Type", deploy.OpSame, false),
	}

	stats := computeUpdateStats(events)

	assert.Equal(t, 4, stats.numNonStackResources)
	assert.Equal(t, 2, len(stats.retainedResources))
}

func makeResourcePreEvent(urn, resType string, op display.StepOp, retainOnDelete bool) engine.Event {
	event := engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op:   op,
			URN:  resource.URN(urn),
			Type: tokens.Type(resType),
			Old: &engine.StepEventStateMetadata{
				State: &resource.State{
					URN:            resource.URN(urn),
					Type:           tokens.Type(resType),
					RetainOnDelete: retainOnDelete,
				},
			},
		},
	})

	return event
}
