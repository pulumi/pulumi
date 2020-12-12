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
	"fmt"
	"os"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

// EvalRunInfo provides information required to execute and deploy resources within a package.
type EvalRunInfo struct {
	Proj    *workspace.Project `json:"proj" yaml:"proj"`                         // the package metadata.
	Pwd     string             `json:"pwd" yaml:"pwd"`                           // the package's working directory.
	Program string             `json:"program" yaml:"program"`                   // the path to the program.
	Args    []string           `json:"args,omitempty" yaml:"args,omitempty"`     // any arguments to pass to the package.
	Target  *Target            `json:"target,omitempty" yaml:"target,omitempty"` // the target being deployed into.
}

// NewEvalSource returns a planning source that fetches resources by evaluating a package with a set of args and
// a confgiuration map.  This evaluation is performed using the given plugin context and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(plugctx *plugin.Context, runinfo *EvalRunInfo,
	defaultProviderVersions map[tokens.Package]*semver.Version, dryRun bool) Source {

	return &evalSource{
		plugctx:                 plugctx,
		runinfo:                 runinfo,
		defaultProviderVersions: defaultProviderVersions,
		dryRun:                  dryRun,
	}
}

type evalSource struct {
	plugctx                 *plugin.Context                    // the plugin context.
	runinfo                 *EvalRunInfo                       // the directives to use when running the program.
	defaultProviderVersions map[tokens.Package]*semver.Version // the default provider versions for this source.
	dryRun                  bool                               // true if this is a dry-run operation only.
}

func (src *evalSource) Close() error {
	return nil
}

// Project is the name of the project being run by this evaluation source.
func (src *evalSource) Project() tokens.PackageName {
	return src.runinfo.Proj.Name
}

// Stack is the name of the stack being targeted by this evaluation source.
func (src *evalSource) Stack() tokens.QName {
	return src.runinfo.Target.Name
}

func (src *evalSource) Info() interface{} { return src.runinfo }

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(
	ctx context.Context, opts Options, providers ProviderSource) (SourceIterator, result.Result) {

	tracingSpan := opentracing.SpanFromContext(ctx)

	// Decrypt the configuration.
	config, err := src.runinfo.Target.Config.Decrypt(src.runinfo.Target.Decrypter)
	if err != nil {
		return nil, result.FromError(errors.Wrap(err, "failed to decrypt config"))
	}

	// First, fire up a resource monitor that will watch for and record resource creation.
	regChan := make(chan *registerResourceEvent)
	regOutChan := make(chan *registerResourceOutputsEvent)
	regReadChan := make(chan *readResourceEvent)
	mon, err := newResourceMonitor(src, providers, regChan, regOutChan, regReadChan, opts, config, tracingSpan)
	if err != nil {
		return nil, result.FromError(errors.Wrap(err, "failed to start resource monitor"))
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		mon:         mon,
		src:         src,
		regChan:     regChan,
		regOutChan:  regOutChan,
		regReadChan: regReadChan,
		finChan:     make(chan result.Result),
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(opts, config)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	mon         SourceResourceMonitor              // the resource monitor, per iterator.
	src         *evalSource                        // the owning eval source object.
	regChan     chan *registerResourceEvent        // the channel that contains resource registrations.
	regOutChan  chan *registerResourceOutputsEvent // the channel that contains resource completions.
	regReadChan chan *readResourceEvent            // the channel that contains read resource requests.
	finChan     chan result.Result                 // the channel that communicates completion.
	done        bool                               // set to true when the evaluation is done.
}

func (iter *evalSourceIterator) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	return iter.mon.Cancel()
}

func (iter *evalSourceIterator) ResourceMonitor() SourceResourceMonitor {
	return iter.mon
}

