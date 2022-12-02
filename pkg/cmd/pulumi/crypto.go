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

package main

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pluginSecrets "github.com/pulumi/pulumi/pkg/v3/secrets/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func getStackEncrypter(ctx context.Context, host plugin.Host, s backend.Stack) (config.Encrypter, error) {
	sm, err := getStackSecretsManager(ctx, host, s)
	if err != nil {
		return nil, err
	}

	return sm.Encrypter()
}

func getStackDecrypter(ctx context.Context, host plugin.Host, s backend.Stack) (config.Decrypter, error) {
	sm, err := getStackSecretsManager(ctx, host, s)
	if err != nil {
		return nil, err
	}

	return sm.Decrypter()
}

func getStackSecretsManager(ctx context.Context, host plugin.Host, s backend.Stack) (secrets.Manager, error) {
	project, _, err := readProject()

	if err != nil {
		return nil, err
	}

	ps, err := loadProjectStack(project, s)
	if err != nil {
		return nil, err
	}

	sm, err := func() (secrets.Manager, error) {
		configFile, err := getProjectStackPath(s)
		if err != nil {
			return nil, err
		}

		if (ps.SecretsProvider.Name == "" && ps.EncryptionSalt == "") || ps.SecretsProvider.Name == "default" {
			return s.DefaultSecretManager(ctx, host, configFile)
		}

		return pluginSecrets.NewPluginSecretsManagerFromConfig(ctx, host, s.Ref().Name(), configFile)

	}()
	if err != nil {
		return nil, err
	}
	return stack.NewCachingSecretsManager(sm), nil
}
