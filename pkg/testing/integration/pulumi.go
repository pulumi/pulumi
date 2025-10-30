package integration

import integration "github.com/pulumi/pulumi/sdk/v3/pkg/testing/integration"

// CreateBasicPulumiRepo will initialize the environment with a basic Pulumi repository and
// project file definition. Returns the repo owner and name used.
func CreateBasicPulumiRepo(e *testing.Environment) {
	integration.CreateBasicPulumiRepo(e)
}

// CreatePulumiRepo will initialize the environment with a basic Pulumi repository and
// project file definition based on the project file content.
// Returns the repo owner and name used.
func CreatePulumiRepo(e *testing.Environment, projectFileContent string) {
	integration.CreatePulumiRepo(e, projectFileContent)
}

// GetStacks returns the list of stacks and current stack by scraping `pulumi stack ls`.
// Assumes .pulumi is in the current working directory. Fails the test on IO errors.
func GetStacks(e *testing.Environment) ([]string, *string) {
	return integration.GetStacks(e)
}

