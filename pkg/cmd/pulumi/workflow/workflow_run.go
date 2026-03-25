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
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type stepResult struct {
	Path       string `json:"path"`
	ResultJSON string `json:"resultJson"`
}

func newWorkflowRunCmd() *cobra.Command {
	var inputJSON string
	var emitJSON bool

	cmd := &cobra.Command{
		Use:   "run <plugin-path> <job>",
		Short: "Run an exported workflow job",
		Long: `Run an exported workflow job from a workflow package plugin path.

For now, <plugin-path> must be a local path (for example to a Python workflow program).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			pluginPath := args[0]
			jobNameOrToken := args[1]

			input, err := parseInputJSON(inputJSON)
			if err != nil {
				return err
			}

			results, jobToken, err := runExportedJob(ctx, pluginPath, jobNameOrToken, input)
			if err != nil {
				return err
			}

			if emitJSON {
				payload := struct {
					Job   string       `json:"job"`
					Steps []stepResult `json:"steps"`
				}{
					Job:   jobToken,
					Steps: results,
				}
				bytes, err := json.MarshalIndent(payload, "", "  ")
				if err != nil {
					return fmt.Errorf("encode result: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(bytes))
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ran %s\n", jobToken)
			for _, result := range results {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", result.Path, result.ResultJSON)
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "plugin-path"},
			{Name: "job"},
		},
		Required: 2,
	})
	cmd.Flags().StringVar(&inputJSON, "input", "{}", "JSON object input passed to the job")
	cmd.Flags().BoolVar(&emitJSON, "json", false, "Emit machine-readable JSON output")

	return cmd
}

func parseInputJSON(input string) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return nil, fmt.Errorf("invalid --input JSON object: %w", err)
	}
	if value == nil {
		value = map[string]any{}
	}
	return value, nil
}

func runExportedJob(
	ctx context.Context,
	pluginPath string,
	jobNameOrToken string,
	input map[string]any,
) ([]stepResult, string, error) {
	server := &monitorServer{}
	grpcServer := grpc.NewServer()
	pulumirpc.RegisterGraphMonitorServer(grpcServer, server)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("listen: %w", err)
	}
	defer func() {
		_ = listener.Close()
		grpcServer.Stop()
	}()
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create workflow plugin context: %w", err)
	}
	defer func() {
		_ = pctx.Close()
	}()

	workflowPlugin, err := pctx.Host.Workflow(pluginPath)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = workflowPlugin.Close()
	}()

	jobToken, err := resolveJobToken(ctx, workflowPlugin, jobNameOrToken)
	if err != nil {
		return nil, "", err
	}

	inputValue, err := structpb.NewValue(input)
	if err != nil {
		return nil, "", fmt.Errorf("encode job input: %w", err)
	}
	workflowContext := &pulumirpc.WorkflowContext{
		ExecutionId: "cli-run",
	}
	jobFilterResp, jobFilterErr := workflowPlugin.RunFilter(ctx, &pulumirpc.RunFilterRequest{
		Path: jobToken,
	})
	if jobFilterErr != nil {
		return nil, "", fmt.Errorf("run filter for exported job %q: %w", jobToken, jobFilterErr)
	}
	if !jobFilterResp.GetPass() {
		return []stepResult{}, jobToken, nil
	}

	generateResp, err := workflowPlugin.GenerateJob(ctx, &pulumirpc.GenerateJobRequest{
		Context:             workflowContext,
		Name:                jobToken,
		GraphMonitorAddress: listener.Addr().String(),
		InputPath:           jobToken,
		InputValue:          inputValue,
	})
	if err != nil {
		return nil, "", fmt.Errorf("generate exported job %q: %w", jobToken, err)
	}
	if generateResp.GetError() != nil && generateResp.GetError().GetReason() != "" {
		return nil, "", fmt.Errorf("generate exported job %q failed: %s", jobToken, generateResp.GetError().GetReason())
	}

	steps := server.snapshotStepsForJob(jobToken)
	if len(steps) == 0 {
		return nil, "", fmt.Errorf("exported job %q has no steps", jobToken)
	}

	results, err := runObservedSteps(ctx, workflowPlugin, workflowContext, steps)
	if err != nil {
		return nil, "", err
	}

	return results, jobToken, nil
}

func resolveJobToken(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	jobNameOrToken string,
) (string, error) {
	// If already token-like, validate and use directly.
	if strings.Contains(jobNameOrToken, ":") {
		if _, err := workflowPlugin.GetJob(ctx, &pulumirpc.GetJobRequest{Token: jobNameOrToken}); err != nil {
			return "", fmt.Errorf("get job metadata for %q: %w", jobNameOrToken, err)
		}
		return jobNameOrToken, nil
	}

	resp, err := workflowPlugin.GetJobs(ctx, &pulumirpc.GetJobsRequest{})
	if err != nil {
		return "", fmt.Errorf("get jobs: %w", err)
	}
	matches := make([]string, 0)
	for _, job := range resp.GetJobs() {
		token := job.GetToken()
		parts := strings.Split(token, ":")
		if len(parts) > 0 && parts[len(parts)-1] == jobNameOrToken {
			matches = append(matches, token)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("exported job %q not found", jobNameOrToken)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("job name %q is ambiguous; use full token (%s)", jobNameOrToken, strings.Join(matches, ", "))
	}
}

func runStepWithRetry(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	workflowContext *pulumirpc.WorkflowContext,
	step observedStep,
) (*pulumirpc.RunStepResponse, error) {
	const maxAttempts = 5

	attempt := 0
	for {
		attempt++
		response, err := workflowPlugin.RunStep(ctx, &pulumirpc.RunStepRequest{
			Context: workflowContext,
			Path:    step.Path,
		})

		if err == nil && (response.GetError() == nil || response.GetError().GetReason() == "") {
			return response, nil
		}

		reason := "step failed"
		if err != nil {
			reason = err.Error()
		}
		if response != nil && response.GetError() != nil && response.GetError().GetReason() != "" {
			reason = response.GetError().GetReason()
		}

		if !step.HasOnError || attempt >= maxAttempts {
			return nil, fmt.Errorf("run step %q failed after %d attempts: %s", step.Path, attempt, reason)
		}

		onErrorResponse, onErrorErr := workflowPlugin.RunOnError(ctx, &pulumirpc.RunOnErrorRequest{
			Context: workflowContext,
			Path:    step.Path,
			Errors: []*pulumirpc.ErrorRecord{
				{
					StepPath: step.Path,
					Reason:   reason,
				},
			},
		})
		if onErrorErr != nil {
			return nil, fmt.Errorf("run onError for step %q: %w", step.Path, onErrorErr)
		}
		if onErrorResponse.GetError() != nil && onErrorResponse.GetError().GetReason() != "" {
			return nil, fmt.Errorf("run onError for step %q failed: %s", step.Path, onErrorResponse.GetError().GetReason())
		}
		if !onErrorResponse.GetRetry() {
			return nil, fmt.Errorf("run step %q failed and onError declined retry: %s", step.Path, reason)
		}

		retryAfter := onErrorResponse.GetRetryAfter()
		if retryAfter != "" {
			delay, parseErr := time.ParseDuration(retryAfter)
			if parseErr == nil && delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
		}
	}
}

func runObservedSteps(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	workflowContext *pulumirpc.WorkflowContext,
	steps []observedStep,
) ([]stepResult, error) {
	results := make([]stepResult, 0, len(steps))
	for _, step := range steps {
		stepFilterResp, stepFilterErr := workflowPlugin.RunFilter(ctx, &pulumirpc.RunFilterRequest{
			Path: step.Path,
		})
		if stepFilterErr != nil {
			return nil, fmt.Errorf("run filter for step %q: %w", step.Path, stepFilterErr)
		}
		if !stepFilterResp.GetPass() {
			continue
		}

		runResp, err := runStepWithRetry(ctx, workflowPlugin, workflowContext, step)
		if err != nil {
			return nil, err
		}
		resultJSON := ""
		if runResp.GetResult() != nil {
			if bytes, marshalErr := protojson.Marshal(runResp.GetResult()); marshalErr == nil {
				resultJSON = string(bytes)
			}
		}
		results = append(results, stepResult{
			Path:       step.Path,
			ResultJSON: resultJSON,
		})
	}
	return results, nil
}
