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

package plugininstall

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	diagutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func New() PluginManager { return pluginManager{} }

type PluginManager interface {
	EnsureSpec(ctx context.Context, spec workspace.PluginSpec) (string, error)
	EnsureInPluginProject(ctx context.Context, plugin *workspace.PluginProject, path string) error
}

type pluginManager struct{}

type step interface {
	run(ctx context.Context) error
}

func (pm pluginManager) EnsureSpec(ctx context.Context, spec workspace.PluginSpec) (string, error) {
	dag := pdag.New[step]()
	root, rootReady := dag.NewNode(noOpStep{})
	ensureSpec(dag, &spec, root)
	rootReady() // Now that at least one spec has been added, it's safe to mark the root as ready.
	err := dag.Walk(ctx, func(ctx context.Context, step step) error {
		return step.run(ctx)
	}, pdag.MaxProcs(4))
	if err != nil {
		return "", err
	}
	return spec.DirPath()
}

func (pluginManager) EnsureInPluginProject(ctx context.Context, plugin *workspace.PluginProject, path string) error {
	dag := pdag.New[step]()
	root, rootReady := dag.NewNode(installPluginAtPathStep{
		plugin: plugin,
		path:   path,
	})
	ensureProjectDependencies(ctx, dag, plugin, path, root)
	rootReady() // Now that at least one spec has been added, it's safe to mark the root as ready.
	return dag.Walk(ctx, func(ctx context.Context, step step) error {
		return step.run(ctx)
	}, pdag.MaxProcs(4))
}

type installPluginAtPathStep struct {
	plugin *workspace.PluginProject
	path   string
}

func (step installPluginAtPathStep) run(ctx context.Context) error {
	path, err := filepath.Abs(step.path)
	if err != nil {
		return err
	}
	pctx, err := plugin.NewContextWithRoot(ctx,
		diagutil.Diag(),
		diagutil.Diag(),
		nil,  // host
		path, // pwd
		path, // root
		step.plugin.RuntimeInfo().Options(),
		false, // disableProviderPreview
		nil,   // tracingSpan
		nil,   // Plugins
		step.plugin.GetPackageSpecs(),
		nil, // config
		nil, // debugging
	)
	if err != nil {
		return err
	}

	return errors.Join(InstallPluginAtPath(pctx, step.plugin, os.Stdout, os.Stderr), pctx.Close())
}

type noOpStep struct{}

func (noOpStep) run(context.Context) error { return nil }

func ensureSpec(dag *pdag.DAG[step], spec *workspace.PluginSpec, root pdag.Node) {
	specInstall, specInstallReady := dag.NewNode(installSpecAfterDependencies{
		spec: spec,
	})
	contract.AssertNoErrorf(dag.NewEdge(specInstall, root), "a new edge is always a-cyclic")

	downloadSpec, downloadSpecReady := dag.NewNode(downloadAndUnpackSpecStep{
		dag:       dag,
		depsRoot:  specInstall,
		depsAdded: specInstallReady,
		spec:      spec,
	})
	contract.AssertNoErrorf(dag.NewEdge(downloadSpec, specInstall), "a new edge is always a-cyclic")
	downloadSpecReady()
}

type installSpecAfterDependencies struct {
	spec *workspace.PluginSpec // The spec to run the install for
}

// run the install for a spec. We can assume that all local dependencies have already been
// installed.
func (step installSpecAfterDependencies) run(ctx context.Context) error {
	dir, err := step.spec.DirPath()
	if err != nil {
		return err
	}

	// We only care about the directory that has the plugin itself.
	dir = filepath.Join(dir, step.spec.SubDir())

	return installDependenciesForPluginSpec(ctx, *step.spec, os.Stdout, os.Stderr)
}

type downloadAndUnpackSpecStep struct {
	dag       *pdag.DAG[step]
	depsRoot  pdag.Node // The node that newly discovered dependencies should target
	depsAdded pdag.Done // Indicate that all dependencies have been discovered and added

	// The spec to download and unpack, we allow spec to be mutated, updating its
	// version as necessary.
	spec *workspace.PluginSpec
}

