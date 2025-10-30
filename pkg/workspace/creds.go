package workspace

import workspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"

// GetCurrentCloudURL returns the URL of the cloud we are currently connected to. This may be empty if we
// have not logged in. Note if PULUMI_BACKEND_URL is set, the corresponding value is returned
// instead irrespective of the backend for current project or stored credentials.
func GetCurrentCloudURL(ws Context, e env.Env, project *workspace.Project) (string, error) {
	return workspace.GetCurrentCloudURL(ws, e, project)
}

// GetCloudInsecure returns if this cloud url is saved as one that should use insecure transport.
func GetCloudInsecure(ws Context, cloudURL string) bool {
	return workspace.GetCloudInsecure(ws, cloudURL)
}

func GetBackendConfigDefaultOrg(project *workspace.Project) (string, error) {
	return workspace.GetBackendConfigDefaultOrg(project)
}

