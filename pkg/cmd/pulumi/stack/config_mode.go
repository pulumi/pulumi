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
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	escConfigFlag    = "esc-config"
	staticConfigFlag = "static-config"
)

// AddConfigModeFlags adds the --esc-config and --static-config flags to cmd, mutually exclusive with each other and
// with each flag in exclusiveWith.
func AddConfigModeFlags(cmd *cobra.Command, escConfig, staticConfig *bool, exclusiveWith ...string) {
	cmd.PersistentFlags().BoolVar(
		escConfig, escConfigFlag, false,
		"Create an ESC environment, managed by the stack, that holds the stack's configuration. "+
			"ESC encrypts secret config values; --secrets-provider applies only to secrets in the stack's state",
	)
	cmd.PersistentFlags().BoolVar(
		staticConfig, staticConfigFlag, false,
		"Keep the stack's configuration in its stack file, "+
			"even if the organization defaults to an ESC environment",
	)
	cmd.MarkFlagsMutuallyExclusive(escConfigFlag, staticConfigFlag)
	for _, flag := range []string{escConfigFlag, staticConfigFlag} {
		if !configModeFlagsVisible() {
			_ = cmd.PersistentFlags().MarkHidden(flag)
		}
		for _, other := range exclusiveWith {
			cmd.MarkFlagsMutuallyExclusive(flag, other)
		}
	}
}

func configModeFlagsVisible() bool {
	return true
}

func followsOrgConfigDefault() bool {
	return !env.AutomationAPI.Value()
}

// NewStackConfigMode returns the CreateStackOptions.ConfigMode for a stack creation command. It is empty for
// --esc-config, which CreateStackOptions.ManagedEnvironment carries so that it fails closed.
func NewStackConfigMode(escConfig, staticConfig bool) apitype.CreateStackConfigMode {
	switch {
	case escConfig:
		return ""
	case !staticConfig && followsOrgConfigDefault():
		return apitype.CreateStackConfigModeDefault
	default:
		return apitype.CreateStackConfigModeStatic
	}
}

// resolveDefaultConfigMode returns the mode to send for a stack that follows the organization default, and, when a
// local blocker makes it static, the reason to report if the organization defaults to ESC.
func resolveDefaultConfigMode(
	b backend.Backend, stackRef backend.StackReference, ps *workspace.ProjectStack, opts CreateStackOptions,
) (apitype.CreateStackConfigMode, string) {
	if _, err := backend.AsManagedEnvironmentsBackend(b); err != nil {
		return "", ""
	}
	path, err := managedStackFilePath(stackRef.Name().Q(), opts.ConfigFile)
	if errors.Is(err, errManagedEnvironmentRequiresYAML) {
		return apitype.CreateStackConfigModeStatic, "its stack file is not YAML"
	}
	if err != nil {
		return apitype.CreateStackConfigModeStatic, ""
	}
	file := filepath.Base(path)
	switch {
	case len(ps.Config) > 0 || ps.Environment != nil:
		return apitype.CreateStackConfigModeStatic, file + " already has config or an environment"
	case ps.SecretsProvider != "" || ps.EncryptionSalt != "" || ps.EncryptedKey != "":
		return apitype.CreateStackConfigModeStatic, file + " already has secrets provider settings"
	case opts.SecretsProvider != "" && opts.SecretsProvider != "default":
		return apitype.CreateStackConfigModeStatic, "a secrets provider is set"
	case opts.UseRemoteConfig:
		return apitype.CreateStackConfigModeStatic, "remote config is set"
	}
	return apitype.CreateStackConfigModeDefault, ""
}
