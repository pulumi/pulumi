// Copyright 2026, Pulumi Corporation.
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

package gen

import (
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

type forTemp struct {
	Name  string
	Value *model.ForExpression
}

func (ft *forTemp) Type() model.Type {
	return ft.Value.Type()
}

func (ft *forTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return ft.Type().Traverse(traverser)
}

func (ft *forTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type forSpiller struct {
	temps []*forTemp
	count int
}

func (fs *forSpiller) spillExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	f, ok := x.(*model.ForExpression)
	if !ok {
		return x, nil
	}
	if f.Group {
		return x, nil
	}
	switch model.ResolveOutputs(f.Collection.Type()).(type) {
	case *model.ListType, *model.TupleType, *model.MapType, *model.ObjectType:
	default:
		return x, nil
	}
	temp := &forTemp{
		Name:  fmt.Sprintf("forResult%d", fs.count),
		Value: f,
	}
	fs.temps = append(fs.temps, temp)
	fs.count++
	return &model.ScopeTraversalExpression{
		RootName:  temp.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteForExpressions(
	x model.Expression,
	spiller *forSpiller,
) (model.Expression, []*forTemp, hcl.Diagnostics) {
	spiller.temps = nil
	x, diags := model.VisitExpression(x, nil, spiller.spillExpression)

	return x, spiller.temps, diags
}

// genForTemp generates the loop for a spilled for expression: a slice append
// for list results, a map assignment for map results. Map and object
// collections are iterated in sorted key order to match HCL's evaluation
// order.
func (g *generator) genForTemp(w io.Writer, t *forTemp) {
	f := t.Value
	suffix := strings.TrimPrefix(t.Name, "forResult")

	valueArgTyp := g.argumentTypeName(f.Value.Type(), false)
	if valueArgTyp == "pulumi.IDInput" {
		valueArgTyp = "pulumi.ID"
	}
	isPulumiTyp := strings.Contains(valueArgTyp, ".")
	switch {
	case f.Key != nil && isPulumiTyp:
		g.Fgenf(w, "%s := %sMap{}\n", t.Name, valueArgTyp)
	case f.Key != nil:
		g.Fgenf(w, "%s := map[string]%s{}\n", t.Name, valueArgTyp)
	case isPulumiTyp:
		g.Fgenf(w, "var %s %sArray\n", t.Name, valueArgTyp)
	default:
		g.Fgenf(w, "var %s []%s\n", t.Name, valueArgTyp)
	}

	accessed := func(v *model.Variable) bool {
		return v != nil &&
			(pcl.VariableAccessed(v.Name, f.Value) ||
				pcl.VariableAccessed(v.Name, f.Key) ||
				pcl.VariableAccessed(v.Name, f.Condition))
	}
	keyUsed, valUsed := accessed(f.KeyVariable), accessed(f.ValueVariable)

	switch model.ResolveOutputs(f.Collection.Type()).(type) {
	case *model.MapType, *model.ObjectType:
		rangeName, keysName := "forRange"+suffix, "forKeys"+suffix
		g.Fgenf(w, "%s := %.v\n", rangeName, f.Collection)
		g.Fgenf(w, "%s := make([]string, 0, len(%s))\n", keysName, rangeName)
		g.Fgenf(w, "for forKey%s := range %s {\n", suffix, rangeName)
		g.Fgenf(w, "%s = append(%s, forKey%s)\n", keysName, keysName, suffix)
		g.Fgenf(w, "}\n")
		g.importer.Import("sort", "sort")
		g.Fgenf(w, "sort.Strings(%s)\n", keysName)

		keyVar := "forKey" + suffix
		if keyUsed {
			keyVar = makeValidIdentifier(f.KeyVariable.Name)
		}
		if keyUsed || valUsed {
			g.Fgenf(w, "for _, %s := range %s {\n", keyVar, keysName)
		} else {
			g.Fgenf(w, "for range %s {\n", keysName)
		}
		if valUsed {
			g.Fgenf(w, "%s := %s[%s]\n", makeValidIdentifier(f.ValueVariable.Name), rangeName, keyVar)
		}
	default:
		switch {
		case valUsed && keyUsed:
			g.Fgenf(w, "for %s, %s := range %.v {\n",
				makeValidIdentifier(f.KeyVariable.Name), makeValidIdentifier(f.ValueVariable.Name), f.Collection)
		case valUsed:
			g.Fgenf(w, "for _, %s := range %.v {\n", makeValidIdentifier(f.ValueVariable.Name), f.Collection)
		case keyUsed:
			g.Fgenf(w, "for %s := range %.v {\n", makeValidIdentifier(f.KeyVariable.Name), f.Collection)
		default:
			g.Fgenf(w, "for range %.v {\n", f.Collection)
		}
	}

	if f.Condition != nil {
		g.Fgenf(w, "if %.v {\n", f.Condition)
	}
	if f.Key != nil {
		g.Fgenf(w, "%s[%.v] = %.v\n", t.Name, f.Key, f.Value)
	} else {
		g.Fgenf(w, "%s = append(%s, %.v)\n", t.Name, t.Name, f.Value)
	}
	if f.Condition != nil {
		g.Fgenf(w, "}\n")
	}
	g.Fgenf(w, "}\n")
}
