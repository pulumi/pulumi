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

package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type workflowFile struct {
	Triggers  []workflowTriggerDef `hcl:"trigger,block"`
	Steps     []workflowStepDef    `hcl:"step,block"`
	Jobs      []workflowJobDef     `hcl:"job,block"`
	Workflows []workflowGraph      `hcl:"workflow,block"`
}

type workflowGraph struct {
	Name        string                 `hcl:"name,label"`
	TriggerRefs []workflowTriggerRef   `hcl:"trigger_ref,block"`
	Triggers    []workflowGraphTrigger `hcl:"trigger,block"`
	Jobs        []workflowGraphJob     `hcl:"job,block"`
}

type workflowTriggerDef struct {
	Name     string `hcl:"name,label"`
	Type     string `hcl:"type,optional"`
	Schedule string `hcl:"schedule,optional"`
}

type workflowTriggerRef struct {
	Name string `hcl:"name,label"`
}

type workflowGraphTrigger struct {
	Name     string `hcl:"name,label"`
	Uses     string `hcl:"uses,optional"`
	Schedule string `hcl:"schedule,optional"`
}

type workflowStepDef struct {
	Name    string `hcl:"name,label"`
	Command string `hcl:"command,optional"`
	Expr    string `hcl:"expr,optional"`
}

type workflowJobDef struct {
	Name  string            `hcl:"name,label"`
	Steps []workflowJobStep `hcl:"step,block"`
}

type workflowJobStep struct {
	Name      string   `hcl:"name,label"`
	Uses      string   `hcl:"uses,optional"`
	Command   string   `hcl:"command,optional"`
	Expr      string   `hcl:"expr,optional"`
	Filter    *bool    `hcl:"filter,optional"`
	DependsOn []string `hcl:"depends_on,optional"`
}

type workflowGraphJob struct {
	Name      string            `hcl:"name,label"`
	Uses      string            `hcl:"uses,optional"`
	Filter    *bool             `hcl:"filter,optional"`
	Steps     []workflowJobStep `hcl:"step,block"`
	DependsOn []string          `hcl:"depends_on,optional"`
}

type WorkflowEvaluator struct {
	pulumirpc.UnimplementedWorkflowEvaluatorServer

	packageName    string
	packageVersion string
	graphsByName   map[string]workflowGraph
	triggersByName map[string]workflowTriggerDef
	stepsByName    map[string]workflowStepDef
	jobsByName     map[string]workflowJobDef
	stepsByPath    map[string]workflowStepDef
	filtersByPath  map[string]bool
}

func NewWorkflowEvaluator(programPath string) (*WorkflowEvaluator, error) {
	parser := hclparse.NewParser()
	hclFile, diags := parser.ParseHCLFile(programPath)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse workflow pcl file %q: %s", programPath, diags.Error())
	}

	var file workflowFile
	decodeDiags := gohcl.DecodeBody(hclFile.Body, nil, &file)
	if decodeDiags.HasErrors() {
		return nil, fmt.Errorf("decode workflow pcl file %q: %s", programPath, decodeDiags.Error())
	}

	graphsByName := map[string]workflowGraph{}
	for _, graph := range file.Workflows {
		if _, exists := graphsByName[graph.Name]; exists {
			return nil, fmt.Errorf("duplicate workflow graph %q", graph.Name)
		}
		graphsByName[graph.Name] = graph
	}
	triggersByName := map[string]workflowTriggerDef{}
	for _, trigger := range file.Triggers {
		if _, exists := triggersByName[trigger.Name]; exists {
			return nil, fmt.Errorf("duplicate trigger definition %q", trigger.Name)
		}
		triggersByName[trigger.Name] = trigger
	}
	stepsByName := map[string]workflowStepDef{}
	for _, step := range file.Steps {
		if _, exists := stepsByName[step.Name]; exists {
			return nil, fmt.Errorf("duplicate step definition %q", step.Name)
		}
		stepsByName[step.Name] = step
	}
	jobsByName := map[string]workflowJobDef{}
	for _, job := range file.Jobs {
		if _, exists := jobsByName[job.Name]; exists {
			return nil, fmt.Errorf("duplicate job definition %q", job.Name)
		}
		jobsByName[job.Name] = job
	}

	if len(graphsByName) == 0 {
		return nil, fmt.Errorf("no workflow blocks found in %q", programPath)
	}

	name := strings.TrimSuffix(filepath.Base(programPath), filepath.Ext(programPath))
	return &WorkflowEvaluator{
		packageName:    name,
		packageVersion: "0.0.1",
		graphsByName:   graphsByName,
		triggersByName: triggersByName,
		stepsByName:    stepsByName,
		jobsByName:     jobsByName,
		stepsByPath:    map[string]workflowStepDef{},
		filtersByPath:  map[string]bool{},
	}, nil
}

