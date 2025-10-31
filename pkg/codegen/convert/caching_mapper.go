package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

// NewCachingMapper creates a new caching mapper backed by the given Mapper.
func NewCachingMapper(mapper Mapper) Mapper {
	return convert.NewCachingMapper(mapper)
}

