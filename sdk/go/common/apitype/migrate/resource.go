// Copyright 2016-2018, Pulumi Corporation.
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

package migrate

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// UpToResourceV2 migrates a resource from ResourceV1 to ResourceV2.
func UpToResourceV2(v1 apitype.ResourceV1) apitype.ResourceV2 {
	var v2 apitype.ResourceV2
	v2.URN = v1.URN
	v2.Custom = v1.Custom
	v2.Delete = v1.Delete
	v2.ID = v1.ID
	v2.Type = v1.Type
	v2.Inputs = make(map[string]interface{})
	for key, value := range v1.Inputs {
		v2.Inputs[key] = value
	}
	// v1.Defaults was deprecated in v2.
	v2.Outputs = make(map[string]interface{})
	for key, value := range v1.Outputs {
		v2.Outputs[key] = value
	}
	v2.Parent = v1.Parent
	v2.Protect = v1.Protect
	// v2.External is a new field that, when true, indicates that this resource's
	// lifecycle is not owned by Pulumi. Since all V1 resources have their lifecycles
	// owned by Pulumi, this is `false` for all V1 resources.
	v2.External = false
	// v2.Provider is a reference to a first-class provider associated with this resource.
	v2.Provider = ""
	v2.Dependencies = append(v2.Dependencies, v1.Dependencies...)
	v2.InitErrors = append(v2.InitErrors, v1.InitErrors...)
	return v2
}

// UpToResourceV3 migrates a resource from ResourceV2 to ResourceV3.
func UpToResourceV3(v2 apitype.ResourceV2) apitype.ResourceV3 {
	var v3 apitype.ResourceV3
	v3.URN = v2.URN
	v3.Custom = v2.Custom
	v3.Delete = v2.Delete
	v3.ID = v2.ID
	v3.Type = v2.Type
	v3.Inputs = v2.Inputs
	v3.Outputs = v2.Outputs
	v3.Parent = v2.Parent
	v3.Protect = v2.Protect
	v3.External = v2.External
	v3.Dependencies = v2.Dependencies
	v3.InitErrors = v2.InitErrors
	v3.Provider = v2.Provider

	// v3.PropertyDependencies tracks dependencies on a per-input-property basis. We conservatively assume that all
	// properties depend on all of the resource's dependencies.
	propertyDependencies := make(map[resource.PropertyKey][]resource.URN)
	for pk := range v3.Inputs {
		propertyDependencies[resource.PropertyKey(pk)] = v3.Dependencies
	}
	v3.PropertyDependencies = propertyDependencies

	return v3
}
