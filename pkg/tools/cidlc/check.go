// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
	"github.com/pulumi/coconut/pkg/util/contract"
)

type goPos interface {
	Pos() token.Pos
}

// goDiag produces a diagnostics object out of a Go type artifact.
func goDiag(pos goPos) diag.Diagable {
	// TODO: implement this.
	return nil
}

type Checker struct {
	Root       string
	Program    *loader.Program
	EnumValues map[types.Type][]string
}

func NewChecker(root string, prog *loader.Program) *Checker {
	return &Checker{
		Root:    root,
		Program: prog,
	}
}

// Check analyzes a Go program, ensures that it is valid as an IDL, and classifies all of the types that it
// encounters.  These classifications are returned.  If problems are encountered, diagnostic messages will be output
// and the returned error will be non-nil.
func (chk *Checker) Check(name tokens.PackageName, pkginfo *loader.PackageInfo) (*Package, error) {
	ok := true

	// First just create a list of the constants and types so we can visit them in the right order.  Also maintain a
	// file map so that we can recover the AST information later on (required for import processing, etc).
	var goconsts []*types.Const
	var gotypes []*types.TypeName

	// Enumerate the scope and classify all objects.
	scope := pkginfo.Pkg.Scope()
	for _, objname := range scope.Names() {
		obj := scope.Lookup(objname)
		switch o := obj.(type) {
		case *types.Const:
			goconsts = append(goconsts, o)
		case *types.TypeName:
			gotypes = append(gotypes, o)
		default:
			ok = false
			cmdutil.Sink().Errorf(
				diag.Message("%v is an unrecognized Go declaration type: %v",
					objname, reflect.TypeOf(obj)).At(goDiag(obj)))
		}
	}

	// Start building a package to return.
	pkg := NewPackage(name, chk.Program, pkginfo)
	oldenums := chk.EnumValues
	chk.EnumValues = make(map[types.Type][]string)
	defer (func() { chk.EnumValues = oldenums })()

	getfile := func(path string) *File {
		// If the file exists, fetch it.
		if file, has := pkg.Files[path]; has {
			return file
		}
		// Otherwise, find the AST node, and create a new object.
		for _, fileast := range pkginfo.Files {
			if RelFilename(chk.Root, chk.Program, fileast) == path {
				file := NewFile(path, fileast)
				pkg.Files[path] = file
				return file
			}
		}
		contract.Failf("Missing file AST for path %v", path)
		return nil
	}
	getdecl := func(file *File, obj types.Object) ast.Decl {
		for _, decl := range file.Node.Decls {
			if gdecl, isgdecl := decl.(*ast.GenDecl); isgdecl {
				for _, spec := range gdecl.Specs {
					switch sp := spec.(type) {
					case *ast.ImportSpec:
						// ignore
					case *ast.TypeSpec:
						if sp.Name.Name == obj.Name() {
							return decl
						}
					case *ast.ValueSpec:
						for _, name := range sp.Names {
							if name.Name == obj.Name() {
								return decl
							}
						}
					default:
						contract.Failf("Unrecognized GenDecl Spec type: %v", reflect.TypeOf(sp))
					}
				}
			}
		}
		contract.Failf("Missing object AST decl for %v in %v", obj.Name(), file)
		return nil
	}

	// Now visit all constants so that we can have them handy as we visit enum types.
	for _, goconst := range goconsts {
		path := RelFilename(chk.Root, chk.Program, goconst)
		file := getfile(path)
		decl := getdecl(file, goconst)
		if c, cok := chk.CheckConst(goconst, file, decl); cok {
			nm := tokens.Name(goconst.Name())
			pkg.AddMember(file, nm, c)
		} else {
			ok = false
		}
	}

	// Next, visit all the types.
	for _, gotype := range gotypes {
		path := RelFilename(chk.Root, chk.Program, gotype)
		file := getfile(path)
		decl := getdecl(file, gotype)
		if t, tok := chk.CheckType(gotype, file, decl); tok {
			nm := tokens.Name(gotype.Name())
			pkg.AddMember(file, nm, t)
		} else {
			ok = false
		}
	}

	if !ok {
		contract.Assert(!cmdutil.Sink().Success())
		return nil, errors.New("one or more problems with the input IDL were found; skipping code-generation")
	}

	return pkg, nil
}

func (chk *Checker) CheckConst(c *types.Const, file *File, decl ast.Decl) (*Const, bool) {
	pt := c.Type()
	var t types.Type
	if IsPrimitive(pt) {
		// A primitive, just use it as-is.
		t = pt
	} else if named, isnamed := pt.(*types.Named); isnamed {
		// A constant of a type alias.  This is how IDL enums are defined, so interpret it as such.
		if basic, isbasic := named.Underlying().(*types.Basic); isbasic && basic.Kind() == types.String {
			// Use this type and remember the enum value.
			t = pt
			chk.EnumValues[t] = append(chk.EnumValues[t], c.Val().String())
		} else {
			cmdutil.Sink().Errorf(diag.Message("enums must be string-backed; %v has type %v", c, named))
		}
	} else {
		cmdutil.Sink().Errorf(diag.Message(
			"only constants of valid primitive types (bool, float64, number, or aliases) supported"))
	}

	if t != nil {
		return &Const{
			member: member{
				name:     tokens.Name(c.Name()),
				exported: c.Exported(),
				pos:      c.Pos(),
			},
			Type:  pt,
			Value: c.Val(),
		}, true
	}

	return nil, false
}

