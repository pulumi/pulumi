// Copyright 2016-2025, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pulumiSecrets "github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var errConfigEditNeedsEncrypter = errors.New("edited config contains secrets but no encrypter is available")

type configEditCmd struct {
	OpenInEditor       func(filename string) error
	LoadProjectStack   func(context.Context, diag.Sink, *workspace.Project, backend.Stack) (*workspace.ProjectStack, error)
	SaveProjectStack   func(context.Context, backend.Stack, *workspace.ProjectStack) error
	SecretsManagerLoad cmdStack.SecretsManagerLoader
}

func newConfigEditCmd(ws pkgWorkspace.Context, stack *string) *cobra.Command {
	edit := &configEditCmd{
		OpenInEditor:       openInEditor,
		LoadProjectStack:   cmdStack.LoadProjectStack,
		SaveProjectStack:   cmdStack.SaveProjectStack,
		SecretsManagerLoad: cmdStack.NewStackSecretsManagerLoaderFromEnv(),
	}

	editCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in your EDITOR",
		Long: "Edit the stack configuration in your configured editor (`EDITOR`).\n\n" +
			"The edited document follows the same format used by `pulumi config --json` and `pulumi config set-all --json`.\n" +
			"Set `secret: true` to store a value as secret. Use `objectValue` for object and array values.",
		Example: "  pulumi config edit\n\n" +
			"  # Example edited JSON for a secret string value:\n" +
			"  # {\n" +
			"  #   \"app:token\": {\n" +
			"  #     \"value\": \"mytoken123\",\n" +
			"  #     \"secret\": true\n" +
			"  #   }\n" +
			"  # }\n\n" +
			"  # Example edited JSON for a secret object value:\n" +
			"  # {\n" +
			"  #   \"app:oauth\": {\n" +
			"  #     \"value\": \"{\\\"clientId\\\":\\\"id\\\",\\\"clientSecret\\\":\\\"secret\\\"}\",\n" +
			"  #     \"objectValue\": {\n" +
			"  #       \"clientId\": \"id\",\n" +
			"  #       \"clientSecret\": \"secret\"\n" +
			"  #     },\n" +
			"  #     \"secret\": true\n" +
			"  #   }\n" +
			"  # }",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.Interactive() {
				return errors.New("pulumi config edit must be run in interactive mode")
			}

			ctx := cmd.Context()
			opts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			s, err := cmdStack.RequireStack(
				ctx,
				cmdutil.Diag(),
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			return edit.Run(ctx, ws, project, s)
		},
	}

	constrictor.AttachArguments(editCmd, constrictor.NoArgs)
	return editCmd
}

func (c *configEditCmd) Run(
	ctx context.Context,
	ws pkgWorkspace.Context,
	project *workspace.Project,
	stack backend.Stack,
) error {
	projectStack, err := c.LoadProjectStack(ctx, cmdutil.Diag(), project, stack)
	if err != nil {
		return err
	}

	if configLocation := stack.ConfigLocation(); configLocation.IsRemote {
		err := errors.New("config edit not supported for remote stack config")
		if configLocation.EscEnv != nil {
			return fmt.Errorf("%w: use `pulumi env set %s pulumiConfig.<key> <value>`",
				err, *configLocation.EscEnv)
		}
		return err
	}

	var (
		secretsManager   pulumiSecrets.Manager
		secretsAvailable bool
	)
	ensureSecretsManager := func() (pulumiSecrets.Manager, error) {
		if secretsAvailable {
			return secretsManager, nil
		}

		sm, state, err := c.SecretsManagerLoad.GetSecretsManager(ctx, stack, projectStack)
		if err != nil {
			return nil, err
		}
		if state != cmdStack.SecretsManagerUnchanged {
			if err = c.SaveProjectStack(ctx, stack, projectStack); err != nil {
				return nil, fmt.Errorf("save stack config: %w", err)
			}
		}

		secretsManager = sm
		secretsAvailable = true
		return sm, nil
	}

	var decrypter config.Decrypter = config.NewPanicCrypter()
	if projectStack.Config.HasSecureValue() {
		sm, err := ensureSecretsManager()
		if err != nil {
			return err
		}
		decrypter = sm.Decrypter()
	}

	initialConfig, err := encodeEditableConfig(projectStack.Config, decrypter)
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp("", "pulumi-config-edit-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file for config edit: %w", err)
	}
	filename := tempFile.Name()
	defer os.Remove(filename)

	if _, err = tempFile.Write(initialConfig); err != nil {
		tempFile.Close()
		return fmt.Errorf("writing editable config file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("closing editable config file: %w", err)
	}

	if err = c.OpenInEditor(filename); err != nil {
		return err
	}

	editedConfig, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading edited config file: %w", err)
	}

	if bytes.Equal(initialConfig, editedConfig) {
		return nil
	}

	var encrypter config.Encrypter
	if secretsAvailable {
		encrypter = secretsManager.Encrypter()
	}
	updatedConfig, err := decodeEditableConfig(ctx, ws, editedConfig, encrypter)
	if errors.Is(err, errConfigEditNeedsEncrypter) {
		sm, serr := ensureSecretsManager()
		if serr != nil {
			return serr
		}
		updatedConfig, err = decodeEditableConfig(ctx, ws, editedConfig, sm.Encrypter())
	}
	if err != nil {
		return err
	}

	projectStack.Config = updatedConfig
	return c.SaveProjectStack(ctx, stack, projectStack)
}

