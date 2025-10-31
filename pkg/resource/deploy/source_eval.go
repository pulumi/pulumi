package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// EvalRunInfo provides information required to execute and deploy resources within a package.
type EvalRunInfo = deploy.EvalRunInfo

// EvalRunInfoOptions provides options for configuring an evaluation source.
type EvalSourceOptions = deploy.EvalSourceOptions

// A transformation function that can be applied to a resource.
type TransformFunction = deploy.TransformFunction

// A transformation function that can be applied to an invoke.
type TransformInvokeFunction = deploy.TransformInvokeFunction

type CallbacksClient = deploy.CallbacksClient

// DialOptions returns dial options to be used for the gRPC client.
type DialOptions = deploy.DialOptions

// NewEvalSource returns a planning source that fetches resources by evaluating a package with a set of args and
// a confgiuration map.  This evaluation is performed using the given plugin context and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(plugctx *plugin.Context, runinfo *EvalRunInfo, defaultProviderInfo map[tokens.Package]workspace.PackageDescriptor, resourceHooks *ResourceHooks, opts EvalSourceOptions) Source {
	return deploy.NewEvalSource(plugctx, runinfo, defaultProviderInfo, resourceHooks, opts)
}

func NewCallbacksClient(conn *grpc.ClientConn) *CallbacksClient {
	return deploy.NewCallbacksClient(conn)
}

