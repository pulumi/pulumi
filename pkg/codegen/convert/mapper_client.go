package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

func NewMapperClient(target string) (Mapper, error) {
	return convert.NewMapperClient(target)
}

