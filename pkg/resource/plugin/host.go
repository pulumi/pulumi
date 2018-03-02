// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// A Host hosts provider plugins and makes them easily accessible by package name.
type Host interface {
	// ServerAddr returns the address at which the host's RPC interface may be found.
	ServerAddr() string

	// Log logs a global message, including errors and warnings.
	Log(sev diag.Severity, msg string)

	// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for it.  If an analyzer
	// could not be found, or an error occurred while creating it, a non-nil error is returned.
	Analyzer(nm tokens.QName) (Analyzer, error)
	// Provider fetches the provider for a given package, lazily allocating it if necessary.  If a provider for this
	// package could not be found, or an error occurs while creating it, a non-nil error is returned.
	Provider(pkg tokens.Package, version *semver.Version) (Provider, error)
	// LanguageRuntime fetches the language runtime plugin for a given language, lazily allocating if necessary.  If
	// an implementation of this language runtime wasn't found, on an error occurs, a non-nil error is returned.
	LanguageRuntime(runtime string) (LanguageRuntime, error)

	// ListPlugins lists all plugins that have been loaded, with version information.
	ListPlugins() []workspace.PluginInfo
	// EnsurePlugins ensures all plugins for the target package are loaded.  If any are missing, and/or there are
	// errors loading one or more plugins, a non-nil error is returned.
	EnsurePlugins(info ProgInfo) error
	// GetRequiredPlugins lists a full set of plugins that will be required by the given program.
	GetRequiredPlugins(info ProgInfo) ([]workspace.PluginInfo, error)

	// Close reclaims any resources associated with the host.
	Close() error
}

// NewDefaultHost implements the standard plugin logic, using the standard installation root to find them.
func NewDefaultHost(ctx *Context, config ConfigSource) (Host, error) {
	host := &defaultHost{
		ctx:             ctx,
		config:          config,
		analyzerPlugins: make(map[tokens.QName]*analyzerPlugin),
		languagePlugins: make(map[string]*languagePlugin),
		resourcePlugins: make(map[tokens.Package]*resourcePlugin),
	}

	// Fire up a gRPC server to listen for requests.  This acts as a RPC interface that plugins can use
	// to "phone home" in case there are things the host must do on behalf of the plugins (like log, etc).
	svr, err := newHostServer(host, ctx)
	if err != nil {
		return nil, err
	}
	host.server = svr

	return host, nil
}

type defaultHost struct {
	ctx             *Context                           // the shared context for this host.
	config          ConfigSource                       // the source for provider configuration parameters.
	analyzerPlugins map[tokens.QName]*analyzerPlugin   // a cache of analyzer plugins and their processes.
	languagePlugins map[string]*languagePlugin         // a cache of language plugins and their processes.
	resourcePlugins map[tokens.Package]*resourcePlugin // a cache of resource plugins and their processes.
	plugins         []workspace.PluginInfo             // a list of plugins allocated by this host.
	server          *hostServer                        // the server's RPC machinery.
}

type analyzerPlugin struct {
	Plugin Analyzer
	Info   workspace.PluginInfo
}

type languagePlugin struct {
	Plugin LanguageRuntime
	Info   workspace.PluginInfo
}

type resourcePlugin struct {
	Plugin Provider
	Info   workspace.PluginInfo
}

func (host *defaultHost) ServerAddr() string {
	return host.server.Address()
}

func (host *defaultHost) Log(sev diag.Severity, msg string) {
	host.ctx.Diag.Logf(sev, diag.RawMessage(msg))
}

