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
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backend_secrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type stackMigrateCmd struct {
	stdout io.Writer

	from            string
	sourceStack     string
	targetStack     string
	secretsProvider string
	yes             bool
	force           bool
}

func newStackMigrateCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var smcmd stackMigrateCmd
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a stack from another backend to the currently logged-in backend",
		Long: "Migrate a stack from another backend (e.g. a DIY backend) to the currently logged-in backend.\n" +
			"\n" +
			"This command exports the source stack's checkpoint, creates a new stack with the same name on\n" +
			"the currently logged-in backend, re-encrypts any encrypted configuration values and stack\n" +
			"secrets with the target stack's secrets provider, and imports the checkpoint into the new stack.\n" +
			"The source stack is left untouched.\n" +
			"\n" +
			"To migrate a stack from a DIY backend (e.g. file://, s3://, azblob://, gs://) to the currently\n" +
			"logged-in Pulumi Cloud backend:\n" +
			"\n" +
			"* `pulumi stack migrate --from file://~ my-app-production`\n" +
			"\n" +
			"To target a specific organization on Pulumi Cloud, supply the fully qualified target stack name:\n" +
			"\n" +
			"* `pulumi stack migrate --from s3://my-bucket --target acmecorp/my-app/production production`\n" +
			"\n" +
			"To use a non-default secrets provider for the target stack, pass `--secrets-provider`. Valid\n" +
			"values are the same as those accepted by `pulumi stack init`: `default`, `passphrase`, `awskms`,\n" +
			"`azurekeyvault`, `gcpkms`, `hashivault`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return smcmd.Run(ctx, ws, lm, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "source-stack-name"},
		},
		Required: 0,
	})

	cmd.PersistentFlags().StringVar(
		&smcmd.from, "from", "",
		"The URL of the source backend to migrate from (e.g. file://~, s3://my-bucket, https://api.pulumi.com)",
	)
	cmd.PersistentFlags().StringVar(
		&smcmd.sourceStack, "source-stack", "",
		"The name of the stack to migrate from the source backend. "+
			"Defaults to the positional argument or the currently selected stack",
	)
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

// normalizeBackendURL canonicalizes a backend URL so equivalent forms (e.g. `file://~` and
// `file:///home/user`, trailing slashes) compare equal in the same-backend guard.
func normalizeBackendURL(s string) string {
	if s == "" {
		return s
	}
	s = strings.TrimRight(s, "/")
	if path, ok := strings.CutPrefix(s, "file://"); ok {
		if rest, hasTilde := strings.CutPrefix(path, "~"); hasTilde {
			if home, err := os.UserHomeDir(); err == nil {
				path = home + rest
			}
		}
		return "file://" + filepath.Clean(path)
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" {
		u.Path = strings.TrimRight(u.Path, "/")
		return u.String()
	}
	return s
}

// stackConfigFilePath mirrors LoadProjectStack's path resolution, honoring the package-level
// ConfigFile override.
func stackConfigFilePath(s backend.Stack) string {
	if ConfigFile != "" {
		return ConfigFile
	}
	_, configPath, err := workspace.DetectProjectStackPath(s.Ref().Name().Q())
	if err != nil {
		return ""
	}
	return configPath
}

