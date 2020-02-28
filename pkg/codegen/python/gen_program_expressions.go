package python

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/zclconf/go-cty/cty"
)

func (g *generator) genExpression(expr model.Expression) string {
	// TODO(pdg): diagnostics

	expr, _ = model.RewriteApplies(expr)
	expr, _ = g.lowerProxyApplies(expr)

	var buf bytes.Buffer
	g.Fgen(&buf, expr)
	return buf.String()
}

func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) {
	g.Fgen(w, "lambda")
	for i, p := range expr.Signature.Parameters {
		if i > 0 {
			g.Fgen(w, ",")
		}
		g.Fgenf(w, " %s", p.Name)
	}

	g.Fgenf(w, ": %v", expr.Body)
}

func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) {
	g.genNYI(w, "BinaryOpExpression")
}

func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) {
	g.genNYI(w, "ConditionalExpression")
}

func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	g.genNYI(w, "ForExpression")
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	//	g.inApplyCall = true
	//	defer func() { g.inApplyCall = false }()

	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := model.ParseApplyCall(expr)
	//g.applyArgs, g.applyArgNames = applyArgs, g.assignApplyArgNames(applyArgs, then)
	//defer func() { g.applyArgs = nil }()

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply`.
		//g.genApplyOutput(w, g.applyArgs[0])
		g.Fgenf(w, "%v.apply(%v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgen(w, "pulumi.Output.all(")
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			//g.genApplyOutput(w, o)
			g.Fgenf(w, "%v", o)
		}
		g.Fgenf(w, ").apply(%v)", then)
	}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case model.IntrinsicApply:
		g.genApply(w, expr)
	case "fileArchive":
		g.Fgenf(w, "pulumi.FileArchive(%v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "pulumi.FileAsset(%v)", expr.Args[0])
	default:
		var rng hcl.Range
		if expr.Syntax != nil {
			rng = expr.Syntax.Range()
		}
		g.genNYI(w, "FunctionCallExpression: %v (%v)", expr.Name, rng)
	}
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.genNYI(w, "IndexExpression")
}

type runeWriter interface {
	WriteRune(c rune) (int, error)
}

// nolint: errcheck
func (g *generator) genEscapedString(w runeWriter, v string, escapeNewlines, escapeBraces bool) {
	for _, c := range v {
		switch c {
		case '\n':
			if escapeNewlines {
				w.WriteRune('\\')
				c = 'n'
			}
		case '"', '\\':
			w.WriteRune('\\')
		case '{', '}':
			if escapeBraces {
				w.WriteRune(c)
			}
		}
		w.WriteRune(c)
	}
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	builder := &strings.Builder{}
	newlines := strings.Count(v, "\n")
	if newlines == 0 || newlines == 1 && (v[0] == '\n' || v[len(v)-1] == '\n') {
		// This string either does not contain newlines or contains a single leading or trailing newline, so we'll
		// Generate a short string literal. Quotes, backslashes, and newlines will be escaped in conformance with
		// https://docs.python.org/3.7/reference/lexical_analysis.html#literals.
		builder.WriteRune('"')
		g.genEscapedString(builder, v, true, false)
		builder.WriteRune('"')
	} else {
		// This string does contain newlines, so we'll generate a long string literal. "${", backquotes, and
		// backslashes will be escaped in conformance with
		// https://docs.python.org/3.7/reference/lexical_analysis.html#literals.
		builder.WriteString(`"""`)
		g.genEscapedString(builder, v, false, false)
		builder.WriteString(`"""`)
	}

	g.Fgenf(w, "%s", builder.String())
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	switch expr.Type() {
	case model.BoolType:
		if expr.Value.BoolValue() {
			g.Fgen(w, "True")
		} else {
			g.Fgen(w, "False")
		}
	case model.NumberType:
		f := expr.Value.NumberValue()
		if float64(int64(f)) == f {
			g.Fgenf(w, "%d", int64(f))
		} else {
			g.Fgenf(w, "%g", f)
		}
	case model.StringType:
		g.genStringLiteral(w, expr.Value.StringValue())
	default:
		contract.Failf("unexpected literal type in GenLiteralValueExpression: %v (%v)", expr.Type(),
			expr.SyntaxNode().Range())
	}
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	if len(expr.Items) == 0 {
		g.Fgen(w, "{}")
	} else {
		g.Fgen(w, "{")
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "\n%s%v: %v,", g.Indent, item.Key, item.Value)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer, traversal hcl.Traversal, types []model.Type) {
	for _, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
		case hcl.TraverseIndex:
			key = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
		}

		switch key.Type() {
		case cty.String:
			keyVal := key.AsString()
			if isLegalIdentifier(keyVal) {
				g.Fgenf(w, ".%s", keyVal)
			} else {
				g.Fgenf(w, "[%q]", keyVal)
			}
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			g.Fgenf(w, "[%q]", key.AsString())
			// g.diagnostics = append(g.diagnostics,
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgen(w, expr.Source)
	g.genRelativeTraversal(w, expr.Syntax.Traversal, expr.Types)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	traversal := expr.Syntax.Traversal
	g.Fgen(w, pyName(traversal.RootName(), false))
	g.genRelativeTraversal(w, traversal.SimpleSplit().Rel, expr.Types)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.genNYI(w, "SplatExpression")
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	// TODO(pdg): triple-quoted string for multi-line literal, quoted braces

	isMultiLine, quotes := false, `"`
	for i, part := range expr.Parts {
		if lit, ok := part.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			v := lit.Value.StringValue()
			switch strings.Count(v, "\n") {
			case 0:
				continue
			case 1:
				if i == 0 && v[0] == '\n' || i == len(expr.Parts)-1 && v[len(v)-1] == '\n' {
					continue
				}
			}
			isMultiLine, quotes = true, `"""`
			break
		}
	}

	b := bufio.NewWriter(w)
	defer b.Flush()

	g.Fprintf(b, `f%s`, quotes)
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			g.genEscapedString(b, lit.Value.StringValue(), !isMultiLine, true)
		} else {
			g.Fgenf(b, "{%v}", expr)
		}
	}
	g.Fprint(b, quotes)
}

func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) {
	g.genNYI(w, "TemplateJoinExpression")
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	switch len(expr.Expressions) {
	case 0:
		g.Fgen(w, "[]")
	case 1:
		g.Fgenf(w, "[%v]", expr.Expressions[0])
	default:
		g.Fgen(w, "[")
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%v,", g.Indent, v)
			}
		})
		g.Fgen(w, "\n", g.Indent, "]")
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	g.genNYI(w, "UnaryOpExpression")
}
