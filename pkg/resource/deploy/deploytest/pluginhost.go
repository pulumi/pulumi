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

package deploytest

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

var UseGrpcProvidersByDefault = false

type LoadProviderFunc func() (plugin.Provider, error)
type LoadProviderWithHostFunc func(host plugin.Host) (plugin.Provider, error)

type ProviderOption func(p *ProviderLoader)

func WithoutGrpc(p *ProviderLoader) {
	p.useGRPC = false
}

func WithGrpc(p *ProviderLoader) {
	p.useGRPC = true
}

type ProviderLoader struct {
	pkg          tokens.Package
	version      semver.Version
	load         LoadProviderFunc
	loadWithHost LoadProviderWithHostFunc
	useGRPC      bool
}

func NewProviderLoader(pkg tokens.Package, version semver.Version, load LoadProviderFunc,
	opts ...ProviderOption) *ProviderLoader {

	p := &ProviderLoader{
		pkg:     pkg,
		version: version,
		load:    load,
		useGRPC: UseGrpcProvidersByDefault,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func NewProviderLoaderWithHost(pkg tokens.Package, version semver.Version,
	load LoadProviderWithHostFunc, opts ...ProviderOption) *ProviderLoader {

	p := &ProviderLoader{
		pkg:          pkg,
		version:      version,
		loadWithHost: load,
		useGRPC:      UseGrpcProvidersByDefault,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

type nopCloserT int

func (nopCloserT) Close() error { return nil }

var nopCloser io.Closer = nopCloserT(0)

type grpcWrapper struct {
	stop chan bool
}

func (w *grpcWrapper) Close() error {
	go func() { w.stop <- true }()
	return nil
}

func wrapProviderWithGrpc(provider plugin.Provider) (plugin.Provider, io.Closer, error) {
	wrapper := &grpcWrapper{stop: make(chan bool)}
	port, _, err := rpcutil.Serve(0, wrapper.stop, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, plugin.NewProviderServer(provider))
			return nil
		},
	}, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not start resource provider service: %w", err)
	}
	conn, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%v", port),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		contract.IgnoreClose(wrapper)
		return nil, nil, fmt.Errorf("could not connect to resource provider service: %v", err)
	}
	wrapped := plugin.NewProviderWithClient(nil, provider.Pkg(), pulumirpc.NewResourceProviderClient(conn), false)
	return wrapped, wrapper, nil
}

type hostEngine struct {
	sink       diag.Sink
	statusSink diag.Sink

	address string
	stop    chan bool
}

func (e *hostEngine) Log(_ context.Context, req *pulumirpc.LogRequest) (*pbempty.Empty, error) {
	var sev diag.Severity
	switch req.Severity {
	case pulumirpc.LogSeverity_DEBUG:
		sev = diag.Debug
	case pulumirpc.LogSeverity_INFO:
		sev = diag.Info
	case pulumirpc.LogSeverity_WARNING:
		sev = diag.Warning
	case pulumirpc.LogSeverity_ERROR:
		sev = diag.Error
	default:
		return nil, errors.Errorf("Unrecognized logging severity: %v", req.Severity)
	}

	if req.Ephemeral {
		e.statusSink.Logf(sev, diag.StreamMessage(resource.URN(req.Urn), req.Message, req.StreamId))
	} else {
		e.sink.Logf(sev, diag.StreamMessage(resource.URN(req.Urn), req.Message, req.StreamId))
	}
	return &pbempty.Empty{}, nil
}
func (e *hostEngine) GetRootResource(_ context.Context,
	req *pulumirpc.GetRootResourceRequest) (*pulumirpc.GetRootResourceResponse, error) {
	return nil, errors.New("unsupported")
}
func (e *hostEngine) SetRootResource(_ context.Context,
	req *pulumirpc.SetRootResourceRequest) (*pulumirpc.SetRootResourceResponse, error) {
	return nil, errors.New("unsupported")
}

