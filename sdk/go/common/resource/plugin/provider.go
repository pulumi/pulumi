// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"context"
	"errors"
	"io"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type GetSchemaRequest struct {
	// Version is the version of the schema to return. If omitted, the latest version of the schema should be returned.
	Version int32
	// Subpackage name to get the schema for.
	SubpackageName string
	// Subpackage version to get the schema for.
	SubpackageVersion *semver.Version
}

// The type of requests sent as part of a Handshake call.
type ProviderHandshakeRequest struct {
	// The gRPC address of the engine handshaking with the provider. At a minimum, this address will expose an instance of
	// the Engine service.
	EngineAddress string

	// A *root directory* where the provider's binary, `PulumiPlugin.yaml`, or other identifying source code is located.
	// In the event that the provider is *not* being booted by the engine (e.g. in the case that the engine has been asked
	// to attach to an existing running provider instance via a host/port number), this field will be empty.
	RootDirectory *string

	// A *program directory* in which the provider should execute. This is generally a subdirectory of the root directory,
	// though this is not required. In the event that the provider is *not* being booted by the engine (e.g. in the case
	// that the engine has been asked to attach to an existing running provider instance via a host/port number), this
	// field will be empty.
	ProgramDirectory *string

	// If true the engine will send URN, Name, Type and ID to the provider as part of the configuration.
	ConfigureWithUrn bool
}

// The type of responses sent as part of a Handshake call.
type ProviderHandshakeResponse struct{}

type ParameterizeParameters interface {
	isParameterizeParameters()
}

type (
	ParameterizeArgs struct {
		Args []string
	}

	ParameterizeValue struct {
		Name    string
		Version semver.Version
		Value   []byte
	}
)

func (*ParameterizeArgs) isParameterizeParameters()  {}
func (*ParameterizeValue) isParameterizeParameters() {}

type ParameterizeRequest struct {
	Parameters ParameterizeParameters
}

type ParameterizeResponse struct {
	Name    string
	Version semver.Version
}

type GetSchemaResponse struct {
	Schema []byte
}

type CheckConfigRequest struct {
	URN           resource.URN
	Name          string
	Type          tokens.Type
	Olds, News    resource.PropertyMap
	AllowUnknowns bool
}

type CheckConfigResponse struct {
	Properties resource.PropertyMap
	Failures   []CheckFailure
}

type DiffConfigRequest struct {
	URN                              resource.URN
	Name                             string
	Type                             tokens.Type
	OldInputs, OldOutputs, NewInputs resource.PropertyMap
	AllowUnknowns                    bool
	IgnoreChanges                    []string
}

type DiffConfigResponse = DiffResult

type ConfigureRequest struct {
	URN    *resource.URN
	Name   *string
	Type   *tokens.Type
	ID     *resource.ID
	Inputs resource.PropertyMap
}

type ConfigureResponse struct{}

// The mode that controls how the provider handles the proposed name. If not specified, defaults to `Propose`.
type AutonamingMode int32

const (
	// Propose: The provider may use the proposed name as a suggestion but is free to modify it.
	AutonamingModePropose AutonamingMode = iota
	// Enforce: The provider must use exactly the proposed name or return an error.
	AutonamingModeEnforce = 1
	// Disabled: The provider should disable automatic naming and return an error if no explicit name is provided
	// by user's program.
	AutonamingModeDisabled = 2
)

// Configuration for automatic resource naming behavior. This structure contains fields that control how the provider
// handles resource names, including proposed names and naming modes.
type AutonamingOptions struct {
	// ProposedName is the name that the provider should use for the resource.
	ProposedName string
	// Mode is the mode that controls how the provider handles the proposed name.
	Mode AutonamingMode
	// WarnIfNoSupport indicates whether the provider plugin should log a warning if the provider does not support
	// autonaming configuration.
	WarnIfNoSupport bool
}

type CheckRequest struct {
	URN  resource.URN
	Name string
	Type tokens.Type
	// TODO Change to (State, Input)
	Olds, News    resource.PropertyMap
	AllowUnknowns bool
	RandomSeed    []byte
	Autonaming    *AutonamingOptions
}

type CheckResponse struct {
	Properties resource.PropertyMap
	Failures   []CheckFailure
}

