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
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// titleCase replaces the first character in the given string with its upper-case equivalent.
func titleCase(s string) string {
	c, sz := utf8.DecodeRuneInString(s)
	if sz == 0 || unicode.IsUpper(c) {
		return s
	}
	return string([]rune{unicode.ToUpper(c)}) + s[sz:]
}

func SourceOrderNodes(nodes []Node) []Node {
	sort.Slice(nodes, func(i, j int) bool {
		return model.SourceOrderLess(nodes[i].SyntaxNode().Range(), nodes[j].SyntaxNode().Range())
	})
	return nodes
}

func DecomposeToken(tok string, sourceRange hcl.Range) (string, string, string, hcl.Diagnostics) {
	components := strings.Split(tok, ":")
	if len(components) != 3 {
		// If we don't have a valid type token, return the invalid token as the type name.
		return "", "", tok, hcl.Diagnostics{malformedToken(tok, sourceRange)}
	}
	return components[0], components[1], components[2], nil
}

func hasDependencyOn(a, b Node) bool {
	for _, d := range a.getDependencies() {
		if d.Name() == b.Name() {
			return true
		}
	}
	return false
}

func mutuallyDependant(a, b Node) bool {
	return hasDependencyOn(a, b) && hasDependencyOn(b, a)
}

func linearizeNode(n Node, done codegen.Set, list *[]Node) {
	if !done.Has(n) {
		for _, d := range n.getDependencies() {
			if !mutuallyDependant(n, d) {
				linearizeNode(d, done, list)
			}
		}

		*list = append(*list, n)
		done.Add(n)
	}
}

// Linearize performs a topological sort of the nodes in the program so that they can be processed by tools that need
// to see all of a node's dependencies before the node itself (e.g. a code generator for a programming language that
// requires variables to be defined before they can be referenced). The sort is stable, and nodes are kept in source
// order as much as possible.
func Linearize(p *Program) []Node {
	type file struct {
		name  string // The name of the HCL source file.
		nodes []Node // The list of nodes defined by the source file.
	}

	// First, collect nodes into files. Ignore config and outputs, as these are sources and sinks, respectively.
	files := map[string]*file{}
	for _, n := range p.Nodes {
		filename := n.SyntaxNode().Range().Filename
		f, ok := files[filename]
		if !ok {
			f = &file{name: filename}
			files[filename] = f
		}
		f.nodes = append(f.nodes, n)
	}

	// Now build a worklist out of the set of files, sorting the nodes in each file in source order as we go.
	worklist := slice.Prealloc[*file](len(files))
	for _, f := range files {
		SourceOrderNodes(f.nodes)
		worklist = append(worklist, f)
	}

	// While the worklist is not empty, add the nodes in the file with the fewest unsatisfied dependencies on nodes in
	// other files.
	doneNodes, nodes := codegen.Set{}, slice.Prealloc[Node](len(p.Nodes))
	for len(worklist) > 0 {
		// Recalculate file weights and find the file with the lowest weight.
		var next *file
		var nextIndex, nextWeight int
		for i, f := range worklist {
			weight, processed := 0, codegen.Set{}
			for _, n := range f.nodes {
				for _, d := range n.getDependencies() {
					// We don't count nodes that we've already counted or nodes that have already been ordered.
					if processed.Has(d) || doneNodes.Has(d) {
						continue
					}

					// If this dependency resides in a different file, increment the current file's weight and mark the
					// depdendency as processed.
					depFilename := d.SyntaxNode().Range().Filename
					if depFilename != f.name {
						weight++
					}
					processed.Add(d)
				}
			}

			// If we haven't yet chosen a file to generate or if this file has fewer unsatisfied dependencies than the
			// current choice, choose this file. Ties are broken by the lexical order of the filenames.
			if next == nil || weight < nextWeight || weight == nextWeight && f.name < next.name {
				next, nextIndex, nextWeight = f, i, weight
			}
		}

		// Swap the chosen file with the tail of the list, then trim the worklist by one.
		worklist[len(worklist)-1], worklist[nextIndex] = worklist[nextIndex], worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]

		// Now generate the nodes in the chosen file and mark the file as done.
		for _, n := range next.nodes {
			linearizeNode(n, doneNodes, &nodes)
		}
	}

	return nodes
}

