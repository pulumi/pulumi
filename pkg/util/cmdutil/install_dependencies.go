package cmdutil

import cmdutil "github.com/pulumi/pulumi/sdk/v3/pkg/util/cmdutil"

// InstallDependencies installs dependencies for the given language runtime, blocking until the installation is
// complete. Standard output and error are streamed to os.Stdout and os.Stderr, respectively, and any errors encountered
// during the installation or streaming of its output are returned.
func InstallDependencies(lang plugin.LanguageRuntime, req plugin.InstallDependenciesRequest) error {
	return cmdutil.InstallDependencies(lang, req)
}

