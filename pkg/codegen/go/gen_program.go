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
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program     *hcl2.Program
	diagnostics hcl.Diagnostics
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	g := &generator{
		program: program,
	}

	g.Formatter = format.NewFormatter(g)

	var index bytes.Buffer
	g.genPreamble(&index, program)

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

// genPreamble generates package decl, imports, and opens the main func
func (g *generator) genPreamble(w io.Writer, program *hcl2.Program) {
	g.Fprint(w, "package main\n")

	imports := g.collectImports(w, program)
	g.Fprintf(w, "import (\n")
	g.Fprintf(w, "\"github.com/pulumi/pulumi/sdk/v2/go/pulumi\"\n")
	for _, pkg := range imports.SortedValues() {
		g.Fprintf(w, "\"%s\"\n", pkg)
	}
	g.Fprintf(w, ")\n")

	g.Fprintf(w, "func main() {\n")
	g.Fprintf(w, "pulumi.Run(func(ctx *pulumi.Context) error {\n")
}

func (g *generator) collectImports(w io.Writer, program *hcl2.Program) codegen.StringSet {
	// Accumulate import statements for the various providers
	pulumiImports := codegen.NewStringSet()
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
	}

	return pulumiImports
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
		// case *hcl2.LocalVariable:
		// 	g.genLocalVariable(w, n)
	}
}

func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {

	resName := r.Name()
	_, mod, typ, _ := r.DecomposeToken()
	isInput := true

	// Add conversions to input properties
	for _, input := range r.Inputs {
		destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
		expr, temps := g.lowerExpression(input.Value, destType.(model.Type), isInput)
		input.Value = expr
		g.genTemps(w, temps)
	}

	g.Fgenf(w, "%s, err := %s.New%s(ctx, \"%[1]s\", ", resName, mod, typ)
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

func (g *generator) genOutputAssignment(w io.Writer, v *hcl2.OutputVariable) {
	isInput := false
	expr, temps := g.lowerExpression(v.Value, v.Type(), isInput)
	g.genTemps(w, temps)
	g.Fgenf(w, "ctx.Export(\"%s\", %.3v)\n", v.Name(), expr)
}

func (g *generator) genTemps(w io.Writer, temps []*ternaryTemp) {
	for _, t := range temps {

		// TODO derive from ambient context
		isInput := false
		g.Fgenf(w, "var %s %s\n", t.Name, argumentTypeName(t.Value.TrueResult, t.Type(), isInput))
		g.Fgenf(w, "if %.v {\n", t.Value.Condition)
		g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.TrueResult)
		g.Fgenf(w, "} else {\n")
		g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.FalseResult)
		g.Fgenf(w, "}\n")
	}
}
