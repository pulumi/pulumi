package pulumi

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/blang/semver"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
	"google.golang.org/grpc"
)

type cliHost struct {
	log    diag.Sink
	status diag.Sink

	main            func(plugin.Host) error
	engine          pulumirpc.EngineClient
	languageRuntime plugin.LanguageRuntime
	runRequests     <-chan RunInfo
	exit            chan error
}

type hostEngine struct {
	logger       plugin.Logger
	rootResource atomic.Value
}

func (e *hostEngine) Log(ctx context.Context, req *pulumirpc.LogRequest, _ ...grpc.CallOption) (*empty.Empty, error) {
	var severity diag.Severity
	switch req.GetSeverity() {
	case pulumirpc.LogSeverity_DEBUG:
		severity = diag.Debug
	case pulumirpc.LogSeverity_INFO:
		severity = diag.Info
	case pulumirpc.LogSeverity_WARNING:
		severity = diag.Warning
	case pulumirpc.LogSeverity_ERROR:
		severity = diag.Error
	}

	if req.GetEphemeral() {
		e.logger.LogStatus(severity, resource.URN(req.GetUrn()), req.GetMessage(), req.GetStreamId())
	} else {
		e.logger.Log(severity, resource.URN(req.GetUrn()), req.GetMessage(), req.GetStreamId())
	}

	return &empty.Empty{}, nil
}

func (e *hostEngine) GetRootResource(ctx context.Context, in *pulumirpc.GetRootResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.GetRootResourceResponse, error) {

	urn, _ := e.rootResource.Load().(string)
	return &pulumirpc.GetRootResourceResponse{Urn: urn}, nil
}

func (e *hostEngine) SetRootResource(ctx context.Context, in *pulumirpc.SetRootResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.SetRootResourceResponse, error) {

	e.rootResource.Store(in.GetUrn())
	return &pulumirpc.SetRootResourceResponse{}, nil
}

type languageRuntime struct {
	ctx    context.Context
	cancel func()

	logger plugin.Logger

	runRequests chan<- RunInfo
}

func newLanguageRuntime(ctx context.Context, logger plugin.Logger) (plugin.LanguageRuntime, <-chan RunInfo) {
	cancelContext, cancel := context.WithCancel(ctx)
	runRequests := make(chan RunInfo)
	return &languageRuntime{
		ctx:         cancelContext,
		cancel:      cancel,
		logger:      logger,
		runRequests: runRequests,
	}, runRequests
}

func (s *languageRuntime) Close() error {
	s.cancel()
	return nil
}

func (s *languageRuntime) GetRequiredPlugins(info plugin.ProgInfo) ([]workspace.PluginInfo, []workspace.PluginInfo, error) {
	return nil, nil, nil
}

func (s *languageRuntime) Run(info plugin.RunInfo) (string, bool, error) {
	config := map[string]string{}
	for k, v := range info.Config {
		config[k.String()] = v
	}

	done := make(chan error)
	runInfo := RunInfo{
		MonitorAddr: info.MonitorAddress,
		Config:      config,
		Project:     info.Project,
		Stack:       info.Stack,
		Parallel:    info.Parallel,
		done:        done,
	}

	select {
	case <-s.ctx.Done():
		return "", false, s.ctx.Err()
	case s.runRequests <- runInfo:
		// OK
	}

	select {
	case <-s.ctx.Done():
		return "", false, s.ctx.Err()
	case err := <-done:
		if err == nil {
			return "", false, nil
		}
		return err.Error(), false, nil
	}
}

func (s *languageRuntime) StartProvider(name string, version semver.Version) (plugin.Provider, error) {
	info := PackageInfo{
		Name:    name,
		Version: version.String(),
	}
	loader := providerRegistry[info]
	if loader == nil {
		return nil, fmt.Errorf("unknown provider %v@%v", info.Name, info.Version)
	}
	return loader(s.logger)
}

func (s *languageRuntime) GetPluginInfo() (workspace.PluginInfo, error) {
	lv := semver.MustParse("1.2.3")
	return workspace.PluginInfo{
		Name:    "go",
		Kind:    workspace.LanguagePlugin,
		Version: &lv,
	}, nil
}

func (h *cliHost) listPlugins() []workspace.PluginInfo {
	infos := make([]workspace.PluginInfo, 0, len(providerRegistry)+1)
	for info := range providerRegistry {
		var version *semver.Version
		if v, err := semver.ParseTolerant(info.Version); err == nil {
			version = &v
		}

		infos = append(infos, workspace.PluginInfo{
			Name:    info.Name,
			Kind:    workspace.ResourcePlugin,
			Version: version,
		})
	}

	lv := semver.MustParse("1.2.3")
	infos = append(infos, workspace.PluginInfo{
		Name:    "go",
		Kind:    workspace.LanguagePlugin,
		Version: &lv,
	})

	return infos
}

func (h *cliHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	h.log.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

func (h *cliHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	h.status.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

func (h *cliHost) ServerAddr() string {
	return ""
}

func (h *cliHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return nil, fmt.Errorf("unknown analyzer %v", nm)
}

func (h *cliHost) PolicyAnalyzer(name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
	return nil, fmt.Errorf("unknown analyzer %v", name)
}

func (h *cliHost) ListAnalyzers() []plugin.Analyzer {
	return nil
}

func (h *cliHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	plugins := h.listPlugins()

	info, err := workspace.GetPluginInfoFromList(plugins, workspace.ResourcePlugin, string(pkg), version)
	if err != nil || info == nil {
		return nil, err
	}

	infoVersion := ""
	if info.Version != nil {
		infoVersion = info.Version.String()
	}

	loader, ok := providerRegistry[PackageInfo{
		Name:    info.Name,
		Version: infoVersion,
	}]
	if !ok {
		return nil, fmt.Errorf("internal error: could not find provider %v@%v", info.Name, info.Version)
	}

	return loader(h)
}

func (h *cliHost) CloseProvider(provider plugin.Provider) error {
	return provider.Close()
}

func (h *cliHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return h.languageRuntime, nil
}

func (h *cliHost) ListPlugins() []workspace.PluginInfo {
	return h.listPlugins()
}

func (h *cliHost) EnsurePlugins(plugins []workspace.PluginInfo, kinds plugin.Flags) error {
	// TODO(pdg): error out for unknown plugins
	return nil
}

func (h *cliHost) SignalCancellation() error {
	return nil
}

func (h *cliHost) Close() error {
	return nil
}

var host *cliHost

func EnableCLI(main func(plugin.Host) error) plugin.Host {
	host = &cliHost{
		log:    cmdutil.Diag(),
		status: cmdutil.Diag(),
		main:   main,
		exit:   make(chan error),
	}
	host.engine = &hostEngine{logger: host}
	host.languageRuntime, host.runRequests = newLanguageRuntime(context.Background(), host)
	return host
}
