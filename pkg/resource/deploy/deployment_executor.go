// Copyright 2016-2018, Pulumi Corporation.
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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blang/semver"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"google.golang.org/grpc"
)

// deploymentExecutor is responsible for taking a deployment and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type deploymentExecutor struct {
	//deployment *Deployment // The deployment that we are executing
	// A Deployment manages the iterative computation and execution of a deployment based on a stream of goal states.
	// A running deployment emits events that indicate its progress. These events must be used to record the new state
	// of the deployment target.
	deployment *struct {
		//ctx                  *plugin.Context                  // the plugin context (for provider operations).
		ctx *struct {
			// Context is used to group related operations together so that
			// associated OS resources can be cached, shared, and reclaimed as
			// appropriate. It also carries shared plugin configuration.
			Diag       diag.Sink // the diagnostics sink to use for messages.
			StatusDiag diag.Sink // the diagnostics sink to use for status messages.
			//Host       plugin.Host // the host that can be used to fetch providers.
			Host interface {
				// A Host hosts provider plugins and makes them easily accessible by package name.

				// ServerAddr returns the address at which the host's RPC interface may be found.
				ServerAddr() string

				// Log logs a message, including errors and warnings.  Messages can have a resource URN
				// associated with them.  If no urn is provided, the message is global.
				Log(sev diag.Severity, urn resource.URN, msg string, streamID int32)

				// LogStatus logs a status message message, including errors and warnings. Status messages show
				// up in the `Info` column of the progress display, but not in the final output. Messages can
				// have a resource URN associated with them.  If no urn is provided, the message is global.
				LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32)

				// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for
				// it.  If an analyzer could not be found, or an error occurred while creating it, a non-nil
				// error is returned.
				Analyzer(nm tokens.QName) (plugin.Analyzer, error)

				// PolicyAnalyzer boots the nodejs analyzer plugin located at a given path. This is useful
				// because policy analyzers generally do not need to be "discovered" -- the engine is given a
				// set of policies that are required to be run during an update, so they tend to be in a
				// well-known place.
				PolicyAnalyzer(name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error)

				// ListAnalyzers returns a list of all analyzer plugins known to the plugin host.
				ListAnalyzers() []plugin.Analyzer

				// Provider loads a new copy of the provider for a given package.  If a provider for this package could not be
				// found, or an error occurs while creating it, a non-nil error is returned.
				Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error)
				// CloseProvider closes the given provider plugin and deregisters it from this host.
				CloseProvider(provider plugin.Provider) error
				// LanguageRuntime fetches the language runtime plugin for a given language, lazily allocating if necessary.  If
				// an implementation of this language runtime wasn't found, on an error occurs, a non-nil error is returned.
				LanguageRuntime(root, pwd, runtime string, options map[string]interface{}) (plugin.LanguageRuntime, error)

				// EnsurePlugins ensures all plugins in the given array are loaded and ready to use.  If any plugins are missing,
				// and/or there are errors loading one or more plugins, a non-nil error is returned.
				EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error
				// InstallPlugin installs a given plugin if it's not available.
				InstallPlugin(plugin workspace.PluginSpec) error

				// ResolvePlugin resolves a plugin kind, name, and optional semver to a candidate plugin to load.
				ResolvePlugin(kind workspace.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error)

				GetProjectPlugins() []workspace.ProjectPlugin

				// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing
				// operations. Operation aborted in this way will return an error (e.g., `Update` and `Create`
				// will either a creation error or an initialization error. SignalCancellation is advisory and
				// non-blocking; it is up to the host to decide how long to wait after SignalCancellation is
				// called before (e.g.) hard-closing any gRPC connection.
				SignalCancellation() error

				// Close reclaims any resources associated with the host.
				Close() error
			}
			Pwd  string // the working directory to spawn all plugins in.
			Root string // the root directory of the project.

			// If non-nil, configures custom gRPC client options. Receives pluginInfo which is a JSON-serializable bit of
			// metadata describing the plugin.
			DialOptions func(pluginInfo interface{}) []grpc.DialOption

			DebugTraceMutex *sync.Mutex // used internally to syncronize debug tracing

			tracingSpan opentracing.Span // the OpenTracing span to parent requests within.

			cancelFuncs []context.CancelFunc
			cancelLock  *sync.Mutex // Guards cancelFuncs.
			baseContext context.Context
		}
		//target               *Target                          // the deployment target.
		target *struct {
			// Target represents information about a deployment target.
			Name         tokens.Name      // the target stack name.
			Organization tokens.Name      // the target organization name (if any).
			Config       config.Map       // optional configuration key/value pairs.
			Decrypter    config.Decrypter // decrypter for secret configuration values.
			Snapshot     *Snapshot        // the last snapshot deployed to the target.
		}
		//prev                 *Snapshot                        // the old resource snapshot for comparison.
		prev *struct {
			// Snapshot is a view of a collection of resources in an stack at a point in time.  It describes resources; their
			// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
			// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
			//Manifest          Manifest             // a deployment manifest of versions, checksums, and so on.
			Manifest struct {
				// Manifest captures versions for all binaries used to construct this snapshot.
				Time    time.Time // the time this snapshot was taken.
				Magic   string    // a magic cookie.
				Version string    // the pulumi command version.
				//Plugins []workspace.PluginInfo // the plugin versions also loaded.
				Plugins []struct {
					// PluginInfo provides basic information about a plugin.  Each plugin gets installed into a system-wide
					// location, by default `~/.pulumi/plugins/<kind>-<name>-<version>/`.  A plugin may contain multiple files,
					// however the primary loadable executable must be named `pulumi-<kind>-<name>`.
					Name         string               // the simple name of the plugin.
					Path         string               // the path that a plugin was loaded from (this will always be a directory)
					Kind         workspace.PluginKind // the kind of the plugin (language, resource, etc).
					Version      *semver.Version      // the plugin's semantic version, if present.
					Size         int64                // the size of the plugin, in bytes.
					InstallTime  time.Time            // the time the plugin was installed.
					LastUsedTime time.Time            // the last time the plugin was used.
					SchemaPath   string               // if set, used as the path for loading and caching the schema
					SchemaTime   time.Time            // if set and newer than the file at SchemaPath, used to invalidate a cached schema
				}
			}
			//SecretsManager    secrets.Manager      // the manager to use use when seralizing this snapshot.
			SecretsManager interface {
				// Manager provides the interface for providing stack encryption.
				// Type retruns a string that reflects the type of this provider. This is serialized along with the state of
				// the manager into the deployment such that we can re-construct the correct manager when deserializing a
				// deployment into a snapshot.
				Type() string
				// An opaque state, which can be JSON serialized and used later to reconstruct the provider when deserializing
				// the deployment into a snapshot.
				State() interface{}
				// Encrypter returns a `config.Encrypter` that can be used to encrypt values when serializing a snapshot into a
				// deployment, or an error if one can not be constructed.
				Encrypter() (config.Encrypter, error)
				// Decrypter returns a `config.Decrypter` that can be used to decrypt values when deserializing a snapshot from a
				// deployment, or an error if one can not be constructed.
				Decrypter() (config.Decrypter, error)
			}

			//Resources         []*resource.State    // fetches all resources and their associated states.
			Resources []*struct {
				// State is a structure containing state associated with a resource.  This resource may have been serialized and
				// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
				// with any runtime objects in memory that may be actively involved in ongoing computations.
				Type                    tokens.Type                             // the resource's type.
				URN                     resource.URN                            // the resource's object urn, a human-friendly, unique name for the resource.
				Custom                  bool                                    // true if the resource is custom, managed by a plugin.
				Delete                  bool                                    // true if this resource is pending deletion due to a replacement.
				ID                      resource.ID                             // the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
				Inputs                  resource.PropertyMap                    // the resource's input properties (as specified by the program).
				Outputs                 resource.PropertyMap                    // the resource's complete output state (as returned by the resource provider).
				Parent                  resource.URN                            // an optional parent URN that this resource belongs to.
				Protect                 bool                                    // true to "protect" this resource (protected resources cannot be deleted).
				External                bool                                    // true if this resource is "external" to Pulumi and we don't control the lifecycle.
				Dependencies            []resource.URN                          // the resource's dependencies.
				InitErrors              []string                                // the set of errors encountered in the process of initializing resource.
				Provider                string                                  // the provider to use for this resource.
				PropertyDependencies    map[resource.PropertyKey][]resource.URN // the set of dependencies that affect each property.
				PendingReplacement      bool                                    // true if this resource was deleted and is awaiting replacement.
				AdditionalSecretOutputs []resource.PropertyKey                  // an additional set of outputs that should be treated as secrets.
				Aliases                 []resource.URN                          // TODO
				CustomTimeouts          resource.CustomTimeouts                 // A config block that will be used to configure timeouts for CRUD operations.
				ImportID                resource.ID                             // the resource's import id, if this was an imported resource.
				RetainOnDelete          bool                                    // if set to True, the providers Delete method will not be called for this resource.
				DeletedWith             resource.URN                            // If set, the providers Delete method will not be called for this resource if specified resource is being deleted as well.
				Created                 *time.Time                              // If set, the time when the state was initially added to the state file. (i.e. Create, Import)
				Modified                *time.Time                              // If set, the time when the state was last modified in the state file.
			}
			PendingOperations []resource.Operation // all currently pending resource operations.
		}

		olds map[resource.URN]*resource.State // a map of all old resources.
		//plan                 *Plan                            // a map of all planned resource changes, if any.
		plan *struct {
			// A Plan is a mapping from URNs to ResourcePlans. The plan defines an expected set of resources and the expected
			// inputs and operations for each. The inputs and operations are treated as constraints, and may allow for inputs or
			// operations that do not exactly match those recorded in the plan. In the case of inputs, unknown values in the plan
			// accept any value (including no value) as valid. For operations, a same step is allowed in place of an update or
			// a replace step, and an update is allowed in place of a replace step. All resource options are required to match
			// exactly.
			ResourcePlans map[resource.URN]*ResourcePlan
			//Manifest      Manifest
			Manifest struct {
				// Manifest captures versions for all binaries used to construct this snapshot.
				Time    time.Time // the time this snapshot was taken.
				Magic   string    // a magic cookie.
				Version string    // the pulumi command version.
				//Plugins []workspace.PluginInfo // the plugin versions also loaded.
				Plugins []struct {
					// PluginInfo provides basic information about a plugin.  Each plugin gets installed into a system-wide
					// location, by default `~/.pulumi/plugins/<kind>-<name>-<version>/`.  A plugin may contain multiple files,
					// however the primary loadable executable must be named `pulumi-<kind>-<name>`.
					Name         string               // the simple name of the plugin.
					Path         string               // the path that a plugin was loaded from (this will always be a directory)
					Kind         workspace.PluginKind // the kind of the plugin (language, resource, etc).
					Version      *semver.Version      // the plugin's semantic version, if present.
					Size         int64                // the size of the plugin, in bytes.
					InstallTime  time.Time            // the time the plugin was installed.
					LastUsedTime time.Time            // the last time the plugin was used.
					SchemaPath   string               // if set, used as the path for loading and caching the schema
					SchemaTime   time.Time            // if set and newer than the file at SchemaPath, used to invalidate a cached schema
				}
			}
			// The configuration in use during the plan.
			Config config.Map
		}
		//imports []Import // resources to import, if this is an import deployment.
		imports []struct {
			// An Import specifies a resource to import.
			Type              tokens.Type     // The type token for the resource. Required.
			Name              tokens.QName    // The name of the resource. Required.
			ID                resource.ID     // The ID of the resource. Required.
			Parent            resource.URN    // The parent of the resource, if any.
			Provider          resource.URN    // The specific provider to use for the resource, if any.
			Version           *semver.Version // The provider version to use for the resource, if any.
			PluginDownloadURL string          // The provider PluginDownloadURL to use for the resource, if any.
			Protect           bool            // Whether to mark the resource as protected after import
			Properties        []string        // Which properties to include (Defaults to required properties)
		}
		isImport bool // true if this is an import deployment.
		//schemaLoader         schema.Loader          // the schema cache for this deployment, if any.
		schemaLoader interface {
			LoadPackage(pkg string, version *semver.Version) (*schema.Package, error)
		}
		//source               Source                 // the source of new resources.
		source interface {
			// A Source can generate a new set of resources that the planner will process accordingly.
			io.Closer

			// Project returns the package name of the Pulumi project we are obtaining resources from.
			Project() tokens.PackageName
			// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
			Info() interface{}

			// Iterate begins iterating the source. Error is non-nil upon failure; otherwise, a valid iterator is returned.
			Iterate(ctx context.Context, opts Options, providers ProviderSource) (SourceIterator, result.Result)
		}
		localPolicyPackPaths []string // the policy packs to run during this deployment's generation.
		preview              bool     // true if this deployment is to be previewed.
		//depGraph             *graph.DependencyGraph // the dependency graph of the old snapshot.
		depGraph *struct {
			// DependencyGraph represents a dependency graph encoded within a resource snapshot.
			index      map[*resource.State]int // A mapping of resource pointers to indexes within the snapshot
			resources  []*resource.State       // The list of resources, obtained from the snapshot
			childrenOf map[resource.URN][]int  // Pre-computed map of transitive children for each resource
		}
		//providers *providers.Registry // the provider registry for this deployment.
		providers *struct {
			// Registry manages the lifecylce of provider resources and their plugins and handles the resolution of provider
			// references to loaded plugins.
			//
			// When a registry is created, it is handed the set of old provider resources that it will manage. Each provider
			// resource in this set is loaded and configured as per its recorded inputs and registered under the provider
			// reference that corresponds to its URN and ID, both of which must be known. At this point, the created registry is
			// prepared to be used to manage the lifecycle of these providers as well as any new provider resources requested by
			// invoking the registry's CRUD operations.
			//
			// In order to fit neatly in to the existing infrastructure for managing resources using Pulumi, a provider regidstry
			// itself implements the plugin.Provider interface.
			host      plugin.Host
			isPreview bool
			//providers map[providers.Reference]plugin.Provider
			providers map[providers.Reference]interface {
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
				// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
				io.Closer
				// Pkg fetches this provider's package.
				Pkg() tokens.Package

				// GetSchema returns the schema for the provider.
				GetSchema(version int) ([]byte, error)

				// CheckConfig validates the configuration for this resource provider.
				CheckConfig(urn resource.URN, olds, news resource.PropertyMap,
					allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error)
				// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
				DiffConfig(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool,
					ignoreChanges []string) (plugin.DiffResult, error)
				// Configure configures the resource provider with "globals" that control its behavior.
				Configure(inputs resource.PropertyMap) error

				// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
				// that should be passed to successive calls to Diff, Create, or Update for this resource.
				Check(urn resource.URN, olds, news resource.PropertyMap,
					allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error)
				// Diff checks what impacts a hypothetical update will have on the resource's properties.
				Diff(urn resource.URN, id resource.ID, olds resource.PropertyMap, news resource.PropertyMap,
					allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error)
				// Create allocates a new instance of the provided resource and returns its unique resource.ID.
				Create(urn resource.URN, news resource.PropertyMap, timeout float64, preview bool) (resource.ID,
					resource.PropertyMap, resource.Status, error)
				// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
				// identify the resource; this is typically just the resource ID, but may also include some properties.  If the
				// resource is missing (for instance, because it has been deleted), the resulting property map will be nil.
				Read(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (ReadResult, resource.Status, error)
				// Update updates an existing resource with new values.
				Update(urn resource.URN, id resource.ID,
					olds resource.PropertyMap, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error)
				// Delete tears down an existing resource.
				Delete(urn resource.URN, id resource.ID, props resource.PropertyMap, timeout float64) (resource.Status, error)

				// Construct creates a new component resource.
				Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN, inputs resource.PropertyMap,
					options plugin.ConstructOptions) (plugin.ConstructResult, error)

				// Invoke dynamically executes a built-in function in the provider.
				Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
				// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream
				// of responses.
				StreamInvoke(
					tok tokens.ModuleMember,
					args resource.PropertyMap,
					onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error)
				// Call dynamically executes a method in the provider associated with a component resource.
				Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
					options plugin.CallOptions) (plugin.CallResult, error)

				// GetPluginInfo returns this plugin's information.
				GetPluginInfo() (workspace.PluginInfo, error)

				// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing
				// operations. Operation aborted in this way will return an error (e.g., `Update` and `Create`
				// will either a creation error or an initialization error. SignalCancellation is advisory and
				// non-blocking; it is up to the host to decide how long to wait after SignalCancellation is
				// called before (e.g.) hard-closing any gRPC connection.
				SignalCancellation() error

				// GetMapping returns the mapping (if any) for the provider. A provider should return an empty response
				// (not an error) if it doesn't have a mapping for the given key.
				GetMapping(key string) ([]byte, string, error)
			}
			builtins plugin.Provider
			aliases  map[resource.URN]resource.URN
			m        sync.RWMutex
		}
		//goals    *goalMap       // the set of resource goals generated by the deployment.
		goals *struct {
			m sync.Map
		}
		//news *resourceMap // the set of new resources generated by the deployment
		news *struct {
			m sync.Map
		}

		//newPlans *resourcePlans // the set of new resource plans.
		newPlans *struct {
			m     sync.RWMutex
			plans Plan
		}
	}

	//stepGen  *stepGenerator // step generator owned by this deployment
	// stepGenerator is responsible for turning resource events into steps that can be fed to the deployment executor.
	// It does this by consulting the deployment and calculating the appropriate step action based on the requested goal
	// state and the existing state of the world.
	stepGen *struct {
		deployment *Deployment // the deployment to which this step generator belongs
		//opts       Options     // options for this step generator
		opts struct {
			// Options controls the deployment process.
			//Events                    Events     // an optional events callback interface.
			Events interface {
				// Events is an interface that can be used to hook interesting engine events.
				StepExecutorEvents
				PolicyEvents
			}
			Parallel    int  // the degree of parallelism for resource operations (<=1 for serial).
			Refresh     bool // whether or not to refresh before executing the deployment.
			RefreshOnly bool // whether or not to exit after refreshing.
			//RefreshTargets            UrnTargets // The specific resources to refresh during a refresh op.
			RefreshTargets struct {
				// An immutable set of urns to target with an operation.
				//
				// The zero value of UrnTargets is the set of all URNs.

				// UrnTargets is internally made up of two components: literals, which are fully
				// specified URNs and globs, which are partially specified URNs.

				literals []resource.URN
				globs    map[string]*regexp.Regexp
			}
			ReplaceTargets            UrnTargets // Specific resources to replace.
			DestroyTargets            UrnTargets // Specific resources to destroy.
			UpdateTargets             UrnTargets // Specific resources to update.
			TargetDependents          bool       // true if we're allowing things to proceed, even with unspecified targets
			TrustDependencies         bool       // whether or not to trust the resource dependency graph.
			UseLegacyDiff             bool       // whether or not to use legacy diffing behavior.
			DisableResourceReferences bool       // true to disable resource reference support.
			DisableOutputValues       bool       // true to disable output value support.
			GeneratePlan              bool       // true to enable plan generation.
		}

		updateTargetsOpt  UrnTargets // the set of resources to update; resources not in this set will be same'd
		replaceTargetsOpt UrnTargets // the set of resoures to replace

		// signals that one or more errors have been reported to the user, and the deployment should terminate
		// in error. This primarily allows `preview` to aggregate many policy violation events and
		// report them all at once.
		sawError bool

		urns     map[resource.URN]bool // set of URNs discovered for this deployment
		reads    map[resource.URN]bool // set of URNs read for this deployment
		deletes  map[resource.URN]bool // set of URNs deleted in this deployment
		replaces map[resource.URN]bool // set of URNs replaced in this deployment
		updates  map[resource.URN]bool // set of URNs updated in this deployment
		creates  map[resource.URN]bool // set of URNs created in this deployment
		sames    map[resource.URN]bool // set of URNs that were not changed in this deployment

		// set of URNs that would have been created, but were filtered out because the user didn't
		// specify them with --target
		skippedCreates map[resource.URN]bool

		pendingDeletes map[*resource.State]bool         // set of resources (not URNs!) that are pending deletion
		providers      map[resource.URN]*resource.State // URN map of providers that we have seen so far.

		// a map from URN to a list of property keys that caused the replacement of a dependent resource during a
		// delete-before-replace.
		dependentReplaceKeys map[resource.URN][]resource.PropertyKey

		// a map from old names (aliased URNs) to the new URN that aliased to them.
		aliased map[resource.URN]resource.URN
		// a map from current URN of the resource to the old URN that it was aliased from.
		aliases map[resource.URN]resource.URN
	}
	//stepExec *stepExecutor // step executor owned by this deployment
	stepExec *struct {
		// stepExecutor is the component of the engine responsible for taking steps and executing
		// them, possibly in parallel if requested. The step generator operates on the granularity
		// of "chains", which are sequences of steps that must be executed exactly in the given order.
		// Chains are a simplification of the full dependency graph DAG within Pulumi programs. Since
		// Pulumi language hosts can only invoke the resource monitor once all of their dependencies have
		// resolved, we (the engine) can assume that any chain given to us by the step generator is already
		// ready to execute.
		deployment      *Deployment // The deployment currently being executed.
		opts            Options     // The options for this current deployment.
		preview         bool        // Whether or not we are doing a preview.
		pendingNews     sync.Map    // Resources that have been created but are pending a RegisterResourceOutputs.
		continueOnError bool        // True if we want to continue the deployment after a step error.

		workers sync.WaitGroup // WaitGroup tracking the worker goroutines that are owned by this step executor.
		//incomingChains chan incomingChain // Incoming chains that we are to execute
		incomingChains chan struct {
			// incomingChain represents a request to the step executor to execute a chain.
			Chain          chain     // The chain we intend to execute
			CompletionChan chan bool // A completion channel to be closed when the chain has completed execution
		}

		ctx      context.Context    // cancellation context for the current deployment.
		cancel   context.CancelFunc // CancelFunc that cancels the above context.
		sawError atomic.Value       // atomic boolean indicating whether or not the step excecutor saw that there was an error.
	}
}

