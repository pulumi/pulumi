// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"fmt"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/tools"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// TODO[pulumi/pulumi-fabric#139]: preserve GoDocs in the generated code.

type PackGenerator struct {
	Program     *loader.Program          // the compiled Go program.
	IDLRoot     string                   // the path to the IDL on disk.
	IDLPkgBase  string                   // the IDL's base package path.
	Out         string                   // where to store the output package.
	CurrPkg     *Package                 // the package currently being visited.
	CurrFile    string                   // the file currently being visited.
	FileHadRes  bool                     // true if the file had at least one resource.
	FileImports map[string]MemberImports // a map of imported package paths and members.
}

type MemberImports map[tokens.Name]string

func NewPackGenerator(prog *loader.Program, root string, pkgBase string, out string) *PackGenerator {
	return &PackGenerator{
		Program:    prog,
		IDLRoot:    root,
		IDLPkgBase: pkgBase,
		Out:        out,
	}
}

// Filename gets the source filename for a given Go element.
func (g *PackGenerator) Filename(elem goPos) string {
	pos := elem.Pos()
	fset := g.Program.Fset
	return fset.Position(pos).Filename
}

// Generate generates a Lumi package's source code from a given compiled IDL program.
func (g *PackGenerator) Generate(pkg *Package) error {
	// Ensure the directory structure exists in the target.
	if err := mirrorDirLayout(pkg, g.Out); err != nil {
		return err
	}

	// Install context about the current entity being visited.
	oldpkg, oldfile := g.CurrPkg, g.CurrFile
	g.CurrPkg = pkg
	defer (func() {
		g.CurrPkg, g.CurrFile = oldpkg, oldfile
	})()

	// Now walk through the package, file by file, and generate the contents.
	for relpath, file := range pkg.Files {
		var members []Member
		for _, nm := range file.MemberNames {
			members = append(members, file.Members[nm])
		}
		g.CurrFile = relpath
		path := filepath.Join(g.Out, relpath)
		if err := g.EmitFile(path, members); err != nil {
			return err
		}
	}

	return nil
}

func (g *PackGenerator) EmitFile(file string, members []Member) error {
	// Set up context.
	oldHadRes, oldImports := g.FileHadRes, g.FileImports
	g.FileHadRes, g.FileImports = false, make(map[string]MemberImports)
	defer (func() {
		g.FileHadRes = oldHadRes
		g.FileImports = oldImports
	})()

	// First, generate the body.  This is required first so we know which imports to emit.
	body := g.genFileBody(members)

	// Next actually open up the file and emit the header, imports, and the body of the module.
	return g.emitFileContents(file, body)
}

func (g *PackGenerator) emitFileContents(file string, body string) error {
	// The output is TypeScript, so alter the extension.
	if dotindex := strings.LastIndex(file, "."); dotindex != -1 {
		file = file[:dotindex]
	}
	file += ".ts"

	// Open up a writer that overwrites whatever file contents already exist.
	w, err := tools.NewGenWriter(lumidl, file)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Emit a header into the file.
	w.EmitHeaderWarning()
	w.Writefmtln("/* tslint:disable:ordered-imports variable-name */")

	// If there are any resources, import the Lumi package.
	if g.FileHadRes {
		w.Writefmtln("import * as lumi from \"@lumi/lumi\";")
		w.Writefmtln("import * as lumirt from \"@lumi/lumirt\";")
		w.Writefmtln("")
	}
	if len(g.FileImports) > 0 {
		// First, sort the imported package names, to ensure determinism.
		var ipkgs []string
		for ipkg := range g.FileImports {
			ipkgs = append(ipkgs, ipkg)
		}
		sort.Strings(ipkgs)

		for _, ipkg := range ipkgs {
			// Produce a map of filename to the members in that file that have been imported.
			importMap := make(map[string][]tokens.Name)
			for member, file := range g.FileImports[ipkg] {
				importMap[file] = append(importMap[file], member)
			}

			// Now sort them to ensure determinism.
			var importFiles []string
			for file := range importMap {
				importFiles = append(importFiles, file)
			}
			sort.Strings(importFiles)

			// Next, walk each imported file and import all members from within it.
			for _, file := range importFiles {
				// Make a relative import path from the current file.
				contract.Assertf(strings.HasPrefix(file, g.IDLRoot),
					"Inter-IDL package references not yet supported (%v is not part of %v)", file, g.IDLRoot)
				dir := filepath.Dir(g.CurrFile)
				relimp, err := filepath.Rel(dir, file[len(g.IDLRoot)+1:])
				contract.Assertf(err == nil, "Unexpected filepath.Rel error: %v", err)
				var impname string
				if strings.HasPrefix(relimp, ".") {
					impname = relimp
				} else {
					impname = "./" + relimp
				}
				if filepath.Ext(impname) != "" {
					lastdot := strings.LastIndex(impname, ".")
					impname = impname[:lastdot]
				}

				// Now produce a sorted list of imported members, again to ensure determinism.
				members := importMap[file]
				contract.Assert(len(members) > 0)
				sort.Slice(members, func(i, j int) bool {
					return string(members[i]) < string(members[j])
				})

				// Finally, go through and produce the import clause.
				w.Writefmt("import {")
				for i, member := range members {
					if i > 0 {
						w.Writefmt(", ")
					}
					w.Writefmt(string(member))
				}
				w.Writefmtln("} from \"%v\";", impname)
			}
		}
		w.Writefmtln("")
	}

	w.Writefmt("%v", body)
	return nil
}