func (step downloadAndUnpackSpecStep) run(ctx context.Context) error {
	defer step.depsAdded()
	if workspace.HasPlugin(*step.spec) {
		// TODO: It's possible that the install failed even if download has
		// succeeded in the past. We should check on that and re-run the install
		// here as necessary.
		return nil
	}

	content, err := downloadPluginFromSpecToTarball(ctx, step.spec)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	if err := unpackTarball(ctx, *step.spec, content, false /* reinstall */); err != nil {
		return fmt.Errorf("failed to unpack plugin: %w", err)
	}

	// Create a file lock file at <pluginsdir>/<kind>-<name>-<version>.lock.
	unlock, err := installLock(*step.spec)
	if err != nil {
		return err
	}
	defer unlock()

	// At this point, we have fully unpacked spec into it's destination directory. We
	// now need to install it's dependencies and then run the final install for the
	// spec.

	dir, err := step.spec.DirPath()
	if err != nil {
		return err
	}

	// We only care about the directory that has the plugin itself.
	dir = filepath.Join(dir, step.spec.SubDir())

	pulumiPluginPath, err := workspace.DetectPluginPathFrom(dir)
	if errors.Is(err, workspace.ErrPluginNotFound) ||
		(err == nil && filepath.Dir(pulumiPluginPath) != dir) {
		// There are multiple valid extensions for "PulumiPlugin.<ext>", but
		// "yaml" is standard so that's what we say in our error message.
		return errors.New("invalid plugin: does not contain a PulumiPlugin.yaml")
	}
	if err != nil {
		return err
	}

	pluginProject, err := workspace.LoadPluginProject(dir)
	if err != nil {
		return err
	}

	return ensureProjectDependencies(ctx, step.dag, pluginProject, dir, step.depsRoot)
}

func detectPluginPathAtDir(dir string) (string, error) {
	pluginPath, err := workspace.DetectPluginPathFrom(dir)
	if err != nil {
		return "", err
	}
	if filepath.Dir(pluginPath) != dir {
		return "", workspace.ErrPluginNotFound
	}
	return pluginPath, nil
}

func ensureProjectDependencies(
	ctx context.Context, dag *pdag.DAG[step],
	project workspace.BaseProject, projectDir string,
	root pdag.Node,
) error {
	for name, spec := range project.GetPackageSpecs() {
		var specIsInstalled pdag.Node
		// If the package is local, then that means it is already on
		// disk. We can load the plugin and install it's dependencies,
		// then install the plugin.
		if plugin.IsLocalPluginPath(ctx, spec.Source) {
			pulumiPluginFile, err := detectPluginPathAtDir(spec.Source)
			if err != nil {
				return err
			}
			pluginProject, err := workspace.LoadPluginProject(pulumiPluginFile)
			if err != nil {
				return fmt.Errorf("Failed to load plugin project '%s': %w", name, err)
			}

			var installReady pdag.Done
			specIsInstalled, installReady = dag.NewNode(installPluginAtPathStep{
				plugin: pluginProject,
				path:   spec.Source,
			})
			defer installReady() // Make sure we don't leak this
			err = ensureProjectDependencies(ctx, dag, pluginProject, spec.Source, specIsInstalled)
			if err != nil {
				return err
			}
			installReady() // We have added our dependencies, so install is now safe to run.
			if err := dag.NewEdge(specIsInstalled, root); err != nil {
				return err
			}
		} else if isFileBasedSource(spec.Source) {
			var ready pdag.Done
			specIsInstalled, ready = dag.NewNode(noOpStep{})
			ready()
		} else {
			// spec needs to be downloaded
			pluginSpec, err := workspace.NewPluginSpec(ctx, spec.Source, apitype.ResourcePlugin,
				nil /* version */, "" /* pluginDownloadURL */, nil /* checksum */)
			if err != nil {
				return fmt.Errorf("invalid plugin spec '%s' for '%s': %w", spec.Source, name, err)
			}

			var ready pdag.Done
			specIsInstalled, ready = dag.NewNode(noOpStep{})
			ensureSpec(dag, &pluginSpec, specIsInstalled)
			ready()
		}

		// After specIsInstalled, spec.Source will be available on the local file
		// system. An SDK needs to be generated and linked into project.

		linked, linkReady := dag.NewNode(genAndLinkInstalledPackageSpecStep{
			projectDir: projectDir,
			project:    project,
			spec:       spec,
		})
		err := dag.NewEdge(specIsInstalled, linked) // Don't run the link step before the plugin is installed
		contract.AssertNoErrorf(err, "new nodes can't have cycles")
		linkReady()
		err = dag.NewEdge(linked, root) // root isn't done until spec is linked
		contract.AssertNoErrorf(err, "new nodes can't have cycles")
	}

	return nil
}

// Launch an already downloaded plugin, maybe parameterize it, generate a schema and link
// in the local SDK.
type genAndLinkInstalledPackageSpecStep struct {
	projectDir string                // The project directory that we are linking spec into.
	project    workspace.BaseProject // The project that we are linking spec into.
	spec       workspace.PackageSpec // The spec we are linking into the project.
}

