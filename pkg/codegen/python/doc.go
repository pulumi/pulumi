// Copyright 2016-2018, Pulumi Corporation.
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

// Pulling out some of the repeated strings tokens into constants would harm readability,
// so we just ignore the goconst linter's warning.
//
// nolint: lll, goconst
package python

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
)

// DocLanguageHelper is the Python-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForPulumiType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string {
	return ""
}

// GetDocLinkForResourceType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForResourceType(pkg *schema.Package, modName, typeName string) string {
	return ""
}

// GetDocLinkForResourceInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	return ""
}

// GetDocLinkForFunctionInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	return ""
}

// GetDocLinkForBuiltInType returns the Python URL for a built-in type.
// Currently not using the typeName parameter because the returned link takes to a general
// top-level page containing info for all built in types.
func (d DocLanguageHelper) GetDocLinkForBuiltInType(typeName string) string {
	return "https://docs.python.org/3/library/stdtypes.html"
}

// GetLanguageTypeString returns the Python-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string {
	name := pyType(t)

	// The Python SDK generator will simply return "list" or "dict" for enumerables.
	// So we examine the underlying types to provide some more information on
	// the elements inside the enumerable.
	switch name {
	case "list":
		arrTy := t.(*schema.ArrayType)
		elType := arrTy.ElementType.String()
		return getListWithTypeName(elementTypeToName(elType))
	case "dict":
		switch dTy := t.(type) {
		case *schema.UnionType:
			types := make([]string, 0, len(dTy.ElementTypes))
			for _, e := range dTy.ElementTypes {
				if schema.IsPrimitiveType(e) {
					types = append(types, e.String())
					continue
				}
				t := d.GetLanguageTypeString(pkg, moduleName, e, input, optional)
				types = append(types, t)
			}
			return strings.Join(types, " | ")
		case *schema.MapType:
			if uTy, ok := dTy.ElementType.(*schema.UnionType); ok {
				return d.GetLanguageTypeString(pkg, moduleName, uTy, input, optional)
			}

			elType := dTy.ElementType.String()
			return getMapWithTypeName(elementTypeToName(elType))
		case *schema.ObjectType:
			return getDictWithTypeName(tokenToName(dTy.Token))
		default:
			return "Dict[str, Any]"
		}
	}
	return name
}

// GetResourceFunctionResultName is not implemented for Python and returns an empty string.
func (d DocLanguageHelper) GetResourceFunctionResultName(resourceName string) string {
	return ""
}

// GenPropertyCaseMap generates the case maps for a property.
func (d DocLanguageHelper) GenPropertyCaseMap(pkg *schema.Package, modName, tool string, prop *schema.Property, snakeCaseToCamelCase, camelCaseToSnakeCase map[string]string) {
	mod := &modContext{
		pkg:                  pkg,
		mod:                  modName,
		tool:                 tool,
		snakeCaseToCamelCase: snakeCaseToCamelCase,
		camelCaseToSnakeCase: camelCaseToSnakeCase,
	}

	if err := mod.recordProperty(prop); err != nil {
		fmt.Printf("error building case map for %q in module %q", prop.Name, modName)
	}
}

// GetPropertyName is not implemented for Python because property names in Python must use
// property case maps, which need to be generated at each provider's package-level.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return "", errors.New("this method is not supported for the python language")
}

// elementTypeToName returns the type name from an element type of the form
// package:module:_type, with its leading "_" stripped.
func elementTypeToName(el string) string {
	parts := strings.Split(el, ":")
	if len(parts) == 3 {
		el = parts[2]
	}
	el = strings.TrimPrefix(el, "_")

	return el
}

// getListWithTypeName returns a Python representation of a list containing
// items of `t`.
func getListWithTypeName(t string) string {
	if t == "string" {
		return "List[str]"
	}

	return fmt.Sprintf("List[%s]", strings.Title(t))
}

// getDictWithTypeName returns the Python representation of a dictionary
// where each item is of type `t`.
func getDictWithTypeName(t string) string {
	return fmt.Sprintf("Dict[%s]", strings.Title(t))
}

// getMapWithTypeName returns the Python representation of a dictionary
// with a string keu and a value of type `t`.
func getMapWithTypeName(t string) string {
	switch t {
	case "string":
		return "Dict[str, str]"
	case "any":
		return "Dict[str, Any]"
	default:
		return fmt.Sprintf("Dict[str, %s]", strings.Title(t))
	}
}
