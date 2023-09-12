// Copyright 2023, Pulumi Corporation.  All rights reserved.

package schema

type BooleanBuilder struct {
	s Schema
}

func Boolean() *BooleanBuilder {
	return &BooleanBuilder{}
}

func (b *BooleanBuilder) Const(v bool) *BooleanBuilder {
	b.s.Const = v
	return b
}

func (b *BooleanBuilder) Title(title string) *BooleanBuilder {
	b.s.Title = title
	return b
}

func (b *BooleanBuilder) Description(description string) *BooleanBuilder {
	b.s.Description = description
	return b
}

func (b *BooleanBuilder) Default(v bool) *BooleanBuilder {
	b.s.Default = v
	return b
}

func (b *BooleanBuilder) Deprecated(deprecated bool) *BooleanBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *BooleanBuilder) Schema() *Schema {
	b.s.Type = "boolean"
	return &b.s
}
