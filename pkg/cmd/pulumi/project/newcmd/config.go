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

package newcmd

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// HandleConfig handles prompting for config values (as needed) and saving config.
func HandleConfig(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	ws pkgWorkspace.Context,
	prompt promptForValueFunc,
	project *workspace.Project,
	s backend.Stack,
	templateNameOrURL string,
	template cmdTemplates.ProjectTemplate,
	configArray []string,
	yes bool,
	path bool,
	opts display.Options,
	configFile string,
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

	// Handle config.
	// If this is an initial preconfigured empty stack (i.e. configured in the Pulumi Console),
	// use its config without prompting.
	// Otherwise, use the values specified on the command line and prompt for new values.
	// If the stack already existed and had previous config, those values will be used as the defaults.
	var c config.Map
	if isPreconfiguredEmptyStack(templateNameOrURL, template.Config, stackConfig, snap) {
		c = stackConfig
		// TODO[pulumi/pulumi#1894] consider warning if templateNameOrURL is different from
		// the stack's `pulumi:template` config value.
	} else {
		// Get config values passed on the command line.
		commandLineConfig, parseErr := ParseConfig(configArray, path)
		if parseErr != nil {
			return parseErr
		}

		// Prompt for config as needed.
		c, err = promptForConfig(
			ctx,
			sink,
			ssml,
			prompt,
			project,
			s,
			template.Config,
			commandLineConfig,
			stackConfig,
			yes,
			opts,
			configFile,
		)
		if err != nil {
			return err
		}
	}

	// Save the config.
	if len(c) > 0 {
		if err = SaveConfig(ctx, sink, ws, s, c, configFile); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Helper used by multiple commands; output goes to process stdout.
		fmt.Println("Saved config") //nolint:forbidigo
		fmt.Println()               //nolint:forbidigo
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
func promptForConfig(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	prompt promptForValueFunc,
	project *workspace.Project,
	stack backend.Stack,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	yes bool,
	opts display.Options,
	configFile string,
) (config.Map, error) {
	encrypter, decrypter, err := stackCrypters(ctx, sink, ssml, project, stack, configFile)
	if err != nil {
		return nil, err
	}

	values, err := resolveTemplateConfig(project.Name, templateConfig, commandLineConfig, stackConfig, decrypter)
	if err != nil {
		return nil, err
	}
	if err := askTemplateConfig(values, prompt, yes, askAll, opts); err != nil {
		return nil, err
	}

	return encryptTemplateConfig(ctx, encrypter, values, commandLineConfig)
}

// A template config entry, held as plaintext so values can be gathered before a stack exists to
// encrypt secrets.
type templateConfigValue struct {
	// As the template declared it; stable when the project name changes.
	templateKey string
	key         config.Key
	value       string
	secret      bool
	promptText  string // "Description (pretty:key)"
	// Never re-asked: the answer would lose to the --config value at save time.
	fromFlag bool
}

func (v templateConfigValue) unset() bool {
	return !v.fromFlag && v.value == ""
}

func resolveTemplateConfig(
	projectName tokens.PackageName,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	decrypter config.Decrypter,
) ([]templateConfigValue, error) {
	// Convert `string` keys to `config.Key`. If a string key is missing a delimiter,
	// the project name will be prepended.
	templateKeys := make(map[config.Key]string, len(templateConfig))
	keys := make(config.KeyArray, 0, len(templateConfig))
	for k := range templateConfig {
		parsedKey, err := cmdConfig.ParseConfigKeyForProject(projectName, k, false)
		if err != nil {
			return nil, err
		}
		templateKeys[parsedKey] = k
		keys = append(keys, parsedKey)
	}

	// Sort keys. Note that we use the fully qualified module member here instead of a `prettyKey` so that
	// all config values for the current program are prompted one after another.
	sort.Sort(keys)

	values := make([]templateConfigValue, 0, len(keys))
	for _, k := range keys {
		templateKey := templateKeys[k]
		tcv := templateConfig[templateKey]

		promptText := cmdConfig.PrettyKeyForProject(k, projectName)
		if tcv.Description != "" {
			promptText = tcv.Description + " (" + promptText + ")"
		}
		entry := templateConfigValue{
			templateKey: templateKey,
			key:         k,
			secret:      tcv.Secret,
			promptText:  promptText,
		}

		if flagValue, ok := commandLineConfig[k]; ok {
			plain, err := flagValue.Value(config.NopDecrypter)
			if err != nil {
				return nil, err
			}
			entry.value, entry.fromFlag = plain, true
		} else if stackValue, ok := stackConfig[k]; ok {
			// It's OK to pass a nil or non-nil crypter for non-secret values.
			plain, err := stackValue.Value(decrypter)
			if err != nil {
				return nil, err
			}
			entry.value = plain
		}
		if entry.value == "" && !entry.fromFlag {
			entry.value = tcv.Default
		}
		values = append(values, entry)
	}
	return values, nil
}

// The scope of keys askTemplateConfig prompts for; keys fixed by a --config flag are never asked.
type askScope int

const (
	askAll askScope = iota
	askUnset
)

func askTemplateConfig(
	values []templateConfigValue, prompt promptForValueFunc,
	yes bool, scope askScope, opts display.Options,
) error {
	for i, v := range values {
		if v.fromFlag || (scope == askUnset && !v.unset()) {
			continue
		}
		answer, err := prompt(yes, v.promptText, v.value, v.secret, nil, opts)
		if err != nil {
			return err
		}
		values[i].value = answer
	}
	return nil
}

// commandLineConfig is merged as-is so --config values keep their parsed structure and win even
// for keys the template does not declare.
func encryptTemplateConfig(
	ctx context.Context, encrypter config.Encrypter,
	values []templateConfigValue, commandLineConfig config.Map,
) (config.Map, error) {
	c := make(config.Map, len(values)+len(commandLineConfig))
	maps.Copy(c, commandLineConfig)
	for _, v := range values {
		if _, ok := c[v.key]; ok {
			continue
		}
		if v.value == "" {
			// Don't add empty values to the config.
			continue
		}
		if v.secret {
			enc, err := encrypter.EncryptValue(ctx, v.value)
			if err != nil {
				return nil, err
			}
			c[v.key] = config.NewSecureValue(enc)
		} else {
			c[v.key] = config.NewValue(v.value)
		}
	}
	return c, nil
}

func stackCrypters(
	ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader,
	project *workspace.Project, s backend.Stack, configFile string,
) (config.Encrypter, config.Decrypter, error) {
	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, s, configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("loading stack config: %w", err)
	}

	sm, state, err := ssml.GetSecretsManager(ctx, s, ps)
	if err != nil {
		return nil, nil, err
	}
	if state != cmdStack.SecretsManagerUnchanged {
		if err = cmdStack.SaveProjectStack(ctx, s, ps, configFile); err != nil {
			return nil, nil, fmt.Errorf("saving stack config: %w", err)
		}
	}
	return sm.Encrypter(), sm.Decrypter(), nil
}

func saveTemplateConfig(
	ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, ws pkgWorkspace.Context,
	project *workspace.Project, s backend.Stack, values []templateConfigValue,
	commandLineConfig config.Map, configFile string,
) error {
	if len(values) == 0 && len(commandLineConfig) == 0 {
		return nil
	}

	encrypter, _, err := stackCrypters(ctx, sink, ssml, project, s, configFile)
	if err != nil {
		return err
	}
	c, err := encryptTemplateConfig(ctx, encrypter, values, commandLineConfig)
	if err != nil {
		return err
	}
	if len(c) == 0 {
		return nil
	}
	return SaveConfig(ctx, sink, ws, s, c, configFile)
}

// ParseConfig parses the config values passed via command line flags.
// These are passed as `-c aws:region=us-east-1 -c foo:bar=blah` and end up
// in configArray as ["aws:region=us-east-1", "foo:bar=blah"].
// This function converts the array into a config.Map.
func ParseConfig(configArray []string, path bool) (config.Map, error) {
	return parseConfig(configArray, path, func(key string) (config.Key, error) {
		return cmdConfig.ParseConfigKey(pkgWorkspace.Instance, key, path)
	})
}

// ParseConfigForProject is [ParseConfig] for callers that already know the project name.
func ParseConfigForProject(
	projectName tokens.PackageName, configArray []string, path bool,
) (config.Map, error) {
	return parseConfig(configArray, path, func(key string) (config.Key, error) {
		return cmdConfig.ParseConfigKeyForProject(projectName, key, path)
	})
}

func parseConfig(
	configArray []string, path bool, parseKey func(string) (config.Key, error),
) (config.Map, error) {
	configMap := make(config.Map)
	for _, c := range configArray {
		kvp := strings.SplitN(c, "=", 2)

		key, err := parseKey(kvp[0])
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

// SaveConfig saves the config for the stack.
func SaveConfig(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stack backend.Stack, c config.Map, configFile string,
) error {
	project, _, err := ws.ReadProject("")
	if err != nil {
		return err
	}

	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, stack, configFile)
	if err != nil {
		return err
	}

	maps.Copy(ps.Config, c)

	return cmdStack.SaveProjectStack(ctx, stack, ps, configFile)
}
