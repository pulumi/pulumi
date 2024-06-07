package codebase

import (
	"bytes"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
)

const Indentation = "    "

const TargetWidth = 120

type Codebase struct {
	Modules map[string]*Module
}

func NewCodebase() Codebase {
	return Codebase{
		Modules: make(map[string]*Module),
	}
}

func (c *Codebase) Module(name string) *Module {
	if m, ok := c.Modules[name]; ok {
		return m
	}

	m := newModule(name)
	c.Modules[name] = m
	return m
}

func (c *Codebase) Instantiate() map[string][]byte {
	files := make(map[string][]byte)
	for _, m := range c.Modules {
		for path, content := range m.Instantiate() {
			files[path] = content
		}
	}

	return files
}

type Modifier int

const (
	Export Modifier = iota
	Public
	Private
	Protected
	Static
	Abstract
	Async
	Readonly
	Optional
	Required
	Output
)

func (m Modifier) WriteSource(b *bytes.Buffer) {
	switch m {
	case Export:
		b.WriteString("export")
	case Public:
		b.WriteString("public")
	case Private:
		b.WriteString("private")
	case Protected:
		b.WriteString("protected")
	case Static:
		b.WriteString("static")
	case Abstract:
		b.WriteString("abstract")
	case Async:
		b.WriteString("async")
	case Readonly:
		b.WriteString("readonly")
	case Output:
		// TODO: comment on this being a pseudo
		b.WriteString("/*out*/")
	case Optional, Required:
		// TODO: comment on ? and !
		return
	}
}

type Modifiers []Modifier

func (ms Modifiers) Has(m Modifier) bool {
	for _, mm := range ms {
		if mm == m {
			return true
		}
	}

	return false
}

func (ms Modifiers) WriteSource(b *bytes.Buffer) {
	for _, m := range ms {
		if m == Optional || m == Required {
			continue
		}

		m.WriteSource(b)
		b.WriteString(" ")
	}
}

type Module struct {
	Name string
	Path string

	Header []string
	Footer []string

	TypeDoc *TypeDoc

	DefaultImports   map[DefaultSymbol]bool
	NamedImports     map[string]map[string]bool
	QualifiedImports map[QualifiedImport]bool

	Namespaces  map[string]*Namespace
	TypeAliases map[string]*TypeAlias
	Interfaces  map[string]*Interface
	Classes     map[string]*Class
	Functions   map[string]*Function
	Constants   map[string]*Constant
}

// name is slashed
func newModule(name string) *Module {
	path := name + ".ts"

	return &Module{
		Name: name,
		Path: path,

		Header: []string{},
		Footer: []string{},

		TypeDoc: nil,

		DefaultImports:   make(map[DefaultSymbol]bool),
		NamedImports:     make(map[string]map[string]bool),
		QualifiedImports: make(map[QualifiedImport]bool),

		Namespaces:  make(map[string]*Namespace),
		TypeAliases: make(map[string]*TypeAlias),
		Interfaces:  make(map[string]*Interface),
		Classes:     make(map[string]*Class),
		Functions:   make(map[string]*Function),
		Constants:   make(map[string]*Constant),
	}
}

func (m *Module) WithHeader(lines ...string) *Module {
	m.Header = lines
	return m
}

func (m *Module) WithFooter(lines ...string) *Module {
	m.Footer = lines
	return m
}

func (m *Module) Documented(d *TypeDoc) *Module {
	m.TypeDoc = d
	return m
}

func (m *Module) Namespace(name string) *Namespace {
	trimmedName := strings.TrimSpace(name)
	parts := strings.Split(trimmedName, ".")

	ns, ok := m.Namespaces[parts[0]]
	if !ok {
		ns = newNamespace(parts[0])
		m.Namespaces[parts[0]] = ns
	}

	for _, part := range parts[1:] {
		ns = ns.Namespace(part)
	}

	return ns
}

func (m *Module) DefaultImport(
	module string,
	defaultName string,
) DefaultSymbol {
	s := DefaultSymbol{Module: module, DefaultName: defaultName}
	if _, ok := m.DefaultImports[s]; ok {
		return s
	}

	m.DefaultImports[s] = true
	return s
}

func (m *Module) NamedImport(module string) *NamedImport {
	return &NamedImport{Parent: m, Module: module}
}

func (m *Module) NamedSymbolImport(s NamedSymbol) NamedSymbol {
	return m.NamedImport(s.Module).MemberAs(s.Name, s.As)
}

func (m *Module) QualifiedImport(module string, qualification string) QualifiedImport {
	i := QualifiedImport{Parent: m, Module: module, Qualification: qualification}
	if _, ok := m.QualifiedImports[i]; ok {
		return i
	}

	m.QualifiedImports[i] = true
	return i
}

func (m *Module) QualifiedSymbolImport(s QualifiedSymbol) QualifiedSymbol {
	return m.QualifiedImport(s.Module, s.Qualification).Member(s.Namespace, s.Name)
}

func (m *Module) TypeAlias(
	modifiers []Modifier,
	name string,
	aliased Type,
) *TypeAlias {
	if ta, ok := m.TypeAliases[name]; ok {
		return ta
	}

	ta := newTypeAlias(modifiers, name, aliased)
	m.TypeAliases[name] = ta
	return ta
}

