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
	"google.golang.org/protobuf/encoding/protojson"
)

func newWorkflowTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <plugin-source> <trigger-name> [args]",
		Short: "Run a trigger mock and print the trigger value as JSON",
		Long: `Run an exported workflow trigger via RunTriggerMock.

The command prints the JSON-encoded property value returned by the trigger mock.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			pluginSource := args[0]
			triggerNameOrToken := args[1]
			triggerArgs := args[2:]

			valueJSON, _, err := runTriggerFromPluginPath(ctx, pluginSource, triggerNameOrToken, triggerArgs)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), valueJSON)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "plugin-source"},
			{Name: "trigger-name"},
			{Name: "args"},
		},
		Required: 2,
	})
	return cmd
}

func runTriggerFromPluginPath(
	ctx context.Context,
	pluginPath string,
	triggerNameOrToken string,
	args []string,
) (string, string, error) {
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil, nil)
	if err != nil {
		return "", "", fmt.Errorf("create workflow plugin context: %w", err)
	}
	defer func() {
		_ = pctx.Close()
	}()

	workflowPlugin, err := pctx.Host.Workflow(pluginPath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		_ = workflowPlugin.Close()
	}()

	return runTriggerMock(ctx, workflowPlugin, triggerNameOrToken, args)
}

func runTriggerMock(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	triggerNameOrToken string,
	args []string,
) (string, string, error) {
	triggerToken, err := resolveTriggerToken(ctx, workflowPlugin, triggerNameOrToken)
	if err != nil {
		return "", "", err
	}

	resp, err := workflowPlugin.RunTriggerMock(ctx, &pulumirpc.RunTriggerMockRequest{
		Token: triggerToken,
		Args:  args,
	})
	if err != nil {
		return "", "", fmt.Errorf("run trigger mock for %q: %w", triggerToken, err)
	}

	jsonBytes, err := protojson.Marshal(resp.GetValue())
	if err != nil {
		return "", "", fmt.Errorf("marshal trigger value for %q: %w", triggerToken, err)
	}

	return string(jsonBytes), triggerToken, nil
}

func resolveTriggerToken(
	ctx context.Context,
	workflowPlugin plugin.Workflow,
	triggerNameOrToken string,
) (string, error) {
	// If already token-like, validate and use directly.
	if strings.Contains(triggerNameOrToken, ":") {
		if _, err := workflowPlugin.GetTrigger(ctx, &pulumirpc.GetTriggerRequest{Token: triggerNameOrToken}); err != nil {
			return "", fmt.Errorf("get trigger metadata for %q: %w", triggerNameOrToken, err)
		}
		return triggerNameOrToken, nil
	}

	resp, err := workflowPlugin.GetTriggers(ctx, &pulumirpc.GetTriggersRequest{})
	if err != nil {
		return "", fmt.Errorf("get triggers: %w", err)
	}

	matches := make([]string, 0)
	for _, token := range resp.GetTriggers() {
		parts := strings.Split(token, ":")
		if len(parts) > 0 && parts[len(parts)-1] == triggerNameOrToken {
			matches = append(matches, token)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("trigger %q not found", triggerNameOrToken)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("trigger name %q is ambiguous; use full token (%s)",
			triggerNameOrToken, strings.Join(matches, ", "))
	}
}
