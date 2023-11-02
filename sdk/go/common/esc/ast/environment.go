// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	yamldiags "github.com/pulumi/esc/diags"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type declNode struct {
	syntax syntax.Node
}

func (x *declNode) Syntax() syntax.Node {
	if x == nil {
		return nil
	}
	return x.syntax
}

func (*declNode) isNode() {}

type Node interface {
	isNode()
}

type parseDecl interface {
	parse(name string, node syntax.Node) syntax.Diagnostics
}

type recordDecl interface {
	recordSyntax() *syntax.Node
}

type nonNilDecl interface {
	defaultValue() interface{}
}

type ArrayDecl[T Node] struct {
	declNode

	Elements []T
}

func (d *ArrayDecl[T]) GetElements() []T {
	if d == nil {
		return nil
	}
	return d.Elements
}

func (d *ArrayDecl[T]) parse(name string, node syntax.Node) syntax.Diagnostics {
	list, ok := node.(*syntax.ArrayNode)
	if !ok {
		return syntax.Diagnostics{syntax.NodeError(node, fmt.Sprintf("%v must be a list", name))}
	}

	var diags syntax.Diagnostics

	elements := make([]T, list.Len())
	for i := range elements {
		ename := fmt.Sprintf("%s[%d]", name, i)
		ediags := parseNode(ename, &elements[i], list.Index(i))
		diags.Extend(ediags...)
	}
	d.Elements = elements

	return diags
}

type MapEntry[T Node] struct {
	syntax syntax.ObjectPropertyDef

	Key   *StringExpr
	Value T
}

type MapDecl[T Node] struct {
	declNode

	Entries []MapEntry[T]
}

func (d *MapDecl[T]) GetEntries() []MapEntry[T] {
	if d == nil {
		return nil
	}
	return d.Entries
}

func (d *MapDecl[T]) defaultValue() interface{} {
	return &MapDecl[T]{}
}

func (d *MapDecl[T]) parse(name string, node syntax.Node) syntax.Diagnostics {
	d.syntax = node

	obj, ok := node.(*syntax.ObjectNode)
	if !ok {
		return syntax.Diagnostics{syntax.NodeError(node, fmt.Sprintf("%v must be an object", name))}
	}

	var diags syntax.Diagnostics

	entries := make([]MapEntry[T], obj.Len())
	for i := range entries {
		kvp := obj.Index(i)

		var v T
		vname := fmt.Sprintf("%s.%s", name, kvp.Key.Value())
		vdiags := parseNode(vname, &v, kvp.Value)
		diags.Extend(vdiags...)

		entries[i] = MapEntry[T]{
			syntax: kvp,
			Key:    StringSyntax(kvp.Key),
			Value:  v,
		}
	}
	d.Entries = entries

	return diags

}

type ImportMetaDecl struct {
	declNode

	Merge *BooleanExpr
}

func (d *ImportMetaDecl) recordSyntax() *syntax.Node {
	return &d.syntax
}

type ImportDecl struct {
	declNode

	Environment *StringExpr
	Meta        *ImportMetaDecl
}

func (d *ImportDecl) parse(name string, node syntax.Node) syntax.Diagnostics {
	d.syntax = node
	switch node := node.(type) {
	case *syntax.StringNode:
		d.Environment = StringSyntax(node)
		return nil
	case *syntax.ObjectNode:
		// single key
		if node.Len() != 1 {
			return syntax.Diagnostics{syntax.NodeError(node, "import must have a single key")}
		}
		kvp := node.Index(0)
		d.Environment = StringSyntax(kvp.Key)

		d.Meta = &ImportMetaDecl{}
		return parseRecord("import", d.Meta, kvp.Value, false)
	default:
		return syntax.Diagnostics{syntax.NodeError(node, "import must be a string or an object")}
	}
}

type ImportListDecl = *ArrayDecl[*ImportDecl]
type PropertyMapEntry = MapEntry[Expr]
type PropertyMapDecl = *MapDecl[Expr]

// An EnvironmentDecl represents a Pulumi environment.
type EnvironmentDecl struct {
	source []byte

	syntax syntax.Node

	Description *StringExpr
	Imports     ImportListDecl
	Values      PropertyMapDecl
}

func (d *EnvironmentDecl) Syntax() syntax.Node {
	if d == nil {
		return nil
	}
	return d.syntax
}

func (d *EnvironmentDecl) recordSyntax() *syntax.Node {
	return &d.syntax
}

// NewDiagnosticWriter returns a new hcl.DiagnosticWriter that can be used to print diagnostics associated with the
// environment.
func (d *EnvironmentDecl) NewDiagnosticWriter(w io.Writer, width uint, color bool) hcl.DiagnosticWriter {
	fileMap := map[string]*hcl.File{}
	if d.source != nil {
		if s := d.syntax; s != nil {
			fileMap[s.Syntax().Range().Filename] = &hcl.File{Bytes: d.source}
		}
	}
	return newDiagnosticWriter(w, fileMap, width, color)
}

func EnvironmentSyntax(node *syntax.ObjectNode, description *StringExpr, imports ImportListDecl, values PropertyMapDecl) *EnvironmentDecl {
	return &EnvironmentDecl{
		syntax:      node,
		Description: description,
		Imports:     imports,
		Values:      values,
	}
}

func Environment(description *StringExpr, imports ImportListDecl, values PropertyMapDecl) *EnvironmentDecl {
	return EnvironmentSyntax(nil, description, imports, values)
}