func (m *Module) Interface(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Interface {
	if i, ok := m.Interfaces[name]; ok {
		return i
	}

	i := newInterface(modifiers, name, parameters)
	m.Interfaces[name] = i
	return i
}

func (m *Module) Class(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Class {
	if c, ok := m.Classes[name]; ok {
		return c
	}

	c := newClass(modifiers, name, parameters)
	m.Classes[name] = c
	return c
}

func (m *Module) Function(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
	body []Statement,
) *Function {
	if f, ok := m.Functions[name]; ok {
		return f
	}

	f := newFunction(modifiers, name, args, returnType, body)
	m.Functions[name] = f
	return f
}

func (m *Module) Constant(
	modifiers []Modifier,
	name string,
	t Type,
	e Expression,
) *Constant {
	if c, ok := m.Constants[name]; ok {
		return c
	}

	c := newConstant(modifiers, name, t, e)
	m.Constants[name] = c
	return c
}

func (c *Module) WriteBlockSource(b *bytes.Buffer, indent string) {
	hasContents := false

	if len(c.Header) > 0 {
		hasContents = true

		for _, line := range c.Header {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if c.TypeDoc != nil {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		c.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n\n")
	}

	defaultImports := maps.Keys(c.DefaultImports)
	if len(defaultImports) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(defaultImports, func(a, b DefaultSymbol) int {
			if a.Module < b.Module {
				return -1
			} else if a.Module > b.Module {
				return 1
			} else {
				return 0
			}
		})

		for _, s := range defaultImports {
			b.WriteString(fmt.Sprintf("import %s from \"%s\";\n", s.DefaultName, s.Module))
		}
	}

	namedImportModules := maps.Keys(c.NamedImports)
	if len(namedImportModules) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.Sort(namedImportModules)
		for _, namedImportModule := range namedImportModules {
			if namedImportModule == c.Name {
				continue
			}

			namedImports := maps.Keys(c.NamedImports[namedImportModule])
			if len(namedImports) == 0 {
				continue
			}

			slices.Sort(namedImports)

			tooLong := func() bool {
				total := 0
				for _, namedImport := range namedImports {
					total += len(namedImport)
					if total > TargetWidth {
						return true
					}
				}

				return false
			}

			if tooLong() {
				b.WriteString("import {\n")
				for _, namedImport := range namedImports {
					b.WriteString(Indentation)
					b.WriteString(namedImport)
					b.WriteString(",\n")
				}
				b.WriteString("} from \"")
				b.WriteString(namedImportModule)
				b.WriteString("\";\n")
			} else {
				b.WriteString("import { ")
				for i, namedImport := range namedImports {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(namedImport)
				}
				b.WriteString(" } from \"")
				b.WriteString(namedImportModule)
				b.WriteString("\";\n")
			}
		}
	}

	qualifiedImports := maps.Keys(c.QualifiedImports)
	if len(qualifiedImports) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(qualifiedImports, func(a, b QualifiedImport) int {
			if a.Module < b.Module {
				return -1
			} else if a.Module > b.Module {
				return 1
			} else {
				return 0
			}
		})

		for _, s := range qualifiedImports {
			// Imports from this module -- we can elide them
			if s.Module == "" || s.Qualification == "" {
				continue
			}

			b.WriteString("import * as ")
			b.WriteString(s.Qualification)
			b.WriteString(" from \"")
			b.WriteString(s.Module)
			b.WriteString("\";\n")
		}
	}

	namespaces := maps.Values(c.Namespaces)
	if len(namespaces) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(namespaces, func(a, b *Namespace) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, ns := range namespaces {
			ns.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if i < len(namespaces)-1 {
				b.WriteString("\n")
			}
		}
	}

	typeAliases := maps.Values(c.TypeAliases)
	if len(typeAliases) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(typeAliases, func(a, b *TypeAlias) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, ta := range typeAliases {
			ta.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if i < len(typeAliases)-1 {
				b.WriteString("\n")
			}
		}
	}

	interfaces := maps.Values(c.Interfaces)
	if len(interfaces) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(interfaces, func(a, b *Interface) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for ix, i := range interfaces {
			i.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if ix < len(interfaces)-1 {
				b.WriteString("\n")
			}
		}
	}

	classes := maps.Values(c.Classes)
	if len(classes) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(classes, func(a, b *Class) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, c := range classes {
			c.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if i < len(classes)-1 {
				b.WriteString("\n")
			}
		}
	}

	functions := maps.Values(c.Functions)
	if len(functions) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(functions, func(a, b *Function) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, f := range functions {
			f.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if i < len(functions)-1 {
				b.WriteString("\n")
			}
		}
	}

	constants := maps.Values(c.Constants)
	if len(constants) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(constants, func(a, b *Constant) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, c := range constants {
			c.WriteBlockSource(b, indent)
			b.WriteString("\n")

			if i < len(constants)-1 {
				b.WriteString("\n")
			}
		}
	}
}

func (m *Module) Instantiate() map[string][]byte {
	b := new(bytes.Buffer)
	m.WriteBlockSource(b, "")

	files := make(map[string][]byte)
	files[m.Path] = b.Bytes()

	return files
}

type NamedImport struct {
	Parent *Module
	Module string
}

func (n *NamedImport) Member(name string) NamedSymbol {
	return n.MemberAs(name, "")
}

func (n *NamedImport) MemberAs(name string, as string) NamedSymbol {
	// TODO comments
	// Defined in this module, rewrite the import to be the definitional name
	if n.Module == n.Parent.Name {
		return NamedSymbol{
			Module: n.Module,
			Name:   name,
			As:     "",
		}
	}

	if n.Parent.NamedImports[n.Module] == nil {
		n.Parent.NamedImports[n.Module] = make(map[string]bool)
	}

	n.Parent.NamedImports[n.Module][name] = true

	return NamedSymbol{
		Module: n.Module,
		Name:   name,
		As:     as,
	}
}

type QualifiedImport struct {
	Parent        *Module
	Module        string
	Qualification string
}

func (q QualifiedImport) Member(namespace string, name string) QualifiedSymbol {
	if q.Module == q.Parent.Name {
		return QualifiedSymbol{
			Module:        q.Module,
			Qualification: "",
			Namespace:     namespace,
			Name:          name,
		}
	}

	return QualifiedSymbol{
		Module:        q.Module,
		Qualification: q.Qualification,
		Namespace:     namespace,
		Name:          name,
	}
}

type Namespace struct {
	Name string

	Namespaces  map[string]*Namespace
	TypeAliases map[string]*TypeAlias
	Interfaces  map[string]*Interface
	Classes     map[string]*Class
	Functions   map[string]*Function
	Constants   map[string]*Constant
}

func newNamespace(name string) *Namespace {
	return &Namespace{
		Name: name,

		Namespaces:  make(map[string]*Namespace),
		TypeAliases: make(map[string]*TypeAlias),
		Interfaces:  make(map[string]*Interface),
		Classes:     make(map[string]*Class),
		Functions:   make(map[string]*Function),
		Constants:   make(map[string]*Constant),
	}
}

func (n *Namespace) Namespace(name string) *Namespace {
	trimmedName := strings.TrimSpace(name)
	parts := strings.Split(trimmedName, ".")

	ns, ok := n.Namespaces[parts[0]]
	if !ok {
		ns = newNamespace(parts[0])
		n.Namespaces[parts[0]] = ns
	}

	for _, part := range parts[1:] {
		ns = ns.Namespace(part)
	}

	return ns
}

func (n *Namespace) TypeAlias(
	modifiers []Modifier,
	name string,
	aliased Type,
) *TypeAlias {
	if ta, ok := n.TypeAliases[name]; ok {
		return ta
	}

	ta := newTypeAlias(modifiers, name, aliased)
	n.TypeAliases[name] = ta
	return ta
}

func (n *Namespace) Interface(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Interface {
	if i, ok := n.Interfaces[name]; ok {
		return i
	}

	i := newInterface(modifiers, name, parameters)
	n.Interfaces[name] = i
	return i
}

func (n *Namespace) Class(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Class {
	if c, ok := n.Classes[name]; ok {
		return c
	}

	c := newClass(modifiers, name, parameters)
	n.Classes[name] = c
	return c
}

func (n *Namespace) Function(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
	body []Statement,
) *Function {
	if f, ok := n.Functions[name]; ok {
		return f
	}

	f := newFunction(modifiers, name, args, returnType, body)
	n.Functions[name] = f
	return f
}

func (n *Namespace) Constant(
	modifiers []Modifier,
	name string,
	t Type,
	e Expression,
) *Constant {
	if c, ok := n.Constants[name]; ok {
		return c
	}

	c := newConstant(modifiers, name, t, e)
	n.Constants[name] = c
	return c
}

func (n *Namespace) WriteBlockSource(b *bytes.Buffer, indent string) {
	hasContents := false

	b.WriteString("namespace ")
	b.WriteString(n.Name)
	b.WriteString(" {\n")

	namespaces := maps.Values(n.Namespaces)
	if len(namespaces) > 0 {
		hasContents = true

		slices.SortFunc(namespaces, func(a, b *Namespace) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, ns := range namespaces {
			b.WriteString(indent)
			b.WriteString(Indentation)

			ns.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if i < len(namespaces)-1 {
				b.WriteString("\n")
			}
		}
	}

	typeAliases := maps.Values(n.TypeAliases)
	if len(typeAliases) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(typeAliases, func(a, b *TypeAlias) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, ta := range typeAliases {
			b.WriteString(indent)
			b.WriteString(Indentation)

			ta.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if i < len(typeAliases)-1 {
				b.WriteString("\n")
			}
		}
	}

	interfaces := maps.Values(n.Interfaces)
	if len(interfaces) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(interfaces, func(a, b *Interface) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for ix, i := range interfaces {
			b.WriteString(indent)
			b.WriteString(Indentation)

			i.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if ix < len(interfaces)-1 {
				b.WriteString("\n")
			}
		}
	}

	classes := maps.Values(n.Classes)
	if len(classes) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(classes, func(a, b *Class) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, c := range classes {
			b.WriteString(indent)
			b.WriteString(Indentation)

			c.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if i < len(classes)-1 {
				b.WriteString("\n")
			}
		}
	}

	functions := maps.Values(n.Functions)
	if len(functions) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(functions, func(a, b *Function) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, f := range functions {
			b.WriteString(indent)
			b.WriteString(Indentation)

			f.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if i < len(functions)-1 {
				b.WriteString("\n")
			}
		}
	}

	constants := maps.Values(n.Constants)
	if len(constants) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(constants, func(a, b *Constant) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, c := range constants {
			b.WriteString(indent)
			b.WriteString(Indentation)

			c.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")

			if i < len(constants)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString(indent)
	b.WriteString("}")
}

type TypeAlias struct {
	TypeDoc   *TypeDoc
	Modifiers []Modifier
	Name      string
	Aliased   Type
}

func newTypeAlias(
	modifiers []Modifier,
	name string,
	aliased Type,
) *TypeAlias {
	return &TypeAlias{
		TypeDoc:   nil,
		Modifiers: modifiers,
		Name:      name,
		Aliased:   aliased,
	}
}

func (ta *TypeAlias) Documented(d *TypeDoc) *TypeAlias {
	ta.TypeDoc = d
	return ta
}

func (ta *TypeAlias) WriteBlockSource(b *bytes.Buffer, indent string) {
	if ta.TypeDoc != nil {
		ta.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(ta.Modifiers).WriteSource(b)

	b.WriteString("type ")
	b.WriteString(ta.Name)
	b.WriteString(" = ")
	ta.Aliased.WriteBlockSource(b, indent)
}

type Interface struct {
	TypeDoc    *TypeDoc
	Modifiers  []Modifier
	Name       string
	Parameters []TypeParameter
	Extended   []Type
	Properties map[string]*InterfaceProperty
	Methods    map[string]*InterfaceMethod
}

func newInterface(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Interface {
	return &Interface{
		TypeDoc:    nil,
		Modifiers:  modifiers,
		Name:       name,
		Parameters: parameters,
		Properties: make(map[string]*InterfaceProperty),
		Methods:    make(map[string]*InterfaceMethod),
	}
}

func (i *Interface) Documented(d *TypeDoc) *Interface {
	i.TypeDoc = d
	return i
}

func (i *Interface) Extends(t Type) *Interface {
	i.Extended = append(i.Extended, t)
	return i
}

func (i *Interface) Property(
	modifiers []Modifier,
	name string,
	t Type,
) *InterfaceProperty {
	if p, ok := i.Properties[name]; ok {
		return p
	}

	p := newInterfaceProperty(modifiers, name, t)
	i.Properties[name] = p
	return p
}

func (i *Interface) Method(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
) *InterfaceMethod {
	if m, ok := i.Methods[name]; ok {
		return m
	}

	m := newInterfaceMethod(modifiers, name, args, returnType)
	i.Methods[name] = m
	return m
}

func (i *Interface) AsExpression() Expression {
	return literalE(i.Name)
}

func (i *Interface) AsType() Type {
	return namedT(i.Name)
}

func (i *Interface) WriteBlockSource(b *bytes.Buffer, indent string) {
	if i.TypeDoc != nil {
		i.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(i.Modifiers).WriteSource(b)

	b.WriteString("interface ")
	b.WriteString(i.Name)

	TypeParameters(i.Parameters).WriteBlockSource(b, indent)
	Extended(i.Extended).WriteBlockSource(b, indent)

	b.WriteString(" {\n")

	hasContents := false

	properties := maps.Values(i.Properties)
	if len(properties) > 0 {
		hasContents = true

		slices.SortFunc(properties, func(a, b *InterfaceProperty) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, p := range properties {
			if i > 0 {
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString(Indentation)

			p.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")
		}
	}

	methods := maps.Values(i.Methods)
	if len(methods) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(methods, func(a, b *InterfaceMethod) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, m := range methods {
			if i > 0 {
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString(Indentation)

			m.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")
		}
	}

	b.WriteString(indent)
	b.WriteString("}")
}

type Class struct {
	TypeDoc     *TypeDoc
	Modifiers   []Modifier
	Name        string
	Parameters  []TypeParameter
	Extended    []Type
	Implemented []Type
	Properties  map[string]*ClassProperty
	Methods     map[string]*ClassMethod
}

func newClass(
	modifiers []Modifier,
	name string,
	parameters []TypeParameter,
) *Class {
	return &Class{
		TypeDoc:     nil,
		Modifiers:   modifiers,
		Name:        name,
		Parameters:  parameters,
		Extended:    make([]Type, 0),
		Implemented: make([]Type, 0),
		Properties:  make(map[string]*ClassProperty),
		Methods:     make(map[string]*ClassMethod),
	}
}

func (c *Class) Documented(d *TypeDoc) *Class {
	c.TypeDoc = d
	return c
}

func (c *Class) Extends(t Type) *Class {
	c.Extended = append(c.Extended, t)
	return c
}

func (c *Class) Implements(t Type) *Class {
	c.Implemented = append(c.Implemented, t)
	return c
}

func (c *Class) Property(
	modifiers []Modifier,
	name string,
	t Type,
) *ClassProperty {
	if p, ok := c.Properties[name]; ok {
		return p
	}

	p := newClassProperty(modifiers, name, t)
	c.Properties[name] = p
	return p
}

func (c *Class) Method(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
	body []Statement,
) *ClassMethod {
	if m, ok := c.Methods[name]; ok {
		return m
	}

	m := newClassMethod(modifiers, name, args, returnType, body)
	c.Methods[name] = m
	return m
}

func (c *Class) AsExpression() Expression {
	return literalE(c.Name)
}

func (c *Class) AsType() Type {
	return namedT(c.Name)
}

func (c *Class) WriteBlockSource(b *bytes.Buffer, indent string) {
	if c.TypeDoc != nil {
		c.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(c.Modifiers).WriteSource(b)

	b.WriteString("class ")
	b.WriteString(c.Name)

	TypeParameters(c.Parameters).WriteBlockSource(b, indent)
	Extended(c.Extended).WriteBlockSource(b, indent)
	Implemented(c.Implemented).WriteBlockSource(b, indent)

	b.WriteString(" {\n")

	hasContents := false

	properties := maps.Values(c.Properties)
	if len(properties) > 0 {
		hasContents = true

		slices.SortFunc(properties, func(a, b *ClassProperty) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, p := range properties {
			if i > 0 {
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString(Indentation)

			p.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")
		}
	}

	methods := maps.Values(c.Methods)
	if len(methods) > 0 {
		if hasContents {
			b.WriteString("\n")
		}
		hasContents = true

		slices.SortFunc(methods, func(a, b *ClassMethod) int {
			if a.Name < b.Name {
				return -1
			} else if a.Name > b.Name {
				return 1
			} else {
				return 0
			}
		})

		for i, m := range methods {
			if i > 0 {
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString(Indentation)

			m.WriteBlockSource(b, indent+Indentation)
			b.WriteString("\n")
		}
	}

	b.WriteString(indent)
	b.WriteString("}")
}

type TypeParameter struct {
	Name       string
	Constraint *Type
}

func TP(name string) TypeParameter {
	return TypeParameter{Name: name, Constraint: nil}
}

func (tp TypeParameter) Extends(t Type) TypeParameter {
	tp.Constraint = &t
	return tp
}

func (tp TypeParameter) WriteBlockSource(b *bytes.Buffer, indent string) {
	b.WriteString(tp.Name)
	if tp.Constraint != nil {
		tooLong := func() bool {
			b := new(bytes.Buffer)
			tp.Constraint.WriteInlineSource(b)

			return b.Len() > TargetWidth
		}

		if tooLong() {
			b.WriteString(" extends\n")
			b.WriteString(indent)
			b.WriteString(Indentation)
			tp.Constraint.WriteBlockSource(b, indent+Indentation)
		} else {
			b.WriteString(" extends ")
			tp.Constraint.WriteInlineSource(b)
		}
	}
}

type TypeParameters []TypeParameter

func (tps TypeParameters) WriteBlockSource(b *bytes.Buffer, indent string) {
	if len(tps) > 0 {
		tooLong := func() bool {
			b := new(bytes.Buffer)
			for _, tp := range tps {
				tp.WriteBlockSource(b, "")
			}

			return b.Len() > TargetWidth
		}

		if tooLong() {
			b.WriteString("<\n")
			for i, tp := range tps {
				b.WriteString(indent)
				b.WriteString(Indentation)
				tp.WriteBlockSource(b, indent+Indentation)
				if i < len(tps)-1 {
					b.WriteString(",")
				}

				b.WriteString("\n")
			}
			b.WriteString(">")
		} else {
			b.WriteString("<")
			for i, tp := range tps {
				if i > 0 {
					b.WriteString(", ")
				}

				tp.WriteBlockSource(b, indent)
			}

			b.WriteString(">")
		}
	}
}

type Extended []Type

func (es Extended) WriteBlockSource(b *bytes.Buffer, indent string) {
	if len(es) > 0 {
		tooLong := func() bool {
			b := new(bytes.Buffer)
			for _, e := range es {
				e.WriteInlineSource(b)
			}

			return b.Len() > TargetWidth
		}

		if tooLong() {
			b.WriteString(" extends\n")
			for i, e := range es {
				b.WriteString(indent)
				b.WriteString(Indentation)
				e.WriteBlockSource(b, indent+Indentation)
				if i < len(es)-1 {
					b.WriteString(",")
				}

				b.WriteString("\n")
			}
		} else {
			b.WriteString(" extends ")
			for i, e := range es {
				if i > 0 {
					b.WriteString(", ")
				}

				e.WriteInlineSource(b)
			}
		}
	}
}

type Implemented []Type

func (is Implemented) WriteBlockSource(b *bytes.Buffer, indent string) {
	if len(is) > 0 {
		tooLong := func() bool {
			b := new(bytes.Buffer)
			for _, i := range is {
				i.WriteInlineSource(b)
			}

			return b.Len() > TargetWidth
		}

		if tooLong() {
			b.WriteString(" implements\n")
			for ix, i := range is {
				b.WriteString(indent)
				b.WriteString(Indentation)
				i.WriteBlockSource(b, indent+Indentation)
				if ix < len(is)-1 {
					b.WriteString(",")
				}

				b.WriteString("\n")
			}
		} else {
			b.WriteString(" implements ")
			for ix, i := range is {
				if ix > 0 {
					b.WriteString(", ")
				}

				i.WriteInlineSource(b)
			}
		}
	}
}

type InterfaceProperty struct {
	TypeDoc   *TypeDoc
	Modifiers []Modifier
	Name      string
	Type      Type
}

func newInterfaceProperty(
	modifiers []Modifier,
	name string,
	t Type,
) *InterfaceProperty {
	return &InterfaceProperty{
		TypeDoc:   nil,
		Modifiers: modifiers,
		Name:      name,
		Type:      t,
	}
}

func (ip *InterfaceProperty) Documented(d *TypeDoc) *InterfaceProperty {
	ip.TypeDoc = d
	return ip
}

func (ip *InterfaceProperty) WriteBlockSource(b *bytes.Buffer, indent string) {
	if ip.TypeDoc != nil {
		ip.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	modifiers := Modifiers(ip.Modifiers)

	modifiers.WriteSource(b)
	b.WriteString(ip.Name)
	if modifiers.Has(Optional) {
		b.WriteString("?")
	}

	if modifiers.Has(Required) {
		b.WriteString("!")
	}

	b.WriteString(": ")
	ip.Type.WriteSource(b, indent)
	b.WriteString(";")
}

type InterfaceMethod struct {
	TypeDoc    *TypeDoc
	Modifiers  []Modifier
	Name       string
	Args       []Argument
	ReturnType Type
}

func newInterfaceMethod(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
) *InterfaceMethod {
	return &InterfaceMethod{
		TypeDoc:    nil,
		Modifiers:  modifiers,
		Name:       name,
		Args:       args,
		ReturnType: returnType,
	}
}

func (im *InterfaceMethod) Documented(d *TypeDoc) *InterfaceMethod {
	im.TypeDoc = d
	return im
}

func (im *InterfaceMethod) WriteBlockSource(b *bytes.Buffer, indent string) {
	if im.TypeDoc != nil {
		im.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(im.Modifiers).WriteSource(b)

	b.WriteString(im.Name)
	b.WriteString("(")

	Arguments(im.Args).WriteBlockSource(b, indent)

	b.WriteString("): ")
	im.ReturnType.WriteSource(b, indent)
}

type ClassProperty struct {
	TypeDoc     *TypeDoc
	Modifiers   []Modifier
	Name        string
	Type        Type
	Initializer *Expression
}

func newClassProperty(
	modifiers []Modifier,
	name string,
	t Type,
) *ClassProperty {
	return &ClassProperty{
		TypeDoc:     nil,
		Modifiers:   modifiers,
		Name:        name,
		Type:        t,
		Initializer: nil,
	}
}

func (cp *ClassProperty) Initialized(e Expression) *ClassProperty {
	cp.Initializer = &e
	return cp
}

func (cp *ClassProperty) Documented(d *TypeDoc) *ClassProperty {
	cp.TypeDoc = d
	return cp
}

func (cp *ClassProperty) WriteBlockSource(b *bytes.Buffer, indent string) {
	if cp.TypeDoc != nil {
		cp.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	modifiers := Modifiers(cp.Modifiers)

	modifiers.WriteSource(b)
	b.WriteString(cp.Name)
	if modifiers.Has(Optional) {
		b.WriteString("?")
	}

	if modifiers.Has(Required) {
		b.WriteString("!")
	}

	b.WriteString(": ")
	cp.Type.WriteSource(b, indent)

	if cp.Initializer != nil {
		b.WriteString(" = ")
		cp.Initializer.WriteSource(b, indent)
	}

	b.WriteString(";")
}

type ClassMethod struct {
	TypeDoc    *TypeDoc
	Name       string
	Modifiers  []Modifier
	Args       []Argument
	ReturnType Type
	Body       []Statement
}

func newClassMethod(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
	body []Statement,
) *ClassMethod {
	return &ClassMethod{
		TypeDoc:    nil,
		Modifiers:  modifiers,
		Name:       name,
		Args:       args,
		ReturnType: returnType,
		Body:       body,
	}
}

func (cm *ClassMethod) Documented(d *TypeDoc) *ClassMethod {
	cm.TypeDoc = d
	return cm
}

func (cm *ClassMethod) WriteBlockSource(b *bytes.Buffer, indent string) {
	if cm.TypeDoc != nil {
		cm.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(cm.Modifiers).WriteSource(b)

	b.WriteString(cm.Name)
	b.WriteString("(")

	Arguments(cm.Args).WriteBlockSource(b, indent)

	b.WriteString("): ")
	cm.ReturnType.WriteSource(b, indent)
	b.WriteString(" {\n")

	for _, s := range cm.Body {
		b.WriteString(indent)
		b.WriteString(Indentation)
		s.WriteSource(b, indent+Indentation)
		b.WriteString("\n")
	}

	b.WriteString(indent)
	b.WriteString("}")
}

type Function struct {
	TypeDoc    *TypeDoc
	Modifiers  []Modifier
	Name       string
	Args       []Argument
	ReturnType Type
	Body       []Statement
}

func newFunction(
	modifiers []Modifier,
	name string,
	args []Argument,
	returnType Type,
	body []Statement,
) *Function {
	return &Function{
		TypeDoc:    nil,
		Modifiers:  modifiers,
		Name:       name,
		Args:       args,
		ReturnType: returnType,
		Body:       body,
	}
}

func (f *Function) WriteBlockSource(b *bytes.Buffer, indent string) {
	if f.TypeDoc != nil {
		f.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(f.Modifiers).WriteSource(b)

	b.WriteString("function ")
	b.WriteString(f.Name)
	b.WriteString("(")

	Arguments(f.Args).WriteBlockSource(b, indent)

	b.WriteString(") {\n")

	for _, s := range f.Body {
		b.WriteString(indent)
		b.WriteString(Indentation)
		s.WriteSource(b, indent+Indentation)
		b.WriteString("\n")
	}

	b.WriteString(indent)
	b.WriteString("}")
}

type Constant struct {
	TypeDoc    *TypeDoc
	Modifiers  []Modifier
	Name       string
	Type       Type
	Expression Expression
}

func newConstant(
	modifiers []Modifier,
	name string,
	t Type,
	e Expression,
) *Constant {
	return &Constant{
		TypeDoc:    nil,
		Modifiers:  modifiers,
		Name:       name,
		Type:       t,
		Expression: e,
	}
}

func (c *Constant) WriteBlockSource(b *bytes.Buffer, indent string) {
	if c.TypeDoc != nil {
		c.TypeDoc.WriteBlockSource(b, indent)
		b.WriteString("\n")
		b.WriteString(indent)
	}

	Modifiers(c.Modifiers).WriteSource(b)

	b.WriteString("const ")
	b.WriteString(c.Name)
	b.WriteString(": ")
	c.Type.WriteSource(b, indent)
	b.WriteString(" = ")
	c.Expression.WriteSource(b, indent)
	b.WriteString(";")
}

type Argument struct {
	Modifiers []Modifier
	Name      string
	Type      Type
}

func (a *Argument) WriteBlockSource(b *bytes.Buffer, indent string) {
	modifiers := Modifiers(a.Modifiers)
	modifiers.WriteSource(b)

	b.WriteString(a.Name)
	if modifiers.Has(Optional) {
		b.WriteString("?")
	}

	b.WriteString(": ")
	a.Type.WriteSource(b, indent)
}

type Arguments []Argument

func (args Arguments) WriteBlockSource(b *bytes.Buffer, indent string) {
	if len(args) > 0 {
		tooLong := func() bool {
			b := new(bytes.Buffer)
			for _, arg := range args {
				arg.WriteBlockSource(b, "")
			}

			return b.Len() > TargetWidth
		}

		if tooLong() {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(Indentation)

			for i, arg := range args {
				if i > 0 {
					b.WriteString(",\n")
					b.WriteString(indent)
					b.WriteString(Indentation)
				}

				arg.WriteBlockSource(b, indent+Indentation)
			}

			b.WriteString("\n")
			b.WriteString(indent)
		} else {
			for i, arg := range args {
				if i > 0 {
					b.WriteString(", ")
				}

				arg.WriteBlockSource(b, indent)
			}
		}
	}
}

type Type struct {
	WriteBlockSource  func(b *bytes.Buffer, indent string)
	WriteInlineSource func(b *bytes.Buffer)
}

func (t Type) WriteSource(b *bytes.Buffer, indent string) {
	var l bytes.Buffer
	t.WriteInlineSource(&l)
	if l.Len() > TargetWidth {
		t.WriteBlockSource(b, indent)
	} else {
		t.WriteInlineSource(b)
	}
}

func (t Type) Apply(ts ...Type) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			t.WriteSource(b, indent)
			b.WriteString("<\n")

			for i, t := range ts {
				b.WriteString(indent)
				b.WriteString(Indentation)
				t.WriteSource(b, indent+Indentation)

				if i < len(ts)-1 {
					b.WriteString(",")
				}

				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString(">")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			t.WriteInlineSource(b)
			b.WriteString("<")

			for i, t := range ts {
				if i > 0 {
					b.WriteString(", ")
				}

				t.WriteInlineSource(b)
			}

			b.WriteString(">")
		},
	}
}

func namedT(name string) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(name)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString(name)
		},
	}
}

var AnyT Type = namedT("any")

var VoidT Type = namedT("void")

var NeverT Type = namedT("never")

var UnknownT Type = namedT("unknown")

var UndefinedT Type = namedT("undefined")

var NullT Type = namedT("null")

var BooleanT Type = namedT("boolean")

var NumberT Type = namedT("number")

var StringT Type = namedT("string")

func GuardT(name string, t Type) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(name)
			b.WriteString(" is ")
			t.WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString(name)
			b.WriteString(" is ")
			t.WriteInlineSource(b)
		},
	}
}

func ArrayT(element Type) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			element.WriteSource(b, indent)
			b.WriteString("[]")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			element.WriteInlineSource(b)
			b.WriteString("[]")
		},
	}
}

func ObjectT(properties map[string]Type) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			if len(properties) == 0 {
				b.WriteString("{}")
				return
			}

			b.WriteString("{\n")
			for name, p := range properties {
				b.WriteString(indent)
				b.WriteString(Indentation)
				b.WriteString(name)
				b.WriteString(": ")
				p.WriteSource(b, indent+Indentation)
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString("}")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString("{ ")
			for i, name := range maps.Keys(properties) {
				if i > 0 {
					b.WriteString("; ")
				}

				b.WriteString(name)
				b.WriteString(": ")
				properties[name].WriteInlineSource(b)
			}

			b.WriteString(" }")
		},
	}
}

func UnionT(ts ...Type) Type {
	return Type{
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			for i, t := range ts {
				b.WriteString("| ")
				t.WriteSource(b, indent)

				if i < len(ts)-1 {
					b.WriteString("\n")
					b.WriteString(indent)
				}
			}
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			for i, t := range ts {
				if i > 0 {
					b.WriteString(" | ")
				}

				t.WriteInlineSource(b)
			}
		},
	}
}

func NullableT(t Type) Type {
	return UnionT(t, NullT)
}

type Statement struct {
	WriteSource func(b *bytes.Buffer, indent string)
}

func ExprS(e Expression) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			e.WriteSource(b, indent)
			b.WriteString(";")
		},
	}
}

func CommentS(comment string) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("//")
			b.WriteString(comment)
		},
	}
}

func declS(
	decl string,
	name string,
	t *Type,
	e Expression,
) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(decl)
			b.WriteString(" ")
			b.WriteString(name)

			if t != nil {
				b.WriteString(": ")
				t.WriteSource(b, indent+Indentation)
			}

			b.WriteString(" = ")
			e.WriteSource(b, indent)
			b.WriteString(";")
		},
	}
}

