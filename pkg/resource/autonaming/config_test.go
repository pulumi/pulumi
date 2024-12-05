// Copyright 2024, Pulumi Corporation.
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

package autonaming

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseAutonamingConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		org        string
		configYAML string
		wantConfig *globalAutonaming
		wantErr    string
	}{
		{
			name:       "empty config returns nil",
			configYAML: "",
			wantConfig: nil,
		},
		{
			name: "default config",
			configYAML: `
pulumi:autonaming:
  mode: default`,
			wantConfig: &globalAutonaming{
				Default:   &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "basic pattern config",
			configYAML: `
pulumi:autonaming:
  pattern: ${name}-${random(8)}
  enforce: true`,
			wantConfig: &globalAutonaming{
				Default: &patternAutonaming{
					Pattern: "${name}-${random(8)}",
					Enforce: true,
				},
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "basic verbatim config",
			configYAML: `
pulumi:autonaming:
  mode: verbatim`,
			wantConfig: &globalAutonaming{
				Default:   &verbatimAutonaming{},
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "basic disabled config",
			configYAML: `
pulumi:autonaming:
  mode: disabled`,
			wantConfig: &globalAutonaming{
				Default:   &disabledAutonaming{},
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "provider pattern config",
			configYAML: `
pulumi:autonaming:
  providers:
    aws:
      pattern: aws-${name}
      enforce: false`,
			wantConfig: &globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &patternAutonaming{
							Pattern: "aws-${name}",
							Enforce: false,
						},
						Resources: map[string]autonamingStrategy{},
					},
				},
			},
		},
		{
			name: "resource pattern config",
			configYAML: `
pulumi:autonaming:
  providers:
    aws:
      resources:
        aws:s3/bucket:Bucket:
          pattern: bucket-${name}
          enforce: true`,
			wantConfig: &globalAutonaming{
				Default: &defaultAutonamingConfig,
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &defaultAutonamingConfig,
						Resources: map[string]autonamingStrategy{
							"aws:s3/bucket:Bucket": &patternAutonaming{
								Pattern: "bucket-${name}",
								Enforce: true,
							},
						},
					},
				},
			},
		},
		{
			name: "basic pattern config with org, project, and stack",
			org:  "myorg",
			configYAML: `
pulumi:autonaming:
  pattern: ${organization}-${project}-${stack}-${name}-${random(8)}`,
			wantConfig: &globalAutonaming{
				Default: &patternAutonaming{
					Pattern: "myorg-myproj-mystack-${name}-${random(8)}",
				},
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "config values are available",
			configYAML: `
pulumi:autonaming:
  pattern: ${name}-${config.foo}
myproj:foo: bar`,
			wantConfig: &globalAutonaming{
				Default: &patternAutonaming{
					Pattern: "${name}-bar",
				},
				Providers: map[string]providerAutonaming{},
			},
		},
		{
			name: "invalid config section returns error",
			configYAML: `
pulumi:autonaming: 123`,
			wantErr: "invalid autonaming config structure",
		},
		{
			name: "invalid mode returns error",
			configYAML: `
pulumi:autonaming:
  mode: invalid`,
			wantErr: "invalid naming mode: invalid",
		},
		{
			name: "invalid provider mode returns error",
			configYAML: `
pulumi:autonaming:
  mode: verbatim
  providers:
    aws:
      mode: magic`,
			wantErr: "invalid naming mode: magic",
		},
		{
			name: "invalid resource mode returns error",
			configYAML: `
pulumi:autonaming:
  mode: verbatim
  providers:
    aws:
      resources:
        aws:s3/bucket:Bucket:
          mode: custom`,
			wantErr: "invalid naming mode: custom",
		},
		{
			name: "cannot specify both mode and pattern",
			configYAML: `
pulumi:autonaming:
  mode: verbatim
  pattern: test-${name}`,
			wantErr: "cannot specify both mode and pattern/enforce",
		},
		{
			name: "cannot specify both mode and enforce",
			configYAML: `
pulumi:autonaming:
  mode: verbatim
  enforce: true`,
			wantErr: "cannot specify both mode and pattern/enforce",
		},
		{
			name: "invalid config structure returns error",
			configYAML: `
pulumi:autonaming:
  mode: verbatim
  invalid_field: value`,
			wantErr: "invalid autonaming config structure",
		},
		{
			name: "error in config",
			configYAML: `
pulumi:autonaming:
  pattern: ${name}-${config.unknown}`,
			wantErr: "no value found for key \"unknown\"",
		},
		{
			name: "complex config with all features",
			configYAML: `
pulumi:autonaming:
  pattern: global-${name}
  enforce: false
  providers:
    aws:
      pattern: ${stack}-aws-${name}
      enforce: true
      resources:
        aws:s3/bucket:Bucket:
          pattern: ${config.foo}-bucket-${name}-${uuid}
          enforce: true
    azure:
      mode: verbatim
      resources:
        azure:storage/account:Account:
          mode: disabled
myproj:foo: bar`,
			wantConfig: &globalAutonaming{
				Default: &patternAutonaming{
					Pattern: "global-${name}",
					Enforce: false,
				},
				Providers: map[string]providerAutonaming{
					"aws": {
						Default: &patternAutonaming{
							Pattern: "mystack-aws-${name}",
							Enforce: true,
						},
						Resources: map[string]autonamingStrategy{
							"aws:s3/bucket:Bucket": &patternAutonaming{
								Pattern: "bar-bucket-${name}-${uuid}",
								Enforce: true,
							},
						},
					},
					"azure": {
						Default: &verbatimAutonaming{},
						Resources: map[string]autonamingStrategy{
							"azure:storage/account:Account": &disabledAutonaming{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Map{}
			err := cfg.UnmarshalYAML(func(v interface{}) error {
				var raw map[string]config.Value
				if err := yaml.Unmarshal([]byte(tt.configYAML), &raw); err != nil {
					return err
				}
				target := v.(*map[string]config.Value)
				*target = raw
				return nil
			})
			require.NoError(t, err)

			decrypter := config.NewPanicCrypter()

			org := tt.org
			if org == "" {
				org = "default"
			}
			stack := &mockStack{orgName: org}
			autonamer, err := ParseAutonamingConfig(&backend.StackConfiguration{Config: cfg}, decrypter, stack)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantConfig == nil {
				assert.Nil(t, autonamer)
			} else {
				got := autonamer.(*globalAutonaming)
				assert.Equal(t, tt.wantConfig.Default, got.Default)
				assert.Equal(t, tt.wantConfig.Providers, got.Providers)
			}
		})
	}
}

type mockStack struct {
	httpstate.Stack
	orgName string
}

func (m *mockStack) OrgName() string {
	return m.orgName
}

func (m *mockStack) Ref() backend.StackReference {
	return &backend.MockStackReference{
		ProjectV: "myproj",
		NameV:    tokens.MustParseStackName("mystack"),
	}
}
