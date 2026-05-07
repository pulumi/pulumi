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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	backend_secrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

	// Test seam: overrides backend_secrets.DefaultProvider during deployment deserialize.
	deploymentSecretsProvider secrets.Provider
}

func newStackMigrateCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var smcmd stackMigrateCmd
	cmd := &cobra.Command{
		Use:   "migrate <url> [stack-name]",
		Short: "Migrate a stack from another backend to the currently logged-in backend",
		Long: "Migrate a stack from another backend (e.g. a DIY backend) to the currently logged-in backend.\n" +
			"\n" +
			"This command exports the source stack's checkpoint, creates a new stack on the currently\n" +
			"logged-in backend, re-encrypts any encrypted configuration values and stack secrets with the\n" +
			"target stack's secrets provider, and imports the checkpoint into the new stack. If --target\n" +
			"names the stack differently from the source, every URN in the imported state is rewritten to\n" +
			"reference the new name. The source stack's backend state is left untouched.\n" +
			"\n" +
			"Note: if the source and target stacks share a name, the local Pulumi.<stack>.yaml file is\n" +
			"rewritten with the target's secrets configuration. The pre-migration content is saved as\n" +
			"`Pulumi.<stack>.yaml.bak` so you can recover the source's secrets metadata if needed.\n" +
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
		"Force the migration to proceed even if the source state fails integrity checks or "+
			"contains resources from a foreign stack. Mirrors `pulumi stack import --force`.",
	)
	return cmd
}

// shouldForceTargetSecretsRewrite returns true when CreateStack short-circuits the
// Pulumi.<stack>.yaml rewrite (cloud target + default provider), so the migrate command needs
// to call CreateSecretsManagerForExistingStack itself. Other paths configure target's SM
// inline and would re-prompt for passphrase if called again.
func shouldForceTargetSecretsRewrite(b backend.Backend, secretsProvider string) bool {
	isDefault := secretsProvider == "" || secretsProvider == "default"
	if !isDefault {
		return false
	}
	_, isCloud := b.(httpstate.Backend)
	return isCloud
}

// reusingSecretsProvider returns cached when type+state match, else delegates. Avoids a second
// passphrase prompt / KMS round trip. State match guards against same-type-different-key cases.
type reusingSecretsProvider struct {
	cached   secrets.Manager
	fallback secrets.Provider
}

func (p *reusingSecretsProvider) OfType(
	ctx context.Context, ty string, state json.RawMessage,
) (secrets.Manager, error) {
	if p.cached != nil && p.cached.Type() == ty && bytes.Equal(p.cached.State(), state) {
		return p.cached, nil
	}
	return p.fallback.OfType(ctx, ty, state)
}

// stackConfigPath returns Pulumi.<stack>.yaml's path, honoring the ConfigFile override.
func stackConfigPath(name tokens.QName) (string, error) {
	if ConfigFile != "" {
		return ConfigFile, nil
	}
	_, configPath, err := workspace.DetectProjectStackPath(name)
	if err != nil {
		return "", fmt.Errorf("detecting project stack path for %q: %w", name, err)
	}
	return configPath, nil
}

