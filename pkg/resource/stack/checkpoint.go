// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package stack contains the serialized and configurable state associated with an stack; or, in other
// words, a deployment target.  It pertains to resources and deployment plans, but is a package unto itself.
package stack

import (
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
type Checkpoint struct {
	Target tokens.QName                   `json:"target"`           // the target stack name.
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
		for _, res := range latest.Resources {
			resources = append(resources, DeserializeResource(res))
		}

		snap = deploy.NewSnapshot(name, chkpoint.Latest.Time, resources, latest.Info)
	}

	// Create a new target and snapshot objects to return.
	return &deploy.Target{
		Name:   name,
		Config: chkpoint.Config,
	}, snap
}