func ConstS(name string, e Expression) Statement {
	return declS("const", name, nil, e)
}

func ConstTS(name string, t Type, e Expression) Statement {
	return declS("const", name, &t, e)
}

func LetS(name string, e Expression) Statement {
	return declS("let", name, nil, e)
}

func LetTS(name string, t Type, e Expression) Statement {
	return declS("let", name, &t, e)
}

func IfS(
	condition Expression,
	then []Statement,
	else_ []Statement,
) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("if (")
			condition.WriteSource(b, indent+Indentation)
			b.WriteString(") {\n")

			for _, s := range then {
				b.WriteString(indent)
				b.WriteString(Indentation)
				s.WriteSource(b, indent+Indentation)
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString("}")

			if len(else_) > 0 {
				b.WriteString(" else {\n")

				for _, s := range else_ {
					b.WriteString(indent)
					b.WriteString(Indentation)
					s.WriteSource(b, indent+Indentation)
					b.WriteString("\n")
				}

				b.WriteString(indent)
				b.WriteString("}")
			}
		},
	}
}

func ForS(
	initializer []Statement,
	condition Expression,
	update []Statement,
	body []Statement,
) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("for (")

			if len(initializer) > 0 {
				for i, s := range initializer {
					if i > 0 {
						b.WriteString(", ")
					}

					s.WriteSource(b, indent)
				}
			}

			b.WriteString("; ")
			condition.WriteInlineSource(b)
			b.WriteString(";")

			if len(update) > 0 {
				b.WriteString(" ")
				for i, s := range update {
					if i > 0 {
						b.WriteString(", ")
					}

					s.WriteSource(b, indent)
				}
			}

			b.WriteString(") {\n")

			for _, s := range body {
				b.WriteString(indent)
				b.WriteString(Indentation)
				s.WriteSource(b, indent+Indentation)
				b.WriteString("\n")
			}

			b.WriteString(indent)
			b.WriteString("}")
		},
	}
}

