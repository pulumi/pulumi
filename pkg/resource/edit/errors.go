package edit

import edit "github.com/pulumi/pulumi/sdk/v3/pkg/resource/edit"

// ResourceHasDependenciesError is returned by DeleteResource if a resource can't be deleted due to the presence of
// resources that depend directly or indirectly upon it.
type ResourceHasDependenciesError = edit.ResourceHasDependenciesError

// ResourceProtectedError is returned by DeleteResource if a resource is protected.
type ResourceProtectedError = edit.ResourceProtectedError

