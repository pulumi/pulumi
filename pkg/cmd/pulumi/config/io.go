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
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	latestConfig := false
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
		latestConfig = true
	}

	sm, err := getAndSaveSecretsManager(ctx, ssml, s, workspaceStack, configFile)
	if err != nil {
		return backend.StackConfiguration{}, nil, err
	}

	config, err := getStackConfigurationFromProjectStack(ctx, s, project, sm, workspaceStack, envOverrides, !latestConfig)
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
	env, ignoredKeys, diags, managedEnv, err := openStackEnvWithManaged(
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
	if managedEnv != nil {
		printManagedEnvironmentNotices(os.Stderr, stack, managedEnv) //nolint:forbidigo
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

	stackConfig := workspaceStack.Config
	if len(ignoredKeys) != 0 {
		stackConfig = maps.Clone(stackConfig)
		for _, key := range ignoredKeys {
			delete(stackConfig, key)
		}
	}

	// If there are no secrets in the configuration, we should never use the decrypter, so it is safe to return
	// one which panics if it is used. This provides for some nice UX in the common case (since, for example, building
	// the correct decrypter for the diy backend would involve prompting for a passphrase)
	if !needsCrypter(stackConfig, pulumiEnv) {
		return backend.StackConfiguration{
			EnvironmentImports:  imports,
			Environment:         pulumiEnv,
			Config:              stackConfig,
			Decrypter:           config.NewPanicCrypter(),
			ManagedEnvironment:  managedEnv,
			DeliveryEnvironment: stack.ConfigLocation().DeliveryEnvironment,
		}, nil
	}

	crypter := sm.Decrypter()

	return backend.StackConfiguration{
		EnvironmentImports:  imports,
		Environment:         pulumiEnv,
		Config:              stackConfig,
		Decrypter:           crypter,
		ManagedEnvironment:  managedEnv,
		DeliveryEnvironment: stack.ConfigLocation().DeliveryEnvironment,
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

func printManagedEnvironmentNotices(
	w io.Writer,
	stack backend.Stack,
	managedEnv *apitype.UpdateProgramEnvironment,
) {
	if managedEnv.ChangeRequestID != "" {
		fmt.Fprintf(w,
			"using local changes to environment %s (based on revision %d); an update publishes them, a preview does not\n",
			stack.ConfigLocation().ManagedEnvironment, managedEnv.BaseRevision)
	}
}

func openStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	env, _, diags, _, err := openStackEnvWithManaged(ctx, stack, workspaceStack, envOverrides, true)
	return env, diags, err
}

func openStackEnvWithManaged(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
	configIsLocal bool,
) (*esc.Environment, []config.Key, []apitype.EnvironmentDiagnostic, *apitype.UpdateProgramEnvironment, error) {
	env, diags, managedEnv, err := openOwnStackEnv(ctx, stack, workspaceStack, envOverrides)
	if err != nil || len(diags) != 0 {
		return nil, nil, diags, nil, err
	}
	env, ignoredKeys, diags, err := composeDeliveryEnvironment(ctx, stack, workspaceStack, env, configIsLocal)
	if err != nil || len(diags) != 0 {
		return nil, nil, diags, nil, err
	}
	return env, ignoredKeys, nil, managedEnv, nil
}

// composeDeliveryEnvironment adds the config Pulumi Delivery holds for the stack to the stack's environment. The
// environment supplies pulumiConfig only. The service decides which revision the caller sees, so Delivery's own work on
// the stack composes the revision of that work.
//
// For a key that both set, whoever runs the stack has its config applied last: Delivery's own work takes Delivery's
// value, and any other run takes the stack's, so that local changes work. The stack's file can't say whether a key is
// a committed value or a local change, so both cases warn. It returns the keys of the stack's file that this run must
// ignore, and composing never changes the stack's file. Config copied from the stack's latest update can hold the
// values Delivery gave that update, so it doesn't warn.
//
// The revision is opened by name: an environment that belongs to a stack isn't meant to be imported, so it also stays
// out of the stack's EnvironmentImports.
func composeDeliveryEnvironment(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	env *esc.Environment,
	configIsLocal bool,
) (*esc.Environment, []config.Key, []apitype.EnvironmentDiagnostic, error) {
	slot := stack.ConfigLocation().DeliveryEnvironment
	if slot == nil {
		return env, nil, nil, nil
	}

	envs, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, nil, errBackendNoEnvironments(stack.Backend())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, nil, nil, fmt.Errorf("cannot determine organization for stack %v", stack.Ref())
	}
	project, name, ok := strings.Cut(slot.Environment, "/")
	if !ok {
		return nil, nil, nil, fmt.Errorf(
			"stack %v has an invalid Pulumi Delivery environment %q", stack.Ref(), slot.Environment)
	}

	slotEnv, diags, err := envs.OpenEnvironmentRevision(
		ctx, orgNamer.OrgName(), project, name, slot.Revision, 2*time.Hour)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening the config Pulumi Delivery holds for stack %v: %w", stack.Ref(), err)
	}
	if len(diags) != 0 {
		return nil, nil, diags, nil
	}

	slotConfig, _ := slotEnv.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	if len(slotConfig) == 0 {
		return env, nil, nil, nil
	}

	var envConfig map[string]esc.Value
	if env != nil {
		envConfig, _ = env.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	}
	merged, ignored, both, err := mergeDeliveryConfig(
		stack, slot.OverridesStackConfig, slotConfig, envConfig, workspaceStack.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	if !configIsLocal {
		both = nil
	}
	// This is a non-command helper, as with printManagedEnvironmentNotices.
	printDeliveryNotices(os.Stderr, slotEnv, slot, both) //nolint:forbidigo

	if env == nil {
		env = &esc.Environment{Properties: map[string]esc.Value{}}
	}
	env.Properties["pulumiConfig"] = esc.NewValue(merged)
	return env, ignored, nil, nil
}

// mergeDeliveryConfig merges the config Delivery holds with the stack's environment. Keys are compared after
// qualifying them with the project, since an environment can leave a key bare. A key that one side wins is left out of
// the other, so that objects don't merge and the two spellings of a key don't race. It returns the merged config, the
// keys of the stack's file to ignore, and every key that both set.
func mergeDeliveryConfig(
	stack backend.Stack,
	overrides bool,
	slotConfig map[string]esc.Value,
	envConfig map[string]esc.Value,
	fileConfig config.Map,
) (map[string]esc.Value, []config.Key, []config.Key, error) {
	project, _ := stack.Ref().Project()
	qualify := func(rawKey string) (config.Key, error) {
		return ParseConfigKeyForProject(tokens.PackageName(project), rawKey, false)
	}

	merged := maps.Clone(envConfig)
	if merged == nil {
		merged = make(map[string]esc.Value, len(slotConfig))
	}
	envRawKeys := make(map[config.Key]string, len(envConfig))
	for rawKey := range envConfig {
		// A key that doesn't parse fails the update when it is merged, with a better error than this one could give.
		if key, err := qualify(rawKey); err == nil {
			envRawKeys[key] = rawKey
		}
	}

	var ignored, both []config.Key
	for rawKey, value := range slotConfig {
		key, err := qualify(rawKey)
		if err != nil {
			return nil, nil, nil, fmt.Errorf(
				"stack %v: Pulumi Delivery holds an invalid config key %q: %w", stack.Ref(), rawKey, err)
		}
		envRawKey, inEnv := envRawKeys[key]
		_, inFile := fileConfig[key]
		if inEnv || inFile {
			both = append(both, key)
		}
		if !overrides && (inEnv || inFile) {
			continue
		}
		if inEnv {
			delete(merged, envRawKey)
		}
		if inFile {
			ignored = append(ignored, key)
		}
		merged[rawKey] = value
	}
	sortConfigKeys := func(a, b config.Key) int { return strings.Compare(a.String(), b.String()) }
	slices.SortFunc(ignored, sortConfigKeys)
	slices.SortFunc(both, sortConfigKeys)
	return merged, ignored, both, nil
}

// printDeliveryNotices says which revision the run composed, and warns about each key that both Delivery and the stack
// set.
func printDeliveryNotices(
	w io.Writer,
	slotEnv *esc.Environment,
	slot *apitype.StackDeliveryEnvironment,
	both []config.Key,
) {
	source := "Pulumi Delivery"
	if pipeline := deliveryPipeline(slotEnv); pipeline != "" {
		source = "Pulumi Delivery (pipeline " + pipeline + ")"
	}
	fmt.Fprintf(w, "using config from %s, revision %d\n", source, slot.Revision)
	if len(both) == 0 {
		return
	}

	keys := make([]string, len(both))
	for i, key := range both {
		keys[i] = key.String()
	}
	color := cmdutil.GetGlobalColorization()
	var message string
	if slot.OverridesStackConfig {
		message = fmt.Sprintf("%s overrides config that this stack also sets: %s. "+
			"Remove these keys from the stack's config, or change the pipeline.", source, strings.Join(keys, ", "))
	} else {
		message = fmt.Sprintf("this stack sets config that %s also sets: %s. This run uses the stack's values, and a "+
			"Delivery run uses the pipeline's. Remove these keys from the stack's config to pick up the pipeline's values.",
			source, strings.Join(keys, ", "))
	}
	fmt.Fprintln(w, color.Colorize(colors.SpecWarning+"warning: "+message+colors.Reset))
}

// deliveryPipeline returns the pipeline that the environment's revision says it came from, if it says.
func deliveryPipeline(slotEnv *esc.Environment) string {
	provenance, _ := slotEnv.Properties["delivery"].Value.(map[string]esc.Value)
	pipeline, _ := provenance["pipeline"].Value.(string)
	return pipeline
}

func openOwnStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	envOverrides []string,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, *apitype.UpdateProgramEnvironment, error) {
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
	return evaluation.Environment, nil, &apitype.UpdateProgramEnvironment{
		BaseRevision:    evaluation.BaseRevision,
		ChangeRequestID: evaluation.ChangeRequestID,
		DraftRevision:   evaluation.DraftRevision,
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
