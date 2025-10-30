package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

func NewLoaderServer(loader ReferenceLoader) codegenrpc.LoaderServer {
	return schema.NewLoaderServer(loader)
}

func LoaderRegistration(l codegenrpc.LoaderServer) func(*grpc.Server) {
	return schema.LoaderRegistration(l)
}