// nextBackupPath returns the first `<path>.bak` / `.bak.N` that does not yet exist.
func nextBackupPath(path string) string {
	candidate := path + ".bak"
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; i < 1000; i++ {
		candidate = fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	// Pathological case: thousand backups already on disk. Suffix with nanos to stay unique.
	return fmt.Sprintf("%s.bak.%d", path, time.Now().UnixNano())
}

// snapshotConfigFile reads bytes+mode at path. existed=false with no err means absent.
func snapshotConfigFile(path string) ([]byte, os.FileMode, bool, error) {
	if path == "" {
		return nil, 0, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.Mode().Perm(), true, nil
}

// renameSnapshotStack rewrites URNs in snap to point at newName (+newProject if set). Covers
// every URN-bearing field on resource.State and ResourceReferences nested in Inputs/Outputs/
// ReplacementTrigger. When oldProject is non-empty, only URNs whose project matches it are
// rewritten (so foreign-project URNs sharing a stack name pass through untouched). force=true
// degrades provider-reference rewrite errors to warnings on w, mirroring SaveSnapshot's --force.
func renameSnapshotStack(
	snap *deploy.Snapshot,
	oldName, newName tokens.StackName,
	oldProject, newProject tokens.PackageName,
	force bool,
	w io.Writer,
) error {
	if snap == nil || (oldName == newName && newProject == "") {
		return nil
	}
	rewrite := func(u resource.URN) resource.URN {
		if u == "" || u.Stack() != oldName.Q() {
			return u
		}
		if oldProject != "" && u.Project() != oldProject {
			return u
		}
		proj := u.Project()
		if newProject != "" {
			proj = newProject
		}
		// Root URN's name is `<project>-<stack>`; rebuild instead of URN.RenameStack.
		if u.QualifiedType() == resource.RootStackType {
			return resource.NewURN(newName.Q(), proj, "", u.QualifiedType(),
				string(tokens.QName(proj)+"-"+newName.Q()))
		}
		return resource.NewURN(newName.Q(), proj, "", u.QualifiedType(), u.Name())
	}
	rewriteState := func(res *resource.State) error {
		if res == nil {
			return nil
		}
		res.URN = rewrite(res.URN)
		if res.Parent != "" {
			res.Parent = rewrite(res.Parent)
		}
		for i := range res.Dependencies {
			res.Dependencies[i] = rewrite(res.Dependencies[i])
		}
		for _, deps := range res.PropertyDependencies {
			for i := range deps {
				deps[i] = rewrite(deps[i])
			}
		}
		if res.DeletedWith != "" {
			res.DeletedWith = rewrite(res.DeletedWith)
		}
		for i := range res.ReplaceWith {
			res.ReplaceWith[i] = rewrite(res.ReplaceWith[i])
		}
		for i := range res.Aliases {
			res.Aliases[i] = rewrite(res.Aliases[i])
		}
		if res.ViewOf != "" {
			res.ViewOf = rewrite(res.ViewOf)
		}
		if res.Provider != "" {
			if err := rewriteProviderRef(res, rewrite, force, w); err != nil {
				return err
			}
		}
		rewritePropertyMap(res.Inputs, rewrite)
		rewritePropertyMap(res.Outputs, rewrite)
		res.ReplacementTrigger = rewritePropertyValue(res.ReplacementTrigger, rewrite)
		return nil
	}
	for _, res := range snap.Resources {
		if err := rewriteState(res); err != nil {
			return err
		}
	}
	for i := range snap.PendingOperations {
		if err := rewriteState(snap.PendingOperations[i].Resource); err != nil {
			return err
		}
	}
	return nil
}

// rewriteProviderRef parses + reissues res.Provider with a rewritten URN. With force=true,
// parse/rebuild failures become warnings on w (consistent with SaveSnapshot --force).
func rewriteProviderRef(
	res *resource.State,
	rewrite func(resource.URN) resource.URN,
	force bool,
	w io.Writer,
) error {
	ref, err := providers.ParseReference(res.Provider)
	if err != nil {
		if force {
			fmt.Fprintf(w, "Warning: parsing provider reference for %q: %v\n", res.URN, err)
			return nil
		}
		return fmt.Errorf("parsing provider reference for %q: %w", res.URN, err)
	}
	newRef, err := providers.NewReference(rewrite(ref.URN()), ref.ID())
	if err != nil {
		if force {
			fmt.Fprintf(w, "Warning: rebuilding provider reference for %q: %v\n", res.URN, err)
			return nil
		}
		return fmt.Errorf("rebuilding provider reference for %q: %w", res.URN, err)
	}
	res.Provider = newRef.String()
	return nil
}

// rewritePropertyMap walks m, rewriting any nested ResourceReference URNs.
func rewritePropertyMap(m resource.PropertyMap, rewrite func(resource.URN) resource.URN) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = rewritePropertyValue(v, rewrite)
	}
}

// rewritePropertyValue rewrites ResourceReference URNs, recursing into arrays/objects/secrets/outputs.
func rewritePropertyValue(v resource.PropertyValue, rewrite func(resource.URN) resource.URN) resource.PropertyValue {
	switch {
	case v.IsResourceReference():
		ref := v.ResourceReferenceValue()
		ref.URN = rewrite(ref.URN)
		return resource.NewProperty(ref)
	case v.IsArray():
		arr := v.ArrayValue()
		for i := range arr {
			arr[i] = rewritePropertyValue(arr[i], rewrite)
		}
		return v
	case v.IsObject():
		rewritePropertyMap(v.ObjectValue(), rewrite)
		return v
	case v.IsSecret():
		s := v.SecretValue()
		s.Element = rewritePropertyValue(s.Element, rewrite)
		return v
	case v.IsOutput():
		o := v.OutputValue()
		o.Element = rewritePropertyValue(o.Element, rewrite)
		for i := range o.Dependencies {
			o.Dependencies[i] = rewrite(o.Dependencies[i])
		}
		return resource.NewProperty(o)
	default:
		return v
	}
}

