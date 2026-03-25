// Copyright 2016, Pulumi Corporation.
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

package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type observedStep struct {
	Path         string
	Dependencies []string
	HasOnError   bool
}

type monitorServer struct {
	pulumirpc.UnimplementedGraphMonitorServer

	mu    sync.Mutex
	steps []observedStep
}

func (m *monitorServer) RegisterGraph(
	context.Context, *pulumirpc.RegisterGraphRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *monitorServer) RegisterTrigger(
	context.Context, *pulumirpc.RegisterTriggerRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *monitorServer) RegisterSensor(
	context.Context, *pulumirpc.RegisterSensorRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *monitorServer) RegisterJob(
	context.Context, *pulumirpc.RegisterJobRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *monitorServer) RegisterStep(
	_ context.Context, req *pulumirpc.RegisterStepRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	stepPath := req.GetPath()
	if stepPath == "" {
		return nil, fmt.Errorf("register step request is missing path")
	}

	m.mu.Lock()
	m.steps = append(m.steps, observedStep{
		Path:         stepPath,
		Dependencies: collectDependencyPaths(req.GetDependencies()),
		HasOnError:   req.GetHasOnError(),
	})
	m.mu.Unlock()
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *monitorServer) snapshotStepsForJob(jobPath string) []observedStep {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]observedStep, 0)
	prefix := jobPath + "/steps/"
	for _, step := range m.steps {
		if strings.HasPrefix(step.Path, prefix) {
			results = append(results, step)
		}
	}
	return results
}

func collectDependencyPaths(expr *pulumirpc.DependencyExpression) []string {
	if expr == nil {
		return nil
	}

	paths := make([]string, 0)
	for _, term := range expr.GetTerms() {
		switch value := term.GetTerm().(type) {
		case *pulumirpc.DependencyTerm_Path:
			if value.Path != "" {
				paths = append(paths, value.Path)
			}
		case *pulumirpc.DependencyTerm_Expression:
			paths = append(paths, collectDependencyPaths(value.Expression)...)
		}
	}
	return paths
}

func orderStepsByRegistration(steps []observedStep) []observedStep {
	ordered := make([]observedStep, len(steps))
	copy(ordered, steps)
	return ordered
}
