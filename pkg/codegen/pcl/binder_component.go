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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

// componentVariableType returns the type of the variable of which the value is a component.
// The type is derived from the outputs of the sub-program of the component into an ObjectType
func componentVariableType(program *Program) model.Type {
	properties := map[string]model.Type{}
	for _, node := range program.Nodes {
		switch node := node.(type) {
		case *OutputVariable:
			switch nodeType := node.Type().(type) {
			case *model.OutputType:
				// if the output variable is already an Output<T>, keep it as is
				properties[node.LogicalName()] = nodeType
			default:
				// otherwise, wrap it as an output
				properties[node.LogicalName()] = &model.OutputType{
					ElementType: nodeType,
				}
			}
		}
	}

	return &model.ObjectType{Properties: properties}
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

		parser := hclsyntax.NewParser()
		// Load all .pp files in the components' directory
		files, err := os.ReadDir(componentSourceDir)
		if err != nil {
			diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
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
					diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
					return nil, diagnostics, err
				}

				err = parser.ParseFile(file, fileName)

				if err != nil {
					diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
					return nil, diagnostics, err
				}

				diags := parser.Diagnostics
				if diags.HasErrors() {
					return nil, diagnostics, err
				}
			}
		}

		componentProgram, programDiags, err := BindProgram(parser.Files,
			Loader(loader),
			DirPath(componentSourceDir),
			ComponentBinder(ComponentProgramBinderFromFileSystem()))

		return componentProgram, programDiags, err
	}
}

func (b *binder) bindComponent(node *Component) hcl.Diagnostics {
	block, diagnostics := model.BindBlock(node.syntax, model.StaticScope(b.root), b.tokens, b.options.modelOptions()...)
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

	componentProgram, programDiags, err := b.options.componentProgramBinder(ComponentProgramBinderArgs{
		BinderLoader:       b.options.loader,
		BinderDirPath:      b.options.dirPath,
		ComponentSource:    node.source,
		ComponentNodeRange: node.SyntaxNode().Range(),
	})
	if err != nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(), err.Error()))
		node.VariableType = model.DynamicType
		return diagnostics
	}

	if programDiags.HasErrors() || componentProgram == nil {
		diagnostics = diagnostics.Append(errorf(node.SyntaxNode().Range(), programDiags.Error()))
		node.VariableType = model.DynamicType
		return diagnostics
	}

	node.Program = componentProgram
	node.VariableType = componentVariableType(componentProgram)
	node.dirPath = filepath.Join(b.options.dirPath, node.source)

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
