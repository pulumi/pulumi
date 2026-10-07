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

package newcmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// initStackEnvironmentFromConfig moves the plain configuration values `pulumi new` saved into an inline
// environment definition and publishes it, creating the environment the new stack manages. Secret values
// stay in the `config` block: they are encrypted with the stack's secrets provider, which the environment
// cannot read, and moving them would mean writing them in plaintext.
func initStackEnvironmentFromConfig(
	ctx context.Context,
	sink diag.Sink,
	ws pkgWorkspace.Context,
	s backend.Stack,
	stdout io.Writer,
) error {
	project, _, err := ws.ReadProject("")
	if err != nil {
		return err
	}
	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, s, "")
	if err != nil {
		return err
	}

	if err := moveConfigIntoEnvironment(ps); err != nil {
		return err
	}
	if err := cmdStack.SaveProjectStack(ctx, s, ps, ""); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return cmdStack.InitStackEnvironment(ctx, stdout, s, ps, "")
}

// moveConfigIntoEnvironment rewrites ps so that its plain configuration values live under
// `environment.values.pulumiConfig` instead of `config`. Secret values are left where they are.
func moveConfigIntoEnvironment(ps *workspace.ProjectStack) error {
	if ps.Environment != nil && !ps.Environment.IsDefinition() {
		return errors.New("the stack's configuration file imports environments; replace the list with an " +
			"inline environment definition to use --esc-config")
	}

	pulumiConfig := map[string]any{}
	for key, value := range ps.Config {
		if value.Secure() {
			continue
		}
		if value.Object() {
			obj, err := value.ToObject()
			if err != nil {
				return fmt.Errorf("reading config %s: %w", key, err)
			}
			pulumiConfig[key.String()] = obj
		} else {
			plain, err := value.Value(config.NopDecrypter)
			if err != nil {
				return fmt.Errorf("reading config %s: %w", key, err)
			}
			pulumiConfig[key.String()] = plain
		}
		delete(ps.Config, key)
	}
	if len(ps.Config) == 0 {
		ps.Config = nil
	}

	definition, err := yaml.Marshal(map[string]any{
		"values": map[string]any{
			"pulumiConfig": pulumiConfig,
		},
	})
	if err != nil {
		return fmt.Errorf("building environment definition: %w", err)
	}
	env, err := workspace.NewEnvironmentDefinition(definition)
	if err != nil {
		return err
	}
	ps.Environment = env
	return nil
}
