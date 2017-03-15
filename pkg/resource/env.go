// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Env represents information about a deployment target.
type Env struct {
	Name   tokens.QName // the target environment name.
	Config ConfigMap    // optional configuration key/values.
}

// Envfile is a serialized deployment target plus a record of the latest deployment.
type Envfile struct {
	Target tokens.QName      `json:"target"`           // the target environment name.
	Config *ConfigMap        `json:"config,omitempty"` // optional configuration key/values.
	Latest *DeploymentRecord `json:"latest,omitempty"` // the latest/current deployment record.
}

// SerializeEnvfile turns a snapshot into a CocoGL data structure suitable for serialization.
func SerializeEnvfile(env *Env, snap Snapshot, reftag string) *Envfile {
	contract.Requiref(env != nil, "env", "!= nil")

	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *DeploymentRecord
	if snap != nil {
		latest = serializeDeploymentRecord(snap, reftag)
	}

	var config *ConfigMap
	if env.Config != nil {
		config = &env.Config
	}

	return &Envfile{
		Target: env.Name,
		Config: config,
		Latest: latest,
	}
}

// DeserializeDeployment takes a serialized deployment record and returns its associated snapshot.
func DeserializeEnvfile(ctx *Context, envfile *Envfile) (*Env, Snapshot) {
	contract.Require(ctx != nil, "ctx")
	contract.Require(envfile != nil, "envfile")

	var snap Snapshot
	name := envfile.Target
	if latest := envfile.Latest; latest != nil {
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

	// Create an environment and snapshot objects to return.
	env := &Env{Name: name}
	if envfile.Config != nil {
		env.Config = *envfile.Config
	}
	return env, snap
}
