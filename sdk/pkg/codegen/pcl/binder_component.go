// Copyright 2016-2020, Pulumi Corporation.
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

package pcl

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	syntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// componentVariableType returns the type of the variable of which the value is a component.
// The type is derived from the outputs of the sub-program of the component into an ObjectType
func componentVariableType(program *Program) model.Type {
	properties := map[string]model.Type{}
	for _, node := range program.Nodes {
		switch node := node.(type) {
		case *OutputVariable:
			if node.Value == nil {
				continue
			}

			switch nodeType := node.Value.Type().(type) {
			case *model.OutputType:
				// if the output variable is already an Output<T>, keep it as is
				properties[node.LogicalName()] = nodeType
			default:
				// otherwise, wrap it as an output
				properties[node.LogicalName()] = model.NewOutputType(nodeType)
			}
		}
	}

	return model.NewObjectType(properties)
}

type componentScopes struct {
	root      *model.Scope
	withRange *model.Scope
	component *Component
}

func newComponentScopes(
	root *model.Scope,
	component *Component,
	rangeKeyType model.Type,
	rangeValueType model.Type,
) model.Scopes {
	scopes := &componentScopes{
		root:      root,
		withRange: root,
		component: component,
	}

	if rangeValueType != nil {
		properties := map[string]model.Type{
			"value": rangeValueType,
		}
		if rangeKeyType != nil {
			properties["key"] = rangeKeyType
		}

		scopes.withRange = root.Push(syntax.None)
		scopes.withRange.Define("range", &model.Variable{
			Name:         "range",
			VariableType: model.NewObjectType(properties),
		})
	}
	return scopes
}

func (s *componentScopes) GetScopesForBlock(block *hclsyntax.Block) (model.Scopes, hcl.Diagnostics) {
	return model.StaticScope(s.withRange), nil
}

func (s *componentScopes) GetScopeForAttribute(attr *hclsyntax.Attribute) (*model.Scope, hcl.Diagnostics) {
	return s.withRange, nil
}

type componentInput struct {
	key      string
	required bool
}

