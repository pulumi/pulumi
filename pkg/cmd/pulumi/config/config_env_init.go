// Copyright 2016-2024, Pulumi Corporation.
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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/charmbracelet/glamour"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigEnvInitCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvInitCmd{
		parent:     parent,
		newCrypter: newConfigEnvInitCrypter,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates an environment for a stack",
		Long: "Creates an environment for a specific stack based on the stack's configuration values,\n" +
			"then replaces the stack's configuration values with a reference to that environment.\n" +
			"The environment will be created in the same organization as the stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(
		&impl.envName, "env", "",
		`The name of the environment to create. Defaults to "<project name>/<stack name>"`)
	cmd.Flags().BoolVar(
		&impl.showSecrets, "show-secrets", false,
		"Show secret values in plaintext instead of ciphertext")
	cmd.Flags().BoolVar(
		&impl.keepConfig, "keep-config", false,
		"Do not remove configuration values from the stack after creating the environment")
	cmd.Flags().BoolVarP(
		&impl.yes, "yes", "y", false,
		"True to save the created environment without prompting")
	cmd.Flags().BoolVar(
		&impl.remoteConfig, "remote-config", false,
		"Migrate local config to a remote ESC environment")

	return cmd
}

var envPreviewTemplate = template.Must(template.New("env-preview").Parse("# Value\n" +
	"```json\n" +
	"{{.Preview}}\n" +
	"```\n" +
	"# Definition\n" +
	"```yaml\n" +
	"{{.Definition}}\n" +
	"```\n"))

type configEnvInitCmd struct {
	parent *configEnvCmd

	newCrypter func() (evalCrypter, error)

	envName       string
	showSecrets   bool
	keepConfig    bool
	yes           bool
	remoteConfig bool
}

func (cmd *configEnvInitCmd) run(ctx context.Context, args []string) error {
	if cmd.remoteConfig {
		return cmd.runMigrate(ctx)
	}

	if !cmd.yes && !cmd.parent.interactive {
		return backenderr.NonInteractiveRequiresYesError{}
	}

	opts := display.Options{Color: cmd.parent.color}

	project, _, err := cmd.parent.ws.ReadProject()
	if err != nil {
		return err
	}

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

	if loc := stack.ConfigLocation(); loc.IsRemote {
		return errors.New("this stack already uses remote configuration; migration is not needed")
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	orgName, err := stackOrgName(stack)
	if err != nil {
		return err
	}

	// Parse given environment name
	// Try to split the given envName into project/env
	// Default to the stack's project and name if the environment project and/or name are not provided
	envProject := project.Name.String()
	envName := stack.Ref().Name().String()
	first, second, found := strings.Cut(cmd.envName, "/")
	if found {
		envProject = first
		envName = second
	} else if first != "" {
		envName = first
	}

	fmt.Fprintf(cmd.parent.stdout, "Creating environment %v/%v for stack %v...\n", envProject, envName, stack.Ref().Name())

	projectStack, config, err := cmd.getStackConfig(ctx, cmdutil.Diag(), project, stack)
	if err != nil {
		return err
	}

	crypter, err := cmd.newCrypter()
	if err != nil {
		return err
	}

	yaml, err := cmd.renderEnvironmentDefinition(ctx, envName, crypter, config, cmd.showSecrets)
	if err != nil {
		return err
	}

	preview, err := cmd.renderPreview(ctx, envBackend, orgName, envName, yaml, cmd.showSecrets)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.parent.stdout, preview)

	if cmd.parent.interactive && !cmd.yes {
		response := ui.PromptUser(
			"Use remote configuration? (recommended)",
			[]string{"yes", "no"}, "yes", cmdutil.GetGlobalColorization())
		if response == "yes" {
			return cmd.runMigrate(ctx)
		}
	}

	if !cmd.yes {
		response := ui.PromptUser("Save?", []string{"yes", "no"}, "yes", cmdutil.GetGlobalColorization())
		switch response {
		case "no":
			return errors.New("canceled")
		case "yes":
		}
	}

	if !cmd.showSecrets {
		yaml, err = eval.DecryptSecrets(ctx, envName, yaml, crypter)
		if err != nil {
			return err
		}
	}

	diags, err := envBackend.CreateEnvironment(ctx, orgName, envProject, envName, yaml)
	if err != nil {
		return fmt.Errorf("creating environment: %w", err)
	}
	if len(diags) != 0 {
		return fmt.Errorf("internal error creating environment: %w", diags)
	}

	fullName := fmt.Sprintf("%s/%s", envProject, envName)
	projectStack.Environment = projectStack.Environment.Append(fullName)
	if !cmd.keepConfig {
		projectStack.Config = nil
	}
	if err = cmd.parent.saveProjectStack(ctx, stack, projectStack); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}

	return nil
}