func (iter *evalSourceIterator) Next() (SourceEvent, result.Result) {
	// If we are done, quit.
	if iter.done {
		return nil, nil
	}

	// Await the program to compute some more state and then inspect what it has to say.
	select {
	case reg := <-iter.regChan:
		contract.Assert(reg != nil)
		goal := reg.Goal()
		logging.V(5).Infof("EvalSourceIterator produced a registration: t=%v,name=%v,#props=%v",
			goal.Type, goal.Name, len(goal.Properties))
		return reg, nil
	case regOut := <-iter.regOutChan:
		contract.Assert(regOut != nil)
		logging.V(5).Infof("EvalSourceIterator produced a completion: urn=%v,#outs=%v",
			regOut.URN(), len(regOut.Outputs()))
		return regOut, nil
	case read := <-iter.regReadChan:
		contract.Assert(read != nil)
		logging.V(5).Infoln("EvalSourceIterator produced a read")
		return read, nil
	case res := <-iter.finChan:
		// If we are finished, we can safely exit.  The contract with the language provider is that this implies
		// that the language runtime has exited and so calling Close on the plugin is fine.
		iter.done = true
		if res != nil {
			if res.IsBail() {
				logging.V(5).Infof("EvalSourceIterator ended with bail.")
			} else {
				logging.V(5).Infof("EvalSourceIterator ended with an error: %v", res.Error())
			}
		}
		return nil, res
	}
}

// forkRun performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func (iter *evalSourceIterator) forkRun(opts Options, config map[config.Key]string) {
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go func() {
		// Next, launch the language plugin.
		run := func() result.Result {
			rt := iter.src.runinfo.Proj.Runtime.Name()
			langhost, err := iter.src.plugctx.Host.LanguageRuntime(rt)
			if err != nil {
				return result.FromError(errors.Wrapf(err, "failed to launch language host %s", rt))
			}
			contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

			// Make sure to clean up before exiting.
			defer contract.IgnoreClose(langhost)

			// Now run the actual program.
			progerr, bail, err := langhost.Run(plugin.RunInfo{
				MonitorAddress: iter.mon.Address(),
				Stack:          string(iter.src.runinfo.Target.Name),
				Project:        string(iter.src.runinfo.Proj.Name),
				Pwd:            iter.src.runinfo.Pwd,
				Program:        iter.src.runinfo.Program,
				Args:           iter.src.runinfo.Args,
				Config:         config,
				DryRun:         iter.src.dryRun,
				Parallel:       opts.Parallel,
			})

			// Check if we were asked to Bail.  This a special random constant used for that
			// purpose.
			if err == nil && bail {
				return result.Bail()
			}

			if err == nil && progerr != "" {
				// If the program had an unhandled error; propagate it to the caller.
				err = errors.Errorf("an unhandled error occurred: %v", progerr)
			}
			return result.WrapIfNonNil(err)
		}

		// Communicate the error, if it exists, or nil if the program exited cleanly.
		iter.finChan <- run()
	}()
}

// defaultProviders manages the registration of default providers. The default provider for a package is the provider
// resource that will be used to manage resources that do not explicitly reference a provider. Default providers will
// only be registered for packages that are used by resources registered by the user's Pulumi program.
type defaultProviders struct {
	// A map of package identifiers to versions, used to disambiguate which plugin to load if no version is provided
	// by the language host.
	defaultVersions map[tokens.Package]*semver.Version

	// A map of ProviderRequest strings to provider references, used to keep track of the set of default providers that
	// have already been loaded.
	providers map[string]providers.Reference
	config    plugin.ConfigSource

	requests        chan defaultProviderRequest
	providerRegChan chan<- *registerResourceEvent
	cancel          <-chan bool
}

type defaultProviderResponse struct {
	ref providers.Reference
	err error
}

type defaultProviderRequest struct {
	req      providers.ProviderRequest
	response chan<- defaultProviderResponse
}

