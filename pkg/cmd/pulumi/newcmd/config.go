// Copyright 2024-2025, Pulumi Corporation.
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

package newcmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// HandleConfig handles prompting for config values (as needed) and saving config.
// Config values are collected with plaintext secrets and then saved via ConfigEditor,
// which handles encryption (local) or fn::secret wrapping (ESC) transparently.
func HandleConfig(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	ws pkgWorkspace.Context,
	prompt promptForValueFunc,
	project *workspace.Project,
	s backend.Stack,
	templateNameOrURL string,
	template workspace.Template,
	configArray []string,
	yes bool,
	path bool,
	opts display.Options,
) error {
	// Get the existing config. stackConfig will be nil if there wasn't a previous deployment.
	latest, err := backend.GetLatestConfiguration(ctx, s)
	if err != nil && err != backenderr.ErrNoPreviousDeployment {
		return err
	}
	stackConfig := latest.Config

	// Get the existing snapshot.
	snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
	if err != nil {
		return err
	}

	// Load the project stack and secrets manager — needed for both prompting (decrypter
	// for existing defaults) and saving (encrypter for the ConfigEditor).
	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, s)
	if err != nil {
		return fmt.Errorf("loading stack config: %w", err)
	}

	sm, state, err := ssml.GetSecretsManager(ctx, s, ps)
	if err != nil {
		return err
	}
	if state != cmdStack.SecretsManagerUnchanged {
		if err = cmdStack.SaveProjectStack(ctx, s, ps); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	// Handle config.
	// If this is an initial preconfigured empty stack (i.e. configured in the Pulumi Console),
	// use its config without prompting.
	// Otherwise, use the values specified on the command line and prompt for new values.
	// If the stack already existed and had previous config, those values will be used as the defaults.
	var c config.Map
	var preconfigured bool
	if isPreconfiguredEmptyStack(templateNameOrURL, template.Config, stackConfig, snap) {
		c = stackConfig
		preconfigured = true
		// TODO[pulumi/pulumi#1894] consider warning if templateNameOrURL is different from
		// the stack's `pulumi:template` config value.
	} else {
		// Get config values passed on the command line.
		commandLineConfig, parseErr := ParseConfig(configArray, path)
		if parseErr != nil {
			return parseErr
		}

		// Prompt for config as needed. Values are returned with plaintext secrets
		// (marked Secure but not encrypted) so the ConfigEditor can handle
		// encryption/wrapping for the appropriate backend.
		c, err = promptForConfig(
			ctx,
			prompt,
			project,
			s,
			template.Config,
			commandLineConfig,
			stackConfig,
			yes,
			sm.Decrypter(),
			opts,
		)
		if err != nil {
			return err
		}
	}

	// Save the config via ConfigEditor, which routes to the local file or ESC
	// environment based on the stack's config location.
	if len(c) > 0 {
		// For preconfigured stacks with service-backed config, the values are already
		// stored in the ESC environment — no need to re-save them.
		if preconfigured && s.ConfigLocation().IsRemote {
			return nil
		}

		editor, editorErr := cmdConfig.NewConfigEditor(ctx, s, ps, sm.Encrypter())
		if editorErr != nil {
			return fmt.Errorf("creating config editor: %w", editorErr)
		}
		for k, v := range c {
			if setErr := editor.Set(ctx, k, v, false); setErr != nil {
				return fmt.Errorf("setting config %v: %w", k, setErr)
			}
		}
		if saveErr := editor.Save(ctx); saveErr != nil {
			return fmt.Errorf("saving config: %w", saveErr)
		}

		fmt.Println("Saved config")
		fmt.Println()
	}

	return nil
}

// isPreconfiguredEmptyStack returns true if the url matches the value of `pulumi:template` in stackConfig,
// the stackConfig values satisfy the config requirements of templateConfig, and the snapshot is empty.
// This is the state of an initial preconfigured empty stack (i.e. a stack that's been created and configured
// in the Pulumi Console).
func isPreconfiguredEmptyStack(
	url string,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	stackConfig config.Map,
	snap *deploy.Snapshot,
) bool {
	// Does stackConfig have a `pulumi:template` value and does it match url?
	if stackConfig == nil {
		return false
	}
	templateURLValue, hasTemplateKey := stackConfig[templateKey]
	if !hasTemplateKey {
		return false
	}
	templateURL, err := templateURLValue.Value(nil)
	if err != nil {
		contract.IgnoreError(err)
		return false
	}
	if templateURL != url {
		return false
	}

	// Does the snapshot only contain a single root resource?
	if len(snap.Resources) != 1 {
		return false
	}
	stackResource, err := stack.GetRootStackResource(snap)
	if err != nil || stackResource == nil {
		return false
	}

	// Can stackConfig satisfy the config requirements of templateConfig?
	for templateKey, templateVal := range templateConfig {
		parsedTemplateKey, parseErr := cmdConfig.ParseConfigKey(pkgWorkspace.Instance, templateKey, false)
		if parseErr != nil {
			contract.IgnoreError(parseErr)
			return false
		}

		stackVal, ok := stackConfig[parsedTemplateKey]
		if !ok {
			return false
		}

		if templateVal.Secret != stackVal.Secure() {
			return false
		}
	}

	return true
}