type DiffRequest struct {
	URN  resource.URN
	Name string
	Type tokens.Type
	ID   resource.ID
	// TODO Change to (OldInputs, OldState, NewInputs)
	OldInputs, OldOutputs, NewInputs resource.PropertyMap
	AllowUnknowns                    bool
	IgnoreChanges                    []string
}

type DiffResponse = DiffResult

type CreateRequest struct {
	URN        resource.URN
	Name       string
	Type       tokens.Type
	Properties resource.PropertyMap
	Timeout    float64
	Preview    bool
}

type CreateResponse struct {
	ID         resource.ID
	Properties resource.PropertyMap
	Status     resource.Status
}

type ReadRequest struct {
	URN           resource.URN
	Name          string
	Type          tokens.Type
	ID            resource.ID
	Inputs, State resource.PropertyMap
}

type ReadResponse struct {
	ReadResult
	Status resource.Status
}

type UpdateRequest struct {
	URN                              resource.URN
	Name                             string
	Type                             tokens.Type
	ID                               resource.ID
	OldInputs, OldOutputs, NewInputs resource.PropertyMap
	Timeout                          float64
	IgnoreChanges                    []string
	Preview                          bool
}

type UpdateResponse struct {
	Properties resource.PropertyMap
	Status     resource.Status
}

type DeleteRequest struct {
	URN             resource.URN
	Name            string
	Type            tokens.Type
	ID              resource.ID
	Inputs, Outputs resource.PropertyMap
	Timeout         float64
}

type DeleteResponse struct {
	Status resource.Status
}

type ConstructRequest struct {
	Info    ConstructInfo
	Type    tokens.Type
	Name    string
	Parent  resource.URN
	Inputs  resource.PropertyMap
	Options ConstructOptions
}

type ConstructResponse = ConstructResult

type InvokeRequest struct {
	Tok  tokens.ModuleMember
	Args resource.PropertyMap
}

type InvokeResponse struct {
	Properties resource.PropertyMap
	Failures   []CheckFailure
}

type StreamInvokeRequest struct {
	Tok    tokens.ModuleMember
	Args   resource.PropertyMap
	OnNext func(resource.PropertyMap) error
}

type StreamInvokeResponse struct {
	Failures []CheckFailure
}

type CallRequest struct {
	Tok     tokens.ModuleMember
	Args    resource.PropertyMap
	Info    CallInfo
	Options CallOptions
}

type CallResponse = CallResult

type GetMappingRequest struct {
	Key, Provider string
}

type GetMappingResponse struct {
	Data     []byte
	Provider string
}

type GetMappingsRequest struct {
	Key string
}

type GetMappingsResponse struct {
	Keys []string
}

