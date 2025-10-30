package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

func NewErrorSource(project tokens.PackageName) Source {
	return deploy.NewErrorSource(project)
}

