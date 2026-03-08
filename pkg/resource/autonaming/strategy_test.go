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
		options                 Global
		wantOptions             *plugin.AutonamingOptions
		wantDeleteBeforeReplace bool
		wantErrMsg              string
	}{
		{
			name:        "no config returns no options",
			options:     Global{},
			wantOptions: nil,
		},
		{
			name: "default config returns no options",
			options: Global{
				Default: defaultAutonamingConfig,
			},
			wantOptions: nil,
		},
		{
			name: "verbatim config enforces logical name",
			options: Global{
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
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
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
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
					"aws": {
						Default: defaultAutonamingConfig,
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
			options: Global{
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
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
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
			options: Global{
				Default: &verbatimAutonaming{},
				Providers: map[string]Provider{
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
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
					"aws": {
						Default: &Pattern{
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
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
					"aws": {
						Default: &Pattern{
							Pattern: "aws-${name}",
							Enforce: false,
						},
						Resources: map[string]Autonamer{
							"aws:s3/bucket:Bucket": &Pattern{
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
			options: Global{
				Default: defaultAutonamingConfig,
			},
			wantErrMsg: "invalid resource type format: invalid:type",
		},
		{
			name: "unrelated provider and resource configs are ignored",
			options: Global{
				Default: defaultAutonamingConfig,
				Providers: map[string]Provider{
					"azure": {
						Default: &disabledAutonaming{},
					},
					"aws": {
						Default: defaultAutonamingConfig,
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
			options: Global{
				Default: &verbatimAutonaming{},
				Providers: map[string]Provider{
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			urn := urn.New("mystack", "myproject", "", tokens.Type("aws:s3/bucket:Bucket"), "myresource")
			got, deleteBeforeReplace := tt.options.AutonamingForResource(urn, nil)
			assert.Equal(t, tt.wantDeleteBeforeReplace, deleteBeforeReplace)
			assert.Equal(t, tt.wantOptions, got)
		})
	}
}

func TestGenerateName(t *testing.T) {
	t.Parallel()
	urn := urn.New("mystack", "myproject", "", "aws:s3/bucket:Bucket", "myresource")
	randomSeed := []byte("test seed")

	tests := []struct {
		name          string
		pattern       string
		want          string
		wantHasRandom bool
	}{
		{
			name:          "hex generation",
			pattern:       "${name}-${hex(4)}",
			want:          "myresource-f93c",
			wantHasRandom: true,
		},
		{
			name:          "alphanum generation",
			pattern:       "${name}-${alphanum(5)}",
			want:          "myresource-3ufv0",
			wantHasRandom: true,
		},
		{
			name:          "string generation",
			pattern:       "${name}-${string(6)}",
			want:          "myresource-xgtfme",
			wantHasRandom: true,
		},
		{
			name:          "num generation",
			pattern:       "${num(7)}_${name}",
			want:          "5657624_myresource",
			wantHasRandom: true,
		},
		{
			name:          "uuid generation",
			pattern:       "${uuid}",
			want:          "f93c359b-82a2-9def-ec84-776cd822c95a",
			wantHasRandom: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, hasRandom := generateName(tt.pattern, urn, randomSeed)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantHasRandom, hasRandom)
		})
	}
}
