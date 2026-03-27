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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	codegenpcl "github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type WorkflowEvaluator struct {
	pulumirpc.UnimplementedWorkflowEvaluatorServer

	packageName        string
	packageVersion     string
	program            *codegenpcl.WorkflowProgram
	stepsByPath        map[string]codegenpcl.WorkflowStepDefinition
	stepResults        map[string]*structpb.Value
	jobInputByPath     map[string]*structpb.Value
	jobExprByPath      map[string]string
	jobInputTypeByPath map[string]string
	filtersByPath      map[string]bool
}

func NewWorkflowEvaluator(programPath string) (*WorkflowEvaluator, error) {
	program, err := codegenpcl.BindWorkflowProgram(programPath)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(programPath), filepath.Ext(programPath))
	return &WorkflowEvaluator{
		packageName:        name,
		packageVersion:     "0.0.1",
		program:            program,
		stepsByPath:        map[string]codegenpcl.WorkflowStepDefinition{},
		stepResults:        map[string]*structpb.Value{},
		jobInputByPath:     map[string]*structpb.Value{},
		jobExprByPath:      map[string]string{},
		jobInputTypeByPath: map[string]string{},
		filtersByPath:      map[string]bool{},
	}, nil
}

func (e *WorkflowEvaluator) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return &pulumirpc.WorkflowHandshakeResponse{}, nil
}

