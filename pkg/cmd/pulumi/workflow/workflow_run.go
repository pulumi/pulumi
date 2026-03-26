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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
	cmd := &cobra.Command{
		Use:   "run <plugin-path> <job>",
		Short: "Run an exported workflow job",
		Long: `Run an exported workflow job from a workflow package plugin path.

For now, <plugin-path> must be a local path (for example to a Python workflow program).`,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return cmd.Help()
			}
			if len(args) < 2 {
				return fmt.Errorf("expected arguments: <plugin-path> <job>")
			}
			ctx := cmd.Context()
			pluginPath := args[0]
			jobNameOrToken := args[1]
			runArgs := args[2:]

			results, jobToken, jobResultJSON, emitJSON, err := runExportedJob(
				ctx, pluginPath, jobNameOrToken, runArgs, resolveExecutionID(""),
			)
			if err != nil {
				return err
			}

			if emitJSON {
				payload := struct {
					Job        string       `json:"job"`
					ResultJSON string       `json:"resultJson"`
					Steps      []stepResult `json:"steps"`
				}{
					Job:        jobToken,
					ResultJSON: jobResultJSON,
					Steps:      results,
				}
				bytes, err := json.MarshalIndent(payload, "", "  ")
				if err != nil {
					return fmt.Errorf("encode result: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(bytes))
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ran %s\n", jobToken)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Result: %s\n", jobResultJSON)
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

	return cmd
}

func parseInputJSON(input string, provided bool) (any, error) {
	if !provided {
		return nil, nil
	}

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
	runArgs []string,
	defaultExecutionID string,
) ([]stepResult, string, string, bool, error) {
	server := &monitorServer{}
	grpcServer := grpc.NewServer()
	pulumirpc.RegisterGraphMonitorServer(grpcServer, server)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, "", "", false, fmt.Errorf("listen: %w", err)
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
		return nil, "", "", false, fmt.Errorf("create workflow plugin context: %w", err)
	}
	defer func() {
		_ = pctx.Close()
	}()

	workflowPlugin, err := pctx.Host.Workflow(pluginPath)
	if err != nil {
		return nil, "", "", false, err
	}
	defer func() {
		_ = workflowPlugin.Close()
	}()

	jobToken, err := resolveJobToken(ctx, workflowPlugin, jobNameOrToken)
	if err != nil {
		return nil, "", "", false, err
	}

	jobInfo, err := workflowPlugin.GetJob(ctx, &pulumirpc.GetJobRequest{Token: jobToken})
	if err != nil {
		return nil, "", "", false, fmt.Errorf("get job metadata for %q: %w", jobToken, err)
	}

	parsedOptions, err := parseRunJobArgs(runArgs, jobInfo.GetInputProperties(), defaultExecutionID)
	if err != nil {
		return nil, "", "", false, err
	}

	inputValue, err := encodeJobInputStruct(parsedOptions.input)
	if err != nil {
		return nil, "", "", false, fmt.Errorf("encode job input: %w", err)
	}
	workflowContext := &pulumirpc.WorkflowContext{
		ExecutionId: parsedOptions.executionID,
	}

	generateResp, err := workflowPlugin.GenerateJob(ctx, &pulumirpc.GenerateJobRequest{
		Context:             workflowContext,
		Name:                jobToken,
		GraphMonitorAddress: listener.Addr().String(),
		InputValue:          inputValue,
	})
	if err != nil {
		return nil, "", "", false, fmt.Errorf("generate exported job %q: %w", jobToken, err)
	}
	if generateResp.GetError() != nil && generateResp.GetError().GetReason() != "" {
		return nil, "", "", false, fmt.Errorf("generate exported job %q failed: %s", jobToken, generateResp.GetError().GetReason())
	}

	steps := server.snapshotStepsForJob(jobToken)
	if len(steps) == 0 {
		return nil, "", "", false, fmt.Errorf("exported job %q has no steps", jobToken)
	}

	results, err := runObservedSteps(ctx, workflowPlugin, workflowContext, steps)
	if err != nil {
		return nil, "", "", false, err
	}

	jobResultJSON, err := resolveObservedJobResult(ctx, workflowPlugin, workflowContext, jobToken)
	if err != nil {
		return nil, "", "", false, err
	}

	return results, jobToken, jobResultJSON, parsedOptions.emitJSON, nil
}

func resolveExecutionID(userProvided string) string {
	if userProvided != "" {
		return userProvided
	}
	return uuid.NewString()
}

