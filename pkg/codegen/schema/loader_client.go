package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// LoaderClient reflects a loader service, loaded dynamically from the engine process over gRPC.
type LoaderClient = schema.LoaderClient

func NewLoaderClient(target string) (*LoaderClient, error) {
	return schema.NewLoaderClient(target)
}

