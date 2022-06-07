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

import "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

// UpToDeploymentV2 migrates a deployment from DeploymentV1 to DeploymentV2.
func UpToDeploymentV2(v1 apitype.DeploymentV1) apitype.DeploymentV2 {
	var v2 apitype.DeploymentV2
	// The manifest format did not change between V1 and V2.
	v2.Manifest = v1.Manifest
	for _, res := range v1.Resources {
		v2.Resources = append(v2.Resources, UpToResourceV2(res))
	}

	return v2
}

// UpToDeploymentV3 migrates a deployment from DeploymentV2 to DeploymentV3.
func UpToDeploymentV3(v2 apitype.DeploymentV2) apitype.DeploymentV3 {
	var v3 apitype.DeploymentV3
	// The manifest format did not change between V2 and V3.
	v3.Manifest = v2.Manifest
	for _, res := range v2.Resources {
		v3.Resources = append(v3.Resources, UpToResourceV3(res))
	}
	for _, op := range v2.PendingOperations {
		v3.PendingOperations = append(v3.PendingOperations, UpToOperationV2(op))
	}

	return v3
}
