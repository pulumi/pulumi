package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

type Parameterization = deploy.Parameterization

// An Import specifies a resource to import.
type Import = deploy.Import

// ImportOptions controls the import process.
type ImportOptions = deploy.ImportOptions

// NewImportDeployment creates a new import deployment from a resource snapshot plus a set of resources to import.
// 
// From the old and new states, it understands how to orchestrate an evaluation and analyze the resulting resources.
// The deployment may be used to simply inspect a series of operations, or actually perform them; these operations are
// generated based on analysis of the old and new states.  If a resource exists in new, but not old, for example, it
// results in a create; if it exists in both, but is different, it results in an update; and so on and so forth.
// 
// Note that a deployment uses internal concurrency and parallelism in various ways, so it must be closed if for some
// reason it isn't carried out to its final conclusion. This will result in cancellation and reclamation of resources.
func NewImportDeployment(ctx *plugin.Context, opts *Options, events Events, target *Target, projectName tokens.PackageName, imports []Import) (*Deployment, error) {
	return deploy.NewImportDeployment(ctx, opts, events, target, projectName, imports)
}

