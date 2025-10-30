package tstypes

import tstypes "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/nodejs/tstypes"

// Supported types include type identifiers, arrays `T[]`, unions
// `A|B`, and maps with string keys.
type TypeAst = tstypes.TypeAst

// Produces a TypeScript type literal for the type, with minimally
// inserted parentheses.
func TypeLiteral(ast TypeAst) string {
	return tstypes.TypeLiteral(ast)
}

// Builds a type identifier (possibly qualified such as
// "my.module.MyType") or a primitive such as "boolean".
func Identifier(id string) TypeAst {
	return tstypes.Identifier(id)
}

// Builds a `T[]` type from a `T` type.
func Array(t TypeAst) TypeAst {
	return tstypes.Array(t)
}

// Builds a `{[key: string]: T}` type from a `T` type.
func StringMap(t TypeAst) TypeAst {
	return tstypes.StringMap(t)
}

// Builds a union `A | B | C` type.
func Union(t ...TypeAst) TypeAst {
	return tstypes.Union(t...)
}

// Normalizes by unnesting unions `A | (B | C) => A | B | C`.
func Normalize(ast TypeAst) TypeAst {
	return tstypes.Normalize(ast)
}

