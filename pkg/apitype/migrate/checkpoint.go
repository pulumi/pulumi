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
	"github.com/pulumi/pulumi/pkg/resource/config"
)

// UpToCheckpointV2 migrates a CheckpointV1 to a CheckpointV2.
func UpToCheckpointV2(v1 apitype.CheckpointV1) apitype.CheckpointV2 {
	var v2 apitype.CheckpointV2
	v2.Stack = v1.Stack
	v2.Config = make(config.Map)
	for key, value := range v1.Config {
		v2.Config[key] = value
	}

	var v2deploy *apitype.DeploymentV2
	if v1.Latest != nil {
		deploy := UpToDeploymentV2(*v1.Latest)
		v2deploy = &deploy
	}
	v2.Latest = v2deploy
	return v2
}
