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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

// DocLanguageHelper is the Python-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForResourceType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForResourceType(packageName, modName, typeName string) string {
	return ""
}

// GetDocLinkForResourceInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(packageName, modName, typeName string, input bool) string {
	return ""
}

// GetDocLinkForFunctionInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(packageName, modName, typeName string, input bool) string {
	return ""
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
		switch dictionaryTy := t.(type) {
		case *schema.MapType:
			elType := dictionaryTy.ElementType.String()
			return getMapWithTypeName(elementTypeToName(elType))
		case *schema.ObjectType:
			return getDictWithTypeName(tokenToName(dictionaryTy.Token))
		default:
			return "Dict[Any, Any]"
		}
	}
	return name
}

// GetResourceFunctionResultName is not implemented for Python and returns an empty string.
func (d DocLanguageHelper) GetResourceFunctionResultName(resourceName string) string {
	return ""
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

	return fmt.Sprintf("List[%s]", PyName(t))
}

// getDictWithTypeName returns the Python representation of a dictionary
// where each item is of type `t`.
func getDictWithTypeName(t string) string {
	return fmt.Sprintf("Dict[%s]", PyName(t))
}

// getMapWithTypeName returns the Python representation of a dictionary
// with a key of type `t` and a value of Any.
func getMapWithTypeName(t string) string {
	switch t {
	case "string":
		return "Dict[str, Any]"
	case "any":
		return "Dict[Any, Any]"
	default:
		return fmt.Sprintf("Dict[%s, Any]", t)
	}
}