// Provider presents a simple interface for orchestrating resource create, read, update, and delete operations.  Each
// provider understands how to handle all of the resource types within a single package.
//
// This interface hides some of the messiness of the underlying machinery, since providers are behind an RPC boundary.
//
// It is important to note that provider operations are not transactional.  (Some providers might decide to offer
// transactional semantics, but such a provider is a rare treat.)  As a result, failures in the operations below can
// range from benign to catastrophic (possibly leaving behind a corrupt resource).  It is up to the provider to make a
// best effort to ensure catastrophes do not occur.  The errors returned from mutating operations indicate both the
// underlying error condition in addition to a bit indicating whether the operation was successfully rolled back.
type Provider interface {
	// When adding new methods:
	//
	// To ensure maximum backwards compatibility, each method should be of the form:
	//
	//	MyMethod(ctx context.Context, request MyMethodRequest) (MyMethodResponse, error)
	//
	// This intentionally mimics the style of gRPC methods and is required to ensure that adding a new input or
	// output field doesn't break existing call sites.

	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer

	// Pkg fetches this provider's package.
	Pkg() tokens.Package

	// Handshake is the first call made by the engine to a provider. It is used to pass the engine's address to the
	// provider so that it may establish its own connections back, and to establish protocol configuration that will be
	// used to communicate between the two parties. Providers that support Handshake implicitly support the set of
	// feature flags previously handled by Configure prior to Handshake's introduction, such as secrets and resource
	// references.
	Handshake(context.Context, ProviderHandshakeRequest) (*ProviderHandshakeResponse, error)

	// Parameterize adds a sub-package to this provider instance.
	Parameterize(context.Context, ParameterizeRequest) (ParameterizeResponse, error)

	// GetSchema returns the schema for the provider.
	GetSchema(context.Context, GetSchemaRequest) (GetSchemaResponse, error)

	// CheckConfig validates the configuration for this resource provider.
	CheckConfig(context.Context, CheckConfigRequest) (CheckConfigResponse, error)
	// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
	DiffConfig(context.Context, DiffConfigRequest) (DiffConfigResponse, error)
	// Configure configures the resource provider with "globals" that control its behavior.
	Configure(context.Context, ConfigureRequest) (ConfigureResponse, error)

	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource.
	Check(context.Context, CheckRequest) (CheckResponse, error)
	// Diff checks what impacts a hypothetical update will have on the resource's properties.
	Diff(context.Context, DiffRequest) (DiffResponse, error)
	// Create allocates a new instance of the provided resource and returns its unique resource.ID.
	Create(context.Context, CreateRequest) (CreateResponse, error)
	// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
	// identify the resource; this is typically just the resource ID, but may also include some properties.  If the
	// resource is missing (for instance, because it has been deleted), the resulting property map will be nil.
	Read(context.Context, ReadRequest) (ReadResponse, error)
	// Update updates an existing resource with new values.
	Update(context.Context, UpdateRequest) (UpdateResponse, error)
	// Delete tears down an existing resource. The inputs and outputs are the last recorded ones from state.
	Delete(context.Context, DeleteRequest) (DeleteResponse, error)

	// Construct creates a new component resource.
	Construct(context.Context, ConstructRequest) (ConstructResponse, error)

	// Invoke dynamically executes a built-in function in the provider.
	Invoke(context.Context, InvokeRequest) (InvokeResponse, error)
	// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream
	// of responses.
	StreamInvoke(context.Context, StreamInvokeRequest) (StreamInvokeResponse, error)
	// Call dynamically executes a method in the provider associated with a component resource.
	Call(context.Context, CallRequest) (CallResponse, error)

	// GetPluginInfo returns this plugin's information.
	GetPluginInfo(context.Context) (workspace.PluginInfo, error)

	// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing
	// operations. Operation aborted in this way will return an error (e.g., `Update` and `Create`
	// will either a creation error or an initialization error. SignalCancellation is advisory and
	// non-blocking; it is up to the host to decide how long to wait after SignalCancellation is
	// called before (e.g.) hard-closing any gRPC connection.
	SignalCancellation(context.Context) error

	// GetMapping returns the mapping (if any) for the provider. A provider should return an empty response
	// (not an error) if it doesn't have a mapping for the given key.
	GetMapping(context.Context, GetMappingRequest) (GetMappingResponse, error)

	// GetMappings returns the mappings (if any) for the providers. A provider should return an empty list (not an
	// error) if it doesn't have any mappings for the given key.
	// If a provider implements this method GetMapping will be called using the results from this method.
	GetMappings(context.Context, GetMappingsRequest) (GetMappingsResponse, error)

	// mustEmbed *requires* that implementers make an explicit choice about forward compatibility.
	//
	// If [UnimplementedProvider] is embedded, then the struct will be forward compatible.
	//
	// If [NotForwardCompatibleProvider] is embedded, then the struct *will not* be forward compatible.
	mustEmbedAForwardCompatibilityOption(UnimplementedProvider, NotForwardCompatibleProvider)
}

type GrpcProvider interface {
	Provider

	// Attach triggers an attach for a currently running provider to the engine
	// TODO It would be nice if this was a HostClient rather than the string address but due to dependency
	// ordering we don't have access to declare that here.
	Attach(address string) error
}

// CheckFailure indicates that a call to check failed; it contains the property and reason for the failure.
type CheckFailure struct {
	Property resource.PropertyKey // the property that failed checking.
	Reason   string               // the reason the property failed to check.
}

// ErrNotYetImplemented may be returned from a provider for optional methods that are not yet implemented.
var ErrNotYetImplemented = errors.New("NYI")

// DiffChanges represents the kind of changes detected by a diff operation.
type DiffChanges int

