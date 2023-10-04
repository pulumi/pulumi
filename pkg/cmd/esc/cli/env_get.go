// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type envGetCommand struct {
	env *envCommand
}

func newEnvGetCmd(env *envCommand) *cobra.Command {
	var explain bool

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

			def, _, err := get.env.esc.client.GetEnvironment(ctx, orgName, envName)
			if err != nil {
				return fmt.Errorf("getting environment definition: %w", err)
			}

			if len(args) == 0 {
				env, _, err := get.env.esc.client.CheckYAMLEnvironment(ctx, orgName, def)
				if err != nil {
					return fmt.Errorf("getting environment metadata: %w", err)
				}
				enc := json.NewEncoder(get.env.esc.stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(esc.NewValue(env.Properties).ToJSON(true))
			}

			var docNode yaml.Node
			if err := yaml.Unmarshal(def, &docNode); err != nil {
				return fmt.Errorf("unmarshaling environment definition: %w", err)
			}
			if docNode.Kind != yaml.DocumentNode {
				return nil
			}

			if len(path) != 0 && path[0] == "imports" {
				node, _ := yamlNode{&docNode}.get(path)
				if node == nil {
					return nil
				}
				enc := yaml.NewEncoder(get.env.esc.stdout)
				return enc.Encode(node)
			}

			env, _, err := get.env.esc.client.CheckYAMLEnvironment(ctx, orgName, def)
			if err != nil {
				return fmt.Errorf("getting environment metadata: %w", err)
			}

			value, ok := getEnvValue(esc.NewValue(env.Properties), path)
			if !ok {
				return nil
			}

			schema := getEnvSchema(env.Schema, path)

			if valuesNode, ok := (yamlNode{&docNode}.get(resource.PropertyPath{"values"})); ok {
				if node, _ := (yamlNode{valuesNode}.get(path)); node != nil {
					expr, ok := getEnvExpr(esc.Expr{Object: env.Exprs}, path)
					if !ok {
						return fmt.Errorf("internal error: no expr for path %v", path)
					}
					return get.showEnvSyntax(value, expr, schema, node, explain)
				}
			}
			return get.showEnvValue(value, schema, explain)
		},
	}

	cmd.Flags().BoolVarP(
		&explain, "explain", "x", false,
		"true to show detailed information about the value")

	return cmd
}

func (cmd *envGetCommand) showEnvSyntax(
	value *esc.Value,
	expr *esc.Expr,
	schema *schema.Schema,
	node *yaml.Node,
	explain bool,
) error {
	enc := json.NewEncoder(cmd.env.esc.stdout)
	enc.SetIndent("", "  ")

	if !explain {
		return enc.Encode(value.ToJSON(true))
	}

	fmt.Fprintln(cmd.env.esc.stdout, "VALUE:")
	if err := enc.Encode(value.ToJSON(true)); err != nil {
		return err
	}
	fmt.Fprintln(cmd.env.esc.stdout)

	fmt.Fprintln(cmd.env.esc.stdout, "DEFINITION:")
	yamlEnc := yaml.NewEncoder(cmd.env.esc.stdout)
	if err := yamlEnc.Encode(node); err != nil {
		return err
	}
	fmt.Fprintln(cmd.env.esc.stdout)

	fmt.Fprintln(cmd.env.esc.stdout, "STACK:")
	for expr != nil {
		rng := expr.Range
		fmt.Fprintf(cmd.env.esc.stdout, "- %v:%v:%v\n", rng.Environment, rng.Begin.Line, rng.Begin.Column)
		expr = expr.Base
	}
	return nil
}

func (cmd *envGetCommand) showEnvValue(value *esc.Value, schema *schema.Schema, explain bool) error {
	enc := json.NewEncoder(cmd.env.esc.stdout)
	enc.SetIndent("", "  ")

	if !explain {
		return enc.Encode(value.ToJSON(true))
	}

	fmt.Fprintln(cmd.env.esc.stdout, "VALUE:")
	if err := enc.Encode(value.ToJSON(true)); err != nil {
		return err
	}
	fmt.Fprintln(cmd.env.esc.stdout)

	fmt.Fprintln(cmd.env.esc.stdout, "STACK:")
	for value != nil {
		rng := value.Trace.Def
		fmt.Fprintf(cmd.env.esc.stdout, "- %v:%v:%v\n", rng.Environment, rng.Begin.Line, rng.Begin.Column)
		value = value.Trace.Base
	}
	return nil
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

func getEnvSchema(root *schema.Schema, path resource.PropertyPath) *schema.Schema {
	if len(path) == 0 {
		return root
	}

	switch accessor := path[0].(type) {
	case int:
		return getEnvSchema(root.Item(accessor), path[1:])
	case string:
		return getEnvSchema(root.Property(accessor), path[1:])
	default:
		contract.Failf("unexpected accessor type %T", accessor)
		return schema.Never()
	}
}
