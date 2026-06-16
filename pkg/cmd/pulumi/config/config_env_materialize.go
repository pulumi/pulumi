// Copyright 2026, Pulumi Corporation.
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

package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// materializeProjectStack converts a decrypted environment definition into a ProjectStack with config
// re-encrypted under a local secrets provider and the environment's imports flattened to a name list. It
// refuses environments that carry values other than pulumiConfig, which a local stack file cannot
// represent. The returned structured slice names imports whose merge options were dropped, so the caller
// can warn. promptAllowed enables the interactive secrets-provider prompt; when false it defaults to
// "default". This is the shared materialization core of `config env eject` and `config env checkout`.
func materializeProjectStack(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	def []byte,
	envProject, envName, secretsProvider string,
	promptAllowed bool,
	opts display.Options,
) (ps *workspace.ProjectStack, imports, structured []string, err error) {
	pulumiConfig, imports, structured, otherValues, err := parseEjectedEnvironment(def)
	if err != nil {
		return nil, nil, nil, err
	}
	// A local stack file holds only config and imports. If the env carries other values
	// (environmentVariables, files, ...), refuse: materializing would silently drop them.
	if len(otherValues) > 0 {
		return nil, nil, nil, fmt.Errorf(
			"environment %s/%s defines values that cannot be represented in a local stack file (values.%s); "+
				"only environments whose values contain solely pulumiConfig are supported. Inspect them with "+
				"`pulumi config edit` or keep the stack on remote configuration",
			envProject, envName, strings.Join(otherValues, ", values."))
	}

	plaintextMap, err := buildPlaintextMap(pulumiConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	hasSecret := false
	for _, pt := range plaintextMap {
		if pt.Secure() {
			hasSecret = true
			break
		}
	}

	ps = &workspace.ProjectStack{}
	encrypter, err := resolveEncrypter(ctx, ssml, stack, ps, secretsProvider, hasSecret, promptAllowed, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	cfg, err := config.EncryptMap(ctx, plaintextMap, encrypter)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("re-encrypting configuration: %w", err)
	}

	ps.Config = config.Map(cfg)
	if len(imports) > 0 {
		ps.Environment = workspace.NewEnvironment(imports)
	}
	return ps, imports, structured, nil
}

// resolveEncrypter builds an Encrypter for local re-encryption: NopEncrypter when there are no secrets,
// otherwise the requested secretsProvider or "default". When promptAllowed is true and no provider is set,
// it prompts (defaulting to "default").
func resolveEncrypter(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	ps *workspace.ProjectStack,
	secretsProvider string,
	hasSecret bool,
	promptAllowed bool,
	opts display.Options,
) (config.Encrypter, error) {
	if !hasSecret {
		return config.NopEncrypter, nil
	}

	provider := secretsProvider
	if provider == "" {
		if promptAllowed {
			value, err := ui.PromptForValue(
				false, "secrets provider", "default", false,
				func(string) error { return nil }, opts)
			if err != nil {
				return nil, err
			}
			provider = value
		}
		// Fall back to "default" when non-interactive or the prompt is empty.
		if provider == "" {
			provider = "default"
		}
	}

	if err := cmdStack.ValidateSecretsProvider(provider); err != nil {
		return nil, err
	}
	if provider != "default" {
		ps.SecretsProvider = provider
	}

	encrypter, _, err := ssml.GetEncrypter(ctx, stack, ps)
	if err != nil {
		return nil, fmt.Errorf("setting up secrets provider %q: %w", provider, err)
	}
	return encrypter, nil
}

// writeWorkingCopy marshals ps and writes it atomically to path with the given banner prepended as
// leading comment lines. The banner is plain YAML trivia: it is ignored on load and excluded from the
// canonical hash, so it does not affect no-op detection.
func writeWorkingCopy(ps *workspace.ProjectStack, path, banner string) error {
	b, err := marshalProjectStack(ps, path)
	if err != nil {
		return err
	}
	if banner != "" {
		b = append([]byte(banner), b...)
	}
	return atomicWriteBytes(path, b)
}

// canonicalCheckoutHash hashes a ProjectStack's canonical content rather than its file bytes. It
// marshals a fresh copy that carries only the semantic fields, deliberately dropping the loaded file's
// trivia (the banner and original formatting a ProjectStack retains for comment-preserving saves). A raw
// byte hash would not survive even a no-op edit, since a later save reflows the file; this canonical hash
// is stable across that round trip and changes only when config values, imports, or secrets settings do.
func canonicalCheckoutHash(ps *workspace.ProjectStack) (string, error) {
	marshaler, ok := encoding.Marshalers[".yaml"]
	if !ok {
		return "", errors.New("no marshaler found for yaml")
	}
	// Copy only the exported (semantic) fields so the unexported raw byte cache does not leak the banner
	// and formatting into the hash.
	canonical := &workspace.ProjectStack{
		SecretsProvider: ps.SecretsProvider,
		EncryptedKey:    ps.EncryptedKey,
		EncryptionSalt:  ps.EncryptionSalt,
		Config:          ps.Config,
		Environment:     ps.Environment,
	}
	b, err := marshaler.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("hashing stack configuration: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
