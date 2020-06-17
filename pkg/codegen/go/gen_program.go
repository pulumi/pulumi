package gen

import (
	"bytes"
	"fmt"
	gofmt "go/format"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program             *hcl2.Program
	diagnostics         hcl.Diagnostics
	jsonTempSpiller     *jsonSpiller
	ternaryTempSpiller  *tempSpiller
	readDirTempSpiller  *readDirSpiller
	splatSpiller        *splatSpiller
	scopeTraversalRoots codegen.StringSet
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	g := &generator{
		program:             program,
		jsonTempSpiller:     &jsonSpiller{},
		ternaryTempSpiller:  &tempSpiller{},
		readDirTempSpiller:  &readDirSpiller{},
		splatSpiller:        &splatSpiller{},
		scopeTraversalRoots: codegen.NewStringSet(),
	}

	g.Formatter = format.NewFormatter(g)

	var index bytes.Buffer
	g.genPreamble(&index, program)

	for _, n := range nodes {
		g.collectScopeRoots(n)
	}

	for _, n := range nodes {
		g.genNode(&index, n)
	}

	g.genPostamble(&index, nodes)

	// Run Go formatter on the code before saving to disk
	formattedSource, err := gofmt.Source(index.Bytes())
	if err != nil {
		panic(errors.Errorf("invalid Go source code:\n\n%s", index.String()))
	}

	files := map[string][]byte{
		"main.go": formattedSource,
	}
	return files, g.diagnostics, nil
}

func (g *generator) collectScopeRoots(n hcl2.Node) {
	diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
		if st, ok := n.(*model.ScopeTraversalExpression); ok {
			g.scopeTraversalRoots.Add(st.RootName)
		}
		return n, nil
	})
	contract.Assert(len(diags) == 0)
}

// genPreamble generates package decl, imports, and opens the main func
func (g *generator) genPreamble(w io.Writer, program *hcl2.Program) {
	g.Fprint(w, "package main\n\n")
	g.Fprintf(w, "import (\n")

	stdImports, pulumiImports := g.collectImports(w, program)
	for _, imp := range stdImports.SortedValues() {
		g.Fprintf(w, "\"%s\"\n", imp)
	}

	g.Fprintf(w, "\n")
	g.Fprintf(w, "\"github.com/pulumi/pulumi/sdk/v2/go/pulumi\"\n")

	for _, imp := range pulumiImports.SortedValues() {
		g.Fprintf(w, "\"%s\"\n", imp)
	}

	g.Fprintf(w, ")\n")
	g.Fprintf(w, "func main() {\n")
	g.Fprintf(w, "pulumi.Run(func(ctx *pulumi.Context) error {\n")
}

// collect Imports returns two sets of packages imported by the program, std lib packages and pulumi packages
func (g *generator) collectImports(w io.Writer, program *hcl2.Program) (codegen.StringSet, codegen.StringSet) {
	// Accumulate import statements for the various providers
	pulumiImports := codegen.NewStringSet()
	stdImports := codegen.NewStringSet()
	for _, n := range program.Nodes {
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, mod, _, _ := r.DecomposeToken()
			version := -1
			for _, p := range program.Packages() {
				if p.Name == pkg {
					version = int(p.Version.Major)
					break
				}
			}

			if version == -1 {
				panic(errors.Errorf("could not find package information for resource with type token:\n\n%s", r.Token))
			}

			vPath := fmt.Sprintf("/v%d", version)
			if version <= 1 {
				vPath = ""
			}

			pulumiImports.Add(fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s/%s", pkg, vPath, pkg, mod))
		}

		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				for _, fnPkg := range g.genFunctionPackages(call) {
					stdImports.Add(fnPkg)
				}
			}
			if t, ok := n.(*model.TemplateExpression); ok {
				if len(t.Parts) > 1 {
					stdImports.Add("fmt")
				}
			}
			return n, nil
		})
		contract.Assert(len(diags) == 0)
	}

	return stdImports, pulumiImports
}

// genPostamble closes the method
func (g *generator) genPostamble(w io.Writer, nodes []hcl2.Node) {

	g.Fprint(w, "return nil\n")
	g.Fprintf(w, "})\n")
	g.Fprintf(w, "}\n")
}

func (g *generator) genNode(w io.Writer, n hcl2.Node) {
	switch n := n.(type) {
	case *hcl2.Resource:
		g.genResource(w, n)
	case *hcl2.OutputVariable:
		g.genOutputAssignment(w, n)
	// TODO
	// case *hcl2.ConfigVariable:
	// 	g.genConfigVariable(w, n)
	case *hcl2.LocalVariable:
		g.genLocalVariable(w, n)
	}
}

