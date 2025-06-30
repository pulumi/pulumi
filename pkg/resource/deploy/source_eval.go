// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blang/semver"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	interceptors "github.com/pulumi/pulumi/pkg/v3/util/rpcdebug"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
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

	mapset "github.com/deckarep/golang-set/v2"
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

// EvalRunInfoOptions provides options for configuring an evaluation source.
type EvalSourceOptions struct {
	// true if the evaluation is producing resources for a dry-run/preview.
	DryRun bool
	// the degree of parallelism for resource operations (<=1 for serial).
	Parallel int32
	// true to disable resource reference support.
	DisableResourceReferences bool
	// true to disable output value support.
	DisableOutputValues bool
	// AttachDebugger is the list of things to debug.  This can be "program", "all", "plugins", or "plugin:<plugin-name>".
	AttachDebugger []string
}

// NewEvalSource returns a planning source that fetches resources by evaluating a package with a set of args and
// a confgiuration map.  This evaluation is performed using the given plugin context and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(
	plugctx *plugin.Context,
	runinfo *EvalRunInfo,
	defaultProviderInfo map[tokens.Package]workspace.PackageDescriptor,
	opts EvalSourceOptions,
) Source {
	return &evalSource{
		plugctx:             plugctx,
		runinfo:             runinfo,
		defaultProviderInfo: defaultProviderInfo,
		opts:                opts,
	}
}

type evalSource struct {
	plugctx             *plugin.Context                                // the plugin context.
	runinfo             *EvalRunInfo                                   // the directives to use when running the program.
	defaultProviderInfo map[tokens.Package]workspace.PackageDescriptor // the default provider versions for this source.
	opts                EvalSourceOptions                              // options for the evaluation source.
}

func (src *evalSource) Close() error {
	return nil
}

// Project is the name of the project being run by this evaluation source.
func (src *evalSource) Project() tokens.PackageName {
	return src.runinfo.Proj.Name
}

// Stack is the name of the stack being targeted by this evaluation source.
func (src *evalSource) Stack() tokens.StackName {
	return src.runinfo.Target.Name
}

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
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
	finChan := make(chan error)
	programComplete := &promise.CompletionSource[struct{}]{}

	mon, err := newResourceMonitor(
		src,
		providers,
		regChan,
		regOutChan,
		regReadChan,
		finChan,
		programComplete.Promise(),
		config,
		configSecretKeys,
		tracingSpan,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start resource monitor: %w", err)
	}

	// Also start up a schema loader for the language runtime to use to fetch schema information.
	loaderRegistration := schema.LoaderRegistration(
		schema.NewLoaderServer(schema.NewPluginLoader(src.plugctx.Host)))
	loaderServer, err := plugin.NewServer(src.plugctx, loaderRegistration)
	if err != nil {
		return nil, fmt.Errorf("failed to start loader server: %w", err)
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		loaderServer:    loaderServer,
		mon:             mon,
		src:             src,
		regChan:         regChan,
		regOutChan:      regOutChan,
		regReadChan:     regReadChan,
		finChan:         finChan,
		programComplete: programComplete,
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(config, configSecretKeys, configMap)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	loaderServer *plugin.GrpcServer                 // the grpc server for the schema loader.
	mon          SourceResourceMonitor              // the resource monitor, per iterator.
	src          *evalSource                        // the owning eval source object.
	regChan      chan *registerResourceEvent        // the channel that contains resource registrations.
	regOutChan   chan *registerResourceOutputsEvent // the channel that contains resource completions.
	regReadChan  chan *readResourceEvent            // the channel that contains read resource requests.
	// the channel that communicates that no more events will be sent from the program.
	finChan         chan error
	programComplete *promise.CompletionSource[struct{}] // the completion source to record program completion.
	done            bool                                // set to true when the evaluation is done.
	aborted         bool                                // set to true when the iterator is aborted.
}

func (iter *evalSourceIterator) Cancel(ctx context.Context) error {
	// Cancel the monitor and reclaim any associated resources.
	return iter.mon.Cancel(ctx)
}

func (iter *evalSourceIterator) ResourceMonitor() SourceResourceMonitor {
	return iter.mon
}

