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

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigEnvEjectCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvEjectCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "eject",
		Short: "Convert a remote-config stack back to local configuration",
		Long: "Converts a stack that stores its configuration remotely in an ESC environment back to a local\n" +
			"Pulumi.<stack>.yaml file. Secrets are re-encrypted with a local secrets provider, the backing\n" +
			"environment's imports are preserved as local stack environment imports, the stack is unlinked\n" +
			"from remote configuration, and the now-unused environment is deleted unless --keep-env is set.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().BoolVar(&impl.keepEnv, "keep-env", false,
		"Keep the backing ESC environment instead of deleting it after ejecting")
	cmd.Flags().StringVar(&impl.secretsProvider, "secrets-provider", "",
		"The secrets provider to use for re-encrypting secrets locally "+
			"(default, passphrase, or a cloud KMS URL such as awskms://...)")
	cmd.Flags().BoolVarP(&impl.yes, "yes", "y", false,
		"True to proceed without prompting")

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

	if _, _, err := cmd.parent.ws.ReadProject(); err != nil {
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
		*cmd.parent.configFile,
	)
	if err != nil {
		return err
	}

	if !stack.ConfigLocation().IsRemote {
		return errors.New("this stack does not use remote configuration; there is nothing to eject")
	}

	// Eject's local write, unlink, and default delete are irreversible; require --yes when no TTY can confirm.
	if !cmd.yes && !cmd.parent.interactive {
		return backenderr.ErrNonInteractiveRequiresYes
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}
	orgName := orgNamer.OrgName()

	loc := stack.ConfigLocation()
	if loc.EscEnv == nil || *loc.EscEnv == "" {
		return errors.New("this stack does not reference a backing environment")
	}
	envProject, envName, err := splitEnvRef(*loc.EscEnv)
	if err != nil {
		return err
	}

	// Decrypt secrets in memory (re-encrypted locally before disk); eject the pinned revision when
	// pinned, else latest. A missing env skips to unlinking.
	def, _, _, err := envBackend.GetEnvironment(ctx, orgName, envProject, envName, envRefVersion(*loc.EscEnv), true)
	envMissing := false
	if err != nil {
		if isNotFound(err) {
			envMissing = true
		} else {
			return fmt.Errorf("getting environment %s/%s: %w", envProject, envName, err)
		}
	}

	ps := &workspace.ProjectStack{}
	hasSecret := false
	if !envMissing {
		pulumiConfig, imports, structured, otherValues, err := parseEjectedEnvironment(def)
		if err != nil {
			return err
		}
		// A local stack file holds only config and imports. If the env carries other values
		// (environmentVariables, files, ...), refuse before mutating: ejecting drops them and the default
		// delete would destroy the only copy.
		if len(otherValues) > 0 {
			return fmt.Errorf(
				"environment %s/%s defines values eject cannot preserve in a local stack file (values.%s); "+
					"eject supports environments whose values contain only pulumiConfig. Inspect them with "+
					"`pulumi config edit` or keep the stack on remote configuration",
				envProject, envName, strings.Join(otherValues, ", values."))
		}
		if len(structured) > 0 {
			fmt.Fprintf(cmd.parent.stdout,
				"Warning: import(s) %s had merge options that are not preserved in the local stack "+
					"file; only the environment name is kept\n", strings.Join(structured, ", "))
		}

		plaintextMap, err := buildPlaintextMap(pulumiConfig)
		if err != nil {
			return err
		}
		for _, pt := range plaintextMap {
			if pt.Secure() {
				hasSecret = true
				break
			}
		}

		encrypter, err := cmd.resolveEncrypter(ctx, stack, ps, hasSecret, opts)
		if err != nil {
			return err
		}

		cfg, err := config.EncryptMap(ctx, plaintextMap, encrypter)
		if err != nil {
			return fmt.Errorf("re-encrypting configuration: %w", err)
		}

		ps.Config = config.Map(cfg)
		if len(imports) > 0 {
			ps.Environment = workspace.NewEnvironment(imports)
		}
	}

	if !cmd.yes && cmd.parent.interactive {
		if !ui.ConfirmPrompt(
			fmt.Sprintf("Eject stack %v: write configuration locally and unlink it from environment %s/%s?",
				stack.Ref().Name(), envProject, envName),
			"yes", opts) {
			return errors.New("eject canceled")
		}
	}

	// Write the local file before unlinking so that a re-encryption or serialization failure leaves the
	// stack untouched (still remote, no partial/plaintext file on disk).
	if !envMissing {
		path := *cmd.parent.configFile
		if path == "" {
			_, detected, err := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
			if err != nil {
				return fmt.Errorf("locating local stack configuration file: %w", err)
			}
			path = detected
		}
		if err := atomicWriteProjectStack(ps, path); err != nil {
			return err
		}
		fmt.Fprintf(cmd.parent.stdout, "Wrote local stack configuration to %s\n", path)
	}

	if err := stack.RemoveRemoteConfig(ctx); err != nil {
		if !envMissing {
			return fmt.Errorf(
				"the local stack configuration was written but the stack is still linked to environment %s/%s: %w; "+
					"re-run `pulumi config env eject` to reconcile", envProject, envName, err)
		}
		return fmt.Errorf("unlinking stack from environment %s/%s: %w", envProject, envName, err)
	}

	// Eject has succeeded; deleting the now-unused environment is best-effort, so warn on failure
	// rather than erroring.
	fmt.Fprintf(cmd.parent.stdout, "Unlinked stack %v from remote configuration\n", stack.Ref().Name())
	deleted, err := cmd.deleteEnvironment(ctx, envBackend, orgName, envProject, envName, envMissing, opts)
	if err != nil {
		fmt.Fprintf(cmd.parent.stdout,
			"Warning: could not delete environment %s/%s: %v\n"+
				"  the stack is fully ejected; remove the environment manually with "+
				"`pulumi env rm %s/%s`\n",
			envProject, envName, err, envProject, envName)
		return nil
	}
	if deleted {
		fmt.Fprintf(cmd.parent.stdout, "Deleted environment %s/%s\n", envProject, envName)
	} else {
		fmt.Fprintf(cmd.parent.stdout, "Kept environment %s/%s\n", envProject, envName)
	}
	return nil
}

