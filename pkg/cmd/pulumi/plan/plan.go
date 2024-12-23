// Copyright 2024, Pulumi Corporation.
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

package plan

import (
	"encoding/json"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func Write(path string, plan *deploy.Plan, enc config.Encrypter, showSecrets bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(f)

	deploymentPlan, err := stack.SerializePlan(plan, enc, showSecrets)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	return encoder.Encode(deploymentPlan)
}

func Read(path string, dec config.Decrypter, enc config.Encrypter) (*deploy.Plan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(f)

	var deploymentPlan apitype.DeploymentPlanV1
	if err := json.NewDecoder(f).Decode(&deploymentPlan); err != nil {
		return nil, err
	}
	return stack.DeserializePlan(deploymentPlan, dec, enc)
}