var templateKey = config.MustMakeKey("pulumi", "template")

// promptForConfig will go through each config key needed by the template and prompt for a value.
// If a config value exists in commandLineConfig, it will be used without prompting.
// If stackConfig is non-nil and a config value exists in stackConfig, it will be used as the default
// value when prompting instead of the default value specified in templateConfig.
//
// Secret values are returned as config.NewSecureValue(plaintext) — encryption is deferred
// to the ConfigEditor so it can handle both local encryption and ESC fn::secret wrapping.
func promptForConfig(
	ctx context.Context,
	prompt promptForValueFunc,
	project *workspace.Project,
	stack backend.Stack,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	yes bool,
	decrypter config.Decrypter,
	opts display.Options,
) (config.Map, error) {
	// Convert `string` keys to `config.Key`. If a string key is missing a delimiter,
	// the project name will be prepended.
	parsedTemplateConfig := make(map[config.Key]workspace.ProjectTemplateConfigValue)
	for k, v := range templateConfig {
		parsedKey, parseErr := cmdConfig.ParseConfigKey(pkgWorkspace.Instance, k, false)
		if parseErr != nil {
			return nil, parseErr
		}
		parsedTemplateConfig[parsedKey] = v
	}

	// Sort keys. Note that we use the fully qualified module member here instead of a `prettyKey` so that
	// all config values for the current program are prompted one after another.
	var keys config.KeyArray
	for k := range parsedTemplateConfig {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	c := make(config.Map)

	for _, k := range keys {
		// If it was passed as a command line flag, use it without prompting.
		if val, ok := commandLineConfig[k]; ok {
			c[k] = val
			continue
		}

		templateConfigValue := parsedTemplateConfig[k]

		// Prepare a default value.
		var defaultValue string
		var secret bool
		if stackConfig != nil {
			// Use the stack's existing value as the default.
			if val, ok := stackConfig[k]; ok {
				// It's OK to pass a nil or non-nil crypter for non-secret values.
				value, err := val.Value(decrypter)
				if err != nil {
					return nil, err
				}
				defaultValue = value
			}
		}
		if defaultValue == "" {
			defaultValue = templateConfigValue.Default
		}
		if !secret {
			secret = templateConfigValue.Secret
		}

		// Prepare the prompt.
		promptText := cmdConfig.PrettyKey(k)
		if templateConfigValue.Description != "" {
			promptText = templateConfigValue.Description + " (" + promptText + ")"
		}

		// Prompt.
		value, err := prompt(yes, promptText, defaultValue, secret, nil, opts)
		if err != nil {
			return nil, err
		}

		if value == "" {
			// Don't add empty values to the config.
			continue
		}

		// Store the value. Secrets are stored as plaintext secure values —
		// the ConfigEditor handles encryption/wrapping on save.
		var v config.Value
		if secret {
			v = config.NewSecureValue(value)
		} else {
			v = config.NewValue(value)
		}

		c[k] = v
	}

	// Add any other config values from the command line.
	for k, v := range commandLineConfig {
		if _, ok := c[k]; !ok {
			c[k] = v
		}
	}

	return c, nil
}

// ParseConfig parses the config values passed via command line flags.
// These are passed as `-c aws:region=us-east-1 -c foo:bar=blah` and end up
// in configArray as ["aws:region=us-east-1", "foo:bar=blah"].
// This function converts the array into a config.Map.
func ParseConfig(configArray []string, path bool) (config.Map, error) {
	configMap := make(config.Map)
	for _, c := range configArray {
		kvp := strings.SplitN(c, "=", 2)

		key, err := cmdConfig.ParseConfigKey(pkgWorkspace.Instance, kvp[0], path)
		if err != nil {
			return nil, err
		}

		value := config.NewValue("")
		if len(kvp) == 2 {
			value = config.NewValue(kvp[1])
		}

		if err = configMap.Set(key, value, path); err != nil {
			return nil, err
		}
	}
	return configMap, nil
}

// SaveConfig saves the config for the stack. For service-backed stacks, values are
// written to the ESC environment via ConfigEditor. For local stacks, values are written
// to the Pulumi.<stack>.yaml file directly (values are assumed to be non-secret or
// already encrypted).
func SaveConfig(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stack backend.Stack, c config.Map) error {
	project, _, err := ws.ReadProject()
	if err != nil {
		return err
	}

	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, stack)
	if err != nil {
		return err
	}

	if stack.ConfigLocation().IsRemote {
		editor, editorErr := cmdConfig.NewConfigEditor(ctx, stack, ps, config.NopEncrypter)
		if editorErr != nil {
			return editorErr
		}
		for k, v := range c {
			if setErr := editor.Set(ctx, k, v, false); setErr != nil {
				return fmt.Errorf("setting config %v: %w", k, setErr)
			}
		}
		return editor.Save(ctx)
	}

	for k, v := range c {
		ps.Config[k] = v
	}

	return cmdStack.SaveProjectStack(ctx, stack, ps)
}