func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {

	resName := r.Name()
	_, mod, typ, _ := r.DecomposeToken()

	// Add conversions to input properties
	for _, input := range r.Inputs {
		destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
		isInput := true
		expr, temps := g.lowerExpression(input.Value, destType.(model.Type), isInput)
		input.Value = expr
		g.genTemps(w, temps)
	}

	instantiate := func(varName, resourceName string) {
		if g.scopeTraversalRoots.Has(varName) || strings.HasPrefix(varName, "_") {
			g.Fgenf(w, "%s, err := %s.New%s(ctx, %s, ", varName, mod, typ, resourceName)
		} else {
			g.Fgenf(w, "_, err = %s.New%s(ctx, %s, ", mod, typ, resourceName)
		}

		if len(r.Inputs) > 0 {
			g.Fgenf(w, "&%s.%sArgs{\n", mod, typ)
			for _, attr := range r.Inputs {
				g.Fgenf(w, "%s: ", strings.Title(attr.Name))
				g.Fgenf(w, "%.v,\n", attr.Value)

			}
			g.Fgenf(w, "})\n")
		} else {
			g.Fgenf(w, "nil)\n")
		}
		g.Fgenf(w, "if err != nil {\n")
		g.Fgenf(w, "return err\n")
		g.Fgenf(w, "}\n")
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr, temps := g.lowerExpression(r.Options.Range, rangeType, false)
		g.genTemps(w, temps)

		g.Fgenf(w, "var %s []*%s.%s\n", resName, mod, typ)

		g.Fgenf(w, "for key0, val0 := range %.v {\n", rangeExpr)
		instantiate("_res", fmt.Sprintf(`fmt.Sprintf("%s-%%v", key0)`, resName))
		g.Fgenf(w, "%s = append(%s, _res)\n", resName, resName)
		g.Fgenf(w, "}\n")

	} else {
		instantiate(resName, fmt.Sprintf("\"%s\"", resName))
	}

}

func (g *generator) genOutputAssignment(w io.Writer, v *hcl2.OutputVariable) {
	isInput := false
	expr, temps := g.lowerExpression(v.Value, v.Type(), isInput)
	g.genTemps(w, temps)
	g.Fgenf(w, "ctx.Export(\"%s\", %.3v)\n", v.Name(), expr)
}
func (g *generator) genTemps(w io.Writer, temps []interface{}) {
	singleReturn := ""
	g.genTempsMultiReturn(w, temps, singleReturn)
}

func (g *generator) genTempsMultiReturn(w io.Writer, temps []interface{}, zeroValueType string) {
	genZeroValueDecl := false

	if zeroValueType != "" {
		for _, t := range temps {
			switch t.(type) {
			case *jsonTemp, *readDirTemp:
				genZeroValueDecl = true
			default:
			}
		}
		if genZeroValueDecl {
			// TODO add entropy to var name
			// currently only used inside anonymous functions (no scope collisions)
			g.Fgenf(w, "var _zero %s\n", zeroValueType)
		}

	}

	for _, t := range temps {
		switch t := t.(type) {
		case *ternaryTemp:
			// TODO derive from ambient context
			isInput := false
			g.Fgenf(w, "var %s %s\n", t.Name, argumentTypeName(t.Value.TrueResult, t.Type(), isInput))
			g.Fgenf(w, "if %.v {\n", t.Value.Condition)
			g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.TrueResult)
			g.Fgenf(w, "} else {\n")
			g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.FalseResult)
			g.Fgenf(w, "}\n")
		case *jsonTemp:
			bytesVar := fmt.Sprintf("tmp%s", strings.ToUpper(t.Name))
			g.Fgenf(w, "%s, err := json.Marshal(", bytesVar)
			args := stripInputs(t.Value.Args[0])
			g.Fgenf(w, "%.v)\n", args)
			g.Fgenf(w, "if err != nil {\n")
			if genZeroValueDecl {
				g.Fgenf(w, "return _zero, err\n")
			} else {
				g.Fgenf(w, "return err\n")
			}
			g.Fgenf(w, "}\n")
			g.Fgenf(w, "%s := string(%s)\n", t.Name, bytesVar)
		case *readDirTemp:
			tmpSuffix := strings.Split(t.Name, "files")[1]
			g.Fgenf(w, "%s, err := ioutil.ReadDir(%.v)\n", t.Name, t.Value.Args[0])
			g.Fgenf(w, "if err != nil {\n")
			if genZeroValueDecl {
				g.Fgenf(w, "return _zero, err\n")
			} else {
				g.Fgenf(w, "return err\n")
			}
			g.Fgenf(w, "}\n")
			namesVar := fmt.Sprintf("fileNames%s", tmpSuffix)
			g.Fgenf(w, "%s := make([]string, len(%s))\n", namesVar, t.Name)
			iVar := fmt.Sprintf("key%s", tmpSuffix)
			valVar := fmt.Sprintf("val%s", tmpSuffix)
			g.Fgenf(w, "for %s, %s := range %s {\n", iVar, valVar, t.Name)
			g.Fgenf(w, "%s[%s] = %s.Name()\n", namesVar, iVar, valVar)
			g.Fgenf(w, "}\n")
		case *splatTemp:
			argTyp := argumentTypeName(t.Value.Each, t.Value.Each.Type(), false)
			g.Fgenf(w, "var %s []%s\n", t.Name, argTyp)
			g.Fgenf(w, "for _, val0 := range %.v {\n", t.Value.Source)
			g.Fgenf(w, "%s = append(%s, %.v)\n", t.Name, t.Name, t.Value.Each)
			g.Fgenf(w, "}\n")
		default:
			contract.Failf("unexpected temp type: %v", t)
		}
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *hcl2.LocalVariable) {
	isInput := false
	expr, temps := g.lowerExpression(v.Definition.Value, v.Type(), isInput)
	g.genTemps(w, temps)
	name := v.Name()
	if !g.scopeTraversalRoots.Has(name) {
		name = "_"
	}
	switch expr := expr.(type) {
	case *model.FunctionCallExpression:
		switch expr.Name {
		case hcl2.Invoke:
			g.Fgenf(w, "%s, err := %.3v;\n", name, expr)
			g.Fgenf(w, "if err != nil {\n")
			g.Fgenf(w, "return err\n")
			g.Fgenf(w, "}\n")
		}
	default:
		g.Fgenf(w, "%s := %.3v;\n", name, expr)

	}

}
