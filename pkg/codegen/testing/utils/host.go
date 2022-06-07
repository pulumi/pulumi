package utils

import (
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func NewHost(schemaDirectoryPath string) plugin.Host {
	return deploytest.NewPluginHost(nil, nil, nil,
		deploytest.NewProviderLoader("aws", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return AWS(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("azure", semver.MustParse("3.24.0"), func() (plugin.Provider, error) {
			return Azure(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("azure-native", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return AzureNative(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("random", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return Random(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("kubernetes", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return Kubernetes(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("aws-native", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return AwsNative(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("other", semver.MustParse("0.1.0"), func() (plugin.Provider, error) {
			return Other(schemaDirectoryPath)
		}),
		deploytest.NewProviderLoader("synthetic", semver.MustParse("0.1.0"), func() (plugin.Provider, error) {
			return Synthetic(schemaDirectoryPath)
		}),
	)
}