type pluginHost struct {
	providerLoaders []*ProviderLoader
	languageRuntime plugin.LanguageRuntime
	sink            diag.Sink
	statusSink      diag.Sink

	engine *hostEngine

	providers map[plugin.Provider]io.Closer
	closed    bool
	m         sync.Mutex
}

func NewPluginHost(sink, statusSink diag.Sink, languageRuntime plugin.LanguageRuntime,
	providerLoaders ...*ProviderLoader) plugin.Host {

	engine := &hostEngine{
		sink:       sink,
		statusSink: statusSink,
		stop:       make(chan bool),
	}
	port, _, err := rpcutil.Serve(0, engine.stop, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterEngineServer(srv, engine)
			return nil
		},
	}, nil)
	if err != nil {
		panic(fmt.Errorf("could not start engine service: %v", err))
	}
	engine.address = fmt.Sprintf("127.0.0.1:%v", port)

	return &pluginHost{
		providerLoaders: providerLoaders,
		languageRuntime: languageRuntime,
		sink:            sink,
		statusSink:      statusSink,
		engine:          engine,
		providers:       map[plugin.Provider]io.Closer{},
	}
}

func (host *pluginHost) isClosed() bool {
	host.m.Lock()
	defer host.m.Unlock()
	return host.closed
}

func (host *pluginHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	var best *ProviderLoader
	for _, l := range host.providerLoaders {
		if l.pkg != pkg {
			continue
		}

		if version != nil {
			if l.version.EQ(*version) {
				best = l
				break
			}
		} else if best == nil || l.version.GT(best.version) {
			best = l
		}
	}
	if best == nil {
		return nil, nil
	}

	load := best.load
	if load == nil {
		load = func() (plugin.Provider, error) {
			return best.loadWithHost(host)
		}
	}

	prov, err := load()
	if err != nil {
		return nil, err
	}

	closer := nopCloser
	if best.useGRPC {
		prov, closer, err = wrapProviderWithGrpc(prov)
		if err != nil {
			return nil, err
		}
	}

	host.m.Lock()
	defer host.m.Unlock()

	host.providers[prov] = closer
	return prov, nil
}

func (host *pluginHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return host.languageRuntime, nil
}

func (host *pluginHost) SignalCancellation() error {
	host.m.Lock()
	defer host.m.Unlock()

	var err error
	for prov := range host.providers {
		if pErr := prov.SignalCancellation(); pErr != nil {
			err = pErr
		}
	}
	return err
}
func (host *pluginHost) Close() error {
	host.m.Lock()
	defer host.m.Unlock()

	var err error
	for _, closer := range host.providers {
		if pErr := closer.Close(); pErr != nil {
			err = pErr
		}
	}

	go func() { host.engine.stop <- true }()
	host.closed = true
	return err
}
func (host *pluginHost) ServerAddr() string {
	return host.engine.address
}
func (host *pluginHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if !host.isClosed() {
		host.sink.Logf(sev, diag.StreamMessage(urn, msg, streamID))
	}
}
func (host *pluginHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if !host.isClosed() {
		host.statusSink.Logf(sev, diag.StreamMessage(urn, msg, streamID))
	}
}
func (host *pluginHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return nil, errors.New("unsupported")
}
func (host *pluginHost) CloseProvider(provider plugin.Provider) error {
	host.m.Lock()
	defer host.m.Unlock()

	delete(host.providers, provider)
	return nil
}
func (host *pluginHost) ListPlugins() []workspace.PluginInfo {
	return nil
}
func (host *pluginHost) EnsurePlugins(plugins []workspace.PluginInfo, kinds plugin.Flags) error {
	return nil
}
func (host *pluginHost) GetRequiredPlugins(info plugin.ProgInfo,
	kinds plugin.Flags) ([]workspace.PluginInfo, error) {
	return nil, nil
}

func (host *pluginHost) PolicyAnalyzer(name tokens.QName, path string,
	opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
	return nil, errors.New("unsupported")
}

func (host *pluginHost) ListAnalyzers() []plugin.Analyzer {
	return nil
}
