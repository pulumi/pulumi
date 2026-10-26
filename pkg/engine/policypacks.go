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
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/progress"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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

	downloadMessage := "Downloading policy pack " + policyID

	// We want to report download progress so that users are not left wondering
	// if their program has hung. To do this we wrap the downloading ReadCloser
	// with one that observes the bytes read and renders a progress bar in some
	// fashion. If we have an event emitter available, we'll use that to report
	// progress by publishing progress events. If not, we'll wrap with a
	// ReadCloser that renders progress directly to the console itself.
	var withDownloadProgress func(io.ReadCloser, int64) io.ReadCloser
	if opts == nil {
		withDownloadProgress = func(stream io.ReadCloser, size int64) io.ReadCloser {
			return progress.Stderr().Wrap(stream, size, downloadMessage, cmdutil.GetGlobalColorization())
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

	if opts == nil {
		var buf bytes.Buffer
		depWriter := &lockedWriter{w: &buf}
		if err := policy.EnsureInstalled(plugctx, withDownloadProgress, depWriter); err != nil {
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

		if err := policy.EnsureInstalled(plugctx, withDownloadProgress, depWriter); err != nil {
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