func (e *WorkflowEvaluator) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return &pulumirpc.WorkflowHandshakeResponse{}, nil
}

func (e *WorkflowEvaluator) GetPackageInfo(
	context.Context, *pulumirpc.GetPackageInfoRequest,
) (*pulumirpc.GetPackageInfoResponse, error) {
	return &pulumirpc.GetPackageInfoResponse{
		Package: &pulumirpc.PackageInfo{
			Name:        e.packageName,
			DisplayName: e.packageName,
			Version:     e.packageVersion,
		},
	}, nil
}

func (e *WorkflowEvaluator) GetGraphs(
	context.Context, *pulumirpc.GetGraphsRequest,
) (*pulumirpc.GetGraphsResponse, error) {
	resp := &pulumirpc.GetGraphsResponse{}
	for name := range e.graphsByName {
		resp.Graphs = append(resp.Graphs, &pulumirpc.GraphInfo{Token: name})
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetGraph(
	_ context.Context, req *pulumirpc.GetGraphRequest,
) (*pulumirpc.GetGraphResponse, error) {
	if _, ok := e.graphsByName[req.GetToken()]; !ok {
		return nil, status.Errorf(codes.NotFound, "unknown graph token %q", req.GetToken())
	}
	return &pulumirpc.GetGraphResponse{Graph: &pulumirpc.GraphInfo{Token: req.GetToken()}}, nil
}

func (e *WorkflowEvaluator) GetTriggers(
	_ context.Context, req *pulumirpc.GetTriggersRequest,
) (*pulumirpc.GetTriggersResponse, error) {
	_ = req
	resp := &pulumirpc.GetTriggersResponse{}
	for name := range e.triggersByName {
		resp.Triggers = append(resp.Triggers, e.triggerToken(name))
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetTrigger(
	_ context.Context, req *pulumirpc.GetTriggerRequest,
) (*pulumirpc.GetTriggerResponse, error) {
	if !e.hasTriggerToken(req.GetToken()) {
		return nil, status.Errorf(codes.NotFound, "unknown trigger token %q", req.GetToken())
	}
	return &pulumirpc.GetTriggerResponse{
		InputType:  &pulumirpc.TypeReference{Token: "pcl.workflow.CronTriggerInput"},
		OutputType: &pulumirpc.TypeReference{Token: "pcl.workflow.CronTriggerOutput"},
	}, nil
}

func (e *WorkflowEvaluator) GetJobs(
	context.Context, *pulumirpc.GetJobsRequest,
) (*pulumirpc.GetJobsResponse, error) {
	return &pulumirpc.GetJobsResponse{}, nil
}

func (e *WorkflowEvaluator) GetJob(
	context.Context, *pulumirpc.GetJobRequest,
) (*pulumirpc.GetJobResponse, error) {
	return nil, status.Error(codes.NotFound, "no exported jobs")
}

func (e *WorkflowEvaluator) GenerateGraph(
	ctx context.Context, req *pulumirpc.GenerateGraphRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	graph, ok := e.graphsByName[req.GetPath()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown graph %q", req.GetPath())
	}
	monitor, conn, err := graphMonitor(req.GetGraphMonitorAddress())
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	if _, err := monitor.RegisterGraph(ctx, &pulumirpc.RegisterGraphRequest{
		Context: req.GetContext(),
		Path:    req.GetPath(),
		Dependencies: &pulumirpc.DependencyExpression{
			Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
		},
	}); err != nil {
		return nil, err
	}

	if err := e.registerGraphTriggers(ctx, req.GetContext(), req.GetPath(), graph, monitor); err != nil {
		return nil, err
	}
	for _, graphJob := range graph.Jobs {
		if graphJob.Filter != nil {
			e.filtersByPath[req.GetPath()+"/jobs/"+graphJob.Name] = *graphJob.Filter
		}
	}
	return &pulumirpc.GenerateNodeResponse{}, nil
}

func (e *WorkflowEvaluator) GenerateJob(
	ctx context.Context, req *pulumirpc.GenerateJobRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	parts := strings.Split(req.GetPath(), "/jobs/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job path %q", req.GetPath())
	}
	graphName, jobName := parts[0], parts[1]
	graph, ok := e.graphsByName[graphName]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown graph %q", graphName)
	}

	monitor, conn, err := graphMonitor(req.GetGraphMonitorAddress())
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	_, _ = monitor.RegisterGraph(ctx, &pulumirpc.RegisterGraphRequest{
		Context: req.GetContext(),
		Path:    graphName,
		Dependencies: &pulumirpc.DependencyExpression{
			Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
		},
	})

	if err := e.registerGraphTriggers(ctx, req.GetContext(), graphName, graph, monitor); err != nil {
		return nil, err
	}

	var selected *workflowGraphJob
	for _, graphJob := range graph.Jobs {
		if graphJob.Name == jobName {
			j := graphJob
			selected = &j
			break
		}
	}
	if selected == nil {
		return nil, status.Errorf(codes.NotFound, "unknown job %q", req.GetPath())
	}
	jobSteps := selected.Steps
	if selected.Uses != "" {
		jobDef, ok := e.jobDefinitionForUse(selected.Uses)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "unknown job definition %q", selected.Uses)
		}
		jobSteps = jobDef.Steps
	}

	jobPath := graphName + "/jobs/" + jobName
	jobDependencies := &pulumirpc.DependencyExpression{
		Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
	}
	for _, dep := range selected.DependsOn {
		jobDependencies.Terms = append(jobDependencies.Terms, &pulumirpc.DependencyTerm{
			Term: &pulumirpc.DependencyTerm_Path{Path: graphName + "/jobs/" + dep},
		})
	}
	if _, err := monitor.RegisterJob(ctx, &pulumirpc.RegisterJobRequest{
		Context:      req.GetContext(),
		Path:         jobPath,
		Dependencies: jobDependencies,
	}); err != nil {
		return nil, err
	}
	if selected.Filter != nil {
		e.filtersByPath[jobPath] = *selected.Filter
	}

	for _, step := range jobSteps {
		stepDef, err := e.stepDefinitionForJobStep(step)
		if err != nil {
			return nil, err
		}
		stepPath := jobPath + "/steps/" + step.Name
		stepDependencies := &pulumirpc.DependencyExpression{
			Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
		}
		for _, dep := range step.DependsOn {
			stepDependencies.Terms = append(stepDependencies.Terms, &pulumirpc.DependencyTerm{
				Term: &pulumirpc.DependencyTerm_Path{Path: jobPath + "/steps/" + dep},
			})
		}
		if _, err := monitor.RegisterStep(ctx, &pulumirpc.RegisterStepRequest{
			Context:      req.GetContext(),
			Name:         step.Name,
			Job:          jobPath,
			Dependencies: stepDependencies,
		}); err != nil {
			return nil, err
		}
		e.stepsByPath[stepPath] = stepDef
		if step.Filter != nil {
			e.filtersByPath[stepPath] = *step.Filter
		}
	}

	return &pulumirpc.GenerateNodeResponse{}, nil
}

