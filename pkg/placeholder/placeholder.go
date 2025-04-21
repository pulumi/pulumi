package placeholder

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func GetDefaultOrg(ctx context.Context, b backend.Backend, currentProject *workspace.Project) (string, error) {
	return getDefaultOrg(ctx, b, currentProject, pkgWorkspace.GetBackendConfigDefaultOrg)
}

func getDefaultOrg(
	ctx context.Context,
	b backend.Backend,
	currentProject *workspace.Project,
	getBackendConfigDefaultOrgF func(*workspace.Project) (string, error),
) (string, error) {
	userConfiguredDefaultOrg, err := getBackendConfigDefaultOrgF(currentProject)
	if err != nil || userConfiguredDefaultOrg != "" {
		return userConfiguredDefaultOrg, err
	}
	// if unset, defer to the backend's opinion of what the default org should be
	return b.GetDefaultOrg(ctx)
}
