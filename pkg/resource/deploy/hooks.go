package deploy

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Callback is the function signature for resource lifecycle hooks.
type Callback func(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	name string,
	typ tokens.Type,
	newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
) error
