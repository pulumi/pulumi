// Copyright 2016-2022, Pulumi Corporation.
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

package plugin

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// How the hell a method like this isn't built into the standard library absolutely beggars belief.
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func NewPluginSecretsManager(ctx context.Context, host plugin.Host, stackName tokens.Name, configFile string,
	pluginName string, arguments []string, rotateSecretsProvider bool) (secrets.Manager, error) {

	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")
	proj, _, err := workspace.DetectProjectStackPath(stackName.Q())
	if err != nil {
		return nil, err
	}

	info, err := workspace.LoadProjectStack(proj, configFile)
	if err != nil {
		return nil, err
	}

	isUrl := func(str string) bool {
		url, err := url.Parse(str)
		if err == nil {
			return url.Scheme != ""
		}
		return false
	}

	if isUrl(info.SecretsProvider.Name) {
		// We manage some back compatibility code here so cloud plugin continues to look the same,
		// that is the "Name" of the secrets provider is actually the cloud url
		info.SecretsProvider.Name = "cloud"
		info.SecretsProvider.Version = nil
		info.SecretsProvider.State = nil
	} else if info.SecretsProvider.Name == "" {
		// We manage some back compatibility code here so passphrase plugin continues to look the same,
		// that is the "Name" of the secrets provider is blank

		info.SecretsProvider.Name = "passphrase"
		info.SecretsProvider.Version = nil
		info.SecretsProvider.State = nil
	}

	// If there's no existing configuration state or the secrets provider is changing
	// then we need to initialize a new secrets provider.
	pluginChanged := pluginName != info.SecretsProvider.Name
	info.SecretsProvider.Name = pluginName

	// Assume the latest version of the plugin if we've changed, else use the current version of the plugin
	if pluginChanged {
		// Try to grab the latest version
		spec := workspace.PluginSpec{Name: pluginName, Kind: workspace.SecretsPlugin}
		latestVersion, err := spec.GetLatestVersion()
		if err != nil {
			info.SecretsProvider.Version = latestVersion
		} else {
			info.SecretsProvider.Version = nil
		}
	}

	secrets, err := host.Secrets(info.SecretsProvider.Name, info.SecretsProvider.Version)
	if err != nil {
		return nil, err
	}

	contract.Assert(rotateSecretsProvider || pluginChanged)

	prompt, maybeState, err := secrets.Initalize(ctx, arguments, nil)
	if err != nil {
		return nil, err
	}
	inputs := make(map[string]string)
	for prompt != nil {
		prompt, maybeState, err = secrets.Initalize(ctx, arguments, inputs)
		if err != nil {
			return nil, err
		}
		// Clear any invalidated inputs
		for key := range inputs {
			if !contains(prompt.Preserve, key) {
				delete(inputs, key)
			}
		}
	}
	contract.Assert(maybeState != nil)
	info.SecretsProvider.State = *maybeState

	sm, err := NewPluginSecretsManagerFromProvider(secrets, info.SecretsProvider.State)
	if err != nil {
		return nil, err
	}

	info.EncryptionSalt = ""
	info.EncryptedKey = ""
	if pluginName == "cloud" {
		// We manage some back-compatibility here so that cloud state stays looking the same (for now).
		type cloudStateStruct struct {
			encryptedkey string
		}
		var cloudState cloudStateStruct
		err = json.Unmarshal(info.SecretsProvider.State, &cloudState)
		if err != nil {
			return nil, err
		}

		info.EncryptedKey = cloudState.encryptedkey
		info.SecretsProvider.Name = arguments[0]
		info.SecretsProvider.Version = nil
		info.SecretsProvider.State = nil
	} else if pluginName == "passphrase" {
		// We manage some back-compatibility here so that passphrase state stays looking the same (for now).
		type passphraseStateStruct struct {
			salt string
		}
		var passphraseState passphraseStateStruct
		err = json.Unmarshal(info.SecretsProvider.State, &passphraseState)
		if err != nil {
			return nil, err
		}

		info.EncryptionSalt = passphraseState.salt
		info.SecretsProvider.Name = ""
		info.SecretsProvider.Version = nil
		info.SecretsProvider.State = nil
	}

	if err = info.Save(configFile); err != nil {
		return nil, err
	}

	return sm, nil
}

