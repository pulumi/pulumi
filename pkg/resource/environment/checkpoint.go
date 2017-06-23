// Copyright 2016-2017, Pulumi Corporation
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

// Package environment contains the serialized and configurable state associated with an environment; or, in other
// words, a deployment target.  It pertains to resources and deployment plans, but is a package unto itself.
package environment

import (
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/deploy"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
type Checkpoint struct {
	Target tokens.QName        `json:"target"`           // the target environment name.
	Config *resource.ConfigMap `json:"config,omitempty"` // optional configuration key/values.
	Latest *Deployment         `json:"latest,omitempty"` // the latest/current deployment information.
}

// SerializeCheckpoint turns a snapshot into a LumiGL data structure suitable for serialization.
func SerializeCheckpoint(targ *deploy.Target, snap *deploy.Snapshot) *Checkpoint {
	contract.Requiref(targ != nil, "targ", "!= nil")

	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *Deployment
	if snap != nil {
		latest = SerializeDeployment(snap)
	}

	var config *resource.ConfigMap
	if targ.Config != nil {
		config = &targ.Config
	}

	return &Checkpoint{
		Target: targ.Name,
		Config: config,
		Latest: latest,
	}
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot.
func DeserializeCheckpoint(chkpoint *Checkpoint) (*deploy.Target, *deploy.Snapshot) {
	contract.Require(chkpoint != nil, "chkpoint")

	var snap *deploy.Snapshot
	name := chkpoint.Target
	if latest := chkpoint.Latest; latest != nil {
		// For every serialized resource vertex, create a ResourceDeployment out of it.
		var resources []*resource.State
		if latest.Resources != nil {
			for _, kvp := range latest.Resources.Iter() {
				// Deserialize the resource properties, if they exist.
				res := kvp.Value
				inputs := DeserializeProperties(res.Inputs)
				outputs := DeserializeProperties(res.Outputs)

				// And now just produce a resource object using the information available.
				state := resource.NewState(res.Type, kvp.Key, res.ID, inputs, outputs)
				resources = append(resources, state)
			}
		}

		snap = deploy.NewSnapshot(name, resources, latest.Info)
	}

	// Create a new target and snapshot objects to return.
	targ := &deploy.Target{Name: name}
	if chkpoint.Config != nil {
		targ.Config = *chkpoint.Config
	}
	return targ, snap
}