// ParseEnvironment parses a environment from the given syntax node. The source text is optional, and is only used to print
// diagnostics.
func ParseEnvironment(source []byte, node syntax.Node) (*EnvironmentDecl, syntax.Diagnostics) {
	environment := EnvironmentDecl{source: source}

	diags := parseRecord("environment", &environment, node, false)
	return &environment, diags
}

var parseDeclType = reflect.TypeOf((*parseDecl)(nil)).Elem()
var nonNilDeclType = reflect.TypeOf((*nonNilDecl)(nil)).Elem()
var recordDeclType = reflect.TypeOf((*recordDecl)(nil)).Elem()
var exprType = reflect.TypeOf((*Expr)(nil)).Elem()

func parseNode[T Node](name string, dest *T, node syntax.Node) syntax.Diagnostics {
	return parseField(name, reflect.ValueOf(dest).Elem(), node)
}

func parseField(name string, dest reflect.Value, node syntax.Node) syntax.Diagnostics {
	if node == nil {
		return nil
	}

	var v reflect.Value
	var diags syntax.Diagnostics

	if dest.CanAddr() && dest.Addr().Type().AssignableTo(nonNilDeclType) {
		// destination is T, and must be a record type (right now)
		defaultValue := (dest.Addr().Interface().(nonNilDecl)).defaultValue()
		switch x := defaultValue.(type) {
		case parseDecl:
			pdiags := x.parse(name, node)
			diags.Extend(pdiags...)
			v = reflect.ValueOf(defaultValue).Elem().Convert(dest.Type())
		case recordDecl:
			pdiags := parseRecord(name, x, node, true)
			diags.Extend(pdiags...)
			v = reflect.ValueOf(defaultValue).Elem().Convert(dest.Type())
		}
		dest.Set(v)
		return diags
	}

	switch {
	case dest.Type().AssignableTo(parseDeclType):
		// assume that dest is *T
		v = reflect.New(dest.Type().Elem())
		pdiags := v.Interface().(parseDecl).parse(name, node)
		diags.Extend(pdiags...)
	case dest.Type().AssignableTo(recordDeclType):
		// assume that dest is *T
		v = reflect.New(dest.Type().Elem())
		rdiags := parseRecord(name, v.Interface().(recordDecl), node, true)
		diags.Extend(rdiags...)
	case dest.Type().AssignableTo(exprType):
		x, xdiags := ParseExpr(node)
		diags.Extend(xdiags...)
		if diags.HasErrors() {
			return diags
		}

		xv := reflect.ValueOf(x)
		if !xv.Type().AssignableTo(dest.Type()) {
			diags.Extend(exprFieldTypeMismatchError(name, dest.Interface(), x))
		} else {
			v = xv
		}
	default:
		panic(fmt.Errorf("unexpected field of type %T", dest.Interface()))
	}

	if !diags.HasErrors() {
		dest.Set(v)
	}
	return diags
}

func parseRecord(objName string, dest recordDecl, node syntax.Node, noMatchWarning bool) syntax.Diagnostics {
	obj, ok := node.(*syntax.ObjectNode)
	if !ok {
		return syntax.Diagnostics{syntax.NodeError(node, fmt.Sprintf("%v must be an object", objName))}
	}
	*dest.recordSyntax() = obj
	contract.Assertf(*dest.recordSyntax() == obj, "%s.recordSyntax took by value, so the assignment failed", objName)

	v := reflect.ValueOf(dest).Elem()
	t := v.Type()

	var diags syntax.Diagnostics
	for i := 0; i < obj.Len(); i++ {
		kvp := obj.Index(i)

		key := kvp.Key.Value()
		var hasMatch bool
		for _, f := range reflect.VisibleFields(t) {
			if f.IsExported() && strings.EqualFold(f.Name, key) {
				diags.Extend(parseField(camel(f.Name), v.FieldByIndex(f.Index), kvp.Value)...)
				hasMatch = true
				break
			}
		}

		if !hasMatch && noMatchWarning {
			var fieldNames []string
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.IsExported() {
					fieldNames = append(fieldNames, fmt.Sprintf("'%s'", camel(f.Name)))
				}
			}
			formatter := yamldiags.NonExistentFieldFormatter{
				ParentLabel: fmt.Sprintf("Object '%s'", objName),
				Fields:      fieldNames,
			}
			msg := formatter.Message(key, fmt.Sprintf("Field '%s'", key))
			nodeError := syntax.NodeError(kvp.Key, msg)
			nodeError.Severity = hcl.DiagWarning
			diags = append(diags, nodeError)
		}

	}

	return diags
}

func exprFieldTypeMismatchError(name string, expected interface{}, actual Expr) *syntax.Diagnostic {
	var typeName string
	switch expected.(type) {
	case *NullExpr:
		typeName = "null"
	case *BooleanExpr:
		typeName = "a boolean value"
	case *NumberExpr:
		typeName = "a number"
	case *StringExpr:
		typeName = "a string"
	case *SymbolExpr:
		typeName = "a symbol"
	case *InterpolateExpr:
		typeName = "an interpolated string"
	case *ArrayExpr:
		typeName = "an array"
	case *ObjectExpr:
		typeName = "an object"
	case BuiltinExpr:
		typeName = "a builtin function call"
	default:
		typeName = fmt.Sprintf("a %T", expected)
	}
	return ExprError(actual, fmt.Sprintf("%v must be %v", name, typeName))
}

func camel(s string) string {
	if s == "" {
		return ""
	}
	name := []rune(s)
	name[0] = unicode.ToLower(name[0])
	return string(name)
}
