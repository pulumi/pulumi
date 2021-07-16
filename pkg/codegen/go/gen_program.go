package gen

import (
	"bytes"
	"fmt"
	gofmt "go/format"
	"io"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program             *hcl2.Program
	packages            map[string]*schema.Package
	contexts            map[string]map[string]*pkgContext
	diagnostics         hcl.Diagnostics
	spills              *spills
	jsonTempSpiller     *jsonSpiller
	ternaryTempSpiller  *tempSpiller
	readDirTempSpiller  *readDirSpiller
	splatSpiller        *splatSpiller
	optionalSpiller     *optionalSpiller
	scopeTraversalRoots codegen.StringSet
	arrayHelpers        map[string]*promptToInputArrayHelper
	isErrAssigned       bool
	configCreated       bool
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	packages, contexts := map[string]*schema.Package{}, map[string]map[string]*pkgContext{}
	for _, pkg := range program.Packages() {
		packages[pkg.Name], contexts[pkg.Name] = pkg, getPackages("tool", pkg)
	}

	g := &generator{
		program:             program,
		packages:            packages,
		contexts:            contexts,
		spills:              &spills{counts: map[string]int{}},
		jsonTempSpiller:     &jsonSpiller{},
		ternaryTempSpiller:  &tempSpiller{},
		readDirTempSpiller:  &readDirSpiller{},
		splatSpiller:        &splatSpiller{},
		optionalSpiller:     &optionalSpiller{},
		scopeTraversalRoots: codegen.NewStringSet(),
		arrayHelpers:        make(map[string]*promptToInputArrayHelper),
	}

	g.Formatter = format.NewFormatter(g)

	// we must collect imports once before lowering, and once after.
	// this allows us to avoid complexity of traversing apply expressions for things like JSON
	// but still have access to types provided by __convert intrinsics after lowering.
	pulumiImports := codegen.NewStringSet()
	stdImports := codegen.NewStringSet()
	g.collectImports(program, stdImports, pulumiImports)

	var progPostamble bytes.Buffer
	for _, n := range nodes {
		g.collectScopeRoots(n)
	}

	for _, n := range nodes {
		g.genNode(&progPostamble, n)
	}

	g.genPostamble(&progPostamble, nodes)

	// We must generate the program first and the preamble second and finally cat the two together.
	// This is because nested object/tuple cons expressions can require imports that aren't
	// present in resource declarations or invokes alone. Expressions are lowered when the program is generated
	// and this must happen first so we can access types via __convert intrinsics.
	var index bytes.Buffer
	g.genPreamble(&index, program, stdImports, pulumiImports)
	index.Write(progPostamble.Bytes())

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

var packageContexts sync.Map

func getPackages(tool string, pkg *schema.Package) map[string]*pkgContext {
	if v, ok := packageContexts.Load(pkg); ok {
		return v.(map[string]*pkgContext)
	}

	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}
	v := generatePackageContextMap(tool, pkg, goPkgInfo)
	packageContexts.Store(pkg, v)
	return v
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
func (g *generator) genPreamble(w io.Writer, program *hcl2.Program, stdImports, pulumiImports codegen.StringSet) {
	g.Fprint(w, "package main\n\n")
	g.Fprintf(w, "import (\n")

	g.collectImports(program, stdImports, pulumiImports)
	for _, imp := range stdImports.SortedValues() {
		g.Fprintf(w, "\"%s\"\n", imp)
	}

	g.Fprintf(w, "\n")
	g.Fprintf(w, "\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"\n")

	for _, imp := range pulumiImports.SortedValues() {
		g.Fprintf(w, "%s\n", imp)
	}

	g.Fprintf(w, ")\n")
	g.Fprintf(w, "func main() {\n")
	g.Fprintf(w, "pulumi.Run(func(ctx *pulumi.Context) error {\n")
}

func (g *generator) collectTypeImports(program *hcl2.Program, t schema.Type, imports codegen.StringSet) {
	var token string
	switch t := t.(type) {
	case *schema.InputType:
		g.collectTypeImports(program, t.ElementType, imports)
		return
	case *schema.OptionalType:
		g.collectTypeImports(program, t.ElementType, imports)
		return
	case *schema.ArrayType:
		g.collectTypeImports(program, t.ElementType, imports)
		return
	case *schema.MapType:
		g.collectTypeImports(program, t.ElementType, imports)
		return
	case *schema.UnionType:
		for _, t := range t.ElementTypes {
			g.collectTypeImports(program, t, imports)
		}
		return
	case *schema.ObjectType:
		token = t.Token
	case *schema.EnumType:
		token = t.Token
	case *schema.TokenType:
		token = t.Token
	case *schema.ResourceType:
		token = t.Token
	}
	if token == "" {
		return
	}

	var tokenRange hcl.Range
	pkg, mod, _, _ := hcl2.DecomposeToken(token, tokenRange)
	vPath, err := g.getVersionPath(program, pkg)
	if err != nil {
		panic(err)
	}
	imports.Add(g.getPulumiImport(pkg, vPath, mod))
}

// collect Imports returns two sets of packages imported by the program, std lib packages and pulumi packages
func (g *generator) collectImports(
	program *hcl2.Program,
	stdImports,
	pulumiImports codegen.StringSet) (codegen.StringSet, codegen.StringSet) {
	// Accumulate import statements for the various providers
	for _, n := range program.Nodes {
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, mod, name, _ := r.DecomposeToken()
			if pkg == "pulumi" && mod == "providers" {
				pkg = name
			}

			vPath, err := g.getVersionPath(program, pkg)
			if err != nil {
				panic(err)
			}

			pulumiImports.Add(g.getPulumiImport(pkg, vPath, mod))
		}
		if _, isConfigVar := n.(*hcl2.ConfigVariable); isConfigVar {
			pulumiImports.Add("\"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config\"")
		}

		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				if call.Name == hcl2.Invoke {
					tokenArg := call.Args[0]
					token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
					tokenRange := tokenArg.SyntaxNode().Range()
					pkg, mod, _, diagnostics := hcl2.DecomposeToken(token, tokenRange)

					contract.Assert(len(diagnostics) == 0)

					vPath, err := g.getVersionPath(program, pkg)
					if err != nil {
						panic(err)
					}
					pulumiImports.Add(g.getPulumiImport(pkg, vPath, mod))
				} else if call.Name == hcl2.IntrinsicConvert {
					if schemaType, ok := hcl2.GetSchemaForType(call.Type()); ok {
						g.collectTypeImports(program, schemaType, pulumiImports)
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

func (g *generator) getVersionPath(program *hcl2.Program, pkg string) (string, error) {
	for _, p := range program.Packages() {
		if p.Name == pkg {
			if p.Version != nil && p.Version.Major > 1 {
				return fmt.Sprintf("/v%d", p.Version.Major), nil
			}
			return "", nil
		}
	}

	return "", errors.Errorf("could not find package version information for pkg: %s", pkg)

}

func (g *generator) getPkgContext(pkg, mod string) (*pkgContext, bool) {
	p, ok := g.contexts[pkg]
	if !ok {
		return nil, false
	}
	m, ok := p[mod]
	return m, ok
}

func (g *generator) getGoPackageInfo(pkg string) (GoPackageInfo, bool) {
	p, ok := g.packages[pkg]
	if !ok {
		return GoPackageInfo{}, false
	}
	info, ok := p.Language["go"].(GoPackageInfo)
	return info, ok
}

func (g *generator) getPulumiImport(pkg, vPath, mod string) string {
	info, _ := g.getGoPackageInfo(pkg)
	if m, ok := info.ModuleToPackage[mod]; ok {
		mod = m
	}

	imp := fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s/%s", pkg, vPath, pkg, mod)
	// namespaceless invokes "aws:index:..."
	if mod == "" {
		imp = fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", pkg, vPath, pkg)
	}

	// All providers don't follow the sdk/go/<package> scheme. Allow ImportBasePath as
	// a means to override this assumption.
	if info.ImportBasePath != "" && mod != "" {
		imp = fmt.Sprintf("%s/%s", info.ImportBasePath, mod)
	}

	if alias, ok := info.PackageImportAliases[imp]; ok {
		return fmt.Sprintf("%s %q", alias, imp)
	}

	modSplit := strings.Split(mod, "/")
	// account for mods like "eks/ClusterVpcConfig" index...
	if len(modSplit) > 1 {
		if modSplit[0] == "" || modSplit[0] == "index" {
			imp = fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", pkg, vPath, pkg)
		} else {
			imp = fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s/%s", pkg, vPath, pkg, strings.Split(mod, "/")[0])
		}
	}
	return fmt.Sprintf("%q", imp)
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
	case *hcl2.ConfigVariable:
		g.genConfigVariable(w, n)
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

		value, valueTemps := g.lowerExpression(value, destType)
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
	if mod == "" || strings.HasPrefix(mod, "/") || strings.HasPrefix(mod, "index/") {
		mod = pkg
	}

	// Compute resource options
	options, temps := g.lowerResourceOptions(r.Options)
	g.genTemps(w, temps)

	// Add conversions to input properties
	for _, input := range r.Inputs {
		destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
		expr, temps := g.lowerExpression(input.Value, destType.(model.Type))
		input.Value = expr
		g.genTemps(w, temps)
	}

	modOrAlias := g.getModOrAlias(pkg, mod)

	instantiate := func(varName, resourceName string, w io.Writer) {
		if g.scopeTraversalRoots.Has(varName) || strings.HasPrefix(varName, "__") {
			g.Fgenf(w, "%s, err := %s.New%s(ctx, %s, ", varName, modOrAlias, typ, resourceName)
		} else {
			assignment := ":="
			if g.isErrAssigned {
				assignment = "="
			}
			g.Fgenf(w, "_, err %s %s.New%s(ctx, %s, ", assignment, modOrAlias, typ, resourceName)
		}
		g.isErrAssigned = true

		if len(r.Inputs) > 0 {
			g.Fgenf(w, "&%s.%sArgs{\n", modOrAlias, typ)
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
		rangeExpr, temps := g.lowerExpression(r.Options.Range, rangeType)
		g.genTemps(w, temps)

		g.Fgenf(w, "var %s []*%s.%s\n", resName, modOrAlias, typ)

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
	expr, temps := g.lowerExpression(v.Value, v.Type())
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
			case *spillTemp, *jsonTemp, *readDirTemp:
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
		case *spillTemp:
			bytesVar := fmt.Sprintf("tmp%s", strings.ToUpper(t.Variable.Name))
			g.Fgenf(w, "%s, err := json.Marshal(", bytesVar)
			args := t.Value.(*model.FunctionCallExpression).Args[0]
			g.Fgenf(w, "%.v)\n", args)
			g.Fgenf(w, "if err != nil {\n")
			if genZeroValueDecl {
				g.Fgenf(w, "return _zero, err\n")
			} else {
				g.Fgenf(w, "return err\n")
			}
			g.Fgenf(w, "}\n")
			g.Fgenf(w, "%s := string(%s)\n", t.Variable.Name, bytesVar)
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
	expr, temps := g.lowerExpression(v.Definition.Value, v.Type())
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

func (g *generator) genConfigVariable(w io.Writer, v *hcl2.ConfigVariable) {
	if !g.configCreated {
		g.Fprint(w, "cfg := config.New(ctx, \"\")\n")
		g.configCreated = true
	}

	getType := ""
	switch v.Type() {
	case model.StringType: // Already default
	case model.NumberType:
		getType = "Float"
	case model.IntType:
		getType = "Int"
	case model.BoolType:
		getType = "Boolean"
	case model.DynamicType:
		getType = "Object"
	}

	getOrRequire := "Get"
	if v.DefaultValue == nil {
		getOrRequire = "Require"
	}

	if v.DefaultValue == nil {
		g.Fgenf(w, "%[1]s := cfg.%[2]s%[3]s(\"%[1]s\")\n", v.Name(), getOrRequire, getType)
	} else {
		expr, temps := g.lowerExpression(v.DefaultValue, v.DefaultValue.Type())
		g.genTemps(w, temps)
		switch expr := expr.(type) {
		case *model.FunctionCallExpression:
			switch expr.Name {
			case hcl2.Invoke:
				g.Fgenf(w, "%s, err := %.3v;\n", v.Name(), expr)
				g.isErrAssigned = true
				g.Fgenf(w, "if err != nil {\n")
				g.Fgenf(w, "return err\n")
				g.Fgenf(w, "}\n")
			}
		default:
			g.Fgenf(w, "%s := %.3v;\n", v.Name(), expr)
		}
		switch v.Type() {
		case model.StringType:
			g.Fgenf(w, "if param := cfg.Get(\"%s\"); param != \"\"{\n", v.Name())
		case model.NumberType:
			g.Fgenf(w, "if param := cfg.GetFloat(\"%s\"); param != 0 {\n", v.Name())
		case model.IntType:
			g.Fgenf(w, "if param := cfg.GetInt(\"%s\"); param != 0 {\n", v.Name())
		case model.BoolType:
			g.Fgenf(w, "if param := cfg.GetBool(\"%s\"); param {\n", v.Name())
		default:
			g.Fgenf(w, "if param := cfg.GetBool(\"%s\"); param != nil {\n", v.Name())
		}
		g.Fgenf(w, "%s = param\n", v.Name())
		g.Fgen(w, "}\n")
	}
}

// nolint: lll
// useLookupInvokeForm takes a token for an invoke and determines whether to use the
// .Get or .Lookup form. The Go SDK has collisions in .Get methods that require renaming.
// For instance, gen.go creates a resource getter for AWS VPCs that collides with a function:
// GetVPC resource getter: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// LookupVPC function: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// Given that the naming here is not consisten, we must reverse the process from gen.go.
func (g *generator) useLookupInvokeForm(token string) bool {
	pkg, module, member, _ := hcl2.DecomposeToken(token, *new(hcl.Range))
	modSplit := strings.Split(module, "/")
	mod := modSplit[0]
	fn := Title(member)
	if mod == "index" && len(modSplit) >= 2 {
		// e.g. "aws:index/getPartition:getPartition" where module is "index/getPartition"
		mod = ""
		fn = Title(modSplit[1])
	} else {
		// e.g. for type "ec2/getVpc:getVpcArgs"
		if _, has := g.contexts[pkg][mod]; !has {
			mod = module
		}
	}
	fnLookup := "Lookup" + fn[3:]
	pkgContext, has := g.contexts[pkg][mod]
	if has && pkgContext.names.Has(fnLookup) {
		return true
	}
	return false
}

// getModOrAlias attempts to reconstruct the import statement and check if the imported package
// is aliased, returning that alias if available.
func (g *generator) getModOrAlias(pkg, mod string) string {
	info, ok := g.getGoPackageInfo(pkg)
	if !ok {
		return mod
	}
	if m, ok := info.ModuleToPackage[mod]; ok {
		mod = m
	}

	imp := fmt.Sprintf("%s/%s", info.ImportBasePath, mod)
	if alias, ok := info.PackageImportAliases[imp]; ok {
		return alias
	}
	return mod
}
