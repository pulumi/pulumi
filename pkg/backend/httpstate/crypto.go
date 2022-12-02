// Copyright 2016-2019, Pulumi Corporation.
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

package httpstate

import (
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewServiceSecretsManager(s Stack, stackName tokens.Name, configFile string) (secrets.Manager, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")

	project, _, err := workspace.DetectProjectStackPath(stackName.Q())
	if err != nil {
		return nil, err
	}

	info, err := workspace.LoadProjectStack(project, configFile)
	if err != nil {
		return nil, err
	}

	client := s.Backend().(Backend).Client()
	id := s.StackIdentifier()

	info.SecretsProvider = workspace.SecretsProvider{}
	info.EncryptedKey = ""
	info.EncryptionSalt = ""
	if err := workspace.SaveProjectStack(stackName.Q(), info); err != nil {
		return nil, err
	}

	return service.NewServiceSecretsManager(client, id)
}
