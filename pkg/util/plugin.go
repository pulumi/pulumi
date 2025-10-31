package util

import util "github.com/pulumi/pulumi/sdk/v3/pkg/util"

// SetKnownPluginDownloadURL sets the PluginDownloadURL for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the URL.
func SetKnownPluginDownloadURL(spec *workspace.PluginSpec) bool {
	return util.SetKnownPluginDownloadURL(spec)
}

// SetKnownPluginVersion sets the Version for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the version.
func SetKnownPluginVersion(spec *workspace.PluginSpec) bool {
	return util.SetKnownPluginVersion(spec)
}

