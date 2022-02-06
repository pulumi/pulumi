package refresher

import (
	"context"
	"errors"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	metadataMessage = "Firefly's Scan"
)

type Client struct {
	URL string
	Ctx context.Context
	Opts backend.UpdateOptions
	Project *workspace.Project
}

func NewClient(ctx context.Context, url string) (*Client, error ) {
	var merr *multierror.Error
	var err error
	var project *workspace.Project
	if project, err = getProject(); err != nil {
		merr = multierror.Append(merr, errors.New("failed getting current project"))
	}

	var c = Client{
		URL: url,
		Ctx: ctx,
		Opts: getOpts(),
		Project: project,
	}
	return &c, nil
}


func (client *Client)GetHttpBackend(backend httpstate.Backend, url string) (*httpstate.CloudBackend) {
	httpCloudBackend := httpstate.CloudBackend{
		D:              cmdutil.Diag(),
		Url:            url,
		BackendClient:  backend.Client(),
		CurrentProject: client.Project,
	}
	return &httpCloudBackend
}

func (client *Client) GetUpdateOpts() *backend.UpdateOperation {
	metadata := backend.UpdateMetadata{Message: metadataMessage, Environment: nil}
	imports := []deploy.Import{}

	updateOpts := backend.UpdateOperation{
		Proj:               client.Project,
		Root:               "",
		Imports:            imports,
		M:                  &metadata,
		Opts:               client.Opts,
		SecretsManager:     nil,
		StackConfiguration: backend.StackConfiguration{},
		Scopes:             backend.CancellationScopeSource(cancellationScopeSource(0)),
	}
	return &updateOpts

}

func (client *Client) GetDryRunApplierOpts() *backend.ApplierOptions {
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return &opts
}

func getOpts() backend.UpdateOptions{
	var opts backend.UpdateOptions
	opts.Display = display.Options{
		Color:                cmdutil.GetGlobalColorization(),
		ShowConfig:           false,
		ShowReplacementSteps: true,
		ShowSameResources:    true,
		SuppressOutputs:      false,
		IsInteractive:        false,
		Type:                 display.DisplayDiff,
		EventLogPath:         "./",
		Debug:                false,
		JSONDisplay:          true,
	}

	var policyPackPaths []string
	var policyPackConfigPaths []string
	replaceURNs := []resource.URN{}
	targetURNs := []resource.URN{}

	opts.Engine = engine.UpdateOptions{
		LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
		Parallel:                  0,
		Debug:                     false,
		Refresh:                   false,
		RefreshTargets:            nil,
		ReplaceTargets:            replaceURNs,
		DestroyTargets:            nil,
		UpdateTargets:             targetURNs,
		TargetDependents:          false,
		UseLegacyDiff:             false,
		DisableProviderPreview:    false, //TODO- true
		DisableResourceReferences: false,
		DisableOutputValues:       false,
	}
	return opts
}

func getProject() (*workspace.Project, error) {
	project, err := workspace.DetectProject()
	if err != nil {
		return nil, err
	}
	return project, err
}