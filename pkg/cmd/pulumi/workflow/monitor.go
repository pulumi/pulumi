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

package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type observedStep struct {
	Path       string
	HasOnError bool
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
	stepPath := req.GetJob() + "/steps/" + req.GetName()
	if req.GetJob() == "" || req.GetName() == "" {
		return nil, fmt.Errorf("register step request is missing path")
	}

	m.mu.Lock()
	m.steps = append(m.steps, observedStep{
		Path:       stepPath,
		HasOnError: req.GetHasOnError(),
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
