// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docs

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type constructorSyntaxGenerator struct {
	indentSize             int
	requiredPropertiesOnly bool
}

type generateAllOptions struct {
	includeResources bool
	includeFunctions bool
	resourcesToSkip  []string
}

func (g *constructorSyntaxGenerator) indented(f func()) {
	g.indentSize += 2
	f()
	g.indentSize -= 2
}

func (g *constructorSyntaxGenerator) indent(buffer *bytes.Buffer) {
	buffer.WriteString(strings.Repeat(" ", g.indentSize))
}

func (g *constructorSyntaxGenerator) write(buffer *bytes.Buffer, format string, args ...interface{}) {
	buffer.WriteString(fmt.Sprintf(format, args...))
}

func (g *constructorSyntaxGenerator) writeValue(
	buffer *bytes.Buffer,
	valueType schema.Type,
	seenTypes codegen.StringSet,
) {
	write := func(format string, args ...interface{}) {
		g.write(buffer, format, args...)
	}

	writeValue := func(valueType schema.Type) {
		g.writeValue(buffer, valueType, seenTypes)
	}

	switch valueType {
	case schema.AnyType:
		write("\"any\"")
	case schema.JSONType:
		write("\"{}\"")
	case schema.BoolType:
		write("false")
	case schema.IntType:
		write("0")
	case schema.NumberType:
		write("0.0")
	case schema.StringType:
		write("\"string\"")
	case schema.ArchiveType:
		write("fileArchive(\"./path/to/archive\")")
	case schema.AssetType:
		write("stringAsset(\"content\")")
	}

	switch valueType := valueType.(type) {
	case *schema.ArrayType:
		write("[")
		if !isResourceType(valueType.ElementType) {
			writeValue(valueType.ElementType)
		}
		write("]")
	case *schema.MapType:
		write("{\n")
		g.indented(func() {
			g.indent(buffer)
			if !isResourceType(valueType.ElementType) {
				write("\"string\" = ")
				writeValue(valueType.ElementType)
				write("\n")
			}
		})
		g.indent(buffer)
		write("}")
	case *schema.ObjectType:
		if seenTypes.Has(valueType.Token) && objectTypeHasRecursiveReference(valueType) {
			write("notImplemented(%q)", valueType.Token)
			return
		}

		seenTypes.Add(valueType.Token)
		write("{\n")
		g.indented(func() {
			sortPropertiesByRequiredFirst(valueType.Properties)
			for _, p := range valueType.Properties {
				if p.DeprecationMessage != "" {
					continue
				}

				if g.requiredPropertiesOnly && !p.IsRequired() {
					continue
				}

				if isResourceType(p.Type) {
					// skip properties that have type resource
					continue
				}

				g.indent(buffer)
				write("%s = ", p.Name)
				if p.ConstValue != nil {
					// constant values used for discriminator properties of object need to be exact
					// we write them out here as strings to generate correct programs
					if stringValue, ok := p.ConstValue.(string); ok {
						write("%q", stringValue)
					} else {
						writeValue(p.Type)
					}
				} else {
					writeValue(p.Type)
				}

				write("\n")
			}
		})
		g.indent(buffer)
		write("}")
	case *schema.ResourceType:
		write("notImplemented(%q)", valueType.Token)
	case *schema.EnumType:
		cases := make([]string, len(valueType.Elements))
		for index, c := range valueType.Elements {
			if c.DeprecationMessage != "" {
				continue
			}

			if stringCase, ok := c.Value.(string); ok && stringCase != "" {
				cases[index] = stringCase
			} else if intCase, ok := c.Value.(int); ok {
				cases[index] = strconv.Itoa(intCase)
			} else {
				if c.Name != "" {
					cases[index] = c.Name
				}
			}
		}

		if len(cases) > 0 {
			write(fmt.Sprintf("%q", cases[0]))
		} else {
			write("null")
		}
	case *schema.UnionType:
		if isUnionOfObjects(valueType) && len(valueType.ElementTypes) >= 1 {
			writeValue(valueType.ElementTypes[0])
			return
		}

		for _, elem := range valueType.ElementTypes {
			if isPrimitiveType(elem) {
				writeValue(elem)
				return
			}
		}
		write("null")

	case *schema.InputType:
		writeValue(valueType.ElementType)
	case *schema.OptionalType:
		writeValue(valueType.ElementType)
	case *schema.TokenType:
		writeValue(valueType.UnderlyingType)
	}
}

