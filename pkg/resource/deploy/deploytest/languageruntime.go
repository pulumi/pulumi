// Copyright 2016-2024, Pulumi Corporation.
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

package deploytest

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var ErrLanguageRuntimeIsClosed = errors.New("language runtime is shutting down")

type LanguageRuntimeFactory func() plugin.LanguageRuntime

type ProgramFunc func(runInfo plugin.RunInfo, monitor *ResourceMonitor) error

func NewLanguageRuntimeF(program ProgramFunc, requiredPackages ...workspace.PackageDescriptor) LanguageRuntimeFactory {
	return func() plugin.LanguageRuntime {
		return NewLanguageRuntime(program, requiredPackages...)
	}
}

func NewLanguageRuntime(program ProgramFunc, requiredPackages ...workspace.PackageDescriptor) plugin.LanguageRuntime {
	return &languageRuntime{
		requiredPackages: requiredPackages,
		program:          program,
	}
}

type languageRuntime struct {
	requiredPackages []workspace.PackageDescriptor
	program          ProgramFunc
	closed           bool
}

func (p *languageRuntime) Close() error {
	p.closed = true
	return nil
}

func (p *languageRuntime) GetRequiredPackages(info plugin.ProgramInfo) ([]workspace.PackageDescriptor, error) {
	if p.closed {
		return nil, ErrLanguageRuntimeIsClosed
	}
	return p.requiredPackages, nil
}

func (p *languageRuntime) Run(info plugin.RunInfo) (string, bool, error) {
	if p.closed {
		return "", false, ErrLanguageRuntimeIsClosed
	}
	monitor, err := dialMonitor(context.Background(), info.MonitorAddress)
	if err != nil {
		return "", false, err
	}
	defer contract.IgnoreClose(monitor)

	// Run the program.
	done := make(chan error)
	go func() {
		done <- p.program(info, monitor)
	}()
	if progerr := <-done; progerr != nil {
		return progerr.Error(), false, nil
	}
	return "", false, nil
}

func (p *languageRuntime) GetPluginInfo() (workspace.PluginInfo, error) {
	if p.closed {
		return workspace.PluginInfo{}, ErrLanguageRuntimeIsClosed
	}
	return workspace.PluginInfo{Name: "TestLanguage"}, nil
}

func (p *languageRuntime) InstallDependencies(
	plugin.InstallDependenciesRequest,
) (io.Reader, io.Reader, <-chan error, error) {
	if p.closed {
		return nil, nil, nil, ErrLanguageRuntimeIsClosed
	}

	// We'll return default readers for stdout and stderr, as well as a closed channel to signal that the installation
	// is complete and that anyone blocking on it can proceed immediately.
	stdout := strings.NewReader("")
	stderr := strings.NewReader("")

	done := make(chan error)
	close(done)

	return stdout, stderr, done, nil
}

func (p *languageRuntime) RuntimeOptionsPrompts(info plugin.ProgramInfo) ([]plugin.RuntimeOptionPrompt, error) {
	if p.closed {
		return []plugin.RuntimeOptionPrompt{}, ErrLanguageRuntimeIsClosed
	}
	return []plugin.RuntimeOptionPrompt{}, nil
}

func (p *languageRuntime) About(info plugin.ProgramInfo) (plugin.AboutInfo, error) {
	if p.closed {
		return plugin.AboutInfo{}, ErrLanguageRuntimeIsClosed
	}
	return plugin.AboutInfo{}, nil
}

func (p *languageRuntime) GetProgramDependencies(
	info plugin.ProgramInfo, transitiveDependencies bool,
) ([]plugin.DependencyInfo, error) {
	if p.closed {
		return nil, ErrLanguageRuntimeIsClosed
	}
	return nil, nil
}

func (p *languageRuntime) RunPlugin(ctx context.Context, info plugin.RunPluginInfo) (
	io.Reader, io.Reader, *promise.Promise[int32], error,
) {
	return nil, nil, nil, errors.New("inline plugins are not currently supported")
}

func (p *languageRuntime) GenerateProject(string, string, string,
	bool, string, map[string]string,
) (hcl.Diagnostics, error) {
	return nil, errors.New("GenerateProject is not supported")
}

func (p *languageRuntime) GeneratePackage(
	string, string, map[string][]byte, string, map[string]string, bool,
) (hcl.Diagnostics, error) {
	return nil, errors.New("GeneratePackage is not supported")
}

func (p *languageRuntime) GenerateProgram(map[string]string, string, bool) (map[string][]byte, hcl.Diagnostics, error) {
	return nil, nil, errors.New("GenerateProgram is not supported")
}

func (p *languageRuntime) Pack(string, string) (string, error) {
	return "", errors.New("Pack is not supported")
}
