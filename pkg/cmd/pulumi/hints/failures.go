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

package hints

import (
	"context"
	"log/slog"
	"os"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// WithFailureRecorder returns a context whose provider calls report the hints for their failures, such as dynamic
// credentials, to this process's hints.
func WithFailureRecorder(ctx context.Context) context.Context {
	return plugin.WithHintRecorder(ctx, defaultManager)
}

// OfferFailureHints offers hints for the failures that providers reported during this command. It is
// called when the command fails.
func OfferFailureHints() { defaultManager.offerFailureHints(isCloudBackend) }

// RecordHint implements plugin.HintRecorder.
func (m *manager) RecordHint(pkg tokens.Package, hint plugin.Hint) {
	if hint != plugin.HintDynamicCredentials {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.dynamicCredentialsHints, pkg) {
		m.dynamicCredentialsHints = append(m.dynamicCredentialsHints, pkg)
	}
}

func (m *manager) offerFailureHints(isCloud func() bool) {
	m.mu.Lock()
	failures := slices.Clone(m.dynamicCredentialsHints)
	m.mu.Unlock()

	for _, pkg := range failures {
		cloud, ok := escCloudForPackage[pkg]
		if !ok {
			continue
		}
		// ESC needs Pulumi Cloud, so only suggest it to Pulumi Cloud users.
		if !isCloud() {
			slog.Info("not suggesting ESC: the backend is not Pulumi Cloud", "package", pkg)
			return
		}
		m.offer(ESCCredentials(cloud.name, cloud.setup))
		return
	}
}

// isCloudBackend reports whether the current project uses a Pulumi Cloud backend, without contacting it.
func isCloudBackend() bool {
	ws := pkgWorkspace.Instance
	// A project may configure its own backend. Without one, the user's current login applies.
	var proj *workspace.Project
	if cwd, err := os.Getwd(); err == nil {
		if p, _, err := ws.ReadProject(cwd); err == nil {
			proj = p
		}
	}
	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), proj)
	if err != nil {
		slog.Info("could not determine backend for hints", "err", err)
		return false
	}
	return url != "" && !diy.IsDIYBackendURL(url)
}
