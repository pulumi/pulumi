package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// StackReference is an opaque type that refers to a stack managed by a backend.  The CLI uses the ParseStackReference
// method to turn a string like "my-great-stack" or "pulumi/my-great-stack" into a stack reference that can be used to
// interact with the stack via the backend. Stack references are specific to a given backend and different back ends
// may interpret the string passed to ParseStackReference differently.
type StackReference = backend.StackReference

// PolicyPackReference is an opaque type that refers to a PolicyPack managed by a backend. The CLI
// uses the ParsePolicyPackReference method to turn a string like "myOrg/mySecurityRules" into a
// PolicyPackReference that can be used to interact with the PolicyPack via the backend.
// PolicyPackReferences are specific to a given backend and different back ends may interpret the
// string passed to ParsePolicyPackReference differently.
type PolicyPackReference = backend.PolicyPackReference

// StackSummary provides a basic description of a stack, without the ability to inspect its resources or make changes.
type StackSummary = backend.StackSummary

// ListStacksFilter describes optional filters when listing stacks.
type ListStacksFilter = backend.ListStacksFilter

// ListStackNamesFilter describes optional filters when listing stack names.
// This filter does not contain tag fields since they cannot be efficiently
// implemented for the DIY backend.
type ListStackNamesFilter = backend.ListStackNamesFilter

// ContinuationToken is an opaque string used for paginated backend requests. If non-nil, means
// there are more results to be returned and the continuation token should be passed into a
// subsequent call to the backend method. A nil continuation token means all results have been
// returned.
type ContinuationToken = backend.ContinuationToken

// Backend is the contract between the Pulumi engine and pluggable backend implementations of the Pulumi Cloud Service.
type Backend = backend.Backend

// EnvironmentsBackend is an interface that defines an optional capability for a backend to work with environments.
type EnvironmentsBackend = backend.EnvironmentsBackend

// SpecificDeploymentExporter is an interface defining an additional capability of a Backend, specifically the
// ability to export a specific versions of a stack's deployment. This isn't a requirement for all backends and
// should be checked for dynamically.
type SpecificDeploymentExporter = backend.SpecificDeploymentExporter

// UpdateOperation is a complete stack update operation (preview, update, import, refresh, or destroy).
type UpdateOperation = backend.UpdateOperation

// StackConfiguration holds the configuration for a stack and it's associated decrypter.
type StackConfiguration = backend.StackConfiguration

// UpdateOptions is the full set of update options, including backend and engine options.
type UpdateOptions = backend.UpdateOptions

// CancellationScope provides a scoped source of cancellation and termination requests.
type CancellationScope = backend.CancellationScope

// CancellationScopeSource provides a source for cancellation scopes.
type CancellationScopeSource = backend.CancellationScopeSource

// CreateStackOptions provides options for stack creation.
// At present, options only apply to the Service.
type CreateStackOptions = backend.CreateStackOptions

// TarReaderCloser is a [tar.Reader] that owns it's backing memory.
// 
// Calling close invalidates the [tar.Reader] returned by Tar.
type TarReaderCloser = backend.TarReaderCloser

// ErrTeamsNotSupported is returned by backends
// which do not support the teams feature.
var ErrTeamsNotSupported = backend.ErrTeamsNotSupported

// ErrTeamsNotSupported is returned by backends
// which do not support the teams feature.
var ErrConfigNotSupported = backend.ErrConfigNotSupported

// Confirm the specified stack's project doesn't contradict the name of the current project.
func CurrentProjectContradictsWorkspace(proj *workspace.Project, stack StackReference) error {
	return backend.CurrentProjectContradictsWorkspace(proj, stack)
}

// NewBackendClient returns a deploy.BackendClient that wraps the given Backend.
func NewBackendClient(backend_ Backend, secretsProvider secrets.Provider) deploy.BackendClient {
	return backend.NewBackendClient(backend_, secretsProvider)
}

