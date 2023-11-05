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
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	interceptors "github.com/pulumi/pulumi/pkg/v3/util/rpcdebug"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// EvalRunInfo provides information required to execute and deploy resources within a package.
type EvalRunInfo struct {
	// the package metadata.
	Proj *workspace.Project `json:"proj" yaml:"proj"`
	// the package's working directory.
	Pwd string `json:"pwd" yaml:"pwd"`
	// the path to the program.
	Program string `json:"program" yaml:"program"`
	// the path to the project's directory.
	ProjectRoot string `json:"projectRoot,omitempty" yaml:"projectRoot,omitempty"`
	// any arguments to pass to the package.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
	// the target being deployed into.
	Target *Target `json:"target,omitempty" yaml:"target,omitempty"`
}

// NewEvalSource returns a planning source that fetches resources by evaluating a package with a set of args and
// a confgiuration map.  This evaluation is performed using the given plugin context and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(plugctx *plugin.Context, runinfo *EvalRunInfo,
	defaultProviderInfo map[tokens.Package]workspace.PluginSpec, dryRun bool,
) Source {
	return &evalSource{
		plugctx:             plugctx,
		runinfo:             runinfo,
		defaultProviderInfo: defaultProviderInfo,
		dryRun:              dryRun,
	}
}

type evalSource struct {
	plugctx             *plugin.Context                         // the plugin context.
	runinfo             *EvalRunInfo                            // the directives to use when running the program.
	defaultProviderInfo map[tokens.Package]workspace.PluginSpec // the default provider versions for this source.
	dryRun              bool                                    // true if this is a dry-run operation only.
}

func (src *evalSource) Close() error {
	return nil
}

// Project is the name of the project being run by this evaluation source.
func (src *evalSource) Project() tokens.PackageName {
	return src.runinfo.Proj.Name
}

// Stack is the name of the stack being targeted by this evaluation source.
func (src *evalSource) Stack() tokens.Name {
	return src.runinfo.Target.Name
}

func (src *evalSource) Info() interface{} { return src.runinfo }

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(
	ctx context.Context, opts Options, providers ProviderSource,
) (SourceIterator, error) {
	tracingSpan := opentracing.SpanFromContext(ctx)

	// Decrypt the configuration.
	config, err := src.runinfo.Target.Config.Decrypt(src.runinfo.Target.Decrypter)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Keep track of any config keys that have secure values.
	configSecretKeys := src.runinfo.Target.Config.SecureKeys()

	configMap, err := src.runinfo.Target.Config.AsDecryptedPropertyMap(ctx, src.runinfo.Target.Decrypter)
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to map: %w", err)
	}

	// First, fire up a resource monitor that will watch for and record resource creation.
	regChan := make(chan *registerResourceEvent)
	regOutChan := make(chan *registerResourceOutputsEvent)
	regReadChan := make(chan *readResourceEvent)
	mon, err := newResourceMonitor(
		src, providers, regChan, regOutChan, regReadChan, opts, config, configSecretKeys, tracingSpan)
	if err != nil {
		return nil, fmt.Errorf("failed to start resource monitor: %w", err)
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		mon:         mon,
		src:         src,
		regChan:     regChan,
		regOutChan:  regOutChan,
		regReadChan: regReadChan,
		finChan:     make(chan error),
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(opts, config, configSecretKeys, configMap)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	mon         SourceResourceMonitor              // the resource monitor, per iterator.
	src         *evalSource                        // the owning eval source object.
	regChan     chan *registerResourceEvent        // the channel that contains resource registrations.
	regOutChan  chan *registerResourceOutputsEvent // the channel that contains resource completions.
	regReadChan chan *readResourceEvent            // the channel that contains read resource requests.
	finChan     chan error                         // the channel that communicates completion.
	done        bool                               // set to true when the evaluation is done.
}

func (iter *evalSourceIterator) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	return iter.mon.Cancel()
}

func (iter *evalSourceIterator) ResourceMonitor() SourceResourceMonitor {
	return iter.mon
}

func (iter *evalSourceIterator) Next() (SourceEvent, error) {
	// If we are done, quit.
	if iter.done {
		return nil, nil
	}

	// Await the program to compute some more state and then inspect what it has to say.
	select {
	case reg := <-iter.regChan:
		contract.Assertf(reg != nil, "received a nil registerResourceEvent")
		goal := reg.Goal()
		logging.V(5).Infof("EvalSourceIterator produced a registration: t=%v,name=%v,#props=%v",
			goal.Type, goal.Name, len(goal.Properties))
		return reg, nil
	case regOut := <-iter.regOutChan:
		contract.Assertf(regOut != nil, "received a nil registerResourceOutputsEvent")
		logging.V(5).Infof("EvalSourceIterator produced a completion: urn=%v,#outs=%v",
			regOut.URN(), len(regOut.Outputs()))
		return regOut, nil
	case read := <-iter.regReadChan:
		contract.Assertf(read != nil, "received a nil readResourceEvent")
		logging.V(5).Infoln("EvalSourceIterator produced a read")
		return read, nil
	case err := <-iter.finChan:
		// If we are finished, we can safely exit.  The contract with the language provider is that this implies
		// that the language runtime has exited and so calling Close on the plugin is fine.
		iter.done = true
		if err != nil {
			if result.IsBail(err) {
				logging.V(5).Infof("EvalSourceIterator ended with bail.")
			} else {
				logging.V(5).Infof("EvalSourceIterator ended with an error: %v", err)
			}
		}
		return nil, err
	}
}