const (
	// DiffUnknown indicates the provider didn't offer information about the changes (legacy behavior).
	DiffUnknown DiffChanges = 0
	// DiffNone indicates the provider performed a diff and concluded that no update is needed.
	DiffNone DiffChanges = 1
	// DiffSome indicates the provider performed a diff and concluded that an update or replacement is needed.
	DiffSome DiffChanges = 2
)

// DiffKind represents the kind of diff that applies to a particular property.
type DiffKind int

func (d DiffKind) String() string {
	switch d {
	case DiffAdd:
		return "add"
	case DiffAddReplace:
		return "add-replace"
	case DiffDelete:
		return "delete"
	case DiffDeleteReplace:
		return "delete-replace"
	case DiffUpdate:
		return "update"
	case DiffUpdateReplace:
		return "update-replace"
	default:
		contract.Failf("Unknown diff kind %v", int(d))
		return ""
	}
}

func (d DiffKind) IsReplace() bool {
	switch d {
	case DiffAddReplace, DiffDeleteReplace, DiffUpdateReplace:
		return true
	case DiffAdd, DiffDelete, DiffUpdate:
		return false
	}
	contract.Failf("Unknown diff kind %v", int(d))
	return false
}

// AsReplace converts a DiffKind into the equivalent replacement if it not already
// a replacement.
func (d DiffKind) AsReplace() DiffKind {
	switch d {
	case DiffAdd:
		return DiffAddReplace
	case DiffAddReplace:
		return DiffAddReplace
	case DiffDelete:
		return DiffDeleteReplace
	case DiffDeleteReplace:
		return DiffDeleteReplace
	case DiffUpdate:
		return DiffUpdateReplace
	case DiffUpdateReplace:
		return DiffUpdateReplace
	default:
		contract.Failf("Unknown diff kind %v", int(d))
		return DiffUpdateReplace
	}
}

// Invert returns the opposite diff kind to the receiver. That is:
//
//   - Add -> Delete
//   - AddReplace -> DeleteReplace
//   - Delete -> Add
//   - DeleteReplace -> AddReplace
//   - Update -> Update
//   - UpdateReplace -> UpdateReplace
func (d DiffKind) Invert() DiffKind {
	switch d {
	case DiffAdd:
		return DiffDelete
	case DiffAddReplace:
		return DiffDeleteReplace
	case DiffDelete:
		return DiffAdd
	case DiffDeleteReplace:
		return DiffAddReplace
	case DiffUpdate:
		return DiffUpdate
	case DiffUpdateReplace:
		return DiffUpdateReplace
	default:
		contract.Failf("Unknown diff kind %v", int(d))
		return DiffUpdate
	}
}

const (
	// DiffAdd indicates that the property was added.
	DiffAdd DiffKind = 0
	// DiffAddReplace indicates that the property was added and requires that the resource be replaced.
	DiffAddReplace DiffKind = 1
	// DiffDelete indicates that the property was deleted.
	DiffDelete DiffKind = 2
	// DiffDeleteReplace indicates that the property was added and requires that the resource be replaced.
	DiffDeleteReplace DiffKind = 3
	// DiffUpdate indicates that the property was updated.
	DiffUpdate DiffKind = 4
	// DiffUpdateReplace indicates that the property was updated and requires that the resource be replaced.
	DiffUpdateReplace DiffKind = 5
)

// PropertyDiff records the difference between a single property's old and new values.
type PropertyDiff struct {
	Kind      DiffKind // The kind of diff.
	InputDiff bool     // True if this is a diff between old and new inputs rather than old state and new inputs.
}

// ToReplace converts the kind of a PropertyDiff into the equivalent replacement if it not already
// a replacement.
func (p PropertyDiff) ToReplace() PropertyDiff {
	return PropertyDiff{
		InputDiff: p.InputDiff,
		Kind:      p.Kind.AsReplace(),
	}
}

// DiffResult indicates whether an operation should replace or update an existing resource.
type DiffResult struct {
	Changes             DiffChanges             // true if this diff represents a changed resource.
	ReplaceKeys         []resource.PropertyKey  // an optional list of replacement keys.
	StableKeys          []resource.PropertyKey  // an optional list of property keys that are stable.
	ChangedKeys         []resource.PropertyKey  // an optional list of keys that changed.
	DetailedDiff        map[string]PropertyDiff // an optional structured diff
	DeleteBeforeReplace bool                    // if true, this resource must be deleted before recreating it.
}

