// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package stack contains the serialized and configurable state associated with an stack; or, in other
// words, a deployment target.  It pertains to resources and deployment plans, but is a package unto itself.
package stack

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
// nolint: lll
type Checkpoint struct {
	Target tokens.QName                         `json:"target" yaml:"target"`                     // the target stack name.
	Config map[tokens.ModuleMember]config.Value `json:"config,omitempty" yaml:"config,omitempty"` // optional configuration key/values.
	Latest *Deployment                          `json:"latest,omitempty" yaml:"latest,omitempty"` // the latest/current deployment information.
}

// GetCheckpoint loads a checkpoint file for the given stack in this project, from the current project workspace.
func GetCheckpoint(w workspace.W, stack tokens.QName) (*Checkpoint, error) {
	chkpath := w.StackPath(stack)
	bytes, err := ioutil.ReadFile(chkpath)
	if err != nil {
		return nil, err
	}
	var checkpoint Checkpoint
	if err = json.Unmarshal(bytes, &checkpoint); err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

// SerializeCheckpoint turns a snapshot into a data structure suitable for serialization.
func SerializeCheckpoint(target tokens.QName,
	config map[tokens.ModuleMember]config.Value, snap *deploy.Snapshot) *Checkpoint {
	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *Deployment
	if snap != nil {
		latest = SerializeDeployment(snap)
	}

	return &Checkpoint{
		Target: target,
		Config: config,
		Latest: latest,
	}
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot.
func DeserializeCheckpoint(chkpoint *Checkpoint) (tokens.QName,
	map[tokens.ModuleMember]config.Value, *deploy.Snapshot, error) {
	contract.Require(chkpoint != nil, "chkpoint")

	var snap *deploy.Snapshot
	name := chkpoint.Target
	if latest := chkpoint.Latest; latest != nil {
		// For every serialized resource vertex, create a ResourceDeployment out of it.
		var resources []*resource.State
		for _, res := range latest.Resources {
			desres, err := DeserializeResource(res)
			if err != nil {
				return "", nil, nil, err
			}
			resources = append(resources, desres)
		}

		snap = deploy.NewSnapshot(name, chkpoint.Latest.Time, resources)
	}

	return name, chkpoint.Config, snap, nil
}
