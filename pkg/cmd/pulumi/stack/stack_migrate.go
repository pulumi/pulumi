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

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backend_secrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type stackMigrateCmd struct {
	targetStack     string
	secretsProvider string
	yes             bool
	force           bool

	// deploymentSecretsProvider, if non-nil, is used in place of backend_secrets.DefaultProvider
	// when deserializing the source deployment. Tests use this to inject crypters whose ciphertext
	// is observable, so they can verify state secrets are re-encrypted under the target manager.
	deploymentSecretsProvider secrets.Provider
}

func newStackMigrateCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var smcmd stackMigrateCmd
	cmd := &cobra.Command{
		Use:   "migrate <url> [stack-name]",
		Short: "Migrate a stack from another backend to the currently logged-in backend",
		Long: "Migrate a stack from another backend (e.g. a DIY backend) to the currently logged-in backend.\n" +
			"\n" +
			"This command exports the source stack's checkpoint, creates a new stack with the same name on\n" +
			"the currently logged-in backend, re-encrypts any encrypted configuration values and stack\n" +
			"secrets with the target stack's secrets provider, and imports the checkpoint into the new stack.\n" +
			"The source stack's backend state is left untouched. Note: if the source and target stacks share\n" +
			"a name, the local Pulumi.<stack>.yaml file is rewritten with the target's secrets configuration\n" +
			"and would need to be restored from version control to keep using the source stack locally.\n" +
			"\n" +
			"To migrate a stack from a DIY backend (e.g. file://, s3://, azblob://, gs://) to the currently\n" +
			"logged-in Pulumi Cloud backend:\n" +
			"\n" +
			"* `pulumi stack migrate file://~ my-app-production`\n" +
			"\n" +
			"To target a specific organization on Pulumi Cloud, supply the fully qualified target stack name:\n" +
			"\n" +
			"* `pulumi stack migrate s3://my-bucket production --target acmecorp/my-app/production`\n" +
			"\n" +
			"If no stack name is given and the terminal is interactive, you will be prompted to choose one\n" +
			"from the source backend, like `pulumi stack select`.\n" +
			"\n" +
			"To use a non-default secrets provider for the target stack, pass `--secrets-provider`. Valid\n" +
			"values are the same as those accepted by `pulumi stack init`: `default`, `passphrase`, `awskms`,\n" +
			"`azurekeyvault`, `gcpkms`, `hashivault`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return smcmd.Run(cmd, ws, lm, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "url"},
			{Name: "stack-name"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().StringVar(
		&smcmd.targetStack, "target", "",
		"The name of the stack to create in the target backend. Defaults to the source stack name. "+
			"For Pulumi Cloud, may be qualified as `<org>/<project>/<stack>`",
	)
	cmd.PersistentFlags().StringVar(
		&smcmd.secretsProvider, "secrets-provider", "default", possibleSecretsProviderChoices,
	)
	cmd.PersistentFlags().BoolVarP(
		&smcmd.yes, "yes", "y", false, "Skip confirmation prompts and proceed",
	)
	cmd.PersistentFlags().BoolVarP(
		&smcmd.force, "force", "f", false,
		"Force the migration to proceed even if the source state contains resources that reference "+
			"a different stack name (typically because --target renames the stack). Mirrors "+
			"`pulumi stack import --force`.",
	)
	return cmd
}

// reusingSecretsProvider returns the cached manager when the deployment's secrets_providers
// type matches, else delegates. Avoids a second passphrase prompt / KMS round trip during
// deployment deserialization.
type reusingSecretsProvider struct {
	cached   secrets.Manager
	fallback secrets.Provider
}

func (p *reusingSecretsProvider) OfType(
	ctx context.Context, ty string, state json.RawMessage,
) (secrets.Manager, error) {
	if p.cached != nil && p.cached.Type() == ty {
		return p.cached, nil
	}
	return p.fallback.OfType(ctx, ty, state)
}

// stackConfigPath returns the on-disk Pulumi.<stack>.yaml path for the given bare stack name,
// mirroring LoadProjectStack's resolution and honoring the package-level ConfigFile override.
func stackConfigPath(name tokens.QName) string {
	if ConfigFile != "" {
		return ConfigFile
	}
	_, configPath, err := workspace.DetectProjectStackPath(name)
	if err != nil {
		return ""
	}
	return configPath
}

// snapshotConfigFile reads the current bytes at path. Returns (bytes, existed, err).
// existed=false with err=nil means the path is absent. Caller should treat read errors as fatal.
func snapshotConfigFile(path string) ([]byte, bool, error) {
	if path == "" {
		return nil, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// restoreConfigFile writes orig back to path if existed, else removes path. Best-effort; logs to w.
func restoreConfigFile(w io.Writer, path string, orig []byte, existed bool) {
	if path == "" {
		return
	}
	if existed {
		if err := os.WriteFile(path, orig, 0o600); err != nil {
			fmt.Fprintf(w, "Warning: failed to restore %s: %v\n", path, err)
		} else {
			fmt.Fprintf(w, "Restored %s to its pre-migration contents.\n", path)
		}
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(w, "Warning: failed to remove %s: %v\n", path, err)
	}
}

func (cmd *stackMigrateCmd) Run(
	cobraCmd *cobra.Command, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, args []string,
) (retErr error) {
	ctx := cobraCmd.Context()
	stdout := cobraCmd.OutOrStdout()
	stderr := cobraCmd.ErrOrStderr()
	stdin := cobraCmd.InOrStdin()
	color := cmdutil.GetGlobalColorization()
	sink := diag.DefaultSink(stdout, stderr, diag.FormatOptions{Color: color})
	dopts := display.Options{
		Color:  color,
		Stdout: stdout,
		Stdin:  stdin,
	}

	sourceURL := args[0]
	var sourceStackName string
	if len(args) >= 2 {
		sourceStackName = args[1]
	}

	if err := ValidateSecretsProvider(cmd.secretsProvider); err != nil {
		return err
	}

	project, root, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	// setCurrent=false: open source without overwriting the saved current cloud URL.
	sourceBE, err := lm.Login(
		ctx, ws, sink, sourceURL, project,
		false,
		pkgWorkspace.GetCloudInsecure(ws, sourceURL), color,
	)
	if err != nil {
		return fmt.Errorf("opening source backend %q: %w", sourceURL, err)
	}

	targetBE, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, dopts)
	if err != nil {
		return fmt.Errorf("opening target backend: %w", err)
	}

	// Backend URLs are assumed pre-normalized at login (httpstate / s3:// / azblob:// / gs://).
	// DIY `file://` URLs preserve the raw input on `b.URL()`, so equivalent forms (e.g. `file://~`
	// vs `file:///home/user`) won't match here; we accept this edge case rather than carry a
	// dedicated normalizer.
	if targetBE.URL() == sourceBE.URL() {
		return fmt.Errorf("source and target backends are the same (%s); nothing to migrate", targetBE.URL())
	}

	var sourceStack backend.Stack
	var srcRef backend.StackReference
	if sourceStackName == "" {
		// Behave like `pulumi stack select`: in interactive mode prompt the user to pick a stack
		// from the source backend; in non-interactive mode ChooseStack errors out.
		// Passing SetCurrent here is XORed off inside ChooseStack so we don't change the
		// workspace's currently-selected stack.
		s, err := ChooseStack(ctx, sink, ws, sourceBE, SetCurrent, dopts)
		if err != nil {
			return err
		}
		sourceStack = s
		srcRef = s.Ref()
	} else {
		ref, err := sourceBE.ParseStackReference(sourceStackName)
		if err != nil {
			return fmt.Errorf("parsing source stack %q: %w", sourceStackName, err)
		}
		s, err := sourceBE.GetStack(ctx, ref)
		if err != nil {
			return fmt.Errorf("looking up source stack %q in %s: %w", sourceStackName, sourceBE.Name(), err)
		}
		if s == nil {
			return fmt.Errorf("source stack %q not found in backend %s", sourceStackName, sourceBE.Name())
		}
		sourceStack = s
		srcRef = ref
	}

	// Resolve target ref + check it does not exist before any source-side work, so we don't
	// prompt for a passphrase only to error afterwards.
	targetStackName := cmd.targetStack
	if targetStackName == "" {
		targetStackName = srcRef.Name().String()
	}
	if err := targetBE.ValidateStackName(targetStackName); err != nil {
		return fmt.Errorf("invalid target stack name %q: %w", targetStackName, err)
	}
	targetRef, err := targetBE.ParseStackReference(targetStackName)
	if err != nil {
		return fmt.Errorf("parsing target stack %q: %w", targetStackName, err)
	}
	if existing, err := targetBE.GetStack(ctx, targetRef); err != nil {
		return fmt.Errorf("checking target backend for existing stack: %w", err)
	} else if existing != nil {
		return fmt.Errorf("target stack %q already exists in %s; remove it first or pick another name with --target",
			targetRef, targetBE.Name())
	}

	// Keep srcPS in memory rather than deep-copying: creating the target stack may rewrite the
	// shared Pulumi.<stack>.yaml on disk, and `deepcopy.Copy` zero-values `config.Key` /
	// `config.Value` (unexported-only structs).
	srcPS, err := LoadProjectStack(ctx, sink, project, sourceStack)
	if err != nil {
		return fmt.Errorf("loading source stack config: %w", err)
	}

	// Env-aware loader honors PULUMI_FALLBACK_TO_STATE_SECRETS_MANAGER.
	ssml := NewStackSecretsManagerLoaderFromEnv()
	var (
		oldDecrypter config.Decrypter
		srcSM        secrets.Manager // cached so DeserializeUntypedDeployment can reuse it
	)
	if srcPS.Config.HasSecureValue() {
		sm, _, smErr := ssml.GetSecretsManager(ctx, sourceStack, srcPS)
		if smErr != nil {
			return fmt.Errorf("building decrypter for source stack: %w", smErr)
		}
		srcSM = sm
		oldDecrypter = sm.Decrypter()
	} else {
		oldDecrypter = config.NewPanicCrypter()
	}

	sourceDeployment, err := backend.ExportStackDeployment(ctx, sourceStack)
	if err != nil {
		return fmt.Errorf("exporting source stack deployment: %w", err)
	}

	if !cmd.yes {
		var sameNameWarn string
		srcCfg := stackConfigPath(srcRef.Name().Q())
		tgtCfg := stackConfigPath(targetRef.Name().Q())
		if srcCfg != "" && srcCfg == tgtCfg {
			sameNameWarn = fmt.Sprintf(
				"Note: %s will be rewritten with the target's secrets configuration. The source\n"+
					"stack's state on %s is unaffected, but the local config file will need to be\n"+
					"restored from version control to keep using the source stack.\n",
				srcCfg, sourceBE.Name(),
			)
		}
		prompt := fmt.Sprintf(
			"This will migrate stack %s from %s to %s as %s.\n%sContinue?",
			sourceStack.Ref(), sourceBE.Name(), targetBE.Name(), targetRef, sameNameWarn,
		)
		if !ui.ConfirmPrompt(prompt, "yes", dopts) {
			fmt.Fprintln(stdout, "Migration cancelled")
			return nil
		}
	}

	// Snapshot both Pulumi.<stack>.yaml paths for rollback (same-name migration collapses them
	// into one). os.Stat-based detection is correct for empty files and remote-config stacks
	// where srcPS.RawValue() would be empty/nil.
	srcConfigPath := stackConfigPath(srcRef.Name().Q())
	tgtConfigPath := stackConfigPath(targetRef.Name().Q())
	srcConfigBytes, srcConfigExisted, err := snapshotConfigFile(srcConfigPath)
	if err != nil {
		return fmt.Errorf("snapshotting source stack config %s for rollback: %w", srcConfigPath, err)
	}
	sameConfigFile := tgtConfigPath != "" && tgtConfigPath == srcConfigPath
	var tgtConfigBytes []byte
	tgtConfigExisted := false
	if !sameConfigFile {
		bytes, existed, snapErr := snapshotConfigFile(tgtConfigPath)
		if snapErr != nil {
			return fmt.Errorf("snapshotting target stack config %s for rollback: %w", tgtConfigPath, snapErr)
		}
		tgtConfigBytes = bytes
		tgtConfigExisted = existed
	}

	var (
		committed     bool
		targetStack   backend.Stack
		targetCreated bool
	)
	defer func() {
		if committed || retErr == nil {
			return
		}
		// Best-effort rollback; surface failures without masking retErr.
		// Restore source first; in same-name migrations it's the only file we touched.
		restoreConfigFile(stdout, srcConfigPath, srcConfigBytes, srcConfigExisted)
		if !sameConfigFile {
			restoreConfigFile(stdout, tgtConfigPath, tgtConfigBytes, tgtConfigExisted)
		}
		if targetCreated && targetStack != nil {
			if _, rmErr := targetBE.RemoveStack(ctx, targetStack, true, false); rmErr != nil {
				fmt.Fprintf(stdout, "Warning: failed to roll back created target stack %s: %v\n", targetRef, rmErr)
				fmt.Fprintf(stdout, "Run `pulumi stack rm %s --yes --force` to clean it up manually.\n", targetRef)
			} else {
				fmt.Fprintf(stdout, "Rolled back partially-migrated target stack %s.\n", targetRef)
			}
		}
	}()

	targetStack, err = CreateStack(
		ctx, sink, ws, targetBE, targetRef, root, nil,
		false,
		cmd.secretsProvider,
		false,
	)
	if err != nil {
		// CreateStack may have created the backend stack but then failed (e.g. saving local config
		// after b.CreateStack succeeded). Recover the handle so the deferred rollback can clean up.
		// We must NOT do this when the failure is "stack already exists" or "over stack limit":
		// in those cases the existing target stack belongs to someone else (or was created by a
		// concurrent process between our preflight check and CreateStack), and force-removing it
		// would clobber unrelated work.
		var alreadyExists *backenderr.StackAlreadyExistsError
		var overLimit *backenderr.OverStackLimitError
		if !errors.As(err, &alreadyExists) && !errors.As(err, &overLimit) {
			if probe, probeErr := targetBE.GetStack(ctx, targetRef); probeErr == nil && probe != nil {
				targetStack = probe
				targetCreated = true
			}
		}
		return fmt.Errorf("creating target stack: %w", err)
	}
	targetCreated = true

	// CreateStack short-circuits the Pulumi.<stack>.yaml rewrite when target=cloud and
	// secrets-provider=default, so without this call the file would still carry the source's
	// secrets config and GetSecretsManager below would build a source-flavored encrypter.
	// creatingStack=false bypasses the same cloud-default short-circuit inside the helper.
	if err := CreateSecretsManagerForExistingStack(
		ctx, sink, ws, targetStack, cmd.secretsProvider,
		false, false,
	); err != nil {
		return fmt.Errorf("configuring target secrets provider: %w", err)
	}

	targetPS, err := LoadProjectStack(ctx, sink, project, targetStack)
	if err != nil {
		return fmt.Errorf("loading target stack config: %w", err)
	}

	newSM, _, err := ssml.GetSecretsManager(ctx, targetStack, targetPS)
	if err != nil {
		return fmt.Errorf("loading target secrets manager: %w", err)
	}
	newEncrypter := newSM.Encrypter()

	newConfig, err := srcPS.Config.Copy(oldDecrypter, newEncrypter)
	if err != nil {
		return fmt.Errorf("re-encrypting stack config: %w", err)
	}
	for key, val := range newConfig {
		if err := targetPS.Config.Set(key, val, false); err != nil {
			return fmt.Errorf("setting config key %q on target: %w", key, err)
		}
	}
	if srcPS.Environment != nil && len(srcPS.Environment.Imports()) > 0 {
		targetPS.Environment = srcPS.Environment
	}
	if err := SaveProjectStack(ctx, targetStack, targetPS); err != nil {
		return fmt.Errorf("saving target stack config: %w", err)
	}

	deploySP := cmd.deploymentSecretsProvider
	if deploySP == nil {
		// Reuse srcSM (built above for config decryption) so passphrase isn't prompted twice.
		deploySP = &reusingSecretsProvider{cached: srcSM, fallback: backend_secrets.DefaultProvider}
	}
	// SaveSnapshot validates URNs match target name, runs integrity check, clears pending ops.
	snap, err := stack.DeserializeUntypedDeployment(ctx, sourceDeployment, deploySP)
	if err != nil {
		return stack.FormatDeploymentDeserializationError(err, sourceStack.Ref().Name().String())
	}
	snap.SecretsManager = newSM
	if err := SaveSnapshot(ctx, targetStack, snap, cmd.force); err != nil {
		return fmt.Errorf("importing deployment into target stack: %w", err)
	}

	committed = true

	fmt.Fprintf(stdout, "Migrated stack %s from %s to %s.\n",
		sourceStack.Ref(), sourceBE.Name(), targetBE.Name())
	fmt.Fprintf(stdout, "Source stack state on %s is untouched.\n", sourceBE.Name())
	if sameConfigFile && srcConfigPath != "" {
		fmt.Fprintf(stdout,
			"Note: %s was rewritten with the target's secrets configuration. To keep using the\n"+
				"source stack locally, restore that file from version control before running pulumi\n"+
				"against %s.\n",
			srcConfigPath, sourceBE.Name())
	}
	return nil
}
