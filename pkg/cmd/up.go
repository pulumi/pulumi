package cmd

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

type UpdateOptions struct {
	Debug       bool
	ExpectNop   bool
	Message     string
	ExecKind    string
	ExecAgent   string
	Stack       string
	ConfigArray []string
	Path        bool
	Client      string

	// Flags for engine.UpdateOptions.
	PolicyPackPaths       []string
	PolicyPackConfigPaths []string
	DiffDisplay           bool
	EventLogPath          string
	Parallel              int
	Refresh               bool
	ShowConfig            bool
	ShowReplacementSteps  bool
	ShowSames             bool
	ShowReads             bool
	SkipPreview           bool
	SuppressOutputs       bool
	SuppressPermaLink     string
	Yes                   bool
	SecretsProvider       string
	Targets               []string
	Replaces              []string
	TargetReplaces        []string
	TargetDependents      bool
}

func UpWorkingDirectory(upOpts UpdateOptions) result.Result {
	opts := backend.UpdateOptions{
		AutoApprove: upOpts.Yes,
		SkipPreview: upOpts.SkipPreview,
	}
	var displayType = display.DisplayProgress
	if upOpts.DiffDisplay {
		displayType = display.DisplayDiff
	}

	opts.Display = display.Options{
		Color:                cmdutil.GetGlobalColorization(),
		ShowConfig:           upOpts.ShowConfig,
		ShowReplacementSteps: upOpts.ShowReplacementSteps,
		ShowSameResources:    upOpts.ShowSames,
		ShowReads:            upOpts.ShowReads,
		SuppressOutputs:      upOpts.SuppressOutputs,
		IsInteractive:        cmdutil.Interactive(),
		Type:                 displayType,
		EventLogPath:         upOpts.EventLogPath,
		Debug:                upOpts.Debug,
	}

	s, err := requireStack(upOpts.Stack, true, opts.Display, false /*setCurrent*/)
	if err != nil {
		return result.FromError(err)
	}

	// Save any config values passed via flags.
	if err := parseAndSaveConfigArray(s, upOpts.ConfigArray, upOpts.Path); err != nil {
		return result.FromError(err)
	}

	proj, root, err := readProjectForUpdate(upOpts.Client)
	if err != nil {
		return result.FromError(err)
	}

	m, err := getUpdateMetadata(upOpts.Message, root, upOpts.ExecKind, upOpts.ExecAgent)
	if err != nil {
		return result.FromError(errors.Wrap(err, "gathering environment metadata"))
	}

	sm, err := getStackSecretsManager(s)
	if err != nil {
		return result.FromError(errors.Wrap(err, "getting secrets manager"))
	}

	cfg, err := getStackConfiguration(s, sm)
	if err != nil {
		return result.FromError(errors.Wrap(err, "getting stack configuration"))
	}

	targetURNs := []resource.URN{}
	for _, t := range upOpts.Targets {
		targetURNs = append(targetURNs, resource.URN(t))
	}

	replaceURNs := []resource.URN{}
	for _, r := range upOpts.Replaces {
		replaceURNs = append(replaceURNs, resource.URN(r))
	}

	for _, tr := range upOpts.TargetReplaces {
		targetURNs = append(targetURNs, resource.URN(tr))
		replaceURNs = append(replaceURNs, resource.URN(tr))
	}

	opts.Engine = engine.UpdateOptions{
		LocalPolicyPacks:          engine.MakeLocalPolicyPacks(upOpts.PolicyPackPaths, upOpts.PolicyPackConfigPaths),
		Parallel:                  upOpts.Parallel,
		Debug:                     upOpts.Debug,
		Refresh:                   upOpts.Refresh,
		RefreshTargets:            targetURNs,
		ReplaceTargets:            replaceURNs,
		UseLegacyDiff:             useLegacyDiff(),
		DisableProviderPreview:    disableProviderPreview(),
		DisableResourceReferences: disableResourceReferences(),
		UpdateTargets:             targetURNs,
		TargetDependents:          upOpts.TargetDependents,
	}

	changes, res := s.Update(commandContext(), backend.UpdateOperation{
		Proj:               proj,
		Root:               root,
		M:                  m,
		Opts:               opts,
		StackConfiguration: cfg,
		SecretsManager:     sm,
		Scopes:             cancellationScopes,
	})
	switch {
	case res != nil && res.Error() == context.Canceled:
		return result.FromError(errors.New("update cancelled"))
	case res != nil:
		return PrintEngineResult(res)
	case upOpts.ExpectNop && changes != nil && changes.HasChanges():
		return result.FromError(errors.New("error: no changes were expected but changes occurred"))
	default:
		return nil
	}
}

func getStackSecretsManager(s backend.Stack) (secrets.Manager, error) {
	ps, err := loadProjectStack(s)
	if err != nil {
		return nil, err
	}

	sm, err := func() (secrets.Manager, error) {
		if ps.SecretsProvider != passphrase.Type && ps.SecretsProvider != "default" && ps.SecretsProvider != "" {
			return newCloudSecretsManager(s.Ref().Name(), stackConfigFile, ps.SecretsProvider)
		}

		if ps.EncryptionSalt != "" {
			return newPassphraseSecretsManager(s.Ref().Name(), stackConfigFile,
				false /* rotatePassphraseSecretsProvider */)
		}

		switch s.(type) {
		case filestate.Stack:
			return newPassphraseSecretsManager(s.Ref().Name(), stackConfigFile,
				false /* rotatePassphraseSecretsProvider */)
		case httpstate.Stack:
			return newServiceSecretsManager(s.(httpstate.Stack), s.Ref().Name(), stackConfigFile)
		}

		return nil, errors.Errorf("unknown stack type %s", reflect.TypeOf(s))
	}()
	if err != nil {
		return nil, err
	}
	return stack.NewCachingSecretsManager(sm), nil
}

func getStackConfiguration(stack backend.Stack, sm secrets.Manager) (backend.StackConfiguration, error) {
	workspaceStack, err := loadProjectStack(stack)
	if err != nil {
		return backend.StackConfiguration{}, errors.Wrap(err, "loading stack configuration")
	}

	// If there are no secrets in the configuration, we should never use the decrypter, so it is safe to return
	// one which panics if it is used. This provides for some nice UX in the common case (since, for example, building
	// the correct decrypter for the local backend would involve prompting for a passphrase)
	if !workspaceStack.Config.HasSecureValue() {
		return backend.StackConfiguration{
			Config:    workspaceStack.Config,
			Decrypter: config.NewPanicCrypter(),
		}, nil
	}

	crypter, err := sm.Decrypter()
	if err != nil {
		return backend.StackConfiguration{}, errors.Wrap(err, "getting configuration decrypter")
	}

	return backend.StackConfiguration{
		Config:    workspaceStack.Config,
		Decrypter: crypter,
	}, nil
}
