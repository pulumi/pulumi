// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type envGetCommand struct {
	env *envCommand
}

func newEnvGetCmd(env *envCommand) *cobra.Command {
	var value string
	var showSecrets bool

	get := &envGetCommand{env: env}

	cmd := &cobra.Command{
		Use:   "get [<org-name>/]<environment-name> <path>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Get a value within an environment.",
		Long: "Get a value within an environment\n" +
			"\n" +
			"This command fetches the current definition for the named environment and gets a\n" +
			"value within it. The path to the value to set is a Pulumi property path. The value\n" +
			"is printed to stdout as YAML.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			orgName, envName, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}

			var path resource.PropertyPath
			if len(args) != 0 {
				path, err = resource.ParsePropertyPath(args[0])
				if err != nil {
					return fmt.Errorf("invalid path: %w", err)
				}
			}

			switch value {
			case "":
				// OK
			case "detailed", "json", "string":
				return get.showValue(ctx, orgName, envName, path, value, showSecrets)
			case "dotenv":
				if len(path) != 0 {
					return fmt.Errorf("output format '%s' may not be used with a property path", value)
				}
				return get.showValue(ctx, orgName, envName, path, value, showSecrets)
			case "shell":
				if len(path) != 0 {
					return fmt.Errorf("output format '%s' may not be used with a property path", value)
				}
				return get.showValue(ctx, orgName, envName, path, value, showSecrets)
			default:
				return fmt.Errorf("unknown output format %q", value)
			}

			def, _, err := get.env.esc.client.GetEnvironment(ctx, orgName, envName, showSecrets)
			if err != nil {
				return fmt.Errorf("getting environment definition: %w", err)
			}

			var data *envGetTemplateData
			if len(args) == 0 {
				data, err = get.getEntireEnvironment(ctx, orgName, def, showSecrets)
			} else {
				data, err = get.getEnvironmentMember(ctx, orgName, envName, def, path, showSecrets)
			}
			if err != nil {
				return err
			}
			if data == nil {
				return nil
			}

			var markdown bytes.Buffer
			if err := envGetTemplate.Execute(&markdown, data); err != nil {
				return fmt.Errorf("internal error: rendering: %w", err)
			}

			if !cmdutil.InteractiveTerminal() {
				fmt.Fprint(get.env.esc.stdout, markdown.String())
				return nil
			}

			renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(0))
			if err != nil {
				return fmt.Errorf("internal error: creating renderer: %w", err)
			}
			rendered, err := renderer.Render(markdown.String())
			if err != nil {
				rendered = markdown.String()
			}
			fmt.Fprint(get.env.esc.stdout, rendered)
			return nil
		},
	}

	cmd.Flags().StringVar(
		&value, "value", "",
		"set to print just the value in the given format. may be 'dotenv', 'json', 'detailed', or 'shell'")
	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show static secrets in plaintext rather than ciphertext")

	return cmd
}

