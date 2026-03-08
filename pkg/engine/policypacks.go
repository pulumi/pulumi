// Copyright 2026, Pulumi Corporation.
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

package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// installPolicyPack downloads and installs a policy pack with progress reporting.
func installPolicyPack(
	ctx context.Context,
	plugctx *plugin.Context,
	opts *deploymentOptions,
	policy RequiredPolicy,
) error {
	policyID := fmt.Sprintf("%s@v%s", policy.Name(), policy.Version())
	logging.V(preparePluginLog).Infof("installPolicyPack(%s): beginning install", policyID)

	// Check if already installed
	if policy.Installed() {
		logging.V(preparePluginLog).Infof("installPolicyPack(%s): already installed", policyID)
		return nil
	}

	downloadMessage := "Downloading policy pack " + policyID
	installMessage := "Installing policy pack " + policyID

	// We want to report download progress so that users are not left wondering
	// if their program has hung. To do this we wrap the downloading ReadCloser
	// with one that observes the bytes read and renders a progress bar in some
	// fashion. If we have an event emitter available, we'll use that to report
	// progress by publishing progress events. If not, we'll wrap with a
	// ReadCloser that renders progress directly to the console itself.
	var withDownloadProgress func(io.ReadCloser, int64) io.ReadCloser
	if opts == nil {
		withDownloadProgress = func(stream io.ReadCloser, size int64) io.ReadCloser {
			return workspace.ReadCloserProgressBar(
				stream,
				size,
				downloadMessage,
				cmdutil.GetGlobalColorization(),
			)
		}
	} else {
		withDownloadProgress = func(stream io.ReadCloser, size int64) io.ReadCloser {
			return NewProgressReportingCloser(
				opts.Events,
				PolicyPackDownload,
				string(PolicyPackDownload)+":"+policyID,
				downloadMessage,
				size,
				100*time.Millisecond, /*reportingInterval */
				stream,
			)
		}
	}

	logging.V(preparePluginVerboseLog).Infof("installPolicyPack(%s): initiating download", policyID)

	downloadStream, size, err := policy.Download(ctx, withDownloadProgress)
	if err != nil {
		return fmt.Errorf("failed to download policy pack %s: %w", policyID, err)
	}

	// Download the tarball to a temp file. This completes the download phase
	// (triggering download progress events and Done on close) before the
	// install phase begins, matching the plugin download/install pattern.
	tmpFile, err := os.CreateTemp("" /* default temp dir */, "pulumi-policypack-tar")
	if err != nil {
		contract.IgnoreClose(downloadStream)
		return fmt.Errorf("failed to download policy pack %s: %w", policyID, err)
	}
	defer func() {
		contract.IgnoreClose(tmpFile)
		contract.IgnoreError(os.Remove(tmpFile.Name()))
	}()

	if _, err := io.Copy(tmpFile, downloadStream); err != nil {
		contract.IgnoreClose(downloadStream)
		return fmt.Errorf("failed to download policy pack %s: %w", policyID, err)
	}
	// Close the download stream to emit its Done progress event.
	contract.IgnoreClose(downloadStream)

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to download policy pack %s: %w", policyID, err)
	}

	logging.V(preparePluginVerboseLog).Infof(
		"installPolicyPack(%s): extracting tarball to installation directory", policyID)

	// In a similar manner to downloads, we'll use a progress bar to show
	// install progress by wrapping the tarball file with a progress reporting
	// ReadCloser where possible.
	var installStream io.ReadCloser
	if opts == nil || size == 0 {
		installStream = tmpFile
		fmt.Fprintf(os.Stderr, "Installing policy pack %s...\n", policyID)
	} else {
		installStream = NewProgressReportingCloser(
			opts.Events,
			PolicyPackInstall,
			string(PolicyPackInstall)+":"+policyID,
			installMessage,
			size,
			100*time.Millisecond, /*reportingInterval */
			tmpFile,
		)
		defer contract.IgnoreClose(installStream)
	}

	// Install the policy pack (extract tarball + install dependencies). If we
	// have an event emitter, wrap the dependency output writers so that a
	// "Installing policy pack X dependencies..." message is shown during
	// dependency installation (emitted on first write, dismissed on Done).
	if opts == nil {
		var buf bytes.Buffer
		depWriter := &lockedWriter{w: &buf}
		if err := policy.Install(plugctx, installStream, depWriter, depWriter); err != nil {
			return fmt.Errorf("failed to install policy pack %s: %w\n\nDependency installation output:\n%s",
				policyID, err, buf.String())
		}
	} else {
		depID := string(PolicyPackInstall) + ":deps:" + policyID
		depMessage := "Installing policy pack " + policyID + " dependencies..."
		depWriter := NewProgressEventWriter(
			opts.Events,
			PolicyPackInstall,
			depID,
			depMessage,
		)
		defer depWriter.Done()

		if err := policy.Install(plugctx, installStream, depWriter, depWriter); err != nil {
			return fmt.Errorf("failed to install policy pack %s: %w\n\nDependency installation output:\n%s",
				policyID, err, depWriter.Output())
		}
	}

	logging.V(preparePluginLog).Infof("installPolicyPack(%s): installation complete", policyID)
	return nil
}

// EnsurePoliciesAreInstalled ensures that all of the given policy packs are installed,
// using the provided errgroup for parallel installation. If installTasks is nil,
// a new errgroup is created and waited on before returning.
func EnsurePoliciesAreInstalled(
	ctx context.Context,
	plugctx *plugin.Context,
	opts *deploymentOptions,
	policies []RequiredPolicy,
) error {
	manager := newInstallManager(false)
	ensurePoliciesAreInstalled(ctx, plugctx, opts, policies, manager)
	return manager.Wait()
}

func ensurePoliciesAreInstalled(
	ctx context.Context,
	plugctx *plugin.Context,
	opts *deploymentOptions,
	policies []RequiredPolicy,
	manager *installManager,
) {
	logging.V(preparePluginLog).Infof("ensurePoliciesAreInstalled(): beginning, %d policies", len(policies))
	for _, policy := range policies {
		manager.InstallPolicyPack(func() error {
			return installPolicyPack(ctx, plugctx, opts, policy)
		})
	}
	logging.V(preparePluginLog).Infof("ensurePoliciesAreInstalled(): completed")
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}