func (host *defaultHost) Analyzer(name tokens.QName) (Analyzer, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.analyzerPlugins[name]; has {
		contract.Assert(plug != nil)
		return plug.Plugin, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewAnalyzer(host, host.ctx, name)
	if err == nil && plug != nil {
		info, infoerr := plug.GetPluginInfo()
		if infoerr != nil {
			return nil, infoerr
		}

		// Memoize the result.
		host.plugins = append(host.plugins, info)
		host.analyzerPlugins[name] = &analyzerPlugin{Plugin: plug, Info: info}
	}

	return plug, err
}

func (host *defaultHost) Provider(pkg tokens.Package, version *semver.Version) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.resourcePlugins[pkg]; has {
		contract.Assert(plug != nil)

		// Make sure the versions match.
		// TODO: support loading multiple plugin versions side-by-side.
		if version != nil {
			if plug.Info.Version == nil {
				return nil,
					errors.Errorf("resource plugin version %s requested, but an unknown version was found",
						version.String())
			} else if !version.EQ(*plug.Info.Version) {
				return nil,
					errors.Errorf("resource plugin version %s requested, but version %s was found",
						version.String(), plug.Info.Version.String())
			}
		}

		return plug.Plugin, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewProvider(host, host.ctx, pkg, version)
	if err == nil && plug != nil {
		info, infoerr := plug.GetPluginInfo()
		if infoerr != nil {
			return nil, infoerr
		}

		// Warn if the plugin version was not what we expected
		if version != nil {
			if info.Version == nil || !version.EQ(*info.Version) {
				var v string
				if info.Version != nil {
					v = info.Version.String()
				}
				host.ctx.Diag.Warningf(
					diag.Message("resource plugin %s mis-reported its own version, expected %s got %s"),
					info.Name, version.String(), v)
			}
		}

		// Configure the provider. If no configuration source is present, assume no configuration. We do this here
		// because resource providers must be configured exactly once before any method besides Configure is called.
		providerConfig := make(map[tokens.ModuleMember]string)
		if host.config != nil {
			packageConfig, packageConfigErr := host.config.GetPackageConfig(pkg)
			if packageConfigErr != nil {
				return nil, errors.Wrapf(packageConfigErr,
					"failed to fetch configuration for pkg '%v' resource provider", pkg)
			}
			for k, v := range packageConfig {
				providerConfig[k.AsModuleMember()] = v
			}
		}
		if err = plug.Configure(providerConfig); err != nil {
			return nil, errors.Wrapf(err, "failed to configure pkg '%v' resource provider", pkg)
		}

		// Memoize the result.
		host.plugins = append(host.plugins, info)
		host.resourcePlugins[pkg] = &resourcePlugin{Plugin: plug, Info: info}
	}

	return plug, err
}

func (host *defaultHost) LanguageRuntime(runtime string) (LanguageRuntime, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.languagePlugins[runtime]; has {
		contract.Assert(plug != nil)
		return plug.Plugin, nil
	}

	// If not, allocate a new one.
	plug, err := NewLanguageRuntime(host, host.ctx, runtime)
	if err == nil && plug != nil {
		info, infoerr := plug.GetPluginInfo()
		if infoerr != nil {
			return nil, infoerr
		}

		// Memoize the result.
		host.plugins = append(host.plugins, info)
		host.languagePlugins[runtime] = &languagePlugin{Plugin: plug, Info: info}
	}

	return plug, err
}

func (host *defaultHost) ListPlugins() []workspace.PluginInfo {
	return host.plugins
}

// EnsurePlugins ensures all plugins for the target package are loaded.  If any are missing, and/or there are
// errors loading one or more plugins, a non-nil error is returned.
func (host *defaultHost) EnsurePlugins(info ProgInfo) error {
	// Compute the list of required plugins, and then iterate them and load 'em up.  This simultaneously ensures
	// they are installed on the system while also loading them into memory for easy subsequent access.
	plugins, err := host.GetRequiredPlugins(info)
	if err != nil {
		return err
	}

	// Use a multieerror to track failures so we can return one big list of all failures at the end.
	var result error
	for _, plugin := range plugins {
		switch plugin.Kind {
		case workspace.AnalyzerPlugin:
			if _, err := host.Analyzer(tokens.QName(plugin.Name)); err != nil {
				result = multierror.Append(result,
					errors.Wrapf(err, "failed to load analyzer plugin %s", plugin.Name))
			}
		case workspace.LanguagePlugin:
			if _, err := host.LanguageRuntime(plugin.Name); err != nil {
				result = multierror.Append(result,
					errors.Wrapf(err, "failed to load language plugin %s", plugin.Name))
			}
		case workspace.ResourcePlugin:
			if _, err := host.Provider(tokens.Package(plugin.Name), plugin.Version); err != nil {
				result = multierror.Append(result,
					errors.Wrapf(err, "failed to load resource plugin %s", plugin.Name))
			}
		default:
			contract.Failf("unexpected plugin kind: %s", plugin.Kind)
		}
	}

	return result
}

// GetRequiredPlugins lists a full set of plugins that will be required by the given program.
func (host *defaultHost) GetRequiredPlugins(info ProgInfo) ([]workspace.PluginInfo, error) {
	var plugins []workspace.PluginInfo

	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	lang, err := host.LanguageRuntime(info.Proj.Runtime)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load language plugin %s", info.Proj.Runtime)
	}
	plugins = append(plugins, workspace.PluginInfo{
		Name: info.Proj.Runtime,
		Kind: workspace.LanguagePlugin,
	})

	// Next, if there are analyzers listed in the project file, use them too.
	// TODO: these are currently not versioned.  We probably need to let folks specify versions in Pulumi.yaml.
	if info.Proj.Analyzers != nil {
		for _, analyzer := range *info.Proj.Analyzers {
			plugins = append(plugins, workspace.PluginInfo{
				Name: string(analyzer),
				Kind: workspace.AnalyzerPlugin,
			})
		}
	}

	// Finally, leverage the language plugin to compute this project's set of plugin dependencies.
	// TODO: we want to support loading precisely what the project needs, rather than doing a static scan of resolved
	//     packages.  Doing this requires that we change our RPC interface and figure out how to configure plugins
	//     later than we do (right now, we do it up front, but at that point we don't know the version).
	deps, err := lang.GetRequiredPlugins(info)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to discover plugin requirements")
	}
	plugins = append(plugins, deps...)

	return plugins, nil
}

func (host *defaultHost) Close() error {
	// Close all plugins.
	for _, plug := range host.analyzerPlugins {
		if err := plug.Plugin.Close(); err != nil {
			glog.Infof("Error closing '%s' analyzer plugin during shutdown; ignoring: %v", plug.Info.Name, err)
		}
	}
	for _, plug := range host.resourcePlugins {
		if err := plug.Plugin.Close(); err != nil {
			glog.Infof("Error closing '%s' resource plugin during shutdown; ignoring: %v", plug.Info.Name, err)
		}
	}
	for _, plug := range host.languagePlugins {
		if err := plug.Plugin.Close(); err != nil {
			glog.Infof("Error closing '%s' language plugin during shutdown; ignoring: %v", plug.Info.Name, err)
		}
	}

	// Empty out all maps.
	host.analyzerPlugins = make(map[tokens.QName]*analyzerPlugin)
	host.languagePlugins = make(map[string]*languagePlugin)
	host.resourcePlugins = make(map[tokens.Package]*resourcePlugin)

	// Finally, shut down the host's gRPC server.
	return host.server.Cancel()
}
