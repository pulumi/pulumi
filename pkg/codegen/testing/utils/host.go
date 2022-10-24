package utils

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// NewHost creates a schema-only plugin host, supporting multiple package versions in tests. This
// enables running tests offline. If this host is used to load a plugin, that is, to run a Pulumi
// program, it will panic.
func NewHost(schemaDirectoryPath string) plugin.Host {
	mockProvider := func(name tokens.Package, version string) *deploytest.PluginLoader {
		return deploytest.NewProviderLoader(name, semver.MustParse(version), func() (plugin.Provider, error) {
			panic(fmt.Sprintf(
				"expected plugin loader to use cached schema path, but cache was missed for package %v@%v, "+
					"is an entry in the makefile or setup for this package missing?",
				name, version))
		}, deploytest.WithPath(schemaDirectoryPath))
	}

	// For the pulumi/pulumi repository, this must be kept in sync with the makefile and/or committed
	// schema files in the given schema directory. This is the minimal set of schemas that must be
	// supplied.
	return deploytest.NewPluginHost(nil, nil, nil,
		mockProvider("aws", "4.26.0"),
		mockProvider("aws", "5.16.2"),
		mockProvider("azure", "4.18.0"),
		mockProvider("azure-native", "1.29.0"),
		mockProvider("random", "4.2.0"),
		mockProvider("kubernetes", "3.7.2"),
		mockProvider("eks", "0.37.1"),
		mockProvider("docker", "3.4.1"),
		mockProvider("other", "0.1.0"),
		mockProvider("synthetic", "1.0.0"),
	)
}
