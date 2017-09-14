// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"fmt"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/tools"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type RPCGenerator struct {
	IDLRoot     string            // the root where IDL is loaded from.
	IDLPkgBase  string            // the IDL's base package path.
	RPCPkgBase  string            // the RPC's base package path.
	Out         string            // where RPC stub outputs will be saved.
	CurrPkg     *Package          // the package currently being visited.
	CurrFile    string            // the file currently being visited.
	FileHadRes  bool              // true if the file had at least one resource.
	FileImports map[string]string // a map of foreign packages used in a file.
}

func NewRPCGenerator(root, idlPkgBase, rpcPkgBase, out string) *RPCGenerator {
	return &RPCGenerator{
		IDLRoot:    root,
		IDLPkgBase: idlPkgBase,
		RPCPkgBase: rpcPkgBase,
		Out:        out,
	}
}

func (g *RPCGenerator) Generate(pkg *Package) error {
	// Ensure the directory structure exists in the target.
	if err := mirrorDirLayout(pkg, g.Out); err != nil {
		return err
	}

	// Install context about the current entity being visited.
	oldpkg, oldfile := g.CurrPkg, g.CurrFile
	g.CurrPkg = pkg
	defer (func() {
		g.CurrPkg = oldpkg
		g.CurrFile = oldfile
	})()

	// Now walk through the package, file by file, and generate the contents.
	for relpath, file := range pkg.Files {
		g.CurrFile = relpath
		var members []Member
		for _, nm := range file.MemberNames {
			members = append(members, file.Members[nm])
		}
		path := filepath.Join(g.Out, relpath)
		if err := g.EmitFile(path, pkg, members); err != nil {
			return err
		}
	}

	return nil
}

func (g *RPCGenerator) EmitFile(file string, pkg *Package, members []Member) error {
	oldHadRes, oldImports := g.FileHadRes, g.FileImports
	g.FileHadRes, g.FileImports = false, make(map[string]string)
	defer (func() {
		g.FileHadRes = oldHadRes
		g.FileImports = oldImports
	})()

	// First, generate the body.  This is required first so we know which imports to emit.
	body := g.genFileBody(file, pkg, members)

	// Open up a writer that overwrites whatever file contents already exist.
	w, err := tools.NewGenWriter(lumidl, file)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Emit a header into the file.
	w.EmitHeaderWarning()

	// Now emit the package name at the top-level.
	w.Writefmtln("package %v", pkg.Pkginfo.Pkg.Name())
	w.Writefmtln("")

	// And all of the imports that we're going to need.
	if g.FileHadRes || len(g.FileImports) > 0 {
		w.Writefmtln("import (")

		if g.FileHadRes {
			w.Writefmtln(`    pbempty "github.com/golang/protobuf/ptypes/empty"`)
			w.Writefmtln(`    pbstruct "github.com/golang/protobuf/ptypes/struct"`)
			w.Writefmtln(`    "golang.org/x/net/context"`)
			w.Writefmtln("")
			w.Writefmtln(`    "github.com/pulumi/pulumi-fabric/pkg/resource"`)
			w.Writefmtln(`    "github.com/pulumi/pulumi-fabric/pkg/resource/plugin"`)
			w.Writefmtln(`    "github.com/pulumi/pulumi-fabric/pkg/tokens"`)
			w.Writefmtln(`    "github.com/pulumi/pulumi-fabric/pkg/util/contract"`)
			w.Writefmtln(`    "github.com/pulumi/pulumi-fabric/pkg/util/mapper"`)
			w.Writefmtln(`    lumirpc "github.com/pulumi/pulumi-fabric/sdk/proto/go"`)
		}

		if len(g.FileImports) > 0 {
			if g.FileHadRes {
				w.Writefmtln("")
			}
			// Sort the imports so they are in a correct, deterministic order.
			var imports []string
			for imp := range g.FileImports {
				imports = append(imports, imp)
			}
			sort.Strings(imports)

			// Now just emit a list of imports with their given names.
			for _, imp := range imports {
				name := g.FileImports[imp]

				// If the import referenced one of the IDL packages, we must rewrite it to an RPC package.
				contract.Assertf(strings.HasPrefix(imp, g.IDLPkgBase),
					"Inter-IDL package references not yet supported (%v is not part of %v)", imp, g.IDLPkgBase)
				var imppath string
				if imp == g.IDLPkgBase {
					imppath = g.RPCPkgBase
				} else {
					relimp := imp[len(g.IDLPkgBase)+1:]
					imppath = g.RPCPkgBase + "/" + relimp
				}

				w.Writefmtln(`    %v "%v"`, name, imppath)
			}
		}

		w.Writefmtln(")")
		w.Writefmtln("")
	}

	// Now finally emit the actual body and close out the file.
	w.Writefmtln("%v", body)
	return w.Flush()
}