func (e *WorkflowEvaluator) GetPackageInfo(
	context.Context, *pulumirpc.EmptyRequest,
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
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetGraphsResponse, error) {
	resp := &pulumirpc.GetGraphsResponse{}
	for _, graph := range e.program.Workflows {
		name := graph.Name
		resp.Graphs = append(resp.Graphs, &pulumirpc.GraphInfo{Token: name})
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetGraph(
	_ context.Context, req *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetGraphResponse, error) {
	if _, ok := e.program.GraphByName(req.GetToken()); !ok {
		return nil, status.Errorf(codes.NotFound, "unknown graph token %q", req.GetToken())
	}
	return &pulumirpc.GetGraphResponse{Graph: &pulumirpc.GraphInfo{Token: req.GetToken()}}, nil
}

func (e *WorkflowEvaluator) GetTriggers(
	_ context.Context, req *pulumirpc.EmptyRequest,
) (*pulumirpc.GetTriggersResponse, error) {
	_ = req
	resp := &pulumirpc.GetTriggersResponse{}
	for _, name := range e.program.TriggerNames() {
		resp.Triggers = append(resp.Triggers, e.triggerToken(name))
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetTrigger(
	_ context.Context, req *pulumirpc.TokenLookupRequest,
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
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetJobsResponse, error) {
	resp := &pulumirpc.GetJobsResponse{}
	for _, name := range e.program.JobNames() {
		job, _ := e.program.JobByName(name)
		resp.Jobs = append(resp.Jobs, &pulumirpc.JobInfo{
			Token:      e.jobToken(name),
			InputType:  typeReferenceFromInputType(job.InputType),
			OutputType: &pulumirpc.TypeReference{Token: "pulumi:json#/Any"},
		})
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetJob(
	_ context.Context, req *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetJobResponse, error) {
	name, ok := e.resolveJobToken(req.GetToken())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown job token %q", req.GetToken())
	}
	job, ok := e.program.JobByName(name)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown job token %q", req.GetToken())
	}
	return &pulumirpc.GetJobResponse{
		Job: &pulumirpc.JobInfo{
			Token:      e.jobToken(name),
			InputType:  typeReferenceFromInputType(job.InputType),
			OutputType: &pulumirpc.TypeReference{Token: "pulumi:json#/Any"},
		},
	}, nil
}

func (e *WorkflowEvaluator) GetSteps(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetStepsResponse, error) {
	resp := &pulumirpc.GetStepsResponse{}
	for _, name := range e.program.StepNames() {
		resp.Steps = append(resp.Steps, e.stepToken(name))
	}
	return resp, nil
}

func (e *WorkflowEvaluator) GetStep(
	_ context.Context, req *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetStepResponse, error) {
	name, ok := e.resolveStepToken(req.GetToken())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown step token %q", req.GetToken())
	}
	step, ok := e.program.StepByName(name)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown step token %q", req.GetToken())
	}
	return &pulumirpc.GetStepResponse{
		InputType:  typeReferenceFromInputType(step.InputType),
		OutputType: &pulumirpc.TypeReference{Token: defaultTypeToken(inferStepOutputType(step))},
	}, nil
}

func (e *WorkflowEvaluator) GenerateGraph(
	ctx context.Context, req *pulumirpc.GenerateGraphRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	graph, ok := e.program.GraphByName(req.GetPath())
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
	if req.GetPath() == "" {
		if req.GetName() == "" {
			return nil, status.Error(codes.InvalidArgument, "either path or name is required")
		}
		name, ok := e.resolveJobToken(req.GetName())
		if !ok {
			return nil, status.Errorf(codes.NotFound, "unknown job token %q", req.GetName())
		}
		jobDef, ok := e.program.JobByName(name)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "unknown job token %q", req.GetName())
		}
		monitor, conn, err := graphMonitor(req.GetGraphMonitorAddress())
		if err != nil {
			return nil, err
		}
		defer func() { _ = conn.Close() }()

		jobPath := e.jobToken(name)
		return e.registerJob(ctx, req.GetContext(), jobPath, req.GetInputValue(), jobDef.Steps, typeReferenceTokenForInputType(jobDef.InputType), jobDef.Expr, monitor)
	}

	parts := strings.Split(req.GetPath(), "/jobs/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job path %q", req.GetPath())
	}
	graphName, jobName := parts[0], parts[1]
	graph, ok := e.program.GraphByName(graphName)
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

	var selected *codegenpcl.WorkflowGraphJob
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
	jobInputType := ""
	jobExpr := selected.Expr
	if selected.Uses != "" {
		jobDef, ok := e.program.JobDefinitionForUse(selected.Uses)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "unknown job definition %q", selected.Uses)
		}
		jobSteps = jobDef.Steps
		jobInputType = typeReferenceTokenForInputType(jobDef.InputType)
		if jobExpr == "" {
			jobExpr = jobDef.Expr
		}
	}

	jobPath := graphName + "/jobs/" + jobName
	if _, err := monitor.RegisterJob(ctx, &pulumirpc.RegisterJobRequest{
		Context: req.GetContext(),
		Path:    jobPath,
		Dependencies: &pulumirpc.DependencyExpression{
			Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
		},
	}); err != nil {
		return nil, err
	}
	e.jobExprByPath[jobPath] = jobExpr
	e.jobInputTypeByPath[jobPath] = jobInputType

	if selected.Filter != nil {
		e.filtersByPath[jobPath] = *selected.Filter
	}
	if req.GetInputValue() != nil {
		e.jobInputByPath[jobPath] = structpb.NewStructValue(req.GetInputValue())
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

func (e *WorkflowEvaluator) registerJob(
	ctx context.Context,
	wfContext *pulumirpc.WorkflowContext,
	jobPath string,
	inputValue *structpb.Struct,
	jobSteps []codegenpcl.WorkflowJobStep,
	jobInputType string,
	jobExpr string,
	monitor pulumirpc.GraphMonitorClient,
) (*pulumirpc.GenerateNodeResponse, error) {
	if _, err := monitor.RegisterJob(ctx, &pulumirpc.RegisterJobRequest{
		Context: wfContext,
		Path:    jobPath,
		Dependencies: &pulumirpc.DependencyExpression{
			Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
		},
	}); err != nil {
		return nil, err
	}
	e.jobExprByPath[jobPath] = jobExpr
	e.jobInputTypeByPath[jobPath] = jobInputType
	if inputValue != nil {
		e.jobInputByPath[jobPath] = structpb.NewStructValue(inputValue)
	}

	for _, step := range jobSteps {
		stepDef, err := e.stepDefinitionForJobStep(step)
		if err != nil {
			return nil, err
		}
		stepPath := jobPath + "/steps/" + step.Name
		if _, err := monitor.RegisterStep(ctx, &pulumirpc.RegisterStepRequest{
			Context: wfContext,
			Name:    step.Name,
			Job:     jobPath,
			Dependencies: &pulumirpc.DependencyExpression{
				Operator: pulumirpc.DependencyExpression_OPERATOR_ALL,
			},
		}); err != nil {
			return nil, err
		}
		e.stepsByPath[stepPath] = stepDef
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
		name, resolved := e.resolveStepToken(req.GetPath())
		if !resolved {
			return nil, status.Errorf(codes.NotFound, "unknown step path %q", req.GetPath())
		}
		step, ok = e.program.StepByName(name)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "unknown step path %q", req.GetPath())
		}
	}

	value, err := executeStepDefinition(step, req.GetInput())
	if err != nil {
		return nil, err
	}
	e.stepResults[req.GetPath()] = value
	return &pulumirpc.RunStepResponse{Result: value}, nil
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

