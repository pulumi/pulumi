package edit

import edit "github.com/pulumi/pulumi/sdk/v3/pkg/resource/edit"

// OperationFunc is the type of functions that edit resources within a snapshot. The edits are made in-place to the
// given snapshot and pertain to the specific passed-in resource.
type OperationFunc = edit.OperationFunc

// DeleteResource deletes a given resource from the snapshot, if it is possible to do so.
// 
// If targetDependents is true, dependents will also be deleted. Otherwise an error
// instance of `ResourceHasDependenciesError` will be returned.
// 
// If non-nil, onProtected will be called on all protected resources planed for deletion.
// 
// If a resource is marked protected after onProtected is called, an error instance of
// `ResourceHasDependenciesError` will be returned.
func DeleteResource(snapshot *deploy.Snapshot, condemnedRes *resource.State, onProtected func(*resource.State) error, targetDependents bool) error {
	return edit.DeleteResource(snapshot, condemnedRes, onProtected, targetDependents)
}

// LocateResource returns all resources in the given snapshot that have the given URN.
func LocateResource(snap *deploy.Snapshot, urn resource.URN) []*resource.State {
	return edit.LocateResource(snap, urn)
}

// RenameStack changes the `stackName` component of every URN in a deployment. In addition, it rewrites the name of
// the root Stack resource itself. May optionally change the project/package name as well.
func RenameStack(deployment *apitype.DeploymentV3, newName tokens.StackName, newProject tokens.PackageName) error {
	return edit.RenameStack(deployment, newName, newProject)
}