func (e *WorkflowEvaluator) RunSensor(
	context.Context, *pulumirpc.RunSensorRequest,
) (*pulumirpc.RunSensorResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RunSensor not implemented for PCL workflows")
}

func (e *WorkflowEvaluator) RunStep(
	_ context.Context, req *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	step, ok := e.stepsByPath[req.GetPath()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown step path %q", req.GetPath())
	}

	var value string
	if step.Command != "" {
		out, err := exec.Command("/bin/sh", "-c", step.Command).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "step command failed: %v", err)
		}
		value = strings.TrimSpace(string(out))
	} else {
		value = step.Expr
	}

	return &pulumirpc.RunStepResponse{
		Result: structpb.NewStringValue(value),
	}, nil
}

func (e *WorkflowEvaluator) ResolveStepResult(
	context.Context, *pulumirpc.ResolveStepResultRequest,
) (*pulumirpc.ResolveStepResultResponse, error) {
	return &pulumirpc.ResolveStepResultResponse{}, nil
}

func (e *WorkflowEvaluator) RunTriggerMock(
	_ context.Context, req *pulumirpc.RunTriggerMockRequest,
) (*pulumirpc.RunTriggerMockResponse, error) {
	if !e.hasTriggerToken(req.GetToken()) {
		return nil, status.Errorf(codes.NotFound, "unknown trigger token %q", req.GetToken())
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	if len(req.GetArgs()) > 0 && req.GetArgs()[0] != "" {
		ts = req.GetArgs()[0]
	}
	return &pulumirpc.RunTriggerMockResponse{
		Value: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"timestamp": structpb.NewStringValue(ts),
			},
		},
	}, nil
}