func (cmd *stackMigrateCmd) Run(
	ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, args []string,
) (retErr error) {
	stdout := cmd.stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	sink := cmdutil.Diag()
	dopts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	if cmd.from == "" {
		return errors.New("--from is required: the URL of the source backend to migrate from")
	}

	if err := ValidateSecretsProvider(cmd.secretsProvider); err != nil {
		return err
	}

	sourceStackName := cmd.sourceStack
	if len(args) > 0 {
		if sourceStackName != "" {
			return errors.New("only one of --source-stack or the positional source stack name may be specified")
		}
		sourceStackName = args[0]
	}

	project, root, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	// setCurrent=false: open source without overwriting the saved current cloud URL.
	sourceInsecure := pkgWorkspace.GetCloudInsecure(ws, cmd.from)
	sourceBE, err := lm.Login(
		ctx, ws, sink, cmd.from, project,
		false,
		sourceInsecure, dopts.Color,
	)
	if err != nil {
		return fmt.Errorf("opening source backend %q: %w", cmd.from, err)
	}

	targetBE, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, dopts)
	if err != nil {
		return fmt.Errorf("opening target backend: %w", err)
	}

	if normalizeBackendURL(targetBE.URL()) == normalizeBackendURL(sourceBE.URL()) {
		return fmt.Errorf("source and target backends are the same (%s); nothing to migrate", targetBE.URL())
	}

	var sourceStack backend.Stack
	var srcRef backend.StackReference
	if sourceStackName == "" {
		s, err := state.CurrentStack(ctx, ws, sourceBE)
		if err != nil {
			return fmt.Errorf("looking up currently selected stack on source backend: %w", err)
		}
		if s == nil {
			return errors.New("no source stack: provide one as a positional argument, via --source-stack, " +
				"or by selecting one with `pulumi stack select`")
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

	// We keep srcPS in memory rather than deep-copying because creating the target stack
	// rewrites the shared Pulumi.<stack>.yaml on disk, and `deepcopy.Copy` zero-values
	// `config.Key`/`config.Value` (unexported-only structs).
	srcPS, err := LoadProjectStack(ctx, sink, project, sourceStack)
	if err != nil {
		return fmt.Errorf("loading source stack config: %w", err)
	}

	// Env-aware loader honors PULUMI_FALLBACK_TO_STATE_SECRETS_MANAGER.
	ssml := NewStackSecretsManagerLoaderFromEnv()
	var oldDecrypter config.Decrypter
	if srcPS.Config.HasSecureValue() {
		dec, _, derr := ssml.GetDecrypter(ctx, sourceStack, srcPS)
		if derr != nil {
			return fmt.Errorf("building decrypter for source stack: %w", derr)
		}
		oldDecrypter = dec
	} else {
		oldDecrypter = config.NewPanicCrypter()
	}

	sourceDeployment, err := backend.ExportStackDeployment(ctx, sourceStack)
	if err != nil {
		return fmt.Errorf("exporting source stack deployment: %w", err)
	}

	if !cmd.yes {
		prompt := fmt.Sprintf(
			"This will migrate stack %s from %s to %s as %s.\n"+
				"The source stack will be left untouched.\n"+
				"Continue?",
			sourceStack.Ref(), sourceBE.Name(), targetBE.Name(), targetRef,
		)
		if !ui.ConfirmPrompt(prompt, "yes", dopts) {
			fmt.Fprintln(stdout, "Migration cancelled")
			return nil
		}
	}

	// Same-name source and target share Pulumi.<stack>.yaml; snapshot it for rollback so
	// a partial failure can't strand the source's secrets undecryptable.
	configPath := stackConfigFilePath(sourceStack)
	var origConfigBytes []byte
	configExisted := false
	if configPath != "" {
		if data, readErr := os.ReadFile(configPath); readErr == nil {
			origConfigBytes = data
			configExisted = true
		} else if !os.IsNotExist(readErr) {
			return fmt.Errorf("snapshotting stack config %s for rollback: %w", configPath, readErr)
		}
	}

	var (
		committed     bool
		targetStack   backend.Stack
		targetCreated bool
		mutatedConfig bool
	)
	defer func() {
		if committed || retErr == nil {
			return
		}
		// Best-effort rollback; surface failures without masking retErr.
		if mutatedConfig && configPath != "" {
			if configExisted {
				if wErr := os.WriteFile(configPath, origConfigBytes, 0o600); wErr != nil {
					fmt.Fprintf(stdout, "Warning: failed to restore %s: %v\n", configPath, wErr)
				} else {
					fmt.Fprintf(stdout, "Restored %s to its pre-migration contents.\n", configPath)
				}
			} else {
				if rErr := os.Remove(configPath); rErr != nil && !os.IsNotExist(rErr) {
					fmt.Fprintf(stdout, "Warning: failed to remove %s: %v\n", configPath, rErr)
				}
			}
		}
		if targetCreated && targetStack != nil {
			if _, rmErr := targetBE.RemoveStack(ctx, targetStack, true /*force*/, false /*removeBackups*/); rmErr != nil {
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
		return fmt.Errorf("creating target stack: %w", err)
	}
	targetCreated = true
	mutatedConfig = true

	// CreateStack short-circuits the Pulumi.<stack>.yaml rewrite when target=cloud and
	// secrets-provider=default, so without this call the file would still carry the source's
	// secrets config and GetSecretsManager below would build a source-flavored encrypter.
	// creatingStack=false bypasses the same cloud-default short-circuit inside the helper.
	if err := CreateSecretsManagerForExistingStack(
		ctx, sink, ws, targetStack, cmd.secretsProvider,
		false,
		false,
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

	// SaveSnapshot validates URNs match target name, runs integrity check, clears pending ops.
	snap, err := stack.DeserializeUntypedDeployment(ctx, sourceDeployment, backend_secrets.DefaultProvider)
	if err != nil {
		return stack.FormatDeploymentDeserializationError(err, sourceStack.Ref().Name().String())
	}
	contract.Assertf(snap != nil, "deserialized snapshot must not be nil")
	snap.SecretsManager = newSM
	if err := SaveSnapshot(ctx, targetStack, snap, cmd.force); err != nil {
		return fmt.Errorf("importing deployment into target stack: %w", err)
	}

	committed = true

	fmt.Fprintf(stdout, "Migrated stack %s from %s to %s.\n",
		sourceStack.Ref(), sourceBE.Name(), targetBE.Name())
	fmt.Fprintf(stdout, "Source stack %s on %s was left untouched.\n",
		sourceStack.Ref(), sourceBE.Name())
	return nil
}