// forkRun performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func (iter *evalSourceIterator) forkRun(
	opts Options, config map[config.Key]string, configSecretKeys []config.Key, configPropertyMap resource.PropertyMap,
) {
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go func() {
		// Next, launch the language plugin.
		run := func() error {
			rt := iter.src.runinfo.Proj.Runtime.Name()
			rtopts := iter.src.runinfo.Proj.Runtime.Options()
			langhost, err := iter.src.plugctx.Host.LanguageRuntime(iter.src.plugctx.Root, iter.src.plugctx.Pwd, rt, rtopts)
			if err != nil {
				return fmt.Errorf("failed to launch language host %s: %w", rt, err)
			}
			contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

			// Now run the actual program.
			progerr, bail, err := langhost.Run(plugin.RunInfo{
				MonitorAddress:    iter.mon.Address(),
				Stack:             string(iter.src.runinfo.Target.Name),
				Project:           string(iter.src.runinfo.Proj.Name),
				Pwd:               iter.src.runinfo.Pwd,
				Program:           iter.src.runinfo.Program,
				Args:              iter.src.runinfo.Args,
				Config:            config,
				ConfigSecretKeys:  configSecretKeys,
				ConfigPropertyMap: configPropertyMap,
				DryRun:            iter.src.dryRun,
				Parallel:          opts.Parallel,
				Organization:      string(iter.src.runinfo.Target.Organization),
			})

			// Check if we were asked to Bail.  This a special random constant used for that
			// purpose.
			if err == nil && bail {
				return result.BailErrorf("run bailed")
			}

			if err == nil && progerr != "" {
				// If the program had an unhandled error; propagate it to the caller.
				err = fmt.Errorf("an unhandled error occurred: %v", progerr)
			}
			return err
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
	defaultProviderInfo map[tokens.Package]workspace.PluginSpec

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
	req providers.ProviderRequest,
) (*registerResourceEvent, <-chan *RegisterResult, error) {
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
		providers.SetProviderVersion(inputs, req.Version())
	} else {
		logging.V(5).Infof(
			"newRegisterDefaultProviderEvent(%s): no version specified, falling back to default version", req)
		if version := d.defaultProviderInfo[req.Package()].Version; version != nil {
			logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): default version hit on version %s", req, version)
			providers.SetProviderVersion(inputs, version)
		} else {
			logging.V(5).Infof(
				"newRegisterDefaultProviderEvent(%s): default provider miss, sending nil version to engine", req)
		}
	}

	if req.PluginDownloadURL() != "" {
		logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): using pluginDownloadURL %s from request",
			req, req.PluginDownloadURL())
		providers.SetProviderURL(inputs, req.PluginDownloadURL())
	} else {
		logging.V(5).Infof(
			"newRegisterDefaultProviderEvent(%s): no pluginDownloadURL specified, falling back to default pluginDownloadURL",
			req)
		if pluginDownloadURL := d.defaultProviderInfo[req.Package()].PluginDownloadURL; pluginDownloadURL != "" {
			logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): default pluginDownloadURL hit on %s",
				req, pluginDownloadURL)
			providers.SetProviderURL(inputs, pluginDownloadURL)
		} else {
			logging.V(5).Infof(
				"newRegisterDefaultProviderEvent(%s): default pluginDownloadURL miss, sending empty string to engine", req)
		}
	}

	if req.PluginChecksums() != nil {
		logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): using pluginChecksums %v from request",
			req, req.PluginChecksums())
		providers.SetProviderChecksums(inputs, req.PluginChecksums())
	} else {
		logging.V(5).Infof(
			"newRegisterDefaultProviderEvent(%s): no pluginChecksums specified, falling back to default pluginChecksums",
			req)
		if pluginChecksums := d.defaultProviderInfo[req.Package()].Checksums; pluginChecksums != nil {
			logging.V(5).Infof("newRegisterDefaultProviderEvent(%s): default pluginChecksums hit on %v",
				req, pluginChecksums)
			providers.SetProviderChecksums(inputs, pluginChecksums)
		} else {
			logging.V(5).Infof(
				"newRegisterDefaultProviderEvent(%s): default pluginChecksums miss, sending empty map to engine", req)
		}
	}

	// Create the result channel and the event.
	done := make(chan *RegisterResult)
	event := &registerResourceEvent{
		goal: resource.NewGoal(
			providers.MakeProviderType(req.Package()),
			req.Name(), true, inputs, "", false, nil, "", nil, nil, nil,
			nil, nil, nil, "", nil, nil, false, "", ""),
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

	denyCreation, err := d.shouldDenyRequest(req)
	if err != nil {
		return providers.Reference{}, err
	}
	if denyCreation {
		logging.V(5).Infof("denied default provider request for package %s", req)
		return providers.NewDenyDefaultProvider(tokens.QName(string(req.Package().Name()))), nil
	}

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
	contract.Assertf(id != "", "default provider for package %s has no ID", req)

	ref, err = providers.NewReference(result.State.URN, id)
	contract.Assertf(err == nil, "could not create provider reference with URN %s and ID %s", result.State.URN, id)
	d.providers[req.String()] = ref

	return ref, nil
}

