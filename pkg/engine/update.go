package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// RequiredPolicy represents a set of policies to apply during an update.
type RequiredPolicy = engine.RequiredPolicy

// LocalPolicyPack represents a set of local Policy Packs to apply during an update.
type LocalPolicyPack = engine.LocalPolicyPack

// UpdateOptions contains all the settings for customizing how an update (deploy, preview, or destroy) is performed.
// 
// This structure is embedded in another which uses some of the unexported fields, which trips up the `structcheck`
// linter.
type UpdateOptions = engine.UpdateOptions

// GetLocalPolicyPackInfoFromEventName round trips the NameForEvents back into a name/path pair.
func GetLocalPolicyPackInfoFromEventName(name string) (string, string) {
	return engine.GetLocalPolicyPackInfoFromEventName(name)
}

// MakeLocalPolicyPacks is a helper function for converting the list of local Policy
// Pack paths to list of LocalPolicyPack. The name of the Local Policy Pack is not set
// since we must load up the Policy Pack plugin to determine its name.
func MakeLocalPolicyPacks(localPaths []string, configPaths []string) []LocalPolicyPack {
	return engine.MakeLocalPolicyPacks(localPaths, configPaths)
}

// ConvertLocalPolicyPacksToPaths is a helper function for converting the list of LocalPolicyPacks
// to a list of paths.
func ConvertLocalPolicyPacksToPaths(localPolicyPack []LocalPolicyPack) []string {
	return engine.ConvertLocalPolicyPacksToPaths(localPolicyPack)
}

// HasChanges returns true if there are any non-same changes in the resulting summary.
func HasChanges(changes display.ResourceChanges) bool {
	return engine.HasChanges(changes)
}

func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.Update(u, ctx, opts, dryRun)
}

