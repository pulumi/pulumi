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
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestParseInputJSON(t *testing.T) {
	t.Parallel()

	valueAny, err := parseInputJSON(`{"message":"hello","repeat":3}`, true)
	if err != nil {
		t.Fatalf("parseInputJSON failed: %v", err)
	}
	value, ok := valueAny.(map[string]any)
	if !ok {
		t.Fatalf("expected map input, got %T", valueAny)
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

	_, err := parseInputJSON(`not-json`, true)
	if err == nil {
		t.Fatalf("expected parseInputJSON to fail")
	}
}

func TestParseInputJSONDefaultsToNullWhenInputNotProvided(t *testing.T) {
	t.Parallel()

	value, err := parseInputJSON(`{"ignored":"because-not-provided"}`, false)
	if err != nil {
		t.Fatalf("parseInputJSON failed: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil default input, got %#v", value)
	}
}

func TestResolveExecutionID(t *testing.T) {
	t.Parallel()

	t.Run("uses provided value", func(t *testing.T) {
		t.Parallel()
		const expected = "manual-id-123"
		if got := resolveExecutionID(expected); got != expected {
			t.Fatalf("expected %q, got %q", expected, got)
		}
	})

	t.Run("defaults to uuid", func(t *testing.T) {
		t.Parallel()
		got := resolveExecutionID("")
		if got == "" {
			t.Fatalf("expected non-empty execution id")
		}
		if _, err := uuid.Parse(got); err != nil {
			t.Fatalf("expected UUID execution id, got %q: %v", got, err)
		}
	})
}

func TestEncodeJobInputStruct(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		value, err := encodeJobInputStruct(nil)
		if err != nil {
			t.Fatalf("encodeJobInputStruct failed: %v", err)
		}
		if value != nil {
			t.Fatalf("expected nil struct for nil input, got %#v", value)
		}
	})

	t.Run("object input", func(t *testing.T) {
		t.Parallel()
		value, err := encodeJobInputStruct(map[string]any{"message": "hello", "repeat": 3})
		if err != nil {
			t.Fatalf("encodeJobInputStruct failed: %v", err)
		}
		if got := value.GetFields()["message"].GetStringValue(); got != "hello" {
			t.Fatalf("unexpected message field: %q", got)
		}
	})

	t.Run("scalar input rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := encodeJobInputStruct("hello"); err == nil {
			t.Fatalf("expected scalar input to fail")
		}
	})
}

func TestParseRunJobArgs(t *testing.T) {
	t.Parallel()

	props := []*pulumirpc.InputProperty{
		{Name: "message", Type: "string", Required: true},
		{Name: "repeat", Type: "integer", Required: true},
		{Name: "dry_run", Type: "boolean", Required: false},
	}

	t.Run("parses typed flags", func(t *testing.T) {
		t.Parallel()
		options, err := parseRunJobArgs(
			[]string{"--message", "hello", "--repeat=3", "--dry-run"},
			props,
			"default-id",
		)
		if err != nil {
			t.Fatalf("parseRunJobArgs failed: %v", err)
		}
		if options.executionID != "default-id" {
			t.Fatalf("unexpected execution id %q", options.executionID)
		}
		if options.emitJSON {
			t.Fatalf("expected emitJSON=false")
		}
		if got := options.input["message"]; got != "hello" {
			t.Fatalf("unexpected message value: %#v", got)
		}
		if got := options.input["repeat"]; got != int64(3) {
			t.Fatalf("unexpected repeat value: %#v", got)
		}
		if got := options.input["dry_run"]; got != true {
			t.Fatalf("unexpected dry_run value: %#v", got)
		}
	})

	t.Run("parses static flags", func(t *testing.T) {
		t.Parallel()
		options, err := parseRunJobArgs(
			[]string{"--message", "hello", "--repeat", "3", "--json", "--execution-id", "abc"},
			props,
			"default-id",
		)
		if err != nil {
			t.Fatalf("parseRunJobArgs failed: %v", err)
		}
		if !options.emitJSON {
			t.Fatalf("expected emitJSON=true")
		}
		if options.executionID != "abc" {
			t.Fatalf("unexpected execution id %q", options.executionID)
		}
	})

	t.Run("missing required input", func(t *testing.T) {
		t.Parallel()
		_, err := parseRunJobArgs([]string{"--message", "hello"}, props, "default-id")
		if err == nil {
			t.Fatalf("expected parseRunJobArgs to fail")
		}
	})
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
		t.Context(),
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

func TestResolveObservedJobResult(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		ResolveJobResultF: func(_ context.Context, req *pulumirpc.ResolveJobResultRequest) (*pulumirpc.ResolveJobResultResponse, error) {
			if req.GetPath() != "example:index:job" {
				t.Fatalf("unexpected resolve path: %q", req.GetPath())
			}
			return &pulumirpc.ResolveJobResultResponse{
				Result: structpb.NewStringValue("done"),
			}, nil
		},
	}

	resultJSON, err := resolveObservedJobResult(
		t.Context(),
		workflowPlugin,
		&pulumirpc.WorkflowContext{ExecutionId: "test"},
		"example:index:job",
	)
	if err != nil {
		t.Fatalf("resolveObservedJobResult failed: %v", err)
	}
	if resultJSON != `"done"` {
		t.Fatalf("unexpected result json: %q", resultJSON)
	}
}

