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
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/spf13/cobra"
)

func newWorkflowDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <plugin-source> <kind> <name-or-token>",
		Short: "Describe an exported workflow graph, job, or trigger",
		Long: `Describe one exported workflow entity from a workflow package plugin.

<kind> must be one of: graph, job, trigger.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			pluginSource := args[0]
			kind := strings.ToLower(args[1])
			nameOrToken := args[2]

			description, err := describeFromPluginPath(ctx, pluginSource, kind, nameOrToken)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), description)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "plugin-source"},
			{Name: "kind"},
			{Name: "name-or-token"},
		},
		Required: 3,
	})
	return cmd
}

func describeFromPluginPath(
	ctx context.Context,
	pluginPath string,
	kind string,
	nameOrToken string,
) (string, error) {
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil, nil)
	if err != nil {
		return "", fmt.Errorf("create workflow plugin context: %w", err)
	}
	defer func() {
		_ = pctx.Close()
	}()

	workflowPlugin, err := pctx.Host.Workflow(pluginPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = workflowPlugin.Close()
	}()

	return describeWorkflow(ctx, workflowPlugin, kind, nameOrToken)
}

func describeWorkflow(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	kind string,
	nameOrToken string,
) (string, error) {
	pkgInfoResp, err := workflowPlugin.GetPackageInfo(ctx, &pulumirpc.GetPackageInfoRequest{})
	if err != nil {
		return "", fmt.Errorf("get package info: %w", err)
	}
	pkgInfo := pkgInfoResp.GetPackage()
	pkgName := ""
	pkgVersion := ""
	if pkgInfo != nil {
		pkgName = pkgInfo.GetName()
		pkgVersion = pkgInfo.GetVersion()
	}

	switch kind {
	case "job":
		token, err := resolveJobToken(ctx, workflowPlugin, nameOrToken)
		if err != nil {
			return "", err
		}
		resp, err := workflowPlugin.GetJob(ctx, &pulumirpc.GetJobRequest{Token: token})
		if err != nil {
			return "", fmt.Errorf("get job %q: %w", token, err)
		}
		job := resp.GetJob()
		if job == nil {
			return "", fmt.Errorf("job %q not found", token)
		}
		return formatDescribeOutput(pkgName, pkgVersion, "job", token,
			job.GetInputType(), job.GetOutputType(), job.GetHasOnError()), nil
	case "graph":
		token, err := resolveGraphToken(ctx, workflowPlugin, nameOrToken)
		if err != nil {
			return "", err
		}
		resp, err := workflowPlugin.GetGraph(ctx, &pulumirpc.GetGraphRequest{Token: token})
		if err != nil {
			return "", fmt.Errorf("get graph %q: %w", token, err)
		}
		graph := resp.GetGraph()
		if graph == nil {
			return "", fmt.Errorf("graph %q not found", token)
		}
		return formatDescribeOutput(pkgName, pkgVersion, "graph", token,
			graph.GetInputType(), graph.GetOutputType(), graph.GetHasOnError()), nil
	case "trigger":
		token, err := resolveTriggerToken(ctx, workflowPlugin, nameOrToken)
		if err != nil {
			return "", err
		}
		resp, err := workflowPlugin.GetTrigger(ctx, &pulumirpc.GetTriggerRequest{Token: token})
		if err != nil {
			return "", fmt.Errorf("get trigger %q: %w", token, err)
		}
		return formatDescribeOutput(pkgName, pkgVersion, "trigger", token,
			resp.GetInputType(), resp.GetOutputType(), false), nil
	default:
		return "", fmt.Errorf("unknown kind %q: expected one of graph, job, trigger", kind)
	}
}

func formatDescribeOutput(
	packageName string,
	packageVersion string,
	kind string,
	token string,
	inputType *pulumirpc.TypeReference,
	outputType *pulumirpc.TypeReference,
	hasOnError bool,
) string {
	lines := []string{
		"Package: " + packageName,
		"Version: " + packageVersion,
		"Kind: " + kind,
		"Token: " + token,
		"Input Type: " + typeToken(inputType),
		"Output Type: " + typeToken(outputType),
	}
	if kind == "job" || kind == "graph" {
		lines = append(lines, fmt.Sprintf("Has OnError: %t", hasOnError))
	}
	return strings.Join(lines, "\n")
}

func typeToken(t *pulumirpc.TypeReference) string {
	if t == nil {
		return ""
	}
	return t.GetToken()
}

func resolveGraphToken(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	graphNameOrToken string,
) (string, error) {
	// If already token-like, validate and use directly.
	if strings.Contains(graphNameOrToken, ":") {
		if _, err := workflowPlugin.GetGraph(ctx, &pulumirpc.GetGraphRequest{Token: graphNameOrToken}); err != nil {
			return "", fmt.Errorf("get graph metadata for %q: %w", graphNameOrToken, err)
		}
		return graphNameOrToken, nil
	}

	resp, err := workflowPlugin.GetGraphs(ctx, &pulumirpc.GetGraphsRequest{})
	if err != nil {
		return "", fmt.Errorf("get graphs: %w", err)
	}

	matches := make([]string, 0)
	for _, graph := range resp.GetGraphs() {
		token := graph.GetToken()
		parts := strings.Split(token, ":")
		if len(parts) > 0 && parts[len(parts)-1] == graphNameOrToken {
			matches = append(matches, token)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("graph %q not found", graphNameOrToken)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("graph name %q is ambiguous; use full token (%s)",
			graphNameOrToken, strings.Join(matches, ", "))
	}
}