func (g *RPCGenerator) genFileBody(file string, pkg *Package, members []Member) string {
	w, err := tools.NewGenWriter(lumidl, "")
	contract.IgnoreError(err)

	// First, for each RPC struct/resource member, emit its appropriate generated code.
	var typedefs []Typedef
	var consts []*Const
	module := g.getFileModule(file)
	for _, m := range members {
		switch t := m.(type) {
		case *Alias:
			typedefs = append(typedefs, t)
		case *Const:
			consts = append(consts, t)
		case *Enum:
			typedefs = append(typedefs, t)
		case *Resource:
			g.EmitResource(w, module, pkg, t)
			g.EmitStructType(w, module, pkg, t)
		case *Struct:
			g.EmitStructType(w, module, pkg, t)
		default:
			contract.Failf("Unrecognized package member type: %v", reflect.TypeOf(t))
		}
	}

	// Next emit all supporting types.  First, aliases and enum types.
	if len(typedefs) > 0 {
		g.EmitTypedefs(w, typedefs)
	}

	// Finally, emit any consts at the very end.
	if len(consts) > 0 {
		g.EmitConstants(w, consts)
	}

	err = w.Flush()
	contract.IgnoreError(err)
	return w.Buffer()
}

// getFileModule generates a module name from a filename.  To do so, we simply find the path part after the root and
// remove any file extensions, to get the underlying package's module token.
func (g *RPCGenerator) getFileModule(file string) tokens.Module {
	module, _ := filepath.Rel(g.Out, file)
	if ext := filepath.Ext(module); ext != "" {
		extix := strings.LastIndex(module, ext)
		module = module[:extix]
	}
	return tokens.Module(module)
}