func (step genAndLinkInstalledPackageSpecStep) run(ctx context.Context) error {
	panic(`TODO: We need to call packages.InstallPackage here (at least by
	functionality), but we can't link it in as is because packages.InstallPackage does
	it's own plugin downloading (and thus depends on this package).

	The next step is to refactor out a version of InstallPackage and it's dependents
	(SchemaFromSchemaSource, GenSDK and LinkPackage) to a separate package that
	doesn't handle installation, only loading.

	I'm starting to think we will need to make this a bigger refactor: 3 new packages:

		pkg/pulumipackage/
			install (this package; depends on link)
			link (links already installed plugins into workspaces; depends on gensdk)
			gensdk (packages.GenSDK)

	We can wrap the complexity of "pkg/pulumipackage/install" with wrapper functions in "pkg/pulumipackage":

		func GetSchema(
			ctx context.Context, packageSource string, parameters plugin.ParameterizeParameters,
			registry registry.Registry, host *plugin.Host,
		)

		func GetProvider(
			ctx context.Context, packageSource string, parameters plugin.ParameterizeParameters,
			registry registry.Registry, host *plugin.Host,
		)

		func GenSDK(
			ctx context.Context, packageSource string, parameters plugin.ParameterizeParameters,
			registry registry.Registry, host *plugin.Host,

			language, outDir, overlays string, local bool,
		)
`)
}

func isFileBasedSource(source string) bool {
	switch filepath.Ext(source) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func downloadPluginFromSpecToTarball(ctx context.Context, spec *workspace.PluginSpec) (PluginContent, error) {
	contract.Assertf(spec != nil, "spec is passed by reference to allow mutation")
	util.SetKnownPluginDownloadURL(spec)
	util.SetKnownPluginVersion(spec)
	if spec.Version == nil {
		var err error
		spec.Version, err = spec.GetLatestVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not find latest version for provider %s: %w", spec.Name, err)
		}
	}

	wrapper := func(stream io.ReadCloser, size int64) io.ReadCloser {
		// Log at info but to stderr so we don't pollute stdout for commands like `package get-schema`
		// TODO: log(diag.Infoerr, "Downloading provider: "+spec.Name)
		return stream
	}

	retry := func(err error, attempt int, limit int, delay time.Duration) {
		// TODO:
		// log(diag.Warning, fmt.Sprintf("error downloading provider: %s\n"+
		// 	"Will retry in %v [%d/%d]", err, delay, attempt, limit))
	}

	logging.V(1).Infof("Automatically downloading provider %s", spec.Name)
	downloadedFile, err := workspace.DownloadToFile(ctx, *spec, wrapper, retry)
	if err != nil {
		return nil, &InstallPluginError{
			Spec: *spec,
			Err:  fmt.Errorf("error downloading provider %s to file: %w", spec.Name, err),
		}
	}

	return SingleFilePlugin(downloadedFile, *spec), nil
}

func unpackTarball(_ context.Context, spec workspace.PluginSpec, content PluginContent, reinstall bool) error {
	defer contract.IgnoreClose(content)

	// Fetch the directory into which we will expand this tarball.
	finalDir, err := spec.DirPath()
	if err != nil {
		return err
	}

	// Cleanup any temp dirs from failed installations of this plugin from previous versions of Pulumi.
	if err := cleanupTempDirs(finalDir); err != nil {
		// We don't want to fail the installation if there was an error cleaning up these old temp dirs.
		// Instead, log the error and continue on.
		logging.V(5).Infof("Install: Error cleaning up temp dirs: %s", err)
	}

	// Get the partial file path (e.g. <pluginsdir>/<kind>-<name>-<version>.partial).
	partialFilePath, err := spec.PartialFilePath()
	if err != nil {
		return err
	}

	// Check whether the directory exists while we were waiting on the lock.
	_, finalDirStatErr := os.Stat(finalDir)
	if finalDirStatErr == nil {
		_, partialFileStatErr := os.Stat(partialFilePath)
		if partialFileStatErr != nil {
			if !os.IsNotExist(partialFileStatErr) {
				return partialFileStatErr
			}
			if !reinstall {
				// finalDir exists, there's no partial file, and we're not reinstalling, so the plugin is already
				// installed.
				return nil
			}
		}

		// Either the partial file exists--meaning a previous attempt at installing the plugin failed--or we're
		// deliberately reinstalling the plugin. Delete finalDir so we can try installing again. There's no need to
		// delete the partial file since we'd just be recreating it again below anyway.
		if err := os.RemoveAll(finalDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(finalDirStatErr) {
		return finalDirStatErr
	}

	// Create an empty partial file to indicate installation is in-progress.
	if err := os.WriteFile(partialFilePath, nil, 0o600); err != nil {
		return err
	}

	// Create the final directory.
	if err := os.MkdirAll(finalDir, 0o700); err != nil {
		return err
	}

	if err := content.writeToDir(finalDir); err != nil {
		return err
	}

	// Even though we deferred closing the tarball at the beginning of this function, go ahead and explicitly close
	// it now since we're finished extracting it, to prevent subsequent output from being displayed oddly with
	// the progress bar.
	contract.IgnoreClose(content)

	// Installation is complete. Remove the partial file.
	return os.Remove(partialFilePath)
}
