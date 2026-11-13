// Copyright 2024, Pulumi Corporation.
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
	"slices"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	yamldiags "github.com/pulumi/pulumi/sdk/v3/go/common/esc/diags"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
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
	defaultValue() any
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

func (d *MapDecl[T]) defaultValue() any {
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
		vname := name + "." + kvp.Key.Value()
		if strings.HasPrefix(kvp.Key.Value(), "fn::") {
			diags.Extend(
				syntax.NodeError(
					kvp.Key,
					fmt.Sprintf("builtin function call %q not allowed at the top level", kvp.Key.Value()),
				),
			)
		}
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

type (
	ImportListDecl   = *ArrayDecl[*ImportDecl]
	PropertyMapEntry = MapEntry[Expr]
	PropertyMapDecl  = *MapDecl[Expr]
)

// An EnvironmentDecl represents a Pulumi environment.
type EnvironmentDecl struct {
	source []byte

	syntax syntax.Node

	Description *StringExpr

	// Imports is the list of environments to import. It holds the elements of whichever of the top-level `imports`
	// and `environments` keys is in effect.
	Imports ImportListDecl

	// Values holds the environment's values. Besides the entries of the top-level `values` key, it holds an entry
	// for each top-level section (`pulumiConfig` or its alias `config`, `environmentVariables`, and `files`), keyed
	// by the section's canonical name. The legacy spelling of a section, as a key inside `values`, and its top-level
	// spelling are therefore indistinguishable to consumers of the declaration.
	Values PropertyMapDecl

	// Environments is the top-level `environments` key, an alias for `imports`. Consumers should read Imports.
	Environments ImportListDecl `json:",omitempty"`

	// PulumiConfig, Config, EnvironmentVariables, and Files are the top-level sections as written. Config is an
	// alias for PulumiConfig. Consumers should read Values.
	PulumiConfig         Expr `json:",omitempty"`
	Config               Expr `json:",omitempty"`
	EnvironmentVariables Expr `json:",omitempty"`
	Files                Expr `json:",omitempty"`
}

const (
	importsKey      = "imports"
	environmentsKey = "environments"
	valuesKey       = "values"

	// PulumiConfigKey is the canonical name of the section that holds Pulumi stack configuration.
	PulumiConfigKey = "pulumiConfig"
	// ConfigKey is an alias for PulumiConfigKey that is only accepted at the top level of an environment.
	ConfigKey = "config"
	// EnvironmentVariablesKey is the name of the section that holds environment variables.
	EnvironmentVariablesKey = "environmentVariables"
	// FilesKey is the name of the section that holds files.
	FilesKey = "files"
)

// topLevelSections maps each top-level section key to the canonical name of the section it declares.
var topLevelSections = []struct {
	key  string
	name string
}{
	{PulumiConfigKey, PulumiConfigKey},
	{ConfigKey, PulumiConfigKey},
	{EnvironmentVariablesKey, EnvironmentVariablesKey},
	{FilesKey, FilesKey},
}

// TopLevelSectionKeys returns the top-level keys that declare the section with the given canonical name, or nil if
// the name is not that of a section.
func TopLevelSectionKeys(name string) []string {
	var keys []string
	for _, section := range topLevelSections {
		if section.name == name {
			keys = append(keys, section.key)
		}
	}
	return keys
}

// TopLevelSectionName returns the canonical name of the section a top-level key declares. Sections are the
// well-known values (`pulumiConfig`, `environmentVariables`, and `files`) that may be written either as keys inside
// `values` or as top-level keys of an environment. Like every top-level key, section keys are case-insensitive.
func TopLevelSectionName(key string) (string, bool) {
	for _, section := range topLevelSections {
		if strings.EqualFold(section.key, key) {
			return section.name, true
		}
	}
	return "", false
}

// topLevelProperty returns the last top-level property whose key matches the given key. Matching is case-insensitive
// and the last match wins, per parseRecord.
func (d *EnvironmentDecl) topLevelProperty(key string) (syntax.ObjectPropertyDef, bool) {
	obj, ok := d.syntax.(*syntax.ObjectNode)
	if !ok {
		return syntax.ObjectPropertyDef{}, false
	}
	for i := obj.Len() - 1; i >= 0; i-- {
		if kvp := obj.Index(i); strings.EqualFold(kvp.Key.Value(), key) {
			return kvp, true
		}
	}
	return syntax.ObjectPropertyDef{}, false
}

// ignoredKeyWarning returns a warning that reports that a top-level key has no effect.
func ignoredKeyWarning(key *syntax.StringNode, reason string) *syntax.Diagnostic {
	diag := syntax.NodeError(key, fmt.Sprintf("%q is ignored: %s", key.Value(), reason))
	diag.Severity = hcl.DiagWarning
	return diag
}

// normalize folds the top-level aliases and sections into Imports and Values, so that the evaluator and every other
// consumer of the declaration sees a single shape regardless of the spelling the author chose.
//
// Environments that predate these keys must keep their meaning. Until now an unrecognized top-level key was ignored
// with a warning, so wherever the legacy spelling is also present the legacy spelling wins and the new key remains
// ignored, with a warning that says why.
func (d *EnvironmentDecl) normalize() syntax.Diagnostics {
	var diags syntax.Diagnostics

	if d.Environments != nil {
		kvp, _ := d.topLevelProperty(environmentsKey)
		if d.Imports == nil {
			d.Imports = d.Environments
		} else if kvp.Key != nil {
			diags.Extend(ignoredKeyWarning(kvp.Key, fmt.Sprintf("%q is also set", importsKey)))
		}
	}

	if d.PulumiConfig != nil && d.Config != nil {
		if kvp, ok := d.topLevelProperty(ConfigKey); ok {
			diags.Extend(syntax.NodeError(
				kvp.Key,
				fmt.Sprintf("%q is an alias for %q: only one of them may be set", ConfigKey, PulumiConfigKey),
			))
		}
	}

	for _, section := range []struct {
		key   string
		value Expr
	}{
		{PulumiConfigKey, d.PulumiConfig},
		{ConfigKey, d.Config},
		{EnvironmentVariablesKey, d.EnvironmentVariables},
		{FilesKey, d.Files},
	} {
		if section.value == nil {
			continue
		}
		kvp, ok := d.topLevelProperty(section.key)
		if !ok {
			continue
		}
		name, _ := TopLevelSectionName(section.key)

		if slices.ContainsFunc(d.Values.GetEntries(), func(e PropertyMapEntry) bool {
			return e.Key.GetValue() == name
		}) {
			// Either the legacy spelling is present, or this is `config` and `pulumiConfig` already claimed the
			// section (which is reported above).
			if section.key != ConfigKey || d.PulumiConfig == nil {
				diags.Extend(ignoredKeyWarning(kvp.Key, fmt.Sprintf("\"%s.%s\" is also set", valuesKey, name)))
			}
			continue
		}

		if d.Values == nil {
			d.Values = &MapDecl[Expr]{}
		}
		d.Values.Entries = append(d.Values.Entries, PropertyMapEntry{
			syntax: kvp,
			Key:    StringSyntaxValue(kvp.Key, name),
			Value:  section.value,
		})
	}

	return diags
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

func EnvironmentSyntax(
	node *syntax.ObjectNode,
	description *StringExpr,
	imports ImportListDecl,
	values PropertyMapDecl,
) *EnvironmentDecl {
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

// ParseEnvironment parses a environment from the given syntax node. The source text is optional, and is only used to
// print
// diagnostics.
func ParseEnvironment(source []byte, node syntax.Node) (*EnvironmentDecl, syntax.Diagnostics) {
	environment := EnvironmentDecl{source: source}

	diags := parseRecord("environment", &environment, node, true)
	diags.Extend(environment.normalize()...)
	return &environment, diags
}

var (
	parseDeclType  = reflect.TypeFor[parseDecl]()
	nonNilDeclType = reflect.TypeFor[nonNilDecl]()
	recordDeclType = reflect.TypeFor[recordDecl]()
	exprType       = reflect.TypeFor[Expr]()
)

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
		defaultValue := dest.Addr().Interface().(nonNilDecl).defaultValue()
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

		xv := reflect.ValueOf(x)
		if !xv.Type().AssignableTo(dest.Type()) {
			diags.Extend(exprFieldTypeMismatchError(name, dest.Interface(), x))
			v = reflect.New(dest.Type().Elem())
		} else {
			v = xv
		}
	default:
		panic(fmt.Errorf("unexpected field of type %T", dest.Interface()))
	}

	dest.Set(v)
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
	seenKeys := make(map[string]bool)
	for i := 0; i < obj.Len(); i++ {
		kvp := obj.Index(i)

		key := kvp.Key.Value()

		// Add warning diagnostic for duplicate top-level keys
		if seenKeys[key] {
			nodeError := syntax.NodeError(kvp.Key, fmt.Sprintf("duplicate key %q", key))
			nodeError.Severity = hcl.DiagWarning
			diags = append(diags, nodeError)
		}
		seenKeys[key] = true

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
			for f := range t.Fields() {
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

func exprFieldTypeMismatchError(name string, expected any, actual Expr) *syntax.Diagnostic {
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