// Remaps the "pulumi:providers:$Package" token style to "$Package:index:Provider", consistent with code generation.
// This mapping is consistent with how provider resources are projected into the schema and removes special casing logic
// to generate registering explicit providers.
//
// The resultant program should be a shallow copy of the source with only the modified resource nodes copied.
func MapProvidersAsResources(p *Program) {
	for _, n := range p.Nodes {
		if r, ok := n.(*Resource); ok && r.Schema != nil {
			pkg, mod, name, _ := r.DecomposeToken()
			if r.Schema.IsProvider && pkg == "pulumi" && mod == "providers" {
				// the binder emits tokens like this when the module is "index"
				r.Token = name + "::Provider"
			}
		}
	}
}

func FixupPulumiPackageTokens(r *Resource) {
	pkg, mod, name, _ := r.DecomposeToken()
	if pkg == "pulumi" && mod == "pulumi" {
		r.Token = "pulumi::" + name
	}
}

// SortedFunctionParameters returns a list of properties of the input type from the schema
// for an invoke function call which has multi argument inputs. We assume here
// that the expression is an invoke which has it's args (2nd parameter) annotated
// with the original schema type. The original schema type has properties sorted.
// This is important because model.ObjectType has no guarantee of property order.
func SortedFunctionParameters(expr *model.FunctionCallExpression) []*schema.Property {
	if !expr.Signature.MultiArgumentInputs {
		return []*schema.Property{}
	}

	switch args := expr.Signature.Parameters[1].Type.(type) {
	case *model.ObjectType:
		originalSchemaType, ok := model.GetObjectTypeAnnotation[*schema.ObjectType](args)
		if !ok {
			return []*schema.Property{}
		}

		return originalSchemaType.Properties
	default:
		return []*schema.Property{}
	}
}

// GenerateMultiArguments takes the input bag (object) of a function invoke and spreads the values of that object
// into multi-argument function call.
// For example, { a: 1, b: 2 } with multiInputArguments: ["a", "b"] would become: 1, 2
//
// However, when optional parameters are omitted, then <undefinedLiteral> is used where they should be.
// Take for example { a: 1, c: 3 } with multiInputArguments: ["a", "b", "c"], it becomes 1, <undefinedLiteral>, 3
// because b was omitted and c was provided so b had to be the provided <undefinedLiteral>
func GenerateMultiArguments(
	f *format.Formatter,
	w io.Writer,
	undefinedLiteral string,
	expr *model.ObjectConsExpression,
	multiArguments []*schema.Property,
) {
	items := make(map[string]model.Expression)
	for _, item := range expr.Items {
		lit := item.Key.(*model.LiteralValueExpression)
		propertyKey := lit.Value.AsString()
		items[propertyKey] = item.Value
	}

	hasMoreArgs := func(index int) bool {
		for _, arg := range multiArguments[index:] {
			if _, ok := items[arg.Name]; ok {
				return true
			}
		}

		return false
	}

	for index, arg := range multiArguments {
		value, ok := items[arg.Name]
		if ok {
			f.Fgenf(w, "%.v", value)
		} else if hasMoreArgs(index) {
			// a positional argument was not provided in the input bag
			// assume it is optional
			f.Fgen(w, undefinedLiteral)
		}

		if hasMoreArgs(index + 1) {
			f.Fgen(w, ", ")
		}
	}
}

func SortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0)
	for propertyName := range m {
		keys = append(keys, propertyName)
	}

	sort.Strings(keys)
	return keys
}

// UnwrapOption returns type T if the input is an Option(T)
func UnwrapOption(exprType model.Type) model.Type {
	switch exprType := exprType.(type) {
	case *model.UnionType:
		if len(exprType.ElementTypes) == 2 && exprType.ElementTypes[0] == model.NoneType {
			return exprType.ElementTypes[1]
		} else if len(exprType.ElementTypes) == 2 && exprType.ElementTypes[1] == model.NoneType {
			return exprType.ElementTypes[0]
		}
		return exprType
	default:
		return exprType
	}
}

// VariableAccessed returns whether the given variable name is accessed in the given expression.
func VariableAccessed(variableName string, expr model.Expression) bool {
	accessed := false
	visitor := func(subExpr model.Expression) (model.Expression, hcl.Diagnostics) {
		if traversal, ok := subExpr.(*model.ScopeTraversalExpression); ok {
			if traversal.RootName == variableName {
				accessed = true
			}
		}
		return subExpr, nil
	}

	_, diags := model.VisitExpression(expr, model.IdentityVisitor, visitor)
	contract.Assertf(len(diags) == 0, "expected no diagnostics from VisitExpression")
	return accessed
}