func componentInputs(program *Program) map[string]componentInput {
	inputs := map[string]componentInput{}
	for _, node := range program.Nodes {
		switch node := node.(type) {
		case *ConfigVariable:
			inputs[node.LogicalName()] = componentInput{
				required: node.DefaultValue == nil && !node.Nullable,
				key:      node.LogicalName(),
			}
		}
	}

	return inputs
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

// ComponentProgramBinderFromFileSystem returns the default component program binder which uses the file system
// to parse and bind PCL files into a program.
func ComponentProgramBinderFromFileSystem() ComponentProgramBinder {
	return func(args ComponentProgramBinderArgs) (*Program, hcl.Diagnostics, error) {
		var diagnostics hcl.Diagnostics
		binderDirPath := args.BinderDirPath
		componentSource := args.ComponentSource
		nodeRange := args.ComponentNodeRange
		loader := args.BinderLoader
		// bind the component here as if it was a new program
		// this becomes the DirPath for the new binder
		componentSourceDir := filepath.Join(binderDirPath, componentSource)

		parser := syntax.NewParser()
		// Load all .pp files in the components' directory
		files, err := os.ReadDir(componentSourceDir)
		if err != nil {
			diagnostics = diagnostics.Append(errorf(nodeRange, "%s", err.Error()))
			return nil, diagnostics, nil
		}

		if len(files) == 0 {
			diagnostics = diagnostics.Append(errorf(nodeRange, "no files found"))
			return nil, diagnostics, nil
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fileName := file.Name()
			path := filepath.Join(componentSourceDir, fileName)

			if filepath.Ext(fileName) == ".pp" {
				file, err := os.Open(path)
				if err != nil {
					diagnostics = diagnostics.Append(errorf(nodeRange, "%s", err.Error()))
					return nil, diagnostics, err
				}

				err = parser.ParseFile(file, fileName)
				contract.IgnoreError(file.Close())
				if err != nil {
					diagnostics = diagnostics.Append(errorf(nodeRange, "%s", err.Error()))
					return nil, diagnostics, err
				}

				diags := parser.Diagnostics
				if diags.HasErrors() {
					return nil, diagnostics, err
				}
			}
		}

		opts := []BindOption{
			Loader(loader),
			DirPath(componentSourceDir),
			ComponentBinder(ComponentProgramBinderFromFileSystem()),
			Cache(args.PackageCache),
		}

		if args.AllowMissingVariables {
			opts = append(opts, AllowMissingVariables)
		}
		if args.AllowMissingProperties {
			opts = append(opts, AllowMissingProperties)
		}
		if args.SkipResourceTypecheck {
			opts = append(opts, SkipResourceTypechecking)
		}
		if args.PreferOutputVersionedInvokes {
			opts = append(opts, PreferOutputVersionedInvokes)
		}
		if args.SkipInvokeTypecheck {
			opts = append(opts, SkipInvokeTypechecking)
		}
		if args.SkipRangeTypecheck {
			opts = append(opts, SkipRangeTypechecking)
		}

		componentProgram, programDiags, err := BindProgram(parser.Files, opts...)

		includeSourceDirectoryInDiagnostics(programDiags, componentSourceDir)

		return componentProgram, programDiags, err
	}
}

// Replaces the Subject of the input diagnostics from just a file name to the full path of the component directory.
func includeSourceDirectoryInDiagnostics(diags hcl.Diagnostics, componentSourceDir string) {
	for _, diag := range diags {
		start := hcl.Pos{}
		end := hcl.Pos{}

		if diag.Subject != nil {
			start = diag.Subject.Start
			end = diag.Subject.End
			componentSourceDir = filepath.Join(componentSourceDir, diag.Subject.Filename)
		}

		diag.Subject = &hcl.Range{
			Filename: componentSourceDir,
			Start:    start,
			End:      end,
		}
	}
}

func (b *binder) bindComponent(node *Component) hcl.Diagnostics {
	// When options { range = <expr> } is present
	// We create a new scope for binding the component.
	// This range expression is potentially a key/value object
	// So here we compute the types for the key and value property of that object
	var rangeKeyType, rangeValueType model.Type

	transformComponentType := func(variableType model.Type) model.Type {
		return variableType
	}

	for _, block := range node.syntax.Body.Blocks {
		if block.Type == "options" {
			if rng, hasRange := block.Body.Attributes["range"]; hasRange {
				expr, _ := model.BindExpression(rng.Expr, b.root, b.tokens, b.options.modelOptions()...)
				typ := model.ResolveOutputs(expr.Type())

				switch {
				case model.InputType(model.BoolType).ConversionFrom(typ) == model.SafeConversion:
					// if range expression has a boolean type
					// then variable type T of the component becomes Option<T>
					transformComponentType = func(variableType model.Type) model.Type {
						return model.NewOptionalType(variableType)
					}
				case model.InputType(model.NumberType).ConversionFrom(typ) == model.SafeConversion:
					// if the range expression has a numeric type
					// then value of the iteration is a number
					// and the variable type T of the component becomes List<T>
					rangeValueType = model.IntType
					transformComponentType = func(variableType model.Type) model.Type {
						return model.NewListType(variableType)
					}
				default:
					// for any other generic type iterations
					// we compute the range key and range value types
					// and the variable type T of the component becomes List<T>
					strict := !b.options.skipRangeTypecheck
					rangeKeyType, rangeValueType, _ = model.GetCollectionTypes(typ, rng.Range(), strict)
					transformComponentType = func(variableType model.Type) model.Type {
						return model.NewListType(variableType)
					}
				}
			}
		}
	}

	scopes := newComponentScopes(b.root, node, rangeKeyType, rangeValueType)

	block, diagnostics := model.BindBlock(node.syntax, scopes, b.tokens, b.options.modelOptions()...)
	node.Definition = block

	// check we can use components and load the program
	if b.options.dirPath == "" {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(),
			"components require the binder to have set a directory path"))
		return diagnostics
	}

	if b.options.componentProgramBinder == nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(),
			"components require the binder to have set the component program binder"))
		return diagnostics
	}

	componentDirPath := filepath.Join(b.options.dirPath, node.source)
	absoluteBinderPath, err := filepath.Abs(b.options.dirPath)
	if err != nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(),
			"failed to get absolute path for binder directory %s: %s", b.options.dirPath, err.Error()))
		return diagnostics
	}

	absoluteComponentPath, err := filepath.Abs(componentDirPath)
	if err != nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(),
			"failed to get absolute path for component directory %s: %s", componentDirPath, err.Error()))
		return diagnostics
	}

	if absoluteBinderPath == absoluteComponentPath {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(),
			"cannot bind component %s from the same directory as the parent program", node.Name()))
		return diagnostics
	}

	componentProgram, programDiags, err := b.options.componentProgramBinder(ComponentProgramBinderArgs{
		AllowMissingVariables:        b.options.allowMissingVariables,
		AllowMissingProperties:       b.options.allowMissingProperties,
		SkipResourceTypecheck:        b.options.skipResourceTypecheck,
		SkipInvokeTypecheck:          b.options.skipInvokeTypecheck,
		SkipRangeTypecheck:           b.options.skipRangeTypecheck,
		PreferOutputVersionedInvokes: b.options.preferOutputVersionedInvokes,
		BinderLoader:                 b.options.loader,
		BinderDirPath:                b.options.dirPath,
		PackageCache:                 b.options.packageCache,
		ComponentSource:              node.source,
		ComponentNodeRange:           node.SyntaxNode().Range(),
	})
	if err != nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(), "%s", err.Error()))
		node.VariableType = model.DynamicType
		return diagnostics
	}

	if programDiags.HasErrors() || componentProgram == nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(), "%s", programDiags.Error()))
		node.VariableType = model.DynamicType
		return diagnostics
	}

	node.Program = componentProgram
	programVariableType := componentVariableType(componentProgram)
	node.VariableType = transformComponentType(programVariableType)
	node.dirPath = componentDirPath

	componentInputs := componentInputs(componentProgram)
	providedInputs := []string{}

	var options *model.Block
	for _, item := range block.Body.Items {
		switch item := item.(type) {
		case *model.Attribute:
			// logical name is a special attribute
			if item.Name == LogicalNamePropertyKey {
				logicalName, lDiags := getStringAttrValue(item)
				if lDiags != nil {
					diagnostics = diagnostics.Append(lDiags)
				} else {
					node.logicalName = logicalName
				}
				continue
			}
			// all other attributes are part of the inputs
			_, knownInput := componentInputs[item.Name]

			if !knownInput {
				diagnostics = append(diagnostics, unsupportedAttribute(item.Name, item.Syntax.NameRange))
				return diagnostics
			}

			node.Inputs = append(node.Inputs, item)
			providedInputs = append(providedInputs, item.Name)
		case *model.Block:
			switch item.Type {
			case "options":
				if options != nil {
					diagnostics = append(diagnostics, duplicateBlock(item.Type, item.Syntax.TypeRange))
				} else {
					options = item
				}
			default:
				diagnostics = append(diagnostics, unsupportedBlock(item.Type, item.Syntax.TypeRange))
			}
		}
	}

	// check that all required inputs are actually set
	for inputKey, componentInput := range componentInputs {
		if componentInput.required && !contains(providedInputs, inputKey) {
			diagnostics = append(diagnostics, missingRequiredAttribute(inputKey, node.SyntaxNode().Range()))
		}
	}

	if options != nil {
		resourceOptions, optionsDiags := bindResourceOptions(options)
		diagnostics = append(diagnostics, optionsDiags...)
		node.Options = resourceOptions
	}

	return diagnostics
}
