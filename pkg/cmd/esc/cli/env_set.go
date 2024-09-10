// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ccojocar/zxcvbn-go"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newEnvSetCmd(env *envCommand) *cobra.Command {
	var secret bool
	var plaintext bool

	cmd := &cobra.Command{
		Use:   "set [<org-name>/][<project-name>/]<environment-name> <path> <value>",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Set a value within an environment.",
		Long: "Set a value within an environment\n" +
			"\n" +
			"This command fetches the current definition for the named environment and modifies a\n" +
			"value within it. The path to the value to set is a Pulumi property path. The value\n" +
			"is interpreted as YAML.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the set command does not accept versions")
			}
			if len(args) < 2 {
				return fmt.Errorf("expected a path and a value")
			}

			path, err := resource.ParsePropertyPath(args[0])
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			if len(path) == 0 {
				return fmt.Errorf("path must contain at least one element")
			}

			var yamlValue yaml.Node
			if err := yaml.Unmarshal([]byte(args[1]), &yamlValue); err != nil {
				return fmt.Errorf("invalid value: %w", err)
			}
			if len(yamlValue.Content) == 0 {
				// This can happen when the value is empty (e.g. when "" is present on the command line). Treat this
				// as the empty string.
				err = yaml.Unmarshal([]byte(`""`), &yamlValue)
				contract.IgnoreError(err)
			}
			yamlValue = *yamlValue.Content[0]

			if looksLikeSecret(path, yamlValue) && !secret && !plaintext {
				return fmt.Errorf("value looks like a secret; rerun with --secret to mark it as such, or --plaintext if you meant to leave it as plaintext")
			}
			if secret {
				if yamlValue.Kind == yaml.ScalarNode && yamlValue.Tag != "!!str" {
					err = yaml.Unmarshal([]byte(strconv.Quote(args[1])), &yamlValue)
					if err != nil {
						return fmt.Errorf("internal error decoding value; try surrounding the argument in both single and double quotes (e.g. '\"foo\"') (%w)", err)
					}
					yamlValue = *yamlValue.Content[0]
				}

				bytes, err := yaml.Marshal(map[string]yaml.Node{"fn::secret": yamlValue})
				if err != nil {
					return fmt.Errorf("internal error: marshaling secret: %w", err)
				}
				if err = yaml.Unmarshal(bytes, &yamlValue); err != nil {
					return fmt.Errorf("internal error: marshaling secret: %w", err)
				}
				yamlValue = *yamlValue.Content[0]
			}

			def, tag, _, err := env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, "", false)
			if err != nil {
				return fmt.Errorf("getting environment definition: %w", err)
			}

			var docNode yaml.Node
			if err := yaml.Unmarshal(def, &docNode); err != nil {
				return fmt.Errorf("unmarshaling environment definition: %w", err)
			}
			if docNode.Kind != yaml.DocumentNode {
				docNode = yaml.Node{
					Kind:    yaml.DocumentNode,
					Content: []*yaml.Node{{}},
				}
			}

			if path[0] == "imports" {
				_, err = yamlNode{&docNode}.set(nil, path, yamlValue)
			} else {
				valuesNode, ok := yamlNode{&docNode}.get(resource.PropertyPath{"values"})
				if !ok {
					valuesNode, err = yamlNode{&docNode}.set(nil, resource.PropertyPath{"values"}, yaml.Node{
						Kind: yaml.MappingNode,
					})
					if err != nil {
						return fmt.Errorf("internal error: %w", err)
					}
				}
				_, err = yamlNode{valuesNode}.set(nil, path, yamlValue)
			}
			if err != nil {
				return err
			}

			newYAML, err := yaml.Marshal(docNode.Content[0])
			if err != nil {
				return fmt.Errorf("marshaling definition: %w", err)
			}

			diags, err := env.esc.client.UpdateEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, newYAML, tag)
			if err != nil {
				return fmt.Errorf("updating environment definition: %w", err)
			}
			if len(diags) != 0 {
				return env.writePropertyEnvironmentDiagnostics(env.esc.stderr, diags)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(
		&secret, "secret", false,
		"true to mark the value as secret")
	cmd.Flags().BoolVar(
		&plaintext, "plaintext", false,
		"true to leave the value in plaintext")

	return cmd
}

// keyPattern is the regular expression a configuration key must match before we check (and error) if we think
// it is a password
var keyPattern = regexp.MustCompile("(?i)passwd|pass|password|pwd|secret|token")

const (
	// maxEntropyCheckLength is the maximum length of a possible secret for entropy checking.
	maxEntropyCheckLength = 16
	// entropyThreshold is the total entropy threshold a potential secret needs to pass before being flagged.
	entropyThreshold = 80.0
	// entropyCharThreshold is the per-char entropy threshold a potential secret needs to pass before being flagged.
	entropyPerCharThreshold = 3.0
)