// checkTargets validates that all the targets passed in refer to existing resources.  Diagnostics
// are generated for any target that cannot be found.  The target must either have existed in the stack
// prior to running the operation, or it must be the urn for a resource that was created.
func (ex *deploymentExecutor) checkTargets(targets UrnTargets, op display.StepOp) result.Result {
	if !targets.IsConstrained() {
		return nil
	}

	olds := ex.deployment.olds
	var news map[resource.URN]bool
	if ex.stepGen != nil {
		news = ex.stepGen.urns
	}

	hasUnknownTarget := false
	for _, target := range targets.Literals() {
		hasOld := olds != nil && olds[target] != nil
		hasNew := news != nil && news[target]
		if !hasOld && !hasNew {
			hasUnknownTarget = true

			logging.V(7).Infof("Resource to %v (%v) could not be found in the stack.", op, target)
			if strings.Contains(string(target), "$") {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundError(), target)
			} else {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundDidYouForgetError(), target)
			}
		}
	}

	if hasUnknownTarget {
		return result.Bail()
	}

	return nil
}

func (ex *deploymentExecutor) printPendingOperationsWarning() {
	pendingOperations := ""
	for _, op := range ex.deployment.prev.PendingOperations {
		pendingOperations = pendingOperations + fmt.Sprintf("  * %s, interrupted while %s\n", op.Resource.URN, op.Type)
	}

	resolutionMessage := "" +
		"These resources are in an unknown state because the Pulumi CLI was interrupted while " +
		"waiting for changes to these resources to complete. You should confirm whether or not the " +
		"operations listed completed successfully by checking the state of the appropriate provider. " +
		"For example, if you are using AWS, you can confirm using the AWS Console.\n" +
		"\n" +
		"Once you have confirmed the status of the interrupted operations, you can repair your stack " +
		"using `pulumi refresh` which will refresh the state from the provider you are using and " +
		"clear the pending operations if there are any.\n" +
		"\n" +
		"Note that `pulumi refresh` will need to be run interactively to clear pending CREATE operations."

	warning := "Attempting to deploy or update resources " +
		fmt.Sprintf("with %d pending operations from previous deployment.\n", len(ex.deployment.prev.PendingOperations)) +
		pendingOperations +
		resolutionMessage

	ex.deployment.Diag().Warningf(diag.RawMessage("" /*urn*/, warning))
}