// NewDetailedDiffFromObjectDiff computes the detailed diff of Updated, Added and Deleted keys.
func NewDetailedDiffFromObjectDiff(diff *resource.ObjectDiff, inputDiff bool) map[string]PropertyDiff {
	if diff == nil {
		return map[string]PropertyDiff{}
	}
	out := map[string]PropertyDiff{}
	objectDiffToDetailedDiff(nil, diff, inputDiff, out)
	return out
}

func objectDiffToDetailedDiff(
	prefix resource.PropertyPath, diff *resource.ObjectDiff, inputDiff bool, acc map[string]PropertyDiff,
) {
	getPrefix := func(k resource.PropertyKey) resource.PropertyPath {
		return append(prefix, string(k))
	}

	for k, vd := range diff.Updates {
		nestedPrefix := getPrefix(k)
		valueDiffToDetailedDiff(nestedPrefix, vd, inputDiff, acc)
	}

	for k := range diff.Adds {
		nestedPrefix := getPrefix(k)
		acc[nestedPrefix.String()] = PropertyDiff{Kind: DiffAdd, InputDiff: inputDiff}
	}

	for k := range diff.Deletes {
		nestedPrefix := getPrefix(k)
		acc[nestedPrefix.String()] = PropertyDiff{Kind: DiffDelete, InputDiff: inputDiff}
	}
}

func arrayDiffToDetailedDiff(
	prefix resource.PropertyPath, d *resource.ArrayDiff, inputDiff bool, acc map[string]PropertyDiff,
) {
	nestedPrefix := func(i int) resource.PropertyPath {
		return append(prefix, i)
	}
	for i, vd := range d.Updates {
		valueDiffToDetailedDiff(nestedPrefix(i), vd, inputDiff, acc)
	}
	for i := range d.Adds {
		acc[nestedPrefix(i).String()] = PropertyDiff{Kind: DiffAdd, InputDiff: inputDiff}
	}
	for i := range d.Deletes {
		acc[nestedPrefix(i).String()] = PropertyDiff{Kind: DiffDelete, InputDiff: inputDiff}
	}
}

func valueDiffToDetailedDiff(
	prefix resource.PropertyPath, vd resource.ValueDiff, inputDiff bool, acc map[string]PropertyDiff,
) {
	if vd.Object != nil {
		objectDiffToDetailedDiff(prefix, vd.Object, inputDiff, acc)
	} else if vd.Array != nil {
		arrayDiffToDetailedDiff(prefix, vd.Array, inputDiff, acc)
	} else {
		switch {
		case vd.Old.V == nil && vd.New.V != nil:
			acc[prefix.String()] = PropertyDiff{Kind: DiffAdd, InputDiff: inputDiff}
		case vd.Old.V != nil && vd.New.V == nil:
			acc[prefix.String()] = PropertyDiff{Kind: DiffDelete, InputDiff: inputDiff}
		default:
			acc[prefix.String()] = PropertyDiff{Kind: DiffUpdate, InputDiff: inputDiff}
		}
	}
}

// Replace returns true if this diff represents a replacement.
func (r DiffResult) Replace() bool {
	for _, v := range r.DetailedDiff {
		if v.Kind.IsReplace() {
			return true
		}
	}
	return len(r.ReplaceKeys) > 0
}

// Invert computes the inverse diff of the receiver -- the diff that would be
// required to "undo" this one.
func (r DiffResult) Invert() DiffResult {
	detailedDiff := make(map[string]PropertyDiff)
	for k, v := range r.DetailedDiff {
		detailedDiff[k] = PropertyDiff{
			Kind:      v.Kind.Invert(),
			InputDiff: v.InputDiff,
		}
	}

	return DiffResult{
		Changes:             r.Changes,
		ReplaceKeys:         r.ReplaceKeys,
		StableKeys:          r.StableKeys,
		ChangedKeys:         r.ChangedKeys,
		DeleteBeforeReplace: r.DeleteBeforeReplace,
		DetailedDiff:        detailedDiff,
	}
}

// DiffUnavailableError may be returned by a provider if the provider is unable to diff a resource.
type DiffUnavailableError struct {
	reason string
}

// DiffUnavailable creates a new DiffUnavailableError with the given message.
func DiffUnavailable(reason string) DiffUnavailableError {
	return DiffUnavailableError{reason: reason}
}

