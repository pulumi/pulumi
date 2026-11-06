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
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/stackenv"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// OverrideEnvFlag registers the --override-env flag on cmd, binding it to overrides.
func OverrideEnvFlag(cmd *cobra.Command, overrides *[]string) {
	cmd.PersistentFlags().StringArrayVar(
		overrides, "override-env", nil,
		"[EXPERIMENTAL] Override an imported environment for this run only, as <env>=<replacement>; repeatable")
}

// Attempts to load configuration for the given stack.
func GetStackConfiguration(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	project *workspace.Project,
	configFile string,
	envOverrides []string,
) (backend.StackConfiguration, secrets.Manager, error) {
	return getStackConfigurationWithFallback(ctx, sink, ssml, stack, project, nil, configFile, envOverrides)
}

var errDestroyManagedOutsideProject = cmdStack.ManagedEnvironmentNotSupportedError(
	"`pulumi destroy` outside the project directory")

// GetStackConfigurationOrLatest attempts to load a current stack configuration
// using getStackConfiguration. If that fails due to not being run within a
// valid project, the latest configuration from the backend is returned. This is
// primarily for use in commands like `pulumi destroy`, where it is useful to be
// able to clean up a stack whose configuration has already been deleted as part
// of that cleanup.
func GetStackConfigurationOrLatest(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	project *workspace.Project,
	configFile string,
	envOverrides []string,
) (backend.StackConfiguration, secrets.Manager, error) {
	return getStackConfigurationWithFallback(
		ctx, sink, ssml, stack, project,
		func(err error) (config.Map, error) {
			if errors.Is(err, workspace.ErrProjectNotFound) {
				if stack.ConfigLocation().ManagedEnvironment != "" {
					return nil, errDestroyManagedOutsideProject
				}
				// This error indicates that we're not being run in a project directory.
				// We should fallback on the backend.
				latest, err := backend.GetLatestConfiguration(ctx, stack)
				return latest.Config, err
			}
			return nil, err
		},
		configFile, envOverrides)
}

func getStackConfigurationWithFallback(
	ctx context.Context,
	sink diag.Sink,
	ssml cmdStack.SecretsManagerLoader,
	s backend.Stack,
	project *workspace.Project,
	fallbackGetConfig func(err error) (config.Map, error), // optional
	configFile string,
	envOverrides []string,
) (backend.StackConfiguration, secrets.Manager, error) {
	workspaceStack, err := cmdStack.LoadProjectStack(ctx, sink, project, s, configFile)
	configIsLocal := true
	if err != nil || workspaceStack == nil {
		if fallbackGetConfig == nil {
			return backend.StackConfiguration{}, nil, err
		}
		// On first run or the latest configuration is unavailable, fallback to check the project's configuration
		cfg, err := fallbackGetConfig(err)
		if err != nil {
			return backend.StackConfiguration{}, nil, fmt.Errorf(
				"stack configuration could not be loaded from either Pulumi.yaml or the backend: %w", err,
			)
		}
		workspaceStack = &workspace.ProjectStack{
			Config: cfg,
		}
		configIsLocal = false
	}

	sm, err := getAndSaveSecretsManager(ctx, ssml, s, workspaceStack, configFile)
	if err != nil {
		return backend.StackConfiguration{}, nil, err
	}

	config, err := getStackConfigurationFromProjectStack(ctx, s, project, sm, workspaceStack, envOverrides, configIsLocal)
	if err != nil {
		return backend.StackConfiguration{}, nil, err
	}
	return config, sm, nil
}

