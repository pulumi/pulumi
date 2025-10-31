package deploytest

import deploytest "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"

type LoadPluginFunc = deploytest.LoadPluginFunc

type LoadPluginWithHostFunc = deploytest.LoadPluginWithHostFunc

type LoadProviderFunc = deploytest.LoadProviderFunc

type LoadProviderWithHostFunc = deploytest.LoadProviderWithHostFunc

type LoadAnalyzerFunc = deploytest.LoadAnalyzerFunc

type LoadAnalyzerWithHostFunc = deploytest.LoadAnalyzerWithHostFunc

type PluginOption = deploytest.PluginOption

type PluginLoader = deploytest.PluginLoader

type ProviderOption = deploytest.ProviderOption

type ProviderLoader = deploytest.ProviderLoader

type PluginHostFactory = deploytest.PluginHostFactory

var ErrHostIsClosed = deploytest.ErrHostIsClosed

var UseGrpcPluginsByDefault = deploytest.UseGrpcPluginsByDefault

func WithoutGrpc(p *PluginLoader) {
	deploytest.WithoutGrpc(p)
}

func WithGrpc(p *PluginLoader) {
	deploytest.WithGrpc(p)
}

func NewProviderLoader(pkg tokens.Package, version semver.Version, load LoadProviderFunc, opts ...ProviderOption) *ProviderLoader {
	return deploytest.NewProviderLoader(pkg, version, load, opts...)
}

func NewProviderLoaderWithHost(pkg tokens.Package, version semver.Version, load LoadProviderWithHostFunc, opts ...ProviderOption) *ProviderLoader {
	return deploytest.NewProviderLoaderWithHost(pkg, version, load, opts...)
}

func NewAnalyzerLoader(name string, load LoadAnalyzerFunc, opts ...PluginOption) *PluginLoader {
	return deploytest.NewAnalyzerLoader(name, load, opts...)
}

func NewAnalyzerLoaderWithHost(name string, load LoadAnalyzerWithHostFunc, opts ...PluginOption) *PluginLoader {
	return deploytest.NewAnalyzerLoaderWithHost(name, load, opts...)
}

// NewPluginHostF returns a factory that produces a plugin host for an operation.
func NewPluginHostF(sink, statusSink diag.Sink, languageRuntimeF LanguageRuntimeFactory, pluginLoaders ...*ProviderLoader) PluginHostFactory {
	return deploytest.NewPluginHostF(sink, statusSink, languageRuntimeF, pluginLoaders...)
}

func NewPluginHost(sink, statusSink diag.Sink, languageRuntime plugin.LanguageRuntime, pluginLoaders ...*ProviderLoader) plugin.Host {
	return deploytest.NewPluginHost(sink, statusSink, languageRuntime, pluginLoaders...)
}