// resolveEncrypter builds an Encrypter for local re-encryption: NopEncrypter when there are no
// secrets, otherwise --secrets-provider or "default".
func (cmd *configEnvEjectCmd) resolveEncrypter(
	ctx context.Context,
	stack backend.Stack,
	ps *workspace.ProjectStack,
	hasSecret bool,
	opts display.Options,
) (config.Encrypter, error) {
	if !hasSecret {
		return config.NopEncrypter, nil
	}

	provider := cmd.secretsProvider
	if provider == "" {
		if !cmd.yes && cmd.parent.interactive {
			value, err := ui.PromptForValue(
				false, "secrets provider", "default", false,
				func(string) error { return nil }, opts)
			if err != nil {
				return nil, err
			}
			provider = value
		}
		// Fall back to "default" when non-interactive or the prompt is empty.
		if provider == "" {
			provider = "default"
		}
	}

	if err := cmdStack.ValidateSecretsProvider(provider); err != nil {
		return nil, err
	}
	if provider != "default" {
		ps.SecretsProvider = provider
	}

	encrypter, _, err := cmd.parent.ssml.GetEncrypter(ctx, stack, ps)
	if err != nil {
		return nil, fmt.Errorf("setting up secrets provider %q: %w", provider, err)
	}
	return encrypter, nil
}

// deleteEnvironment removes the backing environment unless --keep-env is set; an already-deleted env
// (404 or missing from the start) is treated as success.
//
// There is no cross-stack reference check: if another stack imported this env, deleting it removes that
// import's source too. Use --keep-env when the env may be shared.
func (cmd *configEnvEjectCmd) deleteEnvironment(
	ctx context.Context,
	envBackend backend.EnvironmentsBackend,
	orgName, envProject, envName string,
	envMissing bool,
	opts display.Options,
) (bool, error) {
	if cmd.keepEnv {
		return false, nil
	}
	if envMissing {
		return true, nil
	}

	if !cmd.yes && cmd.parent.interactive {
		if !ui.ConfirmPrompt(
			fmt.Sprintf("This will permanently delete the backing environment %s/%s; "+
				"any stack that imports it will break.", envProject, envName),
			"yes", opts) {
			return false, nil
		}
	}

	if err := envBackend.DeleteEnvironmentWithProject(ctx, orgName, envProject, envName); err != nil {
		if isNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting environment %s/%s: %w", envProject, envName, err)
	}
	return true, nil
}

