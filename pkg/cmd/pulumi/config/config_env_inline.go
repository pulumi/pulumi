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
	"errors"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// setStackConfigValue stores a configuration value in the stack's configuration file. When the file
// defines the stack's environment inline, plain values go to `environment.values.pulumiConfig`, which
// `pulumi up` publishes; secrets stay in the `config` block, encrypted with the stack's secrets
// provider, because the inline definition is committed in plaintext. Whichever block receives the
// value, the other one drops its copy: stack config shadows the environment during merging.
func setStackConfigValue(ps *workspace.ProjectStack, key config.Key, v config.Value, path bool) error {
	if !ps.Environment.IsDefinition() {
		return ps.Config.Set(key, v, path)
	}

	envKey, envPath, err := environmentConfigPath(key, path)
	if err != nil {
		return err
	}
	if v.Secure() {
		if _, err := ps.Environment.RemovePulumiConfig(envKey, envPath); err != nil {
			return fmt.Errorf("updating the stack's environment: %w", err)
		}
		return ps.Config.Set(key, v, path)
	}

	node, err := environmentConfigNode(v, len(envPath) > 0)
	if err != nil {
		return err
	}
	if err := ps.Environment.SetPulumiConfig(envKey, envPath, node); err != nil {
		return fmt.Errorf("updating the stack's environment: %w", err)
	}
	if ps.Config != nil {
		if err := ps.Config.Remove(key, path); err != nil {
			return err
		}
	}
	return nil
}

// removeStackConfigValue removes a configuration value from wherever the stack's configuration file
// holds it: the `config` block and, for a stack with an inline environment, `values.pulumiConfig`.
func removeStackConfigValue(ps *workspace.ProjectStack, key config.Key, path bool) error {
	if ps.Environment.IsDefinition() {
		envKey, envPath, err := environmentConfigPath(key, path)
		if err != nil {
			return err
		}
		if _, err := ps.Environment.RemovePulumiConfig(envKey, envPath); err != nil {
			return fmt.Errorf("updating the stack's environment: %w", err)
		}
	}
	if ps.Config == nil {
		return nil
	}
	return ps.Config.Remove(key, path)
}

// environmentConfigPath splits a configuration key into the `pulumiConfig` entry it lives under and
// the path inside that entry's value, mirroring how config.Map reads a `--path` key.
func environmentConfigPath(key config.Key, path bool) (string, []any, error) {
	if !path {
		return key.String(), nil, nil
	}
	p, err := resource.ParsePropertyPathStrict(key.Name())
	if err != nil {
		return "", nil, fmt.Errorf("invalid config key path: %w", err)
	}
	if len(p) == 0 {
		return "", nil, errors.New("empty config key path")
	}
	root, ok := p[0].(string)
	if !ok || root == "" {
		return "", nil, errors.New("first path segment of config key must be a string")
	}
	return config.MustMakeKey(key.Namespace(), root).String(), []any(p[1:]), nil
}

// environmentConfigNode renders a plain configuration value as the YAML node written into the inline
// environment definition. Values nested with --path get the same bool/int coercion config.Map applies.
func environmentConfigNode(v config.Value, nested bool) (*yaml.Node, error) {
	raw, err := v.MarshalYAML()
	if err != nil {
		return nil, err
	}
	if s, ok := raw.(string); ok && nested {
		raw = coerceConfigScalar(s)
	}
	var node yaml.Node
	if err := node.Encode(raw); err != nil {
		return nil, fmt.Errorf("encoding config value: %w", err)
	}
	return &node, nil
}

func coerceConfigScalar(s string) any {
	if b, err := strconv.ParseBool(s); err == nil && (s == "true" || s == "false") {
		return b
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
}