func TestResolveObservedJobResultErrors(t *testing.T) {
	t.Parallel()

	t.Run("grpc error", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			ResolveJobResultF: func(context.Context, *pulumirpc.ResolveJobResultRequest) (*pulumirpc.ResolveJobResultResponse, error) {
				return nil, errors.New("boom")
			},
		}
		_, err := resolveObservedJobResult(t.Context(), workflowPlugin, &pulumirpc.WorkflowContext{}, "job")
		if err == nil {
			t.Fatalf("expected resolveObservedJobResult to fail")
		}
	})

	t.Run("workflow error", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			ResolveJobResultF: func(context.Context, *pulumirpc.ResolveJobResultRequest) (*pulumirpc.ResolveJobResultResponse, error) {
				return &pulumirpc.ResolveJobResultResponse{
					Error: &pulumirpc.WorkflowError{Reason: "failed"},
				}, nil
			},
		}
		_, err := resolveObservedJobResult(t.Context(), workflowPlugin, &pulumirpc.WorkflowContext{}, "job")
		if err == nil {
			t.Fatalf("expected resolveObservedJobResult to fail")
		}
	})

	t.Run("empty result", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			ResolveJobResultF: func(context.Context, *pulumirpc.ResolveJobResultRequest) (*pulumirpc.ResolveJobResultResponse, error) {
				return &pulumirpc.ResolveJobResultResponse{}, nil
			},
		}
		_, err := resolveObservedJobResult(t.Context(), workflowPlugin, &pulumirpc.WorkflowContext{}, "job")
		if err == nil {
			t.Fatalf("expected resolveObservedJobResult to fail")
		}
	})
}

func TestParseRunStepArgs(t *testing.T) {
	t.Parallel()

	opts, err := parseRunStepArgs(
		[]string{"--json", "--execution-id", "exec-1", "--input", `{"message":"hello","count":2}`},
		"default-id",
	)
	if err != nil {
		t.Fatalf("parseRunStepArgs failed: %v", err)
	}
	if !opts.emitJSON {
		t.Fatalf("expected emitJSON=true")
	}
	if opts.executionID != "exec-1" {
		t.Fatalf("unexpected execution id: %q", opts.executionID)
	}
	if got := opts.input.GetStructValue().GetFields()["message"].GetStringValue(); got != "hello" {
		t.Fatalf("unexpected message field: %q", got)
	}
}

func TestRunExportedStepWithPlugin(t *testing.T) {
	t.Parallel()

	var gotReq *pulumirpc.RunStepRequest
	workflowPlugin := &plugin.MockWorkflow{
		RunStepF: func(_ context.Context, req *pulumirpc.RunStepRequest) (*pulumirpc.RunStepResponse, error) {
			gotReq = req
			return &pulumirpc.RunStepResponse{
				Result: structpb.NewStringValue("done"),
			}, nil
		},
	}

	result, emitJSON, err := runExportedStepWithPlugin(
		t.Context(),
		workflowPlugin,
		"example:index:touch",
		[]string{"--json", "--execution-id", "exec-123", "--input", `{"message":"hello"}`},
		"default-id",
	)
	if err != nil {
		t.Fatalf("runExportedStepWithPlugin failed: %v", err)
	}
	if !emitJSON {
		t.Fatalf("expected emitJSON=true")
	}
	if result != `"done"` {
		t.Fatalf("unexpected result JSON: %q", result)
	}
	if gotReq == nil {
		t.Fatalf("expected RunStep request")
	}
	if gotReq.GetPath() != "example:index:touch" {
		t.Fatalf("unexpected step path: %q", gotReq.GetPath())
	}
	if gotReq.GetContext().GetExecutionId() != "exec-123" {
		t.Fatalf("unexpected execution id: %q", gotReq.GetContext().GetExecutionId())
	}
	if got := gotReq.GetInput().GetStructValue().GetFields()["message"].GetStringValue(); got != "hello" {
		t.Fatalf("unexpected input message: %q", got)
	}
}

func TestResolveRunnableToken(t *testing.T) {
	t.Parallel()

	t.Run("job by short name", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			GetJobsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetJobsResponse, error) {
				return &pulumirpc.GetJobsResponse{Jobs: []*pulumirpc.JobInfo{{Token: "build"}}}, nil
			},
			GetStepsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetStepsResponse, error) {
				return &pulumirpc.GetStepsResponse{Steps: []string{"touch"}}, nil
			},
		}
		kind, token, err := resolveRunnableToken(t.Context(), workflowPlugin, "build")
		if err != nil {
			t.Fatalf("resolveRunnableToken failed: %v", err)
		}
		if kind != runnableKindJob || token != "build" {
			t.Fatalf("unexpected resolution: kind=%q token=%q", kind, token)
		}
	})

	t.Run("step by short name", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			GetJobsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetJobsResponse, error) {
				return &pulumirpc.GetJobsResponse{Jobs: []*pulumirpc.JobInfo{{Token: "build"}}}, nil
			},
			GetStepsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetStepsResponse, error) {
				return &pulumirpc.GetStepsResponse{Steps: []string{"touch"}}, nil
			},
		}
		kind, token, err := resolveRunnableToken(t.Context(), workflowPlugin, "touch")
		if err != nil {
			t.Fatalf("resolveRunnableToken failed: %v", err)
		}
		if kind != runnableKindStep || token != "touch" {
			t.Fatalf("unexpected resolution: kind=%q token=%q", kind, token)
		}
	})

	t.Run("ambiguous short name", func(t *testing.T) {
		t.Parallel()
		workflowPlugin := &plugin.MockWorkflow{
			GetJobsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetJobsResponse, error) {
				return &pulumirpc.GetJobsResponse{Jobs: []*pulumirpc.JobInfo{{Token: "build"}}}, nil
			},
			GetStepsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetStepsResponse, error) {
				return &pulumirpc.GetStepsResponse{Steps: []string{"build"}}, nil
			},
		}
		_, _, err := resolveRunnableToken(t.Context(), workflowPlugin, "build")
		if err == nil {
			t.Fatalf("expected ambiguity error")
		}
	})
}