func marshalYAML(v any) (string, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func (get *envGetCommand) showValue(
	ctx context.Context,
	orgName string,
	envName string,
	path resource.PropertyPath,
	format string,
	showSecrets bool,
) error {
	def, _, err := get.env.esc.client.GetEnvironment(ctx, orgName, envName, showSecrets)
	if err != nil {
		return fmt.Errorf("getting environment definition: %w", err)
	}
	env, _, err := get.env.esc.client.CheckYAMLEnvironment(ctx, orgName, def)
	if err != nil {
		return fmt.Errorf("getting environment: %w", err)
	}
	return get.env.renderValue(get.env.esc.stdout, env, path, format, true, showSecrets)
}

func (get *envGetCommand) getEntireEnvironment(
	ctx context.Context,
	orgName string,
	def []byte,
	showSecrets bool,
) (*envGetTemplateData, error) {
	var docNode yaml.Node
	if err := yaml.Unmarshal(def, &docNode); err != nil {
		return nil, fmt.Errorf("unmarshaling environment definition: %w", err)
	}
	if docNode.Kind != yaml.DocumentNode {
		return nil, nil
	}

	env, _, err := get.env.esc.client.CheckYAMLEnvironment(ctx, orgName, def)
	if err != nil {
		return nil, fmt.Errorf("getting environment metadata: %w", err)
	}
	if env == nil {
		return nil, nil
	}

	envJSON, err := json.MarshalIndent(esc.NewValue(env.Properties).ToJSON(!showSecrets), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding value: %w", err)
	}

	defYAML, err := marshalYAML(docNode.Content[0])
	if err != nil {
		return nil, fmt.Errorf("marshaling environment definition: %w", err)
	}

	return &envGetTemplateData{
		Value:      string(envJSON),
		Definition: defYAML,
	}, nil
}

func (get *envGetCommand) getEnvironmentMember(
	ctx context.Context,
	orgName string,
	envName string,
	def []byte,
	path resource.PropertyPath,
	showSecrets bool,
) (*envGetTemplateData, error) {
	var docNode yaml.Node
	if err := yaml.Unmarshal(def, &docNode); err != nil {
		return nil, fmt.Errorf("unmarshaling environment definition: %w", err)
	}
	if docNode.Kind != yaml.DocumentNode {
		return nil, nil
	}

	if len(path) != 0 && path[0] == "imports" {
		node, _ := yamlNode{&docNode}.get(path)
		if node == nil {
			return nil, nil
		}
		def, err := marshalYAML(node)
		if err != nil {
			return nil, fmt.Errorf("marshaling definition: %w", err)
		}
		return &envGetTemplateData{Definition: def}, nil
	}

	env, _, err := get.env.esc.client.CheckYAMLEnvironment(ctx, orgName, def)
	if err != nil {
		return nil, fmt.Errorf("getting environment metadata: %w", err)
	}

	value, _ := getEnvValue(esc.NewValue(env.Properties), path)

	var stacker stackable

	valueJSON := ""
	if value != nil {
		stacker = &stackableValue{v: value}

		j, err := json.MarshalIndent(value.ToJSON(!showSecrets), "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encoding value: %w", err)
		}
		valueJSON = string(j)
	}

	definitionYAML := ""
	if valuesNode, ok := (yamlNode{&docNode}.get(resource.PropertyPath{"values"})); ok {
		if node, _ := (yamlNode{valuesNode}.get(path)); node != nil {
			expr, ok := getEnvExpr(esc.Expr{Object: env.Exprs}, path)
			if !ok {
				return nil, fmt.Errorf("internal error: no expr for path %v", path)
			}
			stacker = &stackableExpr{x: expr}

			d, err := marshalYAML(node)
			if err != nil {
				return nil, fmt.Errorf("marshaling definition: %w", err)
			}
			definitionYAML = d
		}
	}

	var stack []string
	if stacker != nil {
		for stacker.Next() {
			rng := stacker.Range()
			env := rng.Environment
			if env == "<yaml>" {
				env = envName
			}
			stack = append(stack, fmt.Sprintf("%v:%v:%v", env, rng.Begin.Line, rng.Begin.Column))
		}
	}

	return &envGetTemplateData{
		Value:      valueJSON,
		Definition: definitionYAML,
		Stack:      stack,
	}, nil
}

type stackable interface {
	Range() esc.Range
	Next() bool
}

type stackableExpr struct {
	x   *esc.Expr
	any bool
}

func (x *stackableExpr) Range() esc.Range {
	return x.x.Range
}

func (x *stackableExpr) Next() bool {
	if x.any {
		x.x = x.x.Base
	}
	x.any = true
	return x.x != nil
}

type stackableValue struct {
	v   *esc.Value
	any bool
}

func (v *stackableValue) Range() esc.Range {
	return v.v.Trace.Def
}

func (v *stackableValue) Next() bool {
	if v.any {
		v.v = v.v.Trace.Base
	}
	v.any = true
	return v.v != nil
}

type envGetTemplateData struct {
	Value      string
	Definition string
	Stack      []string
}

func getEnvExpr(root esc.Expr, path resource.PropertyPath) (*esc.Expr, bool) {
	if len(path) == 0 {
		return &root, true
	}

	switch {
	case root.Builtin != nil:
		key, ok := path[0].(string)
		if !ok {
			return nil, false
		}
		if key != root.Builtin.Name {
			return nil, false
		}
		return getEnvExpr(root.Builtin.Arg, path[1:])
	case root.List != nil:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(root.List) {
			return nil, false
		}
		return getEnvExpr(root.List[index], path[1:])
	case root.Object != nil:
		key, ok := path[0].(string)
		if !ok {
			return nil, false
		}
		v, ok := root.Object[key]
		if !ok {
			return nil, false
		}
		return getEnvExpr(v, path[1:])
	default:
		return nil, false
	}
}

func getEnvValue(root esc.Value, path resource.PropertyPath) (*esc.Value, bool) {
	if len(path) == 0 {
		return &root, true
	}

	switch v := root.Value.(type) {
	case []esc.Value:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(v) {
			return nil, false
		}
		return getEnvValue(v[index], path[1:])
	case map[string]esc.Value:
		key, ok := path[0].(string)
		if !ok {
			return nil, false
		}
		e, ok := v[key]
		if !ok {
			return nil, false
		}
		return getEnvValue(e, path[1:])
	default:
		return nil, false
	}
}