func ReturnS(e Expression) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("return ")
			e.WriteSource(b, indent)
			b.WriteString(";")
		},
	}
}

type StatementGroupOptions struct {
	Separator func(indent string) string
}

func GroupS(opts *StatementGroupOptions, statements ...Statement) Statement {
	return Statement{
		WriteSource: func(b *bytes.Buffer, indent string) {
			for i, s := range statements {
				s.WriteSource(b, indent)

				if opts != nil && i < len(statements)-1 {
					b.WriteString(opts.Separator(indent))
				}
			}
		},
	}
}

var Compact = &StatementGroupOptions{
	Separator: func(indent string) string {
		return "\n" + indent
	},
}

var Spaced = &StatementGroupOptions{
	Separator: func(indent string) string {
		return "\n\n" + indent
	},
}

const (
	GroupingPrecedence                 int = 18
	AccessAndCallPrecedence            int = 17
	NewPrecedence                      int = 16
	PostfixPrecedence                  int = 15
	PrefixPrecedence                   int = 14
	ExponentiationPrecedence           int = 13
	MultiplicativePrecedence           int = 12
	AdditivePrecedence                 int = 11
	ShiftPrecedence                    int = 10
	RelationalPrecedence               int = 9
	EqualityPrecedence                 int = 8
	BitwiseAndPrecedence               int = 7
	BitwiseXorPrecedence               int = 6
	BitwiseOrPrecedence                int = 5
	LogicalAndPrecedence               int = 4
	LogicalOrNullishCoalescePrecedence int = 3
	AssignmentMiscellaneousPrecedence  int = 2
	CommaPrecedence                    int = 1
)