// looksLikeSecret returns true if a configuration value "looks" like a secret. This is always going to be a heuristic
// that suffers from false positives, but is better (a) than our prior approach of unconditionally printing a warning
// for all plaintext values, and (b)  to be paranoid about such things. Inspired by the gas linter and securego project.
func looksLikeSecret(path resource.PropertyPath, n yaml.Node) bool {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return false
	}
	v := n.Value

	key, ok := path[len(path)-1].(string)
	if !ok || !keyPattern.MatchString(key) {
		return false
	}

	if len(v) > maxEntropyCheckLength {
		v = v[:maxEntropyCheckLength]
	}

	// Compute the strength use the resulting entropy to flag whether this looks like a secret.
	info := zxcvbn.PasswordStrength(v, nil)
	entropyPerChar := info.Entropy / float64(len(v))
	return info.Entropy >= entropyThreshold ||
		(info.Entropy >= (entropyThreshold/2) && entropyPerChar >= entropyPerCharThreshold)
}

type yamlNode struct {
	*yaml.Node
}

func (n yamlNode) get(path resource.PropertyPath) (_ *yaml.Node, ok bool) {
	if n.Kind == yaml.DocumentNode {
		return yamlNode{n.Content[0]}.get(path)
	}

	if len(path) == 0 {
		return n.Node, true
	}

	switch n.Kind {
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(n.Content) {
			return nil, false
		}
		return yamlNode{n.Content[index]}.get(path[1:])
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return nil, false
		}
		for i := 0; i < len(n.Content); i += 2 {
			keyNode, valueNode := n.Content[i], n.Content[i+1]
			if keyNode.Value == key {
				return yamlNode{valueNode}.get(path[1:])
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

func (n yamlNode) set(prefix, path resource.PropertyPath, new yaml.Node) (*yaml.Node, error) {
	if n.Kind == yaml.DocumentNode {
		return yamlNode{n.Content[0]}.set(prefix, path, new)
	}

	if len(path) == 0 {
		n.Content = new.Content
		n.Kind = new.Kind
		n.Tag = new.Tag
		n.Value = new.Value
		return n.Node, nil
	}

	prefix = append(prefix, path[0])
	switch n.Kind {
	case 0:
		switch accessor := path[0].(type) {
		case int:
			n.Kind, n.Tag = yaml.SequenceNode, "!!seq"
		case string:
			n.Kind, n.Tag = yaml.MappingNode, "!!map"
		default:
			contract.Failf("unexpected accessor kind %T", accessor)
			return nil, nil
		}
		return n.set(prefix[:len(prefix)-1], path, new)
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok {
			return nil, fmt.Errorf("%v: key for an array must be an int", prefix)
		}
		if index < 0 || index > len(n.Content) {
			return nil, fmt.Errorf("%v: array index out of range", prefix)
		}
		if index == len(n.Content) {
			n.Content = append(n.Content, &yaml.Node{})
		}
		elem := n.Content[index]
		return yamlNode{elem}.set(prefix, path[1:], new)
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return nil, fmt.Errorf("%v: key for a map must be a string", prefix)
		}

		var valueNode *yaml.Node
		for i := 0; i < len(n.Content); i += 2 {
			keyNode, value := n.Content[i], n.Content[i+1]
			if keyNode.Value == key {
				valueNode = value
				break
			}
		}
		if valueNode == nil {
			n.Content = append(n.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
				Tag:   "!!str",
			})
			n.Content = append(n.Content, &yaml.Node{})
			valueNode = n.Content[len(n.Content)-1]
		}
		return yamlNode{valueNode}.set(prefix, path[1:], new)
	default:
		return nil, fmt.Errorf("%v: expected an array or an object", prefix)
	}
}

func (n yamlNode) delete(prefix, path resource.PropertyPath) error {
	if n.Kind == yaml.DocumentNode {
		return yamlNode{n.Content[0]}.delete(prefix, path)
	}

	prefix = append(prefix, path[0])
	switch n.Kind {
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok {
			return fmt.Errorf("%v: key for an array must be an int", prefix)
		}
		if index < 0 || index >= len(n.Content) {
			return fmt.Errorf("%v: array index out of range", prefix)
		}
		if len(path) == 1 {
			n.Content = append(n.Content[:index], n.Content[index+1:]...)
			return nil
		}
		elem := n.Content[index]
		return yamlNode{elem}.delete(prefix, path[1:])
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return fmt.Errorf("%v: key for a map must be a string", prefix)
		}

		i := 0
		for ; i < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				break
			}
		}
		if len(path) == 1 {
			if i != len(n.Content) {
				n.Content = append(n.Content[:i], n.Content[i+2:]...)
			}
			return nil
		}
		valueNode := n.Content[i+1]
		return yamlNode{valueNode}.delete(prefix, path[1:])
	default:
		return fmt.Errorf("%v: expected an array or an object", prefix)
	}
}