func (chk *Checker) CheckType(t *types.TypeName, file *File, decl ast.Decl) (Member, bool) {
	memb := member{
		name:     tokens.Name(t.Name()),
		exported: t.Exported(),
		pos:      t.Pos(),
	}
	switch typ := t.Type().(type) {
	case *types.Named:
		switch s := typ.Underlying().(type) {
		case *types.Basic:
			// A type alias, possibly interpreted as an enum if there are constants.
			if IsPrimitive(s) {
				if vals, isenum := chk.EnumValues[typ]; isenum {
					// There are enum values defined, use them to create an enum type.
					return &Enum{
						member: memb,
						Values: vals,
					}, true
				} else {
					// Otherwise, this is a simple type alias.
					return &Alias{
						member: memb,
						target: s,
					}, true
				}
			}

			cmdutil.Sink().Errorf(diag.Message(
				"type alias %v is not a valid IDL alias type (must be bool, float64, or string)", t.Name()))
		case *types.Struct:
			// A struct definition, possibly a resource.  First, check that all the fields are supported types.
			isres, isnamed := IsResource(t, s)
			if ok, props, opts := chk.CheckStructFields(typ.Obj(), s, isres); ok {
				// If a resource, return additional information.
				if isres {
					return &Resource{
						member: memb,
						Named:  isnamed,
						s:      s,
						props:  props,
						popts:  opts,
					}, true
				}
				// Otherwise, it's a plain old ordinary struct.
				return &Struct{
					member: memb,
					s:      s,
					props:  props,
					popts:  opts,
				}, true
			}
		}
	default:
		cmdutil.Sink().Errorf(diag.Message("%v is an illegal Go type kind: %v", t.Name(), reflect.TypeOf(typ)))
	}
	return nil, false
}

// CheckStructFields ensures that a struct only contains valid "JSON-like" fields
func (chk *Checker) CheckStructFields(t *types.TypeName, s *types.Struct,
	isres bool) (bool, []*types.Var, []PropertyOptions) {
	ok := true
	var allprops []*types.Var
	var allopts []PropertyOptions
	for i := 0; i < s.NumFields(); i++ {
		fld := s.Field(i)
		if fld.Anonymous() {
			// If an embedded structure, validate its fields deeply.
			anon := fld.Type().(*types.Named)
			embedded := anon.Underlying().(*types.Struct)
			isembres, _ := IsResource(anon.Obj(), embedded)
			isok, props, opts := chk.CheckStructFields(anon.Obj(), embedded, isembres)
			if !isok {
				ok = false
			}
			allprops = append(allprops, props...)
			allopts = append(allopts, opts...)
		} else {
			allprops = append(allprops, fld)
			opts := ParsePropertyOptions(s.Tag(i))
			allopts = append(allopts, opts)
			if opts.Name == "" {
				ok = false
				cmdutil.Sink().Errorf(
					diag.Message("field %v.%v is missing a `coco:\"<name>\"` tag directive",
						t.Name(), fld.Name()))
			}
			if opts.Out && !isres {
				ok = false
				cmdutil.Sink().Errorf(
					diag.Message("field %v.%v is marked `out` but is not a resource property",
						t.Name(), fld.Name()))
			}
			if opts.Replaces && !isres {
				ok = false
				cmdutil.Sink().Errorf(
					diag.Message("field %v.%v is marked `replaces` but is not a resource property",
						t.Name(), fld.Name()))
			}
			if _, isptr := fld.Type().(*types.Pointer); !isptr && opts.Optional {
				ok = false
				cmdutil.Sink().Errorf(
					diag.Message("field %v.%v is marked `optional` but is not a pointer in the IDL",
						t.Name(), fld.Name()))
			}

			// Only these types are legal:
			//     - Primitives: bool, float64, string
			//     - Other structs
			//     - Pointers to any of the above (if-and-only-if an optional property)
			//     - Pointers to other resource types (capabilities)
			//     - Arrays of the above things
			//     - Maps with string keys and any of the above as values
			switch ft := fld.Type().(type) {
			case *types.Basic:
				if !IsPrimitive(ft) {
					ok = false
					cmdutil.Sink().Errorf(
						diag.Message("field %v.%v is an illegal primitive type %v; must be bool, float64, or string",
							t.Name(), fld.Name(), ft))
				}
			case *types.Named:
				// TODO: check recursively?
				switch ut := ft.Underlying().(type) {
				case *types.Basic:
					// A named type alias of a primitive type.  Ensure it is legal.
					if !IsPrimitive(ut) {
						ok = false
						cmdutil.Sink().Errorf(
							diag.Message("field %v.%v of type %v is backed by an illegal primitive type %v; "+
								"must be bool, float64, or string", t.Name(), fld.Name(), ft, ut))
					}
				case *types.Struct:
					// OK so long as it's not a resource (these are required to be pointers).
					if isres, _ := IsResource(ft.Obj(), ut); isres {
						ok = false
						cmdutil.Sink().Errorf(
							diag.Message("field %v.%v refers to a resource type %v by-value; field must be a pointer",
								t.Name(), fld.Name(), ft))
					}
				default:
					ok = false
					cmdutil.Sink().Errorf(
						diag.Message("field %v.%v is an illegal named field type: %v",
							t.Name(), fld.Name(), reflect.TypeOf(ut)))
				}
			case *types.Pointer:
				// A pointer is OK so long as the field is either optional or a resource type.
				if !opts.Optional {
					if isres, _ := IsResource(nil, ft.Elem()); !isres {
						ok = false
						cmdutil.Sink().Errorf(
							diag.Message("field %v.%v is an illegal pointer; must be optional or of a resource type",
								t.Name(), fld.Name()))
					}
				}
			default:
				contract.Failf("Unrecognized field type: %v (type=%v typetype=%v)",
					fld.Name(), fld.Type(), reflect.TypeOf(fld.Type()))
			}
		}
	}
	return ok, allprops, allopts
}
