// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"go/ast"
	"go/types"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

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

// diag produces a nice diagnostic location (document+position) from a Go element.  It should be used for all output
// messages to enable easy correlation with the source IDL artifact that triggered an error.
func (chk *Checker) diag(elem goPos) diag.Diagable {
	return goDiag(chk.Program, elem, chk.Root)
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
			cmdutil.Diag().Errorf(
				diag.Message("%v is an unrecognized Go declaration type: %v").At(chk.diag(obj)),
				objname, reflect.TypeOf(obj))
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
			contract.Assert(!cmdutil.Diag().Success())
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
			contract.Assert(!cmdutil.Diag().Success())
			ok = false
		}
	}

	if !ok {
		contract.Assert(!cmdutil.Diag().Success())
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
			cmdutil.Diag().Errorf(
				diag.Message("enums must be string-backed; %v has type %v").At(chk.diag(decl)),
				c, named,
			)
		}
	} else {
		cmdutil.Diag().Errorf(
			diag.Message("only constants of valid primitive types (bool, float64, number, or aliases) supported").At(
				chk.diag(decl)))
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
				}
				// Otherwise, this is a simple type alias.
				return &Alias{
					member: memb,
					target: s,
				}, true
			}

			cmdutil.Diag().Errorf(diag.Message(
				"type alias %v is not a valid IDL alias type (must be bool, float64, or string)").At(
				chk.diag(decl)))
		case *types.Map, *types.Slice:
			return &Alias{
				member: memb,
				target: s,
			}, true
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
			contract.Assert(!cmdutil.Diag().Success())
		default:
			cmdutil.Diag().Errorf(
				diag.Message("%v is an illegal underlying type: %v").At(chk.diag(decl)), s, reflect.TypeOf(s))
		}
	default:
		cmdutil.Diag().Errorf(
			diag.Message("%v is an illegal Go type kind: %v").At(chk.diag(decl)), t.Name(), reflect.TypeOf(typ))
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
				cmdutil.Diag().Errorf(
					diag.Message("field %v.%v is missing a `lumi:\"<name>\"` tag directive").At(chk.diag(fld)),
					t.Name(), fld.Name())
			}
			if opts.Out && !isres {
				ok = false
				cmdutil.Diag().Errorf(
					diag.Message("field %v.%v is marked `out` but is not a resource property").At(chk.diag(fld)),
					t.Name(), fld.Name())
			}
			if opts.Replaces && !isres {
				ok = false
				cmdutil.Diag().Errorf(
					diag.Message("field %v.%v is marked `replaces` but is not a resource property").At(chk.diag(fld)),
					t.Name(), fld.Name())
			}
			if _, isptr := fld.Type().(*types.Pointer); !isptr && opts.Optional {
				ok = false
				cmdutil.Diag().Errorf(
					diag.Message("field %v.%v is marked `optional` but is not a pointer in the IDL").At(chk.diag(fld)),
					t.Name(), fld.Name())
			}
			if err := chk.CheckIDLType(fld.Type(), opts); err != nil {
				ok = false
				cmdutil.Diag().Errorf(
					diag.Message("field %v.%v is an not a legal IDL type: %v").At(chk.diag(fld)),
					t.Name(), fld.Name(), err)
			}
		}
	}
	return ok, allprops, allopts
}

func (chk *Checker) CheckIDLType(t types.Type, opts PropertyOptions) error {
	// Only these types are legal:
	//     - Primitives: bool, float64, string
	//     - Other structs
	//     - Pointers to any of the above (if-and-only-if an optional property)
	//     - Pointers to other resource types (capabilities)
	//     - Arrays of the above things
	//     - Maps with string keys and any of the above as values
	switch ft := t.(type) {
	case *types.Basic:
		if !IsPrimitive(ft) {
			return errors.Errorf("bad primitive type %v; must be bool, float64, or string", ft)
		}
	case *types.Interface:
		// interface{} is fine and is interpreted as a weakly typed map.
		return nil
	case *types.Named:
		switch ut := ft.Underlying().(type) {
		case *types.Basic:
			// A named type alias of a primitive type.  Ensure it is legal.
			if !IsPrimitive(ut) {
				return errors.Errorf(
					"typedef %v backed by bad primitive type %v; must be bool, float64, or string", ft, ut)
			}
		case *types.Struct:
			// Struct types are okay so long as they aren't entities (these are required to be pointers).
			if isent := IsEntity(ft.Obj(), ut); isent {
				return errors.Errorf("type %v cannot be referenced by-value; must be a pointer", ft)
			}
		default:
			return errors.Errorf("bad named field type: %v", reflect.TypeOf(ut))
		}
	case *types.Pointer:
		// A pointer is OK so long as the field is either optional or an entity type (asset, resource, etc).
		if !opts.Optional && !opts.In && !opts.Out {
			elem := ft.Elem()
			var ok bool
			if named, isnamed := elem.(*types.Named); isnamed {
				ok = IsEntity(named.Obj(), named)
			}
			if !ok {
				return errors.New("bad pointer; must be optional or a resource type")
			}
		}
	case *types.Map:
		// A map is OK so long as its key is a string (or string-backed type) and its element type is okay.
		isstr := false
		switch kt := ft.Key().(type) {
		case *types.Basic:
			isstr = (kt.Kind() == types.String)
		case *types.Named:
			if bt, isbt := kt.Underlying().(*types.Basic); isbt {
				isstr = (bt.Kind() == types.String)
			}
		}
		if !isstr {
			return errors.Errorf("map index type %v must be a string (or string-backed typedef)", ft.Key())
		}
		return chk.CheckIDLType(ft.Elem(), PropertyOptions{})

	case *types.Slice:
		// A slice is OK so long as its element type is also OK.
		return chk.CheckIDLType(ft.Elem(), PropertyOptions{})
	default:
		contract.Failf("Unrecognized field type %v: %v", t, reflect.TypeOf(t))
	}
	return nil
}