// parseEjectedEnvironment extracts the env's own config (values.pulumiConfig) and its top-level imports
// from a decrypted environment definition. otherValues names any values subsection other than
// pulumiConfig (e.g. environmentVariables, files) so the caller can refuse rather than silently drop it.
func parseEjectedEnvironment(
	def []byte,
) (pulumiConfig map[string]any, imports, structured, otherValues []string, err error) {
	var doc struct {
		Imports []yaml.Node          `yaml:"imports"`
		Values  map[string]yaml.Node `yaml:"values"`
	}
	if err := yaml.Unmarshal(def, &doc); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing environment definition: %w", err)
	}

	for i := range doc.Imports {
		node := &doc.Imports[i]
		name, ok := importEntryName(node)
		if !ok {
			continue
		}
		imports = append(imports, name)
		// A structured import (e.g. {env: {merge: false}}) carries options the local stack file's
		// import list cannot represent; record it so the caller can warn that only the name is kept.
		if node.Kind == yaml.MappingNode {
			structured = append(structured, name)
		}
	}

	for key, node := range doc.Values {
		if key == "pulumiConfig" {
			if err := node.Decode(&pulumiConfig); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("parsing values.pulumiConfig: %w", err)
			}
			continue
		}
		otherValues = append(otherValues, key)
	}
	sort.Strings(otherValues)
	return pulumiConfig, imports, structured, otherValues, nil
}

// buildPlaintextMap converts an ESC pulumiConfig (a Go structure with {fn::secret: plaintext} markers at
// any depth) into a config key -> Plaintext map, with secrets marked so config.EncryptMap re-encrypts
// them as nested secure values.
func buildPlaintextMap(pulumiConfig map[string]any) (map[config.Key]config.Plaintext, error) {
	result := map[config.Key]config.Plaintext{}
	for k, v := range pulumiConfig {
		key, err := config.ParseKey(k)
		if err != nil {
			return nil, fmt.Errorf("parsing config key %q: %w", k, err)
		}
		pt, err := escValueToPlaintext(v)
		if err != nil {
			return nil, err
		}
		result[key] = pt
	}
	return result, nil
}

// escValueToPlaintext maps {fn::secret: inner} markers (at any depth) to secure Plaintext, leaving
// plain values plain.
func escValueToPlaintext(v any) (config.Plaintext, error) {
	switch v := v.(type) {
	case nil:
		// Round-trip null as a null Plaintext, not the "<nil>" string the default fmt.Sprintf path gives.
		return config.Plaintext{}, nil
	case map[string]any:
		if len(v) == 1 {
			if inner, ok := v["fn::secret"]; ok {
				s, err := secretInnerString(inner)
				if err != nil {
					return config.Plaintext{}, err
				}
				return config.NewPlaintext(config.PlaintextSecret(s)), nil
			}
		}
		m := make(map[string]config.Plaintext, len(v))
		for k, e := range v {
			pt, err := escValueToPlaintext(e)
			if err != nil {
				return config.Plaintext{}, err
			}
			m[k] = pt
		}
		return config.NewPlaintext(m), nil
	case []any:
		s := make([]config.Plaintext, len(v))
		for i, e := range v {
			pt, err := escValueToPlaintext(e)
			if err != nil {
				return config.Plaintext{}, err
			}
			s[i] = pt
		}
		return config.NewPlaintext(s), nil
	case bool:
		return config.NewPlaintext(v), nil
	case int:
		return config.NewPlaintext(int64(v)), nil
	case int64:
		return config.NewPlaintext(v), nil
	case uint64:
		return config.NewPlaintext(v), nil
	case float64:
		return config.NewPlaintext(v), nil
	case string:
		return config.NewPlaintext(v), nil
	default:
		return config.NewPlaintext(fmt.Sprintf("%v", v)), nil
	}
}

// secretInnerString renders an fn::secret payload as plaintext. Composite payloads (object/array)
// must be JSON-encoded, since Go's default map/slice formatting is not valid JSON.
func secretInnerString(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case bool, int, int64, uint64, float64:
		return fmt.Sprintf("%v", v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("serializing secret value: %w", err)
		}
		return string(b), nil
	}
}

// atomicWriteProjectStack writes ps to path atomically (temp file in the same dir, then rename) so a
// failed write never leaves a half-written (plaintext) file.
func atomicWriteProjectStack(ps *workspace.ProjectStack, path string) error {
	ext := filepath.Ext(path)
	marshaler, ok := encoding.Marshalers[ext]
	if !ok {
		return fmt.Errorf("no marshaler found for file format %q", ext)
	}
	b, err := marshaler.Marshal(ps)
	if err != nil {
		return fmt.Errorf("serializing stack configuration: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pulumi-eject-*"+ext)
	if err != nil {
		return fmt.Errorf("creating temporary stack configuration file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temporary stack configuration file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary stack configuration file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil { //nolint:forbidigo // same-dir rename is atomic
		return fmt.Errorf("writing stack configuration file: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var errResp *apitype.ErrorResponse
	return errors.As(err, &errResp) && errResp.Code == http.StatusNotFound
}