// getStackConfigurationFromProjectStack builds the stack's configuration. configIsLocal says whether workspaceStack's
// config is the stack's own, rather than a copy of what its latest update ran with.
func getStackConfigurationFromProjectStack(
	ctx context.Context,
	stack backend.Stack,
	project *workspace.Project,
	sm secrets.Manager,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
	configIsLocal bool,
) (backend.StackConfiguration, error) {
	env, diags, managedEnv, deliveryNotice, err := openStackEnvWithManaged(
		ctx, stack, workspaceStack, envOverrides, configIsLocal)
	if err != nil {
		return backend.StackConfiguration{}, fmt.Errorf("opening environment: %w", err)
	}
	if len(diags) != 0 {
		// This is a non-command helper; we don't have a *cobra.Command writer
		// to thread through here. Writes go to the process streams directly.
		printESCDiagnostics(os.Stderr, diags) //nolint:forbidigo
		return backend.StackConfiguration{}, errors.New("opening environment: too many errors")
	}
	var managedRevision *apitype.UpdateProgramEnvironment
	var managedNotice string
	if managedEnv != nil {
		managedRevision = managedEnv.update
		managedNotice = managedEnvironmentNotice(stack, managedEnv)
	}

	var pulumiEnv esc.Value
	if env != nil {
		warnOnNoEnvironmentEffects(os.Stdout, env) //nolint:forbidigo

		pulumiEnv = env.Properties["pulumiConfig"]

		_, environ, secrets, _, err := cli.PrepareEnvironment(env, nil)
		if err != nil {
			return backend.StackConfiguration{}, fmt.Errorf("preparing environment: %w", err)
		}
		if len(secrets) != 0 {
			logging.AddGlobalSecretFilter(secrets, "[secret]")
		}

		for _, kvp := range environ {
			if name, value, ok := strings.Cut(kvp, "="); ok {
				if err := os.Setenv(name, value); err != nil {
					return backend.StackConfiguration{}, fmt.Errorf("setting environment variable %v: %w", name, err)
				}
			}
		}
	}

	imports := stackEnvironmentImports(workspaceStack, managedEnv != nil)

	// If there are no secrets in the configuration, we should never use the decrypter, so it is safe to return
	// one which panics if it is used. This provides for some nice UX in the common case (since, for example, building
	// the correct decrypter for the diy backend would involve prompting for a passphrase)
	if !needsCrypter(workspaceStack.Config, pulumiEnv) {
		return backend.StackConfiguration{
			EnvironmentImports:        imports,
			Environment:               pulumiEnv,
			Config:                    workspaceStack.Config,
			Decrypter:                 config.NewPanicCrypter(),
			ManagedEnvironment:        managedRevision,
			ManagedEnvironmentNotice:  managedNotice,
			DeliveryEnvironment:       stack.ConfigLocation().DeliveryEnvironment,
			DeliveryEnvironmentNotice: deliveryNotice,
		}, nil
	}

	crypter := sm.Decrypter()

	return backend.StackConfiguration{
		EnvironmentImports:        imports,
		Environment:               pulumiEnv,
		Config:                    workspaceStack.Config,
		Decrypter:                 crypter,
		ManagedEnvironment:        managedRevision,
		ManagedEnvironmentNotice:  managedNotice,
		DeliveryEnvironment:       stack.ConfigLocation().DeliveryEnvironment,
		DeliveryEnvironmentNotice: deliveryNotice,
	}, nil
}

func getAndSaveSecretsManager(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	configFile string,
) (secrets.Manager, error) {
	sm, state, err := ssml.GetSecretsManager(ctx, stack, workspaceStack)
	if err != nil {
		return nil, fmt.Errorf("get stack secrets manager: %w", err)
	}
	if state != cmdStack.SecretsManagerUnchanged {
		err = cmdStack.SaveProjectStack(ctx, stack, workspaceStack, configFile)
		if err != nil && state == cmdStack.SecretsManagerMustSave {
			return nil, fmt.Errorf("save stack config: %w", err)
		}
	}
	return sm, nil
}

func needsCrypter(cfg config.Map, env esc.Value) bool {
	var hasSecrets func(v esc.Value) bool
	hasSecrets = func(v esc.Value) bool {
		if v.Secret {
			return true
		}
		switch v := v.Value.(type) {
		case []esc.Value:
			if slices.ContainsFunc(v, hasSecrets) {
				return true
			}
		case map[string]esc.Value:
			for _, v := range v {
				if hasSecrets(v) {
					return true
				}
			}
		}
		return false
	}

	return cfg.HasSecureValue() || hasSecrets(env)
}

type managedEnvironment struct {
	update    *apitype.UpdateProgramEnvironment
	local     []byte
	published []byte
}

