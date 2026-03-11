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

package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigEnvEjectCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvEjectCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "eject",
		Short: "Eject a stack from service-backed config to a local config file",
		Long: `Removes the service-backed configuration link from a stack and writes
all current config values to a local Pulumi.<stack>.yaml file.

By default, the linked ESC environment is also deleted. Use --keep-env to preserve it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().BoolVar(&impl.keepEnv, "keep-env", false, "Keep the ESC environment after ejecting (do not delete it)")
	cmd.Flags().StringVar(&impl.secretsProvider, "secrets-provider", "",
		"Secrets provider for encrypting secrets in the local config file (e.g. passphrase, awskms://...)")
	cmd.Flags().BoolVarP(&impl.yes, "yes", "y", false, "Proceed without confirmation")

	return cmd
}

type configEnvEjectCmd struct {
	parent *configEnvCmd

	keepEnv         bool
	secretsProvider string
	yes             bool
}

func (cmd *configEnvEjectCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmd.parent.color}

	stack, err := cmd.parent.requireStack(
		ctx,
		cmd.parent.diags,
		cmd.parent.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.parent.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return errors.New("this stack does not use service-backed configuration")
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	orgName := stack.(interface{ OrgName() string }).OrgName()
	envProject, envName, _ := strings.Cut(*loc.EscEnv, "/")

	// Load the ESC environment YAML with decryption to get plaintext secret values.
	yamlBytes, _, _, getErr := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", true)
	if getErr != nil {
		// If the environment is gone, continue — we still remove the link.
		fmt.Fprintf(cmd.parent.stdout,
			"Warning: could not load ESC environment %s: %v\nProceeding with link removal only.\n",
			*loc.EscEnv, getErr)
		yamlBytes = nil
	}

	// Parse config values from the YAML definition.
	configValues, hasSecrets, parseErr := extractPulumiConfig(yamlBytes)
	if parseErr != nil {
		return fmt.Errorf("parsing ESC environment: %w", parseErr)
	}

	// If secrets are present, a local secrets provider is required for re-encryption.
	if hasSecrets && cmd.secretsProvider == "" {
		if !cmd.parent.interactive {
			return errors.New(
				"--secrets-provider is required when ejecting a stack with secrets in non-interactive mode")
		}
		provider, readErr := cmdutil.ReadConsole(
			"Enter a secrets provider for the local config file (e.g. passphrase, awskms://...)")
		if readErr != nil {
			return readErr
		}
		cmd.secretsProvider = strings.TrimSpace(provider)
	}

	// Show a confirmation prompt describing what will happen.
	if !cmd.yes {
		fmt.Fprintf(cmd.parent.stdout, "This will eject stack %q from service-backed config:\n",
			stack.Ref().Name())
		if len(configValues) > 0 {
			fmt.Fprintf(cmd.parent.stdout, "  • %d config value(s) will be written to Pulumi.%s.yaml\n",
				len(configValues), stack.Ref().Name())
		} else {
			fmt.Fprintf(cmd.parent.stdout, "  • No config values to write (empty environment)\n")
		}
		fmt.Fprintf(cmd.parent.stdout, "  • Stack will be unlinked from ESC environment %s\n", *loc.EscEnv)
		if !cmd.keepEnv {
			fmt.Fprintf(cmd.parent.stdout, "  • ESC environment %s will be deleted\n", *loc.EscEnv)
		}
		response := ui.PromptUser("Proceed?", []string{"yes", "no"}, "yes", cmd.parent.color)
		if response == "no" {
			return errors.New("canceled")
		}
	}

	// Build the local ProjectStack from the extracted config values.
	ps := &workspace.ProjectStack{
		Config: make(config.Map),
	}

	var encrypter config.Encrypter
	if hasSecrets {
		ps.SecretsProvider = cmd.secretsProvider
		enc, _, encErr := cmd.parent.ssml.GetEncrypter(ctx, stack, ps)
		if encErr != nil {
			return fmt.Errorf("setting up secrets provider: %w", encErr)
		}
		encrypter = enc
	}

	for k, v := range configValues {
		key, keyErr := config.ParseKey(k)
		if keyErr != nil {
			return fmt.Errorf("invalid config key %q: %w", k, keyErr)
		}
		if v.secret {
			ciphertext, encErr := encrypter.EncryptValue(ctx, v.plaintext)
			if encErr != nil {
				return fmt.Errorf("encrypting config value %q: %w", k, encErr)
			}
			if setErr := ps.Config.Set(key, config.NewSecureValue(ciphertext), false); setErr != nil {
				return setErr
			}
		} else {
			if setErr := ps.Config.Set(key, config.NewValue(v.plaintext), false); setErr != nil {
				return setErr
			}
		}
	}

	// Write the local config file. Bypass cmdStack.SaveProjectStack to avoid the IsRemote redirect.
	if saveErr := workspace.SaveProjectStack(stack.Ref().Name().Q(), ps); saveErr != nil {
		return fmt.Errorf("writing local config file: %w", saveErr)
	}

	// Remove the service-backed link from the stack.
	if removeErr := stack.RemoveRemoteConfig(ctx); removeErr != nil {
		return fmt.Errorf("removing service-backed config link: %w", removeErr)
	}

	fmt.Fprintf(cmd.parent.stdout, "Ejected stack %q from service-backed config.\n", stack.Ref().Name())
	if len(configValues) > 0 {
		fmt.Fprintf(cmd.parent.stdout, "Config values written to Pulumi.%s.yaml.\n", stack.Ref().Name())
	}

	// Delete the ESC environment unless --keep-env was passed.
	if !cmd.keepEnv {
		if delErr := envBackend.DeleteEnvironmentWithProject(ctx, orgName, envProject, envName); delErr != nil {
			fmt.Fprintf(cmd.parent.stdout, "Warning: failed to delete ESC environment %s: %v\n",
				*loc.EscEnv, delErr)
		} else {
			fmt.Fprintf(cmd.parent.stdout, "Deleted ESC environment %s.\n", *loc.EscEnv)
		}
	}

	return nil
}

// ejectedConfigValue holds a single config value extracted from an ESC environment YAML definition.
type ejectedConfigValue struct {
	plaintext string
	secret    bool
}

// extractPulumiConfig parses the values.pulumiConfig section of an ESC environment YAML definition.
// When the YAML was fetched with decrypt=true, fn::secret values contain the actual plaintext.
func extractPulumiConfig(yamlBytes []byte) (map[string]ejectedConfigValue, bool, error) {
	if len(yamlBytes) == 0 {
		return map[string]ejectedConfigValue{}, false, nil
	}

	var envDef map[string]any
	if err := yaml.Unmarshal(yamlBytes, &envDef); err != nil {
		return nil, false, err
	}

	valuesRaw, ok := envDef["values"]
	if !ok {
		return map[string]ejectedConfigValue{}, false, nil
	}
	values, ok := valuesRaw.(map[string]any)
	if !ok {
		return map[string]ejectedConfigValue{}, false, nil
	}

	pulumiConfigRaw, ok := values["pulumiConfig"]
	if !ok {
		return map[string]ejectedConfigValue{}, false, nil
	}
	pulumiConfig, ok := pulumiConfigRaw.(map[string]any)
	if !ok {
		return map[string]ejectedConfigValue{}, false, nil
	}

	result := make(map[string]ejectedConfigValue, len(pulumiConfig))
	hasSecrets := false

	for k, v := range pulumiConfig {
		switch val := v.(type) {
		case string:
			result[k] = ejectedConfigValue{plaintext: val}
		case map[string]any:
			if plaintext, hasFnSecret := val["fn::secret"]; hasFnSecret {
				if s, ok := plaintext.(string); ok {
					result[k] = ejectedConfigValue{plaintext: s, secret: true}
					hasSecrets = true
				}
			}
			// Nested map values (non-secret objects) are not representable as flat config — skip.
		default:
			// int, float, bool — convert to string.
			result[k] = ejectedConfigValue{plaintext: fmt.Sprintf("%v", v)}
		}
	}

	return result, hasSecrets, nil
}