// newRegisterDefaultProviderEvent creates a RegisterResourceEvent and completion channel that can be sent to the
// engine to register a default provider resource for the indicated package.
func (d *defaultProviders) newRegisterDefaultProviderEvent(
	req providers.ProviderRequest) (*registerResourceEvent, <-chan *RegisterResult, error) {

	// Attempt to get the config for the package.
	inputs, err := d.config.GetPackageConfig(req.Package())
	if err != nil {
		return nil, nil, err
	}

	// Request that the engine instantiate a specific version of this provider, if one was requested. We'll figure out
	// what version to request by:
	//   1. Providing the Version field of the ProviderRequest verbatim, if it was provided, otherwise
	//   2. Querying the list of default versions provided to us on startup and returning the value associated with
	//      the given package, if one exists, otherwise
	//   3. We give nothing to the engine and let the engine figure it out.
	//
	// As we tighen up our approach to provider versioning, 2 and 3 will go away and be replaced entirely by 1. 3 is
	// especially onerous because the engine selects the "newest" plugin available on the machine, which is generally
	// problematic for a lot of reasons.
	if req.Version() != nil {
		logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): using version %s from request", req, req.Version())
		inputs["version"] = resource.NewStringProperty(req.Version().String())
	} else {
		logging.V(5).Infof(
			"newRegisterDefaultProviderEvent(%s): no version specified, falling back to default version", req)
		if version := d.defaultVersions[req.Package()]; version != nil {
			logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): default version hit on version %s", req, version)
			inputs["version"] = resource.NewStringProperty(version.String())
		} else {
			logging.V(5).Infof(
				"newRegisterDefaultProviderEvent(%s): default provider miss, sending nil version to engine", req)
		}
	}

	// Create the result channel and the event.
	done := make(chan *RegisterResult)
	event := &registerResourceEvent{
		goal: resource.NewGoal(
			providers.MakeProviderType(req.Package()),
			req.Name(), true, inputs, "", false, nil, "", nil, nil, nil, nil, nil, nil, "", nil),
		done: done,
	}
	return event, done, nil
}

// handleRequest services a single default provider request. If the request is for a default provider that we have
// already loaded, we will return its reference. If the request is for a default provider that has not yet been
// loaded, we will send a register resource request to the engine, wait for it to complete, and then cache and return
// the reference of the loaded provider.
//
// Note that this function must not be called from two goroutines concurrently; it is the responsibility of d.serve()
// to ensure this.
func (d *defaultProviders) handleRequest(req providers.ProviderRequest) (providers.Reference, error) {
	logging.V(5).Infof("handling default provider request for package %s", req)

	// Have we loaded this provider before? Use the existing reference, if so.
	//
	// Note that we are using the request's String as the key for the provider map. Go auto-derives hash and equality
	// functions for aggregates, but the one auto-derived for ProviderRequest does not have the semantics we want. The
	// use of a string key here is hacky but gets us the desired semantics - that ProviderRequest is a tuple of
	// optional value-typed Version and a package.
	ref, ok := d.providers[req.String()]
	if ok {
		return ref, nil
	}

	event, done, err := d.newRegisterDefaultProviderEvent(req)
	if err != nil {
		return providers.Reference{}, err
	}

	select {
	case d.providerRegChan <- event:
	case <-d.cancel:
		return providers.Reference{}, context.Canceled
	}

	logging.V(5).Infof("waiting for default provider for package %s", req)

	var result *RegisterResult
	select {
	case result = <-done:
	case <-d.cancel:
		return providers.Reference{}, context.Canceled
	}

	logging.V(5).Infof("registered default provider for package %s: %s", req, result.State.URN)

	id := result.State.ID
	if id == "" {
		id = providers.UnknownID
	}

	ref, err = providers.NewReference(result.State.URN, id)
	contract.Assert(err == nil)
	d.providers[req.String()] = ref

	return ref, nil
}

// serve is the primary loop responsible for handling default provider requests.
func (d *defaultProviders) serve() {
	for {
		select {
		case req := <-d.requests:
			// Note that we do not need to handle cancellation when sending the response: every message we receive is
			// guaranteed to have something waiting on the other end of the response channel.
			ref, err := d.handleRequest(req.req)
			req.response <- defaultProviderResponse{ref: ref, err: err}
		case <-d.cancel:
			return
		}
	}
}

// getDefaultProviderRef fetches the provider reference for the default provider for a particular package.
func (d *defaultProviders) getDefaultProviderRef(req providers.ProviderRequest) (providers.Reference, error) {
	response := make(chan defaultProviderResponse)
	select {
	case d.requests <- defaultProviderRequest{req: req, response: response}:
	case <-d.cancel:
		return providers.Reference{}, context.Canceled
	}
	res := <-response
	return res.ref, res.err
}