func (cmd *configEnvInitCmd) runMigrate(ctx context.Context) error {
	if !cmd.yes && !cmd.parent.interactive {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	opts := display.Options{Color: cmd.parent.color}

	project, _, err := cmd.parent.ws.ReadProject()
	if err != nil {
		return err
	}

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

	if loc := stack.ConfigLocation(); loc.IsRemote {
		return errors.New("this stack already uses remote configuration; migration is not needed")
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	orgName, err := stackOrgName(stack)
	if err != nil {
		return err
	}

	envProject := project.Name.String()
	envName := stack.Ref().Name().String()
	first, second, found := strings.Cut(cmd.envName, "/")
	if found {
		envProject = first
		envName = second
	} else if first != "" {
		envName = first
	}

	ps, err := cmd.parent.loadProjectStack(ctx, cmdutil.Diag(), project, stack)
	if err != nil {
		return err
	}

	decrypter, state, err := cmd.parent.ssml.GetDecrypter(ctx, stack, ps)
	if err != nil {
		return err
	}
	if state != cmdStack.SecretsManagerUnchanged {
		if err = cmd.parent.saveProjectStack(ctx, stack, ps); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	decrypted, err := ps.Config.Decrypt(decrypter)
	if err != nil {
		return fmt.Errorf("decrypting config: %w", err)
	}

	pulumiConfig := make(map[string]any, len(decrypted))
	secureKeys := make(map[config.Key]bool)
	for _, k := range ps.Config.SecureKeys() {
		secureKeys[k] = true
	}
	for k, v := range decrypted {
		if secureKeys[k] {
			pulumiConfig[k.String()] = map[string]any{"fn::secret": v}
		} else {
			pulumiConfig[k.String()] = v
		}
	}

	fullEnvName := fmt.Sprintf("%s/%s", envProject, envName)

	envDef := map[string]any{
		"values": map[string]any{
			"pulumiConfig": pulumiConfig,
		},
	}

	// Filter out the environment's own name to avoid cyclic imports. This can
	// happen if "config env init" was run before "--remote-config", adding the
	// environment as an import of the stack that we're now migrating into it.
	existingImports := ps.Environment.Imports()
	filtered := make([]string, 0, len(existingImports))
	for _, imp := range existingImports {
		if imp != fullEnvName {
			filtered = append(filtered, imp)
		}
	}
	if len(filtered) > 0 {
		envDef["imports"] = filtered
	}

	existingYAML, etag, _, getErr := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", false)
	envExists := getErr == nil

	if envExists {
		var existingDef map[string]any
		if err := yaml.Unmarshal(existingYAML, &existingDef); err != nil {
			return fmt.Errorf("parsing existing environment %s: %w", fullEnvName, err)
		}

		existingPC := extractPulumiConfigMap(existingDef)

		for key := range pulumiConfig {
			if _, exists := existingPC[key]; exists {
				fmt.Fprintf(cmd.parent.stdout, "Warning: overwriting existing key %q in environment %s\n", key, fullEnvName)
			}
		}

		mergedDef := mergeMigrateEnvDefs(existingDef, envDef)

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(mergedDef); err != nil {
			return fmt.Errorf("serialising environment: %w", err)
		}
		if err := enc.Close(); err != nil {
			return fmt.Errorf("serialising environment: %w", err)
		}

		diags, updateErr := envBackend.UpdateEnvironmentWithProject(ctx, orgName, envProject, envName, buf.Bytes(), etag)
		if updateErr != nil {
			return fmt.Errorf("updating environment %s: %w", fullEnvName, updateErr)
		}
		if len(diags) != 0 {
			return fmt.Errorf("environment validation failed: %w", diags)
		}
	} else {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(envDef); err != nil {
			return fmt.Errorf("serialising environment: %w", err)
		}
		if err := enc.Close(); err != nil {
			return fmt.Errorf("serialising environment: %w", err)
		}

		diags, createErr := envBackend.CreateEnvironment(ctx, orgName, envProject, envName, buf.Bytes())
		if createErr != nil {
			return fmt.Errorf("creating environment: %w", createErr)
		}
		if len(diags) != 0 {
			return fmt.Errorf("environment validation failed: %w", diags)
		}
	}

	ps.Environment = workspace.NewEnvironment([]string{fullEnvName})
	ps.Config = nil
	if err = stack.SaveRemoteConfig(ctx, ps); err != nil {
		return fmt.Errorf("linking stack to remote config: %w", err)
	}

	fmt.Fprintf(cmd.parent.stdout, "Migrated config to environment %s.\n", fullEnvName)

	// Offer to delete the local config file now that migration is complete.
	_, configFilePath, pathErr := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
	if pathErr == nil {
		if _, statErr := os.Stat(configFilePath); statErr == nil {
			shouldDelete := cmd.yes
			if !shouldDelete && cmd.parent.interactive {
				response := ui.PromptUser(
					fmt.Sprintf("Delete the local config file %s?", configFilePath),
					[]string{"yes", "no"}, "yes",
					cmdutil.GetGlobalColorization(),
				)
				shouldDelete = response == "yes"
			}
			if shouldDelete {
				if rmErr := os.Remove(configFilePath); rmErr != nil {
					fmt.Fprintf(cmd.parent.stdout, "Warning: could not delete %s: %v\n", configFilePath, rmErr)
				} else {
					fmt.Fprintf(cmd.parent.stdout, "Deleted %s.\n", configFilePath)
				}
			} else {
				fmt.Fprintf(cmd.parent.stdout, "Local config file %s retained.\n", configFilePath)
			}
		}
	}

	return nil
}

func extractPulumiConfigMap(envDef map[string]any) map[string]any {
	values, _ := envDef["values"].(map[string]any)
	if values == nil {
		return nil
	}
	pc, _ := values["pulumiConfig"].(map[string]any)
	return pc
}

func mergeMigrateEnvDefs(existing, incoming map[string]any) map[string]any {
	result := make(map[string]any, len(existing))
	for k, v := range existing {
		result[k] = v
	}

	incomingValues, _ := incoming["values"].(map[string]any)
	incomingPC, _ := incomingValues["pulumiConfig"].(map[string]any)

	existingValues, _ := result["values"].(map[string]any)
	if existingValues == nil {
		existingValues = map[string]any{}
	}
	existingPC, _ := existingValues["pulumiConfig"].(map[string]any)
	if existingPC == nil {
		existingPC = map[string]any{}
	}

	for k, v := range incomingPC {
		existingPC[k] = v
	}
	existingValues["pulumiConfig"] = existingPC
	result["values"] = existingValues

	if imports, ok := incoming["imports"]; ok {
		result["imports"] = imports
	}

	return result
}

func (cmd *configEnvInitCmd) getStackConfig(
	ctx context.Context,
	sink diag.Sink,
	project *workspace.Project,
	stack backend.Stack,
) (*workspace.ProjectStack, property.Map, error) {
	ps, err := cmd.parent.loadProjectStack(ctx, sink, project, stack)
	if err != nil {
		return nil, property.Map{}, err
	}

	decrypter, state, err := cmd.parent.ssml.GetDecrypter(ctx, stack, ps)
	if err != nil {
		return nil, property.Map{}, err
	}
	// This may have setup the stack's secrets provider, so save the stack if needed.
	if state != cmdStack.SecretsManagerUnchanged {
		if err = cmd.parent.saveProjectStack(ctx, stack, ps); err != nil {
			return nil, property.Map{}, fmt.Errorf("saving stack config: %w", err)
		}
	}

	m, err := ps.Config.AsDecryptedPropertyMap(ctx, decrypter)
	if err != nil {
		return nil, property.Map{}, err
	}
	return ps, m, nil
}

func (cmd *configEnvInitCmd) render(v property.Value) any {
	switch {
	case v.Secret():
		return map[string]any{
			"fn::secret": cmd.render(v.WithSecret(false)),
		}
	case v.IsBool():
		return v.AsBool()
	case v.IsNumber():
		return v.AsNumber()
	case v.IsString():
		return v.AsString()
	case v.IsArray():
		arrV := v.AsArray()
		rendered := make([]any, arrV.Len())
		for i, v := range arrV.All {
			rendered[i] = cmd.render(v)
		}
		return rendered
	case v.IsMap():
		objV := v.AsMap()
		rendered := make(map[string]any, objV.Len())
		for k, v := range objV.All {
			rendered[k] = cmd.render(v)
		}
		return rendered
	default:
		return nil
	}
}

func (cmd *configEnvInitCmd) renderEnvironmentDefinition(
	ctx context.Context,
	envName string,
	encrypter eval.Encrypter,
	config property.Map,
	showSecrets bool,
) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	err := enc.Encode(map[string]any{
		"values": map[string]any{
			"pulumiConfig": cmd.render(property.New(config)),
		},
	})
	if err != nil {
		return nil, err
	}

	yaml := b.Bytes()
	if !showSecrets {
		yaml, err = eval.EncryptSecrets(ctx, envName, yaml, encrypter)
		if err != nil {
			return nil, err
		}
	}

	return yaml, nil
}

func (cmd *configEnvInitCmd) renderPreview(
	ctx context.Context,
	b backend.EnvironmentsBackend,
	org string,
	name string,
	yaml []byte,
	showSecrets bool,
) (string, error) {
	env, diags, err := b.CheckYAMLEnvironment(ctx, org, yaml)
	if err != nil {
		return "", err
	}
	if len(diags) != 0 {
		return "", fmt.Errorf("internal error: %w", diags)
	}

	envJSON, err := json.MarshalIndent(esc.NewValue(env.Properties).ToJSON(!showSecrets), "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding value: %w", err)
	}

	var markdown bytes.Buffer
	err = envPreviewTemplate.Execute(&markdown, map[string]any{
		"Preview":    string(envJSON),
		"Definition": string(yaml),
	})
	if err != nil {
		return "", fmt.Errorf("rendering preview: %w", err)
	}

	if !cmdutil.InteractiveTerminal() {
		return markdown.String(), nil
	}

	renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(0))
	if err != nil {
		return "", fmt.Errorf("internal error: creating renderer: %w", err)
	}
	rendered, err := renderer.Render(markdown.String())
	if err != nil {
		rendered = markdown.String()
	}

	return rendered, nil
}

type evalCrypter interface {
	eval.Decrypter
	eval.Encrypter
}

type configEnvInitCrypter struct {
	crypter config.Crypter
}

func newConfigEnvInitCrypter() (evalCrypter, error) {
	key := make([]byte, config.SymmetricCrypterKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	crypter := config.NewSymmetricCrypter(key)
	cachedCrypter := config.NewCiphertextToPlaintextCachedCrypter(crypter, crypter)
	return &configEnvInitCrypter{crypter: cachedCrypter}, nil
}

func (c configEnvInitCrypter) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	ciphertext, err := c.crypter.EncryptValue(ctx, string(plaintext))
	if err != nil {
		return nil, err
	}
	return []byte(ciphertext), nil
}

func (c configEnvInitCrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	plaintext, err := c.crypter.DecryptValue(ctx, string(ciphertext))
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}
