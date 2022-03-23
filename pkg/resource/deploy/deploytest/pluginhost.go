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
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"

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

var UseGrpcPluginsByDefault = false

type LoadPluginFunc func(opts interface{}) (interface{}, error)
type LoadPluginWithHostFunc func(opts interface{}, host plugin.Host) (interface{}, error)

type LoadProviderFunc func() (plugin.Provider, error)
type LoadProviderWithHostFunc func(host plugin.Host) (plugin.Provider, error)

type LoadAnalyzerFunc func(opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error)
type LoadAnalyzerWithHostFunc func(opts *plugin.PolicyAnalyzerOptions, host plugin.Host) (plugin.Analyzer, error)

type PluginOption func(p *PluginLoader)

func WithoutGrpc(p *PluginLoader) {
	p.useGRPC = false
}

func WithGrpc(p *PluginLoader) {
	p.useGRPC = true
}

type PluginLoader struct {
	kind         workspace.PluginKind
	name         string
	version      semver.Version
	load         LoadPluginFunc
	loadWithHost LoadPluginWithHostFunc
	useGRPC      bool
}

type ProviderOption = PluginOption
type ProviderLoader = PluginLoader

func NewProviderLoader(pkg tokens.Package, version semver.Version, load LoadProviderFunc,
	opts ...ProviderOption) *ProviderLoader {

	p := &ProviderLoader{
		kind:    workspace.ResourcePlugin,
		name:    string(pkg),
		version: version,
		load:    func(_ interface{}) (interface{}, error) { return load() },
		useGRPC: UseGrpcPluginsByDefault,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func NewProviderLoaderWithHost(pkg tokens.Package, version semver.Version,
	load LoadProviderWithHostFunc, opts ...ProviderOption) *ProviderLoader {

	p := &ProviderLoader{
		kind:         workspace.ResourcePlugin,
		name:         string(pkg),
		version:      version,
		loadWithHost: func(_ interface{}, host plugin.Host) (interface{}, error) { return load(host) },
		useGRPC:      UseGrpcPluginsByDefault,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func NewAnalyzerLoader(name string, load LoadAnalyzerFunc, opts ...PluginOption) *PluginLoader {
	p := &PluginLoader{
		kind: workspace.AnalyzerPlugin,
		name: name,
		load: func(optsI interface{}) (interface{}, error) {
			opts, _ := optsI.(*plugin.PolicyAnalyzerOptions)
			return load(opts)
		},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func NewAnalyzerLoaderWithHost(name string, load LoadAnalyzerWithHostFunc, opts ...PluginOption) *PluginLoader {
	p := &PluginLoader{
		kind: workspace.AnalyzerPlugin,
		name: name,
		loadWithHost: func(optsI interface{}, host plugin.Host) (interface{}, error) {
			opts, _ := optsI.(*plugin.PolicyAnalyzerOptions)
			return load(opts, host)
		},
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
		return nil, fmt.Errorf("Unrecognized logging severity: %v", req.Severity)
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
	pluginLoaders   []*ProviderLoader
	languageRuntime plugin.LanguageRuntime
	sink            diag.Sink
	statusSink      diag.Sink

	engine *hostEngine

	providers []plugin.Provider
	analyzers []plugin.Analyzer
	plugins   map[interface{}]io.Closer
	closed    bool
	m         sync.Mutex
}

func NewPluginHost(sink, statusSink diag.Sink, languageRuntime plugin.LanguageRuntime,
	pluginLoaders ...*ProviderLoader) plugin.Host {

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
		pluginLoaders:   pluginLoaders,
		languageRuntime: languageRuntime,
		sink:            sink,
		statusSink:      statusSink,
		engine:          engine,
		plugins:         map[interface{}]io.Closer{},
	}
}

func (host *pluginHost) isClosed() bool {
	host.m.Lock()
	defer host.m.Unlock()
	return host.closed
}

func (host *pluginHost) plugin(kind workspace.PluginKind, name string, version *semver.Version,
	opts interface{}) (interface{}, error) {

	var best *PluginLoader
	for _, l := range host.pluginLoaders {
		if l.kind != kind || l.name != name {
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
		load = func(opts interface{}) (interface{}, error) {
			return best.loadWithHost(opts, host)
		}
	}

	plug, err := load(opts)
	if err != nil {
		return nil, err
	}

	closer := nopCloser
	if best.useGRPC {
		plug, closer, err = wrapProviderWithGrpc(plug.(plugin.Provider))
		if err != nil {
			return nil, err
		}
	}

	host.m.Lock()
	defer host.m.Unlock()

	switch kind {
	case workspace.AnalyzerPlugin:
		host.analyzers = append(host.analyzers, plug.(plugin.Analyzer))
	case workspace.ResourcePlugin:
		host.providers = append(host.providers, plug.(plugin.Provider))
	}

	host.plugins[plug] = closer
	return plug, nil
}

func (host *pluginHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	plug, err := host.plugin(workspace.ResourcePlugin, string(pkg), version, nil)
	if err != nil {
		return nil, err
	}
	if plug == nil {
		v := "nil"
		if version != nil {
			v = version.String()
		}
		return nil, fmt.Errorf("Could not find plugin for (%s, %s)", pkg.String(), v)
	}
	return plug.(plugin.Provider), nil
}

func (host *pluginHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return host.languageRuntime, nil
}

func (host *pluginHost) SignalCancellation() error {
	host.m.Lock()
	defer host.m.Unlock()

	var err error
	for _, prov := range host.providers {
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
	for _, closer := range host.plugins {
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
	return host.PolicyAnalyzer(nm, "", nil)
}
func (host *pluginHost) CloseProvider(provider plugin.Provider) error {
	host.m.Lock()
	defer host.m.Unlock()

	delete(host.plugins, provider)
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

	plug, err := host.plugin(workspace.AnalyzerPlugin, string(name), nil, opts)
	if err != nil || plug == nil {
		return nil, err
	}
	return plug.(plugin.Analyzer), nil
}

func (host *pluginHost) ListAnalyzers() []plugin.Analyzer {
	host.m.Lock()
	defer host.m.Unlock()

	return host.analyzers
}
