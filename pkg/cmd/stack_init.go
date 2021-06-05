package cmd

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func StackInit(stackName, secretsProvider string) (backend.Stack, error) {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}

	if err := b.ValidateStackName(stackName); err != nil {
		return nil, err
	}

	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	var createOpts interface{} // Backend-specific config options, none currently.
	return createStack(b, stackRef, createOpts, true /*setCurrent*/, secretsProvider)
}
