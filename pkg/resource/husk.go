// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Husk represents information about a deployment target.
type Husk struct {
	Name   tokens.QName // the target environment name.
	Config ConfigMap    // optional configuration key/values.
}

// Huskfile is a serialized deployment target plus a record of the latest deployment.
type Huskfile struct {
	Husk   tokens.QName      `json:"husk"`             // the target environment name.
	Config *ConfigMap        `json:"config,omitempty"` // optional configuration key/values.
	Latest *DeploymentRecord `json:"latest,omitempty"` // the latest/current deployment record.
}

// SerializeHuskfile turns a snapshot into a CocoGL data structure suitable for serialization.
func SerializeHuskfile(husk *Husk, snap Snapshot, reftag string) *Huskfile {
	contract.Requiref(husk != nil, "husk", "!= nil")

	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *DeploymentRecord
	if snap != nil {
		latest = serializeDeploymentRecord(snap, reftag)
	}

	var config *ConfigMap
	if husk.Config != nil {
		config = &husk.Config
	}

	return &Huskfile{
		Husk:   husk.Name,
		Config: config,
		Latest: latest,
	}
}

// DeserializeDeployment takes a serialized deployment record and returns its associated snapshot.
func DeserializeHuskfile(ctx *Context, huskfile *Huskfile) (*Husk, Snapshot) {
	contract.Require(ctx != nil, "ctx")
	contract.Require(huskfile != nil, "huskfile")

	var snap Snapshot
	name := huskfile.Husk
	if latest := huskfile.Latest; latest != nil {
		// Determine the reftag to use.
		var reftag string
		if latest.Reftag == nil {
			reftag = DefaultDeploymentReftag
		} else {
			reftag = *latest.Reftag
		}

		// For every serialized resource vertex, create a ResourceDeployment out of it.
		var resources []Resource
		if latest.Resources != nil {
			for _, kvp := range latest.Resources.Iter() {
				// Deserialize the resources, if they exist.
				res := kvp.Value
				var props PropertyMap
				if res.Properties == nil {
					props = make(PropertyMap)
				} else {
					props = deserializeProperties(*res.Properties, reftag)
				}

				// And now just produce a resource object using the information available.
				var id ID
				if res.ID != nil {
					id = *res.ID
				}

				resources = append(resources, NewResource(id, kvp.Key, res.Type, props))
			}
		}

		// If the args are non-nil, use them.
		var args core.Args
		if latest.Args != nil {
			args = *latest.Args
		}

		snap = NewSnapshot(ctx, name, latest.Package, args, resources)
	}

	// Create husk and snapshot objects to return.
	husk := &Husk{Name: name}
	if huskfile.Config != nil {
		husk.Config = *huskfile.Config
	}
	return husk, snap
}
