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
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program             *hcl2.Program
	contexts            map[string]map[string]*pkgContext
	diagnostics         hcl.Diagnostics
	jsonTempSpiller     *jsonSpiller
	ternaryTempSpiller  *tempSpiller
	readDirTempSpiller  *readDirSpiller
	splatSpiller        *splatSpiller
	optionalSpiller     *optionalSpiller
	scopeTraversalRoots codegen.StringSet
	arrayHelpers        map[string]*promptToInputArrayHelper
	isErrAssigned       bool
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	contexts := make(map[string]map[string]*pkgContext)

	for _, pkg := range program.Packages() {
		contexts[pkg.Name] = getPackages("tool", pkg)
	}

	g := &generator{
		program:             program,
		contexts:            contexts,
		jsonTempSpiller:     &jsonSpiller{},
		ternaryTempSpiller:  &tempSpiller{},
		readDirTempSpiller:  &readDirSpiller{},
		splatSpiller:        &splatSpiller{},
		optionalSpiller:     &optionalSpiller{},
		scopeTraversalRoots: codegen.NewStringSet(),
		arrayHelpers:        make(map[string]*promptToInputArrayHelper),
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

func getPackages(tool string, pkg *schema.Package) map[string]*pkgContext {
	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil
	}

	goInfo, _ := pkg.Language["go"].(GoPackageInfo)
	return generatePackageContextMap(tool, pkg, goInfo)
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
			pkg, mod, name, _ := r.DecomposeToken()
			if pkg == "pulumi" && mod == "providers" {
				pkg = name
			}

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

			var vPath string
			if version > 1 {
				vPath = fmt.Sprintf("/v%d", version)
			}
			if mod == "" {
				pulumiImports.Add(fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", pkg, vPath, pkg))
			} else {
				pulumiImports.Add(fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s/%s", pkg, vPath, pkg, mod))
			}
		}

		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				if call.Name == hcl2.Invoke {
					tokenArg := call.Args[0]
					token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
					tokenRange := tokenArg.SyntaxNode().Range()
					pkg, mod, _, diagnostics := hcl2.DecomposeToken(token, tokenRange)

					contract.Assert(len(diagnostics) == 0)

					version := -1
					for _, p := range program.Packages() {
						if p.Name == pkg {
							version = int(p.Version.Major)
							break
						}
					}

					if version == -1 {
						panic(errors.Errorf("could not find package information for resource with type token:\n\n%s", token))
					}

					var vPath string
					if version > 1 {
						vPath = fmt.Sprintf("/v%d", version)
					}

					// namespaceless invokes "aws:index:..."
					if mod == "" {
						pulumiImports.Add(fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", pkg, vPath, pkg))
					} else {
						pulumiImports.Add(fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s/%s", pkg, vPath, pkg, mod))
					}
				}
			}
			return n, nil
		})
		contract.Assert(len(diags) == 0)

		diags = n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
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

	g.genHelpers(w)
}