// resmon implements the pulumirpc.ResourceMonitor interface and acts as the gateway between a language runtime's
// evaluation of a program and the internal resource planning and deployment logic.
type resmon struct {
	providers        ProviderSource                     // the provider source itself.
	defaultProviders *defaultProviders                  // the default provider manager.
	constructInfo    plugin.ConstructInfo               // information for construct calls.
	regChan          chan *registerResourceEvent        // the channel to send resource registrations to.
	regOutChan       chan *registerResourceOutputsEvent // the channel to send resource output registrations to.
	regReadChan      chan *readResourceEvent            // the channel to send resource reads to.
	cancel           chan bool                          // a channel that can cancel the server.
	done             chan error                         // a channel that resolves when the server completes.
}

var _ SourceResourceMonitor = (*resmon)(nil)

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(src *evalSource, provs ProviderSource, regChan chan *registerResourceEvent,
	regOutChan chan *registerResourceOutputsEvent, regReadChan chan *readResourceEvent, opts Options,
	config map[config.Key]string, tracingSpan opentracing.Span) (*resmon, error) {

	// Create our cancellation channel.
	cancel := make(chan bool)

	// Create a new default provider manager.
	d := &defaultProviders{
		defaultVersions: src.defaultProviderVersions,
		providers:       make(map[string]providers.Reference),
		config:          src.runinfo.Target,
		requests:        make(chan defaultProviderRequest),
		providerRegChan: regChan,
		cancel:          cancel,
	}

	// New up an engine RPC server.
	resmon := &resmon{
		providers:        provs,
		defaultProviders: d,
		regChan:          regChan,
		regOutChan:       regOutChan,
		regReadChan:      regReadChan,
		cancel:           cancel,
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, resmon.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, resmon)
			return nil
		},
	}, tracingSpan)
	if err != nil {
		return nil, err
	}

	resmon.constructInfo = plugin.ConstructInfo{
		Project:        string(src.runinfo.Proj.Name),
		Stack:          string(src.runinfo.Target.Name),
		Config:         config,
		DryRun:         src.dryRun,
		Parallel:       opts.Parallel,
		MonitorAddress: fmt.Sprintf("127.0.0.1:%d", port),
	}
	resmon.done = done

	go d.serve()

	return resmon, nil
}

// Address returns the address at which the monitor's RPC server may be reached.
func (rm *resmon) Address() string {
	return rm.constructInfo.MonitorAddress
}

// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
func (rm *resmon) Cancel() error {
	close(rm.cancel)
	return <-rm.done
}

// getProviderReference fetches the provider reference for a resource, read, or invoke from the given package with the
// given unparsed provider reference. If the unparsed provider reference is empty, this function returns a reference
// to the default provider for the indicated package.
func getProviderReference(defaultProviders *defaultProviders, req providers.ProviderRequest,
	rawProviderRef string) (providers.Reference, error) {
	if rawProviderRef != "" {
		ref, err := providers.ParseReference(rawProviderRef)
		if err != nil {
			return providers.Reference{}, errors.Errorf("could not parse provider reference: %v", err)
		}
		return ref, nil
	}

	ref, err := defaultProviders.getDefaultProviderRef(req)
	if err != nil {
		return providers.Reference{}, err
	}
	return ref, nil
}

// getProviderFromSource fetches the provider plugin for a resource, read, or invoke from the given
// package with the given unparsed provider reference. If the unparsed provider reference is empty,
// this function returns the plugin for the indicated package's default provider.
func getProviderFromSource(
	providers ProviderSource, defaultProviders *defaultProviders,
	req providers.ProviderRequest, rawProviderRef string) (plugin.Provider, error) {

	providerRef, err := getProviderReference(defaultProviders, req, rawProviderRef)
	if err != nil {
		return nil, err
	}
	provider, ok := providers.GetProvider(providerRef)
	if !ok {
		return nil, errors.Errorf("unknown provider '%v'", rawProviderRef)
	}
	return provider, nil
}

func parseProviderRequest(pkg tokens.Package, version string) (providers.ProviderRequest, error) {
	if version == "" {
		logging.V(5).Infof("parseProviderRequest(%s): semver version is the empty string", pkg)
		return providers.NewProviderRequest(nil, pkg), nil
	}

	parsedVersion, err := semver.Parse(version)
	if err != nil {
		logging.V(5).Infof("parseProviderRequest(%s, %s): semver version string is invalid: %v", pkg, version, err)
		return providers.ProviderRequest{}, err
	}

	return providers.NewProviderRequest(&parsedVersion, pkg), nil
}

