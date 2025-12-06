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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func New() PluginManager { return pluginManager{} }

type PluginManager interface {
	EnsureSpec(ctx context.Context, spec workspace.PluginSpec) (string, error)
	EnsureInProject(ctx context.Context, plugin workspace.BaseProject) error
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

type noOpStep struct{}

func (noOpStep) run(context.Context) error { return nil }

func ensureSpec(dag *pdag.DAG[step], spec *workspace.PluginSpec, root pdag.Node) {
	specInstall, specInstallReady := dag.NewNode(installSpecAfterDependencies{
		dag:      dag,
		depsRoot: root,
		spec:     spec,
	})

	downloadSpec, downloadSpecReady := dag.NewNode(downloadAndUnpackSpecStep{
		dag:       dag,
		depsRoot:  specInstall,
		depsAdded: specInstallReady,
		spec:      spec, // We allow spec to be mutated, updating its version as necessary
	})
	contract.AssertNoErrorf(dag.NewEdge(downloadSpec, specInstall), "a new edge is always a-cyclic")
	downloadSpecReady()
}

type installSpecAfterDependencies struct {
	dag      *pdag.DAG[step]
	depsRoot pdag.Node             // The node that newly discovered dependencies should target
	spec     *workspace.PluginSpec // The spec to run the install in
}

// run the install for a spec. We can assume that all local dependencies have already been
// installed.
func (step installSpecAfterDependencies) run(ctx context.Context) error {
	panic("TODO")
}

type downloadAndUnpackSpecStep struct {
	dag       *pdag.DAG[step]
	depsRoot  pdag.Node             // The node that newly discovered dependencies should target
	depsAdded pdag.Done             // Indicate that all dependencies have been discovered and added
	spec      *workspace.PluginSpec // The spec to download and unpack
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

	for _, spec := range pluginProject.GetPackageSpecs() {
		// TODO: Create specInstalled nodes for package specs, then link them to step.depsRoot
		panic(spec)
	}

	return nil
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

func unpackTarball(ctx context.Context, spec workspace.PluginSpec, content PluginContent, reinstall bool) error {
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

func (pluginManager) EnsureInProject(ctx context.Context, plugin workspace.BaseProject) error {
	panic("TODO")
}
