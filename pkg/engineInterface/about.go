// Copyright 2016-2021, Pulumi Corporation.
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

package engineInterface

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/shared"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	enginerpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"
)

func getAbout(ctx context.Context, transitiveDependencies bool,
	selectedStack string) (*enginerpc.AboutResponse, error) {

	response := &enginerpc.AboutResponse{}
	var err error
	getCLIAbout(response)

	addError := func(err error, message string) {
		err = fmt.Errorf("%s: %w", message, err)
		response.Errors = append(response.Errors, err.Error())
	}

	var proj *workspace.Project
	var pwd string
	if proj, pwd, err = shared.ReadProject(); err != nil {
		addError(err, "Failed to read project")
	} else {
		projinfo := &engine.Projinfo{Proj: proj, Root: pwd}
		pwd, program, pluginContext, err := engine.ProjectInfoContext(
			projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil)
		if err != nil {
			addError(err, "Failed to create plugin context")
		} else {
			defer pluginContext.Close()

			// Only try to get project plugins if we managed to read a project
			if plugins, err := getPluginsAbout(pluginContext, proj, pwd, program); err != nil {
				addError(err, "Failed to get information about the plugin")
			} else {
				response.Plugins = plugins
			}

			response.Runtime = proj.Runtime.Name()
			lang, err := pluginContext.Host.LanguageRuntime(response.Runtime)
			if err != nil {
				addError(err, fmt.Sprintf("Failed to load language plugin %s", proj.Runtime.Name()))
			} else {
				aboutResponse, err := lang.About()
				if err != nil {
					addError(err, "Failed to get information about the project runtime")
				} else {
					response.Language = &pulumirpc.AboutResponse{
						Executable: aboutResponse.Executable,
						Version:    aboutResponse.Version,
						Metadata:   aboutResponse.Metadata,
					}
				}

				progInfo := plugin.ProgInfo{Proj: proj, Pwd: pwd, Program: program}
				deps, err := lang.GetProgramDependencies(progInfo, transitiveDependencies)
				if err != nil {
					addError(err, "Failed to get information about the Pulumi program's dependencies")
				} else {
					response.Dependencies = make([]*pulumirpc.DependencyInfo, len(deps))
					for i, dep := range deps {
						response.Dependencies[i] = &pulumirpc.DependencyInfo{
							Name:    dep.Name,
							Version: dep.Version.String(),
						}
					}
				}
			}
		}
	}

	var backend backend.Backend
	backend, err = shared.NonInteractiveCurrentBackend(ctx)
	if err != nil {
		addError(err, "Could not access the backend")
	} else if backend != nil {
		var stack *enginerpc.AboutStack
		if stack, err = getCurrentStackAbout(ctx, backend, selectedStack); err != nil {
			addError(err, "Failed to get information about the current stack")
		} else {
			response.Stack = stack
		}

		tmp := getBackendAbout(backend)
		response.Backend = &tmp
	}

	return response, nil
}

func getPluginsAbout(ctx *plugin.Context, proj *workspace.Project,
	pwd, main string) ([]*pulumirpc.PluginDependency, error) {

	pluginSpec, err := getProjectPluginsSilently(ctx, proj, pwd, main)
	if err != nil {
		return nil, err
	}
	sort.Slice(pluginSpec, func(i, j int) bool {
		pi, pj := pluginSpec[i], pluginSpec[j]
		if pi.Name < pj.Name {
			return true
		} else if pi.Name == pj.Name && pi.Kind == pj.Kind &&
			(pi.Version == nil || (pj.Version != nil && pi.Version.GT(*pj.Version))) {
			return true
		}
		return false
	})

	var plugins = make([]*pulumirpc.PluginDependency, len(pluginSpec))
	for i, p := range pluginSpec {
		var version string
		if p.Version != nil {
			version = p.Version.String()
		}

		plugins[i] = &pulumirpc.PluginDependency{
			Name:    p.Name,
			Version: version,
		}
	}
	return plugins, nil
}

func getBackendAbout(b backend.Backend) enginerpc.AboutBackend {
	currentUser, currentOrgs, err := b.CurrentUser()
	if err != nil {
		currentUser = "Unknown"
	}
	return enginerpc.AboutBackend{
		Name:          b.Name(),
		Url:           b.URL(),
		User:          currentUser,
		Organizations: currentOrgs,
	}
}

func getCurrentStackAbout(ctx context.Context, b backend.Backend, selectedStack string) (*enginerpc.AboutStack, error) {
	var stack backend.Stack
	var err error
	if selectedStack == "" {
		stack, err = state.CurrentStack(ctx, b)
	} else {
		var ref backend.StackReference
		ref, err = b.ParseStackReference(selectedStack)
		if err != nil {
			return nil, err
		}
		stack, err = b.GetStack(ctx, ref)
	}
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("No current stack")
	}

	name := stack.Ref().String()
	var snapshot *deploy.Snapshot
	snapshot, err = stack.Snapshot(ctx)
	if err != nil {
		return nil, err
	} else if snapshot == nil {
		return nil, errors.New("No current snapshot")
	}
	var resources []*resource.State = snapshot.Resources
	var pendingOps []resource.Operation = snapshot.PendingOperations

	var aboutResources = make([]*enginerpc.AboutState, len(resources))
	for i, r := range resources {
		aboutResources[i] = &enginerpc.AboutState{
			Type: string(r.Type),
			Urn:  string(r.URN),
		}
	}
	var aboutPending = make([]*enginerpc.AboutState, len(pendingOps))
	for i, p := range pendingOps {
		aboutPending[i] = &enginerpc.AboutState{
			Type: string(p.Type),
			Urn:  string(p.Resource.URN),
		}
	}
	return &enginerpc.AboutStack{
		Name:              name,
		Resources:         aboutResources,
		PendingOperations: aboutPending,
	}, nil
}

func getCLIAbout(response *enginerpc.AboutResponse) {
	// Version is not supplied in test builds.
	ver, err := semver.ParseTolerant(version.Version)
	if err == nil {
		// To get semver formatting when possible
		response.Version = ver.String()
	} else {
		response.Version = version.Version
	}
	response.GoVersion = runtime.Version()
	response.GoCompiler = runtime.Compiler
}

// This is necessary because dotnet invokes build during the call to
// getProjectPlugins.
func getProjectPluginsSilently(
	ctx *plugin.Context, proj *workspace.Project, pwd, main string) ([]workspace.PluginSpec, error) {
	_, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	os.Stdout = w

	return plugin.GetRequiredPlugins(ctx.Host, plugin.ProgInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
	}, plugin.AllPlugins)
}
