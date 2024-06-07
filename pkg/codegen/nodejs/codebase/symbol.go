package codebase

type BuiltinSymbol struct {
	Name string
}

func (s BuiltinSymbol) AsExpression() Expression {
	return literalE(s.Name)
}

func (s BuiltinSymbol) AsType() Type {
	return namedT(s.Name)
}

type DefaultSymbol struct {
	Module      string
	DefaultName string
}

func (s DefaultSymbol) AsExpression() Expression {
	return literalE(s.DefaultName)
}

func (s DefaultSymbol) AsType() Type {
	return namedT(s.DefaultName)
}

type NamedSymbol struct {
	Module string
	Name   string
	As     string
}

func (s NamedSymbol) ImportedName() string {
	if s.As != "" {
		return s.As
	}

	return s.Name
}

func (s NamedSymbol) AsExpression() Expression {
	return literalE(s.ImportedName())
}

func (s NamedSymbol) AsType() Type {
	return namedT(s.ImportedName())
}

type QualifiedSymbol struct {
	Module        string
	Qualification string
	Namespace     string
	Name          string
}

func (s QualifiedSymbol) QualifiedName() string {
	n := s.Qualification
	if s.Namespace != "" {
		n += "." + s.Namespace
	}

	n += "." + s.Name
	return n
}

func (s QualifiedSymbol) AsExpression() Expression {
	return literalE(s.QualifiedName())
}

func (s QualifiedSymbol) AsType() Type {
	return namedT(s.QualifiedName())
}