func (rm *resmon) SupportsFeature(ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest) (*pulumirpc.SupportsFeatureResponse, error) {

	hasSupport := false

	switch req.Id {
	case "secrets":
		hasSupport = true
	case "resourceReferences":
		// TODO: Temporarily disabling resource ref support (https://github.com/pulumi/pulumi-kubernetes/issues/1405)
		hasSupport = false

		// Allow the resource reference feature to be disabled by explicitly setting an env var.
		if v, ok := os.LookupEnv("PULUMI_DISABLE_RESOURCE_REFERENCES"); ok && cmdutil.IsTruthy(v) {
			hasSupport = false
		}
	}

	logging.V(5).Infof("ResourceMonitor.SupportsFeature(id: %s) = %t", req.Id, hasSupport)

	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: hasSupport,
	}, nil
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *resmon) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	// Fetch the token and load up the resource provider if necessary.
	tok := tokens.ModuleMember(req.GetTok())
	providerReq, err := parseProviderRequest(tok.Package(), req.GetVersion())
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, req.GetProvider())
	if err != nil {
		return nil, err
	}

	label := fmt.Sprintf("ResourceMonitor.Invoke(%s)", tok)

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invocation of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepResources: true,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal %v return", tok)
	}
	var chkfails []*pulumirpc.CheckFailure
	for _, failure := range failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}
	return &pulumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

func (rm *resmon) StreamInvoke(
	req *pulumirpc.InvokeRequest, stream pulumirpc.ResourceMonitor_StreamInvokeServer) error {

	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("ResourceMonitor.StreamInvoke(%s)", tok)

	providerReq, err := parseProviderRequest(tok.Package(), req.GetVersion())
	if err != nil {
		return err
	}
	prov, err := getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, req.GetProvider())
	if err != nil {
		return err
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Synchronously do the StreamInvoke and then return the arguments. This will block until the
	// streaming operation completes!
	logging.V(5).Infof("ResourceMonitor.StreamInvoke received: tok=%v #args=%v", tok, len(args))
	failures, err := prov.StreamInvoke(tok, args, func(event resource.PropertyMap) error {
		mret, err := plugin.MarshalProperties(event, plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepResources: req.GetAcceptResources(),
		})
		if err != nil {
			return errors.Wrapf(err, "failed to marshal return")
		}

		return stream.Send(&pulumirpc.InvokeResponse{Return: mret})
	})
	if err != nil {
		return errors.Wrapf(err, "streaming invocation of %v returned an error", tok)
	}

	var chkfails []*pulumirpc.CheckFailure
	for _, failure := range failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}

	if len(chkfails) > 0 {
		return stream.Send(&pulumirpc.InvokeResponse{Failures: chkfails})
	}
	return nil
}

