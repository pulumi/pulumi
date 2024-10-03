// Copyright 2016-2024, Pulumi Corporation.
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

//go:build go1.20

package client

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func marshalDeployment(d *apitype.DeploymentV3) (json.RawMessage, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return json.Marshal(apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(raw),
	})
}

func marshalVerbatimCheckpointRequest(req apitype.PatchUpdateVerbatimCheckpointRequest) (json.RawMessage, error) {
	return json.Marshal(req)
}