// If req should be allowed, or if we should prevent the request.
func (d *defaultProviders) shouldDenyRequest(req providers.ProviderRequest) (bool, error) {
	logging.V(9).Infof("checking if %#v should be denied", req)

	if req.Package().Name().String() == "pulumi" {
		logging.V(9).Infof("we always allow %#v through", req)
		return false, nil
	}

	pConfig, err := d.config.GetPackageConfig("pulumi")
	if err != nil {
		return true, err
	}

	denyCreation := false
	if value, ok := pConfig["disable-default-providers"]; ok {
		array := []interface{}{}
		if !value.IsString() {
			return true, fmt.Errorf("Unexpected encoding of pulumi:disable-default-providers")
		}
		if value.StringValue() == "" {
			// If the list is provided but empty, we don't encode a empty json
			// list, we just encode the empty string. Check to ensure we don't
			// get parse errors.
			return false, nil
		}
		if err := json.Unmarshal([]byte(value.StringValue()), &array); err != nil {
			return true, fmt.Errorf("Failed to parse %s: %w", value.StringValue(), err)
		}
		for i, v := range array {
			s, ok := v.(string)
			if !ok {
				return true, fmt.Errorf("pulumi:disable-default-providers[%d] must be a string", i)
			}
			barred := strings.TrimSpace(s)
			if barred == "*" || barred == req.Package().Name().String() {
				logging.V(7).Infof("denying %s (star=%t)", req, barred == "*")
				denyCreation = true
				break
			}
		}
	} else {
		logging.V(9).Infof("Did not find a config for 'pulumi'")
	}

	return denyCreation, nil
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
	pulumirpc.UnimplementedResourceMonitorServer

	resGoals                  map[resource.URN]resource.Goal     // map of seen URNs and their goals.
	resGoalsLock              sync.Mutex                         // locks the resGoals map.
	diagostics                diag.Sink                          // logger for user-facing messages
	providers                 ProviderSource                     // the provider source itself.
	componentProviders        map[resource.URN]map[string]string // which providers component resources used
	componentProvidersLock    sync.Mutex                         // which locks the componentProviders map
	defaultProviders          *defaultProviders                  // the default provider manager.
	sourcePositions           *sourcePositions                   // source position manager.
	constructInfo             plugin.ConstructInfo               // information for construct and call calls.
	regChan                   chan *registerResourceEvent        // the channel to send resource registrations to.
	regOutChan                chan *registerResourceOutputsEvent // the channel to send resource output registrations to.
	regReadChan               chan *readResourceEvent            // the channel to send resource reads to.
	cancel                    chan bool                          // a channel that can cancel the server.
	done                      <-chan error                       // a channel that resolves when the server completes.
	disableResourceReferences bool                               // true if resource references are disabled.
	disableOutputValues       bool                               // true if output values are disabled.
}

var _ SourceResourceMonitor = (*resmon)(nil)

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(src *evalSource, provs ProviderSource, regChan chan *registerResourceEvent,
	regOutChan chan *registerResourceOutputsEvent, regReadChan chan *readResourceEvent, opts Options,
	config map[config.Key]string, configSecretKeys []config.Key, tracingSpan opentracing.Span,
) (*resmon, error) {
	// Create our cancellation channel.
	cancel := make(chan bool)

	// Create a new default provider manager.
	d := &defaultProviders{
		defaultProviderInfo: src.defaultProviderInfo,
		providers:           make(map[string]providers.Reference),
		config:              src.runinfo.Target,
		requests:            make(chan defaultProviderRequest),
		providerRegChan:     regChan,
		cancel:              cancel,
	}

	// New up an engine RPC server.
	resmon := &resmon{
		diagostics:                src.plugctx.Diag,
		providers:                 provs,
		defaultProviders:          d,
		sourcePositions:           newSourcePositions(src.runinfo.ProjectRoot),
		resGoals:                  map[resource.URN]resource.Goal{},
		componentProviders:        map[resource.URN]map[string]string{},
		regChan:                   regChan,
		regOutChan:                regOutChan,
		regReadChan:               regReadChan,
		cancel:                    cancel,
		disableResourceReferences: opts.DisableResourceReferences,
		disableOutputValues:       opts.DisableOutputValues,
	}

	// Fire up a gRPC server and start listening for incomings.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: resmon.cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, resmon)
			return nil
		},
		Options: sourceEvalServeOptions(src.plugctx, tracingSpan),
	})
	if err != nil {
		return nil, err
	}

	resmon.constructInfo = plugin.ConstructInfo{
		Project:          string(src.runinfo.Proj.Name),
		Stack:            string(src.runinfo.Target.Name),
		Config:           config,
		ConfigSecretKeys: configSecretKeys,
		DryRun:           src.dryRun,
		Parallel:         opts.Parallel,
		MonitorAddress:   fmt.Sprintf("127.0.0.1:%d", handle.Port),
	}
	resmon.done = handle.Done

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

func sourceEvalServeOptions(ctx *plugin.Context, tracingSpan opentracing.Span) []grpc.ServerOption {
	serveOpts := rpcutil.OpenTracingServerInterceptorOptions(
		tracingSpan,
		otgrpc.SpanDecorator(decorateResourceSpans),
	)
	if logFile := env.DebugGRPC.Value(); logFile != "" {
		di, err := interceptors.NewDebugInterceptor(interceptors.DebugInterceptorOptions{
			LogFile: logFile,
			Mutex:   ctx.DebugTraceMutex,
		})
		if err != nil {
			// ignoring
			return nil
		}
		metadata := map[string]interface{}{
			"mode": "server",
		}
		serveOpts = append(serveOpts, di.ServerOptions(interceptors.LogOptions{
			Metadata: metadata,
		})...)
	}
	return serveOpts
}

