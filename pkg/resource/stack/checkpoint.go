// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package stack contains the serialized and configurable state associated with an stack; or, in other
// words, a deployment target.  It pertains to resources and deployment plans, but is a package unto itself.
package stack

import (
	"encoding/json"
	"io/ioutil"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// GetCheckpoint loads a checkpoint file for the given stack in this project, from the current project workspace.
func GetCheckpoint(w workspace.W, stack tokens.QName) (*apitype.CheckpointV1, error) {
	chkpath := w.StackPath(stack)
	bytes, err := ioutil.ReadFile(chkpath)
	if err != nil {
		return nil, err
	}

	return unmarshalVersionedCheckpointToLatestCheckpoint(bytes)
}

func unmarshalVersionedCheckpointToLatestCheckpoint(bytes []byte) (*apitype.CheckpointV1, error) {
	var versionedCheckpoint apitype.VersionedCheckpoint
	if err := json.Unmarshal(bytes, &versionedCheckpoint); err != nil {
		return nil, err
	}

	switch versionedCheckpoint.Version {
	case 0:
		// The happens when we are loading a checkpoint file from before we started to version things. Go's
		// json package did not support strict marshalling before 1.10, and we use 1.9 in our toolchain today.
		// After we upgrade, we could consider rewriting this code to use DisallowUnknownFields() on the decoder
		// to have the old checkpoint not even deserialize as an apitype.VersionedCheckpoint.
		var checkpoint apitype.CheckpointV1
		if err := json.Unmarshal(bytes, &checkpoint); err != nil {
			return nil, err
		}
		return &checkpoint, nil
	case 1:
		var checkpoint apitype.CheckpointV1
		if err := json.Unmarshal(versionedCheckpoint.Checkpoint, &checkpoint); err != nil {
			return nil, err
		}
		return &checkpoint, nil
	default:
		return nil, errors.Errorf("unsupported checkpoint version %d", versionedCheckpoint.Version)
	}
}

// SerializeCheckpoint turns a snapshot into a data structure suitable for serialization.
func SerializeCheckpoint(stack tokens.QName, config config.Map, snap *deploy.Snapshot) *apitype.VersionedCheckpoint {
	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *apitype.Deployment
	if snap != nil {
		latest = SerializeDeployment(snap)
	}

	b, err := json.Marshal(apitype.CheckpointV1{
		Stack:  stack,
		Config: config,
		Latest: latest,
	})
	contract.AssertNoError(err)

	return &apitype.VersionedCheckpoint{
		Version:    1,
		Checkpoint: json.RawMessage(b),
	}
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot.
func DeserializeCheckpoint(chkpoint *apitype.CheckpointV1) (*deploy.Snapshot, error) {
	contract.Require(chkpoint != nil, "chkpoint")

	var snap *deploy.Snapshot
	stack := chkpoint.Stack
	if latest := chkpoint.Latest; latest != nil {
		// Unpack the versions.
		manifest := deploy.Manifest{
			Time:    latest.Manifest.Time,
			Magic:   latest.Manifest.Magic,
			Version: latest.Manifest.Version,
		}
		for _, plug := range latest.Manifest.Plugins {
			var version *semver.Version
			if v := plug.Version; v != "" {
				sv, err := semver.ParseTolerant(v)
				if err != nil {
					return nil, err
				}
				version = &sv
			}
			manifest.Plugins = append(manifest.Plugins, workspace.PluginInfo{
				Name:    plug.Name,
				Kind:    plug.Type,
				Version: version,
			})
		}

		// For every serialized resource vertex, create a ResourceDeployment out of it.
		var resources []*resource.State
		for _, res := range latest.Resources {
			desres, err := DeserializeResource(res)
			if err != nil {
				return nil, err
			}
			resources = append(resources, desres)
		}

		snap = deploy.NewSnapshot(stack, manifest, resources)
	}

	return snap, nil
}

// GetRootStackResource returns the root stack resource from a given snapshot, or nil if not found.  If the stack
// exists, its output properties, if any, are also returned in the resulting map.
func GetRootStackResource(snap *deploy.Snapshot) (*resource.State, map[string]interface{}) {
	if snap != nil {
		for _, res := range snap.Resources {
			if res.Type == resource.RootStackType {
				return res, SerializeResource(res).Outputs
			}
		}
	}
	return nil, nil
}
