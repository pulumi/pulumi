// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"bufio"
	"bytes"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// TODO: preserve GoDocs.

type PackGenerator struct {
	Root     string
	Out      string
	Currpkg  *Package             // the package currently being visited.
	Currfile string               // the file currently being visited.
	Fhadres  bool                 // true if the file had at least one resource.
	Ffimp    map[string]string    // a map of foreign packages used in a file.
	Flimp    map[tokens.Name]bool // a map of imported members from modules within this package.
}

func NewPackGenerator(root string, out string) *PackGenerator {
	return &PackGenerator{
		Root: root,
		Out:  out,
	}
}

func (g *PackGenerator) Relpath(s string) (string, error) {
	return filepath.Rel(g.Root, s)
}

// Filename gets the target filename for any given member.
func (g *PackGenerator) Filename(pkg *Package, m Member) (string, error) {
	prog := pkg.Program
	source := prog.Fset.Position(m.Pos()).Filename // the source filename.`
	rel, err := g.Relpath(source)
	if err != nil {
		return "", err
	}
	return filepath.Join(g.Out, rel), nil
}

// Generate generates a Coconut package's source code from a given compiled IDL program.
func (g *PackGenerator) Generate(pkg *Package) error {
	// Ensure the directory structure exists in the target.
	if err := mirrorDirLayout(pkg, g.Out); err != nil {
		return err
	}

	// Install context about the current entity being visited.
	oldpkg, oldfile := g.Currpkg, g.Currfile
	g.Currpkg = pkg
	defer (func() {
		g.Currpkg, g.Currfile = oldpkg, oldfile
	})()

	// Now walk through the package, file by file, and generate the contents.
	for relpath, file := range pkg.Files {
		var members []Member
		for _, nm := range file.MemberNames {
			members = append(members, file.Members[nm])
		}
		g.Currfile = relpath
		path := filepath.Join(g.Out, relpath)
		if err := g.EmitFile(path, members); err != nil {
			return err
		}
	}

	return nil
}

