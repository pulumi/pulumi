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

package dotnet

import (
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return makeValidIdentifier(name)
}

func (g *generator) rewriteExpression(expr model.Expression, typ model.Type, rewriteApplies bool) model.Expression {
	expr = pcl.RewritePropertyReferences(expr)
	var diags hcl.Diagnostics
	if rewriteApplies {
		skipToJSONWhenRewritingApplies := true
		expr, diags = pcl.RewriteAppliesWithSkipToJSON(expr, nameInfo(0), !g.asyncInit, skipToJSONWhenRewritingApplies)
	}

	expr, convertDiags := pcl.RewriteConversions(expr, typ)
	diags = diags.Extend(convertDiags)
	if g.asyncInit {
		expr = g.awaitInvokes(expr)
	} else {
		expr = g.outputInvokes(expr)
	}
	g.diagnostics = g.diagnostics.Extend(diags)
	return expr
}

// lowerExpression amends the expression with intrinsics for C# generation.
func (g *generator) lowerExpression(expr model.Expression, typ model.Type) model.Expression {
	rewriteApplies := true
	return g.rewriteExpression(expr, typ, rewriteApplies)
}

// lowerExpressionWithoutApplies is the same as lowerExpression
// but without rewriting applies. Made especially for function invokes that are returning outputs
func (g *generator) lowerExpressionWithoutApplies(expr model.Expression, typ model.Type) model.Expression {
	rewriteApplies := false
	return g.rewriteExpression(expr, typ, rewriteApplies)
}

// awaitInvokes wraps each call to `invoke` with a call to the `await` intrinsic. This rewrite should only be used
// if we are generating an async Initialize, in which case the apply rewriter should also be configured not to treat
// promises as eventuals. Note that this depends on the fact that invokes are the only way to introduce promises
// in to a Pulumi program; if this changes in the future, this transform will need to be applied in a more general way
// (e.g. by the apply rewriter).
func (g *generator) awaitInvokes(x model.Expression) model.Expression {
	contract.Assertf(g.asyncInit,
		"awaitInvokes can be used only if we are generating an async Initialize")

	rewriter := func(x model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to invoke.
		call, ok := x.(*model.FunctionCallExpression)
		if !ok || call.Name != pcl.Invoke {
			return x, nil
		}

		if _, isPromise := call.Type().(*model.PromiseType); isPromise {
			return newAwaitCall(call), nil
		}

		return call, nil
	}
	x, diags := model.VisitExpression(x, model.IdentityVisitor, rewriter)
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	return x
}

// outputInvokes wraps each call to `invoke` with a call to the `output` intrinsic. This rewrite should only be used if
// resources are instantiated within a stack constructor, where `await` operator is not available. We want to avoid the
// nastiness of working with raw `Task` and wrap it into Pulumi's Output immediately to be able to `Apply` on it.
// Note that this depends on the fact that invokes are the only way to introduce promises
// in to a Pulumi program; if this changes in the future, this transform will need to be applied in a more general way
// (e.g. by the apply rewriter).
func (g *generator) outputInvokes(x model.Expression) model.Expression {
	rewriter := func(x model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to invoke.
		call, ok := x.(*model.FunctionCallExpression)
		if !ok || call.Name != pcl.Invoke {
			return x, nil
		}

		if call.Type() == model.DynamicType {
			// ignore if the return type of the invoke is dynamic
			// this means that we are working with an unknown invoke
			return x, nil
		}

		_, isOutput := call.Type().(*model.OutputType)
		if isOutput {
			return x, nil
		}

		_, isPromise := call.Type().(*model.PromiseType)
		contract.Assertf(isPromise, "invoke should return a promise, got %v", call.Type())

		return newOutputCall(call), nil
	}
	x, diags := model.VisitExpression(x, model.IdentityVisitor, rewriter)
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	return x
}

