// Copyright 2020-2025, Pulumi Corporation.
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

package nodejs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return makeValidIdentifier(name)
}

func (g *generator) lowerExpression(expr model.Expression, typ model.Type) model.Expression {
	// TODO(pdg): diagnostics
	if g.asyncMain {
		expr = g.awaitInvokes(expr)
	}

	expr = pcl.RewritePropertyReferences(expr)
	skipToJSONWhenRewritingApplies := true
	expr, diags := pcl.RewriteAppliesWithSkipToJSON(expr, nameInfo(0), !g.asyncMain, skipToJSONWhenRewritingApplies)
	if typ != nil {
		var convertDiags hcl.Diagnostics
		expr, convertDiags = pcl.RewriteConversions(expr, typ)
		diags = diags.Extend(convertDiags)
	}
	expr, lowerProxyDiags := g.lowerProxyApplies(expr)
	diags = diags.Extend(lowerProxyDiags)
	g.diagnostics = g.diagnostics.Extend(diags)
	return expr
}

func (g *generator) RewriteVariableRenames(expr model.Expression, typ model.Type) (model.Expression, hcl.Diagnostics) {
	rewriter := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		traversal, ok := expr.(*model.ScopeTraversalExpression)
		if !ok {
			return expr, nil
		}

		traversal.RootName = makeValidIdentifier(traversal.RootName)

		return expr, nil
	}

	return model.VisitExpression(expr, model.IdentityVisitor, rewriter)
}

func (g *generator) GetPrecedence(expr model.Expression) int {
	// Precedence is derived from
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Operator_Precedence.
	switch expr := expr.(type) {
	case *model.ConditionalExpression:
		return 4
	case *model.BinaryOpExpression:
		switch expr.Operation {
		case hclsyntax.OpLogicalOr:
			return 5
		case hclsyntax.OpLogicalAnd:
			return 6
		case hclsyntax.OpEqual, hclsyntax.OpNotEqual:
			return 11
		case hclsyntax.OpGreaterThan, hclsyntax.OpGreaterThanOrEqual, hclsyntax.OpLessThan,
			hclsyntax.OpLessThanOrEqual:
			return 12
		case hclsyntax.OpAdd, hclsyntax.OpSubtract:
			return 14
		case hclsyntax.OpMultiply, hclsyntax.OpDivide, hclsyntax.OpModulo:
			return 15
		default:
			contract.Failf("unexpected binary expression %v", expr)
		}
	case *model.UnaryOpExpression:
		return 17
	case *model.FunctionCallExpression:
		switch expr.Name {
		case intrinsicAwait:
			return 17
		case intrinsicInterpolate:
			return 22
		default:
			return 20
		}
	case *model.ForExpression, *model.IndexExpression, *model.RelativeTraversalExpression, *model.SplatExpression,
		*model.TemplateJoinExpression:
		return 20
	case *model.AnonymousFunctionExpression, *model.LiteralValueExpression, *model.ObjectConsExpression,
		*model.ScopeTraversalExpression, *model.TemplateExpression, *model.TupleConsExpression:
		return 22
	default:
		contract.Failf("unexpected expression %v of type %T", expr, expr)
	}
	return 0
}

func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) {
	switch len(expr.Signature.Parameters) {
	case 0:
		g.Fgen(w, "()")
	case 1:
		g.Fgenf(w, "%s", expr.Signature.Parameters[0].Name)
	default:
		g.Fgen(w, "([")
		for i, p := range expr.Signature.Parameters {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%s", p.Name)
		}
		g.Fgen(w, "])")
	}

	g.Fgenf(w, " => %.v", expr.Body)
}

func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) {
	var opstr string
	precedence := g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpAdd:
		opstr = "+"
	case hclsyntax.OpDivide:
		opstr = "/"
	case hclsyntax.OpEqual:
		opstr = "=="
	case hclsyntax.OpGreaterThan:
		opstr = ">"
	case hclsyntax.OpGreaterThanOrEqual:
		opstr = ">="
	case hclsyntax.OpLessThan:
		opstr = "<"
	case hclsyntax.OpLessThanOrEqual:
		opstr = "<="
	case hclsyntax.OpLogicalAnd:
		opstr = "&&"
	case hclsyntax.OpLogicalOr:
		opstr = "||"
	case hclsyntax.OpModulo:
		opstr = "%"
	case hclsyntax.OpMultiply:
		opstr = "*"
	case hclsyntax.OpNotEqual:
		opstr = "!="
	case hclsyntax.OpSubtract:
		opstr = "-"
	default:
		opstr, precedence = ",", 1
	}

	g.Fgenf(w, "%.[1]*[2]v %[3]v %.[1]*[4]o", precedence, expr.LeftOperand, opstr, expr.RightOperand)
}