// ReadResource reads the current state associated with a resource from its provider plugin.
func (rm *resmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error) {
	// Read the basic inputs necessary to identify the plugin.
	t, err := tokens.ParseTypeToken(req.GetType())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, err.Error())
	}

	name := tokens.QName(req.GetName())
	parent := resource.URN(req.GetParent())

	provider := req.GetProvider()
	if !providers.IsProviderType(t) && provider == "" {
		providerReq, err := parseProviderRequest(t.Package(), req.GetVersion())
		if err != nil {
			return nil, err
		}
		ref, provErr := rm.defaultProviders.getDefaultProviderRef(providerReq)
		if provErr != nil {
			return nil, provErr
		}
		provider = ref.String()
	}

	id := resource.ID(req.GetId())
	label := fmt.Sprintf("ResourceMonitor.ReadResource(%s, %s, %s, %s)", id, t, name, provider)
	var deps []resource.URN
	for _, depURN := range req.GetDependencies() {
		deps = append(deps, resource.URN(depURN))
	}

	props, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	var additionalSecretOutputs []resource.PropertyKey
	for _, name := range req.GetAdditionalSecretOutputs() {
		additionalSecretOutputs = append(additionalSecretOutputs, resource.PropertyKey(name))
	}

	event := &readResourceEvent{
		id:                      id,
		name:                    name,
		baseType:                t,
		provider:                provider,
		parent:                  parent,
		props:                   props,
		dependencies:            deps,
		additionalSecretOutputs: additionalSecretOutputs,
		done:                    make(chan *ReadResult),
	}
	select {
	case rm.regReadChan <- event:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.ReadResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource registration")
	}

	// Now block waiting for the operation to finish.
	var result *ReadResult
	select {
	case result = <-event.done:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.ReadResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on step's done channel")
	}

	contract.Assert(result != nil)
	marshaled, err := plugin.MarshalProperties(result.State.Outputs, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   req.GetAcceptSecrets(),
		KeepResources: req.GetAcceptResources(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal %s return state", result.State.URN)
	}

	return &pulumirpc.ReadResourceResponse{
		Urn:        string(result.State.URN),
		Properties: marshaled,
	}, nil
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {

	// Communicate the type, name, and object information to the iterator that is awaiting us.
	name := tokens.QName(req.GetName())
	custom := req.GetCustom()
	remote := req.GetRemote()
	parent := resource.URN(req.GetParent())
	protect := req.GetProtect()
	deleteBeforeReplaceValue := req.GetDeleteBeforeReplace()
	ignoreChanges := req.GetIgnoreChanges()
	id := resource.ID(req.GetImportId())
	customTimeouts := req.GetCustomTimeouts()

	// Custom resources must have a three-part type so that we can 1) identify if they are providers and 2) retrieve the
	// provider responsible for managing a particular resource (based on the type's Package).
	var err error
	var t tokens.Type
	if custom || remote {
		t, err = tokens.ParseTypeToken(req.GetType())
		if err != nil {
			return nil, rpcerror.New(codes.InvalidArgument, err.Error())
		}
	} else {
		// Component resources may have any format type.
		t = tokens.Type(req.GetType())
	}

	label := fmt.Sprintf("ResourceMonitor.RegisterResource(%s,%s)", t, name)

	var providerRef providers.Reference
	if custom && !providers.IsProviderType(t) || remote {
		provider := req.GetProvider()
		if provider == "" {
			providerReq, err := parseProviderRequest(t.Package(), req.GetVersion())
			if err != nil {
				return nil, err
			}
			providerRef, err = rm.defaultProviders.getDefaultProviderRef(providerReq)
			if err != nil {
				return nil, err
			}
		} else {
			providerRef, err = providers.ParseReference(provider)
			if err != nil {
				return nil, errors.Errorf("could not parse provider reference '%v': %v", provider, err)
			}
		}
	}

	aliases := []resource.URN{}
	for _, aliasURN := range req.GetAliases() {
		aliases = append(aliases, resource.URN(aliasURN))
	}

	dependencies := []resource.URN{}
	for _, dependingURN := range req.GetDependencies() {
		dependencies = append(dependencies, resource.URN(dependingURN))
	}

	props, err := plugin.UnmarshalProperties(
		req.GetObject(), plugin.MarshalOptions{
			Label:              label,
			KeepUnknowns:       true,
			ComputeAssetHashes: true,
			KeepSecrets:        true,
			KeepResources:      true,
		})
	if err != nil {
		return nil, err
	}

	propertyDependencies := make(map[resource.PropertyKey][]resource.URN)
	if len(req.GetPropertyDependencies()) == 0 {
		// If this request did not specify property dependencies, treat each property as depending on every resource
		// in the request's dependency list.
		for pk := range props {
			propertyDependencies[pk] = dependencies
		}
	} else {
		// Otherwise, unmarshal the per-property dependency information.
		for pk, pd := range req.GetPropertyDependencies() {
			var deps []resource.URN
			for _, d := range pd.Urns {
				deps = append(deps, resource.URN(d))
			}
			propertyDependencies[resource.PropertyKey(pk)] = deps
		}
	}

	var additionalSecretOutputs []resource.PropertyKey
	for _, name := range req.GetAdditionalSecretOutputs() {
		additionalSecretOutputs = append(additionalSecretOutputs, resource.PropertyKey(name))
	}

	var timeouts resource.CustomTimeouts
	if customTimeouts != nil {
		if customTimeouts.Create != "" {
			seconds, err := generateTimeoutInSeconds(customTimeouts.Create)
			if err != nil {
				return nil, err
			}
			timeouts.Create = seconds
		}
		if customTimeouts.Delete != "" {
			seconds, err := generateTimeoutInSeconds(customTimeouts.Delete)
			if err != nil {
				return nil, err
			}
			timeouts.Delete = seconds
		}
		if customTimeouts.Update != "" {
			seconds, err := generateTimeoutInSeconds(customTimeouts.Update)
			if err != nil {
				return nil, err
			}
			timeouts.Update = seconds
		}
	}

	var deleteBeforeReplace *bool
	if deleteBeforeReplaceValue || req.GetDeleteBeforeReplaceDefined() {
		deleteBeforeReplace = &deleteBeforeReplaceValue
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource received: t=%v, name=%v, custom=%v, #props=%v, parent=%v, protect=%v, "+
			"provider=%v, deps=%v, deleteBeforeReplace=%v, ignoreChanges=%v, aliases=%v, customTimeouts=%v",
		t, name, custom, len(props), parent, protect, providerRef, dependencies, deleteBeforeReplace, ignoreChanges,
		aliases, timeouts)

	// If this is a remote component, fetch its provider and issue the construct call. Otherwise, register the resource.
	var result *RegisterResult
	var outputDeps map[string]*pulumirpc.RegisterResourceResponse_PropertyDependencies
	if remote {
		provider, ok := rm.providers.GetProvider(providerRef)
		if !ok {
			return nil, errors.Errorf("unknown provider '%v'", providerRef)
		}

		// Invoke the provider's Construct RPC method.
		options := plugin.ConstructOptions{
			Aliases:              aliases,
			Protect:              protect,
			PropertyDependencies: propertyDependencies,
		}
		constructResult, err := provider.Construct(rm.constructInfo, t, name, parent, props, options)
		if err != nil {
			return nil, err
		}
		result = &RegisterResult{State: &resource.State{URN: constructResult.URN, Outputs: constructResult.Outputs}}

		outputDeps = map[string]*pulumirpc.RegisterResourceResponse_PropertyDependencies{}
		for k, deps := range constructResult.OutputDependencies {
			urns := make([]string, len(deps))
			for i, d := range deps {
				urns[i] = string(d)
			}
			outputDeps[string(k)] = &pulumirpc.RegisterResourceResponse_PropertyDependencies{Urns: urns}
		}
	} else {
		// Send the goal state to the engine.
		step := &registerResourceEvent{
			goal: resource.NewGoal(t, name, custom, props, parent, protect, dependencies,
				providerRef.String(), nil, propertyDependencies, deleteBeforeReplace, ignoreChanges,
				additionalSecretOutputs, aliases, id, &timeouts),
			done: make(chan *RegisterResult),
		}

		select {
		case rm.regChan <- step:
		case <-rm.cancel:
			logging.V(5).Infof("ResourceMonitor.RegisterResource operation canceled, name=%s", name)
			return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource registration")
		}

		// Now block waiting for the operation to finish.
		select {
		case result = <-step.done:
		case <-rm.cancel:
			logging.V(5).Infof("ResourceMonitor.RegisterResource operation canceled, name=%s", name)
			return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on step's done channel")
		}
	}

	// Filter out partially-known values if the requestor does not support them.
	outputs := result.State.Outputs
	if !req.GetSupportsPartialValues() {
		logging.V(5).Infof("stripping unknowns from RegisterResource response for urn %v", result.State.URN)
		filtered := resource.PropertyMap{}
		for k, v := range outputs {
			if !v.ContainsUnknowns() {
				filtered[k] = v
			}
		}
		outputs = filtered
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource operation finished: t=%v, urn=%v, #outs=%v",
		result.State.Type, result.State.URN, len(outputs))

	// Finally, unpack the response into properties that we can return to the language runtime.  This mostly includes
	// an ID, URN, and defaults and output properties that will all be blitted back onto the runtime object.
	obj, err := plugin.MarshalProperties(outputs, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   req.GetAcceptSecrets(),
		KeepResources: req.GetAcceptResources(),
	})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.RegisterResourceResponse{
		Urn:                  string(result.State.URN),
		Id:                   string(result.State.ID),
		Object:               obj,
		PropertyDependencies: outputDeps,
	}, nil
}