func NewPluginSecretsManagerFromConfig(ctx context.Context, host plugin.Host, stackName tokens.Name, configFile string) (secrets.Manager, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")
	proj, _, err := workspace.DetectProjectStackPath(stackName.Q())
	if err != nil {
		return nil, err
	}

	info, err := workspace.LoadProjectStack(proj, configFile)
	if err != nil {
		return nil, err
	}

	isUrl := func(str string) bool {
		url, err := url.Parse(str)
		if err == nil {
			return url.Scheme != ""
		}
		return false
	}

	pluginName := info.SecretsProvider.Name
	pluginState := info.SecretsProvider.State
	if isUrl(info.SecretsProvider.Name) {
		// We manage some back compatibility code here so cloud plugin continues to look the same,
		// that is the "Name" of the secrets provider is actually the cloud url
		type cloudStateStruct struct {
			Url          string `json:"url,omitempty" yaml:"url,omitempty"`
			EncryptedKey string `json:"encryptedkey,omitempty" yaml:"encryptedkey,omitempty"`
		}
		cloudState := cloudStateStruct{
			Url:          info.SecretsProvider.Name,
			EncryptedKey: info.EncryptedKey,
		}
		pluginName = "cloud"
		pluginState, err = json.Marshal(cloudState)
		if err != nil {
			return nil, err
		}
	} else if info.SecretsProvider.Name == "" {
		// We manage some back compatibility code here so passphrase plugin continues to look the same,
		// that is the "Name" of the secrets provider is blank
		type passphraseStateStruct struct {
			Salt string `json:"salt,omitempty" yaml:"salt,omitempty"`
		}
		passphraseState := passphraseStateStruct{
			Salt: info.EncryptionSalt,
		}
		pluginName = "passphrase"
		pluginState, err = json.Marshal(passphraseState)
		if err != nil {
			return nil, err
		}
	}

	secrets, err := host.Secrets(pluginName, info.SecretsProvider.Version)
	if err != nil {
		return nil, err
	}

	prompt, err := secrets.Configure(ctx, pluginState, nil)
	if err != nil {
		return nil, err
	}
	inputs := make(map[string]string)
	for prompt != nil {
		prompt, err = secrets.Configure(ctx, pluginState, nil)
		if err != nil {
			return nil, err
		}
		// Clear any invalidated inputs
		for key := range inputs {
			if !contains(prompt.Preserve, key) {
				delete(inputs, key)
			}
		}
	}

	sm, err := NewPluginSecretsManagerFromProvider(secrets, pluginState)
	if err != nil {
		return nil, err
	}
	return sm, nil
}

func NewPluginSecretsManagerFromState(ctx context.Context, host plugin.Host, name string, version *semver.Version, state json.RawMessage) (secrets.Manager, error) {
	secrets, err := host.Secrets(name, version)
	if err != nil {
		return nil, err
	}
	prompt, err := secrets.Configure(ctx, state, nil)
	if err != nil {
		return nil, err
	}

	inputs := make(map[string]string)

	// Configure may have asked for a prompt, so ask the user for input
	for {
		if prompt == nil {
			break
		}

		response, err := cmdutil.ReadConsoleNoEcho(prompt.Text)
		if err != nil {
			return nil, err
		}
		inputs[prompt.Label] = response

		prompt, err = secrets.Configure(ctx, state, inputs)
		if err != nil {
			return nil, err
		}
	}

	return &manager{secrets: secrets, state: state}, nil
}

func NewPluginSecretsManagerFromProvider(provider plugin.SecretsProvider, state json.RawMessage) (secrets.Manager, error) {
	return &manager{secrets: provider, state: state}, nil
}

type manager struct {
	name    string
	secrets plugin.SecretsProvider
	state   json.RawMessage
}

func (m *manager) Type() string                         { return m.name }
func (m *manager) State() json.RawMessage               { return m.state }
func (m *manager) Encrypter() (config.Encrypter, error) { return m, nil }
func (m *manager) Decrypter() (config.Decrypter, error) { return m, nil }

func (m *manager) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	ciphertexts, err := m.secrets.Encrypt(ctx, []string{plaintext})
	if err != nil {
		return "", err
	}
	return ciphertexts[0], nil
}

func (m *manager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	plaintexts, err := m.secrets.Decrypt(ctx, []string{ciphertext})
	if err != nil {
		return "", err
	}
	return plaintexts[0], nil
}

func (m *manager) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	plaintexts, err := m.secrets.Decrypt(ctx, ciphertexts)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for i := range ciphertexts {
		result[ciphertexts[i]] = plaintexts[i]
	}
	return result, nil
}
