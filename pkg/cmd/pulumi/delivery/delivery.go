// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdstack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type command struct {
	stack string
	json  bool
}

func NewDeliveryCmd() *cobra.Command {
	c := &command{stack: os.Getenv("PULUMI_DELIVERY_PIPELINE")}
	root := &cobra.Command{Use: "delivery", Short: "Manage a delivery pipeline", RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	root.PersistentFlags().StringVarP(&c.stack, "stack", "s", c.stack, "The pipeline stack (defaults to the current stack)")
	root.PersistentFlags().BoolVar(&c.json, "json", false, "Emit JSON")
	root.AddCommand(c.readCommand("status", 0, func(ctx context.Context, api *client.Client, stack client.StackIdentifier, _ []string) (any, error) {
		return api.GetDeliveryPipeline(ctx, stack)
	}))
	root.AddCommand(c.readCommand("releases [release]", 1, func(ctx context.Context, api *client.Client, stack client.StackIdentifier, args []string) (any, error) {
		if len(args) == 1 {
			return api.GetDeliveryRelease(ctx, stack, args[0])
		}
		var all client.ListDeliveryReleasesResponse
		for {
			page, err := api.ListDeliveryReleases(ctx, stack, all.ContinuationToken)
			if err != nil {
				return nil, err
			}
			all.Releases = append(all.Releases, page.Releases...)
			all.ContinuationToken = page.ContinuationToken
			if all.ContinuationToken == "" {
				return all, nil
			}
		}
	}))
	root.AddCommand(c.readCommand("history", 0, func(ctx context.Context, api *client.Client, stack client.StackIdentifier, _ []string) (any, error) {
		var passes []map[string]any
		for token := ""; ; {
			page, err := api.ListDeliveryPasses(ctx, stack, token)
			if err != nil {
				return nil, err
			}
			passes = append(passes, page.Passes...)
			token = page.ContinuationToken
			if token == "" {
				break
			}
		}
		var events []map[string]any
		for token := ""; ; {
			page, err := api.ListDeliveryEvents(ctx, stack, token)
			if err != nil {
				return nil, err
			}
			events = append(events, page.Events...)
			token = page.ContinuationToken
			if token == "" {
				break
			}
		}
		return map[string]any{"passes": passes, "events": events}, nil
	}))
	root.AddCommand(c.logsCommand())
	root.AddCommand(c.reportCommand())
	root.AddCommand(c.reportSourceCommand())
	root.AddCommand(c.previewCandidateCommand())
	root.AddCommand(c.approveCommand())
	for _, action := range []string{"pause", "resume", "retry", "redeploy", "promote", "rollback", "signal"} {
		root.AddCommand(c.actionCommand(action))
	}
	return root
}

func (c *command) reportSourceCommand() *cobra.Command {
	probeID := os.Getenv("PULUMI_DELIVERY_SOURCE_PROBE_ID")
	shapeID := os.Getenv("PULUMI_DELIVERY_SOURCE_SHAPE_ID")
	sourceURN := os.Getenv("PULUMI_DELIVERY_SOURCE_URN")
	sourceName := os.Getenv("PULUMI_DELIVERY_SOURCE_NAME")
	workflowRunID := os.Getenv("PULUMI_DELIVERY_WORKFLOW_RUN_ID")
	if workflowRunID == "" {
		workflowRunID = os.Getenv("PULUMI_WORKFLOW_RUN_ID")
	}
	commit, branch, pathsFile, pathsComplete := "", "", "", false
	cmd := &cobra.Command{Use: "report-source", Args: cobra.NoArgs, Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if probeID == "" || shapeID == "" || sourceURN == "" || sourceName == "" || workflowRunID == "" {
				return errors.New("delivery source report requires probe, shape, source, and workflow run IDs")
			}
			if strings.TrimSpace(commit) == "" {
				return errors.New("--commit is required")
			}
			api, stack, err := c.client(cmd.Context())
			if err != nil {
				return err
			}
			paths, err := readDeliverySourcePaths(pathsFile)
			if err != nil {
				return err
			}
			return api.CompleteDeliverySourceProbe(cmd.Context(), stack, client.DeliverySourceProbeRequest{
				ProbeID: probeID, ShapeID: shapeID, SourceURN: sourceURN, SourceName: sourceName,
				WorkflowRunID: workflowRunID, Commit: strings.TrimSpace(commit), Branch: branch,
				Paths: paths, PathsComplete: pathsComplete,
			})
		}}
	cmd.Flags().StringVar(&commit, "commit", "", "Resolved source commit")
	cmd.Flags().StringVar(&branch, "branch", "", "Resolved source branch")
	cmd.Flags().StringVar(&pathsFile, "paths", "", "File containing changed source paths")
	cmd.Flags().BoolVar(&pathsComplete, "paths-complete", false, "Changed paths cover the full revision range")
	return cmd
}