// LiteralValueString evaluates the given expression and returns the string value if it is a literal value expression
// otherwise returns an empty string for anything else.
func LiteralValueString(x model.Expression) string {
	switch x := x.(type) {
	case *model.LiteralValueExpression:
		if model.StringType.AssignableFrom(x.Type()) {
			return x.Value.AsString()
		}
	case *model.TemplateExpression:
		if len(x.Parts) == 1 {
			if lit, ok := x.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
				return lit.Value.AsString()
			}
		}
	}

	return ""
}

// inferVariableName infers a variable name from the given traversal expression.
// for example if you have component.firstName.lastName it will become componentFirstNameLastName
func InferVariableName(traversal *model.ScopeTraversalExpression) string {
	if len(traversal.Parts) == 1 {
		return traversal.RootName
	}

	parts := make([]string, 0, len(traversal.Parts))
	for _, part := range traversal.Traversal {
		switch part := part.(type) {
		case hcl.TraverseAttr:
			parts = append(parts, titleCase(part.Name))
		case hcl.TraverseIndex:
			var key string
			if part.Key.Type().Equals(cty.String) {
				key = titleCase(part.Key.AsString())
			}

			if part.Key.Type().Equals(cty.Number) {
				key = part.Key.AsBigFloat().String()
			}

			parts = append(parts, "At"+key)
		}
	}

	return traversal.RootName + strings.Join(parts, "")
}

// isComponentReference takes a program and a root name and returns a component if the root
// refers to a component in the given program.
func isComponentReference(program *Program, root string) (*Component, bool) {
	for _, node := range program.Nodes {
		if c, ok := node.(*Component); ok && c.Name() == root {
			return c, true
		}
	}

	return nil, false
}

type DeferredOutputVariable struct {
	Name            string
	Expr            model.Expression
	SourceComponent *Component
}

func ExtractDeferredOutputVariables(
	program *Program,
	component *Component,
	expr model.Expression,
) (model.Expression, []*DeferredOutputVariable) {
	var deferredOutputs []*DeferredOutputVariable

	nodeOrder := map[string]int{}
	for i, node := range program.Nodes {
		nodeOrder[node.Name()] = i
	}

	componentTraversalExpr := func(subExpr model.Expression) (*model.ScopeTraversalExpression, *Component, bool) {
		if traversal, ok := subExpr.(*model.ScopeTraversalExpression); ok {
			if componentRef, ok := isComponentReference(program, traversal.RootName); ok {
				if mutuallyDependantComponents(component, componentRef) {
					if nodeOrder[componentRef.Name()] > nodeOrder[component.Name()] {
						return traversal, componentRef, true
					}
				}
			}
		}
		return nil, nil, false
	}

	visitor := func(subExpr model.Expression) (model.Expression, hcl.Diagnostics) {
		if traversal, componentRef, ok := componentTraversalExpr(subExpr); ok {
			// we found a reference to component that appears later in the program
			variableName := InferVariableName(traversal)
			deferredOutputs = append(deferredOutputs, &DeferredOutputVariable{
				Name:            variableName,
				Expr:            subExpr,
				SourceComponent: componentRef,
			})

			return model.VariableReference(&model.Variable{
				Name:         variableName,
				VariableType: model.NewOutputType(subExpr.Type()),
			}), nil
		}

		// handle for loops where the collection we are looping over
		// is a list of components that are defined later in the program
		// turn the entire the ForExpression into a deferred output variable
		if forExpr, ok := subExpr.(*model.ForExpression); ok {
			if traversal, componentRef, ok := componentTraversalExpr(forExpr.Collection); ok {
				variableName := "loopingOver" + titleCase(InferVariableName(traversal))
				deferredOutputs = append(deferredOutputs, &DeferredOutputVariable{
					Name:            variableName,
					Expr:            forExpr,
					SourceComponent: componentRef,
				})

				return model.VariableReference(&model.Variable{
					Name:         variableName,
					VariableType: model.NewOutputType(forExpr.Type()),
				}), nil
			}
		}

		return subExpr, nil
	}

	modifiedExpr, diags := model.VisitExpression(expr, visitor, model.IdentityVisitor)
	contract.Assertf(len(diags) == 0, "expected no diagnostics from VisitExpression")
	return modifiedExpr, deferredOutputs
}
