// Copyright 2025, Pulumi Corporation.
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

package packageinstallation

import (
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Workspace represents the way that [InstallPlugin] and [InstallInProject] interact with
// the environment.
type Workspace interface {
	packageresolution.PluginWorkspace

	// Return the plugin path of an already installed plugin.
	GetPluginPath(ctx context.Context, spec workspace.PluginSpec) (string, error)

	// Install an already downloaded plugin at a specific path.
	//
	// InstallPlugin should assume that all dependencies of the plugin are already
	// installed.
	InstallPluginAt(ctx context.Context, dirPath string, project *workspace.PluginProject) error

	// IsExecutable returns if the file at binaryPath can be executed.
	//
	// If no file is found at binaryPath, then (false, os.ErrNotExist) should be
	// returned.
	IsExecutable(ctx context.Context, binaryPath string) (bool, error)

	LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error)

	// Download a plugin onto disk, returning the path the plugin was downloaded to.
	DownloadPlugin(ctx context.Context, plugin workspace.PluginSpec) (string, MarkInstallationDone, error)

	DetectPluginPathAt(ctx context.Context, path string) (string, error)

	// Link a package into a project, generating an SDK if appropriate.
	//
	// project and projectDir describe the where the SDK is being generated and linked into.
	//
	// parameters describes any parameters necessary to convert the plugin into a
	// package.
	//
	// The plugin used to generate the SDK will always be installed already, and
	// should be run from pluginDir.
	LinkPackage(
		ctx context.Context,
		project *workspace.ProjectRuntimeInfo, projectDir string, packageName string,
		pluginPath string, params plugin.ParameterizeParameters,
	) error

	// Run a package from a directory, parameterized by params.
	//
	// If the package is served from a binary, then pluginPath will point at that
	// binary. If it's server from a directory, then pluginPath will be that
	// directory.
	RunPackage(
		ctx context.Context,
		rootDir, pluginPath string, params plugin.ParameterizeParameters,
	) (plugin.Provider, error)
}

type MarkInstallationDone = func(success bool)

type Options struct {
	packageresolution.Options
	// The maximum number of concurrent operations.
	Concurrency int
}

// A function to run the installed plugin.
//
// The returned plugin *may* already be parameterized, depending on if the requested
// [workspace.PluginSpec] specified parameterization (via the Pulumi registry).
//
// The plugin will be launched in wd.
type RunPlugin = func(ctx context.Context, wd string) (plugin.Provider, error)

// InstallPlugin installs a plugin into a project.
//
// If baseProject & projectDir are zero values, then InstallPlugin will just install the
// plugin.
//
// IO operations are intermediates by the passed in [Workspace] and
// [registry.Registry]. Both may be operated on in parallel if options.Concurrency > 1 or
// if it's unset.
//
// If a cyclic dependency is found, then an instance of [ErrorCyclicDependencies] will be
// returned. It can be accessed with [errors.As]:
//
//	_, err := packageinstallation.InstallPlugin(...)
//	var cycle packageinstallation.ErrorCyclicDependencies
//	if errros.As(err, &cycle) {
//		fmt.Println(cycle.Cycle)
//	}
func InstallPlugin(
	ctx context.Context,
	spec workspace.PluginSpec,
	baseProject workspace.BaseProject, projectDir string,
	options Options,
	registry registry.Registry, ws Workspace,
) (RunPlugin, error) {
	var runBundle runBundle

	setup := func(ctx context.Context, state state, root pdag.Node) error {
		return ensureUnresolvedSpec(ctx, state, root, spec, project[workspace.BaseProject]{
			proj:       baseProject,
			projectDir: projectDir,
		}, &runBundle)
	}

	err := runInstall(ctx, options, registry, ws, setup)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, wd string) (plugin.Provider, error) {
		return ws.RunPackage(ctx, wd, runBundle.pluginPath, runBundle.params)
	}, nil
}

// Install all plugins in a project, linking them in as necessary.
//
// This is conceptually equivalent to calling [InstallPlugin] for each plugin in a
// project. InstallInProject should be preferred because it deduplicates installs across
// the whole project installation and shares concurrency limit across all project
// dependencies.
func InstallInProject(
	ctx context.Context,
	project_ workspace.BaseProject, projectDir string,
	options Options, registry registry.Registry, ws Workspace,
) error {
	setup := func(ctx context.Context, state state, root pdag.Node) error {
		return ensureProjectDependencies(ctx, state, root, project[workspace.BaseProject]{
			proj:       project_,
			projectDir: projectDir,
		})
	}

	return runInstall(ctx, options, registry, ws, setup)
}

