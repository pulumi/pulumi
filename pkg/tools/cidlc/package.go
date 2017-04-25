// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/util/contract"
)

type Package struct {
	Name        string              // the name of the package.
	Program     *loader.Program     // the fully parsed/analyzed Go program.
	Pkginfo     *loader.PackageInfo // the Go package information.
	Files       map[string]*File    // the files inside of this package.
	MemberFiles map[string]*File    // a map from member to the file containing it.
}

func NewPackage(nm string, prog *loader.Program, pkginfo *loader.PackageInfo) *Package {
	return &Package{
		Name:        nm,
		Program:     prog,
		Pkginfo:     pkginfo,
		Files:       make(map[string]*File),
		MemberFiles: make(map[string]*File),
	}
}

func (pkg *Package) AddMember(file *File, key string, m Member) {
	_, has := file.Members[key]
	contract.Assertf(!has, "Unexpected duplicate member %v", key)
	file.Members[key] = m
	file.MemberKeys = append(file.MemberKeys, key)
	pkg.MemberFiles[key] = file
}

type File struct {
	Path       string            // a relative path to the file.
	Node       *ast.File         // the Go file object.
	Members    map[string]Member // a map of all members, membered and internal.
	MemberKeys []string          // the list of member keys in the order in which they were encountered.
}

func NewFile(path string, node *ast.File) *File {
	return &File{
		Path:    path,
		Node:    node,
		Members: make(map[string]Member),
	}
}

type Member interface {
	Name() string   // the name of the member.
	Exported() bool // true if this member is membered.
	Pos() token.Pos // the file defining this member.
}

type member struct {
	name     string
	exported bool
	pos      token.Pos
}

func (m *member) Name() string   { return m.name }
func (m *member) Exported() bool { return m.exported }
func (m *member) Pos() token.Pos { return m.pos }

type TypeMember interface {
	Member
	Struct() *types.Struct
	PropertyOptions() []PropertyOptions
}

type Resource struct {
	member
	Named bool          // true if this is a named resource.
	s     *types.Struct // the underlying Go struct node.
	popts []PropertyOptions
}

func (r *Resource) Struct() *types.Struct              { return r.s }
func (r *Resource) PropertyOptions() []PropertyOptions { return r.popts }

type Struct struct {
	member
	s     *types.Struct
	popts []PropertyOptions
}

func (r *Struct) Struct() *types.Struct              { return r.s }
func (r *Struct) PropertyOptions() []PropertyOptions { return r.popts }

type Alias struct {
	member
	Target *types.Basic
}

type Enum struct {
	member
	Values []string
}

type Const struct {
	member
	Type  types.Type
	Value constant.Value
}
