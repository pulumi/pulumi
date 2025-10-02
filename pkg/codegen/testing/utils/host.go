// Copyright 2020-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:revive // Legacy package name we don't want to change
package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type SchemaProvider struct {
	name    string
	version string
}

func NewSchemaProvider(name, version string) SchemaProvider {
	return SchemaProvider{name, version}
}

// NewHost creates a schema-only plugin host, supporting multiple package versions in tests. This
// enables running tests offline. If this host is used to load a plugin, that is, to run a Pulumi
// program, it will panic.
func NewHostWithProviders(schemaDirectoryPath string, providers ...SchemaProvider) plugin.Host {
	mockProvider := func(name tokens.Package, version string) *deploytest.PluginLoader {
		return deploytest.NewProviderLoader(name, semver.MustParse(version), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					path := filepath.Join(schemaDirectoryPath, fmt.Sprintf("%s-%s.json", name, version))
					data, err := os.ReadFile(path)
					if err != nil {
						return plugin.GetSchemaResponse{}, err
					}
					return plugin.GetSchemaResponse{
						Schema: data,
					}, nil
				},
			}, nil
		})
	}

	pluginLoaders := slice.Prealloc[*deploytest.PluginLoader](len(providers))

	for _, v := range providers {
		pluginLoaders = append(pluginLoaders, mockProvider(tokens.Package(v.name), v.version))
	}

	// For the pulumi/pulumi repository, this must be kept in sync with the makefile and/or committed
	// schema files in the given schema directory. This is the minimal set of schemas that must be
	// supplied.
	return deploytest.NewPluginHost(nil, nil, nil,
		pluginLoaders...,
	)
}

// NewHost creates a schema-only plugin host, supporting multiple package versions in tests. This
// enables running tests offline. If this host is used to load a plugin, that is, to run a Pulumi
// program, it will panic.
func NewHost(schemaDirectoryPath string) plugin.Host {
	// For the pulumi/pulumi repository, this must be kept in sync with the makefile and/or committed
	// schema files in the given schema directory. This is the minimal set of schemas that must be
	// supplied.
	return NewHostWithProviders(schemaDirectoryPath,
		SchemaProvider{"tls", "4.10.0"},
		SchemaProvider{"aws", "4.15.0"},
		SchemaProvider{"aws", "4.26.0"},
		SchemaProvider{"aws", "4.36.0"},
		SchemaProvider{"aws", "4.37.1"},
		SchemaProvider{"aws", "5.16.2"},
		SchemaProvider{"azure", "4.18.0"},
		SchemaProvider{"azure-native", "1.28.0"},
		SchemaProvider{"azure-native", "1.29.0"},
		SchemaProvider{"random", "4.2.0"},
		SchemaProvider{"random", "4.3.1"},
		SchemaProvider{"random", "4.11.2"},
		SchemaProvider{"kubernetes", "3.7.0"},
		SchemaProvider{"kubernetes", "3.7.2"},
		SchemaProvider{"eks", "0.37.1"},
		SchemaProvider{"google-native", "0.18.2"},
		SchemaProvider{"google-native", "0.27.0"},
		SchemaProvider{"aws-native", "0.99.0"},
		SchemaProvider{"docker", "3.1.0"},
		SchemaProvider{"std", "1.0.0"},
		// PCL examples in 'testing/test/testdata/transpiled_examples require these versions
		SchemaProvider{"aws", "5.4.0"},
		SchemaProvider{"azure-native", "1.56.0"},
		SchemaProvider{"eks", "0.40.0"},
		SchemaProvider{"aws-native", "0.13.0"},
		SchemaProvider{"docker", "4.0.0-alpha.0"},
		SchemaProvider{"awsx", "1.0.0-beta.5"},
		SchemaProvider{"kubernetes", "3.0.0"},
		SchemaProvider{"aws", "4.37.1"},

		SchemaProvider{"component", "13.3.7"},
		SchemaProvider{"other", "0.1.0"},
		SchemaProvider{"synthetic", "1.0.0"},
		SchemaProvider{"basic-unions", "0.1.0"},
		SchemaProvider{"range", "1.0.0"},
		SchemaProvider{"lambda", "0.1.0"},
		SchemaProvider{"remoteref", "1.0.0"},
		SchemaProvider{"splat", "1.0.0"},
		SchemaProvider{"snowflake", "0.66.1"},
		SchemaProvider{"using-dashes", "1.0.0"},
		SchemaProvider{"auto-deploy", "0.0.1"},
		SchemaProvider{"localref", "1.0.0"},
		SchemaProvider{"enum", "1.0.0"},
		SchemaProvider{"plain-properties", "1.0.0"},
		SchemaProvider{"recursive", "1.0.0"},
		SchemaProvider{"aws-static-website", "0.4.0"},

		SchemaProvider{"aliases", "1.0.0"},
		SchemaProvider{"dangling-reference-bad", "0.1.0"},
		SchemaProvider{"dangling-reference-good", "0.1.0"},
	)
}
