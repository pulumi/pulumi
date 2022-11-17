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

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A Host hosts provider plugins and makes them easily accessible by package name.
type Host interface {
	// ServerAddr returns the address at which the host's RPC interface may be found.
	ServerAddr() string

	// Log logs a message, including errors and warnings.  Messages can have a resource URN
	// associated with them.  If no urn is provided, the message is global.
	Log(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	// LogStatus logs a status message message, including errors and warnings. Status messages show
	// up in the `Info` column of the progress display, but not in the final output. Messages can
	// have a resource URN associated with them.  If no urn is provided, the message is global.
	LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for
	// it.  If an analyzer could not be found, or an error occurred while creating it, a non-nil
	// error is returned.
	Analyzer(nm tokens.QName) (Analyzer, error)

	// PolicyAnalyzer boots the nodejs analyzer plugin located at a given path. This is useful
	// because policy analyzers generally do not need to be "discovered" -- the engine is given a
	// set of policies that are required to be run during an update, so they tend to be in a
	// well-known place.
	PolicyAnalyzer(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error)

	// ListAnalyzers returns a list of all analyzer plugins known to the plugin host.
	ListAnalyzers() []Analyzer

	// Provider loads a new copy of the provider for a given package.  If a provider for this package could not be
	// found, or an error occurs while creating it, a non-nil error is returned.
	Provider(pkg tokens.Package, version *semver.Version) (Provider, error)
	// CloseProvider closes the given provider plugin and deregisters it from this host.
	CloseProvider(provider Provider) error
	// LanguageRuntime fetches the language runtime plugin for a given language, lazily allocating if necessary.  If
	// an implementation of this language runtime wasn't found, on an error occurs, a non-nil error is returned.
	LanguageRuntime(root, pwd, runtime string, options map[string]interface{}) (LanguageRuntime, error)

	// EnsurePlugins ensures all plugins in the given array are loaded and ready to use.  If any plugins are missing,
	// and/or there are errors loading one or more plugins, a non-nil error is returned.
	EnsurePlugins(plugins []workspace.PluginSpec, kinds Flags) error
	// InstallPlugin installs a given plugin if it's not available.
	InstallPlugin(plugin workspace.PluginSpec) error

	// ResolvePlugin resolves a plugin kind, name, and optional semver to a candidate plugin to load.
	ResolvePlugin(kind workspace.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error)

	GetProjectPlugins() []workspace.ProjectPlugin

	// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing
	// operations. Operation aborted in this way will return an error (e.g., `Update` and `Create`
	// will either a creation error or an initialization error. SignalCancellation is advisory and
	// non-blocking; it is up to the host to decide how long to wait after SignalCancellation is
	// called before (e.g.) hard-closing any gRPC connection.
	SignalCancellation() error

	// Close reclaims any resources associated with the host.
	Close() error
}

// NewDefaultHost implements the standard plugin logic, using the standard installation root to find them.
func NewDefaultHost(ctx *Context, runtimeOptions map[string]interface{},
	disableProviderPreview bool, plugins *workspace.Plugins) (Host, error) {
	// Create plugin info from providers
	projectPlugins := make([]workspace.ProjectPlugin, 0)
	if plugins != nil {
		for _, providerOpts := range plugins.Providers {
			info, err := parsePluginOpts(providerOpts, workspace.ResourcePlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
		for _, languageOpts := range plugins.Languages {
			info, err := parsePluginOpts(languageOpts, workspace.LanguagePlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
		for _, analyzerOpts := range plugins.Analyzers {
			info, err := parsePluginOpts(analyzerOpts, workspace.AnalyzerPlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
	}

	host := &defaultHost{
		ctx:                     ctx,
		runtimeOptions:          runtimeOptions,
		analyzerPlugins:         make(map[tokens.QName]*analyzerPlugin),
		languagePlugins:         make(map[string]*languagePlugin),
		resourcePlugins:         make(map[Provider]*resourcePlugin),
		reportedResourcePlugins: make(map[string]struct{}),
		languageLoadRequests:    make(chan pluginLoadRequest),
		loadRequests:            make(chan pluginLoadRequest),
		disableProviderPreview:  disableProviderPreview,
		closer:                  new(sync.Once),
		projectPlugins:          projectPlugins,
	}

	// Fire up a gRPC server to listen for requests.  This acts as a RPC interface that plugins can use
	// to "phone home" in case there are things the host must do on behalf of the plugins (like log, etc).
	svr, err := newHostServer(host, ctx)
	if err != nil {
		return nil, err
	}
	host.server = svr

	// Start a goroutine we'll use to satisfy load requests serially and avoid race conditions.
	go func() {
		for req := range host.loadRequests {
			req.result <- req.load()
		}
	}()

	// Start another goroutine we'll use to satisfy load language plugin requests, this is so other plugins
	// can be started up by a language plugin.
	go func() {
		for req := range host.languageLoadRequests {
			req.result <- req.load()
		}
	}()

	return host, nil
}

func parsePluginOpts(providerOpts workspace.PluginOptions, k workspace.PluginKind) (workspace.ProjectPlugin, error) {
	var v *semver.Version
	if providerOpts.Version != "" {
		ver, err := semver.Parse(providerOpts.Version)
		if err != nil {
			return workspace.ProjectPlugin{}, err
		}
		v = &ver
	}

	_, err := os.Stat(providerOpts.Path)
	if err != nil {
		return workspace.ProjectPlugin{}, fmt.Errorf("could not find provider folder at path %s", providerOpts.Path)
	}

	pluginInfo := workspace.ProjectPlugin{
		Name:    providerOpts.Name,
		Path:    filepath.Clean(providerOpts.Path),
		Kind:    k,
		Version: v,
	}
	return pluginInfo, nil
}

// PolicyAnalyzerOptions includes a bag of options to pass along to a policy analyzer.
type PolicyAnalyzerOptions struct {
	Organization string
	Project      string
	Stack        string
	Config       map[config.Key]string
	DryRun       bool
}

type pluginLoadRequest struct {
	load   func() error
	result chan<- error
}

type defaultHost struct {
	ctx *Context // the shared context for this host.

	// the runtime options for the project, passed to resource providers to support dynamic providers.
	runtimeOptions          map[string]interface{}
	analyzerPlugins         map[tokens.QName]*analyzerPlugin // a cache of analyzer plugins and their processes.
	languagePlugins         map[string]*languagePlugin       // a cache of language plugins and their processes.
	resourcePlugins         map[Provider]*resourcePlugin     // the set of loaded resource plugins.
	reportedResourcePlugins map[string]struct{}              // the set of unique resource plugins we'll report.
	languageLoadRequests    chan pluginLoadRequest           // a channel used to satisfy language load requests.
	loadRequests            chan pluginLoadRequest           // a channel used to satisfy plugin load requests.
	server                  *hostServer                      // the server's RPC machinery.
	disableProviderPreview  bool                             // true if provider plugins should disable provider preview

	closer         *sync.Once
	projectPlugins []workspace.ProjectPlugin
}

var _ Host = (*defaultHost)(nil)

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

func (host *defaultHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.ctx.Diag.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

func (host *defaultHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.ctx.StatusDiag.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

// loadPlugin sends an appropriate load request to the plugin loader and returns the loaded plugin (if any) and error.
func loadPlugin(loadRequestChannel chan pluginLoadRequest, load func() (interface{}, error)) (interface{}, error) {
	var plugin interface{}

	result := make(chan error)
	loadRequestChannel <- pluginLoadRequest{
		load: func() error {
			p, err := load()
			plugin = p
			return err
		},
		result: result,
	}
	return plugin, <-result
}

func (host *defaultHost) Analyzer(name tokens.QName) (Analyzer, error) {
	plugin, err := loadPlugin(host.loadRequests, func() (interface{}, error) {
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
			host.analyzerPlugins[name] = &analyzerPlugin{Plugin: plug, Info: info}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(Analyzer), nil
}

func (host *defaultHost) PolicyAnalyzer(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error) {
	plugin, err := loadPlugin(host.loadRequests, func() (interface{}, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.analyzerPlugins[name]; has {
			contract.Assert(plug != nil)
			return plug.Plugin, nil
		}

		// If not, try to load and bind to a plugin.
		plug, err := NewPolicyAnalyzer(host, host.ctx, name, path, opts)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo()
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.analyzerPlugins[name] = &analyzerPlugin{Plugin: plug, Info: info}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(Analyzer), nil
}

func (host *defaultHost) ListAnalyzers() []Analyzer {
	analyzers := []Analyzer{}
	for _, analyzer := range host.analyzerPlugins {
		analyzers = append(analyzers, analyzer.Plugin)
	}
	return analyzers
}

func (host *defaultHost) Provider(pkg tokens.Package, version *semver.Version) (Provider, error) {
	plugin, err := loadPlugin(host.loadRequests, func() (interface{}, error) {
		// Try to load and bind to a plugin.
		plug, err := NewProvider(host, host.ctx, pkg, version, host.runtimeOptions, host.disableProviderPreview)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo()
			if infoerr != nil {
				return nil, infoerr
			}

			// Warn if the plugin version was not what we expected
			if version != nil && !cmdutil.IsTruthy(os.Getenv("PULUMI_DEV")) {
				if info.Version == nil || !info.Version.GTE(*version) {
					var v string
					if info.Version != nil {
						v = info.Version.String()
					}
					host.ctx.Diag.Warningf(
						diag.Message("", /*urn*/
							"resource plugin %s is expected to have version >=%s, but has %s; "+
								"the wrong version may be on your path, or this may be a bug in the plugin"),
						info.Name, version.String(), v)
				}
			}

			// Record the result and add the plugin's info to our list of loaded plugins if it's the first copy of its
			// kind.
			key := info.Name
			if info.Version != nil {
				key += info.Version.String()
			}
			_, alreadyReported := host.reportedResourcePlugins[key]
			if !alreadyReported {
				host.reportedResourcePlugins[key] = struct{}{}
			}
			host.resourcePlugins[plug] = &resourcePlugin{Plugin: plug, Info: info}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(Provider), nil
}

func (host *defaultHost) LanguageRuntime(root, pwd, runtime string,
	options map[string]interface{}) (LanguageRuntime, error) {
	// Language runtimes use their own loading channel not the main one
	plugin, err := loadPlugin(host.languageLoadRequests, func() (interface{}, error) {

		// Key our cached runtime plugins by the runtime name and the options
		jsonOptions, err := json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("could not marshal runtime options to JSON: %w", err)
		}

		key := runtime + ":" + root + ":" + pwd + ":" + string(jsonOptions)

		// First see if we already loaded this plugin.
		if plug, has := host.languagePlugins[key]; has {
			contract.Assert(plug != nil)
			return plug.Plugin, nil
		}

		// If not, allocate a new one.
		plug, err := NewLanguageRuntime(host, host.ctx, root, pwd, runtime, options)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo()
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.languagePlugins[key] = &languagePlugin{Plugin: plug, Info: info}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(LanguageRuntime), nil
}

// EnsurePlugins ensures all plugins in the given array are loaded and ready to use.  If any plugins are missing,
// and/or there are errors loading one or more plugins, a non-nil error is returned.
func (host *defaultHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds Flags) error {
	// Use a multieerror to track failures so we can return one big list of all failures at the end.
	var result error
	for _, plugin := range plugins {
		switch plugin.Kind {
		case workspace.AnalyzerPlugin:
			if kinds&AnalyzerPlugins != 0 {
				if _, err := host.Analyzer(tokens.QName(plugin.Name)); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "failed to load analyzer plugin %s", plugin.Name))
				}
			}
		case workspace.LanguagePlugin:
			if kinds&LanguagePlugins != 0 {
				// Pass nil options here, we just need to check the language plugin is loadable. We can't use
				// host.runtimePlugins because there might be other language plugins reported here (e.g
				// shimless multi-language providers). Pass the host root for the plugin directory, it
				// shouldn't matter because we're starting with no options but it's a directory we've already
				// got hold of.
				if _, err := host.LanguageRuntime(host.ctx.Root, host.ctx.Pwd, plugin.Name, nil); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "failed to load language plugin %s", plugin.Name))
				}
			}
		case workspace.ResourcePlugin:
			if kinds&ResourcePlugins != 0 {
				if _, err := host.Provider(tokens.Package(plugin.Name), plugin.Version); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "failed to load resource plugin %s", plugin.Name))
				}
			}
		default:
			contract.Failf("unexpected plugin kind: %s", plugin.Kind)
		}
	}

	return result
}

func (host *defaultHost) InstallPlugin(pkgPlugin workspace.PluginSpec) error {
	if !workspace.HasPlugin(pkgPlugin) {
		// TODO: schema and provider versions
		// hack: Some of the hcl2 code isn't yet handling versions, so bail out if the version is nil to avoid failing
		// 		 the download. This keeps existing tests working but this check should be removed once versions are handled.
		if pkgPlugin.Version == nil {
			return nil
		}

		tarball, err := workspace.DownloadToFile(pkgPlugin, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to download plugin: %s: %w", pkgPlugin, err)
		}
		defer os.Remove(tarball.Name())
		if err := pkgPlugin.InstallWithContext(host.ctx.baseContext, workspace.TarPlugin(tarball), false); err != nil {
			return fmt.Errorf("failed to install plugin %s: %w", pkgPlugin, err)
		}
	}

	return nil
}

func (host *defaultHost) ResolvePlugin(
	kind workspace.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error) {
	return workspace.GetPluginInfo(kind, name, version, host.GetProjectPlugins())
}

func (host *defaultHost) GetProjectPlugins() []workspace.ProjectPlugin {
	return host.projectPlugins
}

func (host *defaultHost) SignalCancellation() error {
	// NOTE: we're abusing loadPlugin in order to ensure proper synchronization.
	_, err := loadPlugin(host.loadRequests, func() (interface{}, error) {
		var result error
		for _, plug := range host.resourcePlugins {
			if err := plug.Plugin.SignalCancellation(); err != nil {
				result = multierror.Append(result, errors.Wrapf(err,
					"Error signaling cancellation to resource provider '%s'", plug.Info.Name))
			}
		}
		return nil, result
	})
	return err
}

func (host *defaultHost) CloseProvider(provider Provider) error {
	// NOTE: we're abusing loadPlugin in order to ensure proper synchronization.
	_, err := loadPlugin(host.loadRequests, func() (interface{}, error) {
		if err := provider.Close(); err != nil {
			return nil, err
		}
		delete(host.resourcePlugins, provider)
		return nil, nil
	})
	return err
}

func (host *defaultHost) Close() (err error) {
	host.closer.Do(func() {
		// Close all plugins.
		for _, plug := range host.analyzerPlugins {
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' analyzer plugin during shutdown; ignoring: %v", plug.Info.Name, err)
			}
		}
		for _, plug := range host.resourcePlugins {
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' resource plugin during shutdown; ignoring: %v", plug.Info.Name, err)
			}
		}
		for _, plug := range host.languagePlugins {
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' language plugin during shutdown; ignoring: %v", plug.Info.Name, err)
			}
		}

		// Empty out all maps.
		host.analyzerPlugins = make(map[tokens.QName]*analyzerPlugin)
		host.languagePlugins = make(map[string]*languagePlugin)
		host.resourcePlugins = make(map[Provider]*resourcePlugin)

		// Shut down the plugin loader.
		close(host.languageLoadRequests)
		close(host.loadRequests)

		// Finally, shut down the host's gRPC server.
		err = host.server.Cancel()
	})
	return err
}

// Flags can be used to filter out plugins during loading that aren't necessary.
type Flags int

const (
	// AnalyzerPlugins is used to only load analyzers.
	AnalyzerPlugins Flags = 1 << iota
	// LanguagePlugins is used to only load language plugins.
	LanguagePlugins
	// ResourcePlugins is used to only load resource provider plugins.
	ResourcePlugins
)

// AllPlugins uses flags to ensure that all plugin kinds are loaded.
var AllPlugins = AnalyzerPlugins | LanguagePlugins | ResourcePlugins

// GetRequiredPlugins lists a full set of plugins that will be required by the given program.
func GetRequiredPlugins(host Host, root string, info ProgInfo, kinds Flags) ([]workspace.PluginSpec, error) {
	var plugins []workspace.PluginSpec

	if kinds&LanguagePlugins != 0 {
		// First make sure the language plugin is present.  We need this to load the required resource plugins.
		// TODO: we need to think about how best to version this.  For now, it always picks the latest.
		lang, err := host.LanguageRuntime(root, info.Pwd, info.Proj.Runtime.Name(), info.Proj.Runtime.Options())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load language plugin %s", info.Proj.Runtime.Name())
		}
		plugins = append(plugins, workspace.PluginSpec{
			Name: info.Proj.Runtime.Name(),
			Kind: workspace.LanguagePlugin,
		})

		if kinds&ResourcePlugins != 0 {
			// Use the language plugin to compute this project's set of plugin dependencies.
			// TODO: we want to support loading precisely what the project needs, rather than doing a static scan of resolved
			//     packages.  Doing this requires that we change our RPC interface and figure out how to configure plugins
			//     later than we do (right now, we do it up front, but at that point we don't know the version).
			deps, err := lang.GetRequiredPlugins(info)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to discover plugin requirements")
			}
			plugins = append(plugins, deps...)
		}
	} else {
		// If we can't load the language plugin, we can't discover the resource plugins.
		contract.Assertf(kinds&ResourcePlugins != 0,
			"cannot load resource plugins without also loading the language plugin")
	}

	return plugins, nil
}
