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

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"gopkg.in/yaml.v3"
)

// ensureProviderEnv creates the target environment if --create was passed and
// the environment does not already exist. It is a no-op when create is false
// or when the environment exists.
func ensureProviderEnv(ctx context.Context, env *envCommand, ref environmentRef, create bool) error {
	if !create {
		return nil
	}
	exists, err := env.esc.client.EnvironmentExists(ctx, ref.orgName, ref.projectName, ref.envName)
	if err != nil && !client.IsNotFound(err) {
		return fmt.Errorf("checking environment existence: %w", err)
	}
	if exists {
		return nil
	}
	if err := env.esc.client.CreateEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName); err != nil {
		return fmt.Errorf("creating environment: %w", err)
	}
	fmt.Fprintf(env.esc.stdout, "Environment created: %v\n", ref.String())
	return nil
}

// mergeProviderIntoEnv merges providerNode into the YAML environment definition
// at values.<path>, replacing any existing node at that path. The result is the
// new YAML document bytes.
func mergeProviderIntoEnv(envYAML []byte, path resource.PropertyPath, providerNode *yaml.Node) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("path must contain at least one element")
	}

	var docNode yaml.Node
	if len(envYAML) > 0 {
		if err := yaml.Unmarshal(envYAML, &docNode); err != nil {
			return nil, fmt.Errorf("unmarshaling environment definition: %w", err)
		}
	}
	if docNode.Kind != yaml.DocumentNode {
		docNode = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{}}}
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: &docNode}.Get(resource.PropertyPath{"values"})
	if !ok {
		var err error
		valuesNode, err = encoding.YAMLSyntax{Node: &docNode}.Set(nil, resource.PropertyPath{"values"}, yaml.Node{
			Kind: yaml.MappingNode,
		})
		if err != nil {
			return nil, fmt.Errorf("creating values node: %w", err)
		}
	}

	if _, err := (encoding.YAMLSyntax{Node: valuesNode}).Set(nil, path, *providerNode); err != nil {
		return nil, fmt.Errorf("setting provider at %v: %w", path, err)
	}

	out, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return nil, fmt.Errorf("marshaling definition: %w", err)
	}
	return out, nil
}

// secretNode returns a yaml mapping node of the shape `fn::secret: <value>`.
// The value is always emitted as a string scalar (tag !!str), so callers do
// not have to worry about YAML coercing tokens like "true" or "12345" into
// booleans/numbers.
func secretNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::secret"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
		},
	}
}

// stringSequenceNode returns a yaml sequence node containing the given values
// as string scalars. Used by OIDC provider subcommands for fields like
// `policyArns` and `subjectAttributes`.
func stringSequenceNode(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		items = append(items, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
}

func applyProviderUpdate(
	ctx context.Context,
	env *envCommand,
	ref environmentRef,
	draft string,
	path resource.PropertyPath,
	providerNode *yaml.Node,
) error {
	var def []byte
	var tag string
	var err error
	if draft != "" && draft != "new" {
		def, tag, err = env.esc.client.GetEnvironmentDraft(ctx, ref.orgName, ref.projectName, ref.envName, draft)
		if err != nil {
			return fmt.Errorf("getting environment draft definition: %w", err)
		}
	} else {
		def, tag, _, err = env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, "", false)
		if err != nil {
			if client.IsNotFound(err) {
				return fmt.Errorf(
					"environment %s does not exist; pass --create to create it, or run `esc env init %s` first",
					ref.String(), ref.String())
			}
			return fmt.Errorf("getting environment definition: %w", err)
		}
	}

	newYAML, err := mergeProviderIntoEnv(def, path, providerNode)
	if err != nil {
		return err
	}

	diags, err := env.esc.updateEnvironment(ctx, ref, draft, newYAML, tag, "Provider updated.")
	if err != nil {
		return err
	}
	if len(diags) != 0 {
		werr := env.writeYAMLEnvironmentDiagnostics(env.esc.stderr, ref.projectName+"/"+ref.envName, newYAML, diags)
		contract.IgnoreError(werr)
	}
	if client.DiagnosticsHaveErrors(diags) {
		return errors.New("provider update failed")
	}
	return nil
}