// RegisterResourceOutputs records some new output properties for a resource that have arrived after its initial
// provisioning.  These will make their way into the eventual checkpoint state file for that resource.
func (rm *resmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {

	// Obtain and validate the message's inputs (a URN plus the output property map).
	urn := resource.URN(req.GetUrn())
	if urn == "" {
		return nil, errors.New("missing required URN")
	}
	label := fmt.Sprintf("ResourceMonitor.RegisterResourceOutputs(%s)", urn)
	outs, err := plugin.UnmarshalProperties(
		req.GetOutputs(), plugin.MarshalOptions{
			Label:              label,
			KeepUnknowns:       true,
			ComputeAssetHashes: true,
			KeepSecrets:        true,
			KeepResources:      true,
		})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal output properties")
	}
	logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs received: urn=%v, #outs=%v", urn, len(outs))

	// Now send the step over to the engine to perform.
	step := &registerResourceOutputsEvent{
		urn:     urn,
		outputs: outs,
		done:    make(chan bool),
	}

	select {
	case rm.regOutChan <- step:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs operation canceled, urn=%s", urn)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource outputs")
	}

	// Now block waiting for the operation to finish.
	select {
	case <-step.done:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs operation canceled, urn=%s", urn)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on output step's done channel")
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResourceOutputs operation finished: urn=%v, #outs=%v", urn, len(outs))
	return &pbempty.Empty{}, nil
}

