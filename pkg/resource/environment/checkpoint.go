// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package environment contains the serialized and configurable state associated with an environment; or, in other
// words, a deployment target.  It pertains to resources and deployment plans, but is a package unto itself.
package environment

import (
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
type Checkpoint struct {
	Target tokens.QName                   `json:"target"`           // the target environment name.
	Config map[tokens.ModuleMember]string `json:"config,omitempty"` // optional configuration key/values.
	Latest *Deployment                    `json:"latest,omitempty"` // the latest/current deployment information.
}

// SerializeCheckpoint turns a snapshot into a LumiGL data structure suitable for serialization.
func SerializeCheckpoint(targ *deploy.Target, snap *deploy.Snapshot) *Checkpoint {
	contract.Requiref(targ != nil, "targ", "!= nil")

	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *Deployment
	if snap != nil {
		latest = SerializeDeployment(snap)
	}

	return &Checkpoint{
		Target: targ.Name,
		Config: targ.Config,
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
				defaults := DeserializeProperties(res.Defaults)
				outputs := DeserializeProperties(res.Outputs)

				var children []resource.URN
				for _, child := range res.Children {
					children = append(children, resource.URN(child))
				}

				// And now just produce a resource object using the information available.
				resources = append(resources,
					resource.NewState(res.Type, kvp.Key, res.Custom, res.ID,
						inputs, defaults, outputs, children))
			}
		}

		snap = deploy.NewSnapshot(name, chkpoint.Latest.Time, resources, latest.Info)
	}

	// Create a new target and snapshot objects to return.
	return &deploy.Target{
		Name:   name,
		Config: chkpoint.Config,
	}, snap
}