func managedEnvironmentNotice(stack backend.Stack, managedEnv *managedEnvironment) string {
	if managedEnv.update.ChangeRequestID == "" {
		return ""
	}
	name, revision := stack.ConfigLocation().ManagedEnvironment, managedEnv.update.BaseRevision
	changes, err := stackenv.DiffDefinitions(managedEnv.local, managedEnv.published)
	if err != nil {
		slog.Debug("diffing managed environment definitions", "environment", name, "err", err)
		return fmt.Sprintf(
			"using local changes to environment %s (based on revision %d). "+
				"up, refresh and destroy publish them; preview doesn't.\n",
			name, revision)
	}
	var notice strings.Builder
	stackenv.WriteDefinitionDiff(&notice, colors.Raw, name, revision, changes)
	return notice.String()
}

func openStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	env, diags, _, _, err := openStackEnvWithManaged(ctx, stack, workspaceStack, envOverrides, true)
	return env, diags, err
}

// openStackEnvWithManaged opens the stack's environment and composes its Pulumi Delivery environment with it. It also
// returns the stack's managed environment, if any, and the notice that the stack's engine operations show about
// Delivery.
func openStackEnvWithManaged(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
	configIsLocal bool,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, *managedEnvironment, string, error) {
	env, diags, managedEnv, err := openOwnStackEnv(ctx, stack, workspaceStack, envOverrides)
	if err != nil || len(diags) != 0 {
		return nil, diags, nil, "", err
	}
	env, notice, diags, err := composeDeliveryEnvironment(ctx, stack, workspaceStack, env, configIsLocal)
	if err != nil || len(diags) != 0 {
		return nil, diags, nil, "", err
	}
	return env, nil, managedEnv, notice, nil
}

// composeDeliveryEnvironment adds the pulumiConfig of the stack's Pulumi Delivery environment, if it has one, to env,
// and returns the notice about it. The notice warns about the keys the stack's own config also sets only when
// configIsLocal.
func composeDeliveryEnvironment(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	env *esc.Environment,
	configIsLocal bool,
) (*esc.Environment, string, []apitype.EnvironmentDiagnostic, error) {
	loc := stack.ConfigLocation()
	delivery := loc.DeliveryEnvironment
	if delivery == nil {
		return env, "", nil, nil
	}

	envs, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, "", nil, errBackendNoEnvironments(stack.Backend())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, "", nil, fmt.Errorf("cannot determine organization for stack %v", stack.Ref())
	}
	project, name, ok := strings.Cut(delivery.Environment, "/")
	if !ok {
		return nil, "", nil, fmt.Errorf(
			"stack %v has an invalid Pulumi Delivery environment %q", stack.Ref(), delivery.Environment)
	}

	deliveryEnv, diags, err := envs.OpenEnvironmentRevision(
		ctx, orgNamer.OrgName(), project, name, delivery.Revision, 2*time.Hour)
	if err != nil {
		return nil, "", nil, fmt.Errorf("opening the config Pulumi Delivery holds for stack %v: %w", stack.Ref(), err)
	}
	if len(diags) != 0 {
		return nil, "", diags, nil
	}

	deliveryConfig, _ := deliveryEnv.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	if len(deliveryConfig) == 0 {
		return env, "", nil, nil
	}

	var envConfig map[string]esc.Value
	if env != nil {
		envConfig, _ = env.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	}
	// A managed environment holds the stack's config in place of its file, and so does the one environment of a stack
	// whose config is stored remotely.
	envIsOwn := loc.ManagedEnvironment != "" || loc.IsRemote
	merged, both, err := mergeDeliveryConfig(stack, deliveryConfig, envConfig, envIsOwn, workspaceStack.Config)
	if err != nil {
		return nil, "", nil, err
	}

	if !configIsLocal {
		// Config copied from the latest update holds the values Delivery gave that update.
		both = nil
	}
	notice := formatDeliveryNotice(deliveryEnv, delivery, both)

	if env == nil {
		env = &esc.Environment{}
	}
	if env.Properties == nil {
		env.Properties = map[string]esc.Value{}
	}
	env.Properties["pulumiConfig"] = esc.NewValue(merged)
	return env, notice, nil, nil
}