type runJobOptions struct {
	input       map[string]any
	executionID string
	emitJSON    bool
}

func parseRunJobArgs(
	args []string,
	inputProps []*pulumirpc.InputProperty,
	defaultExecutionID string,
) (runJobOptions, error) {
	options := runJobOptions{
		input:       map[string]any{},
		executionID: defaultExecutionID,
		emitJSON:    false,
	}

	propertyByFlag := map[string]*pulumirpc.InputProperty{}
	for _, prop := range inputProps {
		propertyByFlag[prop.GetName()] = prop
		propertyByFlag[strings.ReplaceAll(prop.GetName(), "_", "-")] = prop
	}

	for i := 0; i < len(args); i++ {
		token := args[i]
		if !strings.HasPrefix(token, "--") {
			return runJobOptions{}, fmt.Errorf("unexpected argument %q; expected flags", token)
		}
		name, value, hasValue := splitFlag(token)

		switch name {
		case "json":
			if !hasValue {
				options.emitJSON = true
				continue
			}
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return runJobOptions{}, fmt.Errorf("invalid --json value %q", value)
			}
			options.emitJSON = parsed
			continue
		case "execution-id":
			if !hasValue {
				if i+1 >= len(args) {
					return runJobOptions{}, fmt.Errorf("--execution-id requires a value")
				}
				i++
				value = args[i]
			}
			options.executionID = value
			continue
		case "input":
			if !hasValue {
				if i+1 >= len(args) {
					return runJobOptions{}, fmt.Errorf("--input requires a value")
				}
				i++
				value = args[i]
			}
			obj, err := parseInputJSON(value, true)
			if err != nil {
				return runJobOptions{}, err
			}
			parsed, ok := obj.(map[string]any)
			if !ok {
				return runJobOptions{}, fmt.Errorf("--input must decode to an object")
			}
			for key, fieldValue := range parsed {
				options.input[key] = fieldValue
			}
			continue
		}

		prop := propertyByFlag[name]
		if prop == nil {
			return runJobOptions{}, fmt.Errorf("unknown flag --%s", name)
		}
		if !hasValue {
			if prop.GetType() == "boolean" {
				value = "true"
			} else {
				if i+1 >= len(args) {
					return runJobOptions{}, fmt.Errorf("--%s requires a value", name)
				}
				i++
				value = args[i]
			}
		}
		coerced, err := coerceFlagValue(prop.GetType(), value)
		if err != nil {
			return runJobOptions{}, fmt.Errorf("invalid value for --%s: %w", name, err)
		}
		options.input[prop.GetName()] = coerced
	}

	for _, prop := range inputProps {
		if prop.GetRequired() {
			if _, ok := options.input[prop.GetName()]; !ok {
				return runJobOptions{}, fmt.Errorf("missing required flag --%s", strings.ReplaceAll(prop.GetName(), "_", "-"))
			}
		}
	}

	return options, nil
}

func splitFlag(token string) (string, string, bool) {
	trimmed := strings.TrimPrefix(token, "--")
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return trimmed, "", false
}

func coerceFlagValue(kind string, value string) (any, error) {
	switch kind {
	case "string":
		return value, nil
	case "integer":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case "number":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case "boolean":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported type %q", kind)
	}
}

func encodeJobInputStruct(input any) (*structpb.Struct, error) {
	if input == nil {
		return nil, nil
	}
	obj, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("job input must be a JSON object (got %T)", input)
	}
	return structpb.NewStruct(obj)
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

func resolveObservedJobResult(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	workflowContext *pulumirpc.WorkflowContext,
	jobPath string,
) (string, error) {
	response, err := workflowPlugin.ResolveJobResult(ctx, &pulumirpc.ResolveJobResultRequest{
		Context: workflowContext,
		Path:    jobPath,
	})
	if err != nil {
		return "", fmt.Errorf("resolve job result %q: %w", jobPath, err)
	}
	if response.GetError() != nil && response.GetError().GetReason() != "" {
		return "", fmt.Errorf("resolve job result %q failed: %s", jobPath, response.GetError().GetReason())
	}
	if response.GetResult() == nil {
		return "", fmt.Errorf("resolve job result %q returned empty result", jobPath)
	}

	bytes, err := protojson.Marshal(response.GetResult())
	if err != nil {
		return "", fmt.Errorf("marshal resolved job result for %q: %w", jobPath, err)
	}
	return string(bytes), nil
}
