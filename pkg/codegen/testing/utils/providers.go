package utils

import (
	"os"
	"path/filepath"

	"github.com/edsrzf/mmap-go"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func GetSchema(schemaDirectoryPath, providerName string) ([]byte, error) {
	path := filepath.Join(schemaDirectoryPath, providerName+".json")
	schemaFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	// We'll leak this memory, but we are in a testing environment and
	schemaMmap, err := mmap.Map(schemaFile, mmap.RDONLY, 0)
	if err != nil {
		schemaFile.Close()
		return nil, err
	}

	return schemaMmap, nil
}

type ProviderLoader func(string) (plugin.Provider, error)

func NewProviderLoader(pkg string) ProviderLoader {
	return func(schemaDirectoryPath string) (plugin.Provider, error) {
		// Single instance schema:
		var schema []byte
		var err error
		return &deploytest.Provider{
			GetSchemaF: func(version int) ([]byte, error) {
				if schema != nil {
					return schema, nil
				}

				schema, err = GetSchema(schemaDirectoryPath, pkg)
				if err != nil {
					return nil, err
				}

				return schema, nil
			},
		}, nil
	}
}

var (
	AWS         = NewProviderLoader("aws")
	AwsNative   = NewProviderLoader("aws")
	Azure       = NewProviderLoader("azure")
	AzureNative = NewProviderLoader("azure-native")
	Random      = NewProviderLoader("random")
	Kubernetes  = NewProviderLoader("kubernetes")
	Other       = NewProviderLoader("other")
	Synthetic   = NewProviderLoader("synthetic")
)
