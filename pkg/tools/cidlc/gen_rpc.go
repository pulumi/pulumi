// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"bufio"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

type RPCGenerator struct {
	Root    string
	Out     string
	Currpkg *Package // the package currently being visited.
}

func NewRPCGenerator(root string, out string) *RPCGenerator {
	return &RPCGenerator{
		Root: root,
		Out:  out,
	}
}

func (g *RPCGenerator) Generate(pkg *Package) error {
	// Ensure the directory structure exists in the target.
	if err := mirrorDirLayout(pkg, g.Out); err != nil {
		return err
	}

	// Install context about the current entity being visited.
	oldpkg := g.Currpkg
	g.Currpkg = pkg
	defer (func() {
		g.Currpkg = oldpkg
	})()

	// Now walk through the package, file by file, and generate the contents.
	for relpath, file := range pkg.Files {
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
	writefmtln(w, "package %vrpc", pkg.Name)
	writefmtln(w, "")

	// And all of the imports that we're going to need.
	// TODO: what about imports for cross-package references.
	writefmtln(w, "import (")
	writefmtln(w, `    "errors"`)
	writefmtln(w, `    pbempty "github.com/golang/protobuf/ptypes/empty"`)
	writefmtln(w, `    pbstruct "github.com/golang/protobuf/ptypes/struct"`)
	writefmtln(w, `    "github.com/pulumi/coconut/pkg/resource"`)
	writefmtln(w, `    "github.com/pulumi/coconut/pkg/tokens"`)
	writefmtln(w, `    "github.com/pulumi/coconut/pkg/util/contract"`)
	writefmtln(w, `    "github.com/pulumi/coconut/pkg/util/mapper"`)
	writefmtln(w, `    "github.com/pulumi/coconut/sdk/go/pkg/cocorpc"`)
	writefmtln(w, `    "golang.org/x/net/context"`)
	writefmtln(w, ")")
	writefmtln(w, "")

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

	return w.Flush()
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

	hasouts := false
	propopts := res.PropertyOptions()
	for _, opts := range propopts {
		if opts.Out {
			hasouts = true
			break
		}
	}

	// Emit a type token.
	token := fmt.Sprintf("%v:%v:%v", pkg.Name, module, name)
	writefmtln(w, "// %[1]vToken is the type token corresponding to the %[1]v package type.", name)
	writefmtln(w, `const %vToken = tokens.Type("%v")`, name, token)
	writefmtln(w, "")

	// Now, generate an ops interface that the real provider will implement.
	writefmtln(w, "// %[1]vProviderOps is a pluggable interface for %[1]v-related management functionality.", name)
	writefmtln(w, "type %vProviderOps interface {", name)
	writefmtln(w, "    Check(ctx context.Context, obj *%v) ([]mapper.FieldError, error)", name)
	if !res.Named {
		writefmtln(w, "    Name(ctx context.Context, obj *%v) (string, error)", name)
	}
	writefmt(w, "    Create(ctx context.Context, obj *%v) (string, ", name)
	if hasouts {
		writefmt(w, "*%vOuts, ", name)
	}
	writefmtln(w, "error)")
	writefmtln(w, "    Get(ctx context.Context, id string) (*%v, error)", name)
	writefmtln(w, "    PreviewUpdate(ctx context.Context,")
	writefmtln(w, "        id string, old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) ([]string, error)", name)
	writefmtln(w, "    Update(ctx context.Context,")
	writefmtln(w, "        id string, old *%[1]v, new *%[1]v, diff *resource.ObjectDiff) error", name)
	writefmtln(w, "    Delete(ctx context.Context, id string) error")
	writefmtln(w, "}")
	writefmtln(w, "")

	// Next generate all the RPC scaffolding goo
	writefmtln(w, "// %[1]vProvider is a dynamic gRPC-based plugin for managing %[1]v resources.", name)
	writefmtln(w, "type %vProvider struct {", name)
	writefmtln(w, "    ops %vProviderOps", name)
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "// New%vProvider allocates a resource provider that delegates to a ops instance.", name)
	writefmtln(w, "func New%[1]vProvider(ops %[1]vProviderOps) cocorpc.ResourceProviderServer {", name)
	writefmtln(w, "    contract.Assert(ops != nil)")
	writefmtln(w, "    return &%vProvider{ops: ops}", name)
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Check(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, decerr := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if decerr == nil || len(decerr.Failures()) == 0 {")
	writefmtln(w, "        failures, err := p.ops.Check(ctx, obj)")
	writefmtln(w, "        if err != nil {")
	writefmtln(w, "            return nil, err")
	writefmtln(w, "        }")
	writefmtln(w, "        if len(failures) > 0 {")
	writefmtln(w, "            decerr = mapper.NewDecodeErr(failures)")
	writefmtln(w, "        }")
	writefmtln(w, "    }")
	writefmtln(w, "    return resource.NewCheckResponse(decerr), nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Name(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, err := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	if res.Named {
		// For named resources, we have a canonical way of fetching the name.
		writefmtln(w, `    if obj.Name == "" {`)
		writefmtln(w, `        return nil, errors.New("Name property cannot be empty")`)
		writefmtln(w, "    }")
		writefmtln(w, "    return &cocorpc.NameResponse{Name: obj.Name}, nil")
	} else {
		// For all other resources, delegate to the underlying provider to perform the naming operation.
		writefmtln(w, "    name, err := p.ops.Name(ctx, obj)")
		writefmtln(w, "    return &cocorpc.NameResponse{Name: name}, err")
	}
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Create(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    obj, _, decerr := p.Unmarshal(req.GetProperties())")
	writefmtln(w, "    if decerr != nil {")
	writefmtln(w, "        return nil, decerr")
	writefmtln(w, "    }")
	writefmt(w, "    id, ")
	if hasouts {
		writefmt(w, "outs, ")
	}
	writefmtln(w, "err := p.ops.Create(ctx, obj)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	// TODO: validate the output (e.g., required ones are non-nil, etc).
	writefmtln(w, "    return &cocorpc.CreateResponse{")
	writefmtln(w, "        Id:   id,")
	if hasouts {
		writefmtln(w, "        Outputs: resource.MarshalProperties(")
		writefmtln(w, "            nil, resource.NewPropertyMap(outs), resource.MarshalOptions{},")
		writefmtln(w, "        ),")
	}
	writefmtln(w, "    }, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Get(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.GetRequest) (*cocorpc.GetResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    id := req.GetId()")
	writefmtln(w, "    obj, err := p.ops.Get(ctx, id)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &cocorpc.GetResponse{")
	writefmtln(w, "        Properties: resource.MarshalProperties(")
	writefmtln(w, "            nil, resource.NewPropertyMap(obj), resource.MarshalOptions{}),")
	writefmtln(w, "    }, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) PreviewUpdate(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.PreviewUpdateResponse, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    old, oldprops, decerr := p.Unmarshal(req.GetOlds())")
	writefmtln(w, "    if decerr != nil {")
	writefmtln(w, "        return nil, decerr")
	writefmtln(w, "    }")
	writefmtln(w, "    new, newprops, decerr := p.Unmarshal(req.GetNews())")
	writefmtln(w, "    if decerr != nil {")
	writefmtln(w, "        return nil, decerr")
	writefmtln(w, "    }")
	writefmtln(w, "    diff := oldprops.Diff(newprops)")
	writefmtln(w, "    var replaces []string")
	for _, opts := range propopts {
		if opts.Replaces {
			writefmtln(w, "    if diff.Changed(\"%v\") {", opts.Name)
			writefmtln(w, "        replaces = append(replaces, \"%v\")", opts.Name)
			writefmtln(w, "    }")
		}
	}
	writefmtln(w, "    more, err := p.ops.PreviewUpdate(ctx, req.GetId(), old, new, diff)")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &cocorpc.PreviewUpdateResponse{")
	writefmtln(w, "        Replaces: append(replaces, more...),")
	writefmtln(w, "    }, err")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Update(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    old, oldprops, err := p.Unmarshal(req.GetOlds())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    new, newprops, err := p.Unmarshal(req.GetNews())")
	writefmtln(w, "    if err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    diff := oldprops.Diff(newprops)")
	writefmtln(w, "    if err := p.ops.Update(ctx, req.GetId(), old, new, diff); err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &pbempty.Empty{}, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Delete(", name)
	writefmtln(w, "    ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {")
	writefmtln(w, "    contract.Assert(req.GetType() == string(%vToken))", name)
	writefmtln(w, "    if err := p.ops.Delete(ctx, req.GetId()); err != nil {")
	writefmtln(w, "        return nil, err")
	writefmtln(w, "    }")
	writefmtln(w, "    return &pbempty.Empty{}, nil")
	writefmtln(w, "}")
	writefmtln(w, "")
	writefmtln(w, "func (p *%vProvider) Unmarshal(", name)
	writefmtln(w, "    v *pbstruct.Struct) (*%v, resource.PropertyMap, mapper.DecodeError) {", name)
	writefmtln(w, "    var obj %v", name)
	writefmtln(w, "    props := resource.UnmarshalProperties(v)")
	writefmtln(w, "    result := mapper.MapIU(props.Mappable(), &obj)")
	writefmtln(w, "    return &obj, props, result")
	writefmtln(w, "}")
	writefmtln(w, "")
}

func (g *RPCGenerator) EmitStructType(w *bufio.Writer, module tokens.Module, pkg *Package, t TypeMember) {
	name := t.Name()
	writefmtln(w, "/* Marshalable %v structure(s) */", name)
	writefmtln(w, "")

	var outs []int
	props := t.Properties()
	propopts := t.PropertyOptions()
	writefmtln(w, "// %v is a marshalable representation of its corresponding IDL type.", name)
	writefmtln(w, "type %v struct {", name)
	for i, prop := range props {
		opts := propopts[i]
		if opts.Out {
			outs = append(outs, i) // remember this so we can generate an out struct.
		}
		// Make a JSON tag for this so we can serialize; note that outputs are always optional in this position.
		jsontag := makeJSONTag(opts, opts.Out)
		writefmtln(w, "    %v %v %v", prop.Name(), g.GenTypeName(prop.Type()), jsontag)
	}
	writefmtln(w, "}")
	writefmtln(w, "")

	if len(outs) > 0 {
		writefmtln(w, "// %vOuts is a marshalable representation of its IDL type's output properties.", name)
		writefmtln(w, "type %vOuts struct {", name)
		for _, out := range outs {
			prop := props[out]
			opts := propopts[out]
			jsontag := makeJSONTag(opts, false)
			writefmtln(w, "    %v %v %v", prop.Name(), g.GenTypeName(prop.Type()), jsontag)
		}
		writefmtln(w, "}")
		writefmtln(w, "")
	}

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

// makeJSONTag turns a set of property options into a serializable JSON tag.
func makeJSONTag(opts PropertyOptions, forceopt bool) string {
	var flags string
	if forceopt || opts.Optional {
		flags = ",omitempty"
	}
	return fmt.Sprintf("`json:\"%v%v\"`", opts.Name, flags)
}

func (g *RPCGenerator) GenTypeName(t types.Type) string {
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
	case *types.Named:
		obj := u.Obj()
		// For resource types, simply emit an ID, since that is what will have been serialized.
		if isres, _ := IsResource(obj, u.Underlying()); isres {
			return "resource.ID"
		}

		// Otherwise, see how to reference the type, based on imports.
		pkg := obj.Pkg()
		name := obj.Name()

		// If this came from the same package, Go can access it without qualification.
		if pkg == g.Currpkg.Pkginfo.Pkg {
			return name
		}

		// Otherwise, we will need to refer to a qualified import name.
		// TODO: we will need to generate the right imports before we can emit such names.
		contract.Failf("Cross-package IDL references not yet supported")
	case *types.Map:
		return fmt.Sprintf("map[%v]%v", g.GenTypeName(u.Key()), g.GenTypeName(u.Elem()))
	case *types.Pointer:
		return fmt.Sprintf("*%v", g.GenTypeName(u.Elem()))
	case *types.Slice:
		return fmt.Sprintf("[]%v", g.GenTypeName(u.Elem())) // postfix syntax for arrays.
	default:
		contract.Failf("Unrecognized GenTypeName type: %v", reflect.TypeOf(u))
	}
	return ""
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
		writefmtln(w, "    %v %v = %v", konst.Name(), g.GenTypeName(konst.Type), konst.Value)
	}
	writefmtln(w, ")")

	writefmtln(w, "")
}
