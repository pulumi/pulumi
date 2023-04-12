// Copyright 2016-2022, Pulumi Corporation.
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

package json

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/zclconf/go-cty/cty"
)

func transformTraversal(traversal hcl.Traversal) []interface{} {
	result := make([]interface{}, 0)
	for _, part := range traversal {
		switch part := part.(type) {
		case hcl.TraverseAttr:
			result = append(result, map[string]interface{}{
				"type": "TraverseAttr",
				"name": part.Name,
			})
		case hcl.TraverseIndex:
			index, _ := part.Key.AsBigFloat().Int64()
			result = append(result, map[string]interface{}{
				"type": "TraverseIndex",
				"key":  index,
			})
		case hcl.TraverseRoot:
			result = append(result, map[string]interface{}{
				"type": "TraverseRoot",
				"name": part.Name,
			})

		case hcl.TraverseSplat:
			result = append(result, map[string]interface{}{
				"type": "TraverseSplat",
				"each": transformTraversal(part.Each),
			})
		}
	}

	return result
}

func tranformFunctionParameters(parameters []*model.Variable) []string {
	result := make([]string, len(parameters))
	for i, parameter := range parameters {
		result[i] = parameter.Name
	}
	return result
}

func formatOperation(operation *hclsyntax.Operation) string {
	switch operation {
	case hclsyntax.OpAdd:
		return "Add"
	case hclsyntax.OpDivide:
		return "Divide"
	case hclsyntax.OpEqual:
		return "Equal"
	case hclsyntax.OpGreaterThan:
		return "GreaterThan"
	case hclsyntax.OpGreaterThanOrEqual:
		return "GreaterThanOrEqual"
	case hclsyntax.OpLessThan:
		return "LessThan"
	case hclsyntax.OpLessThanOrEqual:
		return "LessThanOrEqual"
	case hclsyntax.OpLogicalAnd:
		return "LogicalAnd"
	case hclsyntax.OpLogicalOr:
		return "LogicalOr"
	case hclsyntax.OpModulo:
		return "Modulo"
	case hclsyntax.OpMultiply:
		return "Multiply"
	case hclsyntax.OpNotEqual:
		return "NotEqual"
	case hclsyntax.OpSubtract:
		return "Subtract"
	}

	return ""
}