type registerResourceEvent struct {
	goal *resource.Goal       // the resource goal state produced by the iterator.
	done chan *RegisterResult // the channel to communicate with after the resource state is available.
}

var _ RegisterResourceEvent = (*registerResourceEvent)(nil)

func (g *registerResourceEvent) event() {}

func (g *registerResourceEvent) Goal() *resource.Goal {
	return g.goal
}

func (g *registerResourceEvent) Done(result *RegisterResult) {
	// Communicate the resulting state back to the RPC thread, which is parked awaiting our reply.
	g.done <- result
}

type registerResourceOutputsEvent struct {
	urn     resource.URN         // the URN to which this completion applies.
	outputs resource.PropertyMap // an optional property bag for output properties.
	done    chan bool            // the channel to communicate with after the operation completes.
}

var _ RegisterResourceOutputsEvent = (*registerResourceOutputsEvent)(nil)

func (g *registerResourceOutputsEvent) event() {}

func (g *registerResourceOutputsEvent) URN() resource.URN {
	return g.urn
}

func (g *registerResourceOutputsEvent) Outputs() resource.PropertyMap {
	return g.outputs
}

func (g *registerResourceOutputsEvent) Done() {
	// Communicate the resulting state back to the RPC thread, which is parked awaiting our reply.
	g.done <- true
}

type readResourceEvent struct {
	id                      resource.ID
	name                    tokens.QName
	baseType                tokens.Type
	provider                string
	parent                  resource.URN
	props                   resource.PropertyMap
	dependencies            []resource.URN
	additionalSecretOutputs []resource.PropertyKey
	done                    chan *ReadResult
}

var _ ReadResourceEvent = (*readResourceEvent)(nil)

func (g *readResourceEvent) event() {}

func (g *readResourceEvent) ID() resource.ID                  { return g.id }
func (g *readResourceEvent) Name() tokens.QName               { return g.name }
func (g *readResourceEvent) Type() tokens.Type                { return g.baseType }
func (g *readResourceEvent) Provider() string                 { return g.provider }
func (g *readResourceEvent) Parent() resource.URN             { return g.parent }
func (g *readResourceEvent) Properties() resource.PropertyMap { return g.props }
func (g *readResourceEvent) Dependencies() []resource.URN     { return g.dependencies }
func (g *readResourceEvent) AdditionalSecretOutputs() []resource.PropertyKey {
	return g.additionalSecretOutputs
}
func (g *readResourceEvent) Done(result *ReadResult) {
	g.done <- result
}

func generateTimeoutInSeconds(timeout string) (float64, error) {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return 0, errors.Errorf("unable to parse customTimeout Value %s", timeout)
	}

	return duration.Seconds(), nil
}
