package test

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func GetSchema(schemaDirectoryPath, providerName string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(schemaDirectoryPath, providerName+".json"))
}

func AWS(schemaDirectoryPath string) (plugin.Provider, error) {
	schema, err := GetSchema(schemaDirectoryPath, "aws")
	if err != nil {
		return nil, err
	}
	return &deploytest.Provider{
		GetSchemaF: func(version int) ([]byte, error) {
			return schema, nil
		},
	}, nil
}

func Azure(schemaDirectoryPath string) (plugin.Provider, error) {
	schema, err := GetSchema(schemaDirectoryPath, "azure")
	if err != nil {
		return nil, err
	}
	return &deploytest.Provider{
		GetSchemaF: func(version int) ([]byte, error) {
			return schema, nil
		},
	}, nil
}

func AzureNative(schemaDirectoryPath string) (plugin.Provider, error) {
	schema, err := GetSchema(schemaDirectoryPath, "azure-native")
	if err != nil {
		return nil, err
	}
	return &deploytest.Provider{
		GetSchemaF: func(version int) ([]byte, error) {
			return schema, nil
		},
	}, nil
}

func Random(schemaDirectoryPath string) (plugin.Provider, error) {
	schema, err := GetSchema(schemaDirectoryPath, "random")
	if err != nil {
		return nil, err
	}
	return &deploytest.Provider{
		GetSchemaF: func(version int) ([]byte, error) {
			return schema, nil
		},
	}, nil
}

func Kubernetes(schemaDirectoryPath string) (plugin.Provider, error) {
	schema, err := GetSchema(schemaDirectoryPath, "kubernetes")
	if err != nil {
		return nil, err
	}
	return &deploytest.Provider{
		GetSchemaF: func(version int) ([]byte, error) {
			return schema, nil
		},
	}, nil
}