// getProviderReference fetches the provider reference for a resource, read, or invoke from the given package with the
// given unparsed provider reference. If the unparsed provider reference is empty, this function returns a reference
// to the default provider for the indicated package.
func getProviderReference(defaultProviders *defaultProviders, req providers.ProviderRequest,
	rawProviderRef string,
) (providers.Reference, error) {
	if rawProviderRef != "" {
		ref, err := providers.ParseReference(rawProviderRef)
		if err != nil {
			return providers.Reference{}, fmt.Errorf("could not parse provider reference: %v", err)
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
	providerSource ProviderSource, defaultProviders *defaultProviders,
	req providers.ProviderRequest, rawProviderRef string,
	token tokens.ModuleMember,
) (plugin.Provider, error) {
	providerRef, err := getProviderReference(defaultProviders, req, rawProviderRef)
	if err != nil {
		return nil, fmt.Errorf("getProviderFromSource: %w", err)
	} else if providers.IsDenyDefaultsProvider(providerRef) {
		msg := diag.GetDefaultProviderDenied("Invoke").Message
		return nil, fmt.Errorf(msg, req.Package(), token)
	}

	provider, ok := providerSource.GetProvider(providerRef)
	if !ok {
		return nil, fmt.Errorf("unknown provider '%v' -> '%v'", rawProviderRef, providerRef)
	}
	return provider, nil
}

func parseProviderRequest(
	pkg tokens.Package, version,
	pluginDownloadURL string, pluginChecksums map[string][]byte,
) (providers.ProviderRequest, error) {
	if version == "" {
		logging.V(5).Infof("parseProviderRequest(%s): semver version is the empty string", pkg)
		return providers.NewProviderRequest(nil, pkg, pluginDownloadURL, pluginChecksums), nil
	}

	parsedVersion, err := semver.Parse(version)
	if err != nil {
		logging.V(5).Infof("parseProviderRequest(%s, %s): semver version string is invalid: %v", pkg, version, err)
		return providers.ProviderRequest{}, err
	}

	url := strings.TrimSuffix(pluginDownloadURL, "/")

	return providers.NewProviderRequest(&parsedVersion, pkg, url, pluginChecksums), nil
}

func (rm *resmon) SupportsFeature(ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest,
) (*pulumirpc.SupportsFeatureResponse, error) {
	hasSupport := false

	// NOTE: DO NOT ADD ANY MORE FEATURES TO THIS LIST
	//
	// Context: https://github.com/pulumi/pulumi-dotnet/pull/88#pullrequestreview-1265714090
	//
	// We shouldn't add any more features to this list, copying strings around codebases is prone to bugs.
	// Rather than adding a new feature here, setup a new SupportsFeatureV2 method, that takes a grpc enum
	// instead. That can then be safely code generated out to each language with no risk of typos.
	//
	// These old features have to stay as is because old engines DO support them, but wouldn't support the new
	// SupportsFeatureV2 method.

	switch req.Id {
	case "secrets":
		hasSupport = true
	case "resourceReferences":
		hasSupport = !rm.disableResourceReferences
	case "outputValues":
		hasSupport = !rm.disableOutputValues
	case "aliasSpecs":
		hasSupport = true
	case "deletedWith":
		hasSupport = true
	}

	logging.V(5).Infof("ResourceMonitor.SupportsFeature(id: %s) = %t", req.Id, hasSupport)

	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: hasSupport,
	}, nil
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *resmon) Invoke(ctx context.Context, req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error) {
	// Fetch the token and load up the resource provider if necessary.
	tok := tokens.ModuleMember(req.GetTok())
	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums())
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, req.GetProvider(), tok)
	if err != nil {
		return nil, fmt.Errorf("Invoke: %w", err)
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
		return nil, fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, fmt.Errorf("invocation of %v returned an error: %w", tok, err)
	}

	// Respect `AcceptResources` unless `tok` is for the built-in `pulumi:pulumi:getResource` function,
	// in which case always keep resources to maintain the original behavior for older SDKs that are not
	// setting the `AccceptResources` flag.
	keepResources := req.GetAcceptResources()
	if tok == "pulumi:pulumi:getResource" {
		keepResources = true
	}

	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: keepResources,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v return: %w", tok, err)
	}
	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(failures))
	for _, failure := range failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}
	return &pulumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

// Call dynamically executes a method in the provider associated with a component resource.
func (rm *resmon) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	// Fetch the token and load up the resource provider if necessary.
	tok := tokens.ModuleMember(req.GetTok())
	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums())
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, req.GetProvider(), tok)
	if err != nil {
		return nil, err
	}

	label := fmt.Sprintf("ResourceMonitor.Call(%s)", tok)

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
			// To initially scope the use of this new feature, we only keep output values when unmarshaling
			// properties for RegisterResource (when remote is true for multi-lang components) and Call.
			KeepOutputValues: true,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
	}

	argDependencies := map[resource.PropertyKey][]resource.URN{}
	for name, deps := range req.GetArgDependencies() {
		urns := make([]resource.URN, len(deps.Urns))
		for i, urn := range deps.Urns {
			urn, err := resource.ParseURN(urn)
			if err != nil {
				return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency on argument %d URN: %s", i, err))
			}
			urns[i] = urn
		}
		argDependencies[resource.PropertyKey(name)] = urns
	}

	info := plugin.CallInfo{
		Project:        rm.constructInfo.Project,
		Stack:          rm.constructInfo.Stack,
		Config:         rm.constructInfo.Config,
		DryRun:         rm.constructInfo.DryRun,
		Parallel:       rm.constructInfo.Parallel,
		MonitorAddress: rm.constructInfo.MonitorAddress,
	}
	options := plugin.CallOptions{
		ArgDependencies: argDependencies,
	}

	// Do the all and then return the arguments.
	logging.V(5).Infof(
		"ResourceMonitor.Call received: tok=%v #args=%v #info=%v #options=%v", tok, len(args), info, options)
	ret, err := prov.Call(tok, args, info, options)
	if err != nil {
		return nil, fmt.Errorf("call of %v returned an error: %w", tok, err)
	}
	mret, err := plugin.MarshalProperties(ret.Return, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v return: %w", tok, err)
	}

	returnDependencies := map[string]*pulumirpc.CallResponse_ReturnDependencies{}
	for name, deps := range ret.ReturnDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		returnDependencies[string(name)] = &pulumirpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(ret.Failures))
	for _, failure := range ret.Failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}
	return &pulumirpc.CallResponse{Return: mret, ReturnDependencies: returnDependencies, Failures: chkfails}, nil
}

