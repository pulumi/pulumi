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

package config

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type localConfigStack struct {
	reference localConfigReference
}

type localConfigReference struct {
	name      tokens.StackName
	project   tokens.Name
	qualified string
}

func (r localConfigReference) String() string                   { return r.qualified }
func (r localConfigReference) Name() tokens.StackName           { return r.name }
func (r localConfigReference) Project() (tokens.Name, bool)     { return r.project, true }
func (r localConfigReference) FullyQualifiedName() tokens.QName { return tokens.QName(r.qualified) }

func (s *localConfigStack) Ref() backend.StackReference { return s.reference }
func (s *localConfigStack) ConfigLocation() backend.StackConfigLocation {
	return backend.StackConfigLocation{}
}
func (s *localConfigStack) Backend() backend.Backend              { return nil }
func (s *localConfigStack) Tags() map[apitype.StackTagName]string { return nil }

var errLocalConfigOnly = errors.New("operation requires an authenticated stack")

func (s *localConfigStack) LoadRemoteConfig(context.Context, *workspace.Project) (*workspace.ProjectStack, error) {
	return nil, errLocalConfigOnly
}

func (s *localConfigStack) SaveRemoteConfig(context.Context, *workspace.ProjectStack) error {
	return errLocalConfigOnly
}

func (s *localConfigStack) Snapshot(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
	return nil, errLocalConfigOnly
}

func (s *localConfigStack) SnapshotStackOutputs(context.Context, secrets.Provider) (property.Map, error) {
	return property.Map{}, errLocalConfigOnly
}

func (s *localConfigStack) DefaultSecretManager(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
	return nil, errLocalConfigOnly
}

// requireConfigStack resolves local configuration for an unauthenticated agent when the operation needs no backend.
func requireConfigStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	stackName string, lopt cmdStack.LoadOption, opts display.Options, configFile string,
	localAllowed func(*workspace.ProjectStack) bool,
) (backend.Stack, error) {
	stack, _, err := loadLocalConfigStack(ctx, sink, ws, stackName, configFile, localAllowed)
	if err != nil || stack != nil {
		return stack, err
	}
	return cmdStack.RequireStack(ctx, sink, ws, lm, stackName, lopt, opts, configFile)
}

func loadLocalConfigStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName, configFile string,
	localAllowed func(*workspace.ProjectStack) bool,
) (*localConfigStack, *workspace.ProjectStack, error) {
	if localAllowed == nil || agentdetect.Detect(os.Getenv) == "" || os.Getenv("PULUMI_ACCESS_TOKEN") != "" {
		return nil, nil, nil
	}
	project, _, err := ws.ReadProject("")
	if err != nil {
		return nil, nil, err
	}
	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
	if err != nil {
		return nil, nil, err
	}
	if diy.IsDIYBackendURL(url) {
		return nil, nil, nil
	}
	url = httpstate.ValueOrDefaultURL(ws, url)
	if stackName == "" {
		if value, ok := os.LookupEnv("PULUMI_STACK"); ok {
			stackName = value
		} else {
			w, err := ws.New("")
			if err != nil {
				return nil, nil, err
			}
			stackName, _ = w.Settings().StackForBackend(url)
		}
	}
	parts := strings.Split(stackName, "/")
	if len(parts) > 3 || (len(parts) == 3 && parts[1] != string(project.Name)) {
		return nil, nil, nil
	}
	if slices.Contains(parts, "") {
		return nil, nil, nil
	}
	name, err := tokens.ParseStackName(parts[len(parts)-1])
	if err != nil {
		return nil, nil, nil
	}
	if configFile == "" {
		_, configFile, err = workspace.DetectProjectStackPath(name.Q())
		if errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
	}
	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, err
	}
	ps, err := workspace.LoadProjectStack(sink, project, configFile)
	if err != nil {
		return nil, nil, err
	}
	if !localAllowed(ps) {
		return nil, nil, nil
	}
	account, _, err := workspace.GetAccountWithAgentFallback(url)
	if err != nil {
		return nil, nil, err
	}
	if account.HasCredential() {
		// Expired credentials must not send a local operation through Login and create a new agent account.
		current, err := httpstate.NewLoginManager().Current(ctx, url, account.Insecure, false)
		if err != nil {
			return nil, nil, err
		}
		if current != nil {
			return nil, nil, nil
		}
	}
	return &localConfigStack{reference: localConfigReference{
		name: name, project: tokens.Name(project.Name), qualified: stackName,
	}}, ps, nil
}
