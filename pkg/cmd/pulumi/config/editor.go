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
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// configEditor is a write-focused abstraction over a stack's configuration store. Mutations are
// buffered and persisted on Save. Callers pass plaintext for secrets (a config.Value with
// Secure()=true whose raw value is the plaintext); the editor is responsible for encrypting or
// otherwise protecting the secret according to where the config lives.
type configEditor interface {
	// If path is true, key's name is a property path within a map or list.
	Set(ctx context.Context, key config.Key, value config.Value, path bool) error
	// If path is true, key's name is a property path. Removing an absent key is a no-op.
	Remove(ctx context.Context, key config.Key, path bool) error
	Save(ctx context.Context) error
}

// newConfigEditor returns a configEditor for the stack's configuration store. encrypter is used by
// the local editor to encrypt secret values before they are written to the stack file; it is unused
// for stores that protect secrets themselves.
func newConfigEditor(
	_ context.Context, stack backend.Stack, ps *workspace.ProjectStack, encrypter config.Encrypter, configFile string,
) (configEditor, error) {
	if configStoreIsRemote(stack, configFile) {
		return nil, errors.New("editing remote stack configuration is not supported")
	}
	return &localConfigEditor{stack: stack, ps: ps, encrypter: encrypter, configFile: configFile}, nil
}

// configStoreIsRemote reports whether the stack's configuration is effectively stored remotely. An
// explicit --config-file selects a local file regardless of the stack's linked location, mirroring
// the precedence in cmdStack.LoadProjectStack/SaveProjectStack.
func configStoreIsRemote(stack backend.Stack, configFile string) bool {
	return configFile == "" && stack.ConfigLocation().IsRemote
}

type localConfigEditor struct {
	stack      backend.Stack
	ps         *workspace.ProjectStack
	encrypter  config.Encrypter
	configFile string
}

func (e *localConfigEditor) Set(ctx context.Context, key config.Key, value config.Value, path bool) error {
	// Secure object values already carry per-leaf ciphertext the caller produced; encrypting the
	// whole serialized object as one blob would corrupt it, so only scalar secrets are encrypted here.
	if value.Secure() && !value.Object() {
		plaintext, err := value.Value(config.NopDecrypter)
		if err != nil {
			return err
		}
		encrypted, err := e.encrypter.EncryptValue(ctx, plaintext)
		if err != nil {
			return err
		}
		value = config.NewSecureValue(encrypted)
	}
	return e.ps.Config.Set(key, value, path)
}

func (e *localConfigEditor) Remove(_ context.Context, key config.Key, path bool) error {
	return e.ps.Config.Remove(key, path)
}

func (e *localConfigEditor) Save(ctx context.Context) error {
	return cmdStack.SaveProjectStack(ctx, e.stack, e.ps, e.configFile)
}