// restoreConfigFile writes orig back at mode if existed, else removes path. Best-effort.
// Returns true on success so callers can decide whether redundant artifacts (e.g. a `.bak`)
// are safe to clean up.
func restoreConfigFile(w io.Writer, path string, orig []byte, mode os.FileMode, existed bool) bool {
	if path == "" {
		return true
	}
	if existed {
		if err := os.WriteFile(path, orig, mode); err != nil {
			fmt.Fprintf(w, "Warning: failed to restore %s: %v\n", path, err)
			return false
		}
		fmt.Fprintf(w, "Restored %s to its pre-migration contents.\n", path)
		return true
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(w, "Warning: failed to remove %s: %v\n", path, err)
		return false
	}
	return true
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

	// Backend URLs are pre-normalized at login except for DIY `file://`, which keeps the raw
	// input. `file://~` vs `file:///home/user` is a known edge case we accept.
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

	// Resolve target ref + check it doesn't exist before source-side work to fail fast.
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

	// Keep srcPS in memory: deepcopy.Copy zero-values config.Key/Value (unexported-only structs).
	srcPS, err := LoadProjectStack(ctx, sink, project, sourceStack)
	if err != nil {
		return fmt.Errorf("loading source stack config: %w", err)
	}

	// Snapshot source + target ps paths for rollback (collapse to one in same-name case).
	// We snapshot BEFORE the confirmation prompt so the prompt's same-name "backup will be
	// saved" message is accurate even when the source ps file is missing on disk.
	srcConfigPath, err := stackConfigPath(srcRef.Name().Q())
	if err != nil {
		return err
	}
	tgtConfigPath, err := stackConfigPath(targetRef.Name().Q())
	if err != nil {
		return err
	}
	srcConfigBytes, srcConfigMode, srcConfigExisted, err := snapshotConfigFile(srcConfigPath)
	if err != nil {
		return fmt.Errorf("snapshotting source stack config %s for rollback: %w", srcConfigPath, err)
	}
	sameConfigFile := tgtConfigPath != "" && tgtConfigPath == srcConfigPath

	if !cmd.yes {
		var sameNameWarn string
		if sameConfigFile && srcConfigPath != "" {
			if srcConfigExisted {
				sameNameWarn = fmt.Sprintf(
					"Note: %s will be rewritten with the target's secrets configuration. A copy of\n"+
						"the current file is saved as a sibling `.bak` so you can keep using the source\n"+
						"stack locally. The source stack's state on %s is unaffected.\n",
					srcConfigPath, sourceBE.Name(),
				)
			} else {
				sameNameWarn = fmt.Sprintf(
					"Note: %s will be created with the target's secrets configuration.\n"+
						"The source stack's state on %s is unaffected.\n",
					srcConfigPath, sourceBE.Name(),
				)
			}
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

	// Source-side network / passphrase work runs AFTER confirm so cancels are cheap.
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

	// Same-name migration overwrites Pulumi.<stack>.yaml; drop a `.bak` first. nextBackupPath
	// avoids clobbering an earlier .bak. Failure here is fatal: prompt promised the backup.
	var bakPath string
	if sameConfigFile && srcConfigExisted {
		bakPath = nextBackupPath(srcConfigPath)
		if err := os.WriteFile(bakPath, srcConfigBytes, srcConfigMode); err != nil {
			return fmt.Errorf("writing backup %s: %w", bakPath, err)
		}
		fmt.Fprintf(stdout, "Backed up %s to %s\n", srcConfigPath, bakPath)
	}
	var (
		tgtConfigBytes   []byte
		tgtConfigMode    os.FileMode
		tgtConfigExisted bool
	)
	if !sameConfigFile {
		tgtBytes, tgtMode, existed, snapErr := snapshotConfigFile(tgtConfigPath)
		if snapErr != nil {
			return fmt.Errorf("snapshotting target stack config %s for rollback: %w", tgtConfigPath, snapErr)
		}
		tgtConfigBytes = tgtBytes
		tgtConfigMode = tgtMode
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
		srcRestored := restoreConfigFile(stdout, srcConfigPath, srcConfigBytes, srcConfigMode, srcConfigExisted)
		if !sameConfigFile {
			restoreConfigFile(stdout, tgtConfigPath, tgtConfigBytes, tgtConfigMode, tgtConfigExisted)
		}
		// `.bak` is the only off-disk copy of the source config, so only remove it once we've
		// confirmed the source ps file was restored successfully. Otherwise leave it as the
		// recoverable copy and tell the user where it lives.
		if bakPath != "" {
			if srcRestored {
				if rmErr := os.Remove(bakPath); rmErr != nil && !os.IsNotExist(rmErr) {
					fmt.Fprintf(stdout, "Warning: failed to remove backup %s: %v\n", bakPath, rmErr)
				}
			} else {
				fmt.Fprintf(stdout, "Pre-migration source config preserved at %s.\n", bakPath)
			}
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
		// Only adopt-and-rollback when ErrSaveStackConfig signals b.CreateStack succeeded.
		// Other failures (AlreadyExists / OverLimit / network / SM construct) might leave a
		// stack we don't own; force-removing it would clobber unrelated work.
		if errors.Is(err, ErrSaveStackConfig) {
			if probe, probeErr := targetBE.GetStack(ctx, targetRef); probeErr == nil && probe != nil {
				targetStack = probe
				targetCreated = true
			}
		}
		return fmt.Errorf("creating target stack: %w", err)
	}
	targetCreated = true

	// Cloud + default is the one case CreateStack short-circuits the ps rewrite; force it here
	// so the file reflects target's secrets config. Other paths (passphrase / KMS / DIY default)
	// already wrote target's SM via createSecretsManagerForNewStack, so calling this again would
	// re-prompt for passphrase.
	if shouldForceTargetSecretsRewrite(targetBE, cmd.secretsProvider) {
		if err := CreateSecretsManagerForExistingStack(
			ctx, sink, ws, targetStack, cmd.secretsProvider,
			false, false,
		); err != nil {
			return fmt.Errorf("configuring target secrets provider: %w", err)
		}
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
	// Replace, not merge: a pre-existing Pulumi.<target>.yaml may have stale keys.
	// SecretsProvider/EncryptionSalt/EncryptedKey live on separate fields and survive.
	targetPS.Config = newConfig
	targetPS.Environment = srcPS.Environment
	if err := SaveProjectStack(ctx, targetStack, targetPS); err != nil {
		return fmt.Errorf("saving target stack config: %w", err)
	}

	deploySP := cmd.deploymentSecretsProvider
	if deploySP == nil {
		// Reuse srcSM (built above for config decryption) so passphrase isn't prompted twice.
		deploySP = &reusingSecretsProvider{cached: srcSM, fallback: backend_secrets.DefaultProvider}
	}
	// SaveSnapshot validates URNs, runs integrity check, clears pending ops.
	snap, err := stack.DeserializeUntypedDeployment(ctx, sourceDeployment, deploySP)
	if err != nil {
		return stack.FormatDeploymentDeserializationError(err, sourceStack.Ref().Name().String())
	}
	// Rewrite URNs when --target changes name/project so SaveSnapshot's URN check passes.
	// Use targetStack.Ref() (not the parsed targetRef) because the backend may canonicalize
	// the ref returned from CreateStack and SaveSnapshot validates against that canonical form.
	// oldProject filters which URNs get rewritten so foreign-project URNs sharing a stack name
	// pass through. Legacy DIY StackReferences don't expose Project(), so fall back to the
	// local Pulumi.yaml project name when the backend doesn't carry one.
	var oldProject, newProject tokens.PackageName
	if srcProj, ok := srcRef.Project(); ok {
		oldProject = tokens.PackageName(srcProj)
	} else if project != nil {
		oldProject = project.Name
	}
	tgtStackRef := targetStack.Ref()
	if tgtProj, ok := tgtStackRef.Project(); ok {
		if oldProject == "" || oldProject != tokens.PackageName(tgtProj) {
			newProject = tokens.PackageName(tgtProj)
		}
	}
	if err := renameSnapshotStack(
		snap, srcRef.Name(), tgtStackRef.Name(), oldProject, newProject, cmd.force, stdout,
	); err != nil {
		return fmt.Errorf("rewriting URNs for target stack: %w", err)
	}
	snap.SecretsManager = newSM
	if err := SaveSnapshot(ctx, targetStack, snap, cmd.force); err != nil {
		return fmt.Errorf("importing deployment into target stack: %w", err)
	}

	committed = true

	fmt.Fprintf(stdout, "Migrated stack %s from %s to %s.\n",
		sourceStack.Ref(), sourceBE.Name(), targetBE.Name())
	fmt.Fprintf(stdout, "Source stack state on %s is untouched.\n", sourceBE.Name())
	if bakPath != "" {
		fmt.Fprintf(stdout,
			"Note: %s was rewritten with the target's secrets configuration. The pre-migration\n"+
				"contents are saved at %s.\n",
			srcConfigPath, bakPath)
	}
	return nil
}