func runInstall(
	ctx context.Context,
	options Options, registry registry.Registry, ws Workspace,
	setup func(ctx context.Context, state state, root pdag.Node) error,
) error {
	dag := pdag.New[step]()
	root, rootReady := dag.NewNode(noOpStep{})

	// State shared across all nodes.
	state := state{
		ws:       ws,
		registry: registry,
		options:  options,
		dag:      dag,

		// Most Installs will install exactly one binary plugin, so pre-allocate
		// for that.
		seen:  make(map[pluginHash]cachedPlugin, 1),
		seenM: new(sync.Mutex),

		cleanupFuncs: nil,
		cleanupM:     new(sync.Mutex),
	}

	defer func() {
		for _, f := range state.cleanupFuncs {
			f()
		}
	}()

	if err := setup(ctx, state, root); err != nil {
		return err
	}

	rootReady() // Now that at least one spec has been added, it's safe to mark the root as ready.
	err := dag.Walk(ctx, func(ctx context.Context, step step) error {
		return step.run(ctx, state)
	}, pdag.MaxProcs(options.Concurrency))

	return wrapCycleError(err)
}

type ErrorCyclicDependencies struct {
	Cycle []workspace.PluginSpec

	underlying error
}

func (ErrorCyclicDependencies) Error() string { return "cyclic dependency" }

func (err ErrorCyclicDependencies) Unwrap() error { return err.underlying }

func wrapCycleError(err error) error {
	var cycle pdag.ErrorCycle[step]
	if !errors.As(err, &cycle) {
		return err
	}
	steps := cycle.Cycle
	chain := make([]workspace.PluginSpec, 0, len(steps)/2)
	for _, step := range steps {
		marker, ok := step.(pluginMarkerStep)
		if !ok {
			continue
		}
		chain = append(chain, marker.spec)
	}
	chain = append(chain[1:], chain[0])
	return ErrorCyclicDependencies{Cycle: chain, underlying: err}
}

type pluginMarkerStep struct{ spec workspace.PluginSpec }

func (step pluginMarkerStep) run(context.Context, state) error { return nil }

type pluginHash uint64

var mapHashSeed = maphash.MakeSeed()

func hashPluginSpec(spec workspace.PluginSpec) pluginHash {
	var h maphash.Hash
	h.SetSeed(mapHashSeed)
	write := func(b []byte) {
		_, err := h.Write(b)
		contract.AssertNoErrorf(err, "Hashing should never error")
	}

	write([]byte(spec.Name))
	write([]byte(spec.Kind))
	write([]byte(spec.PluginDownloadURL))
	if spec.Version != nil {
		write([]byte{'v', 's'}) // start version
		write([]byte(spec.Version.String()))
		write([]byte{'v', 'e'}) // end version
	}
	for k, v := range spec.Checksums {
		write([]byte{'k', 's'}) // start key
		write([]byte(k))
		write([]byte{'k', 'e'}) // end key
		write(v)
	}
	return pluginHash(h.Sum64())
}

func hashLocalPath(path string) pluginHash {
	var h maphash.Hash
	h.SetSeed(mapHashSeed)
	h.WriteString("local!")
	h.WriteString(path)
	return pluginHash(h.Sum64())
}

type runBundle struct {
	pluginPath string
	params     plugin.ParameterizeParameters
}

type step interface {
	run(ctx context.Context, p state) error
}

type state struct {
	ws       Workspace
	registry registry.Registry
	options  Options
	dag      *pdag.DAG[step]

	// A mapping of plugins already managed by dag.
	seen  map[pluginHash]cachedPlugin
	seenM *sync.Mutex

	cleanupFuncs []func()
	cleanupM     *sync.Mutex
}

type cachedPlugin struct {
	node      pdag.Node
	runBundle *runBundle
}

type noOpStep struct{}

func (noOpStep) run(context.Context, state) error { return nil }

type project[T workspace.BaseProject] struct {
	proj       T
	projectDir string
}

func ensureUnresolvedSpec(
	_ context.Context,
	state state, parent pdag.Node,
	spec workspace.PluginSpec, parentProj project[workspace.BaseProject],
	runBundleOut *runBundle, // An async out param of where the plugin was installed
) error {
	specReady, ready := state.dag.NewNode(noOpStep{})
	contract.AssertNoErrorf(state.dag.NewEdge(specReady, parent), "linking in a new node is safe")

	// First, we need resolve the spec into a concrete option. Since this can involve
	// network calls, we perform resolves in parallel.

	resolve, resolveReady := state.dag.NewNode(resolveStep{
		spec:         spec,
		parentProj:   parentProj,
		parent:       specReady,
		done:         ready,
		runBundleOut: runBundleOut,
	})
	// We know that resolving a spec doesn't have any concrete dependencies, so we can kick that off immediately.
	resolveReady()

	// At minimum, we need to resolve spec before spec is done.
	contract.AssertNoErrorf(state.dag.NewEdge(resolve, specReady), "linking in a new node is safe")
	return nil
}

