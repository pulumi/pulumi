// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Environment represents information about a deployment target.
type Environment struct {
	Name   tokens.QName       // the target environment name.
	Config resource.ConfigMap // optional configuration key/values.
}

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
type Checkpoint struct {
	Target tokens.QName        `json:"target"`           // the target environment name.
	Config *resource.ConfigMap `json:"config,omitempty"` // optional configuration key/values.
	Latest *Deployment         `json:"latest,omitempty"` // the latest/current deployment information.
}

// SerializeCheckpoint turns a snapshot into a LumiGL data structure suitable for serialization.
func SerializeCheckpoint(env *Environment, snap *deployment.Snapshot, reftag string) *Checkpoint {
	contract.Requiref(env != nil, "env", "!= nil")

	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *Deployment
	if snap != nil {
		latest = SerializeDeployment(snap, reftag)
	}

	var config *resource.ConfigMap
	if env.Config != nil {
		config = &env.Config
	}

	return &Checkpoint{
		Target: env.Name,
		Config: config,
		Latest: latest,
	}
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot.
func DeserializeCheckpoint(ctx *Context, envfile *Checkpoint) (*Environment, *Snapshot) {
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
				// Deserialize the resource properties, if they exist.
				res := kvp.Value
				inputs := DeserializeDeploymentProperties(res.Inputs, reftag)
				outputs := DeserializeDeploymentProperties(res.Outputs, reftag)

				// And now just produce a resource object using the information available.
				resources = append(resources, NewResource(res.ID, kvp.Key, res.Type, inputs, outputs))
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
	env := &Environment{Name: name}
	if envfile.Config != nil {
		env.Config = *envfile.Config
	}
	return env, snap
}
