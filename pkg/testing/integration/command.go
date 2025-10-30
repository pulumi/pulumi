package integration

import integration "github.com/pulumi/pulumi/sdk/v3/pkg/testing/integration"

func RunCommand(t *testing.T, name string, args []string, wd string, opts *ProgramTestOptions) error {
	return integration.RunCommand(t, name, args, wd, opts)
}

// RunCommandPulumiHome executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in, and sets the PULUMI_HOME
// environment variable.
func RunCommandPulumiHome(t *testing.T, name string, args []string, wd string, opts *ProgramTestOptions, pulumiHome string) error {
	return integration.RunCommandPulumiHome(t, name, args, wd, opts, pulumiHome)
}

