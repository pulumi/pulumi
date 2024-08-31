package pcl

import (
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func transformTraversal(traversal hcl.Traversal) (*codegen.Traversal, error) {
	result := make([]*codegen.Traverser, len(traversal))
	for i, traverser := range traversal {
		traverser, err := transformTraverser(traverser)
		if err != nil {
			return nil, fmt.Errorf("could not transform traversal: %w", err)
		}
		result[i] = traverser
	}

	return &codegen.Traversal{
		Each: result,
	}, nil
}

func transformTraverser(part hcl.Traverser) (*codegen.Traverser, error) {
	switch part := part.(type) {
	case hcl.TraverseAttr:
		return &codegen.Traverser{
			Value: &codegen.Traverser_TraverseAttr{
				TraverseAttr: &codegen.TraverseAttr{
					Name: part.Name,
				},
			},
		}, nil
	case hcl.TraverseIndex:
		switch part.Key.Type() {
		case cty.Number:
			number, _ := part.Key.AsBigFloat().Float64()
			return &codegen.Traverser{
				Value: &codegen.Traverser_TraverseIndex{
					TraverseIndex: &codegen.TraverseIndex{
						Value: &codegen.TraverseIndex_IntIndex{
							IntIndex: int64(number),
						},
					},
				},
			}, nil
		case cty.String:
			return &codegen.Traverser{
				Value: &codegen.Traverser_TraverseIndex{
					TraverseIndex: &codegen.TraverseIndex{
						Value: &codegen.TraverseIndex_StringIndex{
							StringIndex: part.Key.AsString(),
						},
					},
				},
			}, nil
		default:
			return nil, fmt.Errorf("unknown traverse index type: %v", part.Key.Type())
		}
	case hcl.TraverseRoot:
		return &codegen.Traverser{
			Value: &codegen.Traverser_TraverseRoot{
				TraverseRoot: &codegen.TraverseRoot{
					Name: part.Name,
				},
			},
		}, nil
	case hcl.TraverseSplat:
		{
			each, err := transformTraversal(part.Each)
			if err != nil {
				return nil, fmt.Errorf("could not transform splat traversal: %w", err)
			}
			return &codegen.Traverser{
				Value: &codegen.Traverser_TraverseSplat{
					TraverseSplat: &codegen.TraverseSplat{
						Each: each,
					},
				},
			}, nil
		}
	default:
		return nil, fmt.Errorf("unknown traversal part type: %T", part)
	}
}

func transformFunctionParameters(parameters []*model.Variable) []string {
	result := make([]string, len(parameters))
	for i, parameter := range parameters {
		result[i] = parameter.Name
	}
	return result
}

func formatOperation(operation *hclsyntax.Operation) (codegen.Operation, error) {
	switch operation {
	case hclsyntax.OpAdd:
		return codegen.Operation_ADD, nil
	case hclsyntax.OpDivide:
		return codegen.Operation_DIVIDE, nil
	case hclsyntax.OpEqual:
		return codegen.Operation_EQUAL, nil
	case hclsyntax.OpGreaterThan:
		return codegen.Operation_GREATER_THAN, nil
	case hclsyntax.OpGreaterThanOrEqual:
		return codegen.Operation_GREATER_THAN_OR_EQUAL, nil
	case hclsyntax.OpLessThan:
		return codegen.Operation_LESS_THAN, nil
	case hclsyntax.OpLessThanOrEqual:
		return codegen.Operation_LESS_THAN_OR_EQUAL, nil
	case hclsyntax.OpLogicalAnd:
		return codegen.Operation_LOGICAL_AND, nil
	case hclsyntax.OpLogicalOr:
		return codegen.Operation_LOGICAL_OR, nil
	case hclsyntax.OpModulo:
		return codegen.Operation_MODULO, nil
	case hclsyntax.OpMultiply:
		return codegen.Operation_MULTIPLY, nil
	case hclsyntax.OpNotEqual:
		return codegen.Operation_NOT_EQUAL, nil
	case hclsyntax.OpSubtract:
		return codegen.Operation_SUBTRACT, nil
	default:
		// Cannot return nil - the values are const which cannot be converted to pointers
		return codegen.Operation_ADD, fmt.Errorf("unknown operation type: %v", operation)
	}
}

func transformExpression(expr model.Expression) (*codegen.Expression, error) {
	switch expr := expr.(type) {
	/* TODO: Support enums
	pcl resolves enums into constants on it's own. Must check how it's implemented on other languages
	*/
	case *model.LiteralValueExpression:
		var value *codegen.LiteralValueExpression
		switch expr.Value.Type() {
		case cty.Bool:
			value = &codegen.LiteralValueExpression{
				Value: &codegen.LiteralValueExpression_BoolValue{
					BoolValue: expr.Value.True(),
				},
			}
		case cty.Number:
			number, _ := expr.Value.AsBigFloat().Float64()
			value = &codegen.LiteralValueExpression{
				Value: &codegen.LiteralValueExpression_NumberValue{
					NumberValue: number,
				},
			}
		case cty.String:
			value = &codegen.LiteralValueExpression{
				Value: &codegen.LiteralValueExpression_StringValue{
					StringValue: expr.Value.AsString(),
				},
			}
		default:
			// TODO: Maybe throw error instead? Are we sure this is null?
			value = &codegen.LiteralValueExpression{Value: &codegen.LiteralValueExpression_UnknownValue{UnknownValue: true}}
		}
		return &codegen.Expression{Value: &codegen.Expression_LiteralValueExpression{LiteralValueExpression: value}}, nil

	case *model.TemplateExpression:
		parts := make([]*codegen.Expression, len(expr.Parts))
		for i, part := range expr.Parts {
			transformedPart, err := transformExpression(part)
			if err != nil {
				return nil, err
			}
			parts[i] = transformedPart
		}
		return &codegen.Expression{
			Value: &codegen.Expression_TemplateExpression{
				TemplateExpression: &codegen.TemplateExpression{Parts: parts},
			},
		}, nil

	case *model.IndexExpression:
		collection, err := transformExpression(expr.Collection)
		if err != nil {
			return nil, err
		}
		key, err := transformExpression(expr.Key)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_IndexExpression{
				IndexExpression: &codegen.IndexExpression{Collection: collection, Key: key},
			},
		}, nil

	case *model.ObjectConsExpression:
		properties := make(map[string]*codegen.Expression)
		for _, item := range expr.Items {
			if lit, ok := item.Key.(*model.LiteralValueExpression); ok {
				propertyKey := lit.Value.AsString()
				transformedValue, err := transformExpression(item.Value)
				if err != nil {
					return nil, err
				}
				properties[propertyKey] = transformedValue
			}
		}
		return &codegen.Expression{
			Value: &codegen.Expression_ObjectConsExpression{
				ObjectConsExpression: &codegen.ObjectConsExpression{
					Properties: properties,
				},
			},
		}, nil

	case *model.TupleConsExpression:
		items := make([]*codegen.Expression, len(expr.Expressions))
		for i, item := range expr.Expressions {
			transformedItem, err := transformExpression(item)
			if err != nil {
				return nil, err
			}
			items[i] = transformedItem
		}
		return &codegen.Expression{
			Value: &codegen.Expression_TupleConsExpression{
				TupleConsExpression: &codegen.TupleConsExpression{
					Items: items,
				},
			},
		}, nil

	case *model.FunctionCallExpression:
		args := make([]*codegen.Expression, len(expr.Args))
		for i, arg := range expr.Args {
			transformedArg, err := transformExpression(arg)
			if err != nil {
				return nil, err
			}
			args[i] = transformedArg
		}
		return &codegen.Expression{
			Value: &codegen.Expression_FunctionCallExpression{
				FunctionCallExpression: &codegen.FunctionCallExpression{
					Name: expr.Name,
					Args: args,
				},
			},
		}, nil

	case *model.RelativeTraversalExpression:
		source, err := transformExpression(expr.Source)
		if err != nil {
			return nil, err
		}
		traversal, err := transformTraversal(expr.Traversal)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_RelativeTraversalExpression{
				RelativeTraversalExpression: &codegen.RelativeTraversalExpression{
					Source:    source,
					Traversal: traversal,
				},
			},
		}, nil

	case *model.ScopeTraversalExpression:
		traversal, err := transformTraversal(expr.Traversal)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_ScopeTraversalExpression{
				ScopeTraversalExpression: &codegen.ScopeTraversalExpression{
					RootName:  expr.RootName,
					Traversal: traversal,
				},
			},
		}, nil

	case *model.AnonymousFunctionExpression:
		body, err := transformExpression(expr.Body)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_AnonymousFunctionExpression{
				AnonymousFunctionExpression: &codegen.AnonymousFunctionExpression{
					Parameters: transformFunctionParameters(expr.Parameters),
					Body:       body,
				},
			},
		}, nil

	case *model.ConditionalExpression:
		condition, err := transformExpression(expr.Condition)
		if err != nil {
			return nil, err
		}
		trueExpr, err := transformExpression(expr.TrueResult)
		if err != nil {
			return nil, err
		}
		falseExpr, err := transformExpression(expr.FalseResult)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_ConditionalExpression{
				ConditionalExpression: &codegen.ConditionalExpression{
					Condition: condition,
					TrueExpr:  trueExpr,
					FalseExpr: falseExpr,
				},
			},
		}, nil

	case *model.BinaryOpExpression:
		left, err := transformExpression(expr.LeftOperand)
		if err != nil {
			return nil, err
		}
		right, err := transformExpression(expr.RightOperand)
		if err != nil {
			return nil, err
		}
		operation, err := formatOperation(expr.Operation)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_BinaryOpExpression{
				BinaryOpExpression: &codegen.BinaryOpExpression{
					Left:      left,
					Right:     right,
					Operation: operation,
				},
			},
		}, nil

	case *model.UnaryOpExpression:
		operand, err := transformExpression(expr.Operand)
		if err != nil {
			return nil, err
		}
		operation, err := formatOperation(expr.Operation)
		if err != nil {
			return nil, err
		}
		return &codegen.Expression{
			Value: &codegen.Expression_UnaryOpExpression{
				UnaryOpExpression: &codegen.UnaryOpExpression{
					Operand:   operand,
					Operation: operation,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

func transformResourceOptions(options *ResourceOptions) (*codegen.ResourceOptions, error) {
	optionsProto := &codegen.ResourceOptions{}

	if options.DependsOn != nil {
		dependsOn, err := transformExpression(options.DependsOn)
		if err != nil {
			return nil, err
		}
		optionsProto.DependsOn = dependsOn
	}

	if options.IgnoreChanges != nil {
		ignoreChanges, err := transformExpression(options.IgnoreChanges)
		if err != nil {
			return nil, err
		}
		optionsProto.IgnoreChanges = ignoreChanges
	}

	if options.Parent != nil {
		parent, err := transformExpression(options.Parent)
		if err != nil {
			return nil, err
		}
		optionsProto.Parent = parent
	}

	if options.Protect != nil {
		protect, err := transformExpression(options.Protect)
		if err != nil {
			return nil, err
		}
		optionsProto.Protect = protect
	}

	if options.Provider != nil {
		provider, err := transformExpression(options.Provider)
		if err != nil {
			return nil, err
		}
		optionsProto.Provider = provider
	}

	if options.Version != nil {
		version, err := transformExpression(options.Version)
		if err != nil {
			return nil, err
		}
		optionsProto.Version = version
	}

	return optionsProto, nil
}

func transformResource(resource *Resource) (*codegen.Resource, error) {
	token := resource.Token
	if resource.Schema != nil {
		token = resource.Schema.Token // resource.Token() does not contain "index"
	}
	resourceProto := &codegen.Resource{
		Name:        resource.Name(), // It is deprecated. Should it be removed?
		Token:       token,
		LogicalName: resource.LogicalName(),
	}

	inputs := make([]*codegen.ResourceInput, len(resource.Inputs))
	for i, attr := range resource.Inputs {
		transformedValue, err := transformExpression(attr.Value)
		if err != nil {
			return nil, err
		}
		inputs[i] = &codegen.ResourceInput{
			Name:  attr.Name,
			Value: transformedValue,
		}
	}
	resourceProto.Inputs = inputs

	if resource.Options != nil {
		optionsProto, err := transformResourceOptions(resource.Options)
		if err != nil {
			return nil, err
		}
		resourceProto.Options = optionsProto
	}

	return resourceProto, nil
}

func transformLocalVariable(variable *LocalVariable) (*codegen.LocalVariable, error) {
	value, err := transformExpression(variable.Definition.Value)
	if err != nil {
		return nil, err
	}

	return &codegen.LocalVariable{
		Name:        variable.Name(),
		LogicalName: variable.LogicalName(),
		Value:       value,
	}, nil
}

func transformOutput(output *OutputVariable) (*codegen.OutputVariable, error) {
	value, err := transformExpression(output.Value)
	if err != nil {
		return nil, err
	}

	return &codegen.OutputVariable{
		Name:        output.Name(),
		LogicalName: output.LogicalName(),
		Value:       value,
	}, nil
}

func transformConfigVariable(variable *ConfigVariable) (*codegen.ConfigVariable, error) {
	defaultValue, err := transformExpression(variable.DefaultValue)
	if err != nil {
		return nil, err
	}

	var configType codegen.ConfigType
	switch variable.Type() {
	case model.StringType:
		configType = codegen.ConfigType_STRING
	case model.NumberType:
		configType = codegen.ConfigType_NUMBER
	case model.IntType:
		configType = codegen.ConfigType_INT
	case model.BoolType:
		configType = codegen.ConfigType_BOOL
	default:
		return nil, fmt.Errorf("unknown config variable type: %v", variable.Type())
	}

	return &codegen.ConfigVariable{
		Name:         variable.Name(),
		LogicalName:  variable.LogicalName(),
		ConfigType:   configType,
		DefaultValue: defaultValue,
	}, nil
}

func transformProgram(pclNodes []Node, pclPackages []*schema.Package) (*codegen.PclProtobufProgram, error) {
	nodes := make([]*codegen.Node, len(pclNodes))
	plugins := make([]*codegen.PluginReference, len(pclPackages))

	for i, node := range pclNodes {
		var transformedNode *codegen.Node
		switch node := node.(type) {
		case *Resource:
			transformedResource, err := transformResource(node)
			if err != nil {
				return nil, err
			}
			transformedNode = &codegen.Node{
				Value: &codegen.Node_Resource{Resource: transformedResource},
			}
		case *OutputVariable:
			transformedOutput, err := transformOutput(node)
			if err != nil {
				return nil, err
			}
			transformedNode = &codegen.Node{
				Value: &codegen.Node_OutputVariable{OutputVariable: transformedOutput},
			}
		case *LocalVariable:
			transformedVariable, err := transformLocalVariable(node)
			if err != nil {
				return nil, err
			}
			transformedNode = &codegen.Node{
				Value: &codegen.Node_LocalVariable{LocalVariable: transformedVariable},
			}
		case *ConfigVariable:
			transformedVariable, err := transformConfigVariable(node)
			if err != nil {
				return nil, err
			}
			transformedNode = &codegen.Node{
				Value: &codegen.Node_ConfigVariable{ConfigVariable: transformedVariable},
			}
		default:
			return nil, fmt.Errorf("unknown node type type: %v", node.Type())
		}
		nodes[i] = transformedNode
	}

	for i, pkg := range pclPackages {
		version := ""
		if pkg.Version != nil {
			version = pkg.Version.String()
		}

		pluginRef := &codegen.PluginReference{
			Name:    pkg.Name,
			Version: version,
		}
		plugins[i] = pluginRef
	}

	return &codegen.PclProtobufProgram{
		Nodes:   nodes,
		Plugins: plugins,
	}, nil
}

func generateProtobufProgram(program *Program) (*codegen.PclProtobufProgram, error) {
	MapProvidersAsResources(program)
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := Linearize(program)
	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, err
	}
	serialized, err := transformProgram(nodes, packages)
	if err != nil {
		return nil, err
	}
	return serialized, nil
}

func GenerateJSONProgram(program *Program) (map[string][]byte, hcl.Diagnostics, error) {
	protobuf, err := generateProtobufProgram(program)
	if err != nil {
		return nil, nil, err
	}
	bytes, err := protojson.MarshalOptions{Multiline: true}.Marshal(protobuf)
	if err != nil {
		return nil, nil, err
	}
	return map[string][]byte{"main.pcl.json": bytes}, nil, nil
}

func GenerateSerializedProtobufProgram(program *Program) (map[string][]byte, hcl.Diagnostics, error) {
	protobuf, err := generateProtobufProgram(program)
	if err != nil {
		return nil, nil, err
	}
	out, err := proto.Marshal(protobuf)
	if err != nil {
		return nil, nil, err
	}
	str := base64.StdEncoding.EncodeToString(out)
	return map[string][]byte{"main.pcl.protobuf": []byte(str)}, nil, nil
}