type Expression struct {
	precedence        int
	WriteBlockSource  func(b *bytes.Buffer, indent string)
	WriteInlineSource func(b *bytes.Buffer)
}

func (e Expression) Precedence() int {
	return e.precedence
}

func (e Expression) WriteSource(b *bytes.Buffer, indent string) {
	var l bytes.Buffer
	e.WriteInlineSource(&l)
	if l.Len() > TargetWidth {
		e.WriteBlockSource(b, indent)
	} else {
		e.WriteInlineSource(b)
	}
}

func (e Expression) Call(args ...Expression) Expression {
	return appE(&e, args...)
}

func (e Expression) Index(index Expression) Expression {
	return Expression{
		precedence: AccessAndCallPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			e.WriteSource(b, indent)
			b.WriteString("[")
			index.WriteSource(b, indent)
			b.WriteString("]")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			e.WriteInlineSource(b)
			b.WriteString("[")
			index.WriteInlineSource(b)
			b.WriteString("]")
		},
	}
}

func (e Expression) Method(
	name string,
	args ...Expression,
) Expression {
	return Expression{
		precedence: AccessAndCallPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			e.WriteSource(b, indent)
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(Indentation)
			b.WriteString(".")
			b.WriteString(name)

			appE(nil, args...).WriteSource(b, indent+Indentation)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			e.WriteInlineSource(b)
			b.WriteString(".")
			b.WriteString(name)

			appE(nil, args...).WriteInlineSource(b)
		},
	}
}

