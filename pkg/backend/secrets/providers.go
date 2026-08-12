// Copyright 2025, Pulumi Corporation.
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

package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// DefaultProvider is the default DefaultProvider to use when deserializing deployments.
var DefaultProvider secrets.Provider = &secretsProvider{}

// secretsProvider implements the secrets.ManagerProviderFactory interface. Essentially
// it is the global location where new secrets managers can be registered for use when
// decrypting checkpoints.
type secretsProvider struct{}

// OfType returns a secrets manager for the given secrets type. Returns an error
// if the type is unknown or the state is invalid.
func (secretsProvider) OfType(ctx context.Context, ty string, state json.RawMessage) (secrets.Manager, error) {
	var sm secrets.Manager
	var err error
	switch ty {
	case passphrase.Type:
		sm, err = passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	case service.Type:
		sm, err = service.NewServiceSecretsManagerFromState(ctx, state)
	case cloud.Type:
		sm, err = cloud.NewCloudSecretsManagerFromState(state)
	default:
		return nil, fmt.Errorf("no known secrets provider for type %q", ty)
	}
	if err != nil {
		return nil, fmt.Errorf("constructing secrets manager of type %q: %w", ty, err)
	}

	return stack.NewBatchingCachingSecretsManager(sm), nil
}

// BlindingProvider builds secrets managers that redact every secret to config.BlindingCrypter's "[secret]"
// sentinel instead of decrypting it. It never needs a passphrase or other credentials, so it can be used to
// deserialize a checkpoint when we only need to read non-secret data (or display secrets masked) — for example
// reading a non-secret stack output or listing resources in `pulumi about` — without prompting for a passphrase.
var BlindingProvider secrets.Provider = blindingProvider{}

type blindingProvider struct{}

func (blindingProvider) OfType(_ context.Context, ty string, state json.RawMessage) (secrets.Manager, error) {
	return blindingManager{ty: ty, state: state}, nil
}

type blindingManager struct {
	ty    string
	state json.RawMessage
}

func (m blindingManager) Type() string                { return m.ty }
func (m blindingManager) State() json.RawMessage      { return m.state }
func (m blindingManager) Encrypter() config.Encrypter { return config.BlindingCrypter }
func (m blindingManager) Decrypter() config.Decrypter { return redactingDecrypter{} }

type redactingDecrypter struct{}

// DecryptValue returns config.BlindingCrypter's "[secret]" sentinel, JSON-encoded: deployment deserialization
// unmarshals each decrypted plaintext as a JSON value, so the bare sentinel (not valid JSON) can't be returned
// directly.
func (redactingDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	redacted, err := config.BlindingCrypter.DecryptValue(ctx, ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(redacted)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (d redactingDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return config.DefaultBatchDecrypt(ctx, d, ciphertexts)
}

// NamedStackProvider is the same as the default secrets provider,
// but is aware of the stack name for which it is used.  Currently
// this is only used for prompting passphrase secrets managers to show
// the stackname in the prompt for the passphrase.
type NamedStackProvider struct {
	StackName string
}

// OfType returns a secrets manager for the given secrets type. Returns an error
// if the type is unknown or the state is invalid.
func (s NamedStackProvider) OfType(ctx context.Context, ty string, state json.RawMessage) (secrets.Manager, error) {
	var sm secrets.Manager
	var err error
	switch ty {
	case passphrase.Type:
		sm, err = passphrase.NewStackPromptingPassphraseSecretsManagerFromState(state, s.StackName)
	case service.Type:
		sm, err = service.NewServiceSecretsManagerFromState(ctx, state)
	case cloud.Type:
		sm, err = cloud.NewCloudSecretsManagerFromState(state)
	default:
		return nil, fmt.Errorf("no known secrets provider for type %q", ty)
	}
	if err != nil {
		return nil, fmt.Errorf("constructing secrets manager of type %q: %w", ty, err)
	}

	return stack.NewBatchingCachingSecretsManager(sm), nil
}
