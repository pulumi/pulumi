package metadata

import metadata "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/metadata"

// GetPolicyPublishMetadata returns optional data about the environment performing a
// `pulumi policy publish` command.
func GetPolicyPublishMetadata(root string) map[string]string {
	return metadata.GetPolicyPublishMetadata(root)
}

// GetUpdateMetadata returns an UpdateMetadata object, with optional data about the environment
// performing the update.
func GetUpdateMetadata(msg, root, execKind, execAgent string, updatePlan bool, cfg backend.StackConfiguration, flags *pflag.FlagSet) (*backend.UpdateMetadata, error) {
	return metadata.GetUpdateMetadata(msg, root, execKind, execAgent, updatePlan, cfg, flags)
}