func transformExpression(expr model.Expression) map[string]interface{} {
	switch expr.(type) {
	case *model.LiteralValueExpression:
		literalExpr := expr.(*model.LiteralValueExpression)
		var value interface{}
		switch literalExpr.Value.Type() {
		case cty.Bool:
			value = literalExpr.Value.True()
		case cty.Number:
			number, _ := literalExpr.Value.AsBigFloat().Float64()
			value = number
		case cty.String:
			value = literalExpr.Value.AsString()
		default:
			value = nil
		}

		return map[string]interface{}{
			"type":  "LiteralValueExpression",
			"value": value,
		}

	case *model.TemplateExpression:
		templateExpression := expr.(*model.TemplateExpression)
		if len(templateExpression.Parts) == 1 {
			return transformExpression(templateExpression.Parts[0])
		}

		parts := make([]interface{}, len(templateExpression.Parts))
		for i, part := range templateExpression.Parts {
			parts[i] = transformExpression(part)
		}
		return map[string]interface{}{
			"type":  "TemplateExpression",
			"parts": parts,
		}

	case *model.IndexExpression:
		indexExpr := expr.(*model.IndexExpression)
		return map[string]interface{}{
			"type":       "IndexExpression",
			"collection": transformExpression(indexExpr.Collection),
			"key":        transformExpression(indexExpr.Key),
		}

	case *model.ObjectConsExpression:
		objectExpr := expr.(*model.ObjectConsExpression)
		properties := make(map[string]interface{})
		for _, item := range objectExpr.Items {
			if lit, ok := item.Key.(*model.LiteralValueExpression); ok {
				propertyKey := lit.Value.AsString()
				properties[propertyKey] = transformExpression(item.Value)
			}
		}
		return map[string]interface{}{
			"type":       "ObjectConsExpression",
			"properties": properties,
		}

	case *model.TupleConsExpression:
		tupleExpr := expr.(*model.TupleConsExpression)
		items := make([]interface{}, len(tupleExpr.Expressions))
		for i, item := range tupleExpr.Expressions {
			items[i] = transformExpression(item)
		}
		return map[string]interface{}{
			"type":  "TupleConsExpression",
			"items": items,
		}

	case *model.FunctionCallExpression:
		funcExpr := expr.(*model.FunctionCallExpression)
		args := make([]interface{}, len(funcExpr.Args))
		for i, arg := range funcExpr.Args {
			args[i] = transformExpression(arg)
		}
		return map[string]interface{}{
			"type": "FunctionCallExpression",
			"name": funcExpr.Name,
			"args": args,
		}

	case *model.RelativeTraversalExpression:
		traversalExpr := expr.(*model.RelativeTraversalExpression)
		return map[string]interface{}{
			"type":      "RelativeTraversalExpression",
			"source":    transformExpression(traversalExpr.Source),
			"traversal": transformTraversal(traversalExpr.Traversal),
		}

	case *model.ScopeTraversalExpression:
		traversalExpr := expr.(*model.ScopeTraversalExpression)
		return map[string]interface{}{
			"type":      "ScopeTraversalExpression",
			"rootName":  traversalExpr.RootName,
			"traversal": transformTraversal(traversalExpr.Traversal),
		}

	case *model.AnonymousFunctionExpression:
		funcExpr := expr.(*model.AnonymousFunctionExpression)
		return map[string]interface{}{
			"type":       "AnonymousFunctionExpression",
			"parameters": tranformFunctionParameters(funcExpr.Parameters),
			"body":       transformExpression(funcExpr.Body),
		}

	case *model.ConditionalExpression:
		condExpr := expr.(*model.ConditionalExpression)
		return map[string]interface{}{
			"type":      "ConditionalExpression",
			"condition": transformExpression(condExpr.Condition),
			"trueExpr":  transformExpression(condExpr.TrueResult),
			"falseExpr": transformExpression(condExpr.FalseResult),
		}

	case *model.BinaryOpExpression:
		binaryExpr := expr.(*model.BinaryOpExpression)
		return map[string]interface{}{
			"type":      "BinaryOpExpression",
			"operation": formatOperation(binaryExpr.Operation),
			"left":      transformExpression(binaryExpr.LeftOperand),
			"right":     transformExpression(binaryExpr.RightOperand),
		}

	case *model.UnaryOpExpression:
		unaryExpr := expr.(*model.UnaryOpExpression)
		return map[string]interface{}{
			"type":      "UnaryOpExpression",
			"operation": formatOperation(unaryExpr.Operation),
			"operand":   transformExpression(unaryExpr.Operand),
		}

	default:
		return nil
	}
}

func transformResourceOptions(options *pcl.ResourceOptions) map[string]interface{} {
	optionsJSON := make(map[string]interface{})
	if options.DependsOn != nil {
		optionsJSON["dependsOn"] = transformExpression(options.DependsOn)
	}
	if options.IgnoreChanges != nil {
		optionsJSON["ignoreChanges"] = transformExpression(options.IgnoreChanges)
	}
	if options.Parent != nil {
		optionsJSON["parent"] = transformExpression(options.Parent)
	}
	if options.Protect != nil {
		optionsJSON["protect"] = transformExpression(options.Protect)
	}
	if options.Provider != nil {
		optionsJSON["provider"] = transformExpression(options.Provider)
	}
	if options.Version != nil {
		optionsJSON["version"] = transformExpression(options.Version)
	}
	return optionsJSON
}