func (e Expression) Property(
	property string,
) Expression {
	return Expression{
		precedence: AccessAndCallPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			e.WriteSource(b, indent)
			b.WriteString(".")
			b.WriteString(property)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			e.WriteInlineSource(b)
			b.WriteString(".")
			b.WriteString(property)
		},
	}
}

func literalE(literal string) Expression {
	return Expression{
		precedence: GroupingPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(literal)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString(literal)
		},
	}
}

func RefE(name string) Expression {
	return literalE(name)
}

var UndefinedE = literalE("undefined")

var NullE = literalE("null")

var TrueE = literalE("true")

var FalseE = literalE("false")

func StringE(s string) Expression {
	return literalE(fmt.Sprintf(`"%s"`, s))
}

func IntE(x int64) Expression {
	return literalE(fmt.Sprintf("%d", x))
}

func DoubleE(x float64) Expression {
	return literalE(fmt.Sprintf("%f", x))
}

func NumberE(x *big.Float) Expression {
	return literalE(x.String())
}

func QuoteE(e Expression) Expression {
	return Expression{
		precedence: GroupingPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("\"")
			e.WriteSource(b, indent)
			b.WriteString("\"")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString("\"")
			e.WriteInlineSource(b)
			b.WriteString("\"")
		},
	}
}