func (g *PackGenerator) EmitFile(file string, members []Member) error {
	// Set up context.
	oldhadres, oldffimp, oldflimp := g.Fhadres, g.Ffimp, g.Flimp
	g.Fhadres, g.Ffimp, g.Flimp = false, make(map[string]string), make(map[tokens.Name]bool)
	defer (func() {
		g.Fhadres = oldhadres
		g.Ffimp = oldffimp
		g.Flimp = oldflimp
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
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	// Emit a header into the file.
	emitHeaderWarning(w)

	// If there are any resources, import the Coconut package.
	if g.Fhadres {
		writefmtln(w, "import * as coconut from \"@coconut/coconut\";")
		writefmtln(w, "")
	}
	if len(g.Flimp) > 0 {
		for local := range g.Flimp {
			// For a local import, make sure to manufacture a correct relative import of the members.
			dir := filepath.Dir(file)
			module := g.Currpkg.MemberFiles[local].Path
			relimp, err := filepath.Rel(dir, filepath.Join(g.Out, module))
			contract.Assert(err == nil)
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
			writefmtln(w, "import {%v} from \"%v\";", local, impname)
		}
		writefmtln(w, "")
	}
	if len(g.Ffimp) > 0 {
		for impname, pkg := range g.Ffimp {
			contract.Failf("Foreign imports not yet supported: import=%v pkg=%v", impname, pkg)
		}
		writefmtln(w, "")
	}

	writefmtln(w, "%v", body)
	return w.Flush()
}

func (g *PackGenerator) genFileBody(members []Member) string {
	// Accumulate the buffer in a string.
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	// Now go ahead and emit the code for all members of this package.
	for i, m := range members {
		if i > 0 {
			// Allow aliases and consts to pile up without line breaks.
			_, isalias := m.(*Alias)
			_, isconst := m.(*Const)
			if (!isalias && !isconst) || reflect.TypeOf(m) != reflect.TypeOf(members[i-1]) {
				writefmtln(w, "")
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

	writefmtln(w, "")
	w.Flush()
	return buffer.String()
}

func (g *PackGenerator) EmitAlias(w *bufio.Writer, alias *Alias) {
	writefmtln(w, "export type %v = %v;", alias.Name(), g.GenTypeName(alias.Target()))
}

func (g *PackGenerator) EmitConst(w *bufio.Writer, konst *Const) {
	writefmtln(w, "export let %v: %v = %v;", konst.Name(), g.GenTypeName(konst.Type), konst.Value.String())
}

func (g *PackGenerator) EmitEnum(w *bufio.Writer, enum *Enum) {
	writefmtln(w, "export type %v =", enum.Name())
	contract.Assert(len(enum.Values) > 0)
	for i, value := range enum.Values {
		if i > 0 {
			writefmtln(w, " |")
		}
		writefmt(w, "    %v", value)
	}
	writefmtln(w, ";")
}

func (g *PackGenerator) EmitResource(w *bufio.Writer, res *Resource) {
	// Emit the full resource class definition, including constructor, etc.
	g.emitResourceClass(w, res)
	writefmtln(w, "")

	// Finally, emit an entire struct type for the args interface.
	g.emitStructType(w, res, res.Name()+tokens.Name("Args"))

	// Remember we had a resource in this file so we can import the right stuff.
	g.Fhadres = true
}

func (g *PackGenerator) emitResourceClass(w *bufio.Writer, res *Resource) {
	// Emit the class definition itself.
	name := res.Name()
	writefmtln(w, "export class %v extends coconut.Resource implements %vArgs {", name, name)

	// Now all fields definitions.
	fn := forEachField(res, func(fld *types.Var, opt PropertyOptions) {
		g.emitField(w, fld, opt, "    public ")
	})
	if fn > 0 {
		writefmtln(w, "")
	}

	// Next, a constructor that validates arguments and self-assigns them.
	writefmtln(w, "    constructor(args: %vArgs) {", name)
	writefmtln(w, "        super();")
	forEachField(res, func(fld *types.Var, opt PropertyOptions) {
		// Skip output properties because they won't exist on the arguments.
		if !opt.Out {
			if !opt.Optional {
				// Validate that required parameters exist.
				writefmtln(w, "        if (args.%v === undefined) {", opt.Name)
				writefmtln(w, "            throw new Error(\"Missing required argument '%v'\");", opt.Name)
				writefmtln(w, "        }")
			}
			writefmtln(w, "        this.%v = args.%v;", opt.Name, opt.Name)
		}
	})
	writefmtln(w, "    }")

	writefmtln(w, "}")
}

func (g *PackGenerator) EmitStruct(w *bufio.Writer, s *Struct) {
	g.emitStructType(w, s, s.Name())
}

func (g *PackGenerator) emitStructType(w *bufio.Writer, t TypeMember, name tokens.Name) {
	writefmtln(w, fmt.Sprintf("export interface %v {", name))
	forEachField(t, func(fld *types.Var, opt PropertyOptions) {
		// Skip output properties, since those exist solely on the resource class.
		if !opt.Out {
			g.emitField(w, fld, opt, "    ")
		}
	})
	writefmtln(w, "}")
}

func (g *PackGenerator) emitField(w *bufio.Writer, fld *types.Var, opt PropertyOptions, prefix string) {
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
	writefmtln(w, "%v%v%v%v: %v;", prefix, readonly, opt.Name, optional, typ)
}

// registerForeign registers that we have seen a foreign package and requests that the imports be emitted for it.
func (g *PackGenerator) registerForeign(pkg *types.Package) string {
	path := pkg.Path()
	if impname, has := g.Ffimp[path]; has {
		return impname
	}

	// If we haven't seen this yet, allocate an import name for it.  For now, just use the package name.
	name := pkg.Name()
	g.Ffimp[path] = name
	return name
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
	case *types.Named:
		// If this came from the same package; the imports will have arranged for it to be available by name.
		obj := u.Obj()
		pkg := obj.Pkg()
		name := obj.Name()
		if pkg == g.Currpkg.Pkginfo.Pkg {
			// If this wasn't in the same file, we still need a relative module import to get the name in scope.
			nm := tokens.Name(name)
			if g.Currpkg.MemberFiles[nm].Path != g.Currfile {
				g.Flimp[nm] = true
			}
			return name
		}

		// Otherwise, we will need to refer to a qualified import name.
		impname := g.registerForeign(pkg)
		return fmt.Sprintf("%v.%v", impname, name)
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
