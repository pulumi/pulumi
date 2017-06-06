// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumidl

import (
	"bufio"
	"bytes"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type RPCGenerator struct {
	IDLRoot         string            // the root where IDL is loaded from.
	IDLPkgBase      string            // the IDL's base package path.
	RPCPkgBase      string            // the RPC's base package path.
	Out             string            // where RPC stub outputs will be saved.
	CurrPkg         *Package          // the package currently being visited.
	CurrFile        string            // the file currently being visited.
	FileHadRes      bool              // true if the file had at least one resource.
	FileHadNamedRes bool              // true if the file had at least one named resource.
	FileImports     map[string]string // a map of foreign packages used in a file.
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
	oldHadRes, oldHadNamedRes, oldImports := g.FileHadRes, g.FileHadNamedRes, g.FileImports
	g.FileHadRes, g.FileHadNamedRes, g.FileImports = false, false, make(map[string]string)
	defer (func() {
		g.FileHadRes = oldHadRes
		g.FileHadNamedRes = oldHadNamedRes
		g.FileImports = oldImports
	})()

	// First, generate the body.  This is required first so we know which imports to emit.
	body := g.genFileBody(file, pkg, members)

	// Open up a writer that overwrites whatever file contents already exist.
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	// Emit a header into the file.
	emitHeaderWarning(w)

	// Now emit the package name at the top-level.
	writefmtln(w, "package %v", pkg.Pkginfo.Pkg.Name())
	writefmtln(w, "")

	// And all of the imports that we're going to need.
	if g.FileHadRes || len(g.FileImports) > 0 {
		writefmtln(w, "import (")

		if g.FileHadRes {
			if g.FileHadNamedRes {
				writefmtln(w, `    "errors"`)
				writefmtln(w, "")
			}
			writefmtln(w, `    pbempty "github.com/golang/protobuf/ptypes/empty"`)
			writefmtln(w, `    pbstruct "github.com/golang/protobuf/ptypes/struct"`)
			writefmtln(w, `    "golang.org/x/net/context"`)
			writefmtln(w, "")
			writefmtln(w, `    "github.com/pulumi/lumi/pkg/resource"`)
			writefmtln(w, `    "github.com/pulumi/lumi/pkg/tokens"`)
			writefmtln(w, `    "github.com/pulumi/lumi/pkg/util/contract"`)
			writefmtln(w, `    "github.com/pulumi/lumi/pkg/util/mapper"`)
			writefmtln(w, `    "github.com/pulumi/lumi/sdk/go/pkg/lumirpc"`)
		}

		if len(g.FileImports) > 0 {
			if g.FileHadRes {
				writefmtln(w, "")
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

				writefmtln(w, `    %v "%v"`, name, imppath)
			}
		}

		writefmtln(w, ")")
		writefmtln(w, "")
	}

	// Now finally emit the actual body and close out the file.
	writefmtln(w, "%v", body)
	return w.Flush()
}

func (g *RPCGenerator) genFileBody(file string, pkg *Package, members []Member) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

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

	w.Flush()
	return buffer.String()
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

func (g *RPCGenerator) EmitResource(w *bufio.Writer, module tokens.Module, pkg *Package, res *Resource) {
	name := res.Name()
	writefmtln(w, "/* RPC stubs for %v resource provider */", name)
	writefmtln(w, "")

	// Remember when we encounter resources so we can import the right packages.
	g.FileHadRes = true
	if res.Named {
		g.FileHadNamedRes = true
	}

	propopts := res.PropertyOptions()

	// Emit a type token.
	token := fmt.Sprintf("%v:%v:%v", pkg.Name, module, name)
	writefmtln(w, "// %[1]vToken is the type token corresponding to the %[1]v package type.", name)
	writefmtln(w, `const %vToken = tokens.Type("%v")`, name, token)
	writefmtln(w, "")

	// Now, generate an ops interface that the real provider will implement.
	writefmtln(w, "// %[1]vProviderOps is a pluggable interface for %[1]v-related management functionality.", name)
	writefmtln(w, "type %vProviderOps interface {", name)
	writefmtln(w, "    Check(ctx context.Context, obj *%v) ([]error, error)", name)
	if !res.Named {
		writefmtln(w, "    Name(ctx context.Context, obj *%v) (string, error)", name)
	}
	writefmtln(w, "    Create(ctx context.Context, obj *%v) (resource.ID, error)", name)
	writefmtln(w, "    Get(ctx context.Context, id resource.ID) (*%v, error)", name)
	writefmtln(w, "    InspectChange(ctx context.Context,")
	writefmtln(w, "        id resource.ID, old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) ([]string, error)", name)
	writefmtln(w, "    Update(ctx context.Context,")
	writefmtln(w, "        id resource.ID, old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) error", name)
	writefmtln(w, "    Delete(ctx context.Context, id resource.ID) error")
	writefmtln(w, "}")
	writefmtln(w, "")

	// Next generate all the RPC scaffolding goo
	writefmtln(w, "// %[1]vProvider is a dynamic gRPC-based plugin for managing %[1]v resources.", name)
	writefmtln(w, "type %vProvider struct {", name)
	writefmtln(w, "    ops %vProviderOps", name)
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "// New%vProvider allocates a resource provider that delegates to a ops instance.", name)
	writefmtln(w, "func New%[1]vProvider(ops %[1]vProviderOps) lumirpc.ResourceProviderServer {", name)
	writefmtln(w, "    contract.Assert(ops != nil)")
	writefmtln(w, "    return &%vProvider{ops: ops}", name)
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Check(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.CheckRequest) (*lumirpc.CheckResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, err := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if err == nil {")
	writefmtln(w, "        if failures, err := p.ops.Check(ctx, obj); err != nil {")
	writefmtln(w, "            return nil, err")
	writefmtln(w, "        } else if len(failures) > 0 {")
	writefmtln(w, "            err = resource.NewCheckError(failures)")
	writefmtln(w, "        }")
	writefmtln(w, "    }")
	writefmtln(w, "    return resource.NewCheckResponse(err), nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Name(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.NameRequest) (*lumirpc.NameResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, err := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	if res.Named {
		// For named resources, we have a canonical way of fetching the name.
		writefmtln(w, `    if obj.Name == nil || *obj.Name == "" {`)
		writefmtln(w, `        if req.Unknowns[%v_Name] {`, name)
		writefmtln(w, `            return nil, errors.New("Name property cannot be computed from unknown outputs")`)
		writefmtln(w, "        }")
		writefmtln(w, `        return nil, errors.New("Name property cannot be empty")`)
		writefmtln(w, "    }")
		writefmtln(w, "    return &lumirpc.NameResponse{Name: *obj.Name}, nil")
	} else {
		// For all other resources, delegate to the underlying provider to perform the naming operation.
		writefmtln(w, "    name, err := p.ops.Name(ctx, obj)")
		writefmtln(w, "    return &lumirpc.NameResponse{Name: name}, err")
	}
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Create(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.CreateRequest) (*lumirpc.CreateResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, err := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    id, err := p.ops.Create(ctx, obj)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &lumirpc.CreateResponse{Id: string(id)}, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Get(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.GetRequest) (*lumirpc.GetResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    id := resource.ID(req.GetId())")
	writefmtln(w, "    obj, err := p.ops.Get(ctx, id)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &lumirpc.GetResponse{")
	writefmtln(w, "        Properties: resource.MarshalProperties(")
	writefmtln(w, "            nil, resource.NewPropertyMap(obj), resource.MarshalOptions{}),")
	writefmtln(w, "    }, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) InspectChange(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.InspectChangeRequest) (*lumirpc.InspectChangeResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    id := resource.ID(req.GetId())")
	writefmtln(w, "    old, oldprops, err := p.Unmarshal(req.GetOlds())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    new, newprops, err := p.Unmarshal(req.GetNews())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    var replaces []string")
	writefmtln(w, "    diff := oldprops.Diff(newprops)")
	writefmtln(w, "    if diff != nil {")
	for _, opts := range propopts {
		if opts.Replaces {
			writefmtln(w, "        if diff.Changed(\"%v\") {", opts.Name)
			writefmtln(w, "            replaces = append(replaces, \"%v\")", opts.Name)
			writefmtln(w, "        }")
		}
	}
	writefmtln(w, "    }")
	writefmtln(w, "    more, err := p.ops.InspectChange(ctx, id, old, new, diff)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &lumirpc.InspectChangeResponse{")
	writefmtln(w, "        Replaces: append(replaces, more...),")
	writefmtln(w, "    }, err")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Update(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.UpdateRequest) (*pbempty.Empty, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    id := resource.ID(req.GetId())")
	writefmtln(w, "    old, oldprops, err := p.Unmarshal(req.GetOlds())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    new, newprops, err := p.Unmarshal(req.GetNews())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    diff := oldprops.Diff(newprops)")
	writefmtln(w, "    if err := p.ops.Update(ctx, id, old, new, diff); err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &pbempty.Empty{}, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Delete(", name)
	writefmtln(w, "    ctx context.Context, req *lumirpc.DeleteRequest) (*pbempty.Empty, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    id := resource.ID(req.GetId())")
	writefmtln(w, "    if err := p.ops.Delete(ctx, id); err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &pbempty.Empty{}, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Unmarshal(", name)
	writefmtln(w, "    v *pbstruct.Struct) (*%v, resource.PropertyMap, error) {", name)
	writefmtln(w, "    var obj %v", name)
	writefmtln(w, "    props := resource.UnmarshalProperties(nil, v, resource.MarshalOptions{RawResources: true})")
	writefmtln(w, "    return &obj, props, mapper.MapIU(props.Mappable(), &obj)")
	writefmtln(w, "}")
	writefmtln(w, "")
}

func (g *RPCGenerator) EmitStructType(w *bufio.Writer, module tokens.Module, pkg *Package, t TypeMember) {
	name := t.Name()
	writefmtln(w, "/* Marshalable %v structure(s) */", name)
	writefmtln(w, "")

	props := t.Properties()
	propopts := t.PropertyOptions()
	writefmtln(w, "// %v is a marshalable representation of its corresponding IDL type.", name)
	writefmtln(w, "type %v struct {", name)
	for i, prop := range props {
		opts := propopts[i]
		// Make a JSON tag for this so we can serialize; note that outputs are always optional in this position.
		jsontag := makeLumiTag(opts)
		writefmtln(w, "    %v %v %v",
			prop.Name(), g.GenTypeName(prop.Type(), opts.Optional || opts.In || opts.Out), jsontag)
	}
	writefmtln(w, "}")
	writefmtln(w, "")

	if len(props) > 0 {
		writefmtln(w, "// %v's properties have constants to make dealing with diffs and property bags easier.", name)
		writefmtln(w, "const (")
		for i, prop := range props {
			opts := propopts[i]
			writefmtln(w, "    %v_%v = \"%v\"", name, prop.Name(), opts.Name)
		}
		writefmtln(w, ")")
		writefmtln(w, "")
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
		if res, _ := IsResource(obj, u); res {
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
				if res, _ := IsResource(elnm.Obj(), elnm); res {
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

func (g *RPCGenerator) EmitTypedefs(w *bufio.Writer, typedefs []Typedef) {
	writefmtln(w, "/* Typedefs */")
	writefmtln(w, "")

	writefmtln(w, "type (")
	for _, td := range typedefs {
		writefmtln(w, "    %v %v", td.Name(), td.Target())
	}
	writefmtln(w, ")")

	writefmtln(w, "")
}

func (g *RPCGenerator) EmitConstants(w *bufio.Writer, consts []*Const) {
	writefmtln(w, "/* Constants */")
	writefmtln(w, "")

	writefmtln(w, "const (")
	for _, konst := range consts {
		writefmtln(w, "    %v %v = %v", konst.Name(), g.GenTypeName(konst.Type, false), konst.Value)
	}
	writefmtln(w, ")")

	writefmtln(w, "")
}