func AsE(
	e Expression,
	t Type,
) Expression {
	return Expression{
		precedence: RelationalPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			PrecedenceE(RelationalPrecedence, e).WriteSource(b, indent)
			b.WriteString(" as ")
			t.WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			PrecedenceE(RelationalPrecedence, e).WriteInlineSource(b)
			b.WriteString(" as ")
			t.WriteInlineSource(b)
		},
	}
}

func NewE(
	t Type,
	args ...Expression,
) Expression {
	newCls := &Expression{
		precedence: NewPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("new ")
			t.WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString("new ")
			t.WriteInlineSource(b)
		},
	}

	return appE(newCls, args...)
}

func ObjectE(properties ...ObjectProperty) Expression {
	return Expression{
		precedence: GroupingPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			if len(properties) == 0 {
				b.WriteString("{}")
				return
			}

			b.WriteString("{\n")
			for _, p := range properties {
				b.WriteString(indent)
				b.WriteString(Indentation)
				p.WriteSource(b, indent+Indentation)
				b.WriteString(",\n")
			}

			b.WriteString(indent)
			b.WriteString("}")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			if len(properties) == 0 {
				b.WriteString("{}")
				return
			}

			b.WriteString("{ ")
			for i, p := range properties {
				if i > 0 {
					b.WriteString(", ")
				}

				p.WriteSource(b, "")
			}

			b.WriteString(" }")
		},
	}
}

type ObjectProperty struct {
	WriteSource func(b *bytes.Buffer, indent string)
}

func StringKeyOP(key string, value Expression) ObjectProperty {
	return ObjectProperty{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(key)
			b.WriteString(": ")
			value.WriteSource(b, indent)
		},
	}
}

func NumberKeyOP(key int, value Expression) ObjectProperty {
	return ObjectProperty{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(fmt.Sprintf("%d: ", key))
			value.WriteSource(b, indent)
		},
	}
}

func ComputedKeyOP(key Expression, value Expression) ObjectProperty {
	return ObjectProperty{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("[")
			key.WriteSource(b, indent)
			b.WriteString("]: ")
			value.WriteSource(b, indent)
		},
	}
}

func SpreadOP(e Expression) ObjectProperty {
	return ObjectProperty{
		WriteSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("...")
			PrecedenceE(AssignmentMiscellaneousPrecedence, e).WriteSource(b, indent)
		},
	}
}

func ParenE(e Expression) Expression {
	return Expression{
		precedence: GroupingPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString("(")
			e.WriteSource(b, indent)
			b.WriteString(")")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString("(")
			e.WriteInlineSource(b)
			b.WriteString(")")
		},
	}
}

func PrecedenceE(
	precedence int,
	e Expression,
) Expression {
	return Expression{
		precedence: precedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			if e.Precedence() < precedence {
				ParenE(e).WriteSource(b, indent)
			} else {
				e.WriteSource(b, indent)
			}
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			if e.Precedence() < precedence {
				ParenE(e).WriteInlineSource(b)
			} else {
				e.WriteInlineSource(b)
			}
		},
	}
}

func prefixUnOpE(
	precedence int,
	op string,
	e Expression,
) Expression {
	return Expression{
		precedence: precedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			b.WriteString(op)
			PrecedenceE(precedence, e).WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			b.WriteString(op)
			PrecedenceE(precedence, e).WriteInlineSource(b)
		},
	}
}

func postfixUnOpE(
	precedence int,
	op string,
	e Expression,
) Expression {
	return Expression{
		precedence: precedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			PrecedenceE(precedence, e).WriteSource(b, indent)
			b.WriteString(op)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			PrecedenceE(precedence, e).WriteInlineSource(b)
			b.WriteString(op)
		},
	}
}

func binOpE(
	precedence int,
	op string,
	lhs Expression,
	rhs Expression,
) Expression {
	return Expression{
		precedence: precedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			PrecedenceE(precedence, lhs).WriteSource(b, indent)
			b.WriteString(" ")
			b.WriteString(op)
			b.WriteString(" ")
			PrecedenceE(precedence, rhs).WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			PrecedenceE(precedence, lhs).WriteInlineSource(b)
			b.WriteString(" ")
			b.WriteString(op)
			b.WriteString(" ")
			PrecedenceE(precedence, rhs).WriteInlineSource(b)
		},
	}
}

func AndE(lhs Expression, rhs Expression) Expression {
	return binOpE(LogicalAndPrecedence, "&&", lhs, rhs)
}

func OrE(lhs Expression, rhs Expression) Expression {
	return binOpE(LogicalOrNullishCoalescePrecedence, "||", lhs, rhs)
}

func NotE(e Expression) Expression {
	return prefixUnOpE(PrefixPrecedence, "!", e)
}

func AddE(lhs Expression, rhs Expression) Expression {
	return binOpE(AdditivePrecedence, "+", lhs, rhs)
}

func SubtractE(lhs Expression, rhs Expression) Expression {
	return binOpE(AdditivePrecedence, "-", lhs, rhs)
}

func MultiplyE(lhs Expression, rhs Expression) Expression {
	return binOpE(MultiplicativePrecedence, "*", lhs, rhs)
}

func DivideE(lhs Expression, rhs Expression) Expression {
	return binOpE(MultiplicativePrecedence, "/", lhs, rhs)
}

func ModuloE(lhs Expression, rhs Expression) Expression {
	return binOpE(MultiplicativePrecedence, "%", lhs, rhs)
}

func NegateE(e Expression) Expression {
	return prefixUnOpE(PrefixPrecedence, "-", e)
}

