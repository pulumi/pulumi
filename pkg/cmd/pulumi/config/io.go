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
	"os"
	"strings"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Attempts to load configuration for the given stack.
func GetStackConfiguration(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	project *workspace.Project,
) (backend.StackConfiguration, secrets.Manager, error) {
	return getStackConfigurationWithFallback(ctx, ssml, stack, project, nil)
}

// GetStackConfigurationOrLatest attempts to load a current stack configuration
// using getStackConfiguration. If that fails due to not being run within a
// valid project, the latest configuration from the backend is returned. This is
// primarily for use in commands like `pulumi destroy`, where it is useful to be
// able to clean up a stack whose configuration has already been deleted as part
// of that cleanup.
func GetStackConfigurationOrLatest(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	project *workspace.Project,
) (backend.StackConfiguration, secrets.Manager, error) {
	return getStackConfigurationWithFallback(
		ctx, ssml, stack, project,
		func(err error) (config.Map, error) {
			if errors.Is(err, workspace.ErrProjectNotFound) {
				// This error indicates that we're not being run in a project directory.
				// We should fallback on the backend.
				return backend.GetLatestConfiguration(ctx, stack)
			}
			return nil, err
		})
}

func getStackConfigurationWithFallback(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	s backend.Stack,
	project *workspace.Project,
	fallbackGetConfig func(err error) (config.Map, error), // optional
) (backend.StackConfiguration, secrets.Manager, error) {
	workspaceStack, err := cmdStack.LoadProjectStack(project, s)
	if err != nil || workspaceStack == nil {
		if fallbackGetConfig == nil {
			return backend.StackConfiguration{}, nil, err
		}
		// On first run or the latest configuration is unavailable, fallback to check the project's configuration
		cfg, err := fallbackGetConfig(err)
		if err != nil {
			return backend.StackConfiguration{}, nil, fmt.Errorf(
				"stack configuration could not be loaded from either Pulumi.yaml or the backend: %w", err)
		}
		workspaceStack = &workspace.ProjectStack{
			Config: cfg,
		}
	}

	sm, err := getAndSaveSecretsManager(ctx, ssml, s, workspaceStack)
	if err != nil {
		return backend.StackConfiguration{}, nil, err
	}

	config, err := getStackConfigurationFromProjectStack(ctx, s, project, sm, workspaceStack)
	if err != nil {
		return backend.StackConfiguration{}, nil, err
	}
	return config, sm, nil
}

func getStackConfigurationFromProjectStack(
	ctx context.Context,
	stack backend.Stack,
	project *workspace.Project,
	sm secrets.Manager,
	workspaceStack *workspace.ProjectStack,
) (backend.StackConfiguration, error) {
	env, diags, err := openStackEnv(ctx, stack, workspaceStack)
	if err != nil {
		return backend.StackConfiguration{}, fmt.Errorf("opening environment: %w", err)
	}
	if len(diags) != 0 {
		printESCDiagnostics(os.Stderr, diags)
		return backend.StackConfiguration{}, errors.New("opening environment: too many errors")
	}

	var pulumiEnv esc.Value
	if env != nil {
		warnOnNoEnvironmentEffects(os.Stdout, env)

		pulumiEnv = env.Properties["pulumiConfig"]

		_, environ, secrets, err := cli.PrepareEnvironment(env, nil)
		if err != nil {
			return backend.StackConfiguration{}, fmt.Errorf("preparing environment: %w", err)
		}
		if len(secrets) != 0 {
			logging.AddGlobalFilter(logging.CreateFilter(secrets, "[secret]"))
		}

		for _, kvp := range environ {
			if name, value, ok := strings.Cut(kvp, "="); ok {
				if err := os.Setenv(name, value); err != nil {
					return backend.StackConfiguration{}, fmt.Errorf("setting environment variable %v: %w", name, err)
				}
			}
		}
	}

	// If there are no secrets in the configuration, we should never use the decrypter, so it is safe to return
	// one which panics if it is used. This provides for some nice UX in the common case (since, for example, building
	// the correct decrypter for the diy backend would involve prompting for a passphrase)
	if !needsCrypter(workspaceStack.Config, pulumiEnv) {
		return backend.StackConfiguration{
			EnvironmentImports: workspaceStack.Environment.Imports(),
			Environment:        pulumiEnv,
			Config:             workspaceStack.Config,
			Decrypter:          config.NewPanicCrypter(),
		}, nil
	}

	crypter, err := sm.Decrypter()
	if err != nil {
		return backend.StackConfiguration{}, fmt.Errorf("getting configuration decrypter: %w", err)
	}

	return backend.StackConfiguration{
		EnvironmentImports: workspaceStack.Environment.Imports(),
		Environment:        pulumiEnv,
		Config:             workspaceStack.Config,
		Decrypter:          crypter,
	}, nil
}

func getAndSaveSecretsManager(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
) (secrets.Manager, error) {
	sm, state, err := ssml.GetSecretsManager(ctx, stack, workspaceStack)
	if err != nil {
		return nil, fmt.Errorf("get stack secrets manager: %w", err)
	}
	if state != cmdStack.SecretsManagerUnchanged {
		if err = cmdStack.SaveProjectStack(stack, workspaceStack); err != nil && state == cmdStack.SecretsManagerMustSave {
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
			for _, v := range v {
				if hasSecrets(v) {
					return true
				}
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

func openStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	yaml := workspaceStack.EnvironmentBytes()
	if len(yaml) == 0 {
		return nil, nil, nil
	}

	envs, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, nil, fmt.Errorf("cannot determine organzation for stack %v", stack.Ref())
	}
	orgName := orgNamer.OrgName()

	return envs.OpenYAMLEnvironment(ctx, orgName, yaml, 2*time.Hour)
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
) error {
	var decrypter config.Decrypter
	key, err := ParseConfigKey(configKey)
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

	return cmdStack.SaveProjectStack(destinationStack, destinationProjectStack)
}

func parseKeyValuePair(pair string) (config.Key, string, error) {
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
	key, err := ParseConfigKey(splitArg[0])
	if err != nil {
		return config.Key{}, "", fmt.Errorf("invalid configuration key: %w", err)
	}

	value := splitArg[1]
	return key, value, nil
}

func ParseConfigKey(key string) (config.Key, error) {
	// As a convenience, we'll treat any key with no delimiter as if:
	// <program-name>:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		proj, err := workspace.DetectProject()
		if err != nil {
			return config.Key{}, err
		}

		return config.ParseKey(fmt.Sprintf("%s:%s", proj.Name, key))
	}

	return config.ParseKey(key)
}

func PrettyKey(k config.Key) string {
	proj, err := workspace.DetectProject()
	if err != nil {
		return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
	}

	return prettyKeyForProject(k, proj)
}

func prettyKeyForProject(k config.Key, proj *workspace.Project) string {
	if k.Namespace() == string(proj.Name) {
		return k.Name()
	}

	return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
}