func (e *WorkflowEvaluator) ResolveJobResult(
	_ context.Context, req *pulumirpc.ResolveJobResultRequest,
) (*pulumirpc.ResolveJobResultResponse, error) {
	jobPath := req.GetPath()
	if jobPath == "" {
		return nil, status.Error(codes.InvalidArgument, "job path is required")
	}
	expr := strings.TrimSpace(e.jobExprByPath[jobPath])
	if expr == "" {
		return &pulumirpc.ResolveJobResultResponse{
			Result: structpb.NewNullValue(),
		}, nil
	}

	if stepValue, ok := e.stepResults[jobPath+"/steps/"+expr]; ok {
		return &pulumirpc.ResolveJobResultResponse{Result: stepValue}, nil
	}

	if inputValue, ok := e.jobInputByPath[jobPath]; ok {
		value, err := executeStepDefinition(codegenpcl.WorkflowStepDefinition{
			InputType: codegenpcl.WorkflowInputType{Token: e.jobInputTypeByPath[jobPath]},
			Expr:      expr,
		}, inputValue)
		if err != nil {
			return nil, err
		}
		return &pulumirpc.ResolveJobResultResponse{Result: value}, nil
	}
	return &pulumirpc.ResolveJobResultResponse{
		Result: structpb.NewStringValue(expr),
	}, nil
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

func (e *WorkflowEvaluator) stepToken(name string) string {
	return fmt.Sprintf("%s:index:%s", e.packageName, name)
}

func (e *WorkflowEvaluator) jobToken(name string) string {
	return fmt.Sprintf("%s:index:%s", e.packageName, name)
}

func (e *WorkflowEvaluator) hasTriggerToken(token string) bool {
	for _, name := range e.program.TriggerNames() {
		if token == name || token == e.triggerToken(name) {
			return true
		}
	}
	return false
}

func (e *WorkflowEvaluator) resolveStepToken(token string) (string, bool) {
	for _, name := range e.program.StepNames() {
		if token == name || token == e.stepToken(name) {
			return name, true
		}
	}
	return "", false
}

func (e *WorkflowEvaluator) resolveJobToken(token string) (string, bool) {
	for _, name := range e.program.JobNames() {
		if token == name || token == e.jobToken(name) {
			return name, true
		}
	}
	return "", false
}

func defaultTypeToken(token string) string {
	if token != "" {
		return token
	}
	return "pulumi:json#/Any"
}

func inferStepOutputType(step codegenpcl.WorkflowStepDefinition) string {
	if step.OutputType != "" {
		return step.OutputType
	}
	if step.Command != "" {
		return "string"
	}

	expr := strings.TrimSpace(step.Expr)
	switch expr {
	case "input":
		return typeReferenceTokenForInputType(step.InputType)
	case "!input", "not input":
		return "bool"
	default:
		if expr != "" {
			return "string"
		}
	}
	return ""
}

func typeReferenceTokenForInputType(inputType codegenpcl.WorkflowInputType) string {
	if inputType.TokenOrEmpty() != "" {
		return inputType.TokenOrEmpty()
	}
	if inputType.IsStruct() {
		return "pulumi:json#/Any"
	}
	return ""
}

func typeReferenceFromInputType(inputType codegenpcl.WorkflowInputType) *pulumirpc.TypeReference {
	if inputType.TokenOrEmpty() != "" {
		return &pulumirpc.TypeReference{Token: inputType.TokenOrEmpty()}
	}
	if inputType.IsStruct() {
		properties := map[string]*pulumirpc.PropertySpec{}
		for name, typ := range inputType.Fields {
			properties[name] = &pulumirpc.PropertySpec{Type: typ}
		}
		return &pulumirpc.TypeReference{
			Object: &pulumirpc.StructObject{
				Properties: properties,
			},
		}
	}
	return &pulumirpc.TypeReference{Token: "pulumi:json#/Any"}
}

func executeStepDefinition(step codegenpcl.WorkflowStepDefinition, input *structpb.Value) (*structpb.Value, error) {
	if step.Command != "" {
		cmd := exec.Command("/bin/sh", "-c", step.Command) //nolint:gosec
		cmd.Env = os.Environ()
		if input != nil {
			switch kind := input.GetKind().(type) {
			case *structpb.Value_StructValue:
				for key, value := range kind.StructValue.GetFields() {
					s := value.GetStringValue()
					cmd.Env = append(cmd.Env, key+"="+s)
					cmd.Env = append(cmd.Env, strings.ToUpper(key)+"="+s)
				}
			default:
				cmd.Env = append(cmd.Env, "INPUT="+input.GetStringValue())
			}
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "step command failed: %v", err)
		}
		return structpb.NewStringValue(strings.TrimSpace(string(out))), nil
	}
	expr := strings.TrimSpace(step.Expr)
	inputBool := false
	hasInputBool := false
	if input != nil {
		if input.GetKind() != nil {
			switch input.GetKind().(type) {
			case *structpb.Value_BoolValue:
				inputBool = input.GetBoolValue()
				hasInputBool = true
			case *structpb.Value_StructValue:
				if field, ok := input.GetStructValue().GetFields()["input"]; ok {
					inputBool = field.GetBoolValue()
					hasInputBool = true
				}
			}
		}
	}

	switch expr {
	case "input":
		if input == nil {
			return structpb.NewNullValue(), nil
		}
		if input.GetStructValue() != nil {
			if field, ok := input.GetStructValue().GetFields()["input"]; ok {
				return field, nil
			}
		}
		return input, nil
	case "!input", "not input":
		if !hasInputBool {
			return nil, status.Error(codes.InvalidArgument, "step expression requires bool input")
		}
		return structpb.NewBoolValue(!inputBool), nil
	default:
		return structpb.NewStringValue(step.Expr), nil
	}
}

