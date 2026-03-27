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

package tests

import (
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func init() {
	LanguageTests["workflow-expr-step"] = LanguageTest{
		Runs: []TestRun{
			{
				AssertWorkflow: func(l *L, args AssertWorkflowArgs) {
					steps, err := args.Workflow.GetSteps(args.Context, &pulumirpc.GetStepsRequest{})
					require.NoError(l, err)
					require.Len(l, steps.GetSteps(), 1)

					stepToken := steps.GetSteps()[0]

					runTrueResp, err := args.Workflow.RunStep(args.Context, &pulumirpc.RunStepRequest{
						Context: &pulumirpc.WorkflowContext{ExecutionId: "test-true"},
						Path:    stepToken,
						Input:   structpb.NewBoolValue(true),
					})
					require.NoError(l, err)
					require.NotNil(l, runTrueResp.GetResult())
					assert.False(l, runTrueResp.GetResult().GetBoolValue())

					runFalseResp, err := args.Workflow.RunStep(args.Context, &pulumirpc.RunStepRequest{
						Context: &pulumirpc.WorkflowContext{ExecutionId: "test-false"},
						Path:    stepToken,
						Input:   structpb.NewBoolValue(false),
					})
					require.NoError(l, err)
					require.NotNil(l, runFalseResp.GetResult())
					assert.True(l, runFalseResp.GetResult().GetBoolValue())
				},
			},
		},
	}
}
