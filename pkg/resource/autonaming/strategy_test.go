// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package autonaming

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestGlobalAutonaming_AutonamingForResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		options                 globalAutonaming
		wantOptions             *plugin.AutonamingOptions
		wantDeleteBeforeReplace bool
		wantErrMsg              string
	}{
		{
			name:        "no config returns no options",
			options:     globalAutonaming{},
			wantOptions: nil,
		},
		{
			name: "default config returns no options",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
			},
			wantOptions: nil,
		},
		{
			name: "verbatim config enforces logical name",
			options: globalAutonaming{
				Default: &verbatimAutonaming{},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "myresource",
				Mode:            plugin.AutonamingModeEnforce,
				WarnIfNoSupport: false,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "verbatim config on provider enforces logical name",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &verbatimAutonaming{},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "myresource",
				Mode:            plugin.AutonamingModeEnforce,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "verbatim config on resource enforces logical name",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &defaultAutonamingConfig,
						Resources: map[string]Autonamer{
							"aws:s3/bucket:Bucket": &verbatimAutonaming{},
						},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "myresource",
				Mode:            plugin.AutonamingModeEnforce,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "disabled config",
			options: globalAutonaming{
				Default: &disabledAutonaming{},
			},
			wantOptions: &plugin.AutonamingOptions{
				Mode:            plugin.AutonamingModeDisabled,
				WarnIfNoSupport: false,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "disabled config on provider",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &disabledAutonaming{},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				Mode:            plugin.AutonamingModeDisabled,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "disabled config on resource",
			options: globalAutonaming{
				Default: &verbatimAutonaming{},
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &verbatimAutonaming{},
						Resources: map[string]Autonamer{
							"aws:s3/bucket:Bucket": &disabledAutonaming{},
						},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				Mode:            plugin.AutonamingModeDisabled,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "provider-specific config overrides default",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &patternAutonaming{
							Pattern: "aws-${name}",
							Enforce: false,
						},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "aws-myresource",
				Mode:            plugin.AutonamingModePropose,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "resource-specific config overrides provider default",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &patternAutonaming{
							Pattern: "aws-${name}",
							Enforce: false,
						},
						Resources: map[string]Autonamer{
							"aws:s3/bucket:Bucket": &patternAutonaming{
								Pattern: "bucket-${name}",
								Enforce: true,
							},
						},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "bucket-myresource",
				Mode:            plugin.AutonamingModeEnforce,
				WarnIfNoSupport: true,
			},
			wantDeleteBeforeReplace: true,
		},
		{
			name: "invalid resource type returns error",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
			},
			wantErrMsg: "invalid resource type format: invalid:type",
		},
		{
			name: "unrelated provider and resource configs are ignored",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"azure": {
						Default: &disabledAutonaming{},
					},
					"aws": {
						Default: &defaultAutonamingConfig,
						Resources: map[string]Autonamer{
							"aws:s3/object:Object": &disabledAutonaming{},
						},
					},
				},
			},
			wantOptions:             nil,
			wantDeleteBeforeReplace: false,
		},
		{
			name: "global config is used if provider does not define a config other than specific resource",
			options: globalAutonaming{
				Default: &verbatimAutonaming{},
				Providers: map[string]providerAutonaming{
					"aws": {
						Resources: map[string]Autonamer{
							"aws:s3/object:Object": &disabledAutonaming{},
						},
					},
				},
			},
			wantOptions: &plugin.AutonamingOptions{
				ProposedName:    "myresource",
				Mode:            plugin.AutonamingModeEnforce,
				WarnIfNoSupport: false,
			},
			wantDeleteBeforeReplace: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			urn := urn.New("mystack", "myproject", "", tokens.Type("aws:s3/bucket:Bucket"), "myresource")
			got, deleteBeforeReplace := tt.options.AutonamingForResource(urn, nil)
			assert.Equal(t, tt.wantDeleteBeforeReplace, deleteBeforeReplace)
			assert.Equal(t, tt.wantOptions, got)
		})
	}
}
