package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

type ResultState = deploy.ResultState

// A ProviderSource allows a Source to lookup provider plugins.
type ProviderSource = deploy.ProviderSource

// A Source can generate a new set of resources that the planner will process accordingly.
type Source = deploy.Source

// A SourceIterator enumerates the list of resources that a source has to offer and tracks associated state.
type SourceIterator = deploy.SourceIterator

// SourceResourceMonitor directs resource operations from the `Source` to various resource
// providers.
type SourceResourceMonitor = deploy.SourceResourceMonitor

// SourceEvent is an event associated with the enumeration of a plan.  It is an intent expressed by the source
// program, and it is the responsibility of the engine to make it so.
type SourceEvent = deploy.SourceEvent

// RegisterResourceEvent is a step that asks the engine to provision a resource.
type RegisterResourceEvent = deploy.RegisterResourceEvent

// RegisterResult is the state of the resource after it has been registered.
type RegisterResult = deploy.RegisterResult

// RegisterResourceOutputsEvent is an event that asks the engine to complete the provisioning of a resource.
type RegisterResourceOutputsEvent = deploy.RegisterResourceOutputsEvent

// ReadResourceEvent is an event that asks the engine to read the state of an existing resource.
type ReadResourceEvent = deploy.ReadResourceEvent

type ReadResult = deploy.ReadResult

const ResultStateSuccess = deploy.ResultStateSuccess

const ResultStateFailed = deploy.ResultStateFailed

const ResultStateSkipped = deploy.ResultStateSkipped