type copyStep[T any] struct {
	src, dst *T
}

func (step copyStep[T]) run(context.Context, state) error { *step.dst = *step.src; return nil }

// Add nodes to dag to ensure that:
//
// 1. Each project dependency is downloaded and installed.
// 2.Each project dependency is *linked*.
func ensureProjectDependencies(
	ctx context.Context,
	state state, parent pdag.Node,
	proj project[workspace.BaseProject],
) error {
	// Sort package names for deterministic ordering
	packages := proj.proj.GetPackageSpecs()
	for _, name := range slices.Sorted(maps.Keys(packages)) {
		source := packages[name]
		runBundle := new(runBundle)
		link, linkReady := state.dag.NewNode(linkPackageStep{
			packageName: name,
			project:     proj,
			runBundle:   runBundle, // We don't know this until after we install the spec
		})
		defer linkReady()
		contract.AssertNoErrorf(state.dag.NewEdge(link, parent), "new nodes cannot be cyclic")

		var version *semver.Version
		if source.Version != "" {
			v, err := semver.ParseTolerant(source.Version)
			if err != nil {
				return nil
			}
			version = &v
		}
		err := ensureUnresolvedSpec(ctx, state, link, workspace.PluginSpec{
			Name:    source.Source,
			Kind:    apitype.ResourcePlugin,
			Version: version,
		}, proj, runBundle)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureDownloadedPluginDirHasDependenciesAndIsInstalled(
	ctx context.Context, state state,
	parent pdag.Node, name, projectDir string,
	downloadCleanup *downloadCleanup,
	runBundleOut *runBundle,
) error {
	filePath, err := state.ws.DetectPluginPathAt(ctx, projectDir)
	switch {
	// There is a PulumiPlugin file, so it may have dependencies. We need to
	// gather dependencies and install them before we can run the install
	// here.
	case err == nil:
		runBundleOut.pluginPath = projectDir
		pluginProject, err := state.ws.LoadPluginProject(ctx, filePath)
		if err != nil {
			return err
		}

		install, installReady := state.dag.NewNode(installStep{
			downloadCleanup: downloadCleanup,
			project: project[*workspace.PluginProject]{
				proj:       pluginProject,
				projectDir: projectDir,
			},
		})
		contract.AssertNoErrorf(state.dag.NewEdge(install, parent), "new nodes cannot be cyclic")
		defer installReady()

		return ensureProjectDependencies(ctx, state, install, project[workspace.BaseProject]{
			proj:       pluginProject,
			projectDir: projectDir,
		})

	// We didn't detect a PulumiPlugin file. This may be a binary plugin, so
	// let's check. If there is a appropriately named binary there, we are
	// done.
	case errors.Is(err, workspace.ErrPluginNotFound):
		binaryName := "pulumi-resource-" + name
		binaryPath := filepath.Join(projectDir, binaryName)
		isExec, err := state.ws.IsExecutable(ctx, binaryPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		} else if isExec {
			runBundleOut.pluginPath = binaryPath
			// A binary was found, so this plugin is done.
			downloadCleanup.f(true)
			downloadCleanup.called = true
			return nil
		}
		return fmt.Errorf("expected %s to have an executable named %s or a PulumiPlugin file", projectDir, binaryName)

	// An unknown error was returned, so bubble the error up.
	default:
		return err
	}
}

type linkPackageStep struct {
	// The name of the package, as described in the Pulumi or PulumiPlugin packages
	// section.
	packageName string

	// The directory the plugin was installed into.
	//
	// runBundle must be set to a non-empty value by the time this step executes.
	runBundle *runBundle

	// The project we are linking into.
	project project[workspace.BaseProject]
}

func (step linkPackageStep) run(ctx context.Context, p state) error {
	contract.Assertf(step.runBundle != nil, "must set run bundle before running this step")
	if step.runBundle.params != nil && !step.runBundle.params.Empty() &&
		len(step.project.proj.GetPackageSpecs()[step.packageName].Parameters) > 0 {
		return fmt.Errorf("%s specified duplicate parameter sources", step.packageName)
	}
	var params plugin.ParameterizeParameters
	if step.runBundle.params != nil && !step.runBundle.params.Empty() {
		params = step.runBundle.params
	} else if p := step.project.proj.GetPackageSpecs()[step.packageName].Parameters; len(p) > 0 {
		params = &plugin.ParameterizeArgs{Args: p}
	}
	return p.ws.LinkPackage(
		ctx,
		step.project.proj.RuntimeInfo(), step.project.projectDir,
		step.packageName, step.runBundle.pluginPath, params)
}

// Resolve a spec into a plugin, then add necessary follow up steps.
type resolveStep struct {
	spec         workspace.PluginSpec
	parentProj   project[workspace.BaseProject]
	parent       pdag.Node
	done         pdag.Done
	runBundleOut *runBundle
}

// newSpecNode adds a new spec to the DAG, or de-duplicates the spec.
//
// Correct usage looks like this:
//
//	spec, ready, isDuplicate, err := newSpecNode(...)
//	if err != nil {
//		return err
//	}
//	if isDuplicate {
//		return nil
//	}
//	// At this point, we are now responsible for ensuring that ready is called.
func newSpecNode(
	hash pluginHash, spec workspace.PluginSpec, runBundleOut *runBundle, state state, parent pdag.Node,
) (pdag.Node, pdag.Done, bool, error) {
	specReady, ready := state.dag.NewNode(pluginMarkerStep{
		spec: spec,
	})

	state.seenM.Lock()
	defer state.seenM.Unlock()
	if n, ok := state.seen[hash]; ok {
		// After n has resolved, we need to update our runBundleOut to be the same
		// as what is cached. That means that cached plugins have as their node format:
		//
		//	original plugin -> copy runBundle -> spec ready -> parent

		defer ready()
		contract.AssertNoErrorf(state.dag.NewEdge(specReady, parent),
			"linking in a new node is safe")

		copyBundle, ready := state.dag.NewNode(copyStep[runBundle]{
			src: n.runBundle,
			dst: runBundleOut,
		})
		defer ready()
		contract.AssertNoErrorf(state.dag.NewEdge(copyBundle, specReady),
			"linking in a new node is safe")

		return n.node, func() {}, true, state.dag.NewEdge(n.node, copyBundle)
	}

	err := state.dag.NewEdge(specReady, parent)
	if err != nil {
		ready()
		return pdag.Node{}, nil, false, err
	}
	state.seen[hash] = cachedPlugin{
		node:      specReady,
		runBundle: runBundleOut,
	}

	return specReady, ready, false, nil
}

// Resolve a package into something that we can get.
//
// The resolution step is intertwined with de-duplicating nodes
func (step resolveStep) run(ctx context.Context, p state) error {
	defer step.done()

	// TODO: The registry should be wrapped in a caching layer to de-duplicate calls.
	result, err := packageresolution.Resolve(ctx, p.registry, p.ws, step.spec,
		p.options.Options, step.parentProj.proj)
	if err != nil {
		return err
	}

	switch result := result.(type) {
	// Just check that the project is there, and install any dependencies if there is
	// a PulumiPlugin file found.
	case packageresolution.LocalPathResult:
		projectDir := result.LocalPath
		// TODO: What about [result.RelativeToWorkspace]? Right now it's always
		// false, but I don't think it should be.
		if !filepath.IsAbs(projectDir) /* && result.RelativeToWorkspace */ {
			projectDir = filepath.Join(step.parentProj.projectDir, result.LocalPath)
		}

		// Now that we have fully resolved the file path, we can de-duplicate to
		// make sure that we won't reference the same node twice in the graph.
		//
		// Local paths are identified uniquely by their paths, so we use that to
		// de-duplicate.

		absPath, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		specNode, ready, isDuplicate, err := newSpecNode(hashLocalPath(absPath), step.spec, step.runBundleOut, p, step.parent)
		if err != nil {
			return err
		}
		if isDuplicate {
			return nil
		}

		defer ready()

		// We don't need to download what's at a local path result, but we might
		// need to download it's dependencies.
		return ensureDownloadedPluginDirHasDependenciesAndIsInstalled(ctx, p, specNode, "", projectDir, nil, step.runBundleOut)

	// We have a normal spec to download and install, so let's run that process.
	//
	// To install from an external source, we need to:
	//
	// 1. Download the plugin.
	//
	// 2. Check for any dependencies, making sure that dependencies are downloaded
	// *and* installed.
	//
	// 3. Install the downloaded project.
	case packageresolution.ExternalSourceResult:
		specFinished, specReady, isDuplicate, err := newSpecNode(
			hashPluginSpec(result.Spec), result.Spec, step.runBundleOut, p, step.parent)
		if err != nil {
			return err
		}
		if isDuplicate {
			return nil
		}

		// Start with the download. The downloadStep will take care of attaching
		// steps for (2) and (3) to specFinished.
		download, downloadReady := p.dag.NewNode(downloadStep{
			spec:            result.Spec,
			parent:          specFinished,
			done:            specReady,
			runBundleOut:    step.runBundleOut,
			downloadCleanup: new(downloadCleanup),
		})
		downloadReady()
		contract.AssertNoErrorf(p.dag.NewEdge(download, specFinished), "new nodes cannot be cyclic")
		return nil

	case packageresolution.RegistryResult:
		spec := workspace.PluginSpec{
			Name:              result.Metadata.Name,
			Kind:              apitype.ResourcePlugin,
			Version:           &result.Metadata.Version,
			PluginDownloadURL: result.Metadata.PluginDownloadURL,
		}
		if result.Metadata.Parameterization != nil {
			p := result.Metadata.Parameterization
			spec.Name = p.BaseProvider.Name
			spec.Version = &p.BaseProvider.Version
			step.runBundleOut.params = &plugin.ParameterizeValue{
				Name:    result.Metadata.Name,
				Version: result.Metadata.Version,
				Value:   p.Parameter,
			}
		}

		specFinished, specReady, isDuplicate, err := newSpecNode(
			hashPluginSpec(spec), spec, step.runBundleOut, p, step.parent)
		if err != nil {
			return err
		}
		if isDuplicate {
			return nil
		}

		// Start with the download. The downloadStep will take care of attaching
		// steps for (2) and (3) to specFinished.
		download, downloadReady := p.dag.NewNode(downloadStep{
			spec:            spec,
			parent:          specFinished,
			done:            specReady,
			runBundleOut:    step.runBundleOut,
			downloadCleanup: new(downloadCleanup),
		})
		downloadReady()
		contract.AssertNoErrorf(p.dag.NewEdge(download, specFinished), "new nodes cannot be cyclic")
		return nil

	case packageresolution.InstalledInWorkspaceResult:
		// the package is already installed, which means we don't need to do
		// anything.
		//
		// For now, we assume that [packageresolution.InstalledInWorkspaceResult]
		// means:
		//
		// 1. the package was installed
		// 2. it's dependencies were installed
		// 3. it was installed correctly
		//
		// (1) and (2) are guaranteed by using [Install]. (3) will be guaranteed
		// by distinguishing between install status on disk with lockfiles.
		//
		// TODO: Verify that (3) is a problem, then open an issue.
		path, err := p.ws.GetPluginPath(ctx, step.spec)
		step.runBundleOut.pluginPath = path
		return err
	default:
		panic(fmt.Sprintf("unexpected package resolution result of type %T: %[1]s", result))
	}
}

// Download an external spec, then attach appropriate follow up nodes to the DAG.
type downloadStep struct {
	spec         workspace.PluginSpec // An already resolved spec
	parent       pdag.Node
	done         pdag.Done
	runBundleOut *runBundle

	downloadCleanup *downloadCleanup
}

type downloadCleanup struct {
	f      func(success bool)
	called bool
}

func (step downloadStep) run(ctx context.Context, p state) error {
	defer step.done()
	pluginDir, doneF, err := p.ws.DownloadPlugin(ctx, step.spec)
	if err != nil {
		return err
	}
	step.downloadCleanup.f = doneF

	// Add a hook to cleanup the download after usage.
	p.cleanupM.Lock()
	p.cleanupFuncs = append(p.cleanupFuncs, func() {
		if step.downloadCleanup.called {
			return
		}
		step.downloadCleanup.f(false)
	})
	p.cleanupM.Unlock()

	step.runBundleOut.pluginPath = pluginDir
	return ensureDownloadedPluginDirHasDependenciesAndIsInstalled(
		ctx, p, step.parent, step.spec.Name, pluginDir, step.downloadCleanup, step.runBundleOut)
}

type installStep struct {
	downloadCleanup *downloadCleanup
	project         project[*workspace.PluginProject]
}

func (step installStep) run(ctx context.Context, p state) error {
	err := p.ws.InstallPluginAt(ctx, step.project.projectDir, step.project.proj)

	// If the location we are installing was downloaded, then we need to call the
	// cleanup function.
	if step.downloadCleanup != nil {
		step.downloadCleanup.called = true
		step.downloadCleanup.f(err == nil)
	}
	return err
}