func (g *RPCGenerator) EmitResource(w *tools.GenWriter, module tokens.Module, pkg *Package, res *Resource) {
	name := res.Name()
	w.Writefmtln("/* RPC stubs for %v resource provider */", name)
	w.Writefmtln("")

	// Remember when we encounter resources so we can import the right packages.
	g.FileHadRes = true

	propopts := res.PropertyOptions()
	var hasinputs bool
	for _, propopt := range propopts {
		if !propopt.Out {
			hasinputs = true
			break
		}
	}

	// Emit a type token.
	token := fmt.Sprintf("%v:%v:%v", pkg.Name, module, name)
	w.Writefmtln("// %[1]vToken is the type token corresponding to the %[1]v package type.", name)
	w.Writefmtln(`const %vToken = tokens.Type("%v")`, name, token)
	w.Writefmtln("")

	// Now, generate an ops interface that the real provider will implement.
	w.Writefmtln("// %[1]vProviderOps is a pluggable interface for %[1]v-related management functionality.", name)
	w.Writefmtln("type %vProviderOps interface {", name)
	w.Writefmtln("    Configure(ctx context.Context, vars map[tokens.ModuleMember]string) error")
	w.Writefmtln("    Check(ctx context.Context, obj *%v, property string) error", name)
	w.Writefmtln("    Diff(ctx context.Context, id resource.ID,")
	w.Writefmtln("        old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) ([]string, error)", name)
	w.Writefmtln("    Create(ctx context.Context, obj *%v) (resource.ID, error)", name)
	w.Writefmtln("    Update(ctx context.Context, id resource.ID,")
	w.Writefmtln("        old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) error", name)
	w.Writefmtln("    Delete(ctx context.Context, id resource.ID, obj %v) error", name)
	w.Writefmtln("}")
	w.Writefmtln("")

	// Next generate all the RPC scaffolding goo
	w.Writefmtln("// %[1]vProvider is a dynamic gRPC-based plugin for managing %[1]v resources.", name)
	w.Writefmtln("type %vProvider struct {", name)
	w.Writefmtln("    ops %vProviderOps", name)
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("// New%vProvider allocates a resource provider that delegates to a ops instance.", name)
	w.Writefmtln("func New%[1]vProvider(ops %[1]vProviderOps) lumirpc.ResourceProviderServer {", name)
	w.Writefmtln("    contract.Assert(ops != nil)")
	w.Writefmtln("    return &%vProvider{ops: ops}", name)
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Configure(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.ConfigureRequest) (*pbempty.Empty, error) {")
	w.Writefmtln("    vars := make(map[tokens.ModuleMember]string)")
	w.Writefmtln("    for k, v := range req.GetVariables() {")
	w.Writefmtln("        vars[tokens.ModuleMember(k)] = v")
	w.Writefmtln("    }")
	w.Writefmtln("    if err := p.ops.Configure(ctx, vars); err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    return &pbempty.Empty{}, nil")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Check(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.CheckRequest) (*lumirpc.CheckResponse, error) {")
	w.Writefmtln("    contract.Assert(resource.URN(req.GetUrn()).Type() == %vToken)", name)
	w.Writefmtln("    obj, _, err := p.Unmarshal(req.GetProperties(), true)")
	w.Writefmtln("    if obj == nil || err != nil {")
	w.Writefmtln("        return plugin.NewCheckResponse(err), nil")
	w.Writefmtln("    }")
	w.Writefmtln("    var failures []error")
	// check global properties:
	w.Writefmtln("    if failure := p.ops.Check(ctx, obj, \"\"); failure != nil {")
	w.Writefmtln("        failures = append(failures, failure)")
	w.Writefmtln("    }")
	// check each input property:
	if hasinputs {
		for _, opts := range propopts {
			if !opts.Out {
				w.Writefmtln("    if failure := p.ops.Check(ctx, obj, \"%v\"); failure != nil {", opts.Name)
				w.Writefmtln("        failures = append(failures,")
				w.Writefmtln("            resource.NewPropertyError(\"%v\", \"%v\", failure))", name, opts.Name)
				w.Writefmtln("    }")
			}
		}
		w.Writefmtln("    if len(failures) > 0 {")
		w.Writefmtln("        return plugin.NewCheckResponse(resource.NewErrors(failures)), nil")
		w.Writefmtln("    }")
	}
	w.Writefmtln("    return plugin.NewCheckResponse(nil), nil")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Create(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.CreateRequest) (*lumirpc.CreateResponse, error) {")
	w.Writefmtln("    contract.Assert(resource.URN(req.GetUrn()).Type() == %vToken)", name)
	w.Writefmtln("    obj, _, err := p.Unmarshal(req.GetProperties(), false)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    id, err := p.ops.Create(ctx, obj)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    props, err := plugin.MarshalProperties(")
	w.Writefmtln("        resource.NewPropertyMap(obj), plugin.MarshalOptions{})")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    return &lumirpc.CreateResponse{Id: string(id), Properties: props}, nil")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Diff(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.DiffRequest) (*lumirpc.DiffResponse, error) {")
	w.Writefmtln("    contract.Assert(resource.URN(req.GetUrn()).Type() == %vToken)", name)
	w.Writefmtln("    id := resource.ID(req.GetId())")
	w.Writefmtln("    old, oldprops, err := p.Unmarshal(req.GetOlds(), false)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    new, newprops, err := p.Unmarshal(req.GetNews(), true)")
	w.Writefmtln("    if new == nil || err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    var replaces []string")
	w.Writefmtln("    diff := oldprops.Diff(newprops)")
	w.Writefmtln("    if diff != nil {")
	for _, opts := range propopts {
		if opts.Replaces {
			w.Writefmtln("        if diff.Changed(\"%v\") {", opts.Name)
			w.Writefmtln("            replaces = append(replaces, \"%v\")", opts.Name)
			w.Writefmtln("        }")
		}
	}
	w.Writefmtln("    }")
	w.Writefmtln("    more, err := p.ops.Diff(ctx, id, old, new, diff)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    return &lumirpc.DiffResponse{")
	w.Writefmtln("        Replaces: append(replaces, more...),")
	w.Writefmtln("    }, err")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Update(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.UpdateRequest) (*lumirpc.UpdateResponse, error) {")
	w.Writefmtln("    contract.Assert(resource.URN(req.GetUrn()).Type() == %vToken)", name)
	w.Writefmtln("    id := resource.ID(req.GetId())")
	w.Writefmtln("    old, oldprops, err := p.Unmarshal(req.GetOlds(), false)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    new, newprops, err := p.Unmarshal(req.GetNews(), false)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    diff := oldprops.Diff(newprops)")
	w.Writefmtln("    if err := p.ops.Update(ctx, id, old, new, diff); err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    props, err := plugin.MarshalProperties(")
	w.Writefmtln("        resource.NewPropertyMap(new), plugin.MarshalOptions{})")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    return &lumirpc.UpdateResponse{Properties: props}, nil")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Delete(", name)
	w.Writefmtln("    ctx context.Context, req *lumirpc.DeleteRequest) (*pbempty.Empty, error) {")
	w.Writefmtln("    contract.Assert(resource.URN(req.GetUrn()).Type() == %vToken)", name)
	w.Writefmtln("    id := resource.ID(req.GetId())")
	w.Writefmtln("    obj, _, err := p.Unmarshal(req.GetProperties(), false)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    if err := p.ops.Delete(ctx, id, *obj); err != nil {")
	w.Writefmtln("        return nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    return &pbempty.Empty{}, nil")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("func (p *%vProvider) Unmarshal(", name)
	w.Writefmtln("    v *pbstruct.Struct, allowUnknowns bool) (*%v, resource.PropertyMap, error) {", name)
	w.Writefmtln("    opts := plugin.MarshalOptions{AllowUnknowns: allowUnknowns}")
	w.Writefmtln("    props, err := plugin.UnmarshalProperties(v, opts)")
	w.Writefmtln("    if err != nil {")
	w.Writefmtln("        return nil, nil, err")
	w.Writefmtln("    }")
	w.Writefmtln("    if allowUnknowns && props.ContainsUnknowns() {")
	w.Writefmtln("        return nil, props, nil")
	w.Writefmtln("    }")
	w.Writefmtln("    var obj %v", name)
	w.Writefmtln("    return &obj, props, mapper.MapIU(props.Mappable(), &obj)")
	w.Writefmtln("}")
	w.Writefmtln("")
}

func (g *RPCGenerator) EmitStructType(w *tools.GenWriter, module tokens.Module, pkg *Package, t TypeMember) {
	name := t.Name()
	w.Writefmtln("/* Marshalable %v structure(s) */", name)
	w.Writefmtln("")

	props := t.Properties()
	propopts := t.PropertyOptions()
	w.Writefmtln("// %v is a marshalable representation of its corresponding IDL type.", name)
	w.Writefmtln("type %v struct {", name)
	for i, prop := range props {
		opts := propopts[i]
		// Make a JSON tag for this so we can serialize; note that outputs are always optional in this position.
		jsontag := makeLumiTag(opts)
		w.Writefmtln("    %v %v %v",
			prop.Name(), g.GenTypeName(prop.Type(), opts.Optional || opts.In || opts.Out), jsontag)
	}
	w.Writefmtln("}")
	w.Writefmtln("")

	if len(props) > 0 {
		w.Writefmtln("// %v's properties have constants to make dealing with diffs and property bags easier.", name)
		w.Writefmtln("const (")
		for i, prop := range props {
			opts := propopts[i]
			w.Writefmtln("    %v_%v = \"%v\"", name, prop.Name(), opts.Name)
		}
		w.Writefmtln(")")
		w.Writefmtln("")
	}
}

// makeLumiTag turns a set of property options into a serializable JSON tag.
func makeLumiTag(opts PropertyOptions) string {
	var flags string
	if opts.Optional || opts.In || opts.Out {
		flags = ",optional"
	}
	return fmt.Sprintf("`lumi:\"%v%v\"`", opts.Name, flags)
}

func (g *RPCGenerator) GenTypeName(t types.Type, opt bool) string {
	switch u := t.(type) {
	case *types.Basic:
		switch k := u.Kind(); k {
		case types.Bool:
			return "bool"
		case types.String:
			return "string"
		case types.Float64:
			return "float64"
		default:
			contract.Failf("Unrecognized GenTypeName basic type: %v", k)
		}
	case *types.Interface:
		return "interface{}"
	case *types.Named:
		obj := u.Obj()
		// For resource types, simply emit an ID, since that is what will have been serialized.
		if IsResource(obj, u) {
			return "resource.ID"
		}

		// For references to the special predefined types, use the runtime provider representation.
		if spec, kind := IsSpecial(obj); spec {
			switch kind {
			case SpecialArchiveType:
				return "resource.Archive"
			case SpecialAssetType:
				return "resource.Asset"
			default:
				contract.Failf("Unexpected special kind: %v", kind)
			}
		}

		// Otherwise, see how to reference the type, based on imports.
		pkg := obj.Pkg()
		name := obj.Name()

		// If this came from the same package, Go can access it without qualification.
		if pkg == g.CurrPkg.Pkginfo.Pkg {
			return name
		}

		// Otherwise, we will need to refer to a qualified import name.
		impname := g.registerImport(pkg)
		return fmt.Sprintf("%v.%v", impname, name)
	case *types.Map:
		return fmt.Sprintf("map[%v]%v", g.GenTypeName(u.Key(), false), g.GenTypeName(u.Elem(), false))
	case *types.Pointer:
		// If this isn't an optional property, and the underlying type is a resource or special type, unpointerize it.
		elem := u.Elem()
		unptr := false
		if !opt {
			if elnm, iselnm := elem.(*types.Named); iselnm {
				if IsResource(elnm.Obj(), elnm) {
					unptr = true
				} else if spec, _ := IsSpecial(elnm.Obj()); spec {
					unptr = true
				}
			}
		}
		if unptr {
			return g.GenTypeName(elem, false)
		}
		return fmt.Sprintf("*%v", g.GenTypeName(u.Elem(), false))
	case *types.Slice:
		return fmt.Sprintf("[]%v", g.GenTypeName(u.Elem(), false)) // postfix syntax for arrays.
	default:
		contract.Failf("Unrecognized GenTypeName type: %v", reflect.TypeOf(u))
	}
	return ""
}

// registerImport registers that we have seen a foreign package and requests that the imports be emitted for it.
func (g *RPCGenerator) registerImport(pkg *types.Package) string {
	path := pkg.Path()
	if impname, has := g.FileImports[path]; has {
		return impname
	}

	// If we haven't seen this yet, allocate an import name for it.  For now, we just use the package name with two
	// leading underscores, to avoid accidental collisions with other names in the file.
	name := "__" + pkg.Name()
	g.FileImports[path] = name
	return name
}

func (g *RPCGenerator) EmitTypedefs(w *tools.GenWriter, typedefs []Typedef) {
	w.Writefmtln("/* Typedefs */")
	w.Writefmtln("")

	w.Writefmtln("type (")
	for _, td := range typedefs {
		w.Writefmtln("    %v %v", td.Name(), td.Target())
	}
	w.Writefmtln(")")

	w.Writefmtln("")
}

func (g *RPCGenerator) EmitConstants(w *tools.GenWriter, consts []*Const) {
	w.Writefmtln("/* Constants */")
	w.Writefmtln("")

	w.Writefmtln("const (")
	for _, konst := range consts {
		w.Writefmtln("    %v %v = %v", konst.Name(), g.GenTypeName(konst.Type, false), konst.Value)
	}
	w.Writefmtln(")")

	w.Writefmtln("")
}
