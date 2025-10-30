package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

func NewMapperServer(mapper Mapper) codegenrpc.MapperServer {
	return convert.NewMapperServer(mapper)
}

func MapperRegistration(m codegenrpc.MapperServer) func(*grpc.Server) {
	return convert.MapperRegistration(m)
}

