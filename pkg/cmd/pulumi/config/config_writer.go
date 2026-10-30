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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ConfigWriter writes a stack's config to its stack file or to its ESC-managed environment's definition.
type ConfigWriter interface {
	Set(ctx context.Context, key config.Key, v config.Value, path bool) error
	Remove(ctx context.Context, key config.Key, path bool) error
	// CheckSet returns the error that Set would return for key whatever the value, so a caller can fail before it
	// encrypts a secret.
	CheckSet(ctx context.Context, key config.Key, path bool) error
	// Call Encrypter only to write a secret: for the stack file it loads the stack secrets manager, which can prompt for a
	// passphrase or need cloud credentials.
	Encrypter(ctx context.Context) (config.Encrypter, error)
	Save(ctx context.Context) error
}

func newConfigWriter(
	s backend.Stack,
	projectName tokens.PackageName,
	ps *workspace.ProjectStack,
	configFile string,
	ef encrypterFactory,
	overrideImport bool,
) (ConfigWriter, error) {
	if s.ConfigLocation().ManagedEnvironment == "" {
		return &stackFileWriter{stack: s, ps: ps, configFile: configFile, ef: ef}, nil
	}
	managed, err := newManagedStore(s, projectName, ps, configFile, overrideImport)
	if err != nil {
		return nil, err
	}
	return managed, nil
}

type stackFileWriter struct {
	stack      backend.Stack
	ps         *workspace.ProjectStack
	configFile string
	ef         encrypterFactory
}

func (w *stackFileWriter) Set(_ context.Context, key config.Key, v config.Value, path bool) error {
	return w.ps.Config.Set(key, v, path)
}

func (w *stackFileWriter) CheckSet(context.Context, config.Key, bool) error {
	return nil
}

func (w *stackFileWriter) Remove(_ context.Context, key config.Key, path bool) error {
	return w.ps.Config.Remove(key, path)
}

func (w *stackFileWriter) Encrypter(ctx context.Context) (config.Encrypter, error) {
	// The command always saves afterwards, so whether GetEncrypter changed the stack file does not matter.
	enc, _, err := w.ef.GetEncrypter(ctx, w.stack, w.ps)
	return enc, err
}

func (w *stackFileWriter) Save(ctx context.Context) error {
	return cmdStack.SaveProjectStack(ctx, w.stack, w.ps, w.configFile)
}
