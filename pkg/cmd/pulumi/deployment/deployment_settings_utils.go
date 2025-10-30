package deployment

import deployment "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/deployment"

// ValidateRelativeDirectory ensures a relative path points to a valid directory
func ValidateRelativeDirectory(rootDir string) func(string) error {
	return deployment.ValidateRelativeDirectory(rootDir)
}

func ValidateGitURL(s string) error {
	return deployment.ValidateGitURL(s)
}

func ValidateShortInputNonEmpty(s string) error {
	return deployment.ValidateShortInputNonEmpty(s)
}

func ValidateShortInput(s string) error {
	return deployment.ValidateShortInput(s)
}

