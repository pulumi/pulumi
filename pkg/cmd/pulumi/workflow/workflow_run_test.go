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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestParseInputJSON(t *testing.T) {
	t.Parallel()

	value, err := parseInputJSON(`{"message":"hello","repeat":3}`)
	if err != nil {
		t.Fatalf("parseInputJSON failed: %v", err)
	}
	if got := value["message"]; got != "hello" {
		t.Fatalf("unexpected message value: %#v", got)
	}
	if got := value["repeat"]; got != float64(3) {
		t.Fatalf("unexpected repeat value: %#v", got)
	}
}

func TestParseInputJSONInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseInputJSON(`not-json`)
	if err == nil {
		t.Fatalf("expected parseInputJSON to fail")
	}
}

func TestRunObservedStepsAppliesStepFilters(t *testing.T) {
	t.Parallel()

	filterByPath := map[string]bool{
		"/main/steps/first":  true,
		"/main/steps/second": false,
	}
	filterCalls := make([]string, 0)
	runStepCalls := make([]string, 0)
	workflowPlugin := &plugin.MockWorkflow{
		RunFilterF: func(_ context.Context, req *pulumirpc.RunFilterRequest) (*pulumirpc.RunFilterResponse, error) {
			filterCalls = append(filterCalls, req.GetPath())
			return &pulumirpc.RunFilterResponse{Pass: filterByPath[req.GetPath()]}, nil
		},
		RunStepF: func(_ context.Context, req *pulumirpc.RunStepRequest) (*pulumirpc.RunStepResponse, error) {
			runStepCalls = append(runStepCalls, req.GetPath())
			return &pulumirpc.RunStepResponse{Result: structpb.NewStringValue(req.GetPath())}, nil
		},
	}

	results, err := runObservedSteps(
		context.Background(),
		workflowPlugin,
		&pulumirpc.WorkflowContext{ExecutionId: "test"},
		[]observedStep{
			{Path: "/main/steps/first"},
			{Path: "/main/steps/second"},
		},
	)
	if err != nil {
		t.Fatalf("runObservedSteps failed: %v", err)
	}

	if len(filterCalls) != 2 {
		t.Fatalf("expected 2 filter calls, got %d", len(filterCalls))
	}
	if len(runStepCalls) != 1 {
		t.Fatalf("expected 1 step execution, got %d", len(runStepCalls))
	}
	if runStepCalls[0] != "/main/steps/first" {
		t.Fatalf("unexpected executed step: %q", runStepCalls[0])
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(results))
	}
	if results[0].Path != "/main/steps/first" {
		t.Fatalf("unexpected result path: %q", results[0].Path)
	}
}
