package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

type NewTypeNameGeneratorFunc = test.NewTypeNameGeneratorFunc

type TypeNameGeneratorFunc = test.TypeNameGeneratorFunc

func TestTypeNameCodegen(t *testing.T, language string, newTypeNameGenerator NewTypeNameGeneratorFunc) {
	test.TestTypeNameCodegen(t, language, newTypeNameGenerator)
}

