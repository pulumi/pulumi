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
	makeURN := func(name, typ string) urn.URN {
		return urn.New("mystack", "myproject", "", tokens.Type(typ), name)
	}

	tests := []struct {
		name                    string
		options                 globalAutonaming
		urn                     urn.URN
		wantOptions             *plugin.AutonamingOptions
		wantDeleteBeforeReplace bool
		wantErrMsg              string
	}{
		{
			name:        "no config returns no options",
			options:     globalAutonaming{},
			urn:         makeURN("myresource", "aws:s3/bucket:Bucket"),
			wantOptions: nil,
		},
		{
			name: "default config returns no options",
			options: globalAutonaming{
				Default: &defaultAutonamingConfig,
			},
			urn:         makeURN("myresource", "aws:s3/bucket:Bucket"),
			wantOptions: nil,
		},
		{
			name: "verbatim config enforces logical name",
			options: globalAutonaming{
				Default: &verbatimAutonaming{},
			},
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
						Resources: map[string]autonamingStrategy{
							"aws:s3/bucket:Bucket": &verbatimAutonaming{},
						},
					},
				},
			},
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
						Resources: map[string]autonamingStrategy{
							"aws:s3/bucket:Bucket": &disabledAutonaming{},
						},
					},
				},
			},
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
						Resources: map[string]autonamingStrategy{
							"aws:s3/bucket:Bucket": &patternAutonaming{
								Pattern: "bucket-${name}",
								Enforce: true,
							},
						},
					},
				},
			},
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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
			urn:        makeURN("myresource", "invalid:type"),
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
						Resources: map[string]autonamingStrategy{
							"aws:s3/object:Object": &disabledAutonaming{},
						},
					},
				},
			},
			urn:                     makeURN("myresource", "aws:s3/bucket:Bucket"),
			wantOptions:             nil,
			wantDeleteBeforeReplace: false,
		},
		{
			name: "global config is used if provider does not define a config other than specific resource",
			options: globalAutonaming{
				Default: &verbatimAutonaming{},
				Providers: map[string]providerAutonaming{
					"aws": {
						Resources: map[string]autonamingStrategy{
							"aws:s3/object:Object": &disabledAutonaming{},
						},
					},
				},
			},
			urn: makeURN("myresource", "aws:s3/bucket:Bucket"),
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

			got, deleteBeforeReplace, err := tt.options.AutonamingForResource(tt.urn, nil)

			if tt.wantErrMsg != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantDeleteBeforeReplace, deleteBeforeReplace)
			assert.Equal(t, tt.wantOptions, got)
		})
	}
}