// reportExecResult issues an appropriate diagnostic depending on went wrong.
func (ex *deploymentExecutor) reportExecResult(message string, preview bool) {
	kind := "update"
	if preview {
		kind = "preview"
	}

	ex.reportError("", errors.New(kind+" "+message))
}

// reportError reports a single error to the executor's diag stream with the indicated URN for context.
func (ex *deploymentExecutor) reportError(urn resource.URN, err error) {
	ex.deployment.Diag().Errorf(diag.RawMessage(urn, err.Error()))
}

// Execute executes a deployment to completion, using the given cancellation context and running a preview
// or update.
func (ex *deploymentExecutor) Execute(callerCtx context.Context, opts Options, preview bool) (*Plan, result.Result) {
	// Set up a goroutine that will signal cancellation to the deployment's plugins if the caller context is cancelled.
	// We do not hang this off of the context we create below because we do not want the failure of a single step to
	// cause other steps to fail.
	done := make(chan bool)
	defer close(done)
	go func() {
		select {
		case <-callerCtx.Done():
			logging.V(4).Infof("deploymentExecutor.Execute(...): signalling cancellation to providers...")
			// TODO(dixler) this should be a code smell because we are reaching deep into nested data structures to do something.
			// It's hard to be sure where this value was provided in the codebase.
			cancelErr := ex.deployment.ctx.Host.SignalCancellation()
			if cancelErr != nil {
				logging.V(4).Infof("deploymentExecutor.Execute(...): failed to signal cancellation to providers: %v", cancelErr)
			}
		case <-done:
			logging.V(4).Infof("deploymentExecutor.Execute(...): exiting provider canceller")
		}
	}()

	// If this deployment is an import, run the imports and exit.
	if ex.deployment.isImport {
		return ex.importResources(callerCtx, opts, preview)
	}

	// TODO(dixler) Cosider breaking the remainder of this method into another function
	{

		// Before doing anything else, optionally refresh each resource in the base checkpoint.
		if opts.Refresh {
			res := ex.refresh(callerCtx, opts, preview)
			if res != nil {
				return nil, res
			}

			// TODO(dixler) this is an early valid exit. nil, nil is discouraged.
			if opts.RefreshOnly {
				return nil, nil
			}
			// TODO(dixler) determine what this case is. hasPendingOperations?
		} else if ex.deployment.prev != nil && len(ex.deployment.prev.PendingOperations) > 0 && !preview {
			// Print a warning for users that there are pending operations.
			// Explain that these operations can be cleared using pulumi refresh (except for CREATE operations)
			// since these require user intevention:
			ex.printPendingOperationsWarning()
		}

		// The set of -t targets provided on the command line.  'nil' means 'update everything'.
		// Non-nil means 'update only in this set'.  We don't error if the user specifies a target
		// during `update` that we don't know about because it might be the urn for a resource they
		// want to create.

		// TODO(dixler) yikes these should probably be pointers since they're optional
		updateTargetsOpt := opts.UpdateTargets
		replaceTargetsOpt := opts.ReplaceTargets
		destroyTargetsOpt := opts.DestroyTargets
		if res := ex.checkTargets(opts.ReplaceTargets, OpReplace); res != nil {
			return nil, res
		}
		if res := ex.checkTargets(opts.DestroyTargets, OpDelete); res != nil {
			return nil, res
		}

		if (updateTargetsOpt.IsConstrained() || replaceTargetsOpt.IsConstrained()) && destroyTargetsOpt.IsConstrained() {
			contract.Failf("Should not be possible to have both .DestroyTargets and .UpdateTargets or .ReplaceTargets")
		}

		// Set up a step generator for this deployment.
		ex.stepGen = newStepGenerator(ex.deployment, opts, updateTargetsOpt, replaceTargetsOpt)

		// Derive a cancellable context for this deployment. We will only cancel this context if some piece of the
		// deployment's execution fails.
		ctx, cancel := context.WithCancel(callerCtx)

		// Set up a step generator and executor for this deployment.
		ex.stepExec = newStepExecutor(ctx, cancel, ex.deployment, opts, preview, false)

		// We iterate the source in its own goroutine because iteration is blocking and we want the main loop to be able to
		// respond to cancellation requests promptly.
		type nextEvent struct {
			Event  SourceEvent
			Result result.Result
		}
		incomingEvents := make(chan nextEvent)
		{
			// TODO(dixler) consider breaking this into a function.
			// Begin iterating the source.
			src, res := ex.deployment.source.Iterate(callerCtx, opts, ex.deployment)
			if res != nil {
				return nil, res
			}

			go func() {
				for {
					event, sourceErr := src.Next()
					select {
					case incomingEvents <- nextEvent{event, sourceErr}:
						if event == nil {
							return
						}
					case <-done:
						logging.V(4).Infof("deploymentExecutor.Execute(...): incoming events goroutine exiting")
						return
					}
				}
			}()
		}

		// The main loop. We'll continuously select for incoming events and the cancellation signal. There are
		// a three ways we can exit this loop:
		//  1. The SourceIterator sends us a `nil` event. This means that we're done processing source events and
		//     we should begin processing deletes.
		//  2. The SourceIterator sends us an error. This means some error occurred in the source program and we
		//     should bail.
		//  3. The stepExecCancel cancel context gets canceled. This means some error occurred in the step executor
		//     and we need to bail. This can also happen if the user hits Ctrl-C.
		canceled, res := func() (bool, result.Result) {
			logging.V(4).Infof("deploymentExecutor.Execute(...): waiting for incoming events")
			for {
				select {
				case event := <-incomingEvents:
					logging.V(4).Infof("deploymentExecutor.Execute(...): incoming event (nil? %v, %v)", event.Event == nil,
						event.Result)

					if event.Result != nil {
						if !event.Result.IsBail() {
							ex.reportError("", event.Result.Error())
						}
						cancel()

						// We reported any errors above.  So we can just bail now.
						return false, result.Bail()
					}

					if event.Event == nil {
						res := ex.performDeletes(ctx, updateTargetsOpt, destroyTargetsOpt)
						if res != nil {
							if resErr := res.Error(); resErr != nil {
								logging.V(4).Infof("deploymentExecutor.Execute(...): error performing deletes: %v", resErr)
								ex.reportError("", resErr)
								return false, result.Bail()
							}
						}
						return false, res
					}

					if res := ex.handleSingleEvent(event.Event); res != nil {
						if resErr := res.Error(); resErr != nil {
							logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", resErr)
							ex.reportError(ex.deployment.generateEventURN(event.Event), resErr)
						}
						cancel()
						return false, result.Bail()
					}
				case <-ctx.Done():
					logging.V(4).Infof("deploymentExecutor.Execute(...): context finished: %v", ctx.Err())

					// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
					// cancellation from internally-initiated cancellation.
					return callerCtx.Err() != nil, nil
				}
			}
		}()

		ex.stepExec.WaitForCompletion()
		logging.V(4).Infof("deploymentExecutor.Execute(...): step executor has completed")

		// Now that we've performed all steps in the deployment, ensure that the list of targets to update was
		// valid.  We have to do this *after* performing the steps as the target list may have referred
		// to a resource that was created in one of the steps.
		if res == nil {
			res = ex.checkTargets(opts.UpdateTargets, OpUpdate)
		}

		// Check that we did operations for everything expected in the plan. We mutate ResourcePlan.Ops as we run
		// so by the time we get here everything in the map should have an empty ops list (except for unneeded
		// deletes). We skip this check if we already have an error, chances are if the deployment failed lots of
		// operations wouldn't have got a chance to run so we'll spam errors about all of those failed operations
		// making it less clear to the user what the root cause error was.
		if res == nil && ex.deployment.plan != nil {
			for urn, resourcePlan := range ex.deployment.plan.ResourcePlans {
				if len(resourcePlan.Ops) != 0 {
					if len(resourcePlan.Ops) == 1 && resourcePlan.Ops[0] == OpDelete {
						// We haven't done a delete for this resource check if it was in the snapshot,
						// if it's already gone this wasn't done because it wasn't needed
						found := false
						for i := range ex.deployment.prev.Resources {
							if ex.deployment.prev.Resources[i].URN == urn {
								found = true
								break
							}
						}

						// Didn't find the resource in the old snapshot so this was just an unneeded delete
						if !found {
							continue
						}
					}

					err := fmt.Errorf("expected resource operations for %v but none were seen", urn)
					logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", err)
					ex.reportError(urn, err)
					res = result.Bail()
				}
			}
		}

		if res != nil && res.IsBail() {
			return nil, res
		}

		// If the step generator and step executor were both successful, then we send all the resources
		// observed to be analyzed. Otherwise, this step is skipped.
		if res == nil && !ex.stepExec.Errored() {
			res := ex.stepGen.AnalyzeResources()
			if res != nil {
				if resErr := res.Error(); resErr != nil {
					logging.V(4).Infof("deploymentExecutor.Execute(...): error analyzing resources: %v", resErr)
					ex.reportError("", resErr)
				}
				return nil, result.Bail()
			}
		}

		// Figure out if execution failed and why. Step generation and execution errors trump cancellation.
		if res != nil || ex.stepExec.Errored() || ex.stepGen.Errored() {
			// TODO(cyrusn): We seem to be losing any information about the original 'res's errors.  Should
			// we be doing a merge here?
			ex.reportExecResult("failed", preview)
			return nil, result.Bail()
		} else if canceled {
			ex.reportExecResult("canceled", preview)
			return nil, result.Bail()
		}

		return ex.deployment.newPlans.plan(), res
	}
}

