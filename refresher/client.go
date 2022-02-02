package refresher

import (
	"context"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type Client struct {
	URL string
	Ctx context.Context
	Opts backend.UpdateOptions
}

func NewClient(ctx context.Context, url string) *Client {
	var c = Client{
		URL: url,
		Ctx: ctx,
		Opts: getOpts(),
	}
	return &c
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