func transformResource(resource *pcl.Resource) map[string]interface{} {
	resourceJSON := make(map[string]interface{})
	resourceJSON["type"] = "Resource"
	resourceJSON["name"] = resource.Name()
	resourceJSON["token"] = resource.Token
	resourceJSON["logicalName"] = resource.LogicalName()
	inputs := make(map[string]interface{})
	for _, attr := range resource.Inputs {
		inputs[attr.Name] = transformExpression(attr.Value)
	}
	resourceJSON["inputs"] = inputs
	if resource.Options != nil {
		resourceJSON["options"] = transformResourceOptions(resource.Options)
	}

	return resourceJSON
}

func transformLocalVariable(variable *pcl.LocalVariable) map[string]interface{} {
	variableJSON := make(map[string]interface{})
	variableJSON["type"] = "LocalVariable"
	variableJSON["name"] = variable.Name()
	variableJSON["logicalName"] = variable.LogicalName()
	variableJSON["value"] = transformExpression(variable.Definition.Value)
	return variableJSON
}

func transformOutput(output *pcl.OutputVariable) map[string]interface{} {
	outputJSON := make(map[string]interface{})
	outputJSON["type"] = "OutputVariable"
	outputJSON["name"] = output.Name()
	outputJSON["logicalName"] = output.LogicalName()
	outputJSON["value"] = transformExpression(output.Value)
	return outputJSON
}

func transformConfigVariable(variable *pcl.ConfigVariable) map[string]interface{} {
	variableJSON := make(map[string]interface{})
	variableJSON["type"] = "ConfigVariable"
	variableJSON["defaultValue"] = transformExpression(variable.DefaultValue)
	variableJSON["name"] = variable.Name()
	variableJSON["logicalName"] = variable.LogicalName()
	switch variable.Type() {
	case model.StringType:
		variableJSON["configType"] = "string"
	case model.NumberType:
		variableJSON["configType"] = "number"
	case model.IntType:
		variableJSON["configType"] = "int"
	case model.BoolType:
		variableJSON["configType"] = "bool"
	default:
		variableJSON["configType"] = "unknown"
	}

	return variableJSON
}

func transformProgram(program *pcl.Program) map[string]interface{} {
	programJSON := make(map[string]interface{})
	nodes := make([]interface{}, 0, len(program.Nodes))
	plugins := make([]interface{}, 0, len(program.Packages()))
	for _, node := range program.Nodes {
		switch node := node.(type) {
		case *pcl.Resource:
			transformedResource := transformResource(node)
			nodes = append(nodes, transformedResource)
		case *pcl.OutputVariable:
			transformedOutput := transformOutput(node)
			nodes = append(nodes, transformedOutput)
		case *pcl.LocalVariable:
			tranformedVariable := transformLocalVariable(node)
			nodes = append(nodes, tranformedVariable)
		case *pcl.ConfigVariable:
			tranformedVariable := transformConfigVariable(node)
			nodes = append(nodes, tranformedVariable)
		}
	}

	for _, pkg := range program.Packages() {
		pluginRef := map[string]interface{}{
			"name":    pkg.Name,
			"version": pkg.Version.String(),
		}

		plugins = append(plugins, pluginRef)
	}

	programJSON["nodes"] = nodes
	programJSON["plugins"] = plugins
	return programJSON
}

//go:embed pcl-json-schema.json
var pclJSONSchema string

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	files := make(map[string][]byte)
	diagnostics := hcl.Diagnostics{}
	programJSON := transformProgram(program)
	schemaCompiler := jsonschema.NewCompiler()
	schemaCompiler.LoadURL = func(_ string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(pclJSONSchema)), nil
	}

	schema := schemaCompiler.MustCompile("blob://pcl-json-schema.json")

	err := schema.Validate(programJSON)

	if err != nil {
		return nil, nil, fmt.Errorf("generated PCL program as JSON did not conform to the schema: %w", err)
	}

	programBytes, err := json.MarshalIndent(programJSON, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("could not marshal program to JSON: %w", err)
	}

	files["program.json"] = programBytes
	return files, diagnostics, nil
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program) error {
	files, diagnostics, err := GenerateProgram(program)
	if err != nil {
		return err
	}
	if diagnostics.HasErrors() {
		return diagnostics
	}

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := ioutil.WriteFile(outPath, data, 0600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
}