// mergeDeliveryConfig merges Delivery's config into the stack's environment config the way the stack's file merges
// over its environment: objects merge key by key, and any other value replaces the one beneath it. Delivery's config
// goes over the environment's, unless envIsOwn, when the environment holds the stack's own config and goes over
// Delivery's. It returns the merged config and the keys that Delivery and the stack's own config both set.
func mergeDeliveryConfig(
	stack backend.Stack,
	deliveryConfig map[string]esc.Value,
	envConfig map[string]esc.Value,
	envIsOwn bool,
	fileConfig config.Map,
) (map[string]esc.Value, []config.Key, error) {
	project, _ := stack.Ref().Project()
	qualify := func(rawKey string) (config.Key, error) {
		return ParseConfigKeyForProject(tokens.PackageName(project), rawKey, false)
	}

	merged := maps.Clone(envConfig)
	if merged == nil {
		merged = make(map[string]esc.Value, len(deliveryConfig))
	}
	envRawKeys := make(map[config.Key]string, len(envConfig))
	for rawKey := range envConfig {
		// A key that doesn't parse fails the update when it is merged, with a better error than this one could give.
		if key, err := qualify(rawKey); err == nil {
			envRawKeys[key] = rawKey
		}
	}

	var both []config.Key
	for rawKey, value := range deliveryConfig {
		key, err := qualify(rawKey)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"stack %v: Pulumi Delivery holds an invalid config key %q: %w", stack.Ref(), rawKey, err)
		}
		envRawKey, inEnv := envRawKeys[key]
		if _, inFile := fileConfig[key]; inFile || (inEnv && envIsOwn) {
			both = append(both, key)
		}
		if inEnv {
			envValue := merged[envRawKey]
			// The environment may spell the key bare, so keep one spelling rather than letting both race.
			delete(merged, envRawKey)
			if envIsOwn {
				value = mergeConfigValue(envValue, value)
			} else {
				value = mergeConfigValue(value, envValue)
			}
		}
		merged[rawKey] = value
	}
	slices.SortFunc(both, func(a, b config.Key) int { return strings.Compare(a.String(), b.String()) })
	return merged, both, nil
}

// mergeConfigValue merges value over base with JSON merge patch semantics, as config.Value.Merge does.
func mergeConfigValue(value, base esc.Value) esc.Value {
	object, ok := value.Value.(map[string]esc.Value)
	baseObject, baseOK := base.Value.(map[string]esc.Value)
	if !ok || !baseOK {
		return value
	}
	merged := maps.Clone(baseObject)
	for key, v := range object {
		if b, ok := merged[key]; ok {
			v = mergeConfigValue(v, b)
		}
		merged[key] = v
	}
	result := esc.NewValue(merged)
	result.Secret = value.Secret || base.Secret
	return result
}

// formatDeliveryNotice says which revision the run composed, and warns about each key of the stack's own config that
// Delivery also sets. It is text with colorization tags.
func formatDeliveryNotice(
	deliveryEnv *esc.Environment,
	delivery *apitype.StackDeliveryEnvironment,
	both []config.Key,
) string {
	source := "Pulumi Delivery"
	if pipeline := deliveryPipeline(deliveryEnv); pipeline != "" {
		source = "Pulumi Delivery (pipeline " + pipeline + ")"
	}
	notice := fmt.Sprintf("using config from %s, revision %d\n", source, delivery.Revision)
	if len(both) == 0 {
		return notice
	}

	keys := make([]string, len(both))
	for i, key := range both {
		keys[i] = key.String()
	}
	return notice + colors.SpecWarning + fmt.Sprintf("warning: this stack sets config that %s also sets: %s. "+
		"The stack's values win over the pipeline's, in Delivery runs too. Remove these keys from the stack's config "+
		"to use the pipeline's values.", source, strings.Join(keys, ", ")) + colors.Reset + "\n"
}

// deliveryPipeline returns the pipeline that the environment's revision says it came from, if it says.
func deliveryPipeline(deliveryEnv *esc.Environment) string {
	provenance, _ := deliveryEnv.Properties["delivery"].Value.(map[string]esc.Value)
	pipeline, _ := provenance["pipeline"].Value.(string)
	return pipeline
}

func openOwnStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, *managedEnvironment, error) {
	loc := stack.ConfigLocation()
	if loc.ManagedEnvironment == "" {
		env, diags, err := openUnmanagedStackEnv(ctx, stack, workspaceStack, envOverrides)
		return env, diags, nil, err
	}

	org, definition, err := managedEnvironmentDefinition(stack, workspaceStack)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(envOverrides) != 0 {
		return nil, nil, nil, cmdStack.ManagedEnvironmentNotSupportedError("--override-env")
	}
	project, name := loc.ManagedEnvironmentRef()

	managed, err := backend.AsManagedEnvironmentsBackend(stack.Backend())
	if err != nil {
		return nil, nil, nil, err
	}
	evaluation, diags, err := managed.EvaluateManagedEnvironment(ctx, org, project, name, definition)
	if err != nil || len(diags) != 0 {
		return nil, diags, nil, err
	}
	return evaluation.Environment, nil, &managedEnvironment{
		update: &apitype.UpdateProgramEnvironment{
			BaseRevision:    evaluation.BaseRevision,
			ChangeRequestID: evaluation.ChangeRequestID,
			DraftRevision:   evaluation.DraftRevision,
		},
		local:     definition,
		published: evaluation.PublishedDefinition,
	}, nil
}

func managedEnvironmentDefinition(
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
) (string, []byte, error) {
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return "", nil, fmt.Errorf("cannot determine organization for stack %v", stack.Ref())
	}
	org := orgNamer.OrgName()
	node, err := cmdStack.ManagedEnvironmentDefinition(
		stack.ConfigLocation(), workspaceStack, org, stack.Ref().Name().String(), "")
	if err != nil {
		return "", nil, err
	}
	definition, err := yaml.Marshal(node)
	return org, definition, err
}

func openUnmanagedStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	yaml := workspaceStack.EnvironmentBytes()
	if len(yaml) == 0 {
		return nil, nil, nil
	}

	overrides, err := parseEnvironmentOverrides(envOverrides)
	if err != nil {
		return nil, nil, err
	}

	envs, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, errBackendNoEnvironments(stack.Backend())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, nil, fmt.Errorf("cannot determine organzation for stack %v", stack.Ref())
	}
	orgName := orgNamer.OrgName()

	return envs.OpenYAMLEnvironment(ctx, orgName, yaml, 2*time.Hour, overrides)
}

// parseEnvironmentOverrides converts <env>=<replacement> pairs into a map sent to ESC,
// which substitutes the matching imports while resolving the environment import graph.
func parseEnvironmentOverrides(envOverrides []string) (map[string]string, error) {
	if len(envOverrides) == 0 {
		return nil, nil
	}
	overrides := make(map[string]string, len(envOverrides))
	for _, o := range envOverrides {
		match, replacement, ok := strings.Cut(o, "=")
		if !ok || match == "" || replacement == "" {
			return nil, fmt.Errorf("invalid --override-env value %q: expected format <env>=<replacement>", o)
		}
		overrides[match] = replacement
	}
	return overrides, nil
}

func copySingleConfigKey(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	configKey string,
	path bool,
	currentStack backend.Stack,
	currentProjectStack *workspace.ProjectStack,
	destinationStack backend.Stack,
	destinationProjectStack *workspace.ProjectStack,
	configFile string,
) error {
	var decrypter config.Decrypter
	key, err := ParseConfigKey(pkgWorkspace.Instance, configKey, path)
	if err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	v, ok, err := currentProjectStack.Config.Get(key, path)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("configuration key '%s' not found for stack '%s'", PrettyKey(key), currentStack.Ref())
	}

	if v.Secure() {
		var err error
		var state cmdStack.SecretsManagerState
		if decrypter, state, err = ssml.GetDecrypter(ctx, currentStack, currentProjectStack); err != nil {
			return fmt.Errorf("could not create a decrypter: %w", err)
		}
		contract.Assertf(
			state == cmdStack.SecretsManagerUnchanged,
			"We're reading a secure value so the encryption information must be present already",
		)
	} else {
		decrypter = config.NewPanicCrypter()
	}

	encrypter, _, cerr := ssml.GetEncrypter(ctx, destinationStack, destinationProjectStack)
	if cerr != nil {
		return cerr
	}

	val, err := v.Copy(decrypter, encrypter)
	if err != nil {
		return err
	}

	err = destinationProjectStack.Config.Set(key, val, path)
	if err != nil {
		return err
	}

	return cmdStack.SaveProjectStack(ctx, destinationStack, destinationProjectStack, configFile)
}