func (rm *resmon) StreamInvoke(
	req *pulumirpc.ResourceInvokeRequest, stream pulumirpc.ResourceMonitor_StreamInvokeServer,
) error {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("ResourceMonitor.StreamInvoke(%s)", tok)

	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums())
	if err != nil {
		return err
	}
	prov, err := getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, req.GetProvider(), tok)
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
		return fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
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
			return fmt.Errorf("failed to marshal return: %w", err)
		}

		return stream.Send(&pulumirpc.InvokeResponse{Return: mret})
	})
	if err != nil {
		return fmt.Errorf("streaming invocation of %v returned an error: %w", tok, err)
	}

	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(failures))
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
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	// Read the basic inputs necessary to identify the plugin.
	t, err := tokens.ParseTypeToken(req.GetType())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, err.Error())
	}

	name := tokens.QName(req.GetName())
	parent, err := resource.ParseOptionalURN(req.GetParent())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent URN: %s", err))
	}

	provider := req.GetProvider()
	if !providers.IsProviderType(t) && provider == "" {
		providerReq, err := parseProviderRequest(
			t.Package(), req.GetVersion(),
			req.GetPluginDownloadURL(), req.GetPluginChecksums())
		if err != nil {
			return nil, err
		}
		ref, provErr := rm.defaultProviders.getDefaultProviderRef(providerReq)
		if provErr != nil {
			return nil, provErr
		} else if providers.IsDenyDefaultsProvider(ref) {
			msg := diag.GetDefaultProviderDenied("Read").Message
			return nil, fmt.Errorf(msg, req.GetType(), t)
		}
		provider = ref.String()
	}

	id := resource.ID(req.GetId())
	label := fmt.Sprintf("ResourceMonitor.ReadResource(%s, %s, %s, %s)", id, t, name, provider)
	deps := slice.Prealloc[resource.URN](len(req.GetDependencies()))
	for _, depURN := range req.GetDependencies() {
		urn, err := resource.ParseURN(depURN)
		if err != nil {
			return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency: %s", err))
		}
		deps = append(deps, urn)
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

	additionalSecretOutputs := slice.Prealloc[resource.PropertyKey](len(req.GetAdditionalSecretOutputs()))
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
		sourcePosition:          rm.sourcePositions.getFromRequest(req),
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

	contract.Assertf(result != nil, "ReadResource operation returned a nil result")
	marshaled, err := plugin.MarshalProperties(result.State.Outputs, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   req.GetAcceptSecrets(),
		KeepResources: req.GetAcceptResources(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s return state: %w", result.State.URN, err)
	}

	return &pulumirpc.ReadResourceResponse{
		Urn:        string(result.State.URN),
		Properties: marshaled,
	}, nil
}

// inheritFromParent returns a new goal that inherits from the given parent goal.
// Currently only inherits DeletedWith from parent.
func inheritFromParent(child resource.Goal, parent resource.Goal) *resource.Goal {
	goal := child
	if goal.DeletedWith == "" {
		goal.DeletedWith = parent.DeletedWith
	}
	return &goal
}

type sourcePositions struct {
	projectRoot string
}

func newSourcePositions(projectRoot string) *sourcePositions {
	if projectRoot == "" {
		projectRoot = "/"
	} else {
		contract.Assertf(filepath.IsAbs(projectRoot), "projectRoot is not an absolute path")
		projectRoot = filepath.Clean(projectRoot)
	}
	return &sourcePositions{projectRoot: projectRoot}
}

func (s *sourcePositions) parseSourcePosition(raw *pulumirpc.SourcePosition) (string, error) {
	if raw == nil {
		return "", nil
	}

	if raw.Line <= 0 {
		return "", fmt.Errorf("invalid line number %v", raw.Line)
	}

	col := ""
	if raw.Column != 0 {
		if raw.Column < 0 {
			return "", fmt.Errorf("invalid column number %v", raw.Column)
		}
		col = "," + strconv.FormatInt(int64(raw.Column), 10)
	}

	posURL, err := url.Parse(raw.Uri)
	if err != nil {
		return "", err
	}
	if posURL.Scheme != "file" {
		return "", fmt.Errorf("unrecognized scheme %q", posURL.Scheme)
	}

	file := filepath.FromSlash(posURL.Path)
	if !filepath.IsAbs(file) {
		return "", fmt.Errorf("source positions must include absolute paths")
	}
	rel, err := filepath.Rel(s.projectRoot, file)
	if err != nil {
		return "", fmt.Errorf("making relative path: %w", err)
	}

	posURL.Scheme = "project"
	posURL.Path = "/" + filepath.ToSlash(rel)
	posURL.Fragment = fmt.Sprintf("%v%s", raw.Line, col)

	return posURL.String(), nil
}

// Allow getFromRequest to accept any gRPC request that has a source position (ReadResourceRequest,
// RegisterResourceRequest, ResourceInvokeRequest, and CallRequest).
type hasSourcePosition interface {
	GetSourcePosition() *pulumirpc.SourcePosition
}

// getFromRequest returns any source position information from an incoming request.
func (s *sourcePositions) getFromRequest(req hasSourcePosition) string {
	pos, err := s.parseSourcePosition(req.GetSourcePosition())
	if err != nil {
		logging.V(5).Infof("parsing source position %#v: %v", req.GetSourcePosition(), err)
		return ""
	}
	return pos
}

// requestFromNodeJS returns true if the request is coming from a Node.js language runtime
// or SDK. This is determined by checking if the request has a "pulumi-runtime" metadata
// header with a value of "nodejs". If no "pulumi-runtime" header is present, then it
// checks if the request has a "user-agent" metadata header that has a value that starts
// with "grpc-node-js/".
func requestFromNodeJS(ctx context.Context) bool {
	if md, hasMetadata := metadata.FromIncomingContext(ctx); hasMetadata {
		// Check for the "pulumi-runtime" header first.
		// We'll always respect this header value when present.
		if runtime, ok := md["pulumi-runtime"]; ok {
			return len(runtime) == 1 && runtime[0] == "nodejs"
		}
		// Otherwise, check the "user-agent" header.
		if ua, ok := md["user-agent"]; ok {
			return len(ua) == 1 && strings.HasPrefix(ua[0], "grpc-node-js/")
		}
	}
	return false
}

// transformAliasForNodeJSCompat transforms the alias from the legacy Node.js values to properly specified values.
func transformAliasForNodeJSCompat(alias resource.Alias) resource.Alias {
	contract.Assertf(alias.URN == "", "alias.URN must be empty")
	// The original implementation in the Node.js SDK did not specify aliases correctly:
	//
	// - It did not set NoParent when it should have, but instead set Parent to empty.
	// - It set NoParent to true and left Parent empty when both the alias and resource had no Parent specified.
	//
	// To maintain compatibility with such versions of the Node.js SDK, we transform these incorrectly
	// specified aliases into properly specified ones that work with this implementation of the engine:
	//
	// - { Parent: "", NoParent: false } -> { Parent: "", NoParent: true }
	// - { Parent: "", NoParent: true }  -> { Parent: "", NoParent: false }
	if alias.Parent == "" {
		alias.NoParent = !alias.NoParent
	}
	return alias
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	// Communicate the type, name, and object information to the iterator that is awaiting us.
	name := tokens.QName(req.GetName())
	custom := req.GetCustom()
	remote := req.GetRemote()
	parent, err := resource.ParseOptionalURN(req.GetParent())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent URN: %s", err))
	}
	protect := req.GetProtect()
	deleteBeforeReplaceValue := req.GetDeleteBeforeReplace()
	ignoreChanges := req.GetIgnoreChanges()
	replaceOnChanges := req.GetReplaceOnChanges()
	id := resource.ID(req.GetImportId())
	customTimeouts := req.GetCustomTimeouts()
	retainOnDelete := req.GetRetainOnDelete()
	deletedWith, err := resource.ParseOptionalURN(req.GetDeletedWith())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid DeletedWith URN: %s", err))
	}
	sourcePosition := rm.sourcePositions.getFromRequest(req)

	// Custom resources must have a three-part type so that we can 1) identify if they are providers and 2) retrieve the
	// provider responsible for managing a particular resource (based on the type's Package).
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

	// We handle updating the providers map to include the providers field of the parent if
	// both the current resource and its parent is a component resource.
	func() {
		// Function exists to scope the lock
		rm.componentProvidersLock.Lock()
		defer rm.componentProvidersLock.Unlock()
		if parentsProviders, parentIsComponent := rm.componentProviders[parent]; !custom &&
			parent != "" && parentIsComponent {
			for k, v := range parentsProviders {
				if req.Providers == nil {
					req.Providers = map[string]string{}
				}
				if _, ok := req.Providers[k]; !ok {
					req.Providers[k] = v
				}
			}
		}
	}()

	label := fmt.Sprintf("ResourceMonitor.RegisterResource(%s,%s)", t, name)

	var providerRef providers.Reference
	var providerRefs map[string]string

	if custom && !providers.IsProviderType(t) || remote {
		providerReq, err := parseProviderRequest(
			t.Package(), req.GetVersion(),
			req.GetPluginDownloadURL(), req.GetPluginChecksums())
		if err != nil {
			return nil, err
		}

		providerRef, err = getProviderReference(rm.defaultProviders, providerReq, req.GetProvider())
		if err != nil {
			return nil, err
		}

		providerRefs = make(map[string]string, len(req.GetProviders()))
		for name, provider := range req.GetProviders() {
			ref, err := getProviderReference(rm.defaultProviders, providerReq, provider)
			if err != nil {
				return nil, err
			}
			providerRefs[name] = ref.String()
		}
	}

	aliases := []resource.Alias{}
	for _, aliasURN := range req.GetAliasURNs() {
		urn, err := resource.ParseURN(aliasURN)
		if err != nil {
			return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid alias URN: %s", err))
		}
		aliases = append(aliases, resource.Alias{URN: urn})
	}

	// We assume aliases are properly specified. However, if a request hasn't explicitly
	// indicated that it is using properly specified aliases and the request is coming
	// from Node.js, transform the aliases from the incorrect Node.js values to properly
	// specified values, to maintain backward compatibility for users of older Node.js
	// SDKs that aren't sending properly specified aliases.
	transformAliases := !req.GetAliasSpecs() && requestFromNodeJS(ctx)

	for _, aliasObject := range req.GetAliases() {
		aliasSpec := aliasObject.GetSpec()
		var alias resource.Alias
		if aliasSpec != nil {
			parentURN, err := resource.ParseOptionalURN(aliasSpec.GetParentUrn())
			if err != nil {
				return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent alias URN: %s", err))
			}
			alias = resource.Alias{
				Name:     aliasSpec.Name,
				Type:     aliasSpec.Type,
				Stack:    aliasSpec.Stack,
				Project:  aliasSpec.Project,
				Parent:   parentURN,
				NoParent: aliasSpec.GetNoParent(),
			}
			if transformAliases {
				alias = transformAliasForNodeJSCompat(alias)
			}
		} else {
			urn, err := resource.ParseURN(aliasObject.GetUrn())
			if err != nil {
				return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid alias URN: %s", err))
			}
			alias = resource.Alias{URN: urn}
		}
		aliases = append(aliases, alias)
	}

	dependencies := []resource.URN{}
	for _, dependingURN := range req.GetDependencies() {
		urn, err := resource.ParseURN(dependingURN)
		if err != nil {
			return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency URN: %s", err))
		}
		dependencies = append(dependencies, urn)
	}

	props, err := plugin.UnmarshalProperties(
		req.GetObject(), plugin.MarshalOptions{
			Label:              label,
			KeepUnknowns:       true,
			ComputeAssetHashes: true,
			KeepSecrets:        true,
			KeepResources:      true,
			// To initially scope the use of this new feature, we only keep output values when unmarshaling
			// properties for RegisterResource (when remote is true for multi-lang components) and Call.
			KeepOutputValues: remote,
		})
	if err != nil {
		return nil, err
	}
	if providers.IsProviderType(t) {
		if req.GetVersion() != "" {
			version, err := semver.Parse(req.GetVersion())
			if err != nil {
				return nil, fmt.Errorf("%s: passed invalid version: %w", label, err)
			}
			providers.SetProviderVersion(props, &version)
		}
		if req.GetPluginDownloadURL() != "" {
			providers.SetProviderURL(props, req.GetPluginDownloadURL())
		}

		// Make sure that an explicit provider which doesn't specify its plugin gets the
		// same plugin as the default provider for the package.
		defaultProvider, ok := rm.defaultProviders.defaultProviderInfo[providers.GetProviderPackage(t)]
		if ok && req.GetVersion() == "" && req.GetPluginDownloadURL() == "" {
			if defaultProvider.Version != nil {
				providers.SetProviderVersion(props, defaultProvider.Version)
			}
			if defaultProvider.PluginDownloadURL != "" {
				providers.SetProviderURL(props, defaultProvider.PluginDownloadURL)
			}
		}
	}

	propertyDependencies := make(map[resource.PropertyKey][]resource.URN)
	if len(req.GetPropertyDependencies()) == 0 && !remote {
		// If this request did not specify property dependencies, treat each property as depending on every resource
		// in the request's dependency list. We don't need to do this when remote is true, because all clients that
		// support remote already support passing property dependencies, so there's no need to backfill here.
		for pk := range props {
			propertyDependencies[pk] = dependencies
		}
	} else {
		// Otherwise, unmarshal the per-property dependency information.
		for pk, pd := range req.GetPropertyDependencies() {
			var deps []resource.URN
			for _, d := range pd.Urns {
				urn, err := resource.ParseURN(d)
				if err != nil {
					return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency on property %s URN: %s", pk, err))
				}
				deps = append(deps, urn)
			}
			propertyDependencies[resource.PropertyKey(pk)] = deps
		}
	}

	additionalSecretOutputs := req.GetAdditionalSecretOutputs()

	var deleteBeforeReplace *bool
	if deleteBeforeReplaceValue || req.GetDeleteBeforeReplaceDefined() {
		deleteBeforeReplace = &deleteBeforeReplaceValue
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource received: t=%v, name=%v, custom=%v, #props=%v, parent=%v, protect=%v, "+
			"provider=%v, deps=%v, deleteBeforeReplace=%v, ignoreChanges=%v, aliases=%v, customTimeouts=%v, "+
			"providers=%v, replaceOnChanges=%v, retainOnDelete=%v, deletedWith=%v",
		t, name, custom, len(props), parent, protect, providerRef, dependencies, deleteBeforeReplace, ignoreChanges,
		aliases, customTimeouts, providerRefs, replaceOnChanges, retainOnDelete, deletedWith)

	// If this is a remote component, fetch its provider and issue the construct call. Otherwise, register the resource.
	var result *RegisterResult

	var outputDeps map[string]*pulumirpc.RegisterResourceResponse_PropertyDependencies
	if remote {
		provider, ok := rm.providers.GetProvider(providerRef)
		if providers.IsDenyDefaultsProvider(providerRef) {
			msg := diag.GetDefaultProviderDenied("").Message
			return nil, fmt.Errorf(msg, t.Package().String(), t.String())
		}
		if !ok {
			return nil, fmt.Errorf("unknown provider '%v'", providerRef)
		}

		// Invoke the provider's Construct RPC method.
		options := plugin.ConstructOptions{
			// We don't actually need to send a list of aliases to construct anymore because the engine does
			// all alias construction.
			Aliases:                 []resource.Alias{},
			Dependencies:            dependencies,
			Protect:                 protect,
			PropertyDependencies:    propertyDependencies,
			Providers:               providerRefs,
			AdditionalSecretOutputs: additionalSecretOutputs,
			DeletedWith:             deletedWith,
			IgnoreChanges:           ignoreChanges,
			ReplaceOnChanges:        replaceOnChanges,
			RetainOnDelete:          retainOnDelete,
		}
		if customTimeouts != nil {
			options.CustomTimeouts = &plugin.CustomTimeouts{
				Create: customTimeouts.Create,
				Update: customTimeouts.Update,
				Delete: customTimeouts.Delete,
			}
		}
		if deleteBeforeReplace != nil {
			options.DeleteBeforeReplace = *deleteBeforeReplace
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
		additionalSecretKeys := slice.Prealloc[resource.PropertyKey](len(additionalSecretOutputs))
		for _, name := range additionalSecretOutputs {
			additionalSecretKeys = append(additionalSecretKeys, resource.PropertyKey(name))
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

		goal := resource.NewGoal(t, name, custom, props, parent, protect, dependencies,
			providerRef.String(), nil, propertyDependencies, deleteBeforeReplace, ignoreChanges,
			additionalSecretKeys, aliases, id, &timeouts, replaceOnChanges, retainOnDelete, deletedWith,
			sourcePosition,
		)

		if goal.Parent != "" {
			rm.resGoalsLock.Lock()
			parentGoal, ok := rm.resGoals[goal.Parent]
			if ok {
				goal = inheritFromParent(*goal, parentGoal)
			}
			rm.resGoalsLock.Unlock()
		}
		// Send the goal state to the engine.
		step := &registerResourceEvent{
			goal: goal,
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
		if result != nil && result.State != nil && result.State.URN != "" {
			rm.resGoalsLock.Lock()
			rm.resGoals[result.State.URN] = *goal
			rm.resGoalsLock.Unlock()
		}
	}

	if !custom && result != nil && result.State != nil && result.State.URN != "" {
		func() {
			rm.componentProvidersLock.Lock()
			defer rm.componentProvidersLock.Unlock()
			rm.componentProviders[result.State.URN] = req.GetProviders()
		}()
	}

	// Filter out partially-known values if the requestor does not support them.
	outputs := result.State.Outputs

	// Local ComponentResources may contain unresolved resource refs, so ignore those outputs.
	if !req.GetCustom() && !remote {
		// In the case of a SameStep, the old resource outputs are returned to the language host after the step is
		// executed. The outputs of a ComponentResource may depend on resources that have not been registered at the
		// time the ComponentResource is itself registered, as the outputs are set by a later call to
		// RegisterResourceOutputs. Therefore, when the SameStep returns the old resource outputs for a
		// ComponentResource, it may return references to resources that have not yet been registered, which will cause
		// the SDK's calls to getResource to fail when it attempts to resolve those references.
		//
		// Work on a more targeted fix is tracked in https://github.com/pulumi/pulumi/issues/5978
		outputs = resource.PropertyMap{}
	}

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

	// TODO(@platform):
	// Currently component resources ignore these options:
	// • ignoreChanges
	// • customTimeouts
	// • additionalSecretOutputs
	// • replaceOnChanges
	// • retainOnDelete
	// • deletedWith
	// Revisit these semantics in Pulumi v4.0
	// See this issue for more: https://github.com/pulumi/pulumi/issues/9704
	if !custom {
		rm.checkComponentOption(result.State.URN, "ignoreChanges", func() bool {
			return len(ignoreChanges) > 0
		})
		rm.checkComponentOption(result.State.URN, "customTimeouts", func() bool {
			if customTimeouts == nil {
				return false
			}
			hasUpdateTimeout := customTimeouts.Update != ""
			hasCreateTimeout := customTimeouts.Create != ""
			hasDeleteTimeout := customTimeouts.Delete != ""
			return hasCreateTimeout || hasUpdateTimeout || hasDeleteTimeout
		})
		rm.checkComponentOption(result.State.URN, "additionalSecretOutputs", func() bool {
			return len(additionalSecretOutputs) > 0
		})
		rm.checkComponentOption(result.State.URN, "replaceOnChanges", func() bool {
			return len(replaceOnChanges) > 0
		})
		rm.checkComponentOption(result.State.URN, "retainOnDelete", func() bool {
			return retainOnDelete
		})
		rm.checkComponentOption(result.State.URN, "deletedWith", func() bool {
			return deletedWith != ""
		})
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

	// Assert that we never leak the unconfigured provider ID to the language host.
	contract.Assertf(
		!providers.IsProviderType(result.State.Type) || result.State.ID != providers.UnconfiguredID,
		"provider resource %s has unconfigured ID", result.State.URN)

	return &pulumirpc.RegisterResourceResponse{
		Urn:                  string(result.State.URN),
		Id:                   string(result.State.ID),
		Object:               obj,
		PropertyDependencies: outputDeps,
	}, nil
}

// checkComponentOption generates a warning message on the resource
// 'urn' if 'check' returns true.
// This function is intended to validate options passed to component resources,
// so urn is expected to refer to a component.
func (rm *resmon) checkComponentOption(urn resource.URN, optName string, check func() bool) {
	if check() {
		logging.V(10).Infof("The option '%s' has no automatic effect on component resource '%s', "+
			"ensure it is handled correctly in the component code.", optName, urn)
	}
}

// RegisterResourceOutputs records some new output properties for a resource that have arrived after its initial
// provisioning.  These will make their way into the eventual checkpoint state file for that resource.
func (rm *resmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*pbempty.Empty, error) {
	// Obtain and validate the message's inputs (a URN plus the output property map).
	urn, err := resource.ParseURN(req.Urn)
	if err != nil {
		return nil, fmt.Errorf("invalid resource URN: %s", err)
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
		return nil, fmt.Errorf("cannot unmarshal output properties: %w", err)
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
	sourcePosition          string
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
func (g *readResourceEvent) SourcePosition() string { return g.sourcePosition }

func (g *readResourceEvent) Done(result *ReadResult) {
	g.done <- result
}

func generateTimeoutInSeconds(timeout string) (float64, error) {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return 0, fmt.Errorf("unable to parse customTimeout Value %s", timeout)
	}

	return duration.Seconds(), nil
}

func decorateResourceSpans(span opentracing.Span, method string, req, resp interface{}, grpcError error) {
	if req == nil {
		return
	}

	switch method {
	case "/pulumirpc.ResourceMonitor/Invoke":
		span.SetTag("pulumi-decorator", req.(*pulumirpc.ResourceInvokeRequest).Tok)
	case "/pulumirpc.ResourceMonitor/ReadResource":
		span.SetTag("pulumi-decorator", req.(*pulumirpc.ReadResourceRequest).Type)
	case "/pulumirpc.ResourceMonitor/RegisterResource":
		span.SetTag("pulumi-decorator", req.(*pulumirpc.RegisterResourceRequest).Type)
	}
}
