// Copyright 2016-2018, Pulumi Corporation.
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
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type ProgramFunc func(runInfo plugin.RunInfo, monitor *ResourceMonitor) error

func NewLanguageRuntime(program ProgramFunc, requiredPlugins ...workspace.PluginInfo) plugin.LanguageRuntime {
	return &languageRuntime{
		requiredPlugins: requiredPlugins,
		program:         program,
	}
}

type languageRuntime struct {
	requiredPlugins []workspace.PluginInfo
	program         ProgramFunc
}

func (p *languageRuntime) Close() error {
	return nil
}

func (p *languageRuntime) GetRequiredPlugins(info plugin.ProgInfo) ([]workspace.PluginInfo, error) {
	return p.requiredPlugins, nil
}

func (p *languageRuntime) Run(info plugin.RunInfo) (string, bool, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.Dial(info.MonitorAddress, grpc.WithInsecure())
	if err != nil {
		return "", false, errors.Wrapf(err, "could not connect to resource monitor")
	}
	defer contract.IgnoreClose(conn)

	// Fire up a resource monitor client
	resmon := pulumirpc.NewResourceMonitorClient(conn)

	// Run the program.
	done := make(chan error)
	go func() {
		done <- p.program(info, &ResourceMonitor{resmon: resmon})
	}()
	if progerr := <-done; progerr != nil {
		return progerr.Error(), false, nil
	}
	return "", false, nil
}

func (p *languageRuntime) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{Name: "TestLanguage"}, nil
}
