package cmd

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func StackSelect(stack string) (backend.Stack, error) {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}

	stackRef, stackErr := b.ParseStackReference(stack)
	if stackErr != nil {
		return nil, stackErr
	}

	s, stackErr := b.GetStack(commandContext(), stackRef)
	if stackErr != nil {
		return nil, stackErr
	}
	return s, state.SetCurrentStack(stackRef.String())
}
