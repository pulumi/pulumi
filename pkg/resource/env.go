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

package resource

import (
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
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

// SerializeEnvfile turns a snapshot into a LumiGL data structure suitable for serialization.
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

// DeserializeEnvfile takes a serialized deployment record and returns its associated snapshot.
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
				resobj := NewResource(id, kvp.Key, res.Type, props)

				// Mark any inferred properties so we know how and when to diff them appropriately.
				if res.Outputs != nil {
					for _, k := range *res.Outputs {
						resobj.MarkOutput(PropertyKey(k))
					}
				}

				resources = append(resources, resobj)
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