func (iter *evalSourceIterator) Next() (SourceEvent, error) {
	// if the iterator is aborted, return an error.
	if iter.aborted {
		return nil, result.BailErrorf("EvalSourceIterator aborted")
	}
	// If we are done, quit.
	if iter.done {
		return nil, nil
	}

	// Await the program to compute some more state and then inspect what it has to say.
	select {
	case <-iter.mon.AbortChan():
		iter.aborted = true
		return nil, result.BailErrorf("EvalSourceIterator aborted")
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

// forkRun performs the evaluation from a distinct goroutine. This function blocks until it's our turn to go.
func (iter *evalSourceIterator) forkRun(
	config map[config.Key]string,
	configSecretKeys []config.Key,
	configPropertyMap resource.PropertyMap,
) {
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go func() {
		// Next, launch the language plugin.
		run := func() error {
			rt := iter.src.runinfo.Proj.Runtime.Name()

			rtopts := iter.src.runinfo.Proj.Runtime.Options()
			programInfo := plugin.NewProgramInfo(
				/* rootDirectory */ iter.src.runinfo.ProjectRoot,
				/* programDirectory */ iter.src.runinfo.Pwd,
				/* entryPoint */ iter.src.runinfo.Program,
				/* options */ rtopts)

			langhost, err := iter.src.plugctx.Host.LanguageRuntime(rt, programInfo)
			if err != nil {
				return fmt.Errorf("failed to launch language host %s: %w", rt, err)
			}
			contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

			// Now run the actual program.
			progerr, bail, err := langhost.Run(plugin.RunInfo{
				MonitorAddress:    iter.mon.Address(),
				Stack:             iter.src.runinfo.Target.Name.String(),
				Project:           string(iter.src.runinfo.Proj.Name),
				Pwd:               iter.src.runinfo.Pwd,
				Args:              iter.src.runinfo.Args,
				Config:            config,
				ConfigSecretKeys:  configSecretKeys,
				ConfigPropertyMap: configPropertyMap,
				DryRun:            iter.src.opts.DryRun,
				Parallel:          iter.src.opts.Parallel,
				Organization:      string(iter.src.runinfo.Target.Organization),
				Info:              programInfo,
				LoaderAddress:     iter.loaderServer.Addr(),
				AttachDebugger:    iter.src.plugctx.Host.AttachDebugger(plugin.DebugSpec{Type: plugin.DebugTypeProgram}),
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
		err := run()
		if err != nil {
			logging.V(5).Infof("Program exited with error: %s", err)
		} else {
			logging.V(5).Infof("Program exited with no error")
		}

		// Signal that the program as exited.
		if err != nil {
			iter.programComplete.Reject(err)
		} else {
			iter.programComplete.Fulfill(struct{}{})
		}
		// Signal that the program will not be generating further events. New
		// SDKs will already have signalled to `iter.finChan` via
		// `SignalAndWaitForShutdown`, but old SDKs signal completion here when
		// they exit.
		iter.finChan <- err
	}()
}

// defaultProviders manages the registration of default providers. The default provider for a package is the provider
// resource that will be used to manage resources that do not explicitly reference a provider. Default providers will
// only be registered for packages that are used by resources registered by the user's Pulumi program.
type defaultProviders struct {
	// A map of package identifiers to versions, used to disambiguate which plugin to load if no version is provided
	// by the language host.
	defaultProviderInfo map[tokens.Package]workspace.PackageDescriptor

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

func (d *defaultProviders) normalizeProviderRequest(req providers.ProviderRequest) providers.ProviderRequest {
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
		logging.V(5).Infof("normalizeProviderRequest(%s): using version %s from request", req, req.Version())
	} else {
		if version := d.defaultProviderInfo[req.Package()].Version; version != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default version hit on version %s", req, version)
			req = providers.NewProviderRequest(
				req.Package(), version, req.PluginDownloadURL(), req.PluginChecksums(), req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default provider miss, sending nil version to engine", req)
		}
	}

	if req.PluginDownloadURL() != "" {
		logging.V(5).Infof("normalizeProviderRequest(%s): using pluginDownloadURL %s from request",
			req, req.PluginDownloadURL())
	} else {
		if pluginDownloadURL := d.defaultProviderInfo[req.Package()].PluginDownloadURL; pluginDownloadURL != "" {
			logging.V(5).Infof("normalizeProviderRequest(%s): default pluginDownloadURL hit on %s",
				req, pluginDownloadURL)
			req = providers.NewProviderRequest(
				req.Package(), req.Version(), pluginDownloadURL, req.PluginChecksums(), req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default pluginDownloadURL miss, sending empty string to engine", req)
		}
	}

	if req.PluginChecksums() != nil {
		logging.V(5).Infof("normalizeProviderRequest(%s): using pluginChecksums %v from request",
			req, req.PluginChecksums())
	} else {
		if pluginChecksums := d.defaultProviderInfo[req.Package()].Checksums; pluginChecksums != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default pluginChecksums hit on %v",
				req, pluginChecksums)
			req = providers.NewProviderRequest(
				req.Package(), req.Version(), req.PluginDownloadURL(), pluginChecksums, req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default pluginChecksums miss, sending empty map to engine", req)
		}
	}

	if req.Parameterization() != nil {
		logging.V(5).Infof("normalizeProviderRequest(%s): using parameterization %v from request",
			req, req.Parameterization())
	} else {
		if parameterization := d.defaultProviderInfo[req.Package()].Parameterization; parameterization != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default parameterization hit on %v",
				req, parameterization)

			req = providers.NewProviderRequest(
				req.Package(), req.Version(), req.PluginDownloadURL(), req.PluginChecksums(), parameterization)
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default parameterization miss, sending nil to engine", req)
		}
	}

	return req
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
	if req.Version() != nil {
		providers.SetProviderVersion(inputs, req.Version())
	}
	if req.PluginDownloadURL() != "" {
		providers.SetProviderURL(inputs, req.PluginDownloadURL())
	}
	if req.PluginChecksums() != nil {
		providers.SetProviderChecksums(inputs, req.PluginChecksums())
	}
	if req.Parameterization() != nil {
		providers.SetProviderName(inputs, req.Name())
		providers.SetProviderParameterization(inputs, req.Parameterization())
	}

	// Create the result channel and the event.
	done := make(chan *RegisterResult)
	event := &registerResourceEvent{
		goal: resource.NewGoal(
			providers.MakeProviderType(req.Package()),
			req.DefaultName(), true, inputs, "", nil, nil, "", nil, nil, nil,
			nil, nil, nil, "", nil, nil, nil, "", ""),
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

	req = d.normalizeProviderRequest(req)

	denyCreation, err := d.shouldDenyRequest(req)
	if err != nil {
		return providers.Reference{}, err
	}
	if denyCreation {
		logging.V(5).Infof("denied default provider request for package %s", req)
		return providers.NewDenyDefaultProvider(string(req.Package().Name())), nil
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
			return true, errors.New("Unexpected encoding of pulumi:disable-default-providers")
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

// A transformation function that can be applied to a resource.
type TransformFunction func(
	ctx context.Context,
	name, typ string, custom bool, parent resource.URN,
	props resource.PropertyMap,
	opts *pulumirpc.TransformResourceOptions,
) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error)

// A transformation function that can be applied to an invoke.
type TransformInvokeFunction func(
	ctx context.Context, token string, args resource.PropertyMap,
	opts *pulumirpc.TransformInvokeOptions,
) (resource.PropertyMap, *pulumirpc.TransformInvokeOptions, error)

type CallbacksClient struct {
	pulumirpc.CallbacksClient

	conn *grpc.ClientConn
}

func (c *CallbacksClient) Close() error {
	return c.conn.Close()
}

func NewCallbacksClient(conn *grpc.ClientConn) *CallbacksClient {
	return &CallbacksClient{
		CallbacksClient: pulumirpc.NewCallbacksClient(conn),
		conn:            conn,
	}
}

// resmon implements the pulumirpc.ResourceMonitor interface and acts as the gateway between a language runtime's
// evaluation of a program and the internal resource planning and deployment logic.
type resmon struct {
	pulumirpc.UnsafeResourceMonitorServer

	pendingTransforms     map[string][]TransformFunction // pending transformation functions for a constructed resource
	pendingTransformsLock sync.Mutex

	parents     map[resource.URN]resource.URN // map of child URNs to their parent URNs
	parentsLock sync.Mutex

	resGoals               map[resource.URN]resource.Goal     // map of seen URNs and their goals.
	resGoalsLock           sync.Mutex                         // locks the resGoals map.
	diagnostics            diag.Sink                          // logger for user-facing messages
	providers              ProviderSource                     // the provider source itself.
	componentProviders     map[resource.URN]map[string]string // which providers component resources used
	componentProvidersLock sync.Mutex                         // which locks the componentProviders map
	defaultProviders       *defaultProviders                  // the default provider manager.
	sourcePositions        *sourcePositions                   // source position manager.
	constructInfo          plugin.ConstructInfo               // information for construct and call calls.
	regChan                chan *registerResourceEvent        // the channel to send resource registrations to.
	regOutChan             chan *registerResourceOutputsEvent // the channel to send resource output registrations to.
	regReadChan            chan *readResourceEvent            // the channel to send resource reads to.
	abortChan              chan bool                          // a channel that can abort iteration of resources.
	cancel                 chan bool                          // a channel that can cancel the server.
	done                   <-chan error                       // a channel that resolves when the server completes.
	// a channel to signal that no more events will be sent from the program.
	finChan             chan<- error
	programComplete     *promise.Promise[struct{}] // a promise that resolves when the program has exited.
	waitForShutdownChan chan struct{}              // a channel on which the runtime can wait before shutting down.
	hasWaiter           atomic.Bool                // indicates whether something is waiting on `waitForShutdownChan`.
	opts                EvalSourceOptions          // options for the resource monitor.

	// the working directory for the resources sent to this monitor.
	workingDirectory string

	stackTransformsLock       sync.Mutex
	stackTransforms           []TransformFunction // stack transformation functions
	stackInvokeTransformsLock sync.Mutex
	stackInvokeTransforms     []TransformInvokeFunction // invoke transformation functions
	resourceTransformsLock    sync.Mutex
	resourceTransforms        map[resource.URN][]TransformFunction // option transformation functions per resource
	callbacksLock             sync.Mutex
	callbacks                 map[string]*CallbacksClient // callbacks clients per target address
	grpcDialOptions           func(metadata interface{}) []grpc.DialOption

	packageRefLock sync.Mutex
	// A map of UUIDs to the description of a provider package they correspond to
	packageRefMap map[string]providers.ProviderRequest
}

var _ SourceResourceMonitor = (*resmon)(nil)

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(
	src *evalSource,
	provs ProviderSource,
	regChan chan *registerResourceEvent,
	regOutChan chan *registerResourceOutputsEvent,
	regReadChan chan *readResourceEvent,
	finChan chan<- error,
	programComplete *promise.Promise[struct{}],
	config map[config.Key]string,
	configSecretKeys []config.Key,
	tracingSpan opentracing.Span,
) (*resmon, error) {
	abortChan := make(chan bool)

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
		diagnostics:         src.plugctx.Diag,
		providers:           provs,
		defaultProviders:    d,
		workingDirectory:    src.runinfo.Pwd,
		sourcePositions:     newSourcePositions(src.runinfo.ProjectRoot),
		pendingTransforms:   map[string][]TransformFunction{},
		parents:             map[resource.URN]resource.URN{},
		resGoals:            map[resource.URN]resource.Goal{},
		componentProviders:  map[resource.URN]map[string]string{},
		regChan:             regChan,
		regOutChan:          regOutChan,
		regReadChan:         regReadChan,
		abortChan:           abortChan,
		cancel:              cancel,
		finChan:             finChan,
		programComplete:     programComplete,
		waitForShutdownChan: make(chan struct{}, 1),
		opts:                src.opts,
		callbacks:           map[string]*CallbacksClient{},
		resourceTransforms:  map[resource.URN][]TransformFunction{},
		packageRefMap:       map[string]providers.ProviderRequest{},
		grpcDialOptions:     src.plugctx.DialOptions,
	}

	// Fire up a gRPC server and start listening for incomings.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: resmon.cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, resmon)
			return nil
		},
		Options: sourceEvalServeOptions(src.plugctx, tracingSpan, env.DebugGRPC.Value()),
	})
	if err != nil {
		return nil, err
	}

	resmon.constructInfo = plugin.ConstructInfo{
		Project:          string(src.runinfo.Proj.Name),
		Stack:            src.runinfo.Target.Name.String(),
		Config:           config,
		ConfigSecretKeys: configSecretKeys,
		DryRun:           src.opts.DryRun,
		Parallel:         src.opts.Parallel,
		MonitorAddress:   fmt.Sprintf("127.0.0.1:%d", handle.Port),
	}
	resmon.done = handle.Done

	go d.serve()

	return resmon, nil
}

func (rm *resmon) AbortChan() <-chan bool {
	return rm.abortChan
}

// Get or allocate a new grpc client for the given callback address.
func (rm *resmon) GetCallbacksClient(target string) (*CallbacksClient, error) {
	rm.callbacksLock.Lock()
	defer rm.callbacksLock.Unlock()

	if client, has := rm.callbacks[target]; has {
		return client, nil
	}

	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if rm.grpcDialOptions != nil {
		opts := rm.grpcDialOptions(map[string]interface{}{
			"mode": "client",
			"kind": "callbacks",
		})
		dialOpts = append(dialOpts, opts...)
	}

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, err
	}

	client := NewCallbacksClient(conn)
	rm.callbacks[target] = client
	return client, nil
}

