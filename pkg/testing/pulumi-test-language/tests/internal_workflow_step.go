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
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["internal-workflow-step"] = LanguageTest{
		Runs: []TestRun{
			{
				AssertWorkflow: func(l *L, args AssertWorkflowArgs) {
					ctx := context.Background()

					steps, err := args.Workflow.GetSteps(ctx, &pulumirpc.GetStepsRequest{})
					require.NoError(l, err)
					require.Len(l, steps.GetSteps(), 1)

					stepToken := steps.GetSteps()[0]
					step, err := args.Workflow.GetStep(ctx, &pulumirpc.GetStepRequest{Token: stepToken})
					require.NoError(l, err)
					require.NotNil(l, step.GetInputType())
					require.NotNil(l, step.GetOutputType())
					assert.Equal(l, "internal:workflow:StepInput", step.GetInputType().GetToken())
					assert.Equal(l, "internal:workflow:StepOutput", step.GetOutputType().GetToken())

					runResp, err := args.Workflow.RunStep(ctx, &pulumirpc.RunStepRequest{
						Context: &pulumirpc.WorkflowContext{ExecutionId: "test"},
						Path:    stepToken,
					})
					require.NoError(l, err)
					require.NotNil(l, runResp.GetResult())
					assert.Equal(l, "step-output", runResp.GetResult().GetStringValue())
				},
			},
		},
	}
}