func encodeEditableConfig(cfg config.Map, decrypter config.Decrypter) ([]byte, error) {
	editable := make(map[string]configValueJSON, len(cfg))
	for key, value := range cfg {
		encodedValue, err := encodeEditableConfigValue(value, decrypter)
		if err != nil {
			return nil, fmt.Errorf("encoding value for %q: %w", key.String(), err)
		}
		editable[key.String()] = encodedValue
	}

	out, err := json.MarshalIndent(editable, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func encodeEditableConfigValue(v config.Value, decrypter config.Decrypter) (configValueJSON, error) {
	var d config.Decrypter = config.NewPanicCrypter()
	if v.Secure() {
		d = decrypter
	}

	raw, err := v.Value(d)
	if err != nil {
		return configValueJSON{}, err
	}

	entry := configValueJSON{
		Value:  &raw,
		Secret: v.Secure(),
	}
	if v.Object() {
		var editable any
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&editable); err != nil {
			return configValueJSON{}, err
		}
		entry.ObjectValue = editable
	}

	return entry, nil
}

func decodeEditableConfig(
	ctx context.Context,
	ws pkgWorkspace.Context,
	editableConfig []byte,
	encrypter config.Encrypter,
) (config.Map, error) {
	dec := json.NewDecoder(bytes.NewReader(editableConfig))
	dec.UseNumber()

	var edited map[string]configValueJSON
	if err := dec.Decode(&edited); err != nil {
		return nil, fmt.Errorf("parsing edited config document: %w", err)
	}
	if err := ensureSingleJSONValue(dec); err != nil {
		return nil, err
	}

	result := make(config.Map, len(edited))
	for rawKey, rawValue := range edited {
		key, err := ParseConfigKey(ws, rawKey, false /*path*/)
		if err != nil {
			return nil, fmt.Errorf("invalid edited config key %q: %w", rawKey, err)
		}

		value, err := decodeEditableConfigValue(ctx, rawValue, encrypter)
		if err != nil {
			return nil, fmt.Errorf("invalid edited config value for key %q: %w", rawKey, err)
		}
		result[key] = value
	}

	return result, nil
}

func ensureSingleJSONValue(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return err
	default:
		return errors.New("edited config document must contain a single JSON object")
	}
}

func decodeEditableConfigValue(
	ctx context.Context,
	rawValue configValueJSON,
	encrypter config.Encrypter,
) (config.Value, error) {
	var valueText string
	if rawValue.ObjectValue != nil {
		encodedObject, err := json.Marshal(rawValue.ObjectValue)
		if err != nil {
			return config.Value{}, err
		}
		valueText = string(encodedObject)
	} else if rawValue.Value != nil {
		valueText = *rawValue.Value
	} else {
		return config.Value{}, errors.New("value is nil")
	}

	if !rawValue.Secret {
		if rawValue.ObjectValue != nil {
			return config.NewObjectValue(valueText), nil
		}
		return config.NewValue(valueText), nil
	}

	if encrypter == nil {
		return config.Value{}, errConfigEditNeedsEncrypter
	}

	encrypted, err := encrypter.EncryptValue(ctx, valueText)
	if err != nil {
		return config.Value{}, err
	}
	if rawValue.ObjectValue != nil {
		// Preserve object metadata while storing the object payload as a secret ciphertext.
		secureObject, err := json.Marshal(map[string]string{"secure": encrypted})
		if err != nil {
			return config.Value{}, err
		}
		return config.NewSecureObjectValue(string(secureObject)), nil
	}
	return config.NewSecureValue(encrypted), nil
}

func openInEditor(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("no EDITOR environment variable set")
	}
	return openInEditorInternal(editor, filename)
}

func openInEditorInternal(editor, filename string) error {
	args, err := shlex.Split(editor)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("configured editor is empty")
	}

	args = append(args, filename)
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