func EqualE(lhs Expression, rhs Expression) Expression {
	return binOpE(EqualityPrecedence, "===", lhs, rhs)
}

func NotEqualE(lhs Expression, rhs Expression) Expression {
	return binOpE(EqualityPrecedence, "!==", lhs, rhs)
}

func LessThanE(lhs Expression, rhs Expression) Expression {
	return binOpE(RelationalPrecedence, "<", lhs, rhs)
}

func LessThanOrEqualE(lhs Expression, rhs Expression) Expression {
	return binOpE(RelationalPrecedence, "<=", lhs, rhs)
}

func GreaterThanE(lhs Expression, rhs Expression) Expression {
	return binOpE(RelationalPrecedence, ">", lhs, rhs)
}

func GreaterThanOrEqualE(lhs Expression, rhs Expression) Expression {
	return binOpE(RelationalPrecedence, ">=", lhs, rhs)
}

func PrefixIncrementE(e Expression) Expression {
	return prefixUnOpE(PrefixPrecedence, "++", e)
}

func PrefixDecrementE(e Expression) Expression {
	return prefixUnOpE(PrefixPrecedence, "--", e)
}

func PostfixIncrementE(e Expression) Expression {
	return postfixUnOpE(PostfixPrecedence, "++", e)
}

func PostfixDecrementE(e Expression) Expression {
	return postfixUnOpE(PostfixPrecedence, "--", e)
}

func TernaryE(
	condition Expression,
	t Expression,
	f Expression,
) Expression {
	return Expression{
		precedence: AssignmentMiscellaneousPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			PrecedenceE(AssignmentMiscellaneousPrecedence, condition).WriteSource(b, indent)
			b.WriteString(" ? ")
			PrecedenceE(AssignmentMiscellaneousPrecedence, t).WriteSource(b, indent)
			b.WriteString(" : ")
			PrecedenceE(AssignmentMiscellaneousPrecedence, f).WriteSource(b, indent)
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			PrecedenceE(AssignmentMiscellaneousPrecedence, condition).WriteInlineSource(b)
			b.WriteString(" ? ")
			PrecedenceE(AssignmentMiscellaneousPrecedence, t).WriteInlineSource(b)
			b.WriteString(" : ")
			PrecedenceE(AssignmentMiscellaneousPrecedence, f).WriteInlineSource(b)
		},
	}
}

func appE(
	f *Expression,
	args ...Expression,
) Expression {
	return Expression{
		precedence: AccessAndCallPrecedence,
		WriteBlockSource: func(b *bytes.Buffer, indent string) {
			if f != nil {
				f.WriteSource(b, indent)
			}

			b.WriteString("(")

			if len(args) > 0 {
				b.WriteString("\n")
				b.WriteString(indent)
				b.WriteString(Indentation)

				for i, arg := range args {
					if i > 0 {
						b.WriteString(",\n")
						b.WriteString(indent)
						b.WriteString(Indentation)
					}

					arg.WriteSource(b, indent+Indentation)
				}

				b.WriteString("\n")
				b.WriteString(indent)
			}

			b.WriteString(")")
		},
		WriteInlineSource: func(b *bytes.Buffer) {
			if f != nil {
				f.WriteInlineSource(b)
			}

			b.WriteString("(")

			for i, arg := range args {
				if i > 0 {
					b.WriteString(", ")
				}

				arg.WriteInlineSource(b)
			}

			b.WriteString(")")
		},
	}
}

type TypeDoc struct {
	CommentLines           []string
	TypeParameters         []TypeDocTypeParameter
	Parameters             []TypeDocParameter
	Deprecation            string
	IsPackageDocumentation bool
	IsInternal             bool
}

type TypeDocTypeParameter struct {
	Name    string
	Comment string
}

type TypeDocParameter struct {
	Name    string
	Comment string
}

func TD(comment string) *TypeDoc {
	var commentLines []string
	if comment == "" {
		commentLines = nil
	} else {
		commentLines = strings.Split(comment, "\n")
	}

	return &TypeDoc{
		CommentLines:           commentLines,
		TypeParameters:         make([]TypeDocTypeParameter, 0),
		Parameters:             make([]TypeDocParameter, 0),
		Deprecation:            "",
		IsPackageDocumentation: false,
		IsInternal:             false,
	}
}

func (td *TypeDoc) TypeParameter(name string, comment string) *TypeDoc {
	td.TypeParameters = append(
		td.TypeParameters,
		TypeDocTypeParameter{Name: name, Comment: comment},
	)

	return td
}

func (td *TypeDoc) Parameter(name string, comment string) *TypeDoc {
	td.Parameters = append(
		td.Parameters,
		TypeDocParameter{Name: name, Comment: comment},
	)

	return td
}

func (td *TypeDoc) Deprecated(deprecation string) *TypeDoc {
	td.Deprecation = deprecation
	return td
}

func (td *TypeDoc) PackageDocumentation() *TypeDoc {
	td.IsPackageDocumentation = true
	return td
}

func (td *TypeDoc) Internal() *TypeDoc {
	td.IsInternal = true
	return td
}

func (td *TypeDoc) WriteBlockSource(b *bytes.Buffer, indent string) {
	hasOutput := len(td.CommentLines) > 0 ||
		len(td.TypeParameters) > 0 ||
		len(td.Parameters) > 0 ||
		td.Deprecation != "" ||
		td.IsPackageDocumentation ||
		td.IsInternal

	if !hasOutput {
		return
	}

	hasContents := false

	b.WriteString("/**\n")
	b.WriteString(indent)
	b.WriteString(" *")

	if len(td.CommentLines) > 0 {
		hasContents = true

		for _, line := range td.CommentLines {
			if line != "" {
				b.WriteString(" ")
				b.WriteString(line)
			}

			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
	}

	if len(td.TypeParameters) > 0 {
		if hasContents {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
		hasContents = true

		for _, tp := range td.TypeParameters {
			b.WriteString(" @typeParam ")
			b.WriteString(tp.Name)
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *  ")
			b.WriteString(tp.Comment)
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
	}

	if len(td.Parameters) > 0 {
		if hasContents {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
		hasContents = true

		for _, p := range td.Parameters {
			b.WriteString(" @param ")
			b.WriteString(p.Name)
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *  ")
			b.WriteString(p.Comment)
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
	}

	if td.Deprecation != "" {
		if hasContents {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
		hasContents = true

		b.WriteString(" @deprecated\n")
		b.WriteString(indent)
		b.WriteString(" *  ")
		b.WriteString(td.Deprecation)
		b.WriteString("\n")
		b.WriteString(indent)
		b.WriteString(" *")
	}

	if td.IsPackageDocumentation {
		if hasContents {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
		hasContents = true

		b.WriteString(" @packageDocumentation\n")
		b.WriteString(indent)
		b.WriteString(" *")
	}

	if td.IsInternal {
		if hasContents {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(" *")
		}
		hasContents = true

		b.WriteString(" @internal\n")
		b.WriteString(indent)
		b.WriteString(" *")
	}

	b.WriteString("/")
}