func (e *WorkflowEvaluator) RunFilter(
	_ context.Context, req *pulumirpc.RunFilterRequest,
) (*pulumirpc.RunFilterResponse, error) {
	if pass, ok := e.filtersByPath[req.GetPath()]; ok {
		return &pulumirpc.RunFilterResponse{Pass: pass}, nil
	}
	if req.GetValue() == nil || req.GetValue().GetStructValue() == nil {
		return &pulumirpc.RunFilterResponse{Pass: true}, nil
	}
	value := req.GetValue().GetStructValue().GetFields()["timestamp"].GetStringValue()
	return &pulumirpc.RunFilterResponse{Pass: strings.HasSuffix(value, "00:00+00:00")}, nil
}

func (e *WorkflowEvaluator) RunOnError(
	context.Context, *pulumirpc.RunOnErrorRequest,
) (*pulumirpc.RunOnErrorResponse, error) {
	return &pulumirpc.RunOnErrorResponse{Retry: false}, nil
}

func graphMonitor(address string) (pulumirpc.GraphMonitorClient, *grpc.ClientConn, error) {
	if address == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "graph monitor address is required")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, status.Errorf(codes.Unavailable, "connect graph monitor: %v", err)
	}
	return pulumirpc.NewGraphMonitorClient(conn), conn, nil
}

func (e *WorkflowEvaluator) triggerToken(name string) string {
	return fmt.Sprintf("%s:index:%s", e.packageName, name)
}

func (e *WorkflowEvaluator) hasTriggerToken(token string) bool {
	for name := range e.triggersByName {
		if token == name || token == e.triggerToken(name) {
			return true
		}
	}
	return false
}

func (e *WorkflowEvaluator) stepDefinitionForUse(uses string) (workflowStepDef, bool) {
	if uses == "" {
		return workflowStepDef{}, false
	}
	if step, ok := e.stepsByName[uses]; ok {
		return step, true
	}
	if !strings.Contains(uses, ":") {
		return workflowStepDef{}, false
	}
	_, name, found := strings.Cut(uses, ":")
	if !found || name == "" {
		return workflowStepDef{}, false
	}
	step, ok := e.stepsByName[name]
	return step, ok
}

