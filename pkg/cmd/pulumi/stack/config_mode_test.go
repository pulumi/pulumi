// Copyright 2026, Pulumi Corporation.
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

package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestNewStackConfigMode(t *testing.T) {
	tests := []struct {
		name          string
		automationAPI string
		escConfig     bool
		staticConfig  bool
		want          apitype.CreateStackConfigMode
	}{
		{name: "no flag", want: apitype.CreateStackConfigModeDefault},
		{name: "automation API", automationAPI: "true", want: apitype.CreateStackConfigModeStatic},
		{name: "--static-config", staticConfig: true, want: apitype.CreateStackConfigModeStatic},
		{name: "--esc-config", escConfig: true},
		{name: "--esc-config under automation API", automationAPI: "true", escConfig: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_AUTOMATION_API", tt.automationAPI)

			assert.Equal(t, tt.want, NewStackConfigMode(tt.escConfig, tt.staticConfig))
		})
	}
}

func TestStackNewConfigMode(t *testing.T) {
	tests := []struct {
		name          string
		cmd           stackNewCmd
		automationAPI string
		want          apitype.CreateStackConfigMode
		wantReason    string
	}{
		{name: "no flag", want: apitype.CreateStackConfigModeDefault},
		{name: "--static-config", cmd: stackNewCmd{staticConfig: true}, want: apitype.CreateStackConfigModeStatic},
		{
			name:       "--copy-config-from",
			cmd:        stackNewCmd{stackToCopy: "dev"},
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "--copy-config-from is set",
		},
		{
			name: "--copy-config-from with --static-config",
			cmd:  stackNewCmd{stackToCopy: "dev", staticConfig: true},
			want: apitype.CreateStackConfigModeStatic,
		},
		{
			name:          "--copy-config-from under automation API",
			cmd:           stackNewCmd{stackToCopy: "dev"},
			automationAPI: "true",
			want:          apitype.CreateStackConfigModeStatic,
		},
		{name: "--esc-config", cmd: stackNewCmd{escConfig: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_AUTOMATION_API", tt.automationAPI)

			mode, reason := tt.cmd.configMode()
			assert.Equal(t, tt.want, mode)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestResolveDefaultConfigMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configFile string
		ps         workspace.ProjectStack
		opts       CreateStackOptions
		want       apitype.CreateStackConfigMode
		wantReason string
	}{
		{name: "no blocker", want: apitype.CreateStackConfigModeDefault},
		{
			name:       "JSON stack file",
			configFile: "Pulumi.dev.json",
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "its stack file is not YAML",
		},
		{
			name:       "existing config",
			ps:         workspace.ProjectStack{Config: config.Map{config.MustMakeKey("p", "a"): config.NewValue("1")}},
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "Pulumi.dev.yaml already has config or an environment",
		},
		{
			name:       "secrets provider settings",
			ps:         workspace.ProjectStack{EncryptionSalt: "salt"},
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "Pulumi.dev.yaml already has secrets provider settings",
		},
		{
			name:       "non-default secrets provider",
			opts:       CreateStackOptions{SecretsProvider: "passphrase"},
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "a secrets provider is set",
		},
		{
			name: "default secrets provider", opts: CreateStackOptions{SecretsProvider: "default"},
			want: apitype.CreateStackConfigModeDefault,
		},
		{
			name:       "remote config",
			opts:       CreateStackOptions{UseRemoteConfig: true},
			want:       apitype.CreateStackConfigModeStatic,
			wantReason: "remote config is set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := tt.opts
			opts.ConfigFile = tt.configFile
			if opts.ConfigFile == "" {
				opts.ConfigFile = "Pulumi.dev.yaml"
			}
			var createdManaged bool
			mode, reason := resolveDefaultConfigMode(newManagedTestBackend("", &createdManaged),
				&backend.MockStackReference{StringV: "dev", NameV: tokens.MustParseStackName("dev")},
				&tt.ps, opts)

			assert.Equal(t, tt.want, mode)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}