func (g *PackGenerator) genFileBody(members []Member) string {
	// Accumulate the buffer in a string.
	w, err := tools.NewGenWriter(lumidl, "")
	contract.IgnoreError(err)

	// Now go ahead and emit the code for all members of this package.
	for i, m := range members {
		if i > 0 {
			// Allow aliases and consts to pile up without line breaks.
			_, isalias := m.(*Alias)
			_, isconst := m.(*Const)
			if (!isalias && !isconst) || reflect.TypeOf(m) != reflect.TypeOf(members[i-1]) {
				w.Writefmtln("")
			}
		}
		switch t := m.(type) {
		case *Alias:
			g.EmitAlias(w, t)
		case *Const:
			g.EmitConst(w, t)
		case *Enum:
			g.EmitEnum(w, t)
		case *Resource:
			g.EmitResource(w, t)
		case *Struct:
			g.EmitStruct(w, t)
		default:
			contract.Failf("Unrecognized package member type: %v", reflect.TypeOf(m))
		}
	}

	w.Writefmtln("")
	err = w.Flush()
	contract.IgnoreError(err)
	return w.Buffer()
}

func (g *PackGenerator) EmitAlias(w *tools.GenWriter, alias *Alias) {
	w.Writefmtln("export type %v = %v;", alias.Name(), g.GenTypeName(alias.Target()))
}

func (g *PackGenerator) EmitConst(w *tools.GenWriter, konst *Const) {
	w.Writefmtln("export let %v: %v = %v;", konst.Name(), g.GenTypeName(konst.Type), konst.Value.String())
}

func (g *PackGenerator) EmitEnum(w *tools.GenWriter, enum *Enum) {
	w.Writefmtln("export type %v =", enum.Name())
	contract.Assert(len(enum.Values) > 0)
	for i, value := range enum.Values {
		if i > 0 {
			w.Writefmtln(" |")
		}
		w.Writefmt("    %v", value)
	}
	w.Writefmtln(";")
}

func (g *PackGenerator) EmitResource(w *tools.GenWriter, res *Resource) {
	// Emit the full resource class definition, including constructor, etc.
	g.emitResourceClass(w, res)
	w.Writefmtln("")

	// Finally, emit an entire struct type for the args interface.
	g.emitStructType(w, res, res.Name()+tokens.Name("Args"))

	// Remember we had a resource in this file so we can import the right stuff.
	g.FileHadRes = true
}

func (g *PackGenerator) emitResourceClass(w *tools.GenWriter, res *Resource) {
	// Emit the class definition itself.
	name := res.Name()
	var base string
	if res.Named {
		base = "NamedResource"
	} else {
		base = "Resource"
	}
	w.Writefmtln("export class %v extends lumi.%v implements %vArgs {", name, base, name)

	// First, emit all fields definitions.
	hasArgs := false
	hasName := false
	hasRequiredArgs := false
	fn := forEachField(res, func(fld *types.Var, opt PropertyOptions) {
		if isResourceNameProperty(res, opt) {
			hasName = true
		} else {
			g.emitField(w, fld, opt, "    public ")
			if !opt.Out {
				hasArgs = true
				if !opt.Optional {
					hasRequiredArgs = true
				}
			}
		}
	})
	if fn > 0 {
		w.Writefmtln("")
	}

	// Add the standard "factory" functions: get and query.  These are static, so they go before the constructor.
	w.Writefmtln("    public static get(id: lumi.ID): %v {", name)
	w.Writefmtln("        return <any>undefined; // functionality provided by the runtime")
	w.Writefmtln("    }")
	w.Writefmtln("")
	w.Writefmtln("    public static query(q: any): %v[] {", name)
	w.Writefmtln("        return <any>undefined; // functionality provided by the runtime")
	w.Writefmtln("    }")
	w.Writefmtln("")

	// Next, a constructor that validates arguments and self-assigns them.
	w.Writefmt("    constructor(")
	if res.Named {
		w.Writefmt("urnName: string, ")
	}
	w.Writefmt("args")
	if !hasRequiredArgs {
		w.Writefmt("?")
	}
	w.Writefmtln(": %vArgs) {", name)

	if hasName {
		// Named properties are passed as the constructor's first argument.
		w.Writefmtln("        super(urnName);")
	} else {
		w.Writefmtln("        super();")
	}

	// Next, validate that required parameters exist, and store all arguments on the object.
	argLinePrefix := "        "
	needsArgsCheck := hasArgs && !hasRequiredArgs
	if needsArgsCheck {
		w.Writefmtln("        if (args !== undefined) {")
		argLinePrefix += "    "
	}
	forEachField(res, func(fld *types.Var, opt PropertyOptions) {
		if !opt.Out && !isResourceNameProperty(res, opt) {
			if !opt.Optional {
				w.Writefmtln("%vif (lumirt.defaultIfComputed(args.%v, \"\") === undefined) {", argLinePrefix, opt.Name)
				w.Writefmtln("%v    throw new Error(\"Missing required argument '%v'\");", argLinePrefix, opt.Name)
				w.Writefmtln("%v}", argLinePrefix)
			}
			w.Writefmtln("%vthis.%v = args.%v;", argLinePrefix, opt.Name, opt.Name)
		}
	})
	if needsArgsCheck {
		w.Writefmtln("        }")
	}

	w.Writefmtln("    }")
	w.Writefmtln("}")
}

