package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A SnapshotSpec specifies a snapshot containing a set of resources managed by a set of providers.
type SnapshotSpec = fuzzing.SnapshotSpec

// A ResourceDependenciesSpec specifies the dependencies of a resource in a snapshot.
type ResourceDependenciesSpec = fuzzing.ResourceDependenciesSpec

// A set of options for configuring the generation of a SnapshotSpec.
type SnapshotSpecOptions = fuzzing.SnapshotSpecOptions

// The type of action to take when generating a resource in a snapshot.
type SnapshotSpecAction = fuzzing.SnapshotSpecAction

const SnapshotSpecNew = fuzzing.SnapshotSpecNew

const SnapshotSpecOld = fuzzing.SnapshotSpecOld

const SnapshotSpecProvider = fuzzing.SnapshotSpecProvider

// Creates a SnapshotSpec from the given deploy.Snapshot.
func FromSnapshot(s *deploy.Snapshot) *SnapshotSpec {
	return fuzzing.FromSnapshot(s)
}

// Creates a SnapshotSpec from the ResourceV3s in the given DeploymentV3.
func FromDeploymentV3(d *apitype.DeploymentV3) *SnapshotSpec {
	return fuzzing.FromDeploymentV3(d)
}

// Given a SnapshotSpec and ResourceSpec, returns a rapid.Generator that yields random (valid) sets of dependencies for
// the given resource on resources in the given snapshot.
func GeneratedResourceDependencies(ss *SnapshotSpec, r *ResourceSpec, include func(*ResourceSpec) bool) *interface{} {
	return fuzzing.GeneratedResourceDependencies(ss, r, include)
}

// Given a set of StackSpecOptions and SnapshotSpecOptions, returns a rapid.Generator that yields random SnapshotSpecs.
func GeneratedSnapshotSpec(sso StackSpecOptions, snso SnapshotSpecOptions) *interface{} {
	return fuzzing.GeneratedSnapshotSpec(sso, snso)
}

