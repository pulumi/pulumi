package utils

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func GetSchema(schemaDirectoryPath, providerName string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(schemaDirectoryPath, providerName+".json"))
}

type ProviderLoader func(string) (plugin.Provider, error)

func NewProviderLoader(pkg string) ProviderLoader {
	return func(schemaDirectoryPath string) (plugin.Provider, error) {
		schema, err := GetSchema(schemaDirectoryPath, pkg)
		if err != nil {
			return nil, err
		}
		return &deploytest.Provider{
			GetSchemaF: func(version int) ([]byte, error) {
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