func (g *constructorSyntaxGenerator) exampleResourceWithName(r *schema.Resource, name func(string) string) string {
	buffer := bytes.Buffer{}
	seenTypes := codegen.NewStringSet()
	resourceName := name(r.Token)
	g.write(&buffer, "resource \"%s\" %q {\n", resourceName, r.Token)
	g.indented(func() {
		sortPropertiesByRequiredFirst(r.InputProperties)
		for _, p := range r.InputProperties {
			if p.DeprecationMessage != "" {
				continue
			}

			if g.requiredPropertiesOnly && !p.IsRequired() {
				continue
			}

			g.indent(&buffer)
			g.write(&buffer, "%s = ", p.Name)
			g.writeValue(&buffer, codegen.ResolvedType(p.Type), seenTypes)
			g.write(&buffer, "\n")
		}
	})

	g.write(&buffer, "}")
	return buffer.String()
}

func (g *constructorSyntaxGenerator) exampleInvokeWithName(function *schema.Function, name func(string) string) string {
	buffer := bytes.Buffer{}
	seenTypes := codegen.NewStringSet()
	functionName := name(function.Token)
	g.write(&buffer, "%s = invoke(\"%s\", {\n", functionName, function.Token)
	g.indented(func() {
		if function.Inputs == nil {
			return
		}

		sortPropertiesByRequiredFirst(function.Inputs.Properties)
		for _, p := range function.Inputs.Properties {
			if p.DeprecationMessage != "" {
				continue
			}

			if g.requiredPropertiesOnly && !p.IsRequired() {
				continue
			}

			g.indent(&buffer)
			g.write(&buffer, "%s = ", p.Name)
			g.writeValue(&buffer, codegen.ResolvedType(p.Type), seenTypes)
			g.write(&buffer, "\n")
		}
	})

	g.write(&buffer, "})")
	return buffer.String()
}

func (g *constructorSyntaxGenerator) bindProgram(loader schema.ReferenceLoader, program string) (*pcl.Program, error) {
	parser := syntax.NewParser()
	err := parser.ParseFile(bytes.NewReader([]byte(program)), "main.pp")
	if err != nil {
		return nil, fmt.Errorf("could not parse program: %w", err)
	}

	if parser.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("failed to parse program: %w", parser.Diagnostics)
	}

	bindOptions := make([]pcl.BindOption, 0)
	bindOptions = append(bindOptions, pcl.Loader(loader))
	bindOptions = append(bindOptions, pcl.NonStrictBindOptions()...)
	boundProgram, diagnostics, err := pcl.BindProgram(parser.Files, bindOptions...)
	if err != nil {
		return nil, fmt.Errorf("could not bind program: %w", err)
	}

	if diagnostics.HasErrors() {
		return nil, fmt.Errorf("failed to bind program: %w", diagnostics)
	}

	return boundProgram, nil
}

func camelCase(s string) string {
	return cgstrings.Camel(s)
}

func isResourceType(t schema.Type) bool {
	if optional, ok := t.(*schema.OptionalType); ok {
		return isResourceType(optional.ElementType)
	}
	if inputType, ok := t.(*schema.InputType); ok {
		return isResourceType(inputType.ElementType)
	}
	if tokenType, isTokenType := t.(*schema.TokenType); isTokenType {
		return isResourceType(tokenType.UnderlyingType)
	}
	_, isResource := t.(*schema.ResourceType)
	return isResource
}

func cleanModuleName(moduleName string) string {
	return strings.ReplaceAll(moduleName, "/", "")
}