func (g *generator) genHelpers(w io.Writer) {
	for _, v := range g.arrayHelpers {
		v.generateHelperMethod(w)
	}
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

var resourceType = model.MustNewOpaqueType("pulumi.Resource")

func (g *generator) lowerResourceOptions(opts *hcl2.ResourceOptions) (*model.Block, []interface{}) {
	if opts == nil {
		return nil, nil
	}

	var block *model.Block
	var temps []interface{}
	appendOption := func(name string, value model.Expression, destType model.Type) {
		if block == nil {
			block = &model.Block{
				Type: "options",
				Body: &model.Body{},
			}
		}

		value, valueTemps := g.lowerExpression(value, destType, false)
		temps = append(temps, valueTemps...)

		block.Body.Items = append(block.Body.Items, &model.Attribute{
			Tokens: syntax.NewAttributeTokens(name),
			Name:   name,
			Value:  value,
		})
	}

	if opts.Parent != nil {
		appendOption("Parent", opts.Parent, model.DynamicType)
	}
	if opts.Provider != nil {
		appendOption("Provider", opts.Provider, model.DynamicType)
	}
	if opts.DependsOn != nil {
		appendOption("DependsOn", opts.DependsOn, model.NewListType(resourceType))
	}
	if opts.Protect != nil {
		appendOption("Protect", opts.Protect, model.BoolType)
	}
	if opts.IgnoreChanges != nil {
		appendOption("IgnoreChanges", opts.IgnoreChanges, model.NewListType(model.StringType))
	}

	return block, temps
}

func (g *generator) genResourceOptions(w io.Writer, block *model.Block) {
	if block == nil {
		return
	}

	for _, item := range block.Body.Items {
		attr := item.(*model.Attribute)
		g.Fgenf(w, ", pulumi.%s(%v)", attr.Name, attr.Value)
	}
}

func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {

	resName := makeValidIdentifier(r.Name())
	pkg, mod, typ, _ := r.DecomposeToken()
	mod = strings.ToLower(strings.Replace(mod, "/", ".", -1))
	if mod == "" {
		mod = pkg
	}

	// Compute resource options
	options, temps := g.lowerResourceOptions(r.Options)
	g.genTemps(w, temps)

	// Add conversions to input properties
	for _, input := range r.Inputs {
		destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
		isInput := true
		expr, temps := g.lowerExpression(input.Value, destType.(model.Type), isInput)
		input.Value = expr
		g.genTemps(w, temps)
	}

	instantiate := func(varName, resourceName string, w io.Writer) {
		if g.scopeTraversalRoots.Has(varName) || strings.HasPrefix(varName, "__") {
			g.Fgenf(w, "%s, err := %s.New%s(ctx, %s, ", varName, mod, typ, resourceName)
		} else {
			assignment := ":="
			if g.isErrAssigned {
				assignment = "="
			}
			g.Fgenf(w, "_, err %s %s.New%s(ctx, %s, ", assignment, mod, typ, resourceName)
		}
		g.isErrAssigned = true

		if len(r.Inputs) > 0 {
			g.Fgenf(w, "&%s.%sArgs{\n", mod, typ)
			for _, attr := range r.Inputs {
				g.Fgenf(w, "%s: ", strings.Title(attr.Name))
				g.Fgenf(w, "%.v,\n", attr.Value)

			}
			g.Fprint(w, "}")
		} else {
			g.Fprint(w, "nil")
		}
		g.genResourceOptions(w, options)
		g.Fprint(w, ")\n")
		g.Fgenf(w, "if err != nil {\n")
		g.Fgenf(w, "return err\n")
		g.Fgenf(w, "}\n")
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr, temps := g.lowerExpression(r.Options.Range, rangeType, false)
		g.genTemps(w, temps)

		g.Fgenf(w, "var %s []*%s.%s\n", resName, mod, typ)

		// ahead of range statement declaration generate the resource instantiation
		// to detect and removed unused k,v variables
		var buf bytes.Buffer
		instantiate("__res", fmt.Sprintf(`fmt.Sprintf("%s-%%v", key0)`, resName), &buf)
		instantiation := buf.String()
		isValUsed := strings.Contains(instantiation, "val0")
		valVar := "_"
		if isValUsed {
			valVar = "val0"
		}

		g.Fgenf(w, "for key0, %s := range %.v {\n", valVar, rangeExpr)
		g.Fgen(w, instantiation)
		g.Fgenf(w, "%s = append(%s, __res)\n", resName, resName)
		g.Fgenf(w, "}\n")

	} else {
		instantiate(resName, fmt.Sprintf("%q", resName), w)
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
			g.Fgenf(w, "var %s %s\n", t.Name, g.argumentTypeName(t.Value.TrueResult, t.Type(), isInput))
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
			argTyp := g.argumentTypeName(t.Value.Each, t.Value.Each.Type(), false)
			if strings.Contains(argTyp, ".") {
				g.Fgenf(w, "var %s %sArray\n", t.Name, argTyp)
			} else {
				g.Fgenf(w, "var %s []%s\n", t.Name, argTyp)
			}
			g.Fgenf(w, "for _, val0 := range %.v {\n", t.Value.Source)
			g.Fgenf(w, "%s = append(%s, %.v)\n", t.Name, t.Name, t.Value.Each)
			g.Fgenf(w, "}\n")
		case *optionalTemp:
			g.Fgenf(w, "%s := %.v\n", t.Name, t.Value)
		default:
			contract.Failf("unexpected temp type: %v", t)
		}
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *hcl2.LocalVariable) {
	isInput := false
	expr, temps := g.lowerExpression(v.Definition.Value, v.Type(), isInput)
	g.genTemps(w, temps)
	name := makeValidIdentifier(v.Name())
	assignment := ":="
	if !g.scopeTraversalRoots.Has(v.Name()) {
		name = "_"
		if g.isErrAssigned {
			assignment = "="
		}
	}
	switch expr := expr.(type) {
	case *model.FunctionCallExpression:
		switch expr.Name {
		case hcl2.Invoke:
			g.Fgenf(w, "%s, err %s %.3v;\n", name, assignment, expr)
			g.isErrAssigned = true
			g.Fgenf(w, "if err != nil {\n")
			g.Fgenf(w, "return err\n")
			g.Fgenf(w, "}\n")
		}
	default:
		g.Fgenf(w, "%s := %.3v;\n", name, expr)

	}

}

// nolint: lll
// uselookupInvokeForm takes a token for an invoke and determines whether to use the
// .Get or .Lookup form. The Go SDK has collisions in .Get methods that require renaming.
// For instance, gen.go creates a resource getter for AWS VPCs that collides with a function:
// GetVPC resource getter: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// LookupVPC function: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// Given that the naming here is not consisten, we must reverse the process from gen.go.
func (g *generator) useLookupInvokeForm(token string) bool {
	pkg, module, member, _ := hcl2.DecomposeToken(token, *new(hcl.Range))
	modSplit := strings.Split(module, "/")
	mod := modSplit[0]
	if mod == "index" {
		mod = ""
	}
	fn := Title(member)
	if len(modSplit) >= 2 {
		fn = Title(modSplit[1])
	}
	fnLookup := "Lookup" + fn[3:]
	pkgContext := g.contexts[pkg][mod]
	if pkgContext.names.has(fnLookup) {
		return true
	}

	return false
}