func (e *WorkflowEvaluator) jobDefinitionForUse(uses string) (workflowJobDef, bool) {
	if uses == "" {
		return workflowJobDef{}, false
	}
	if job, ok := e.jobsByName[uses]; ok {
		return job, true
	}
	if !strings.Contains(uses, ":") {
		return workflowJobDef{}, false
	}
	_, name, found := strings.Cut(uses, ":")
	if !found || name == "" {
		return workflowJobDef{}, false
	}
	job, ok := e.jobsByName[name]
	return job, ok
}

func (e *WorkflowEvaluator) stepDefinitionForJobStep(step workflowJobStep) (workflowStepDef, error) {
	if step.Uses != "" {
		stepDef, ok := e.stepDefinitionForUse(step.Uses)
		if !ok {
			return workflowStepDef{}, status.Errorf(codes.NotFound, "unknown step definition %q", step.Uses)
		}
		return stepDef, nil
	}

	if step.Command != "" || step.Expr != "" {
		return workflowStepDef{
			Name:    step.Name,
			Command: step.Command,
			Expr:    step.Expr,
		}, nil
	}

	return workflowStepDef{}, status.Errorf(
		codes.InvalidArgument,
		"step %q must set one of uses, command, or expr",
		step.Name,
	)
}

func (e *WorkflowEvaluator) registerGraphTriggers(
	ctx context.Context,
	wfContext *pulumirpc.WorkflowContext,
	graphPath string,
	graph workflowGraph,
	monitor pulumirpc.GraphMonitorClient,
) error {
	if len(graph.Triggers) > 0 {
		for _, graphTrigger := range graph.Triggers {
			triggerName := graphTrigger.Name
			if graphTrigger.Uses != "" {
				if _, resolved, ok := e.resolveTriggerNameFromUse(graphTrigger.Uses); ok {
					triggerName = resolved
				}
			}
			trigger, ok := e.triggersByName[triggerName]
			if !ok {
				return status.Errorf(codes.NotFound, "unknown trigger %q", graphTrigger.Uses)
			}
			spec := &structpb.Struct{Fields: map[string]*structpb.Value{}}
			schedule := graphTrigger.Schedule
			if schedule == "" {
				schedule = trigger.Schedule
			}
			if schedule != "" {
				spec.Fields["schedule"] = structpb.NewStringValue(schedule)
			}
			if _, err := monitor.RegisterTrigger(ctx, &pulumirpc.RegisterTriggerRequest{
				Context: wfContext,
				Path:    graphPath + "/" + graphTrigger.Name,
				Type:    e.triggerToken(triggerName),
				Spec:    spec,
			}); err != nil {
				return err
			}
		}
		return nil
	}

	for _, triggerRef := range graph.TriggerRefs {
		trigger, ok := e.triggersByName[triggerRef.Name]
		if !ok {
			return status.Errorf(codes.NotFound, "unknown trigger ref %q", triggerRef.Name)
		}
		spec := &structpb.Struct{Fields: map[string]*structpb.Value{}}
		if trigger.Schedule != "" {
			spec.Fields["schedule"] = structpb.NewStringValue(trigger.Schedule)
		}
		if _, err := monitor.RegisterTrigger(ctx, &pulumirpc.RegisterTriggerRequest{
			Context: wfContext,
			Path:    graphPath + "/" + triggerRef.Name,
			Type:    e.triggerToken(triggerRef.Name),
			Spec:    spec,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (e *WorkflowEvaluator) resolveTriggerNameFromUse(uses string) (string, string, bool) {
	if uses == "" {
		return "", "", false
	}
	if name, ok := e.triggersByName[uses]; ok {
		return name.Name, uses, true
	}
	if !strings.Contains(uses, ":") {
		return "", "", false
	}
	_, name, found := strings.Cut(uses, ":")
	if !found || name == "" {
		return "", "", false
	}
	_, ok := e.triggersByName[name]
	return uses, name, ok
}
