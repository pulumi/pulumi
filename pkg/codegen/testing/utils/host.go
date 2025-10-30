package utils

import utils "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/utils"

type SchemaProvider = utils.SchemaProvider

func NewSchemaProvider(name, version string) SchemaProvider {
	return utils.NewSchemaProvider(name, version)
}

// NewHost creates a schema-only plugin host, supporting multiple package versions in tests. This
// enables running tests offline. If this host is used to load a plugin, that is, to run a Pulumi
// program, it will panic.
func NewHostWithProviders(schemaDirectoryPath string, providers ...SchemaProvider) plugin.Host {
	return utils.NewHostWithProviders(schemaDirectoryPath, providers...)
}

// NewHost creates a schema-only plugin host, supporting multiple package versions in tests. This
// enables running tests offline. If this host is used to load a plugin, that is, to run a Pulumi
// program, it will panic.
func NewHost(schemaDirectoryPath string) plugin.Host {
	return utils.NewHost(schemaDirectoryPath)
}

