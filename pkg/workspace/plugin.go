package workspace

import workspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"

// InstallPluginError is returned by InstallPlugin if we couldn't install the plugin
type InstallPluginError = workspace.InstallPluginError

type PluginContent = workspace.PluginContent

func InstallPlugin(ctx context.Context, pluginSpec workspace.PluginSpec, log func(diag.Severity, string)) (*semver.Version, error) {
	return workspace.InstallPlugin(ctx, pluginSpec, log)
}

func SingleFilePlugin(f *os.File, spec workspace.PluginSpec) PluginContent {
	return workspace.SingleFilePlugin(f, spec)
}

func TarPlugin(tgz io.ReadCloser) PluginContent {
	return workspace.TarPlugin(tgz)
}

func DirPlugin(rootPath string) PluginContent {
	return workspace.DirPlugin(rootPath)
}

// InstallPluginContent installs a plugin's tarball into the cache. It validates that plugin names are in the expected
// format. Previous versions of Pulumi extracted the tarball to a temp directory first, and then renamed the temp
// directory to the final directory. The rename operation fails often enough on Windows due to aggressive virus scanners
// opening files in the temp directory. To address this, we now extract the tarball directly into the final directory,
// and use file locks to prevent concurrent installs.
// 
// Each plugin has its own file lock, with the same name as the plugin directory, with a `.lock` suffix.
// During installation an empty file with a `.partial` suffix is created, indicating that installation is in-progress.
// The `.partial` file is deleted when installation is complete, indicating that the plugin has finished installing.
// If a failure occurs during installation, the `.partial` file will remain, indicating the plugin wasn't fully
// installed. The next time the plugin is installed, the old installation directory will be removed and replaced with
// a fresh installation.
func InstallPluginContent(ctx context.Context, spec workspace.PluginSpec, content PluginContent, reinstall bool) error {
	return workspace.InstallPluginContent(ctx, spec, content, reinstall)
}