func readDeliverySourcePaths(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading delivery source paths: %w", err)
	}
	separator := "\n"
	if strings.ContainsRune(string(data), '\x00') {
		separator = "\x00"
	}
	var paths []string
	for _, path := range strings.Split(string(data), separator) {
		if path = strings.TrimSpace(path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (c *command) approveCommand() *cobra.Command {
	var revision int
	var comment string
	cmd := &cobra.Command{Use: "approve <change-request-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if revision <= 0 {
			return errors.New("--revision is required and must be greater than zero")
		}
		api, stack, err := c.client(cmd.Context())
		if err != nil {
			return err
		}
		return api.ApproveChangeRequest(cmd.Context(), stack.Owner, args[0], revision, comment)
	}}
	cmd.Flags().IntVar(&revision, "revision", 0, "Change request revision to approve")
	cmd.Flags().StringVar(&comment, "comment", "", "Approval comment")
	return cmd
}

func (c *command) reportCommand() *cobra.Command {
	workflowRunID := os.Getenv("PULUMI_DELIVERY_WORKFLOW_RUN_ID")
	if workflowRunID == "" {
		workflowRunID = os.Getenv("PULUMI_WORKFLOW_RUN_ID")
	}
	transition, attempt, workflowRun, phase, outputs, message, environmentRevision :=
		os.Getenv("PULUMI_DELIVERY_TRANSITION_ID"), os.Getenv("PULUMI_DELIVERY_ATTEMPT_ID"),
		workflowRunID, "", os.Getenv("PULUMI_DELIVERY_OUTPUTS"), "", ""
	cmd := &cobra.Command{Use: "report", Args: cobra.NoArgs, Hidden: true, RunE: func(cmd *cobra.Command, _ []string) error {
		if transition == "" || attempt == "" || workflowRun == "" {
			return errors.New("delivery report requires transition, attempt, and workflow run IDs")
		}
		if phase != "succeeded" && phase != "failed" {
			return errors.New("--phase must be succeeded or failed")
		}
		var raw json.RawMessage
		if outputs != "" && phase == "succeeded" {
			file, err := os.Open(outputs)
			if err != nil {
				return fmt.Errorf("reading delivery outputs: %w", err)
			}
			defer file.Close()
			data, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
			if err != nil {
				return fmt.Errorf("reading delivery outputs: %w", err)
			}
			if len(data) > 1<<20 {
				return errors.New("delivery outputs exceed the 1 MiB limit")
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(data, &object); err != nil || object == nil {
				return errors.New("delivery outputs must be a JSON object")
			}
			raw = data
		}
		api, stack, err := c.client(cmd.Context())
		if err != nil {
			return err
		}
		return api.CompleteDeliveryTransition(cmd.Context(), stack, transition,
			client.CompleteDeliveryTransitionRequest{AttemptID: attempt, WorkflowRunID: workflowRun, Phase: phase,
				Outputs: raw, Message: message, EnvironmentRevision: environmentRevision})
	}}
	cmd.Flags().StringVar(&transition, "transition", transition, "Delivery transition ID")
	cmd.Flags().StringVar(&attempt, "attempt", attempt, "Delivery attempt ID")
	cmd.Flags().StringVar(&workflowRun, "workflow-run", workflowRun, "Workflow run ID")
	cmd.Flags().StringVar(&phase, "phase", phase, "Terminal phase: succeeded or failed")
	cmd.Flags().StringVar(&outputs, "outputs", outputs, "Path to a JSON object containing declared outputs")
	cmd.Flags().StringVar(&message, "message", message, "Completion message")
	cmd.Flags().StringVar(&environmentRevision, "environment-revision", "", "Opened ESC environment revision")
	return cmd
}

type readFunc func(context.Context, *client.Client, client.StackIdentifier, []string) (any, error)

func (c *command) readCommand(use string, maxArgs int, read readFunc) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.MaximumNArgs(maxArgs), RunE: func(cmd *cobra.Command, args []string) error {
		api, stack, err := c.client(cmd.Context())
		if err != nil {
			return err
		}
		value, err := read(cmd.Context(), api, stack, args)
		if err != nil {
			return err
		}
		return c.render(cmd.OutOrStdout(), value)
	}}
}