// Error returns the error message for this DiffUnavailableError.
func (e DiffUnavailableError) Error() string {
	return e.reason
}

// ReadResult is the result of a call to Read.
type ReadResult struct {
	// This is the ID for the resource. This ID will always be populated and will ensure we get the most up-to-date
	// resource ID.
	ID resource.ID
	// Inputs contains the new inputs for the resource, if any. If this field is nil, the provider does not support
	// returning inputs from a call to Read and the old inputs (if any) should be preserved.
	Inputs resource.PropertyMap
	// Outputs contains the new outputs/state for the resource, if any. If this field is nil, the resource does not
	// exist.
	Outputs resource.PropertyMap
}

// ConstructInfo contains all of the information required to register resources as part of a call to Construct.
type ConstructInfo struct {
	Project          string                // the project name housing the program being run.
	Stack            string                // the stack name being evaluated.
	Config           map[config.Key]string // the configuration variables to apply before running.
	ConfigSecretKeys []config.Key          // the configuration keys that have secret values.
	DryRun           bool                  // true if we are performing a dry-run (preview).
	Parallel         int32                 // the degree of parallelism for resource operations (<=1 for serial).
	MonitorAddress   string                // the RPC address to the host resource monitor.
}

// ConstructOptions captures options for a call to Construct.
type ConstructOptions struct {
	// Aliases is the set of aliases for the component.
	Aliases []resource.Alias

	// Dependencies is the list of resources this component depends on.
	Dependencies []resource.URN

	// Protect is true if the component is protected.
	Protect bool

	// Providers is a map from package name to provider reference.
	Providers map[string]string

	// PropertyDependencies is a map from property name to a list of resources that property depends on.
	PropertyDependencies map[resource.PropertyKey][]resource.URN

	// AdditionalSecretOutputs lists extra output properties
	// that should be treated as secrets.
	AdditionalSecretOutputs []string

	// CustomTimeouts overrides default timeouts for resource operations.
	CustomTimeouts *CustomTimeouts

	// DeletedWith specifies that if the given resource is deleted,
	// it will also delete this resource.
	DeletedWith resource.URN

	// DeleteBeforeReplace specifies that replacements of this resource
	// should delete the old resource before creating the new resource.
	DeleteBeforeReplace bool

	// IgnoreChanges lists properties that should be ignored
	// when determining whether the resource should has changed.
	IgnoreChanges []string

	// ReplaceOnChanges lists properties changing which should cause
	// the resource to be replaced.
	ReplaceOnChanges []string

	// RetainOnDelete is true if deletion of the resource should not
	// delete the resource in the provider.
	RetainOnDelete bool
}

// CustomTimeouts overrides default timeouts for resource operations.
// Timeout values are strings in the format accepted by time.ParseDuration.
type CustomTimeouts struct {
	Create string
	Update string
	Delete string
}

// ConstructResult is the result of a call to Construct.
type ConstructResult struct {
	// The URN of the constructed component resource.
	URN resource.URN
	// The output properties of the component resource.
	Outputs resource.PropertyMap
	// The resources that each output property depends on.
	OutputDependencies map[resource.PropertyKey][]resource.URN
}

// CallInfo contains all of the information required to register resources as part of a call to Construct.
type CallInfo struct {
	Project        string                // the project name housing the program being run.
	Stack          string                // the stack name being evaluated.
	Config         map[config.Key]string // the configuration variables to apply before running.
	DryRun         bool                  // true if we are performing a dry-run (preview).
	Parallel       int32                 // the degree of parallelism for resource operations (<=1 for serial).
	MonitorAddress string                // the RPC address to the host resource monitor.
}

// CallOptions captures options for a call to Call.
type CallOptions struct {
	// ArgDependencies is a map from argument keys to a list of resources that the argument depends on.
	ArgDependencies map[resource.PropertyKey][]resource.URN
}

// CallResult is the result of a call to Call.
type CallResult struct {
	// The returned values, if the call was successful.
	Return resource.PropertyMap
	// A map from return value keys to the dependencies of the return value.
	ReturnDependencies map[resource.PropertyKey][]resource.URN
	// The failures if any arguments didn't pass verification.
	Failures []CheckFailure
}
