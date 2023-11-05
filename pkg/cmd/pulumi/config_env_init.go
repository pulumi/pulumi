// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"

	"github.com/charmbracelet/glamour"
	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigEnvInitCmd(stackRef *string) *cobra.Command {
	var envName string
	var showSecrets bool
	var keepConfig bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates an environment for a stack",
		Long: "Creates an environment for a specific stack based on the stack's configuration values,\n" +
			"then replaces the stack's configuration values with a reference to that environment.\n" +
			"The environment will be created in the same organization as the stack.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if !yes && !cmdutil.Interactive() {
				return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
			}

			ctx := commandContext()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := readProject()
			if err != nil {
				return err
			}

			stack, err := requireStack(ctx, *stackRef, stackOfferNew|stackSetCurrent, opts)
			if err != nil {
				return err
			}

			envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
			if !ok {
				return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
			}

			orgName := stack.(httpstate.Stack).OrgName()

			if envName == "" {
				envName = fmt.Sprintf("%v-%v", project.Name, stack.Ref().Name())
			}

			fmt.Printf("Creating environment %v for stack %v...\n", envName, stack.Ref().Name())

			var envInit envInitCmd
			projectStack, config, err := envInit.getStackConfig(ctx, project, stack)
			if err != nil {
				return err
			}

			crypter, err := newEnvInitCrypter()
			if err != nil {
				return err
			}

			yaml, err := envInit.renderEnvironmentDefinition(ctx, envName, crypter, config, showSecrets)
			if err != nil {
				return err
			}

			preview, err := envInit.renderPreview(ctx, envBackend, orgName, envName, yaml, showSecrets)
			if err != nil {
				return err
			}
			fmt.Print(preview)

			if !yes {
				save, err := confirmation.New("Save?", confirmation.Yes).RunPrompt()
				if err != nil {
					return err
				}
				if !save {
					return errors.New("canceled")
				}
			}

			if !showSecrets {
				yaml, err = eval.DecryptSecrets(ctx, envName, yaml, crypter)
				if err != nil {
					return err
				}
			}

			diags, err := envBackend.CreateEnvironment(ctx, orgName, envName, yaml)
			if err != nil {
				return fmt.Errorf("creating environment: %w", err)
			}
			if len(diags) != 0 {
				return fmt.Errorf("internal error creating environment: %w", diags)
			}

			projectStack.Environment = projectStack.Environment.Append(envName)
			if !keepConfig {
				projectStack.Config = nil
			}
			if err = saveProjectStack(stack, projectStack); err != nil {
				return fmt.Errorf("saving stack config: %w", err)
			}
			return nil
		}),
	}

	cmd.Flags().StringVar(
		&envName, "env", "",
		`The name of the environment to create. Defaults to "<project name>-<stack name>"`)
	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret values in plaintext instead of ciphertext")
	cmd.Flags().BoolVar(
		&keepConfig, "keep-config", false,
		"Do not remove configuration values from the stack after creating the environment")
	cmd.Flags().BoolVarP(
		&yes, "yes", "y", false,
		"True to save the created environment without prompting")

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

type envInitCmd struct{}

func (cmd envInitCmd) getStackConfig(
	ctx context.Context,
	project *workspace.Project,
	stack backend.Stack,
) (*workspace.ProjectStack, resource.PropertyMap, error) {
	ps, err := loadProjectStack(project, stack)
	if err != nil {
		return nil, nil, err
	}

	decrypter, needsSave, err := getStackDecrypter(stack, ps)
	if err != nil {
		return nil, nil, err
	}
	// This may have setup the stack's secrets provider, so save the stack if needed.
	if needsSave {
		if err = saveProjectStack(stack, ps); err != nil {
			return nil, nil, fmt.Errorf("saving stack config: %w", err)
		}
	}

	m, err := ps.Config.AsDecryptedPropertyMap(ctx, decrypter)
	if err != nil {
		return nil, nil, err
	}
	return ps, m, nil
}

func (cmd envInitCmd) render(v resource.PropertyValue) any {
	switch {
	case v.IsBool():
		return v.BoolValue()
	case v.IsNumber():
		return v.NumberValue()
	case v.IsString():
		return v.StringValue()
	case v.IsArray():
		arrV := v.ArrayValue()
		rendered := make([]any, len(arrV))
		for i, v := range arrV {
			rendered[i] = cmd.render(v)
		}
		return rendered
	case v.IsObject():
		objV := v.ObjectValue()
		rendered := make(map[string]any, len(objV))
		for k, v := range objV {
			rendered[string(k)] = cmd.render(v)
		}
		return rendered
	case v.IsSecret():
		return map[string]any{
			"fn::secret": cmd.render(v.SecretValue().Element),
		}
	default:
		return nil
	}
}

func (cmd envInitCmd) renderEnvironmentDefinition(
	ctx context.Context,
	envName string,
	encrypter eval.Encrypter,
	config resource.PropertyMap,
	showSecrets bool,
) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	err := enc.Encode(map[string]any{
		"values": map[string]any{
			"pulumiConfig": cmd.render(resource.NewObjectProperty(config)),
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

func (cmd envInitCmd) renderPreview(
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

type envInitCrypter struct {
	crypter config.Crypter
}

func newEnvInitCrypter() (*envInitCrypter, error) {
	key := make([]byte, config.SymmetricCrypterKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	return &envInitCrypter{crypter: config.NewSymmetricCrypter(key)}, nil
}

func (c envInitCrypter) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	ciphertext, err := c.crypter.EncryptValue(ctx, string(plaintext))
	if err != nil {
		return nil, err
	}
	return []byte(ciphertext), nil
}

func (c envInitCrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	plaintext, err := c.crypter.DecryptValue(ctx, string(ciphertext))
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}
