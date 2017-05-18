// Copyright 2017 Pulumi, Inc. All rights reserved.

package lumidl

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type Package struct {
	Name        tokens.PackageName    // the package name.
	Program     *loader.Program       // the fully parsed/analyzed Go program.
	Pkginfo     *loader.PackageInfo   // the Go package information.
	Files       map[string]*File      // the files inside of this package.
	MemberFiles map[tokens.Name]*File // a map from member to the file containing it.
}

func NewPackage(name tokens.PackageName, prog *loader.Program, pkginfo *loader.PackageInfo) *Package {
	return &Package{
		Name:        name,
		Program:     prog,
		Pkginfo:     pkginfo,
		Files:       make(map[string]*File),
		MemberFiles: make(map[tokens.Name]*File),
	}
}

func (pkg *Package) AddMember(file *File, nm tokens.Name, m Member) {
	_, has := file.Members[nm]
	contract.Assertf(!has, "Unexpected duplicate member %v", nm)
	file.Members[nm] = m
	file.MemberNames = append(file.MemberNames, nm)
	pkg.MemberFiles[nm] = file
}

type File struct {
	Path        string                 // a relative path to the file.
	Node        *ast.File              // the Go file object.
	Members     map[tokens.Name]Member // a map of all members, membered and internal.
	MemberNames []tokens.Name          // the list of member keys in the order in which they were encountered.
}

func NewFile(path string, node *ast.File) *File {
	return &File{
		Path:    path,
		Node:    node,
		Members: make(map[tokens.Name]Member),
	}
}

type Member interface {
	Name() tokens.Name // the name of the member.
	Exported() bool    // true if this member is membered.
	Pos() token.Pos    // the file defining this member.
}

type member struct {
	name     tokens.Name
	exported bool
	pos      token.Pos
}

func (m *member) Name() tokens.Name { return m.name }
func (m *member) Exported() bool    { return m.exported }
func (m *member) Pos() token.Pos    { return m.pos }

type TypeMember interface {
	Member
	Struct() *types.Struct              // the raw underlying struct.
	Properties() []*types.Var           // a flattened list of all properties (including embedded ones).
	PropertyOptions() []PropertyOptions // a flattened list of all property options.
}

type Resource struct {
	member
	Named bool          // true if this is a named resource.
	s     *types.Struct // the underlying Go struct node.
	props []*types.Var
	popts []PropertyOptions
}

func (r *Resource) Struct() *types.Struct              { return r.s }
func (r *Resource) Properties() []*types.Var           { return r.props }
func (r *Resource) PropertyOptions() []PropertyOptions { return r.popts }

type Struct struct {
	member
	s     *types.Struct
	props []*types.Var
	popts []PropertyOptions
}

func (r *Struct) Struct() *types.Struct              { return r.s }
func (r *Struct) Properties() []*types.Var           { return r.props }
func (r *Struct) PropertyOptions() []PropertyOptions { return r.popts }

type Typedef interface {
	Member
	Target() types.Type
}

type Alias struct {
	member
	target types.Type
}

func (a *Alias) Target() types.Type { return a.target }

type Enum struct {
	member
	Values []string
}

func (a *Enum) Target() types.Type { return types.Typ[types.String] }

type Const struct {
	member
	Type  types.Type
	Value constant.Value
}
