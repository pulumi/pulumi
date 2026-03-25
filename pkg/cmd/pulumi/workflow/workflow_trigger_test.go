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
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRunTriggerMockByName(t *testing.T) {
	t.Parallel()

	var calledToken string
	var calledArgs []string
	workflowPlugin := &plugin.MockWorkflow{
		GetTriggersF: func(context.Context, *pulumirpc.GetTriggersRequest) (*pulumirpc.GetTriggersResponse, error) {
			return &pulumirpc.GetTriggersResponse{
				Triggers: []string{"example:index:cron"},
			}, nil
		},
		RunTriggerMockF: func(_ context.Context, req *pulumirpc.RunTriggerMockRequest) (*pulumirpc.RunTriggerMockResponse, error) {
			calledToken = req.GetToken()
			calledArgs = append([]string{}, req.GetArgs()...)
			return &pulumirpc.RunTriggerMockResponse{
				Value: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"timestamp": structpb.NewStringValue("2024-05-25T00:00:00Z"),
					},
				},
			}, nil
		},
	}

	valueJSON, token, err := runTriggerMock(context.Background(), workflowPlugin, "cron", []string{"25th May 2024"})
	if err != nil {
		t.Fatalf("runTriggerMock failed: %v", err)
	}

	if token != "example:index:cron" {
		t.Fatalf("expected resolved token, got %q", token)
	}
	if calledToken != "example:index:cron" {
		t.Fatalf("expected mock call with resolved token, got %q", calledToken)
	}
	if !reflect.DeepEqual(calledArgs, []string{"25th May 2024"}) {
		t.Fatalf("unexpected args: %#v", calledArgs)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(valueJSON), &decoded); err != nil {
		t.Fatalf("decode value json: %v", err)
	}
	if got := decoded["timestamp"]; got != "2024-05-25T00:00:00Z" {
		t.Fatalf("unexpected timestamp value: %#v", got)
	}
}

func TestRunTriggerMockByToken(t *testing.T) {
	t.Parallel()

	var getTriggerToken string
	var runTriggerToken string
	workflowPlugin := &plugin.MockWorkflow{
		GetTriggerF: func(_ context.Context, req *pulumirpc.GetTriggerRequest) (*pulumirpc.GetTriggerResponse, error) {
			getTriggerToken = req.GetToken()
			return &pulumirpc.GetTriggerResponse{}, nil
		},
		RunTriggerMockF: func(_ context.Context, req *pulumirpc.RunTriggerMockRequest) (*pulumirpc.RunTriggerMockResponse, error) {
			runTriggerToken = req.GetToken()
			return &pulumirpc.RunTriggerMockResponse{
				Value: &structpb.Struct{},
			}, nil
		},
	}

	_, token, err := runTriggerMock(context.Background(), workflowPlugin, "example:index:cron", nil)
	if err != nil {
		t.Fatalf("runTriggerMock failed: %v", err)
	}

	if token != "example:index:cron" {
		t.Fatalf("unexpected token: %q", token)
	}
	if getTriggerToken != "example:index:cron" {
		t.Fatalf("expected GetTrigger validation, got %q", getTriggerToken)
	}
	if runTriggerToken != "example:index:cron" {
		t.Fatalf("expected RunTriggerMock call with token, got %q", runTriggerToken)
	}
}

func TestResolveTriggerTokenAmbiguous(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetTriggersF: func(context.Context, *pulumirpc.GetTriggersRequest) (*pulumirpc.GetTriggersResponse, error) {
			return &pulumirpc.GetTriggersResponse{
				Triggers: []string{"example:index:cron", "other:index:cron"},
			}, nil
		},
	}

	_, err := resolveTriggerToken(context.Background(), workflowPlugin, "cron")
	if err == nil {
		t.Fatalf("expected ambiguous trigger resolution failure")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveTriggerTokenNotFound(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetTriggersF: func(context.Context, *pulumirpc.GetTriggersRequest) (*pulumirpc.GetTriggersResponse, error) {
			return &pulumirpc.GetTriggersResponse{
				Triggers: []string{"example:index:push"},
			}, nil
		},
	}

	_, err := resolveTriggerToken(context.Background(), workflowPlugin, "cron")
	if err == nil {
		t.Fatalf("expected trigger not found failure")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTriggerMockReturnsError(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetTriggersF: func(context.Context, *pulumirpc.GetTriggersRequest) (*pulumirpc.GetTriggersResponse, error) {
			return &pulumirpc.GetTriggersResponse{
				Triggers: []string{"example:index:cron"},
			}, nil
		},
		RunTriggerMockF: func(context.Context, *pulumirpc.RunTriggerMockRequest) (*pulumirpc.RunTriggerMockResponse, error) {
			return nil, errors.New("boom")
		},
	}

	_, _, err := runTriggerMock(context.Background(), workflowPlugin, "cron", nil)
	if err == nil {
		t.Fatalf("expected run trigger mock failure")
	}
	if !strings.Contains(err.Error(), "run trigger mock") {
		t.Fatalf("unexpected error: %v", err)
	}
}
