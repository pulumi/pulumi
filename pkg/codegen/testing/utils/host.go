package utils

import (
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func NewHost(schemaDirectoryPath string) plugin.Host {
	mockProvider := func(name tokens.Package, loader ProviderLoader) *deploytest.PluginLoader {
		return deploytest.NewProviderLoader(name, semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return loader(schemaDirectoryPath)
		}, deploytest.WithPath(schemaDirectoryPath))
	}

	return deploytest.NewPluginHost(nil, nil, nil,
		mockProvider("aws", AWS),
		mockProvider("azure", Azure),
		mockProvider("azure-native", AzureNative),
		mockProvider("random", Random),
		mockProvider("kubernetes", Kubernetes),
		mockProvider("aws-native", AwsNative),
		mockProvider("other", Other),
		mockProvider("synthetic", Synthetic),
	)
}
