package unauthenticatedregistry

import unauthenticatedregistry "github.com/pulumi/pulumi/sdk/v3/pkg/backend/diy/unauthenticatedregistry"

func New(sink diag.Sink, store env.Env) registry.Registry {
	return unauthenticatedregistry.New(sink, store)
}