func (g *generator) GetPrecedence(expr model.Expression) int {
	// TODO(msh): Current values copied from Node, update based on
	// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/operators/
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
		g.Fgenf(w, " => %v", expr.Body)
	default:
		g.Fgen(w, "values =>\n")
		g.Fgenf(w, "%s{\n", g.Indent)
		g.Indented(func() {
			for i, p := range expr.Signature.Parameters {
				g.Fgenf(w, "%svar %s = values.Item%d;\n", g.Indent, p.Name, i+1)
			}
			g.Fgenf(w, "%sreturn %v;\n", g.Indent, expr.Body)
		})
		g.Fgenf(w, "%s}", g.Indent)
	}
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
			g.Fgenf(w, "%.20v.Select((value, i) => new { Key = i.ToString(), Value = pair.Value })",
				expr.Collection)
		}
	case *model.MapType:
		if expr.KeyVariable == nil {
			g.Fgenf(w, "(%.v).Values", expr.Collection)
		} else {
			g.Fgenf(w, "%.20v.Select(pair => new { pair.Key, pair.Value })", expr.Collection)
		}
	}

	switch expr.Type().(type) {
	case *model.ListType:
		// the result of the expression is a list
		if expr.Condition != nil {
			g.Fgenf(w, ".Where(%s => %.v)", expr.ValueVariable.Name, expr.Condition)
		}

		g.Fgenf(w, ".Select(%s => \n", expr.ValueVariable.Name)

		g.Fgenf(w, "%s{\n", g.Indent)
		g.Indented(func() {
			g.Fgenf(w, "%sreturn %v;", g.Indent, expr.Value)
		})
		g.Fgen(w, "\n")
		// .ToList() is added so that the expressions returns `List<T>
		// which can be implicitly converted to InputList<T>
		g.Fgenf(w, "%s}).ToList()", g.Indent)
	case *model.MapType:
		// the result of the expression is a dictionary
		g.Fgen(w, ".ToDictionary(item => {\n")
		g.Indented(func() {
			if expr.KeyVariable != nil && pcl.VariableAccessed(expr.KeyVariable.Name, expr.Key) {
				g.Fgenf(w, "%svar %s = item.Key;\n", g.Indent, expr.KeyVariable.Name)
			}

			if expr.ValueVariable != nil && pcl.VariableAccessed(expr.ValueVariable.Name, expr.Key) {
				g.Fgenf(w, "%svar %s = item.Value;\n", g.Indent, expr.ValueVariable.Name)
			}

			g.Fgenf(w, "%sreturn %s;\n", g.Indent, expr.Key)
		})

		g.Fgenf(w, "%s}, item => {\n", g.Indent)
		g.Indented(func() {
			if expr.KeyVariable != nil && pcl.VariableAccessed(expr.KeyVariable.Name, expr.Value) {
				g.Fgenf(w, "%svar %s = item.Key;\n", g.Indent, expr.KeyVariable.Name)
			}

			if expr.ValueVariable != nil && pcl.VariableAccessed(expr.ValueVariable.Name, expr.Value) {
				g.Fgenf(w, "%svar %s = item.Value;\n", g.Indent, expr.ValueVariable.Name)
			}

			g.Fgenf(w, "%sreturn %v;\n", g.Indent, expr.Value)
		})

		g.Fgenf(w, "%s})", g.Indent)
	}
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := pcl.ParseApplyCall(expr)

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.Apply`
		g.Fgenf(w, "%.v.Apply(%.v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `Output.Tuple().Apply()`.
		g.Fgen(w, "Output.Tuple(")
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%.v", o)
		}

		g.Fgenf(w, ").Apply(%.v)", then)
	}
}

func (g *generator) genRange(w io.Writer, call *model.FunctionCallExpression, entries bool) {
	g.genNYI(w, "Range %.v %.v", call, entries)
}

var functionNamespaces = map[string][]string{
	"assetArchive":     {"System.Collections.Generic"},
	"readDir":          {"System.IO", "System.Linq"},
	"readFile":         {"System.IO"},
	"cwd":              {"System.IO"},
	"filebase64":       {"System", "System.IO"},
	"filebase64sha256": {"System", "System.IO", "System.Security.Cryptography", "System.Text"},
	"toJSON":           {"System.Text.Json", "System.Collections.Generic"},
	"toBase64":         {"System"},
	"fromBase64":       {"System"},
	"sha1":             {"System.Security.Cryptography", "System.Text"},
	"singleOrNone":     {"System.Linq"},
}

func (g *generator) genFunctionUsings(x *model.FunctionCallExpression) []string {
	if x.Name != pcl.Invoke {
		return functionNamespaces[x.Name]
	}

	pkg, _ := g.functionName(x.Args[0])
	return []string{fmt.Sprintf("%s = Pulumi.%[1]s", pkg)}
}

func (g *generator) genSafeEnum(w io.Writer, to *model.EnumType) func(member *schema.Enum) {
	return func(member *schema.Enum) {
		// We know the enum value at the call site, so we can directly stamp in a
		// valid enum instance. We don't need to convert.
		pkg, name := enumName(to)
		contract.Assertf(pkg != "", "pkg cannot be empty")
		contract.Assertf(name != "", "name cannot be empty")
		memberTag := member.Name
		if memberTag == "" {
			memberTag = member.Value.(string)
		}
		memberTag, err := makeSafeEnumName(memberTag, name)
		contract.AssertNoErrorf(err, "Enum is invalid")
		g.Fgenf(w, "%s.%s.%s", pkg, name, memberTag)
	}
}

func enumName(enum *model.EnumType) (string, string) {
	components := strings.Split(enum.Token, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", enum.Token)
	modParts := strings.Split(components[1], "/")
	// if the token has the format {pkg}:{mod}/{name}:{Name}
	// then we simplify into {pkg}:{mod}:{Name}
	if len(modParts) == 2 && strings.EqualFold(modParts[1], components[2]) {
		components[1] = modParts[0]
	}
	enumName := tokenToName(enum.Token)
	e, ok := pcl.GetSchemaForType(enum)
	if !ok {
		return "", ""
	}
	et := e.(*schema.EnumType)
	def, err := et.PackageReference.Definition()
	contract.AssertNoErrorf(err, "error loading definition for package %q", et.PackageReference.Name())
	var namespaceMap map[string]string
	pkgInfo, ok := def.Language["csharp"].(CSharpPackageInfo)
	if ok {
		namespaceMap = pkgInfo.Namespaces
	}

	namespace := namespaceName(namespaceMap, components[0])
	if components[1] != "" && components[1] != "index" {
		namespace += "." + namespaceName(namespaceMap, components[1])
	}
	return namespace, enumName
}

func (g *generator) genIntrensic(w io.Writer, from model.Expression, to model.Type) {
	to = pcl.LowerConversion(from, to)
	output, isOutput := to.(*model.OutputType)
	if isOutput {
		to = output.ElementType
	}
	switch to := to.(type) {
	case *model.EnumType:
		pkg, name := enumName(to)
		if pkg == "" || name == "" {
			// Something has gone wrong. Produce a best effort result.
			g.Fgenf(w, "%.v", from)
			return
		}

		convertFn := func() string {
			if to.Type.Equals(model.StringType) {
				return fmt.Sprintf("System.Enum.Parse<%s.%s>", pkg, name)
			}

			panic(fmt.Sprintf(
				"Unsafe enum conversions from type %s not implemented yet: %s => %s",
				from.Type(), from, to))
		}

		if isOutput {
			g.Fgenf(w, "%.v.Apply(%s)", from, convertFn())
		} else {
			diag := pcl.GenEnum(to, from, g.genSafeEnum(w, to), func(from model.Expression) {
				g.Fgenf(w, "%s(%v)", convertFn(), from)
			})
			if diag != nil {
				g.diagnostics = append(g.diagnostics, diag)
			}
		}
	default:
		g.Fgenf(w, "%.v", from) // <- probably wrong w.r.t. precedence
	}
}

func (g *generator) genEntries(w io.Writer, expr *model.FunctionCallExpression) {
	switch model.ResolveOutputs(expr.Args[0].Type()).(type) {
	case *model.ListType, *model.TupleType:
		if call, ok := expr.Args[0].(*model.FunctionCallExpression); ok && call.Name == "range" {
			g.genRange(w, call, true)
			return
		}
		g.Fgenf(w, "%.20v.Select((v, k) => new { Key = k, Value = v })", expr.Args[0])
	case *model.MapType, *model.ObjectType:
		g.Fgenf(w, "%.20v.Select(pair => new { pair.Key, pair.Value })", expr.Args[0])
	}
}

func (g *generator) withinAwaitBlock(run func()) {
	if g.insideAwait {
		// already inside await block?
		// only run the function
		run()
	} else {
		// not inside await? flag it as true, run the function,
		// then set it back to false
		g.insideAwait = true
		run()
		g.insideAwait = false
	}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case pcl.IntrinsicConvert:
		switch arg := expr.Args[0].(type) {
		case *model.ObjectConsExpression:
			g.genObjectConsExpression(w, arg, expr.Type())
		default:
			g.genIntrensic(w, expr.Args[0], expr.Signature.ReturnType)
		}
	case pcl.IntrinsicApply:
		switch expr.Args[0].(type) {
		case *model.ScopeTraversalExpression:
			traversal := expr.Args[0].(*model.ScopeTraversalExpression)
			if len(traversal.Parts) == 1 {
				_, isInvoke := g.functionInvokes[traversal.RootName]
				if isInvoke {
					switch expr.Args[1].(type) {
					case *model.AnonymousFunctionExpression:
						anonFunction := expr.Args[1].(*model.AnonymousFunctionExpression)
						g.Fgenf(w, "%v", anonFunction.Body)
						return
					}
				}
			}
		}

		g.genApply(w, expr)
	case intrinsicAwait:
		g.withinAwaitBlock(func() {
			g.Fgenf(w, "await %.17v", expr.Args[0])
		})

	case intrinsicOutput:
		// if we are calling Output.Create(FuncInvokeAsync())
		// then we can simplify to just FuncInvoke() which already returns Output
		if funcExpr, isFunc := expr.Args[0].(*model.FunctionCallExpression); isFunc && funcExpr.Name == pcl.Invoke {
			_, fullFunctionName := g.functionName(funcExpr.Args[0])
			g.Fprintf(w, "%s.Invoke(", fullFunctionName)
			functionParts := strings.Split(fullFunctionName, ".")
			functionName := functionParts[len(functionParts)-1]
			innerFunc, isFunc := funcExpr.Args[1].(*model.FunctionCallExpression)
			if isFunc && innerFunc.Name == pcl.IntrinsicConvert {
				switch arg := innerFunc.Args[0].(type) {
				case *model.ObjectConsExpression:
					g.withinFunctionInvoke(func() {
						useImplicitTypeName := g.generateOptions.implicitResourceArgsTypeName
						inputTypeName := functionName + "InvokeArgs"
						destTypeName := strings.ReplaceAll(fullFunctionName, functionName, inputTypeName)
						g.genObjectConsExpressionWithTypeName(w, arg, destTypeName, useImplicitTypeName,
							pcl.SortedFunctionParameters(funcExpr))
					})
				default:
					g.genIntrensic(w, funcExpr.Args[0], expr.Signature.ReturnType)
				}
			} else {
				if objectExpr, ok := funcExpr.Args[1].(*model.ObjectConsExpression); ok {
					g.withinFunctionInvoke(func() {
						useImplicitTypeName := g.generateOptions.implicitResourceArgsTypeName
						inputTypeName := functionName + "InvokeArgs"
						destTypeName := strings.ReplaceAll(fullFunctionName, functionName, inputTypeName)
						g.genObjectConsExpressionWithTypeName(w, objectExpr, destTypeName, useImplicitTypeName,
							pcl.SortedFunctionParameters(funcExpr))
					})
				} else {
					g.Fgenf(w, "%v", funcExpr.Args[1])
				}
			}

			g.Fprint(w, ")")
		} else {
			g.Fgenf(w, "Output.Create(%.v)", expr.Args[0])
		}

	case "element":
		g.Fgenf(w, "%.20v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		g.genEntries(w, expr)
	case "fileArchive":
		g.Fgenf(w, "new FileArchive(%.v)", expr.Args[0])
	case "remoteArchive":
		g.Fgenf(w, "new RemoteArchive(%.v)", expr.Args[0])
	case "assetArchive":
		g.Fgen(w, "new AssetArchive(")
		g.genDictionary(w, expr.Args[0].(*model.ObjectConsExpression), "AssetOrArchive")
		g.Fgen(w, ")")
	case "fileAsset":
		g.Fgenf(w, "new FileAsset(%.v)", expr.Args[0])
	case "stringAsset":
		g.Fgenf(w, "new StringAsset(%.v)", expr.Args[0])
	case "remoteAsset":
		g.Fgenf(w, "new RemoteAsset(%.v)", expr.Args[0])
	case "filebase64":
		// Assuming the existence of the following helper method located earlier in the preamble
		g.Fgenf(w, "ReadFileBase64(%v)", expr.Args[0])
	case "filebase64sha256":
		// Assuming the existence of the following helper method located earlier in the preamble
		g.Fgenf(w, "ComputeFileBase64Sha256(%v)", expr.Args[0])
	case "notImplemented":
		g.Fgenf(w, "NotImplemented(%v)", expr.Args[0])
	case "singleOrNone":
		g.Fgenf(w, "Enumerable.Single(%v)", expr.Args[0])
	case pcl.Invoke:
		_, fullFunctionName := g.functionName(expr.Args[0])
		functionParts := strings.Split(fullFunctionName, ".")
		functionName := functionParts[len(functionParts)-1]
		if g.insideAwait {
			g.Fprintf(w, "%s.InvokeAsync(", fullFunctionName)
		} else {
			g.Fprintf(w, "%s.Invoke(", fullFunctionName)
		}

		innerFunc, isFunc := expr.Args[1].(*model.FunctionCallExpression)
		if isFunc && innerFunc.Name == pcl.IntrinsicConvert {
			// function has been "lowered" i.e. rewritten with __convert
			switch arg := innerFunc.Args[0].(type) {
			case *model.ObjectConsExpression:
				g.withinFunctionInvoke(func() {
					useImplicitTypeName := g.generateOptions.implicitResourceArgsTypeName
					inputTypeName := functionName + "InvokeArgs"
					if g.insideAwait {
						inputTypeName = functionName + "Args"
					}

					destTypeName := strings.ReplaceAll(fullFunctionName, functionName, inputTypeName)
					g.genObjectConsExpressionWithTypeName(w, arg, destTypeName, useImplicitTypeName,
						pcl.SortedFunctionParameters(expr))
				})
			default:
				g.genIntrensic(w, expr.Args[0], expr.Signature.ReturnType)
			}
		} else {
			// function has not been rewritten
			switch arg := expr.Args[1].(type) {
			case *model.ObjectConsExpression:
				useImplicitTypeName := true
				destTypeName := "Irrelevant"
				g.genObjectConsExpressionWithTypeName(w, arg, destTypeName, useImplicitTypeName,
					pcl.SortedFunctionParameters(expr))
			default:
				g.genIntrensic(w, expr.Args[0], expr.Signature.ReturnType)
			}
		}

		if len(expr.Args) == 3 {
			if invokeOptions, ok := expr.Args[2].(*model.ObjectConsExpression); ok {
				g.Fgen(w, ", new() {\n")
				g.Indented(func() {
					for _, item := range invokeOptions.Items {
						key := pcl.LiteralValueString(item.Key)
						switch key {
						case "pluginDownloadUrl":
							// in .NET SDK the field is PluginDownloadURL so we have to special-case it
							g.Fgenf(w, "%sPluginDownloadURL = %v,\n", g.Indent, item.Value)
						default:
							g.Fgenf(w, "%s%s = %v,\n", g.Indent, Title(key), item.Value)
						}
					}
				})
				g.Fgenf(w, "%s}", g.Indent)
			}
		}
		g.Fprint(w, ")")
	case "join":
		g.Fgenf(w, "string.Join(%v, %v)", expr.Args[0], expr.Args[1])
	case "length":
		g.Fgenf(w, "%.20v.Length", expr.Args[0])
	case "lookup":
		g.Fgenf(w, "%v[%v]", expr.Args[0], expr.Args[1])
		if len(expr.Args) == 3 {
			g.Fgenf(w, " ?? %v", expr.Args[2])
		}
	case "range":
		g.genRange(w, expr, false)
	case "readFile":
		g.Fgenf(w, "File.ReadAllText(%v)", expr.Args[0])
	case "readDir":
		g.Fgenf(w, "Directory.GetFiles(%.v).Select(Path.GetFileName)", expr.Args[0])
	case "secret":
		g.Fgenf(w, "Output.CreateSecret(%v)", expr.Args[0])
	case "unsecret":
		g.Fgenf(w, "Output.Unsecret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.20v.Split(%v)", expr.Args[1], expr.Args[0])
	case "toBase64":
		g.Fgenf(w, "Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(%v))", expr.Args[0])
	case "fromBase64":
		g.Fgenf(w, "System.Text.Encoding.UTF8.GetString(Convert.FromBase64String(%v))", expr.Args[0])
	case "toJSON":
		if model.ContainsOutputs(expr.Args[0].Type()) {
			g.Fgen(w, "Output.JsonSerialize(Output.Create(")
			g.genDictionaryOrTuple(w, expr.Args[0])
			g.Fgen(w, "))")
		} else {
			g.Fgen(w, "JsonSerializer.Serialize(")
			g.genDictionaryOrTuple(w, expr.Args[0])
			g.Fgen(w, ")")
		}
	case "sha1":
		// Assuming the existence of the following helper method located earlier in the preamble
		g.Fgenf(w, "ComputeSHA1(%v)", expr.Args[0])
	case "stack":
		g.Fgen(w, "Deployment.Instance.StackName")
	case "project":
		g.Fgen(w, "Deployment.Instance.ProjectName")
	case "organization":
		g.Fgen(w, "Deployment.Instance.OrganizationName")
	case "cwd":
		g.Fgenf(w, "Directory.GetCurrentDirectory()")
	case "rootDirectory":
		g.genRootDirectory(w)
	default:
		g.genNYI(w, "call %v", expr.Name)
	}
}

func (g *generator) genDictionaryOrTuple(w io.Writer, expr model.Expression) {
	switch expr := expr.(type) {
	case *model.ObjectConsExpression:
		g.genDictionary(w, expr, "object?")
	case *model.TupleConsExpression:
		if g.isListOfDifferentTypes(expr) {
			g.Fgen(w, "new object?[]\n")
		} else {
			g.Fgen(w, "new[]\n")
		}

		g.Fgenf(w, "%[1]s{\n", g.Indent)
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "%s", g.Indent)
				g.genDictionaryOrTuple(w, v)
				g.Fgen(w, ",\n")
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	default:
		g.Fgenf(w, "%.v", expr)
	}
}

func (g *generator) genRootDirectory(w io.Writer) {
	g.Fgenf(w, "Pulumi.Deployment.Instance.RootDirectory")
}

func (g *generator) genDictionary(w io.Writer, expr *model.ObjectConsExpression, valueType string) {
	g.Fgenf(w, "new Dictionary<string, %s>\n", valueType)
	g.Fgenf(w, "%s{\n", g.Indent)
	g.Indented(func() {
		for _, item := range expr.Items {
			g.Fgenf(w, "%s[%.v] = ", g.Indent, item.Key)
			g.genDictionaryOrTuple(w, item.Value)
			g.Fgen(w, ",\n")
		}
	})
	g.Fgenf(w, "%s}", g.Indent)
}

func (g *generator) isListOfDifferentTypes(expr *model.TupleConsExpression) bool {
	var prevType model.Type
	for _, v := range expr.Expressions {
		if prevType == nil {
			prevType = v.Type()
			continue
		}

		_, isObjectType := prevType.(*model.ObjectType)
		_, isMap := prevType.(*model.MapType)

		if isObjectType || isMap {
			// don't actually compare object types or maps because these are always
			// mapped to Dictionary<string, object?> in C# so they will be the same type
			// even if their contents are different
			continue
		}

		conversionFrom := prevType.ConversionFrom(v.Type())
		conversionTo := v.Type().ConversionFrom(prevType)

		if conversionTo != model.SafeConversion || conversionFrom != model.SafeConversion {
			return true
		}
	}

	return false
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.20v[%.v]", expr.Collection, expr.Key)
}

func (g *generator) escapeString(v string, verbatim, expressions bool) string {
	builder := strings.Builder{}
	for _, c := range v {
		if c == '\x00' {
			// escape NUL bytes
			builder.WriteString("\u0000")
			continue
		}

		if verbatim {
			if c == '"' {
				builder.WriteRune('"')
			}
		} else {
			if c == '"' || c == '\\' {
				builder.WriteRune('\\')
			}
		}
		if expressions && (c == '{' || c == '}') {
			builder.WriteRune(c)
		}
		builder.WriteRune(c)
	}
	return builder.String()
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	newlines := strings.Contains(v, "\n")
	if !newlines {
		// This string does not contain newlines so we'll generate a regular string literal. Quotes and backslashes
		// will be escaped in conformance with
		// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
		g.Fgen(w, "\"")
		g.Fgen(w, g.escapeString(v, false, false))
		g.Fgen(w, "\"")
	} else {
		// This string does contain newlines, so we'll generate a verbatim string literal. Quotes will be escaped
		// in conformance with
		// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
		g.Fgen(w, "@\"")
		g.Fgen(w, g.escapeString(v, true, false))
		g.Fgen(w, "\"")
	}
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

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	switch argType := expr.Type().(type) {
	case *model.ObjectType:
		if configMetadata, ok := model.GetObjectTypeAnnotation[*ObjectTypeFromConfigMetadata](argType); ok {
			fullTypeName := fmt.Sprintf("Components.%sArgs.%s",
				configMetadata.ComponentName,
				configMetadata.TypeName)
			g.genObjectConsExpressionWithTypeName(w, expr, fullTypeName, false, nil)
			return
		}
	}
	g.genObjectConsExpression(w, expr, expr.Type())
}

func (g *generator) genObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression, destType model.Type) {
	if len(expr.Items) == 0 {
		g.Fgenf(w, "null")
		return
	}

	if schemaType, ok := g.toSchemaType(destType); ok {
		if codegen.ResolvedType(schemaType) == schema.AnyType {
			g.genDictionaryOrTuple(w, expr)
			return
		}
	}

	destTypeName := g.argumentTypeName(expr, destType)
	g.genObjectConsExpressionWithTypeName(w, expr, destTypeName, false, nil)
}

func propertyNameOverrides(exprType model.Type) map[string]string {
	overrides := make(map[string]string)
	schemaType, ok := pcl.GetSchemaForType(exprType)
	if !ok {
		return overrides
	}

	switch arg := schemaType.(type) {
	case *schema.ObjectType:
		for _, property := range arg.Properties {
			foundOverride := false
			if csharp, ok := property.Language["csharp"]; ok {
				if options, ok := csharp.(CSharpPropertyInfo); ok {
					overrides[property.Name] = options.Name
					foundOverride = true
				}
			}

			if !foundOverride {
				overrides[property.Name] = property.Name
			}
		}
	}

	return overrides
}

func resolvePropertyName(property string, overrides map[string]string) string {
	foundOverride, ok := overrides[property]
	if ok {
		return propertyName(foundOverride)
	}

	return propertyName(property)
}

func unwrapIntrinsicConvert(expr model.Expression) model.Expression {
	if call, ok := expr.(*model.FunctionCallExpression); ok && call.Name == pcl.IntrinsicConvert {
		return call.Args[0]
	}

	return expr
}

func isEmptyList(expr model.Expression) bool {
	expr = unwrapIntrinsicConvert(expr)
	if list, ok := expr.(*model.TupleConsExpression); ok {
		return len(list.Expressions) == 0
	}

	return false
}

func objectKey(item model.ObjectConsItem) string {
	switch key := item.Key.(type) {
	case *model.LiteralValueExpression:
		return key.Value.AsString()
	case *model.TemplateExpression:
		// assume a template expression has one constant part that is a LiteralValueExpression
		if len(key.Parts) == 1 {
			if literal, ok := key.Parts[0].(*model.LiteralValueExpression); ok {
				return literal.Value.AsString()
			}
		}
	}

	return ""
}

func (g *generator) genObjectConsExpressionWithTypeName(
	w io.Writer,
	expr *model.ObjectConsExpression,
	destTypeName string,
	implicitTypeName bool,
	multiArguments []*schema.Property,
) {
	if len(expr.Items) == 0 {
		return
	}

	if len(multiArguments) > 0 {
		pcl.GenerateMultiArguments(g.Formatter, w, "null", expr, multiArguments)
		return
	}

	typeName := destTypeName
	if typeName != "" {
		if implicitTypeName {
			g.Fgenf(w, "new()")
		} else {
			g.Fgenf(w, "new %s", typeName)
		}

		propertyNames := propertyNameOverrides(expr.Type())
		g.Fgenf(w, "\n%s{\n", g.Indent)
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "%s", g.Indent)
				propertyKey := objectKey(item)
				g.Fprint(w, resolvePropertyName(propertyKey, propertyNames))
				if g.usingDefaultListInitializer() && isEmptyList(item.Value) {
					g.Fgen(w, " = new() { },\n")
				} else {
					g.Fgenf(w, " = %.v,\n", item.Value)
				}
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	} else {
		g.Fgenf(w, "\n%s{\n", g.Indent)
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "%s{ %.v, %.v },\n", g.Indent, item.Key, item.Value)
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer,
	traversal hcl.Traversal, parts []model.Traversable, objType *schema.ObjectType,
) {
	for i, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
			if objType != nil {
				if p, ok := objType.Property(part.Name); ok {
					if info, ok := p.Language["csharp"].(CSharpPropertyInfo); ok && info.Name != "" {
						key = cty.StringVal(info.Name)
					}
				}
			}
		case hcl.TraverseIndex:
			key = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
		}

		switch key.Type() {
		case cty.String:
			if model.IsOptionalType(model.GetTraversableType(parts[i])) {
				g.Fgen(w, "?")
			}
			g.Fgenf(w, ".%s", propertyName(key.AsString()))
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			contract.Failf("unexpected traversal key of type %T (%v)", key, key.AsString())
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.20v", expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts, nil)
}

func (g *generator) schemaTypeName(schemaType *schema.ObjectType) string {
	fullyQualifiedTypeName := schemaType.Token
	nameParts := strings.Split(fullyQualifiedTypeName, ":")
	return Title(nameParts[len(nameParts)-1])
}

func (g *generator) withinFunctionInvoke(run func()) {
	if g.insideFunctionInvoke {
		// already inside this block?
		// just run the function
		run()
	} else {
		// not inside function invoke?
		// set it to true first, run, then set it back to false
		g.insideFunctionInvoke = true
		run()
		g.insideFunctionInvoke = false
	}
}

func (g *generator) isDeferredOutputVariable(expr *model.ScopeTraversalExpression) bool {
	if len(expr.Parts) != 1 {
		return false
	}

	for _, output := range g.deferredOutputVariables {
		if output.Name == expr.RootName {
			_, isOutput := expr.Type().(*model.OutputType)
			return isOutput
		}
	}

	return false
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	if g.isDeferredOutputVariable(expr) {
		g.Fgenf(w, "%s.Output", expr.RootName)
		return
	}

	rootName := makeValidIdentifier(expr.RootName)
	if g.isComponent {
		configVars := map[string]*pcl.ConfigVariable{}
		for _, configVar := range g.program.ConfigVariables() {
			configVars[configVar.Name()] = configVar
		}

		if _, isConfig := configVars[expr.RootName]; isConfig {
			if _, configReference := expr.Parts[0].(*pcl.ConfigVariable); configReference {
				rootName = "args." + Title(expr.RootName)
			}
		}
	}

	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "__item"
	}

	g.Fgen(w, rootName)

	invokedFunctionSchema, isFunctionInvoke := g.functionInvokes[rootName]

	if isFunctionInvoke && !g.asyncInit && len(expr.Parts) > 1 {
		lambdaArg := "invoke"
		if invokedFunctionSchema.ReturnType != nil {
			if objectType, ok := invokedFunctionSchema.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				lambdaArg = LowerCamelCase(g.schemaTypeName(objectType))
			}
		}

		// Assume invokes are returning Output<T> instead of Task<T>
		g.Fgenf(w, ".Apply(%s => %s", lambdaArg, lambdaArg)
	}

	var objType *schema.ObjectType
	if resource, ok := expr.Parts[0].(*pcl.Resource); ok {
		if schemaType, ok := pcl.GetSchemaForType(resource.InputType); ok {
			objType, _ = schemaType.(*schema.ObjectType)
		}
	}
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts, objType)

	if isFunctionInvoke && !g.asyncInit && len(expr.Parts) > 1 {
		g.Fgenf(w, ")")
	}
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.Fgenf(w, "%.20v.Select(__item => %.v).ToList()", expr.Source, expr.Each)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	multiLine := false
	expressions := false
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			if strings.Contains(lit.Value.AsString(), "\n") {
				multiLine = true
			}
		} else {
			expressions = true
		}
	}

	if multiLine {
		g.Fgen(w, "@")
	}
	if expressions {
		g.Fgen(w, "$")
	}
	g.Fgen(w, "\"")
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.Fgen(w, g.escapeString(lit.Value.AsString(), multiLine, expressions))
		} else {
			g.Fgenf(w, "{%.v}", expr)
		}
	}
	g.Fgen(w, "\"")
}

func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) {
	g.genNYI(w, "TemplateJoinExpression")
}

// Removes duplicate strings. Useful when collecting a distinct set of imports
func removeDuplicates(inputs []string) []string {
	distinctInputs := make([]string, 0)
	seenTexts := make(map[string]bool)
	for _, input := range inputs {
		if _, seen := seenTexts[input]; !seen {
			seenTexts[input] = true
			distinctInputs = append(distinctInputs, input)
		}
	}

	return distinctInputs
}

func (g *generator) isListOfDifferentObjectTypes(expr *model.TupleConsExpression) bool {
	switch expr.Type().(type) {
	case *model.TupleType:
		tupleType := expr.Type().(*model.TupleType)
		typeNames := make([]string, 0)
		for _, elemType := range tupleType.ElementTypes {
			if schemaType, ok := pcl.GetSchemaForType(elemType); ok {
				if objectType, ok := schemaType.(*schema.ObjectType); ok {
					typeName := g.schemaTypeName(objectType)
					typeNames = append(typeNames, typeName)
				}
			}
		}

		return len(removeDuplicates(typeNames)) > 1
	}

	return false
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	switch len(expr.Expressions) {
	case 0:
		g.Fgenf(w, "%s {}", g.listInitializer)
	default:
		if !g.isListOfDifferentObjectTypes(expr) {
			// only generate a list initializer when we don't have a list of union types
			// because list of a union is mapped to InputList<object>
			// which means new[] will not work because type-inference won't
			// know the type of the array beforehand
			g.Fgenf(w, "%s", g.listInitializer)
		}

		g.Fgenf(w, "\n%s{", g.Indent)

		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%.v,", g.Indent, v)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	opstr, precedence := "", g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}
