package workspace

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func GetBackendConfigDefaultOrg(project *workspace.Project) (string, error) {
	config, err := workspace.GetPulumiConfig()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	backendURL, err := workspace.GetCurrentCloudURL(project)
	if err != nil {
		return "", err
	}

	if beConfig, ok := config.BackendConfig[backendURL]; ok {
		if beConfig.DefaultOrg != "" {
			return beConfig.DefaultOrg, nil
		}
	}

	return "", nil
}