func (g *constructorSyntaxGenerator) generateAll(schema *schema.Package, opts generateAllOptions) string {
	buffer := bytes.Buffer{}
	seenNames := codegen.NewStringSet()
	resourcesToSkip := codegen.NewStringSet(opts.resourcesToSkip...)
	if opts.includeResources {
		for _, r := range schema.Resources {
			if r.DeprecationMessage != "" {
				continue
			}

			if resourcesToSkip.Has(r.Token) {
				continue
			}

			resourceCode := g.exampleResourceWithName(r, func(resourceToken string) string {
				pkg, modName, memberName, _ := pcl.DecomposeToken(resourceToken, hcl.Range{})
				resourceName := fmt.Sprintf("%sResource", camelCase(memberName))
				if !seenNames.Has(resourceName) {
					seenNames.Add(resourceName)
					return resourceName
				}

				resourceNameWithPkg := fmt.Sprintf("%s%sResource", pkg, memberName)
				if !seenNames.Has(resourceNameWithPkg) {
					seenNames.Add(resourceNameWithPkg)
					return resourceNameWithPkg
				}

				resourceNameWithModule := fmt.Sprintf(
					"example%sResourceFrom%s", resourceName, cgstrings.UppercaseFirst(cleanModuleName(modName)))
				seenNames.Add(resourceNameWithModule)
				return resourceNameWithModule
			})

			buffer.WriteString(fmt.Sprintf("// Resource %s", r.Token))
			buffer.WriteString("\n")
			buffer.WriteString(resourceCode)
			buffer.WriteString("\n")
		}
	}

	if opts.includeFunctions {
		for _, f := range schema.Functions {
			if f.DeprecationMessage != "" {
				continue
			}

			functionCode := g.exampleInvokeWithName(f, func(functionToken string) string {
				pkg, moduleName, memberName, _ := pcl.DecomposeToken(functionToken, hcl.Range{})
				if !seenNames.Has(memberName) {
					seenNames.Add(memberName)
					return memberName
				}

				functionName := fmt.Sprintf("%sResult", memberName)
				if !seenNames.Has(functionName) {
					seenNames.Add(functionName)
					return functionName
				}

				functionNameWithPkg := fmt.Sprintf("%sFrom%s", functionName, cgstrings.UppercaseFirst(pkg))
				if !seenNames.Has(functionNameWithPkg) {
					seenNames.Add(functionNameWithPkg)
					return functionNameWithPkg
				}

				functionNameWithMod := fmt.Sprintf(
					"example%sFrom%s",
					cgstrings.UppercaseFirst(functionName),
					cgstrings.UppercaseFirst(cleanModuleName(moduleName)))
				seenNames.Add(functionNameWithMod)
				return functionNameWithMod
			})

			buffer.WriteString(fmt.Sprintf("// Invoking %s", f.Token))
			buffer.WriteString("\n")
			buffer.WriteString(functionCode)
			buffer.WriteString("\n")
		}
	}

	return buffer.String()
}

func sortPropertiesByRequiredFirst(props []*schema.Property) {
	sort.Slice(props, func(i, j int) bool {
		return props[i].IsRequired() && !props[j].IsRequired()
	})
}

func isPrimitiveType(t schema.Type) bool {
	switch t {
	case schema.BoolType, schema.IntType, schema.NumberType, schema.StringType:
		return true
	default:
		switch argType := t.(type) {
		case *schema.OptionalType:
			return isPrimitiveType(argType.ElementType)
		case *schema.EnumType, *schema.ResourceType:
			return true
		}
		return false
	}
}

func isUnionOfObjects(schemaType *schema.UnionType) bool {
	for _, elementType := range schemaType.ElementTypes {
		if _, isObjectType := elementType.(*schema.ObjectType); !isObjectType {
			return false
		}
	}

	return true
}

func objectTypeHasRecursiveReference(objectType *schema.ObjectType) bool {
	isRecursive := false
	codegen.VisitTypeClosure(objectType.Properties, func(t schema.Type) {
		if objectRef, ok := t.(*schema.ObjectType); ok {
			if objectRef.Token == objectType.Token {
				isRecursive = true
			}
		}
	})

	return isRecursive
}
