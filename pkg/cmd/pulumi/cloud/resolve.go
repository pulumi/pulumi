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

package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ResolvedContext carries the resolved Pulumi Cloud context for an API call:
// a Client (authenticated when LoggedIn is true, anonymous otherwise), the
// cloud URL it targets, the resolved default org (best-effort: from the
// local default, the backend's opinion, or empty), the current Pulumi
// project (nil outside a project directory), the (org, project, stack) of
// the currently-selected stack (any/all empty when no stack is selected),
// and a LoggedIn flag callers can use to decide whether to require
// credentials.
//
// Always returns a usable Client + CloudURL so commands that hit public
// endpoints (e.g. fetching the OpenAPI spec) work without a login.
type ResolvedContext struct {
	Client    *client.Client
	CloudURL  string
	OrgName   string
	Project   *workspace.Project
	StackOrg  string
	StackProj string
	StackName string
	LoggedIn  bool
}

// ResolveContext returns the Pulumi Cloud context for a `pulumi api`
// invocation. The resolved OrgName comes from pkgBackend.GetDefaultOrg
// (which prefers a locally-configured default and falls back to the
// backend's opinion) so {orgName} template vars resolve sensibly even
// outside a project directory.
//
// Credential lookup is non-interactive: when no credentials are stored,
// ResolveContext returns an anonymous Client + the resolved CloudURL with
// LoggedIn=false rather than prompting or failing. Callers that require
// authentication should check LoggedIn and surface their own error.
func ResolveContext(ctx context.Context) (*ResolvedContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current working directory: %w", err)
	}

	ws, lm := pkgWorkspace.Instance, cmdBackend.DefaultLoginManager
	project, _, err := ws.ReadProject(cwd)
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, fmt.Errorf("reading project: %w", err)
	}

	cloudURL, _, err := cmdBackend.PrepareCurrentBackend(ctx, lm.Session(), ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("preparing backend: %w", err)
	}
	if diy.IsDIYBackendURL(cloudURL) {
		return nil, errors.New("`pulumi api` requires the Pulumi Cloud backend; run `pulumi login`")
	}

	be, err := lm.Current(ctx, ws, cmdutil.Diag(), cloudURL, project, false)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	if be == nil {
		insecure := pkgWorkspace.GetCloudInsecure(ws, cloudURL)
		apiClient := client.NewClient(cloudURL, "", insecure, cmdutil.Diag()).
			WithHTTPAuth(lm.Session().HTTPAuth(cloudURL))
		return &ResolvedContext{Client: apiClient, CloudURL: cloudURL, Project: project}, nil
	}

	cloudBe, ok := be.(httpstate.Backend)
	if !ok {
		return nil, errors.New("`pulumi api` requires the Pulumi Cloud backend; " +
			"run `pulumi login`")
	}

	orgName, err := be.GetDefaultOrg(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolving default org: %w", err)
	}

	stackOrg, stackProj, stackName := currentStackSelection(ctx, ws, be)

	return &ResolvedContext{
		Client:    cloudBe.Client(),
		CloudURL:  cloudBe.CloudURL(),
		OrgName:   orgName,
		Project:   project,
		StackOrg:  stackOrg,
		StackProj: stackProj,
		StackName: stackName,
		LoggedIn:  true,
	}, nil
}

// currentStackSelection returns the (org, project, stack) of the user's
// currently-selected stack via state.CurrentStack. Returns empty strings
// when no stack is selected or the lookup fails (e.g. the ref points at
// a deleted stack). Caller must have already verified be is an
// httpstate.Backend; the stack is unwrapped to httpstate.Stack to read
// OrgName.
func currentStackSelection(
	ctx context.Context, ws pkgWorkspace.Context, be pkgBackend.Backend,
) (org, project, stack string) {
	s, err := state.CurrentStack(ctx, ws, be)
	if err != nil || s == nil {
		return "", "", ""
	}
	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return "", "", ""
	}
	ref := cloudStack.Ref()
	stack = ref.Name().String()
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	org = cloudStack.OrgName()
	return org, project, stack
}