func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) {
	g.Fgenf(w, "%.4v ? %.4v : %.4v", expr.Condition, expr.TrueResult, expr.FalseResult)
}

func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	switch expr.Collection.Type().(type) {
	case *model.ListType, *model.TupleType:
		if expr.KeyVariable == nil {
			g.Fgenf(w, "%.20v", expr.Collection)
		} else {
			g.Fgenf(w, "%.20v.map((v, k) => [k, v])", expr.Collection)
		}
	case *model.MapType, *model.ObjectType:
		if expr.KeyVariable == nil {
			g.Fgenf(w, "Object.values(%.v)", expr.Collection)
		} else {
			g.Fgenf(w, "Object.entries(%.v)", expr.Collection)
		}
	}

	fnParams, reduceParams := expr.ValueVariable.Name, expr.ValueVariable.Name
	if expr.KeyVariable != nil {
		reduceParams = fmt.Sprintf("[%s, %s]", expr.KeyVariable.Name, expr.ValueVariable.Name)
		fnParams = fmt.Sprintf("(%s)", reduceParams)
	}

	if expr.Condition != nil {
		g.Fgenf(w, ".filter(%s => %.v)", fnParams, expr.Condition)
	}

	if expr.Key != nil {
		// TODO(pdg): grouping
		g.Fgenf(w, ".reduce((__obj, %s) => ({ ...__obj, [%.v]: %.v }))", reduceParams, expr.Key, expr.Value)
	} else {
		g.Fgenf(w, ".map(%s => (%.v))", fnParams, expr.Value)
	}
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := pcl.ParseApplyCall(expr)

	// If all of the arguments are promises, use promise methods. If any argument is an output, convert all other args
	// to outputs and use output methods.
	anyOutputs := false
	for _, arg := range applyArgs {
		if isOutputType(arg.Type()) {
			anyOutputs = true
		}
	}

	apply, all := "then", "Promise.all"
	if anyOutputs {
		apply, all = "apply", "pulumi.all"
	}

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply` or `.then`.
		g.Fgenf(w, "%.20v.%v(%.v)", applyArgs[0], apply, then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgenf(w, "%v([", all)
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%v", o)
		}
		g.Fgenf(w, "]).%v(%.v)", apply, then)
	}
}

// functionName computes the NodeJS package, module, and name for the given function token.
func functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := pcl.DecomposeToken(token, tokenRange)
	return pkg, strings.ReplaceAll(module, "/", "."), member, diagnostics
}

func (g *generator) genRange(w io.Writer, call *model.FunctionCallExpression, entries bool) {
	var from, to model.Expression
	switch len(call.Args) {
	case 1:
		from, to = &model.LiteralValueExpression{Value: cty.NumberIntVal(0)}, call.Args[0]
	case 2:
		from, to = call.Args[0], call.Args[1]
	default:
		contract.Failf("expected range() to have exactly 1 or 2 args; got %v", len(call.Args))
	}

	genPrefix := func() { g.Fprint(w, "((from, to) => (new Array(to - from))") }
	mapValue := "from + i"
	genSuffix := func() { g.Fgenf(w, ")(%.v, %.v)", from, to) }

	if litFrom, ok := from.(*model.LiteralValueExpression); ok {
		fromV, err := convert.Convert(litFrom.Value, cty.Number)
		contract.AssertNoErrorf(err, "conversion of %v to number failed", litFrom.Value.Type())

		from, _ := fromV.AsBigFloat().Int64()
		if litTo, ok := to.(*model.LiteralValueExpression); ok {
			toV, err := convert.Convert(litTo.Value, cty.Number)
			contract.AssertNoErrorf(err, "conversion of %v to number failed", litTo.Value.Type())

			to, _ := toV.AsBigFloat().Int64()
			if from == 0 {
				mapValue = "i"
			} else {
				mapValue = fmt.Sprintf("%d + i", from)
			}
			genPrefix = func() { g.Fprintf(w, "(new Array(%d))", to-from) }
			genSuffix = func() {}
		} else if from == 0 {
			genPrefix = func() { g.Fgenf(w, "(new Array(%.v))", to) }
			mapValue = "i"
			genSuffix = func() {}
		}
	}

	if entries {
		mapValue = fmt.Sprintf("{key: %[1]s, value: %[1]s}", mapValue)
	}

	genPrefix()
	g.Fprintf(w, ".map((_, i) => %v)", mapValue)
	genSuffix()
}

var functionImports = map[string][]string{
	intrinsicInterpolate: {"@pulumi/pulumi"},
	"fileArchive":        {"@pulumi/pulumi"},
	"remoteArchive":      {"@pulumi/pulumi"},
	"assetArchive":       {"@pulumi/pulumi"},
	"fileAsset":          {"@pulumi/pulumi"},
	"stringAsset":        {"@pulumi/pulumi"},
	"remoteAsset":        {"@pulumi/pulumi"},
	"rootDirectory":      {"@pulumi/pulumi"},
	"filebase64":         {"fs"},
	"filebase64sha256":   {"fs", "crypto"},
	"readFile":           {"fs"},
	"readDir":            {"fs"},
	"sha1":               {"crypto"},
}

func (g *generator) visitFunctionImports(
	x *model.FunctionCallExpression,
	visitNodeImport func(nodeImportString string),
	visitPackageImport func(pkg string),
) {
	if x.Name != pcl.Invoke {
		for _, i := range functionImports[x.Name] {
			visitNodeImport(i)
		}
		return
	}

	pkg, _, _, diags := functionName(x.Args[0])
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	visitPackageImport(pkg)
}

func enumName(enum *model.EnumType) (string, error) {
	e, ok := pcl.GetSchemaForType(enum)
	if !ok {
		return "", errors.New("Could not get associated enum")
	}
	pkgRef := e.(*schema.EnumType).PackageReference
	return enumNameWithPackage(enum.Token, pkgRef)
}

func enumNameWithPackage(enumToken string, pkgRef schema.PackageReference) (string, error) {
	components := strings.Split(enumToken, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", enumToken)
	name := tokenToName(enumToken)
	pkg := makeValidIdentifier(components[0])
	if mod := components[1]; mod != "" && mod != "index" {
		// if the token has the format {pkg}:{mod}/{name}:{Name}
		// then we simplify into {pkg}:{mod}:{Name}
		modParts := strings.Split(mod, "/")
		if len(modParts) == 2 && strings.EqualFold(modParts[1], components[2]) {
			mod = modParts[0]
		}
		if pkgRef != nil {
			mod = moduleName(mod, pkgRef)
		}
		pkg += "." + mod
	}
	return fmt.Sprintf("%s.%s", pkg, name), nil
}

func (g *generator) genEntries(w io.Writer, expr *model.FunctionCallExpression) {
	entriesArg := expr.Args[0]
	entriesArgType := pcl.UnwrapOption(model.ResolveOutputs(entriesArg.Type()))
	switch entriesArgType.(type) {
	case *model.ListType, *model.TupleType:
		if call, ok := expr.Args[0].(*model.FunctionCallExpression); ok && call.Name == "range" {
			g.genRange(w, call, true)
			return
		}
		// Mapping over a list with a tuple receiver accepts (value, index).
		g.Fgenf(w, "%.20v.map((v, k)", expr.Args[0])
	case *model.MapType, *model.ObjectType:
		g.Fgenf(w, "Object.entries(%.v).map(([k, v])", expr.Args[0])
	case *model.OpaqueType:
		if entriesArgType.Equals(model.DynamicType) {
			g.Fgenf(w, "Object.entries(%.v).map(([k, v])", expr.Args[0])
		}
	}
	g.Fgenf(w, " => ({key: k, value: v}))")
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case pcl.IntrinsicConvert:
		from := expr.Args[0]
		to := pcl.LowerConversion(from, expr.Signature.ReturnType)
		output, isOutput := to.(*model.OutputType)
		if isOutput {
			to = output.ElementType
		}
		switch to := to.(type) {
		case *model.EnumType:
			if enum, err := enumName(to); err == nil {
				if isOutput {
					g.Fgenf(w, "%.v.apply((x) => %s[x])", from, enum)
				} else {
					diag := pcl.GenEnum(to, from, func(member *schema.Enum) {
						memberTag, err := enumMemberName(tokenToName(to.Token), member)
						contract.AssertNoErrorf(err, "Failed to get member name on enum '%s'", enum)
						g.Fgenf(w, "%s.%s", enum, memberTag)
					}, func(from model.Expression) {
						g.Fgenf(w, "%s[%.v]", enum, from)
					})
					if diag != nil {
						g.diagnostics = append(g.diagnostics, diag)
					}
				}
			} else {
				g.Fgenf(w, "%v", from)
			}
		default:
			g.Fgenf(w, "%v", from)
		}
	case pcl.IntrinsicApply:
		g.genApply(w, expr)
	case intrinsicAwait:
		g.Fgenf(w, "await %.17v", expr.Args[0])
	case intrinsicInterpolate:
		g.Fgen(w, "pulumi.interpolate`")
		for _, part := range expr.Args {
			if lit, ok := part.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
				g.Fgen(w, lit.Value.AsString())
			} else {
				g.Fgenf(w, "${%.v}", part)
			}
		}
		g.Fgen(w, "`")
	case "element":
		g.Fgenf(w, "%.20v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		g.genEntries(w, expr)
	case "fileArchive":
		g.Fgenf(w, "new pulumi.asset.FileArchive(%.v)", expr.Args[0])
	case "remoteArchive":
		g.Fgenf(w, "new pulumi.asset.RemoteArchive(%.v)", expr.Args[0])
	case "assetArchive":
		g.Fgenf(w, "new pulumi.asset.AssetArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "new pulumi.asset.FileAsset(%.v)", expr.Args[0])
	case "stringAsset":
		g.Fgenf(w, "new pulumi.asset.StringAsset(%.v)", expr.Args[0])
	case "remoteAsset":
		g.Fgenf(w, "new pulumi.asset.RemoteAsset(%.v)", expr.Args[0])
	case "filebase64":
		g.Fgenf(w, "fs.readFileSync(%v, { encoding: \"base64\" })", expr.Args[0])
	case "filebase64sha256":
		// Assuming the existence of the following helper method
		g.Fgenf(w, "computeFilebase64sha256(%v)", expr.Args[0])
	case "notImplemented":
		g.Fgenf(w, "notImplemented(%v)", expr.Args[0])
	case "singleOrNone":
		g.Fgenf(w, "singleOrNone(%v)", expr.Args[0])
	case "mimeType":
		g.Fgenf(w, "mimeType(%v)", expr.Args[0])
	case pcl.Call:
		self := expr.Args[0]
		method := expr.Args[1].(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()

		if expr.Signature.MultiArgumentInputs {
			err := fmt.Errorf("nodejs program-gen does not implement MultiArgumentInputs for method '%s'", method)
			panic(err)
		}

		validMethod := makeValidIdentifier(method)
		g.Fgenf(w, "%v.%s(", self, validMethod)

		var args *model.ObjectConsExpression
		if converted, objectArgs, _ := pcl.RecognizeTypedObjectCons(expr.Args[2]); converted {
			args = objectArgs
		} else {
			args = expr.Args[2].(*model.ObjectConsExpression)
		}
		if len(args.Items) > 0 {
			g.Fgen(w, args)
		}

		g.Fprint(w, ")")
	case pcl.Invoke:
		pkg, module, fn, diags := functionName(expr.Args[0])
		contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
		if module != "" {
			module = "." + module
		}
		isOut := pcl.IsOutputVersionInvokeCall(expr)
		name := fmt.Sprintf("%s%s.%s", makeValidIdentifier(pkg), module, fn)
		if isOut {
			name = name + "Output"
		}
		g.Fprintf(w, "%s(", name)
		if len(expr.Args) >= 2 {
			if expr.Signature.MultiArgumentInputs {
				var invokeArgs *model.ObjectConsExpression
				// extract invoke args in case we have the form invoke("token", __convert(args))
				if converted, objectArgs, _ := pcl.RecognizeTypedObjectCons(expr.Args[1]); converted {
					invokeArgs = objectArgs
				} else {
					// otherwise, we have the form invoke("token", args)
					invokeArgs = expr.Args[1].(*model.ObjectConsExpression)
				}

				pcl.GenerateMultiArguments(g.Formatter, w, "undefined", invokeArgs, pcl.SortedFunctionParameters(expr))
			} else {
				g.Fgenf(w, "%.v", expr.Args[1])
			}
		}
		if len(expr.Args) == 3 {
			if invokeOptions, ok := expr.Args[2].(*model.ObjectConsExpression); ok {
				g.Fgen(w, ", {")
				g.Indented(func() {
					for _, item := range invokeOptions.Items {
						key := pcl.LiteralValueString(item.Key)
						g.Fgenf(w, "\n%s", g.Indent)
						switch key {
						case "pluginDownloadUrl":
							// the casing of the key is important here so we special case pluginDownloadURL
							// in PCL it is pluginDownloadURL, but in TS it is pluginDownloadUrl
							g.Fgenf(w, "pluginDownloadURL: %v,", item.Value)
						default:
							g.Fgenf(w, "%s: %v,", key, item.Value)
						}
					}
				})
				g.Fgenf(w, "\n%s}", g.Indent)
			}
		}
		g.Fprint(w, ")")
	case "join":
		g.Fgenf(w, "%.20v.join(%v)", expr.Args[1], expr.Args[0])
	case "length":
		g.Fgenf(w, "%.20v.length", expr.Args[0])
	case "lookup":
		g.Fgenf(w, "%v[%v]", expr.Args[0], expr.Args[1])
		if len(expr.Args) == 3 {
			g.Fgenf(w, " || %v", expr.Args[2])
		}
	case "range":
		g.genRange(w, expr, false)
	case "readFile":
		g.Fgenf(w, "fs.readFileSync(%v, \"utf8\")", expr.Args[0])
	case "readDir":
		g.Fgenf(w, "fs.readdirSync(%v)", expr.Args[0])
	case "secret":
		g.Fgenf(w, "pulumi.secret(%v)", expr.Args[0])
	case "unsecret":
		g.Fgenf(w, "pulumi.unsecret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.20v.split(%v)", expr.Args[1], expr.Args[0])
	case "toBase64":
		g.Fgenf(w, "Buffer.from(%v).toString(\"base64\")", expr.Args[0])
	case "fromBase64":
		g.Fgenf(w, "Buffer.from(%v, \"base64\").toString(\"utf8\")", expr.Args[0])
	case "toJSON":
		if model.ContainsOutputs(expr.Args[0].Type()) {
			g.Fgenf(w, "pulumi.jsonStringify(%v)", expr.Args[0])
		} else {
			g.Fgenf(w, "JSON.stringify(%v)", expr.Args[0])
		}
	case "sha1":
		g.Fgenf(w, "crypto.createHash('sha1').update(%v).digest('hex')", expr.Args[0])
	case "stack":
		g.Fgenf(w, "pulumi.getStack()")
	case "project":
		g.Fgenf(w, "pulumi.getProject()")
	case "organization":
		g.Fgenf(w, "pulumi.getOrganization()")
	case "cwd":
		g.Fgen(w, "process.cwd()")
	case "getOutput":
		g.Fgenf(w, "%s.getOutput(%v)", expr.Args[0], expr.Args[1])
	case "try":
		g.genTry(w, expr)
	case "can":
		g.genCan(w, expr)
	case "rootDirectory":
		g.genRootDirectory(w)
	case "pulumiResourceName":
		g.Fgenf(w, "pulumi.resourceName(%v)", expr.Args[0])
	case "pulumiResourceType":
		g.Fgenf(w, "pulumi.resourceType(%v)", expr.Args[0])
	default:
		var rng hcl.Range
		if expr.Syntax != nil {
			rng = expr.Syntax.Range()
		}
		g.genNYI(w, "FunctionCallExpression: %v (%v)", expr.Name, rng)
	}
}

// genTry generates code for a `try` expression. Each argument is transformed into a closure to prevent its evaluation
// (which may fail) from happening until the `try_` utility function chooses. Since the whole point of `try` is to
// support unsafe expressions that may fail, we also disable type checking for the arguments. This results in an
// expression of the form:
//
//	try_(
//	    () => <arg1>,
//	    () => <arg2>,
//	    ...
//	)
func (g *generator) genTry(w io.Writer, expr *model.FunctionCallExpression) {
	args := expr.Args
	contract.Assertf(len(args) > 0, "expected at least one argument to try")
	_, shouldUseOutputTry := expr.Signature.ReturnType.(*model.OutputType)

	functionName := "try_"
	if shouldUseOutputTry {
		functionName = "tryOutput_"
	}

	g.Fprintf(w, "%s(", functionName)
	for i, arg := range args {
		g.Indented(func() {
			g.Fgenf(w, "\n%s() => %v", g.Indent, g.lowerExpression(arg, arg.Type()))
		})
		if i < len(args)-1 {
			g.Fgen(w, ",")
		} else {
			g.Fgen(w, "\n")
		}
	}
	g.Fprintf(w, "%s)", g.Indent)
}

// genCan generates code for a `can` expression.  Much like try, it attempts to
// run the code by transforming it into a closure to prevent its evaluation
// which may fail until the `can_` utility function chooses to run it (catching the potential error).
// We also disable type checking for the arguments, resulting in expression of the form:
//
//	can_(
//	    () => <arg1
//	)
//
// which returns a bool indicating if the closure ran successfully.
func (g *generator) genCan(w io.Writer, expr *model.FunctionCallExpression) {
	args := expr.Args
	contract.Assertf(len(args) == 1, "expected exactly one argument to can")
	_, shouldUseOutputCan := expr.Signature.ReturnType.(*model.OutputType)

	functionName := "can_"
	if shouldUseOutputCan {
		functionName = "canOutput_"
	}

	arg := args[0]
	g.Fgenf(w, "\n%s(() => %v)", functionName, g.lowerExpression(arg, arg.Type()))
}

func (g *generator) genRootDirectory(w io.Writer) {
	g.Fgen(w, "pulumi.runtime.getRootDirectory()")
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.20v[%.v]", expr.Collection, expr.Key)
}

func escapeRune(c rune) string {
	if uint(c) <= 0xFF {
		return fmt.Sprintf("\\x%02x", c)
	} else if uint(c) <= 0xFFFF {
		return fmt.Sprintf("\\u%04x", c)
	}
	return fmt.Sprintf("\\u{%x}", c)
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	builder := strings.Builder{}
	newlines := strings.Count(v, "\n")
	if newlines == 0 || newlines == 1 && (v[0] == '\n' || v[len(v)-1] == '\n') {
		// This string either does not contain newlines or contains a single leading or trailing newline, so we'll
		// Generate a normal string literal. Quotes, backslashes, and newlines will be escaped in conformance with
		// ECMA-262 11.8.4 ("String Literals").
		builder.WriteRune('"')
		for _, c := range v {
			if c == '"' || c == '\\' {
				builder.WriteRune('\\')
				builder.WriteRune(c)
			} else if c == '\n' {
				builder.WriteString(`\n`)
			} else if unicode.IsPrint(c) {
				builder.WriteRune(c)
			} else {
				// This is a non-printable character. We'll emit an escape sequence for it.
				builder.WriteString(escapeRune(c))
			}
		}
		builder.WriteRune('"')
	} else {
		// This string does contain newlines, so we'll Generate a template string literal. "${", backquotes, and
		// backslashes will be escaped in conformance with ECMA-262 11.8.6 ("Template Literal Lexical Components").
		runes := []rune(v)
		builder.WriteRune('`')
		for i, c := range runes {
			if c == '`' || c == '\\' {
				builder.WriteRune('\\')
				builder.WriteRune(c)
			} else if c == '$' {
				if i < len(runes)-1 && runes[i+1] == '{' {
					builder.WriteRune('\\')
					builder.WriteRune('$')
				}
			} else if c == '\n' {
				builder.WriteRune('\n')
			} else if unicode.IsPrint(c) {
				builder.WriteRune(c)
			} else {
				// This is a non-printable character. We'll emit an escape sequence for it.
				builder.WriteString(escapeRune(c))
			}
		}
		builder.WriteRune('`')
	}

	g.Fgenf(w, "%s", builder.String())
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	typ := expr.Type()
	if cns, ok := typ.(*model.ConstType); ok {
		typ = cns.Type
	}

	switch typ {
	case model.BoolType:
		g.Fgenf(w, "%v", expr.Value.True())
	case model.NoneType:
		g.Fgen(w, "null")
	case model.NumberType:
		bf := expr.Value.AsBigFloat()
		if i, acc := bf.Int64(); acc == big.Exact {
			g.Fgenf(w, "%d", i)
		} else {
			f, _ := bf.Float64()
			g.Fgenf(w, "%g", f)
		}
	case model.StringType:
		g.genStringLiteral(w, expr.Value.AsString())
	default:
		contract.Failf("unexpected literal type in GenLiteralValueExpression: %v (%v)", expr.Type(),
			expr.SyntaxNode().Range())
	}
}

func (g *generator) literalKey(x model.Expression) (string, bool) {
	var strKey string
	switch x := x.(type) {
	case *model.LiteralValueExpression:
		if model.StringType.AssignableFrom(x.Type()) {
			strKey = x.Value.AsString()
			break
		}
		var buf bytes.Buffer
		g.GenLiteralValueExpression(&buf, x)
		return buf.String(), true
	case *model.TemplateExpression:
		if len(x.Parts) == 1 {
			if lit, ok := x.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
				strKey = lit.Value.AsString()
				break
			}
		}
		return "", false
	default:
		return "", false
	}

	if isLegalIdentifier(strKey) {
		return strKey, true
	}
	return fmt.Sprintf("%q", strKey), true
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	if len(expr.Items) == 0 {
		g.Fgen(w, "{}")
	} else {
		g.Fgen(w, "{")
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "\n%s", g.Indent)
				if lit, ok := g.literalKey(item.Key); ok {
					g.Fgenf(w, "%s", lit)
				} else {
					g.Fgenf(w, "[%.v]", item.Key)
				}
				g.Fgenf(w, ": %.v,", item.Value)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer, traversal hcl.Traversal, parts []model.Traversable) {
	for i, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
		case hcl.TraverseIndex:
			key = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
		}

		var indexPrefix string
		if model.IsOptionalType(model.GetTraversableType(parts[i])) {
			g.Fgen(w, "?")
			// `expr?[expr]` is not valid typescript, since it looks like a ternary
			// operator.
			//
			// Typescript solves this by inserting a `.` in before the `[`: `expr?.[expr]`
			//
			// We need to do the same when generating index based expressions.
			indexPrefix = "."
		}

		genIndex := func(inner string, value interface{}) {
			g.Fgenf(w, "%s["+inner+"]", indexPrefix, value)
		}

		switch key.Type() {
		case cty.String:
			keyVal := key.AsString()
			if isLegalIdentifier(keyVal) {
				g.Fgenf(w, ".%s", keyVal)
			} else {
				genIndex("%q", keyVal)
			}
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			genIndex("%d", idx)
		default:
			genIndex("%q", key.AsString())
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.20v", expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	rootName := makeValidIdentifier(expr.RootName)
	if g.isComponent {
		if expr.RootName == "this" {
			// special case for parent: this
			g.Fgenf(w, "%s", expr.RootName)
			return
		}

		configVars := map[string]*pcl.ConfigVariable{}
		for _, configVar := range g.program.ConfigVariables() {
			configVars[configVar.Name()] = configVar
		}

		if _, isConfig := configVars[expr.RootName]; isConfig {
			if _, configReference := expr.Parts[0].(*pcl.ConfigVariable); configReference {
				rootName = "args." + expr.RootName
			}
		}
	}

	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "__item"
	}

	g.Fgen(w, rootName)
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.Fgenf(w, "%.20v.map(__item => %.v)", expr.Source, expr.Each)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	if len(expr.Parts) == 1 {
		if lit, ok := expr.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.GenLiteralValueExpression(w, lit)
			return
		}
	}

	g.Fgen(w, "`")
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.Fgen(w, lit.Value.AsString())
		} else {
			g.Fgenf(w, "${%.v}", expr)
		}
	}
	g.Fgen(w, "`")
}

func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) {
	g.genNYI(w, "TemplateJoinExpression")
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	switch len(expr.Expressions) {
	case 0:
		g.Fgen(w, "[]")
	case 1:
		g.Fgenf(w, "[%.v]", expr.Expressions[0])
	default:
		g.Fgen(w, "[")
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%.v,", g.Indent, v)
			}
		})
		g.Fgen(w, "\n", g.Indent, "]")
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	var opstr string
	precedence := g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}
