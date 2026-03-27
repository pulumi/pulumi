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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestDescribeWorkflowJobByName(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetPackageInfoF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetPackageInfoResponse, error) {
			return &pulumirpc.GetPackageInfoResponse{
				Package: &pulumirpc.PackageInfo{Name: "example", Version: "1.2.3"},
			}, nil
		},
		GetJobsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetJobsResponse, error) {
			return &pulumirpc.GetJobsResponse{
				Jobs: []*pulumirpc.JobInfo{
					{Token: "example:index:build"},
				},
			}, nil
		},
		GetJobF: func(_ context.Context, req *pulumirpc.TokenLookupRequest) (*pulumirpc.GetJobResponse, error) {
			return &pulumirpc.GetJobResponse{
				Job: &pulumirpc.JobInfo{
					Token:      req.GetToken(),
					InputType:  &pulumirpc.TypeReference{Token: "example:index:BuildInput"},
					OutputType: &pulumirpc.TypeReference{Token: "example:index:BuildOutput"},
					HasOnError: true,
				},
			}, nil
		},
	}

	out, err := describeWorkflow(t.Context(), workflowPlugin, "job", "build")
	if err != nil {
		t.Fatalf("describeWorkflow failed: %v", err)
	}

	expected := []string{
		"Package: example",
		"Version: 1.2.3",
		"Kind: job",
		"Token: example:index:build",
		"Input Type: example:index:BuildInput",
		"Output Type: example:index:BuildOutput",
		"Has OnError: true",
	}
	for _, segment := range expected {
		if !strings.Contains(out, segment) {
			t.Fatalf("expected output to contain %q, got:\n%s", segment, out)
		}
	}
}

func TestDescribeWorkflowTriggerByToken(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetPackageInfoF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetPackageInfoResponse, error) {
			return &pulumirpc.GetPackageInfoResponse{
				Package: &pulumirpc.PackageInfo{Name: "example", Version: "1.0.0"},
			}, nil
		},
		GetTriggerF: func(_ context.Context, req *pulumirpc.TokenLookupRequest) (*pulumirpc.GetTriggerResponse, error) {
			if req.GetToken() != "example:index:cron" {
				t.Fatalf("unexpected trigger token: %q", req.GetToken())
			}
			return &pulumirpc.GetTriggerResponse{
				InputType:  &pulumirpc.TypeReference{Token: "example:index:CronInput"},
				OutputType: &pulumirpc.TypeReference{Token: "example:index:CronOutput"},
			}, nil
		},
	}

	out, err := describeWorkflow(t.Context(), workflowPlugin, "trigger", "example:index:cron")
	if err != nil {
		t.Fatalf("describeWorkflow failed: %v", err)
	}

	if strings.Contains(out, "Has OnError:") {
		t.Fatalf("trigger output should not include on_error field: %s", out)
	}
	if !strings.Contains(out, "Kind: trigger") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Token: example:index:cron") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestDescribeWorkflowUnknownKind(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetPackageInfoF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetPackageInfoResponse, error) {
			return &pulumirpc.GetPackageInfoResponse{}, nil
		},
	}

	_, err := describeWorkflow(t.Context(), workflowPlugin, "sensor", "foo")
	if err == nil {
		t.Fatalf("expected unknown kind error")
	}
	if !strings.Contains(err.Error(), "expected one of graph, job, trigger") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGraphTokenAmbiguous(t *testing.T) {
	t.Parallel()

	workflowPlugin := &plugin.MockWorkflow{
		GetGraphsF: func(context.Context, *pulumirpc.EmptyRequest) (*pulumirpc.GetGraphsResponse, error) {
			return &pulumirpc.GetGraphsResponse{
				Graphs: []*pulumirpc.GraphInfo{
					{Token: "example:index:main"},
					{Token: "other:index:main"},
				},
			}, nil
		},
	}

	_, err := resolveGraphToken(t.Context(), workflowPlugin, "main")
	if err == nil {
		t.Fatalf("expected ambiguous graph resolution failure")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}