func parseKeyValuePair(pair string, path bool) (config.Key, string, error) {
	// Split the arg on the first '=' to separate key and value.
	splitArg := strings.SplitN(pair, "=", 2)

	// Check if the key is wrapped in quote marks and split on the '=' following the wrapping quote.
	firstChar := string([]rune(pair)[0])
	if firstChar == "\"" || firstChar == "'" {
		pair = strings.TrimPrefix(pair, firstChar)
		splitArg = strings.SplitN(pair, firstChar+"=", 2)
	}

	if len(splitArg) < 2 {
		return config.Key{}, "", errors.New("config value must be in the form [key]=[value]")
	}
	key, err := ParseConfigKey(pkgWorkspace.Instance, splitArg[0], path)
	if err != nil {
		return config.Key{}, "", fmt.Errorf("invalid configuration key: %w", err)
	}

	value := splitArg[1]
	return key, value, nil
}

// ParseConfigKey converts a given key string to a config.Key.
// Depending on whether the key is a path or not, the same string can either
// be valid or not, and also parse to different keys. For example:
// foo.bar:buzz is a (namespace: foo.bar, key: buzz) if not path, and
// (namespace: <project-name>, key: foo.bar:buzz) if path.
func ParseConfigKey(ws pkgWorkspace.Context, key string, path bool) (config.Key, error) {
	return parseConfigKey(key, path, func() (tokens.PackageName, error) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting current working directory: %w", err)
		}

		proj, _, err := ws.ReadProject(cwd)
		if err != nil {
			return "", err
		}
		return proj.Name, nil
	})
}

// ParseConfigKeyForProject is [ParseConfigKey] for callers that already know the project name.
func ParseConfigKeyForProject(projectName tokens.PackageName, key string, path bool) (config.Key, error) {
	return parseConfigKey(key, path, func() (tokens.PackageName, error) { return projectName, nil })
}

func parseConfigKey(key string, path bool, projectName func() (tokens.PackageName, error)) (config.Key, error) {
	// If the key is a path, the namespacing requirement only applies to the
	// top-level key, while sub-keys may have arbitrary names.
	if path {
		// Figure out if the key has multiple segments (delimetered by dots and square brackets).
		// If they exist, we'll parse the first segment to validate its structure and obtain its
		// namespace. Then we'll use this namespace with the rest of the key.
		bracketOrDotIndex := strings.IndexAny(key, "[.")
		if bracketOrDotIndex > 0 {
			topSegment := key[:bracketOrDotIndex]
			topKey, err := parseConfigKey(topSegment, false, projectName)
			if err != nil {
				return config.Key{}, err
			}
			return config.MustMakeKey(topKey.Namespace(), topKey.Name()+key[bracketOrDotIndex:]), nil
		}
	}

	// As a convenience, we'll treat any key with no delimiter as if:
	// <program-name>:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		name, err := projectName()
		if err != nil {
			return config.Key{}, err
		}

		return config.ParseKey(fmt.Sprintf("%s:%s", name, key))
	}

	return config.ParseKey(key)
}

func PrettyKey(k config.Key) string {
	proj, err := pkgWorkspace.DetectProject()
	if err != nil {
		return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
	}

	return PrettyKeyForProject(k, proj.Name)
}

// PrettyKeyForProject is [PrettyKey] for callers that already know the project name.
func PrettyKeyForProject(k config.Key, projectName tokens.PackageName) string {
	if k.Namespace() == string(projectName) {
		return k.Name()
	}

	return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
}

// Environment.Imports names the inline values "yaml". On a managed stack they are the managed environment itself, and
// the console would link "yaml" to a default/yaml environment that does not exist.
func stackEnvironmentImports(ps *workspace.ProjectStack, managed bool) []string {
	imports := ps.Environment.Imports()
	if managed && len(imports) > 0 && imports[len(imports)-1] == "yaml" {
		return imports[:len(imports)-1]
	}
	return imports
}