// Address returns the address at which the monitor's RPC server may be reached.
func (rm *resmon) Address() string {
	return rm.constructInfo.MonitorAddress
}

// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
func (rm *resmon) Cancel(ctx context.Context) error {
	// By closing `rm.cancel` we cancel all in flight steps and initiate the
	// graceful shutdown of the server. We won't accept any new connections, but
	// pending connections will be allowed to complete.
	//
	// We need to do this before receiving from `rm.programCompleteChan`,
	// otherwise we may deadlock: For example a `RegisterResource` call will not
	// return to the program until we cancel, and we will never write to
	// `rm.programCompleteChan` unless the program exits.
	close(rm.cancel)
	close(rm.waitForShutdownChan)                   // Signal to the program that we are ready to shutdown ...
	_, programErr := rm.programComplete.Result(ctx) // ... and wait for the program to complete.
	errs := []error{<-rm.done, programErr}
	for _, client := range rm.callbacks {
		errs = append(errs, client.Close())
	}
	return errors.Join(errs...)
}

func sourceEvalServeOptions(ctx *plugin.Context, tracingSpan opentracing.Span, logFile string) []grpc.ServerOption {
	serveOpts := rpcutil.OpenTracingServerInterceptorOptions(
		tracingSpan,
		otgrpc.SpanDecorator(decorateResourceSpans),
	)
	if logFile != "" {
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
func (rm *resmon) getProviderReference(defaultProviders *defaultProviders, req providers.ProviderRequest,
	rawProviderRef string,
) (providers.Reference, error) {
	if rawProviderRef != "" {
		ref, err := providers.ParseReference(rawProviderRef)
		if err != nil {
			return providers.Reference{}, fmt.Errorf("could not parse provider reference: %w", err)
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
func (rm *resmon) getProviderFromSource(
	providerSource ProviderSource, defaultProviders *defaultProviders,
	req providers.ProviderRequest, rawProviderRef string,
	token tokens.ModuleMember,
) (plugin.Provider, error) {
	providerRef, err := rm.getProviderReference(defaultProviders, req, rawProviderRef)
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
	parameterization *workspace.Parameterization,
) (providers.ProviderRequest, error) {
	if version == "" {
		logging.V(5).Infof("parseProviderRequest(%s): semver version is the empty string", pkg)
		return providers.NewProviderRequest(pkg, nil, pluginDownloadURL, pluginChecksums, parameterization), nil
	}

	parsedVersion, err := semver.Parse(version)
	if err != nil {
		logging.V(5).Infof("parseProviderRequest(%s, %s): semver version string is invalid: %v", pkg, version, err)
		return providers.ProviderRequest{}, err
	}

	url := strings.TrimSuffix(pluginDownloadURL, "/")

	return providers.NewProviderRequest(pkg, &parsedVersion, url, pluginChecksums, parameterization), nil
}

func (rm *resmon) RegisterPackage(ctx context.Context,
	req *pulumirpc.RegisterPackageRequest,
) (*pulumirpc.RegisterPackageResponse, error) {
	logging.V(5).Infof("ResourceMonitor.RegisterPackage(%v)", req)

	name := tokens.Package(req.Name)
	if name == "" {
		return nil, errors.New("package name is empty")
	}

	// First parse the request into a ProviderRequest
	var version *semver.Version
	if req.Version != "" {
		v, err := semver.Parse(req.Version)
		if err != nil {
			return nil, fmt.Errorf("parse package version %s: %w", req.Version, err)
		}
		version = &v
	}
	// Parse the parameterization
	var parameterization *workspace.Parameterization
	if req.Parameterization != nil {
		parameterizationVersion, err := semver.Parse(req.Parameterization.Version)
		if err != nil {
			return nil, fmt.Errorf("parse parameter version %s: %w", req.Parameterization.Version, err)
		}

		// RegisterPackageRequest keeps all the plugin information in the root fields "name", "version" etc, while the
		// information about the parameterized package is in the "parameterization" field. Internally in the engine, and
		// for resource state we need to flip that around a bit.
		parameterization = &workspace.Parameterization{
			Name:    req.Parameterization.Name,
			Version: parameterizationVersion,
			Value:   req.Parameterization.Value,
		}
	}

	pi := providers.NewProviderRequest(
		tokens.Package(req.Name), version, req.DownloadUrl, req.Checksums,
		parameterization)

	rm.packageRefLock.Lock()
	defer rm.packageRefLock.Unlock()

	// See if this package is already registered, else add it to the map.
	for uuid, candidate := range rm.packageRefMap {
		if reflect.DeepEqual(candidate, pi) {
			logging.V(5).Infof("ResourceMonitor.RegisterPackage(%v) matched %s", req, uuid)
			return &pulumirpc.RegisterPackageResponse{Ref: uuid}, nil
		}
	}

	// Wasn't found add it to the map
	uuid := uuid.New().String()
	rm.packageRefMap[uuid] = pi
	logging.V(5).Infof("ResourceMonitor.RegisterPackage(%v) created %s", req, uuid)
	return &pulumirpc.RegisterPackageResponse{Ref: uuid}, nil
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
		hasSupport = !rm.opts.DisableResourceReferences
	case "outputValues":
		hasSupport = !rm.opts.DisableOutputValues
	case "aliasSpecs":
		hasSupport = true
	case "deletedWith":
		hasSupport = true
	case "transforms":
		hasSupport = true
	case "invokeTransforms":
		hasSupport = true
	case "parameterization":
		// N.B This serves a dual purpose of also indicating that package references are supported.
		hasSupport = true
	}

	logging.V(5).Infof("ResourceMonitor.SupportsFeature(id: %s) = %t", req.Id, hasSupport)

	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: hasSupport,
	}, nil
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *resmon) Invoke(ctx context.Context, req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error) {
	// Fetch the token.
	tok := tokens.ModuleMember(req.GetTok())

	label := fmt.Sprintf("ResourceMonitor.Invoke(%s)", tok)
	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:            label,
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			WorkingDirectory: rm.workingDirectory,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
	}

	opts := &pulumirpc.TransformInvokeOptions{
		Provider:          req.GetProvider(),
		Version:           req.GetVersion(),
		PluginDownloadUrl: req.GetPluginDownloadURL(),
		PluginChecksums:   req.GetPluginChecksums(),
	}

	// Lock the invoke transforms and run all of those before loading the provider.
	err = func() error {
		// Function exists to scope the lock
		rm.stackInvokeTransformsLock.Lock()
		defer rm.stackInvokeTransformsLock.Unlock()

		for _, transform := range rm.stackInvokeTransforms {
			newArgs, newOpts, err := transform(ctx, string(tok), args, opts)
			if err != nil {
				return err
			}

			args = newArgs
			opts = newOpts
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Load up the resource provider if necessary.
	providerReq, err := parseProviderRequest(
		tok.Package(), opts.Version, opts.PluginDownloadUrl, opts.PluginChecksums, nil)
	if err != nil {
		return nil, err
	}

	packageRef := req.GetPackageRef()
	if packageRef != "" {
		var has bool
		providerReq, has = rm.packageRefMap[packageRef]
		if !has {
			return nil, fmt.Errorf("unknown provider package '%v'", packageRef)
		}
	}

	prov, err := rm.getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, opts.Provider, tok)
	if err != nil {
		return nil, fmt.Errorf("Invoke: %w", err)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	resp, err := prov.Invoke(ctx, plugin.InvokeRequest{
		Tok:  tok,
		Args: args,
	})
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

	mret, err := plugin.MarshalProperties(resp.Properties, plugin.MarshalOptions{
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    keepResources,
		WorkingDirectory: rm.workingDirectory,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v return: %w", tok, err)
	}
	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(resp.Failures))
	for _, failure := range resp.Failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}
	return &pulumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

// Call dynamically executes a method in the provider associated with a component resource.
func (rm *resmon) Call(ctx context.Context, req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error) {
	// Fetch the token and load up the resource provider if necessary.
	tok := tokens.ModuleMember(req.GetTok())

	// In order to allow method calls on *provider resources themselves*, we'll check if the token references a provider
	// type (e.g. pulumi:providers:<package>/<method>). If it does, we need to use that provider resource *both* as the
	// receiver of the method *and* the provider instance that handles the `Call` implementation itself. That is, we
	// *don't* want to e.g. boot up a default provider instance and then call `Call` on it with a `__self__` referencing
	// some explicit provider -- `__self__` and the handling instance must be the same. To this end, if we see a provider
	// token type, we'll build a provider request and reference from the token and `__self__` arguments. If it doesn't,
	// we'll proceed as normal, using the request's provider information to boot an instance.
	var providerReq providers.ProviderRequest
	var rawProviderRef string
	var err error
	if providers.IsProviderType(tokens.Type(tok)) {
		parts := strings.Split(tok.Name().String(), "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid provider method token %v", tok)
		}

		packageName := tokens.Package(parts[0])
		providerReq, err = parseProviderRequest(
			packageName, req.GetVersion(),
			req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)

		self, ok := req.GetArgs().Fields["__self__"]
		if !ok {
			return nil, errors.New("missing __self__ argument for provider method call")
		}

		selfFields := self.GetStructValue().Fields
		if selfFields == nil {
			return nil, errors.New("missing __self__ argument properties for provider method call")
		}

		provURN, hasProvURN := self.GetStructValue().Fields["urn"]
		if !hasProvURN {
			return nil, errors.New("missing __self__.urn for provider method call")
		}

		provID, hasProvID := self.GetStructValue().Fields["id"]
		if !hasProvID {
			return nil, errors.New("missing __self__.id for provider method call")
		}

		rawProviderRef = fmt.Sprintf("%s::%s", provURN.GetStringValue(), provID.GetStringValue())
	} else {
		providerReq, err = parseProviderRequest(
			tok.Package(), req.GetVersion(),
			req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)

		rawProviderRef = req.GetProvider()
	}
	if err != nil {
		return nil, err
	}

	// If we've got a package reference, this takes precedence over any provider request we'd compute.
	packageRef := req.GetPackageRef()
	if packageRef != "" {
		var has bool
		providerReq, has = rm.packageRefMap[packageRef]
		if !has {
			return nil, fmt.Errorf("unknown provider package '%v'", packageRef)
		}
	}

	prov, err := rm.getProviderFromSource(rm.providers, rm.defaultProviders, providerReq, rawProviderRef, tok)
	if err != nil {
		return nil, err
	}

	label := fmt.Sprintf("ResourceMonitor.Call(%s)", tok)

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:                 label,
			KeepUnknowns:          true,
			KeepSecrets:           true,
			KeepResources:         true,
			KeepOutputValues:      true,
			UpgradeToOutputValues: true,
			WorkingDirectory:      rm.workingDirectory,
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

	// If we have output values we can add the dependencies from them to the args dependencies map we send to the provider.
	for key, output := range args {
		argDependencies[key] = extendOutputDependencies(argDependencies[key], output)
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
	ret, err := prov.Call(ctx, plugin.CallRequest{
		Tok:     tok,
		Args:    args,
		Info:    info,
		Options: options,
	})
	if err != nil {
		var rpcError error
		rpcError, ok := rpcerror.FromError(err)
		if !ok {
			rpcError = err
		}

		message := errorToMessage(rpcError, args)
		rm.diagnostics.Errorf(diag.GetCallFailedError(), tok, message)

		rm.abortChan <- true
		<-rm.cancel
		return nil, fmt.Errorf("call of %v returned an error: %w", tok, err)
	}

	if ret.ReturnDependencies == nil {
		ret.ReturnDependencies = map[resource.PropertyKey][]resource.URN{}
	}
	for k, v := range ret.Return {
		ret.ReturnDependencies[k] = extendOutputDependencies(ret.ReturnDependencies[k], v)
	}

	returnDependencies := map[string]*pulumirpc.CallResponse_ReturnDependencies{}
	for name, deps := range ret.ReturnDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		returnDependencies[string(name)] = &pulumirpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	mret, err := plugin.MarshalProperties(ret.Return, plugin.MarshalOptions{
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		WorkingDirectory: rm.workingDirectory,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v return: %w", tok, err)
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

// ReadResource reads the current state associated with a resource from its provider plugin.
func (rm *resmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	// Read the basic inputs necessary to identify the plugin.
	t, err := tokens.ParseTypeToken(req.GetType())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, err.Error())
	}

	name := req.GetName()
	parent, err := resource.ParseOptionalURN(req.GetParent())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent URN: %s", err))
	}

	provider := req.GetProvider()
	if !providers.IsProviderType(t) && provider == "" {
		providerReq, err := parseProviderRequest(
			t.Package(), req.GetVersion(),
			req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)
		if err != nil {
			return nil, err
		}

		packageRef := req.GetPackageRef()
		if packageRef != "" {
			var has bool
			providerReq, has = rm.packageRefMap[packageRef]
			if !has {
				return nil, fmt.Errorf("unknown provider package '%v'", packageRef)
			}
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
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		WorkingDirectory: rm.workingDirectory,
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
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      req.GetAcceptSecrets(),
		KeepResources:    req.GetAcceptResources(),
		WorkingDirectory: rm.workingDirectory,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s return state: %w", result.State.URN, err)
	}

	return &pulumirpc.ReadResourceResponse{
		Urn:        string(result.State.URN),
		Properties: marshaled,
	}, nil
}

// Wrap the transform callback so the engine can call the callback server, which will then execute the function.  The
// wrapper takes care of all the necessary marshalling and unmarshalling.
func (rm *resmon) wrapTransformCallback(cb *pulumirpc.Callback) (TransformFunction, error) {
	client, err := rm.GetCallbacksClient(cb.Target)
	if err != nil {
		return nil, err
	}

	token := cb.Token
	return func(
		ctx context.Context, name, typ string, custom bool, parent resource.URN,
		props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
	) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
		logging.V(5).Infof("Transform: name=%v type=%v custom=%v parent=%v props=%v opts=%v",
			name, typ, custom, parent, props, opts)

		mopts := plugin.MarshalOptions{
			KeepUnknowns:       true,
			KeepSecrets:        true,
			KeepResources:      true,
			KeepOutputValues:   true,
			WorkingDirectory:   rm.workingDirectory,
			ComputeAssetHashes: true,
		}

		mprops, err := plugin.MarshalProperties(props, mopts)
		if err != nil {
			return nil, nil, err
		}

		request, err := proto.Marshal(&pulumirpc.TransformRequest{
			Name:       name,
			Type:       typ,
			Parent:     string(parent),
			Custom:     custom,
			Properties: mprops,
			Options:    opts,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling request: %w", err)
		}

		resp, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{
			Token:   token,
			Request: request,
		})
		if err != nil {
			logging.V(5).Infof("Transform callback error: %v", err)
			return nil, nil, err
		}

		var response pulumirpc.TransformResponse
		err = proto.Unmarshal(resp.Response, &response)
		if err != nil {
			return nil, nil, fmt.Errorf("unmarshaling response: %w", err)
		}

		newOpts := opts
		if response.Options != nil {
			newOpts = response.Options
		}

		newProps := props
		if response.Properties != nil {
			newProps, err = plugin.UnmarshalProperties(response.Properties, mopts)
			if err != nil {
				return nil, nil, err
			}
		}

		logging.V(5).Infof("Transform: props=%v opts=%v", newProps, newOpts)

		return newProps, newOpts, nil
	}, nil
}

// Wrap the transform callback so the engine can call the callback server, which will then execute the function.  The
// wrapper takes care of all the necessary marshalling and unmarshalling.
func (rm *resmon) wrapInvokeTransformCallback(cb *pulumirpc.Callback) (TransformInvokeFunction, error) {
	client, err := rm.GetCallbacksClient(cb.Target)
	if err != nil {
		return nil, err
	}

	token := cb.Token
	return func(
		ctx context.Context, invokeToken string,
		args resource.PropertyMap, opts *pulumirpc.TransformInvokeOptions,
	) (resource.PropertyMap, *pulumirpc.TransformInvokeOptions, error) {
		logging.V(5).Infof("Invoke transform: token=%v props=%v opts=%v",
			invokeToken, args, opts)

		margs := plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
			WorkingDirectory: rm.workingDirectory,
		}

		mprops, err := plugin.MarshalProperties(args, margs)
		if err != nil {
			return nil, nil, err
		}

		var request []byte
		request, err = proto.Marshal(&pulumirpc.TransformInvokeRequest{
			Token:   invokeToken,
			Args:    mprops,
			Options: opts,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling request: %w", err)
		}

		resp, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{
			Token:   token,
			Request: request,
		})
		if err != nil {
			logging.V(5).Infof("Invoke transform callback error: %v", err)
			return nil, nil, err
		}

		newOpts := opts
		var newProps resource.PropertyMap
		var response pulumirpc.TransformInvokeResponse
		err = proto.Unmarshal(resp.Response, &response)
		if err != nil {
			return nil, nil, fmt.Errorf("unmarshaling response: %w", err)
		}

		if response.Options != nil {
			newOpts = response.Options
		}
		newProps = args
		if response.Args != nil {
			newProps, err = plugin.UnmarshalProperties(response.Args, margs)
			if err != nil {
				return nil, nil, err
			}
		}

		logging.V(5).Infof("Invoke transform: props=%v opts=%v", newProps, newOpts)

		return newProps, newOpts, nil
	}, nil
}

func (rm *resmon) RegisterStackTransform(ctx context.Context, cb *pulumirpc.Callback) (*emptypb.Empty, error) {
	rm.stackTransformsLock.Lock()
	defer rm.stackTransformsLock.Unlock()

	if cb.Target == "" {
		return nil, errors.New("target must be specified")
	}

	wrapped, err := rm.wrapTransformCallback(cb)
	if err != nil {
		return nil, err
	}

	rm.stackTransforms = append(rm.stackTransforms, wrapped)
	return &emptypb.Empty{}, nil
}

func (rm *resmon) RegisterStackInvokeTransform(ctx context.Context, cb *pulumirpc.Callback) (*emptypb.Empty, error) {
	rm.stackInvokeTransformsLock.Lock()
	defer rm.stackInvokeTransformsLock.Unlock()

	if cb.Target == "" {
		return nil, errors.New("target must be specified")
	}

	wrapped, err := rm.wrapInvokeTransformCallback(cb)
	if err != nil {
		return nil, err
	}

	rm.stackInvokeTransforms = append(rm.stackInvokeTransforms, wrapped)
	return &emptypb.Empty{}, nil
}

// SignalAndWaitForShutdown lets the resource monitor know that no more events
// will be generated. This call blocks until the resource monitor is finished,
// which will happen once all the steps have executed. This allows the language
// runtime to stay running and handle callback requests, even after the user
// program has completed. Runtime SDKs should call this after executing the
// user's program. This can only be called once.
func (rm *resmon) SignalAndWaitForShutdown(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	logging.V(6).Infof("SignalAndWaitForShutdown waiting ...")
	if rm.hasWaiter.CompareAndSwap(false, true) {
		rm.finChan <- nil        // Let the source iterator know there will be no more events ...
		<-rm.waitForShutdownChan // and then wait for the resource monitor to tell us it's done.
	} else {
		return &emptypb.Empty{}, errors.New("Already waiting for shutdown")
	}
	logging.V(6).Infof("SignalAndWaitForShutdown completed")
	return &emptypb.Empty{}, nil
}

func (rm *resmon) RegisterResourceHook(ctx context.Context, req *pulumirpc.RegisterResourceHookRequest) (
	*emptypb.Empty, error,
) {
	panic("not implemented")
}

// inheritFromParent returns a new goal that inherits from the given parent goal.
// Currently only inherits DeletedWith, Protect, and RetainOnDelete from parent.
func inheritFromParent(child resource.Goal, parent resource.Goal) *resource.Goal {
	goal := child
	if goal.DeletedWith == "" {
		goal.DeletedWith = parent.DeletedWith
	}
	if goal.Protect == nil {
		goal.Protect = parent.Protect
	}
	if goal.RetainOnDelete == nil {
		goal.RetainOnDelete = parent.RetainOnDelete
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
		return "", errors.New("source positions must include absolute paths")
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
func transformAliasForNodeJSCompat(alias *pulumirpc.Alias) *pulumirpc.Alias {
	switch a := alias.Alias.(type) {
	case *pulumirpc.Alias_Spec_:
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
		spec := &pulumirpc.Alias_Spec{
			Name:    a.Spec.Name,
			Type:    a.Spec.Type,
			Stack:   a.Spec.Stack,
			Project: a.Spec.Project,
		}

		switch p := a.Spec.Parent.(type) {
		case *pulumirpc.Alias_Spec_ParentUrn:
			if p.ParentUrn == "" {
				spec.Parent = &pulumirpc.Alias_Spec_NoParent{NoParent: true}
			} else {
				spec.Parent = p
			}
		case *pulumirpc.Alias_Spec_NoParent:
			if p.NoParent {
				spec.Parent = nil
			} else {
				spec.Parent = p
			}
		default:
			spec.Parent = &pulumirpc.Alias_Spec_NoParent{NoParent: true}
		}

		return &pulumirpc.Alias{
			Alias: &pulumirpc.Alias_Spec_{
				Spec: spec,
			},
		}
	}

	return alias
}

func (rm *resmon) resolveProvider(
	provider string, providers map[string]string, parent resource.URN, pkg tokens.Package,
) string {
	if provider != "" {
		return provider
	}
	if prov, ok := providers[string(pkg)]; ok {
		return prov
	}
	if parent != "" {
		rm.componentProvidersLock.Lock()
		defer rm.componentProvidersLock.Unlock()
		if parentsProvider, ok := rm.componentProviders[parent]; ok {
			if prov, ok := parentsProvider[string(pkg)]; ok {
				return prov
			}
		}
	}
	return ""
}

// Turn the GRPC status into a message, which can later be logged.  Currently we only support a subset
// of the possible details types, which can be expanded later.  If the details type is not recognized, we
// still return the message, but will leave out the details.  This will allow us to be forward compatible
// when new details types are added.
func errorToMessage(err error, inputs resource.PropertyMap) string {
	switch e := err.(type) {
	case *rpcerror.Error:
		message := e.Message()
		if e.Cause() != nil {
			message = fmt.Sprintf("%v: %v", message, e.Cause().Message())
		}
		if len(e.InputPropertiesErrors()) > 0 {
			props := resource.NewObjectProperty(inputs)
			for _, err := range e.InputPropertiesErrors() {
				propertyPath, e := resource.ParsePropertyPath(err.PropertyPath)
				if e == nil {
					value, ok := propertyPath.Get(props)
					if ok {
						message = fmt.Sprintf("%v\n\t\t- property %v with value '%v' has a problem: %v",
							message, err.PropertyPath, value, err.Reason)
						continue
					}
				}
				message = fmt.Sprintf("%v\n\t\t- property %v has a problem: %v",
					message, err.PropertyPath, err.Reason)
			}
		}
		return message
	default:
		return err.Error()
	}
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	// Communicate the type, name, and object information to the iterator that is awaiting us.
	name := req.GetName()
	custom := req.GetCustom()
	remote := req.GetRemote()
	parent, err := resource.ParseOptionalURN(req.GetParent())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent URN: %s", err))
	}
	id := resource.ID(req.GetImportId())
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

	label := fmt.Sprintf("ResourceMonitor.RegisterResource(%s,%s)", t, name)

	// We need to build the full alias spec list here, so we can pass it to transforms.
	aliases := []*pulumirpc.Alias{}
	for _, aliasURN := range req.GetAliasURNs() {
		aliases = append(aliases, &pulumirpc.Alias{Alias: &pulumirpc.Alias_Urn{Urn: aliasURN}})
	}

	// We assume aliases are properly specified. However, if a request hasn't explicitly
	// indicated that it is using properly specified aliases and the request is coming
	// from Node.js, transform the aliases from the incorrect Node.js values to properly
	// specified values, to maintain backward compatibility for users of older Node.js
	// SDKs that aren't sending properly specified aliases.
	transformAliases := !req.GetAliasSpecs() && requestFromNodeJS(ctx)

	for _, aliasObject := range req.GetAliases() {
		if transformAliases {
			aliasObject = transformAliasForNodeJSCompat(aliasObject)
		}
		aliases = append(aliases, aliasObject)
	}

	var deleteBeforeReplace *bool
	// Technically DeleteBeforeReplaceDefined should be used to decided if DeleteBeforeReplace should be looked at or
	// not. However the Go sdk doesn't set Defined so we have a fallback here of respecting this field if either Defined
	// is set or DeleteBeforeReplace is true.
	if req.GetDeleteBeforeReplaceDefined() || req.GetDeleteBeforeReplace() {
		deleteBeforeReplace = &req.DeleteBeforeReplace
	}

	props, err := plugin.UnmarshalProperties(
		req.GetObject(), plugin.MarshalOptions{
			Label:                 label,
			KeepUnknowns:          true,
			ComputeAssetHashes:    true,
			KeepSecrets:           true,
			KeepResources:         true,
			KeepOutputValues:      true,
			UpgradeToOutputValues: true,
			WorkingDirectory:      rm.workingDirectory,
		})
	if err != nil {
		return nil, err
	}

	// Before we pass the props to the transform function we need to ensure that they correctly carry any dependency
	// information.
	dependencies := mapset.NewSet[resource.URN]()
	for _, dependingURN := range req.GetDependencies() {
		urn, err := resource.ParseURN(dependingURN)
		if err != nil {
			return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency URN: %s", err))
		}
		dependencies.Add(urn)
	}

	propertyDependencies := make(map[resource.PropertyKey]mapset.Set[resource.URN])
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
			deps := mapset.NewSet[resource.URN]()
			for _, d := range pd.Urns {
				urn, err := resource.ParseURN(d)
				if err != nil {
					return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency on property %s URN: %s", pk, err))
				}
				deps.Add(urn)
			}
			propertyDependencies[resource.PropertyKey(pk)] = deps
		}
	}

	// If we're running any transforms we need to update all the property values to Outputs to track dependencies.
	if len(req.Transforms) > 0 {
		props = upgradeOutputValues(props, propertyDependencies)
	}

	provider := req.GetProvider()
	if custom || remote {
		provider = rm.resolveProvider(req.GetProvider(), req.GetProviders(), parent, t.Package())
	}

	opts := &pulumirpc.TransformResourceOptions{
		DependsOn:               req.GetDependencies(),
		Protect:                 req.Protect,
		IgnoreChanges:           req.GetIgnoreChanges(),
		ReplaceOnChanges:        req.GetReplaceOnChanges(),
		Version:                 req.GetVersion(),
		Aliases:                 aliases,
		Provider:                provider,
		Providers:               req.GetProviders(),
		CustomTimeouts:          req.GetCustomTimeouts(),
		PluginDownloadUrl:       req.GetPluginDownloadURL(),
		RetainOnDelete:          req.RetainOnDelete,
		DeletedWith:             req.GetDeletedWith(),
		DeleteBeforeReplace:     deleteBeforeReplace,
		AdditionalSecretOutputs: req.GetAdditionalSecretOutputs(),
		PluginChecksums:         req.GetPluginChecksums(),
	}

	// This might be a resource registation for a resource that another process requested to be constructed.
	// If so we'll have saved the pending transforms for this and we should use those rather than what is on
	// the request. ourTransforms is the list of transforms declared on _this_ resource, we save it later to
	// the resourceTransforms map. transforms is a collected list of _all_ transforms that need to run on this
	// resource, including those from parents and the stack.
	var ourTransforms []TransformFunction
	var transforms []TransformFunction
	pendingKey := fmt.Sprintf("%s::%s::%s", parent, t, name)
	err = func() error {
		rm.pendingTransformsLock.Lock()
		defer rm.pendingTransformsLock.Unlock()

		if pending, ok := rm.pendingTransforms[pendingKey]; ok {
			delete(rm.pendingTransforms, pendingKey) // Remove the pending transforms, we don't need them again.
			ourTransforms = pending
		} else {
			ourTransforms, err = slice.MapError(req.Transforms, rm.wrapTransformCallback)
			if err != nil {
				return err
			}
			// We only need to save this for remote calls
			if remote && len(ourTransforms) > 0 {
				// Make a copy of the slice here, otherwise later appends will modify what we save here.
				rm.pendingTransforms[pendingKey] = ourTransforms
			}
		}
		// Copy our transforms into the transforms slice, so we can run them later.
		transforms = append(transforms, ourTransforms...)
		return nil
	}()
	if err != nil {
		return nil, err
	}
	// Lookup our parents transformations and add those to the list of transforms to run.
	err = func() error {
		// Function exists to scope the lock
		rm.resourceTransformsLock.Lock()
		defer rm.resourceTransformsLock.Unlock()
		rm.parentsLock.Lock()
		defer rm.parentsLock.Unlock()

		current := parent
		for current != "" {
			if parentTransforms, ok := rm.resourceTransforms[current]; ok {
				transforms = append(transforms, parentTransforms...)
			}
			current = rm.parents[current]
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	// Then lock the stack transformations and collect all of those
	func() {
		// Function exists to scope the lock
		rm.stackTransformsLock.Lock()
		defer rm.stackTransformsLock.Unlock()

		transforms = append(transforms, rm.stackTransforms...)
	}()

	// Before we calculate anything else run the transformations. First run the transforms for this resource,
	// then it's parents, then the stack
	for _, transform := range transforms {
		newProps, newOpts, err := transform(ctx, name, string(t), custom, parent, props, opts)
		if err != nil {
			return nil, err
		}
		props = newProps
		opts = newOpts
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
				if opts.Providers == nil {
					opts.Providers = map[string]string{}
				}
				if _, ok := opts.Providers[k]; !ok {
					opts.Providers[k] = v
				}
			}
		}
	}()

	var providerRef providers.Reference
	var providerRefs map[string]string

	if custom && !providers.IsProviderType(t) || remote {
		providerReq, err := parseProviderRequest(
			t.Package(), opts.GetVersion(),
			opts.GetPluginDownloadUrl(), opts.GetPluginChecksums(), nil)
		if err != nil {
			return nil, err
		}

		packageRef := req.GetPackageRef()
		if packageRef != "" {
			var has bool
			providerReq, has = rm.packageRefMap[packageRef]
			if !has {
				return nil, fmt.Errorf("unknown provider package '%v'", packageRef)
			}
		}

		providerRef, err = rm.getProviderReference(rm.defaultProviders, providerReq, opts.GetProvider())
		if err != nil {
			return nil, err
		}

		providerRefs = make(map[string]string, len(opts.GetProviders()))
		for name, provider := range opts.GetProviders() {
			ref, err := rm.getProviderReference(rm.defaultProviders, providerReq, provider)
			if err != nil {
				return nil, err
			}
			providerRefs[name] = ref.String()
		}
	}

	parsedAliases := []resource.Alias{}
	for _, aliasObject := range opts.Aliases {
		aliasSpec := aliasObject.GetSpec()
		var alias resource.Alias
		if aliasSpec != nil {
			alias = resource.Alias{
				Name:    aliasSpec.Name,
				Type:    aliasSpec.Type,
				Stack:   aliasSpec.Stack,
				Project: aliasSpec.Project,
			}
			switch parent := aliasSpec.GetParent().(type) {
			case *pulumirpc.Alias_Spec_ParentUrn:
				// Technically an SDK shouldn't set `parent` at all to specify the default parent, but both NodeJS and
				// Python have buggy SDKs that set parent to an empty URN to specify the default parent. We handle this
				// case here to maintain backward compatibility with older SDKs but it would be good to fix this to be
				// strict in V4.
				parentURN, err := resource.ParseOptionalURN(parent.ParentUrn)
				if err != nil {
					return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid parent alias URN: %s", err))
				}
				alias.Parent = parentURN
			case *pulumirpc.Alias_Spec_NoParent:
				alias.NoParent = parent.NoParent
			}
		} else {
			urn, err := resource.ParseURN(aliasObject.GetUrn())
			if err != nil {
				return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid alias URN: %s", err))
			}
			alias = resource.Alias{URN: urn}
		}
		parsedAliases = append(parsedAliases, alias)
	}

	// Reparse the dependency information from any transformation results
	if len(req.Transforms) > 0 {
		dependencies = mapset.NewSet[resource.URN]()
		for _, dependingURN := range opts.DependsOn {
			urn, err := resource.ParseURN(dependingURN)
			if err != nil {
				return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid dependency URN: %s", err))
			}
			dependencies.Add(urn)
		}
		// Now we've run the transforms we can rebuild the property dependency maps. If we have output values we can add the
		// dependencies from them to the dependencies map we send to the provider and save to state.
		propertyDependencies = make(map[resource.PropertyKey]mapset.Set[resource.URN])
		for key, output := range props {
			deps := mapset.NewSet[resource.URN]()
			addOutputDependencies(deps, output)
			propertyDependencies[key] = deps

			// Also add these to the overall dependencies
			dependencies = dependencies.Union(deps)
		}
	} else {
		// If we ran transforms we would have merged all the dependencies togther already, but if we didn't we want to
		// ensure any output values add their dependencies to the dependencies map we send to the provider.
		for key, output := range props {
			if propertyDependencies[key] == nil {
				propertyDependencies[key] = mapset.NewSet[resource.URN]()
			}
			addOutputDependencies(propertyDependencies[key], output)
		}
	}

	rawDependencies := dependencies.ToSlice()
	rawPropertyDependencies := make(map[resource.PropertyKey][]resource.URN)
	for key, deps := range propertyDependencies {
		rawPropertyDependencies[key] = deps.ToSlice()
	}

	if providers.IsProviderType(t) {
		if opts.GetVersion() != "" {
			version, err := semver.Parse(opts.GetVersion())
			if err != nil {
				return nil, fmt.Errorf("%s: passed invalid version: %w", label, err)
			}
			providers.SetProviderVersion(props, &version)
		}
		if opts.GetPluginDownloadUrl() != "" {
			providers.SetProviderURL(props, opts.GetPluginDownloadUrl())
		}

		if req.GetPackageRef() != "" {
			// If the provider resource has a package ref then we need to set all it's input fields as in
			// newRegisterDefaultProviderEvent.
			packageRef := req.GetPackageRef()
			providerReq, has := rm.packageRefMap[packageRef]
			if !has {
				return nil, fmt.Errorf("unknown provider package '%v'", packageRef)
			}

			if providerReq.Version() != nil {
				providers.SetProviderVersion(props, providerReq.Version())
			}
			if providerReq.PluginDownloadURL() != "" {
				providers.SetProviderURL(props, providerReq.PluginDownloadURL())
			}
			if providerReq.PluginChecksums() != nil {
				providers.SetProviderChecksums(props, providerReq.PluginChecksums())
			}
			if providerReq.Parameterization() != nil {
				providers.SetProviderName(props, providerReq.Name())
				providers.SetProviderParameterization(props, providerReq.Parameterization())
			}
		}

		// Make sure that an explicit provider which doesn't specify its plugin gets the
		// same plugin as the default provider for the package.
		defaultProvider, ok := rm.defaultProviders.defaultProviderInfo[providers.GetProviderPackage(t)]
		if ok && opts.GetVersion() == "" && opts.GetPluginDownloadUrl() == "" {
			if defaultProvider.Version != nil {
				providers.SetProviderVersion(props, defaultProvider.Version)
			}
			if defaultProvider.PluginDownloadURL != "" {
				providers.SetProviderURL(props, defaultProvider.PluginDownloadURL)
			}
		}
	}

	protect := opts.Protect
	ignoreChanges := opts.IgnoreChanges
	replaceOnChanges := opts.ReplaceOnChanges
	retainOnDelete := opts.RetainOnDelete
	deletedWith, err := resource.ParseOptionalURN(opts.GetDeletedWith())
	if err != nil {
		return nil, rpcerror.New(codes.InvalidArgument, fmt.Sprintf("invalid DeletedWith URN: %s", err))
	}
	customTimeouts := opts.CustomTimeouts

	additionalSecretOutputs := opts.GetAdditionalSecretOutputs()

	// At this point we're going to forward these properties to the rest of the engine and potentially to providers. As
	// we add features to the code above (most notably transforms) we could end up with more instances of `OutputValue`
	// than the rest of the system historically expects. To minimize the disruption we downgrade `OutputValue`s with no
	// dependencies down to `Computed` and `Secret` or their plain values. We only do this for non-remote resources.
	// Remote resources already deal with `OutputValue`s and even though it would be more consistent to downgrade them
	// here it would be a break change.
	if !remote {
		props = downgradeOutputValues(props)
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource received: t=%v, name=%v, custom=%v, #props=%v, parent=%v, protect=%v, "+
			"provider=%v, deps=%v, deleteBeforeReplace=%v, ignoreChanges=%v, aliases=%v, customTimeouts=%v, "+
			"providers=%v, replaceOnChanges=%v, retainOnDelete=%v, deletedWith=%v",
		t, name, custom, len(props), parent, protect, providerRef, rawDependencies, opts.DeleteBeforeReplace, ignoreChanges,
		parsedAliases, customTimeouts, providerRefs, replaceOnChanges, retainOnDelete, deletedWith)

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
			Dependencies:            rawDependencies,
			Protect:                 protect,
			PropertyDependencies:    rawPropertyDependencies,
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
		options.DeleteBeforeReplace = opts.DeleteBeforeReplace

		// Run construct in a go routine so we can react to a cancellation on rm.cancel.
		var constructResult plugin.ConstructResult
		constructDone := make(chan error)
		go func() {
			constructResult, err = provider.Construct(ctx, plugin.ConstructRequest{
				Info:    rm.constructInfo,
				Type:    t,
				Name:    name,
				Parent:  parent,
				Inputs:  props,
				Options: options,
			})
			if err != nil {
				var rpcError error
				rpcError, ok := rpcerror.FromError(err)
				if !ok {
					rpcError = err
				}
				message := errorToMessage(rpcError, props)
				rm.diagnostics.Errorf(diag.GetResourceInvalidError(constructResult.URN), t, name, message)

				rm.abortChan <- true
				constructDone <- rpcerror.New(codes.Unknown, "resource monitor shut down")
			}
			close(constructDone)
		}()

		select {
		case err := <-constructDone:
			if err != nil {
				logging.V(5).Infof("ResourceMonitor.RegisterResource construct returned an error, name=%s err=%s",
					name, err)
				return nil, err
			}
		case <-rm.cancel:
			logging.V(5).Infof("ResourceMonitor.RegisterResource construct canceled, name=%s", name)
			return nil, rpcerror.New(codes.Unavailable,
				"resource monitor shut down while waiting for construct to complete")
		}

		result = &RegisterResult{State: &resource.State{URN: constructResult.URN, Outputs: constructResult.Outputs}}

		// The provider may have returned OutputValues in "Outputs", we need to downgrade them to Computed or
		// Secret but also add them to the outputDeps map.
		if constructResult.OutputDependencies == nil {
			constructResult.OutputDependencies = map[resource.PropertyKey][]resource.URN{}
		}
		for k, v := range result.State.Outputs {
			constructResult.OutputDependencies[k] = extendOutputDependencies(constructResult.OutputDependencies[k], v)
		}

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

		goal := resource.NewGoal(t, name, custom, props, parent, protect, rawDependencies,
			providerRef.String(), nil, rawPropertyDependencies, opts.DeleteBeforeReplace, ignoreChanges,
			additionalSecretKeys, parsedAliases, id, &timeouts, replaceOnChanges, retainOnDelete, deletedWith,
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
		if result != nil && result.Result != ResultStateSuccess && !req.GetSupportsResultReporting() {
			return nil, rpcerror.New(codes.Internal, "resource registration failed")
		}
		if result != nil && result.State != nil && result.State.URN != "" {
			rm.resGoalsLock.Lock()
			rm.resGoals[result.State.URN] = *goal
			rm.resGoalsLock.Unlock()
		}
	}

	if result != nil && result.State != nil && result.State.URN != "" {
		// We've got a safe URN now, save the parent and transformations
		func() {
			rm.parentsLock.Lock()
			defer rm.parentsLock.Unlock()
			rm.parents[result.State.URN] = parent
		}()
		func() {
			rm.resourceTransformsLock.Lock()
			defer rm.resourceTransformsLock.Unlock()
			rm.resourceTransforms[result.State.URN] = ourTransforms
		}()
		if !custom {
			func() {
				rm.componentProvidersLock.Lock()
				defer rm.componentProvidersLock.Unlock()
				rm.componentProviders[result.State.URN] = opts.GetProviders()
			}()
		}
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
			return retainOnDelete != nil
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
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      req.GetAcceptSecrets(),
		KeepResources:    req.GetAcceptResources(),
		WorkingDirectory: rm.workingDirectory,
	})
	if err != nil {
		return nil, err
	}

	// Assert that we never leak the unconfigured provider ID to the language host.
	contract.Assertf(
		!providers.IsProviderType(result.State.Type) || result.State.ID != providers.UnconfiguredID,
		"provider resource %s has unconfigured ID", result.State.URN)

	reason := pulumirpc.Result_SUCCESS
	switch result.Result { //nolint:exhaustive // golangci-lint v2 upgrade
	case ResultStateSkipped:
		reason = pulumirpc.Result_SKIP
	case ResultStateFailed:
		reason = pulumirpc.Result_FAIL
	}
	return &pulumirpc.RegisterResourceResponse{
		Urn:                  string(result.State.URN),
		Id:                   string(result.State.ID),
		Object:               obj,
		PropertyDependencies: outputDeps,
		Result:               reason,
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
) (*emptypb.Empty, error) {
	// Obtain and validate the message's inputs (a URN plus the output property map).
	urn, err := resource.ParseURN(req.Urn)
	if err != nil {
		return nil, fmt.Errorf("invalid resource URN: %w", err)
	}

	label := fmt.Sprintf("ResourceMonitor.RegisterResourceOutputs(%s)", urn)
	outs, err := plugin.UnmarshalProperties(
		req.GetOutputs(), plugin.MarshalOptions{
			Label:              label,
			KeepUnknowns:       true,
			ComputeAssetHashes: true,
			KeepSecrets:        true,
			KeepResources:      true,
			WorkingDirectory:   rm.workingDirectory,
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
	return &emptypb.Empty{}, nil
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
	name                    string
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
func (g *readResourceEvent) Name() string                     { return g.name }
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

// downgradeOutputValues recursively replaces all Output values with `Computed`, `Secret`, or their plain
// value. This loses all dependency information.
func downgradeOutputValues(v resource.PropertyMap) resource.PropertyMap {
	var downgradeOutputPropertyValue func(v resource.PropertyValue) resource.PropertyValue

	downgradeOutputPropertyValue = func(v resource.PropertyValue) resource.PropertyValue {
		if v.IsOutput() {
			output := v.OutputValue()
			var result resource.PropertyValue
			if output.Known {
				result = downgradeOutputPropertyValue(output.Element)
			} else {
				result = resource.MakeComputed(resource.NewStringProperty(""))
			}
			if output.Secret {
				result = resource.MakeSecret(result)
			}
			return result
		}
		if v.IsObject() {
			return resource.NewObjectProperty(downgradeOutputValues(v.ObjectValue()))
		}
		if v.IsArray() {
			result := make([]resource.PropertyValue, len(v.ArrayValue()))
			for i, elem := range v.ArrayValue() {
				result[i] = downgradeOutputPropertyValue(elem)
			}
			return resource.NewArrayProperty(result)
		}
		if v.IsSecret() {
			return resource.MakeSecret(downgradeOutputPropertyValue(v.SecretValue().Element))
		}
		if v.IsResourceReference() {
			ref := v.ResourceReferenceValue()
			return resource.NewResourceReferenceProperty(
				resource.ResourceReference{
					URN:            ref.URN,
					ID:             downgradeOutputPropertyValue(ref.ID),
					PackageVersion: ref.PackageVersion,
				})
		}
		return v
	}

	result := make(resource.PropertyMap)
	for k, pv := range v {
		result[k] = downgradeOutputPropertyValue(pv)
	}
	return result
}

func upgradeOutputValues(
	v resource.PropertyMap, propertyDependencies map[resource.PropertyKey]mapset.Set[resource.URN],
) resource.PropertyMap {
	// We assume that by the time this is being called we've upgraded all Secret/Computed values to outputs. We just
	// need to add the dependency information from propertyDependencies.

	result := make(resource.PropertyMap)
	for k, pv := range v {
		if deps, has := propertyDependencies[k]; has {
			currentDeps := mapset.NewSet[resource.URN]()
			addOutputDependencies(currentDeps, pv)
			if currentDeps.IsSuperset(deps) {
				// already has the deps, just copy across
				result[k] = pv
			} else {
				var output resource.Output
				if pv.IsOutput() {
					output = pv.OutputValue()
				} else {
					output = resource.Output{
						Element: pv,
						Known:   true,
					}
				}

				// Merge all the dependencies from the propertyDependencies map with any current dependencies on this
				// output value.
				currentDeps.Clear()
				currentDeps.Append(output.Dependencies...)
				currentDeps = currentDeps.Union(deps)

				output.Dependencies = currentDeps.ToSlice()
				result[k] = resource.NewOutputProperty(output)
			}
		} else {
			// no deps just copy across
			result[k] = pv
		}
	}
	return result
}

func extendOutputDependencies(deps []resource.URN, v resource.PropertyValue) []resource.URN {
	set := mapset.NewSet(deps...)
	addOutputDependencies(set, v)
	return set.ToSlice()
}

func addOutputDependencies(deps mapset.Set[resource.URN], v resource.PropertyValue) {
	if v.IsOutput() {
		output := v.OutputValue()
		if output.Known {
			addOutputDependencies(deps, output.Element)
		}
		deps.Append(output.Dependencies...)
	}
	if v.IsResourceReference() {
		ref := v.ResourceReferenceValue()
		addOutputDependencies(deps, ref.ID)
	}
	if v.IsObject() {
		for _, elem := range v.ObjectValue() {
			addOutputDependencies(deps, elem)
		}
	}
	if v.IsArray() {
		for _, elem := range v.ArrayValue() {
			addOutputDependencies(deps, elem)
		}
	}
	if v.IsSecret() {
		addOutputDependencies(deps, v.SecretValue().Element)
	}
}