func (ex *deploymentExecutor) performDeletes(
	ctx context.Context, updateTargetsOpt, destroyTargetsOpt UrnTargets,
) result.Result {
	defer func() {
		// We're done here - signal completion so that the step executor knows to terminate.
		ex.stepExec.SignalCompletion()
	}()

	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	logging.V(7).Infof("performDeletes(...): beginning")

	// At this point we have generated the set of resources above that we would normally want to
	// delete.  However, if the user provided -target's we will only actually delete the specific
	// resources that are in the set explicitly asked for.
	var targetsOpt UrnTargets
	if updateTargetsOpt.IsConstrained() {
		targetsOpt = updateTargetsOpt
	} else if destroyTargetsOpt.IsConstrained() {
		targetsOpt = destroyTargetsOpt
	}

	deleteSteps, res := ex.stepGen.GenerateDeletes(targetsOpt)
	if res != nil {
		logging.V(7).Infof("performDeletes(...): generating deletes produced error result")
		return res
	}

	deletes := ex.stepGen.ScheduleDeletes(deleteSteps)

	// ScheduleDeletes gives us a list of lists of steps. Each list of steps can safely be executed
	// in parallel, but each list must execute completes before the next list can safely begin
	// executing.
	//
	// This is not "true" delete parallelism, since there may be resources that could safely begin
	// deleting but we won't until the previous set of deletes fully completes. This approximation
	// is conservative, but correct.
	for _, antichain := range deletes {
		logging.V(4).Infof("deploymentExecutor.Execute(...): beginning delete antichain")
		tok := ex.stepExec.ExecuteParallel(antichain)
		tok.Wait(ctx)
		logging.V(4).Infof("deploymentExecutor.Execute(...): antichain complete")
	}

	// After executing targeted deletes, we may now have resources that depend on the resource that
	// were deleted.  Go through and clean things up accordingly for them.
	if targetsOpt.IsConstrained() {
		resourceToStep := make(map[*resource.State]Step)
		for _, step := range deleteSteps {
			resourceToStep[ex.deployment.olds[step.URN()]] = step
		}

		ex.rebuildBaseState(resourceToStep, false /*refresh*/)
	}

	return nil
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (ex *deploymentExecutor) handleSingleEvent(event SourceEvent) result.Result {
	contract.Requiref(event != nil, "event", "must not be nil")

	var steps []Step
	var res result.Result
	switch e := event.(type) {
	case RegisterResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, res = ex.stepGen.GenerateSteps(e)
	case ReadResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, res = ex.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received register resource outputs")
		return ex.stepExec.ExecuteRegisterResourceOutputs(e)
	}

	if res != nil {
		return res
	}

	ex.stepExec.ExecuteSerial(steps)
	return nil
}

// import imports a list of resources into a stack.
func (ex *deploymentExecutor) importResources(
	callerCtx context.Context,
	opts Options,
	preview bool,
) (*Plan, result.Result) {
	if len(ex.deployment.imports) == 0 {
		return nil, nil
	}

	// Create an executor for this import.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, true)

	importer := &importer{
		deployment: ex.deployment,
		executor:   stepExec,
		preview:    preview,
	}
	res := importer.importResources(ctx)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	if res != nil || stepExec.Errored() {
		if res != nil && res.Error() != nil {
			ex.reportExecResult(fmt.Sprintf("failed: %s", res.Error()), preview)
		} else {
			ex.reportExecResult("failed", preview)
		}
		return nil, result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return nil, result.Bail()
	}
	return ex.deployment.newPlans.plan(), nil
}

// refresh refreshes the state of the base checkpoint file for the current deployment in memory.
func (ex *deploymentExecutor) refresh(callerCtx context.Context, opts Options, preview bool) result.Result {
	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	// Make sure if there were any targets specified, that they all refer to existing resources.
	if res := ex.checkTargets(opts.RefreshTargets, OpRefresh); res != nil {
		return res
	}

	// If the user did not provide any --target's, create a refresh step for each resource in the
	// old snapshot.  If they did provider --target's then only create refresh steps for those
	// specific targets.
	steps := []Step{}
	resourceToStep := map[*resource.State]Step{}
	for _, res := range prev.Resources {
		if opts.RefreshTargets.Contains(res.URN) {
			step := NewRefreshStep(ex.deployment, res, nil)
			steps = append(steps, step)
			resourceToStep[res] = step
		}
	}

	// Fire up a worker pool and issue each refresh in turn.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, true)
	stepExec.ExecuteParallel(steps)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	ex.rebuildBaseState(resourceToStep, true /*refresh*/)

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	if stepExec.Errored() {
		ex.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

func (ex *deploymentExecutor) rebuildBaseState(resourceToStep map[*resource.State]Step, refresh bool) {
	// Rebuild this deployment's map of old resources and dependency graph, stripping out any deleted
	// resources and repairing dependency lists as necessary. Note that this updates the base
	// snapshot _in memory_, so it is critical that any components that use the snapshot refer to
	// the same instance and avoid reading it concurrently with this rebuild.
	//
	// The process of repairing dependency lists is a bit subtle. Because multiple physical
	// resources may share a URN, the ability of a particular URN to be referenced in a dependency
	// list can change based on the dependent resource's position in the resource list. For example,
	// consider the following list of resources, where each resource is a (URN, ID, Dependencies)
	// tuple:
	//
	//     [ (A, 0, []), (B, 0, [A]), (A, 1, []), (A, 2, []), (C, 0, [A]) ]
	//
	// Let `(A, 0, [])` and `(A, 2, [])` be deleted by the refresh. This produces the following
	// intermediate list before dependency lists are repaired:
	//
	//     [ (B, 0, [A]), (A, 1, []), (C, 0, [A]) ]
	//
	// In order to repair the dependency lists, we iterate over the intermediate resource list,
	// keeping track of which URNs refer to at least one physical resource at each point in the
	// list, and remove any dependencies that refer to URNs that do not refer to any physical
	// resources. This process produces the following final list:
	//
	//     [ (B, 0, []), (A, 1, []), (C, 0, [A]) ]
	//
	// Note that the correctness of this process depends on the fact that the list of resources is a
	// topological sort of its corresponding dependency graph, so a resource always appears in the
	// list after any resources on which it may depend.
	resources := []*resource.State{}
	referenceable := make(map[resource.URN]bool)
	olds := make(map[resource.URN]*resource.State)
	for _, s := range ex.deployment.prev.Resources {
		var old, new *resource.State
		if step, has := resourceToStep[s]; has {
			// We produced a refresh step for this specific resource.  Use the new information about
			// its dependencies during the update.
			old = step.Old()
			new = step.New()
		} else {
			// We didn't do anything with this resource.  However, we still may want to update its
			// dependencies.  So use this resource itself as the 'new' one to update.
			old = s
			new = s
		}

		if new == nil {
			if refresh {
				contract.Assertf(old.Custom, "expected custom resource")
				contract.Assertf(!providers.IsProviderType(old.Type), "expected non-provider resource")
			}
			continue
		}

		// Remove any deleted resources from this resource's dependency list.
		if len(new.Dependencies) != 0 {
			deps := make([]resource.URN, 0, len(new.Dependencies))
			for _, d := range new.Dependencies {
				if referenceable[d] {
					deps = append(deps, d)
				}
			}
			new.Dependencies = deps
		}

		// Add this resource to the resource list and mark it as referenceable.
		resources = append(resources, new)
		referenceable[new.URN] = true

		// Do not record resources that are pending deletion in the "olds" lookup table.
		if !new.Delete {
			olds[new.URN] = new
		}
	}

	undangleParentResources(olds, resources)

	ex.deployment.prev.Resources = resources
	ex.deployment.olds, ex.deployment.depGraph = olds, graph.NewDependencyGraph(resources)
}

func undangleParentResources(undeleted map[resource.URN]*resource.State, resources []*resource.State) {
	// Since a refresh may delete arbitrary resources, we need to handle the case where
	// the parent of a still existing resource is deleted.
	//
	// Invalid parents need to be fixed since otherwise they leave the state invalid, and
	// the user sees an error:
	// ```
	// snapshot integrity failure; refusing to use it: child resource ${validURN} refers to missing parent ${deletedURN}
	// ```
	// To solve the problem we traverse the topologically sorted list of resources in
	// order, setting newly invalidated parent URNS to the URN of the parent's parent.
	//
	// This can be illustrated by an example. Consider the graph of resource parents:
	//
	//         A            xBx
	//       /   \           |
	//    xCx      D        xEx
	//     |     /   \       |
	//     F    G     xHx    I
	//
	// When a capital letter is marked for deletion, it is bracketed by `x`s.
	// We can obtain a topological sort by reading left to right, top to bottom.
	//
	// A..D -> valid parents, so we do nothing
	// E -> The parent of E is marked for deletion, so set E.Parent to E.Parent.Parent.
	//      Since B (E's parent) has no parent, we set E.Parent to "".
	// F -> The parent of F is marked for deletion, so set F.Parent to F.Parent.Parent.
	//      We set F.Parent to "A"
	// G, H -> valid parents, do nothing
	// I -> The parent of I is marked for deletion, so set I.Parent to I.Parent.Parent.
	//      The parent of I has parent "", (since we addressed the parent of E
	//      previously), so we set I.Parent = "".
	//
	// The new graph looks like this:
	//
	//         A        xBx   xEx   I
	//       / | \
	//     xCx F  D
	//          /   \
	//         G    xHx
	// We observe that it is perfectly valid for deleted nodes to be leaf nodes, but they
	// cannot be intermediary nodes.
	_, hasEmptyValue := undeleted[""]
	contract.Assertf(!hasEmptyValue, "the zero value for an URN is not a valid URN")
	availableParents := map[resource.URN]resource.URN{}
	for _, r := range resources {
		if _, ok := undeleted[r.Parent]; !ok {
			// Since existing must obey a topological sort, we have already addressed
			// p.Parent. Since we know that it doesn't dangle, and that r.Parent no longer
			// exists, we set r.Parent as r.Parent.Parent.
			r.Parent = availableParents[r.Parent]
		}
		availableParents[r.URN] = r.Parent
	}
}