func isResourceNameProperty(res *Resource, prop PropertyOptions) bool {
	return res.Named && prop.Name == "urnName"
}

func (g *PackGenerator) EmitStruct(w *tools.GenWriter, s *Struct) {
	g.emitStructType(w, s, s.Name())
}

func (g *PackGenerator) emitStructType(w *tools.GenWriter, t TypeMember, name tokens.Name) {
	w.Writefmtln(fmt.Sprintf("export interface %v {", name))
	forEachField(t, func(fld *types.Var, opt PropertyOptions) {
		if opt.Out {
			return // skip output properties, since those exist solely on the resource class.
		} else if res, isres := t.(*Resource); isres && isResourceNameProperty(res, opt) {
			return // skip resource names, since those are part of the resource but not its property object.
		}
		g.emitField(w, fld, opt, "    ")
	})
	w.Writefmtln("}")
}

func (g *PackGenerator) emitField(w *tools.GenWriter, fld *types.Var, opt PropertyOptions, prefix string) {
	var readonly string
	var optional string
	var typ string
	if opt.Replaces {
		readonly = "readonly "
	}
	if opt.Optional {
		optional = "?"
	}
	typ = g.GenTypeName(fld.Type())
	w.Writefmtln("%v%v%v%v: %v;", prefix, readonly, opt.Name, optional, typ)
}

func (g *PackGenerator) GenTypeName(t types.Type) string {
	switch u := t.(type) {
	case *types.Basic:
		switch k := u.Kind(); k {
		case types.Bool:
			return "boolean"
		case types.String:
			return "string"
		case types.Float64:
			return "number"
		default:
			contract.Failf("Unrecognized GenTypeName basic type: %v", k)
		}
	case *types.Interface:
		return "any"
	case *types.Named:
		obj := u.Obj()
		if spec, kind := IsSpecial(obj); spec {
			switch kind {
			case SpecialArchiveType:
				return "lumi.asset.Archive"
			case SpecialAssetType:
				return "lumi.asset.Asset"
			case SpecialResourceType, SpecialNamedResourceType:
				return "lumi.Resource"
			default:
				contract.Failf("Unrecognized special type: %v", kind)
			}
		}

		// Our import logic will have arranged for the type name to be available.
		// IDEA: consider auto-generated import names to avoid conflicts between imported and local names.
		g.trackNameReference(obj)
		return obj.Name()
	case *types.Map:
		return fmt.Sprintf("{[key: %v]: %v}", g.GenTypeName(u.Key()), g.GenTypeName(u.Elem()))
	case *types.Pointer:
		return g.GenTypeName(u.Elem()) // no pointers in TypeScript, just emit the underlying type.
	case *types.Slice:
		return fmt.Sprintf("%v[]", g.GenTypeName(u.Elem())) // postfix syntax for arrays.
	default:
		contract.Failf("Unrecognized GenTypeName type: %v", reflect.TypeOf(u))
	}
	return ""
}

// trackNameReference registers that we have seen a foreign package and requests that the imports be emitted for it.
func (g *PackGenerator) trackNameReference(obj *types.TypeName) {
	// If a reference to a type within the same package and file, there is no need to register anything.
	pkg := obj.Pkg()
	member := tokens.Name(obj.Name())
	if pkg == g.CurrPkg.Pkginfo.Pkg &&
		g.CurrPkg.MemberFiles[member].Path == g.CurrFile {
		return
	}

	// Otherwise, we need to track the member so that we can import it later on.  Make sure not to add duplicates
	// because we want to ensure we don't import the same thing twice.
	path := pkg.Path()
	members, has := g.FileImports[path]
	if !has {
		members = make(MemberImports)
		g.FileImports[path] = members
	}
	members[member] = g.Filename(obj)
}