func (e *WorkflowEvaluator) stepDefinitionForJobStep(
	step codegenpcl.WorkflowJobStep,
) (codegenpcl.WorkflowStepDefinition, error) {
	if step.Uses != "" {
		stepDef, ok := e.program.StepDefinitionForUse(step.Uses)
		if !ok {
			return codegenpcl.WorkflowStepDefinition{}, status.Errorf(codes.NotFound, "unknown step definition %q", step.Uses)
		}
		return stepDef, nil
	}

	if step.Command != "" || step.Expr != "" {
		return codegenpcl.WorkflowStepDefinition{
			Name:    step.Name,
			Command: step.Command,
			Expr:    step.Expr,
		}, nil
	}

	return codegenpcl.WorkflowStepDefinition{}, status.Errorf(
		codes.InvalidArgument,
		"step %q must set one of uses, command, or expr",
		step.Name,
	)
}

func (e *WorkflowEvaluator) registerGraphTriggers(
	ctx context.Context,
	wfContext *pulumirpc.WorkflowContext,
	graphPath string,
	graph codegenpcl.WorkflowGraph,
	monitor pulumirpc.GraphMonitorClient,
) error {
	if len(graph.Triggers) > 0 {
		for _, graphTrigger := range graph.Triggers {
			triggerName := graphTrigger.Name
			if graphTrigger.Uses != "" {
				if _, resolved, ok := e.program.ResolveTriggerNameFromUse(graphTrigger.Uses); ok {
					triggerName = resolved
				}
			}
			trigger, ok := e.program.TriggerByName(triggerName)
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
		trigger, ok := e.program.TriggerByName(triggerRef.Name)
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
