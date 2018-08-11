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
	"github.com/pulumi/pulumi/pkg/apitype"
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
