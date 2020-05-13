package gen

import (
	"bytes"
	gofmt "go/format"
	"io"

	"github.com/hashicorp/hcl/v2"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model/format"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program      *hcl2.Program
	functionArgs map[string]string
	diagnostics  hcl.Diagnostics
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

	// TODO: process nodes
	// for _, n := range nodes {
	// 	g.genNode(&index, n)
	// }

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
	// TODO calculate real imports
	g.Fprintf(w, "import (\n")
	g.Fprintf(w, "\"github.com/pulumi/pulumi/sdk/v2/go/pulumi\"\n")
	g.Fprintf(w, ")\n")

	g.Fprintf(w, "func main() {\n")
	g.Fprintf(w, "pulumi.Run(func(ctx *pulumi.Context) error {\n")

}

// genPostamble closes the method
func (g *generator) genPostamble(w io.Writer, nodes []hcl2.Node) {

	g.Fprint(w, "return nil\n")
	g.Fprintf(w, "})\n")
	g.Fprintf(w, "}\n")
}

// GetPrecedence returns the precedence for the indicated expression. Lower numbers bind more tightly than higher
// numbers.
func (g *generator) GetPrecedence(expr model.Expression) int { /*TODO*/ return -1 }

// GenAnonymousFunctionExpression generates code for an AnonymousFunctionExpression.
func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) { /*TODO*/
}

// GenBinaryOpExpression generates code for a BinaryOpExpression.
func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) { /*TODO*/ }

// GenConditionalExpression generates code for a ConditionalExpression.
func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) { /*TODO*/ }

// GenForExpression generates code for a ForExpression.
func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) { /*TODO*/ }

// GenFunctionCallExpression generates code for a FunctionCallExpression.
func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) { /*TODO*/
}

// GenIndexExpression generates code for an IndexExpression.
func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) { /*TODO*/ }

// GenLiteralValueExpression generates code for a LiteralValueExpression.
func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) { /*TODO*/
}

// GenObjectConsExpression generates code for an ObjectConsExpression.
func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) { /*TODO*/ }

// GenRelativeTraversalExpression generates code for a RelativeTraversalExpression.
func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) { /*TODO*/
}

// GenScopeTraversalExpression generates code for a ScopeTraversalExpression.
func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) { /*TODO*/
}

// GenSplatExpression generates code for a SplatExpression.
func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) { /*TODO*/ }

// GenTemplateExpression generates code for a TemplateExpression.
func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) { /*TODO*/ }

// GenTemplateJoinExpression generates code for a TemplateJoinExpression.
func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) { /*TODO*/
}

// GenTupleConsExpression generates code for a TupleConsExpression.
func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) { /*TODO*/ }

// GenUnaryOpExpression generates code for a UnaryOpExpression.
func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) { /*TODO*/ }