func (c *command) actionCommand(action string) *cobra.Command {
	var version int
	var release, reason, rule string
	cmd := &cobra.Command{Use: action + " <stage>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if version <= 0 {
			return errors.New("--version is required and must be greater than zero; run `pulumi delivery status` first")
		}
		if (action == "promote" || action == "rollback" || action == "signal") && release == "" {
			return errors.New("--release is required")
		}
		api, stack, err := c.client(cmd.Context())
		if err != nil {
			return err
		}
		stage, err := api.DeliveryStageAction(cmd.Context(), stack, args[0], action, client.DeliveryStageActionRequest{
			Version: version, ReleaseID: release, Reason: reason, RuleName: rule,
		})
		if err != nil {
			return fmt.Errorf("%s stage %q at version %d: %w", action, args[0], version, err)
		}
		return c.render(cmd.OutOrStdout(), stage)
	}}
	cmd.Flags().IntVar(&version, "version", 0, "Stage version read from delivery status")
	cmd.Flags().StringVar(&release, "release", "", "Release ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Operator reason")
	cmd.Flags().StringVar(&rule, "rule", "", "Signal rule name")
	return cmd
}

func (c *command) logsCommand() *cobra.Command {
	return &cobra.Command{Use: "logs <workflow-run-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		api, stack, err := c.client(cmd.Context())
		if err != nil {
			return err
		}
		logs, err := api.GetDeliveryJobLogs(cmd.Context(), stack, args[0])
		if err != nil {
			return err
		}
		return c.render(cmd.OutOrStdout(), logs)
	}}
}

func (c *command) client(ctx context.Context) (*client.Client, client.StackIdentifier, error) {
	if parts := strings.Split(c.stack, "/"); len(parts) == 3 {
		stackName, err := tokens.ParseStackName(parts[2])
		if err != nil {
			return nil, client.StackIdentifier{}, err
		}
		backend, err := cmdBackend.NonInteractiveCurrentBackend(ctx, pkgWorkspace.Instance,
			cmdBackend.DefaultLoginManager, nil)
		if err != nil {
			return nil, client.StackIdentifier{}, err
		}
		cloudBackend, ok := backend.(httpstate.Backend)
		if !ok {
			return nil, client.StackIdentifier{}, errors.New("delivery requires the Pulumi Cloud backend")
		}
		return cloudBackend.Client(), client.StackIdentifier{
			Owner: parts[0], Project: parts[1], Stack: stackName,
		}, nil
	}
	stack, err := cmdstack.RequireStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager,
		c.stack, cmdstack.LoadOnly, display.Options{Color: cmdutil.GetGlobalColorization()}, "")
	if err != nil {
		return nil, client.StackIdentifier{}, err
	}
	cloudStack, ok := stack.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{}, errors.New("delivery requires the Pulumi Cloud backend")
	}
	project, ok := cloudStack.Ref().Project()
	if !ok {
		return nil, client.StackIdentifier{}, errors.New("the pipeline stack must include a project")
	}
	backend, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, client.StackIdentifier{}, errors.New("delivery requires the Pulumi Cloud backend")
	}
	return backend.Client(), client.StackIdentifier{Owner: cloudStack.OrgName(), Project: string(project), Stack: cloudStack.Ref().Name()}, nil
}

func (c *command) render(w io.Writer, value any) error {
	if c.json {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	switch value := value.(type) {
	case client.DeliveryPipelineSnapshot:
		if _, err := fmt.Fprintf(tw, "PIPELINE\t%s\nID\t%s\nSHAPE VERSION\t%d\nNEWEST RELEASE\t%s\n\nSTAGE\tSTATE\tRELEASE\tVERSION\tWAITING\n",
			value.Name, value.ID, value.ShapeVersion, value.NewestReleaseID); err != nil {
			return err
		}
		for _, stage := range value.Stages {
			waiting := ""
			if rule, ok := stage.Rule.(map[string]any); ok {
				waiting = fmt.Sprint(rule["waitingOn"])
			}
			release := ""
			if stage.Desired != nil {
				release = stage.Desired.ReleaseID
			}
			state := stage.State
			if stage.Paused {
				state += " (paused: " + stage.PauseReason + ")"
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", stage.Name, state, release, stage.Version, waiting); err != nil {
				return err
			}
			for _, member := range stage.Members {
				reason := ""
				if transition, ok := member["transition"].(map[string]any); ok {
					reason, _ = transition["message"].(string)
				}
				if _, err := fmt.Fprintf(tw, "  %v\t%v\t\t\t%s\n", member["name"], member["state"], reason); err != nil {
					return err
				}
			}
		}
	case client.DeliveryJobLogsResponse:
		if _, err := fmt.Fprintf(tw, "STATUS\t%s\n", value.Status); err != nil {
			return err
		}
		for _, line := range value.Lines {
			header, _ := line["header"].(string)
			text, _ := line["line"].(string)
			if _, err := fmt.Fprintf(tw, "%v\t%s%s\n", line["timestamp"], header, text); err != nil {
				return err
			}
		}
	default:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(tw, string(data)); err != nil {
			return err
		}
	}
	return tw.Flush()
}
