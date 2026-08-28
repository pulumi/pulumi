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
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// saveTemplateConfigToEnvironment is the `--esc-config` counterpart of saveTemplateConfig: the config the
// wizard gathered becomes the initial contents of the stack's own ESC environment instead of the stack
// file's `config:` block.
//
// The stack's secrets manager is never involved. A prompted secret is wrapped in `fn::secret` and
// encrypted by ESC, so its plaintext exists only in the in-memory YAML node this builds, never on disk
// and never in a log.
func saveTemplateConfigToEnvironment(
	ctx context.Context, sink diag.Sink, out io.Writer,
	project *workspace.Project, s backend.Stack,
	values []templateConfigValue, commandLineConfig config.Map, configFile string,
) error {
	if out == nil {
		out = io.Discard
	}

	envValues, err := templateConfigNodes(values, commandLineConfig)
	if err != nil {
		return err
	}

	mainEnv, err := cmdStack.CreateStackEnvironments(ctx, s, cmdStack.StackEnvironmentOptions{
		EnvProject: project.Name.String(),
		EnvName:    s.Ref().Name().String(),
		Values:     envValues,
		Stdout:     out,
	})
	if err != nil {
		return err
	}

	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, s, configFile)
	if err != nil {
		return err
	}
	ps.MainEnvironment = mainEnv
	if err := cmdStack.SaveProjectStack(ctx, s, ps, configFile); err != nil {
		return err
	}

	if len(envValues) > 0 {
		fmt.Fprintf(out, "Saved config to %s\n", mainEnv.Ref())
		fmt.Fprintln(out)
	}
	return nil
}

// templateConfigNodes renders the gathered config as the `values.pulumiConfig` entries of an environment
// definition. `--config` values win over prompted ones, matching encryptTemplateConfig.
func templateConfigNodes(
	values []templateConfigValue, commandLineConfig config.Map,
) (map[string]yaml.Node, error) {
	nodes := make(map[string]yaml.Node, len(values)+len(commandLineConfig))

	for key, val := range commandLineConfig {
		// Command-line values come from ParseConfig, which never produces a secure value, so no
		// decrypter is needed to read them back.
		if val.Object() {
			obj, err := val.ToObject()
			if err != nil {
				return nil, err
			}
			var node yaml.Node
			if err := node.Encode(obj); err != nil {
				return nil, fmt.Errorf("rendering config value %v: %w", key, err)
			}
			nodes[key.String()] = node
			continue
		}
		plain, err := val.Value(config.NopDecrypter)
		if err != nil {
			return nil, err
		}
		node, err := cmdConfig.ConfigValueNode(plain, "" /*typ*/, false /*secret*/)
		if err != nil {
			return nil, err
		}
		nodes[key.String()] = node
	}

	for _, v := range values {
		if _, ok := nodes[v.key.String()]; ok {
			continue
		}
		if v.value == "" {
			// Don't add empty values to the config.
			continue
		}
		node, err := cmdConfig.ConfigValueNode(v.value, "" /*typ*/, v.secret)
		if err != nil {
			return nil, err
		}
		nodes[v.key.String()] = node
	}

	return nodes, nil
}
