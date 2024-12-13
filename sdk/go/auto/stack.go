// Copyright 2016-2022, Pulumi Corporation.
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

// Package auto contains the Pulumi Automation API, the programmatic interface for driving Pulumi programs
// without the CLI.
// Generally this can be thought of as encapsulating the functionality of the CLI (`pulumi up`, `pulumi preview`,
// pulumi destroy`, `pulumi stack init`, etc.) but with more flexibility. This still requires a
// CLI binary to be installed and available on your $PATH.
//
// In addition to fine-grained building blocks, Automation API provides three out of the box ways to work with Stacks:
//
//  1. Programs locally available on-disk and addressed via a filepath (NewStackLocalSource)
//     stack, err := NewStackLocalSource(ctx, "myOrg/myProj/myStack", filepath.Join("..", "path", "to", "project"))
//
//  2. Programs fetched from a Git URL (NewStackRemoteSource)
//     stack, err := NewStackRemoteSource(ctx, "myOrg/myProj/myStack", GitRepo{
//     URL:         "https://github.com/pulumi/test-repo.git",
//     ProjectPath: filepath.Join("project", "path", "repo", "root", "relative"),
//     })
//
//  3. Programs defined as a function alongside your Automation API code (NewStackInlineSource)
//     stack, err := NewStackInlineSource(ctx, "myOrg/myProj/myStack", func(pCtx *pulumi.Context) error {
//     bucket, err := s3.NewBucket(pCtx, "bucket", nil)
//     if err != nil {
//     return err
//     }
//     pCtx.Export("bucketName", bucket.Bucket)
//     return nil
//     })
//
// Each of these creates a stack with access to the full range of Pulumi lifecycle methods
// (up/preview/refresh/destroy), as well as methods for managing config, stack, and project settings.
//
//	err := stack.SetConfig(ctx, "key", ConfigValue{ Value: "value", Secret: true })
//	preRes, err := stack.Preview(ctx)
//	// detailed info about results
//	fmt.Println(preRes.prev.Steps[0].URN)
//
// The Automation API provides a natural way to orchestrate multiple stacks,
// feeding the output of one stack as an input to the next as shown in the package-level example below.
// The package can be used for a number of use cases:
//
//   - Driving pulumi deployments within CI/CD workflows
//
//   - Integration testing
//
//   - Multi-stage deployments such as blue-green deployment patterns
//
//   - Deployments involving application code like database migrations
//
//   - Building higher level tools, custom CLIs over pulumi, etc
//
//   - Using pulumi behind a REST or GRPC API
//
//   - Debugging Pulumi programs (by using a single main entrypoint with "inline" programs)
//
// To enable a broad range of runtime customization the API defines a `Workspace` interface.
// A Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
// Workspaces are used to manage the execution environment, providing various utilities such as plugin
// installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
// Every Stack including those in the above examples are backed by a Workspace which can be accessed via:
//
//	w = stack.Workspace()
//	err := w.InstallPlugin("aws", "v3.2.0")
//
// Workspaces can be explicitly created and customized beyond the three Stack creation helpers noted above:
//
//	w, err := NewLocalWorkspace(ctx, WorkDir(filepath.Join(".", "project", "path"), PulumiHome("~/.pulumi"))
//	s := NewStack(ctx, "org/proj/stack", w)
//
// A default implementation of workspace is provided as `LocalWorkspace`. This implementation relies on Pulumi.yaml
// and Pulumi.<stack>.yaml as the intermediate format for Project and Stack settings. Modifying ProjectSettings will
// alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.<stack>.yaml file.
// This is identical to the behavior of Pulumi CLI driven workspaces. Custom Workspace
// implementations can be used to store Project and Stack settings as well as Config in a different format,
// such as an in-memory data structure, a shared persistent SQL database, or cloud object storage. Regardless of
// the backing Workspace implementation, the Pulumi SaaS Console will still be able to display configuration
// applied to updates as it does with the local version of the Workspace today.
//
// The Automation API also provides error handling utilities to detect common cases such as concurrent update
// conflicts:
//
//	uRes, err :=stack.Up(ctx)
//	if err != nil && IsConcurrentUpdateError(err) { /* retry logic here */ }
package auto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optimport"

	"github.com/blang/semver"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/opthistory"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tail"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Stack is an isolated, independently configurable instance of a Pulumi program.
// Stack exposes methods for the full pulumi lifecycle (up/preview/refresh/destroy), as well as managing configuration.
// Multiple Stacks are commonly used to denote different phases of development
// (such as development, staging and production) or feature branches (such as feature-x-dev, jane-feature-x-dev).
type Stack struct {
	workspace Workspace
	stackName string
}

// FullyQualifiedStackName returns a stack name formatted with the greatest possible specificity:
// org/project/stack or user/project/stack
// Using this format avoids ambiguity in stack identity guards creating or selecting the wrong stack.
// Note that legacy diy backends (local file, S3, Azure Blob) do not support stack names in this
// format, and instead only use the stack name without an org/user or project to qualify it.
// See: https://github.com/pulumi/pulumi/issues/2522.
// Non-legacy diy backends do support the org/project/stack format but org must be set to "organization".
func FullyQualifiedStackName(org, project, stack string) string {
	return fmt.Sprintf("%s/%s/%s", org, project, stack)
}

// NewStack creates a new stack using the given workspace, and stack name.
// It fails if a stack with that name already exists
func NewStack(ctx context.Context, stackName string, ws Workspace) (Stack, error) {
	s := Stack{
		workspace: ws,
		stackName: stackName,
	}

	err := ws.CreateStack(ctx, stackName)
	if err != nil {
		return s, err
	}

	return s, nil
}

// SelectStack selects stack using the given workspace, and stack name.
// It returns an error if the given Stack does not exist.
func SelectStack(ctx context.Context, stackName string, ws Workspace) (Stack, error) {
	s := Stack{
		workspace: ws,
		stackName: stackName,
	}

	err := ws.SelectStack(ctx, stackName)
	if err != nil {
		return s, err
	}

	return s, nil
}

// UpsertStack tries to select a stack using the given workspace and
// stack name, or falls back to trying to create the stack if
// it does not exist.
func UpsertStack(ctx context.Context, stackName string, ws Workspace) (Stack, error) {
	s, err := SelectStack(ctx, stackName, ws)
	// If the stack is not found, attempt to create it.
	if err != nil && IsSelectStack404Error(err) {
		return NewStack(ctx, stackName, ws)
	}
	return s, err
}

// Name returns the stack name
func (s *Stack) Name() string {
	return s.stackName
}

// Workspace returns the underlying Workspace backing the Stack.
// This handles state associated with the Project and child Stacks including
// settings, configuration, and environment.
func (s *Stack) Workspace() Workspace {
	return s.workspace
}

// ChangeSecretsProvider edits the secrets provider for the stack.
func (s *Stack) ChangeSecretsProvider(
	ctx context.Context, newSecretsProvider string, opts *ChangeSecretsProviderOptions,
) error {
	return s.workspace.ChangeStackSecretsProvider(ctx, s.stackName, newSecretsProvider, opts)
}

// Preview preforms a dry-run update to a stack, returning pending changes.
// https://www.pulumi.com/docs/cli/commands/pulumi_preview/
func (s *Stack) Preview(ctx context.Context, opts ...optpreview.Option) (PreviewResult, error) {
	var res PreviewResult

	preOpts := &optpreview.Options{}
	for _, o := range opts {
		o.ApplyOption(preOpts)
	}

	bufferSizeHint := len(preOpts.Replace) + len(preOpts.Target) +
		len(preOpts.PolicyPacks) + len(preOpts.PolicyPackConfigs)
	sharedArgs := slice.Prealloc[string](bufferSizeHint)

	sharedArgs = debug.AddArgs(&preOpts.DebugLogOpts, sharedArgs)
	if preOpts.Message != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--message=%q", preOpts.Message))
	}
	if preOpts.ExpectNoChanges {
		sharedArgs = append(sharedArgs, "--expect-no-changes")
	}
	if preOpts.Diff {
		sharedArgs = append(sharedArgs, "--diff")
	}
	for _, rURN := range preOpts.Replace {
		sharedArgs = append(sharedArgs, "--replace="+rURN)
	}
	for _, tURN := range preOpts.Target {
		sharedArgs = append(sharedArgs, "--target="+tURN)
	}
	for _, pack := range preOpts.PolicyPacks {
		sharedArgs = append(sharedArgs, "--policy-pack="+pack)
	}
	for _, packConfig := range preOpts.PolicyPackConfigs {
		sharedArgs = append(sharedArgs, "--policy-pack-config="+packConfig)
	}
	if preOpts.TargetDependents {
		sharedArgs = append(sharedArgs, "--target-dependents")
	}
	if preOpts.Parallel > 0 {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--parallel=%d", preOpts.Parallel))
	}
	if preOpts.UserAgent != "" {
		sharedArgs = append(sharedArgs, "--exec-agent="+preOpts.UserAgent)
	}
	if preOpts.Color != "" {
		sharedArgs = append(sharedArgs, "--color="+preOpts.Color)
	}
	if preOpts.Plan != "" {
		sharedArgs = append(sharedArgs, "--save-plan="+preOpts.Plan)
	}
	if preOpts.Refresh {
		sharedArgs = append(sharedArgs, "--refresh")
	}
	if preOpts.SuppressOutputs {
		sharedArgs = append(sharedArgs, "--suppress-outputs")
	}
	if preOpts.SuppressProgress {
		sharedArgs = append(sharedArgs, "--suppress-progress")
	}
	if preOpts.ImportFile != "" {
		sharedArgs = append(sharedArgs, "--import-file="+preOpts.ImportFile)
	}
	if preOpts.AttachDebugger {
		sharedArgs = append(sharedArgs, "--attach-debugger")
	}
	if preOpts.ConfigFile != "" {
		sharedArgs = append(sharedArgs, "--config-file="+preOpts.ConfigFile)
	}

	// Apply the remote args, if needed.
	sharedArgs = append(sharedArgs, s.remoteArgs()...)

	kind, args := constant.ExecKindAutoLocal, []string{"preview"}
	if program := s.Workspace().Program(); program != nil {
		server, err := startLanguageRuntimeServer(program)
		if err != nil {
			return res, err
		}
		defer contract.IgnoreClose(server)

		kind, args = constant.ExecKindAutoInline, append(args, "--client="+server.address)
	}

	args = append(args, "--exec-kind="+kind)
	args = append(args, sharedArgs...)

	var summaryEvents []apitype.SummaryEvent
	eventChannel := make(chan events.EngineEvent)
	eventsDone := make(chan bool)
	go func() {
		for {
			event, ok := <-eventChannel
			if !ok {
				close(eventsDone)
				return
			}
			if event.SummaryEvent != nil {
				summaryEvents = append(summaryEvents, *event.SummaryEvent)
			}
		}
	}()

	eventChannels := []chan<- events.EngineEvent{eventChannel}
	eventChannels = append(eventChannels, preOpts.EventStreams...)

	t, err := tailLogs("preview", eventChannels)
	if err != nil {
		return res, fmt.Errorf("failed to tail logs: %w", err)
	}
	defer t.Close()
	args = append(args, "--event-log", t.Filename)

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		preOpts.ProgressStreams,      /* additionalOutput */
		preOpts.ErrorProgressStreams, /* additionalErrorOutput */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to run preview: %w", err), stdout, stderr, code)
	}

	// Close the file watcher wait for all events to send
	t.Close()
	<-eventsDone

	if len(summaryEvents) == 0 {
		return res, newAutoError(errors.New("failed to get preview summary"), stdout, stderr, code)
	}
	if len(summaryEvents) > 1 {
		return res, newAutoError(errors.New("got multiple preview summaries"), stdout, stderr, code)
	}

	res.StdOut = stdout
	res.StdErr = stderr
	res.ChangeSummary = summaryEvents[0].ResourceChanges

	return res, nil
}

// Up creates or updates the resources in a stack by executing the program in the Workspace.
// https://www.pulumi.com/docs/cli/commands/pulumi_up/
func (s *Stack) Up(ctx context.Context, opts ...optup.Option) (UpResult, error) {
	var res UpResult

	upOpts := &optup.Options{}
	for _, o := range opts {
		o.ApplyOption(upOpts)
	}

	bufferSizeHint := len(upOpts.Replace) + len(upOpts.Target) + len(upOpts.PolicyPacks) + len(upOpts.PolicyPackConfigs)
	sharedArgs := slice.Prealloc[string](bufferSizeHint)

	sharedArgs = debug.AddArgs(&upOpts.DebugLogOpts, sharedArgs)
	if upOpts.Message != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--message=%q", upOpts.Message))
	}
	if upOpts.ExpectNoChanges {
		sharedArgs = append(sharedArgs, "--expect-no-changes")
	}
	if upOpts.Diff {
		sharedArgs = append(sharedArgs, "--diff")
	}
	for _, rURN := range upOpts.Replace {
		sharedArgs = append(sharedArgs, "--replace="+rURN)
	}
	for _, tURN := range upOpts.Target {
		sharedArgs = append(sharedArgs, "--target="+tURN)
	}
	for _, pack := range upOpts.PolicyPacks {
		sharedArgs = append(sharedArgs, "--policy-pack="+pack)
	}
	for _, packConfig := range upOpts.PolicyPackConfigs {
		sharedArgs = append(sharedArgs, "--policy-pack-config="+packConfig)
	}
	if upOpts.TargetDependents {
		sharedArgs = append(sharedArgs, "--target-dependents")
	}
	if upOpts.Parallel > 0 {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--parallel=%d", upOpts.Parallel))
	}
	if upOpts.UserAgent != "" {
		sharedArgs = append(sharedArgs, "--exec-agent="+upOpts.UserAgent)
	}
	if upOpts.Color != "" {
		sharedArgs = append(sharedArgs, "--color="+upOpts.Color)
	}
	if upOpts.Plan != "" {
		sharedArgs = append(sharedArgs, "--plan="+upOpts.Plan)
	}
	if upOpts.Refresh {
		sharedArgs = append(sharedArgs, "--refresh")
	}
	if upOpts.SuppressOutputs {
		sharedArgs = append(sharedArgs, "--suppress-outputs")
	}
	if upOpts.SuppressProgress {
		sharedArgs = append(sharedArgs, "--suppress-progress")
	}
	if upOpts.ContinueOnError {
		sharedArgs = append(sharedArgs, "--continue-on-error")
	}
	if upOpts.AttachDebugger {
		sharedArgs = append(sharedArgs, "--attach-debugger")
	}
	if upOpts.ConfigFile != "" {
		sharedArgs = append(sharedArgs, "--config-file="+upOpts.ConfigFile)
	}

	// Apply the remote args, if needed.
	sharedArgs = append(sharedArgs, s.remoteArgs()...)

	kind, args := constant.ExecKindAutoLocal, []string{"up", "--yes", "--skip-preview"}
	if program := s.Workspace().Program(); program != nil {
		server, err := startLanguageRuntimeServer(program)
		if err != nil {
			return res, err
		}
		defer contract.IgnoreClose(server)

		kind, args = constant.ExecKindAutoInline, append(args, "--client="+server.address)
	}
	args = append(args, "--exec-kind="+kind)

	if len(upOpts.EventStreams) > 0 {
		eventChannels := upOpts.EventStreams
		t, err := tailLogs("up", eventChannels)
		if err != nil {
			return res, fmt.Errorf("failed to tail logs: %w", err)
		}
		defer t.Close()
		args = append(args, "--event-log", t.Filename)
	}

	args = append(args, sharedArgs...)
	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, upOpts.ProgressStreams, upOpts.ErrorProgressStreams, args...)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to run update: %w", err), stdout, stderr, code)
	}

	outs, err := s.Outputs(ctx)
	if err != nil {
		return res, err
	}

	historyOpts := []opthistory.Option{}
	if upOpts.ShowSecrets != nil {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(*upOpts.ShowSecrets))
	}
	// If it's a remote workspace, explicitly set ShowSecrets to false to prevent attempting to
	// load the project file.
	if s.isRemote() {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(false))
	}
	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/, historyOpts...)
	if err != nil {
		return res, err
	}

	res = UpResult{
		Outputs: outs,
		StdOut:  stdout,
		StdErr:  stderr,
	}

	if len(history) > 0 {
		res.Summary = history[0]
	}

	return res, nil
}

// ImportResources imports resources into a stack using the given resources and options.
func (s *Stack) ImportResources(ctx context.Context, opts ...optimport.Option) (ImportResult, error) {
	var res ImportResult

	importOpts := &optimport.Options{}
	for _, o := range opts {
		o.ApplyOption(importOpts)
	}

	tempDir, err := os.MkdirTemp("", "pulumi-import-")
	if err != nil {
		return res, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// clean-up the temp directory after we are done
	defer os.RemoveAll(tempDir)

	args := []string{"import", "--yes", "--skip-preview"}

	if importOpts.Resources != nil {
		importFilePath := filepath.Join(tempDir, "import.json")
		importContent := map[string]interface{}{
			"resources": importOpts.Resources,
		}

		if importOpts.NameTable != nil {
			importContent["nameTable"] = importOpts.NameTable
		}

		importContentBytes, err := json.Marshal(importContent)
		if err != nil {
			return res, fmt.Errorf("failed to marshal import content: %w", err)
		}

		if err := os.WriteFile(importFilePath, importContentBytes, 0o600); err != nil {
			return res, fmt.Errorf("failed to write import file: %w", err)
		}

		args = append(args, "--file", importFilePath)
	}

	if importOpts.Protect != nil && !*importOpts.Protect {
		// the protect flag is true by default so only add the flag if it's explicitly set to false
		args = append(args, "--protect=false")
	}

	generatedCodePath := filepath.Join(tempDir, "generated_code.txt")
	if importOpts.GenerateCode != nil && !*importOpts.GenerateCode {
		// the generate code flag is true by default so only add the flag if it's explicitly set to false
		args = append(args, "--generate-code=false")
	} else {
		args = append(args, "--out", generatedCodePath)
	}

	if importOpts.Converter != nil {
		args = append(args, "--from", *importOpts.Converter)
		if importOpts.ConverterArgs != nil {
			args = append(args, "--")
			args = append(args, importOpts.ConverterArgs...)
		}
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		importOpts.ProgressStreams,      /* additionalOutputs */
		importOpts.ErrorProgressStreams, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to import resources: %w", err), stdout, stderr, code)
	}

	history, err := s.History(ctx, 1 /*pageSize*/, 1, /*page*/
		opthistory.ShowSecrets(importOpts.ShowSecrets && !s.isRemote()))
	if err != nil {
		return res, fmt.Errorf("failed to import resources: %w", err)
	}

	generatedCode, err := os.ReadFile(generatedCodePath)
	if err != nil {
		return res, fmt.Errorf("failed to read generated code: %w", err)
	}

	var summary UpdateSummary
	if len(history) > 0 {
		summary = history[0]
	}

	res = ImportResult{
		Summary:       summary,
		StdOut:        stdout,
		StdErr:        stderr,
		GeneratedCode: string(generatedCode),
	}

	return res, nil
}

func (s *Stack) PreviewRefresh(ctx context.Context, opts ...optrefresh.Option) (PreviewResult, error) {
	var res PreviewResult

	// 3.105.0 added this flag (https://github.com/pulumi/pulumi/releases/tag/v3.105.0)
	if s.Workspace().PulumiCommand().Version().LT(semver.Version{Major: 3, Minor: 105}) {
		return res, errors.New("PreviewRefresh requires Pulumi CLI version >= 3.105.0")
	}

	refreshOpts := &optrefresh.Options{}
	for _, o := range opts {
		o.ApplyOption(refreshOpts)
	}

	args := refreshOptsToCmd(refreshOpts, s, true /*isPreview*/)

	var summaryEvents []apitype.SummaryEvent
	eventChannel := make(chan events.EngineEvent)
	eventsDone := make(chan bool)
	go func() {
		for {
			event, ok := <-eventChannel
			if !ok {
				close(eventsDone)
				return
			}
			if event.SummaryEvent != nil {
				summaryEvents = append(summaryEvents, *event.SummaryEvent)
			}
		}
	}()

	eventChannels := []chan<- events.EngineEvent{eventChannel}
	eventChannels = append(eventChannels, refreshOpts.EventStreams...)

	t, err := tailLogs("refresh", eventChannels)
	if err != nil {
		return res, fmt.Errorf("failed to tail logs: %w", err)
	}
	defer t.Close()
	args = append(args, "--event-log", t.Filename)

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		refreshOpts.ProgressStreams,      /* additionalOutputs */
		refreshOpts.ErrorProgressStreams, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to preview refresh: %w", err), stdout, stderr, code)
	}

	// Close the file watcher wait for all events to send
	t.Close()
	<-eventsDone

	if len(summaryEvents) == 0 {
		return res, newAutoError(errors.New("failed to get preview refresh summary"), stdout, stderr, code)
	}
	if len(summaryEvents) > 1 {
		return res, newAutoError(errors.New("got multiple preview refresh summaries"), stdout, stderr, code)
	}

	res = PreviewResult{
		ChangeSummary: summaryEvents[0].ResourceChanges,
		StdOut:        stdout,
		StdErr:        stderr,
	}

	return res, nil
}

// Refresh compares the current stack’s resource state with the state known to exist in the actual
// cloud provider. Any such changes are adopted into the current stack.
func (s *Stack) Refresh(ctx context.Context, opts ...optrefresh.Option) (RefreshResult, error) {
	var res RefreshResult

	refreshOpts := &optrefresh.Options{}
	for _, o := range opts {
		o.ApplyOption(refreshOpts)
	}

	args := refreshOptsToCmd(refreshOpts, s, false /*isPreview*/)

	if len(refreshOpts.EventStreams) > 0 {
		eventChannels := refreshOpts.EventStreams
		t, err := tailLogs("refresh", eventChannels)
		if err != nil {
			return res, fmt.Errorf("failed to tail logs: %w", err)
		}
		defer t.Close()
		args = append(args, "--event-log", t.Filename)
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		refreshOpts.ProgressStreams,      /* additionalOutputs */
		refreshOpts.ErrorProgressStreams, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to refresh stack: %w", err), stdout, stderr, code)
	}

	historyOpts := []opthistory.Option{}
	if showSecrets := refreshOpts.ShowSecrets; showSecrets != nil {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(*showSecrets))
	}
	// If it's a remote workspace, explicitly set ShowSecrets to false to prevent attempting to
	// load the project file.
	if s.isRemote() {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(false))
	}
	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/, historyOpts...)
	if err != nil {
		return res, fmt.Errorf("failed to refresh stack: %w", err)
	}

	var summary UpdateSummary
	if len(history) > 0 {
		summary = history[0]
	}

	res = RefreshResult{
		Summary: summary,
		StdOut:  stdout,
		StdErr:  stderr,
	}

	return res, nil
}

func refreshOptsToCmd(o *optrefresh.Options, s *Stack, isPreview bool) []string {
	args := slice.Prealloc[string](len(o.Target))

	args = debug.AddArgs(&o.DebugLogOpts, args)
	args = append(args, "refresh")
	if isPreview {
		args = append(args, "--preview-only")
	} else {
		args = append(args, "--yes", "--skip-preview")
	}
	if o.Message != "" {
		args = append(args, fmt.Sprintf("--message=%q", o.Message))
	}
	if o.ExpectNoChanges {
		args = append(args, "--expect-no-changes")
	}
	for _, tURN := range o.Target {
		args = append(args, "--target="+tURN)
	}
	if o.Parallel > 0 {
		args = append(args, fmt.Sprintf("--parallel=%d", o.Parallel))
	}
	if o.UserAgent != "" {
		args = append(args, "--exec-agent="+o.UserAgent)
	}
	if o.Color != "" {
		args = append(args, "--color="+o.Color)
	}
	if o.SuppressOutputs {
		args = append(args, "--suppress-outputs")
	}
	if o.SuppressProgress {
		args = append(args, "--suppress-progress")
	}
	if o.ConfigFile != "" {
		args = append(args, "--config-file="+o.ConfigFile)
	}

	// Apply the remote args, if needed.
	args = append(args, s.remoteArgs()...)

	execKind := constant.ExecKindAutoLocal
	if s.Workspace().Program() != nil {
		execKind = constant.ExecKindAutoInline
	}
	args = append(args, "--exec-kind="+execKind)

	return args
}

func (s *Stack) PreviewDestroy(ctx context.Context, opts ...optdestroy.Option) (PreviewResult, error) {
	var res PreviewResult

	// 3.105.0 added this flag (https://github.com/pulumi/pulumi/releases/tag/v3.105.0)
	if minVer := (semver.Version{Major: 3, Minor: 105}); s.Workspace().PulumiCommand().Version().LT(minVer) {
		return res, fmt.Errorf("PreviewRefresh requires Pulumi CLI version >= %s", minVer)
	}

	destroyOpts := &optdestroy.Options{}
	for _, o := range opts {
		o.ApplyOption(destroyOpts)
	}

	args := destroyOptsToCmd(destroyOpts, s)
	args = append(args, "--preview-only")

	var summaryEvents []apitype.SummaryEvent
	eventChannel := make(chan events.EngineEvent)
	eventsDone := make(chan bool)
	go func() {
		for {
			event, ok := <-eventChannel
			if !ok {
				close(eventsDone)
				return
			}
			if event.SummaryEvent != nil {
				summaryEvents = append(summaryEvents, *event.SummaryEvent)
			}
		}
	}()

	eventChannels := []chan<- events.EngineEvent{eventChannel}
	eventChannels = append(eventChannels, destroyOpts.EventStreams...)
	t, err := tailLogs("destroy", eventChannels)
	if err != nil {
		return res, fmt.Errorf("failed to tail logs: %w", err)
	}
	defer t.Close()
	args = append(args, "--event-log", t.Filename)

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		destroyOpts.ProgressStreams,      /* additionalOutputs */
		destroyOpts.ErrorProgressStreams, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to preview destroy: %w", err), stdout, stderr, code)
	}

	// Close the file watcher wait for all events to send
	t.Close()
	<-eventsDone

	if len(summaryEvents) == 0 {
		return res, newAutoError(errors.New("failed to get preview refresh summary"), stdout, stderr, code)
	}
	if len(summaryEvents) > 1 {
		return res, newAutoError(errors.New("got multiple preview refresh summaries"), stdout, stderr, code)
	}

	res = PreviewResult{
		ChangeSummary: summaryEvents[0].ResourceChanges,
		StdOut:        stdout,
		StdErr:        stderr,
	}

	return res, nil
}

// Destroy deletes all resources in a stack, leaving all history and configuration intact.
func (s *Stack) Destroy(ctx context.Context, opts ...optdestroy.Option) (DestroyResult, error) {
	var res DestroyResult

	destroyOpts := &optdestroy.Options{}
	for _, o := range opts {
		o.ApplyOption(destroyOpts)
	}

	args := destroyOptsToCmd(destroyOpts, s)
	args = append(args, "--yes", "--skip-preview")

	if len(destroyOpts.EventStreams) > 0 {
		eventChannels := destroyOpts.EventStreams
		t, err := tailLogs("destroy", eventChannels)
		if err != nil {
			return res, fmt.Errorf("failed to tail logs: %w", err)
		}
		defer t.Close()
		args = append(args, "--event-log", t.Filename)
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(
		ctx,
		destroyOpts.ProgressStreams,      /* additionalOutputs */
		destroyOpts.ErrorProgressStreams, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return res, newAutoError(fmt.Errorf("failed to destroy stack: %w", err), stdout, stderr, code)
	}

	historyOpts := []opthistory.Option{}
	if showSecrets := destroyOpts.ShowSecrets; showSecrets != nil {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(*showSecrets))
	}
	// If it's a remote workspace, explicitly set ShowSecrets to false to prevent attempting to
	// load the project file.
	if s.isRemote() {
		historyOpts = append(historyOpts, opthistory.ShowSecrets(false))
	}
	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/, historyOpts...)
	if err != nil {
		return res, fmt.Errorf("failed to destroy stack: %w", err)
	}

	var summary UpdateSummary
	if len(history) > 0 {
		summary = history[0]
	}

	// If `remove` was set, remove the stack now. We take this approach rather
	// than passing `--remove` to `pulumi destroy` because the latter would make
	// it impossible for us to retrieve a summary of the operation above for
	// returning to the caller.
	if destroyOpts.Remove {
		if err := s.Workspace().RemoveStack(ctx, s.Name()); err != nil {
			return res, fmt.Errorf("failed to remove stack: %w", err)
		}
	}

	res = DestroyResult{
		Summary: summary,
		StdOut:  stdout,
		StdErr:  stderr,
	}

	return res, nil
}

func destroyOptsToCmd(destroyOpts *optdestroy.Options, s *Stack) []string {
	args := slice.Prealloc[string](len(destroyOpts.Target))

	args = debug.AddArgs(&destroyOpts.DebugLogOpts, args)
	args = append(args, "destroy")
	if destroyOpts.Message != "" {
		args = append(args, fmt.Sprintf("--message=%q", destroyOpts.Message))
	}
	for _, tURN := range destroyOpts.Target {
		args = append(args, "--target="+tURN)
	}
	if destroyOpts.TargetDependents {
		args = append(args, "--target-dependents")
	}
	if destroyOpts.Parallel > 0 {
		args = append(args, fmt.Sprintf("--parallel=%d", destroyOpts.Parallel))
	}
	if destroyOpts.UserAgent != "" {
		args = append(args, "--exec-agent="+destroyOpts.UserAgent)
	}
	if destroyOpts.Color != "" {
		args = append(args, "--color="+destroyOpts.Color)
	}
	if destroyOpts.Refresh {
		args = append(args, "--refresh")
	}
	if destroyOpts.SuppressOutputs {
		args = append(args, "--suppress-outputs")
	}
	if destroyOpts.SuppressProgress {
		args = append(args, "--suppress-progress")
	}
	if destroyOpts.ContinueOnError {
		args = append(args, "--continue-on-error")
	}
	if destroyOpts.ConfigFile != "" {
		args = append(args, "--config-file="+destroyOpts.ConfigFile)
	}

	execKind := constant.ExecKindAutoLocal
	if s.Workspace().Program() != nil {
		execKind = constant.ExecKindAutoInline
	}
	args = append(args, "--exec-kind="+execKind)

	// Apply the remote args, if needed.
	args = append(args, s.remoteArgs()...)

	return args
}

// Outputs get the current set of Stack outputs from the last Stack.Up().
func (s *Stack) Outputs(ctx context.Context) (OutputMap, error) {
	return s.Workspace().StackOutputs(ctx, s.Name())
}

// History returns a list summarizing all previous and current results from Stack lifecycle operations
// (up/preview/refresh/destroy).
func (s *Stack) History(ctx context.Context,
	pageSize int, page int, opts ...opthistory.Option,
) ([]UpdateSummary, error) {
	var options opthistory.Options
	for _, opt := range opts {
		opt.ApplyOption(&options)
	}
	showSecrets := true
	if options.ShowSecrets != nil {
		showSecrets = *options.ShowSecrets
	}
	args := []string{"stack", "history", "--json"}
	if showSecrets {
		args = append(args, "--show-secrets")
	}
	if pageSize > 0 {
		// default page=1 if unset when pageSize is set
		if page < 1 {
			page = 1
		}
		args = append(args, "--page-size", strconv.Itoa(pageSize), "--page", strconv.Itoa(page))
	}

	stdout, stderr, errCode, err := s.runPulumiCmdSync(
		ctx,
		nil, /* additionalOutputs */
		nil, /* additionalErrorOutputs */
		args...,
	)
	if err != nil {
		return nil, newAutoError(fmt.Errorf("failed to get stack history: %w", err), stdout, stderr, errCode)
	}

	var history []UpdateSummary
	err = json.Unmarshal([]byte(stdout), &history)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal history result: %w", err)
	}

	return history, nil
}

// AddEnvironments adds environments to the end of a stack's import list. Imported environments are merged in order
// per the ESC merge rules. The list of environments behaves as if it were the import list in an anonymous
// environment.
func (s *Stack) AddEnvironments(ctx context.Context, envs ...string) error {
	return s.Workspace().AddEnvironments(ctx, s.Name(), envs...)
}

// ListEnvironments returns the list of environments from the stack's configuration.
func (s *Stack) ListEnvironments(ctx context.Context) ([]string, error) {
	return s.Workspace().ListEnvironments(ctx, s.Name())
}

// RemoveEnvironment removes an environment from a stack's configuration.
func (s *Stack) RemoveEnvironment(ctx context.Context, env string) error {
	return s.Workspace().RemoveEnvironment(ctx, s.Name(), env)
}

// GetConfig returns the config value associated with the specified key.
func (s *Stack) GetConfig(ctx context.Context, key string) (ConfigValue, error) {
	return s.Workspace().GetConfig(ctx, s.Name(), key)
}

// GetConfigWithOptions returns the config value associated with the specified key using the optional ConfigOptions.
func (s *Stack) GetConfigWithOptions(ctx context.Context, key string, opts *ConfigOptions) (ConfigValue, error) {
	return s.Workspace().GetConfigWithOptions(ctx, s.Name(), key, opts)
}

// GetAllConfig returns the full config map.
func (s *Stack) GetAllConfig(ctx context.Context) (ConfigMap, error) {
	return s.Workspace().GetAllConfig(ctx, s.Name())
}

// GetAllConfigWithOptions returns the full config map with optional ConfigAllConfigOptions.
// Allows using a config file and controlling how secrets are shown
func (s *Stack) GetAllConfigWithOptions(ctx context.Context, opts *GetAllConfigOptions) (ConfigMap, error) {
	return s.Workspace().GetAllConfigWithOptions(ctx, s.Name(), opts)
}

// SetConfig sets the specified config key-value pair.
func (s *Stack) SetConfig(ctx context.Context, key string, val ConfigValue) error {
	return s.Workspace().SetConfig(ctx, s.Name(), key, val)
}

// SetConfigWithOptions sets the specified config key-value pair using the optional ConfigOptions.
func (s *Stack) SetConfigWithOptions(ctx context.Context, key string, val ConfigValue, opts *ConfigOptions) error {
	return s.Workspace().SetConfigWithOptions(ctx, s.Name(), key, val, opts)
}

// SetAllConfig sets all values in the provided config map.
func (s *Stack) SetAllConfig(ctx context.Context, config ConfigMap) error {
	return s.Workspace().SetAllConfig(ctx, s.Name(), config)
}

// SetAllConfigWithOptions sets all values in the provided config map using the optional ConfigOptions.
func (s *Stack) SetAllConfigWithOptions(ctx context.Context, config ConfigMap, opts *ConfigOptions) error {
	return s.Workspace().SetAllConfigWithOptions(ctx, s.Name(), config, opts)
}

// RemoveConfig removes the specified config key-value pair.
func (s *Stack) RemoveConfig(ctx context.Context, key string) error {
	return s.Workspace().RemoveConfig(ctx, s.Name(), key)
}

// RemoveConfigWithOptions removes the specified config key-value pair using the optional ConfigOptions.
func (s *Stack) RemoveConfigWithOptions(ctx context.Context, key string, opts *ConfigOptions) error {
	return s.Workspace().RemoveConfigWithOptions(ctx, s.Name(), key, opts)
}

// RemoveAllConfig removes all values in the provided list of keys.
func (s *Stack) RemoveAllConfig(ctx context.Context, keys []string) error {
	return s.Workspace().RemoveAllConfig(ctx, s.Name(), keys)
}

// RemoveAllConfigWithOptions removes all values in the provided list of keys using the optional ConfigOptions.
func (s *Stack) RemoveAllConfigWithOptions(ctx context.Context, keys []string, opts *ConfigOptions) error {
	return s.Workspace().RemoveAllConfigWithOptions(ctx, s.Name(), keys, opts)
}

// RefreshConfig gets and sets the config map used with the last Update.
func (s *Stack) RefreshConfig(ctx context.Context) (ConfigMap, error) {
	return s.Workspace().RefreshConfig(ctx, s.Name())
}

// GetTag returns the tag value associated with specified key.
func (s *Stack) GetTag(ctx context.Context, key string) (string, error) {
	return s.Workspace().GetTag(ctx, s.Name(), key)
}

// SetTag sets a tag key-value pair on the stack.
func (s *Stack) SetTag(ctx context.Context, key string, value string) error {
	return s.Workspace().SetTag(ctx, s.Name(), key, value)
}

// RemoveTag removes the specified tag key-value pair from the stack.
func (s *Stack) RemoveTag(ctx context.Context, key string) error {
	return s.Workspace().RemoveTag(ctx, s.Name(), key)
}

// ListTags returns the full key-value tag map associated with the stack.
func (s *Stack) ListTags(ctx context.Context) (map[string]string, error) {
	return s.Workspace().ListTags(ctx, s.Name())
}

// Info returns a summary of the Stack including its URL.
func (s *Stack) Info(ctx context.Context) (StackSummary, error) {
	var info StackSummary
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return info, fmt.Errorf("failed to fetch stack info: %w", err)
	}

	summary, err := s.Workspace().Stack(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch stack info: %w", err)
	}

	if summary != nil {
		info = *summary
	}

	return info, nil
}

// Cancel stops a stack's currently running update. It returns an error if no update is currently running.
// Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
// if a resource operation was pending when the update was canceled.
func (s *Stack) Cancel(ctx context.Context) error {
	stdout, stderr, errCode, err := s.runPulumiCmdSync(
		ctx,
		nil, /* additionalOutput */
		nil, /* additionalErrorOutput */
		"cancel", "--yes")
	if err != nil {
		return newAutoError(fmt.Errorf("failed to cancel update: %w", err), stdout, stderr, errCode)
	}

	return nil
}

// Export exports the deployment state of the stack.
// This can be combined with Stack.Import to edit a stack's state (such as recovery from failed deployments).
func (s *Stack) Export(ctx context.Context) (apitype.UntypedDeployment, error) {
	return s.Workspace().ExportStack(ctx, s.Name())
}

// Import imports the specified deployment state into the stack.
// This can be combined with Stack.Export to edit a stack's state (such as recovery from failed deployments).
func (s *Stack) Import(ctx context.Context, state apitype.UntypedDeployment) error {
	return s.Workspace().ImportStack(ctx, s.Name(), state)
}

// UpdateSummary provides a summary of a Stack lifecycle operation (up/preview/refresh/destroy).
type UpdateSummary struct {
	Version     int               `json:"version"`
	Kind        string            `json:"kind"`
	StartTime   string            `json:"startTime"`
	Message     string            `json:"message"`
	Environment map[string]string `json:"environment"`
	Config      ConfigMap         `json:"config"`
	Result      string            `json:"result,omitempty"`

	// These values are only present once the update finishes
	EndTime         *string         `json:"endTime,omitempty"`
	ResourceChanges *map[string]int `json:"resourceChanges,omitempty"`
}

// OutputValue models a Pulumi Stack output, providing the plaintext value and a boolean indicating secretness.
type OutputValue struct {
	Value  interface{}
	Secret bool
}

// UpResult contains information about a Stack.Up operation,
// including Outputs, and a summary of the deployed changes.
type UpResult struct {
	StdOut  string
	StdErr  string
	Outputs OutputMap
	Summary UpdateSummary
}

// ImportResult contains information about a Stack.Import operation,
type ImportResult struct {
	StdOut        string
	StdErr        string
	GeneratedCode string
	Summary       UpdateSummary
}

// GetPermalink returns the permalink URL in the Pulumi Console for the update operation.
func (ur *UpResult) GetPermalink() (string, error) {
	return GetPermalink(ur.StdOut)
}

// ErrParsePermalinkFailed occurs when the the generated permalink URL can't be found in the op result
var ErrParsePermalinkFailed = errors.New("failed to get permalink")

// GetPermalink returns the permalink URL in the Pulumi Console for the update
// or refresh operation. This will error for alternate, diy backends.
func GetPermalink(stdout string) (string, error) {
	const permalinkSearchStr = `View Live: |View in Browser: |View in Browser \(Ctrl\+O\): |Permalink: `
	startRegex := regexp.MustCompile(permalinkSearchStr)
	endRegex := regexp.MustCompile("\n")

	// Find the start of the permalink in the output.
	start := startRegex.FindStringIndex(stdout)
	if start == nil {
		return "", ErrParsePermalinkFailed
	}
	permalinkStart := stdout[start[1]:]

	// Find the end of the permalink.
	end := endRegex.FindStringIndex(permalinkStart)
	if end == nil {
		return "", ErrParsePermalinkFailed
	}
	permalink := permalinkStart[:end[1]-1]
	return permalink, nil
}

// OutputMap is the output result of running a Pulumi program
type OutputMap map[string]OutputValue

// PreviewStep is a summary of the expected state transition of a given resource based on running the current program.
type PreviewStep struct {
	// Op is the kind of operation being performed.
	Op string `json:"op"`
	// URN is the resource being affected by this operation.
	URN resource.URN `json:"urn"`
	// Provider is the provider that will perform this step.
	Provider string `json:"provider,omitempty"`
	// OldState is the old state for this resource, if appropriate given the operation type.
	OldState *apitype.ResourceV3 `json:"oldState,omitempty"`
	// NewState is the new state for this resource, if appropriate given the operation type.
	NewState *apitype.ResourceV3 `json:"newState,omitempty"`
	// DiffReasons is a list of keys that are causing a diff (for updating steps only).
	DiffReasons []resource.PropertyKey `json:"diffReasons,omitempty"`
	// ReplaceReasons is a list of keys that are causing replacement (for replacement steps only).
	ReplaceReasons []resource.PropertyKey `json:"replaceReasons,omitempty"`
	// DetailedDiff is a structured diff that indicates precise per-property differences.
	DetailedDiff map[string]PropertyDiff `json:"detailedDiff"`
}

// PropertyDiff contains information about the difference in a single property value.
type PropertyDiff struct {
	// Kind is the kind of difference.
	Kind string `json:"kind"`
	// InputDiff is true if this is a difference between old and new inputs instead of old state and new inputs.
	InputDiff bool `json:"inputDiff"`
}

// PreviewResult is the output of Stack.Preview() describing the expected set of changes from the next Stack.Up()
type PreviewResult struct {
	StdOut        string
	StdErr        string
	ChangeSummary map[apitype.OpType]int
}

// GetPermalink returns the permalink URL in the Pulumi Console for the preview operation.
func (pr *PreviewResult) GetPermalink() (string, error) {
	return GetPermalink(pr.StdOut)
}

// RefreshResult is the output of a successful Stack.Refresh operation
type RefreshResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

// GetPermalink returns the permalink URL in the Pulumi Console for the refresh operation.
func (rr *RefreshResult) GetPermalink() (string, error) {
	return GetPermalink(rr.StdOut)
}

// DestroyResult is the output of a successful Stack.Destroy operation
type DestroyResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

// GetPermalink returns the permalink URL in the Pulumi Console for the destroy operation.
func (dr *DestroyResult) GetPermalink() (string, error) {
	return GetPermalink(dr.StdOut)
}

// secretSentinel represents the CLI response for an output marked as "secret"
const secretSentinel = "[secret]"

func (s *Stack) runPulumiCmdSync(
	ctx context.Context,
	additionalOutput []io.Writer,
	additionalErrorOutput []io.Writer,
	args ...string,
) (string, string, int, error) {
	var env []string
	debugEnv := fmt.Sprintf("%s=%s", "PULUMI_DEBUG_COMMANDS", "true")
	env = append(env, debugEnv)

	var remote bool
	if lws, isLocalWorkspace := s.Workspace().(*LocalWorkspace); isLocalWorkspace {
		remote = lws.remote
	}
	if remote {
		experimentalEnv := fmt.Sprintf("%s=%s", "PULUMI_EXPERIMENTAL", "true")
		env = append(env, experimentalEnv)
	}

	if s.Workspace().PulumiHome() != "" {
		homeEnv := fmt.Sprintf("%s=%s", pulumiHomeEnv, s.Workspace().PulumiHome())
		env = append(env, homeEnv)
	}
	if envvars := s.Workspace().GetEnvVars(); envvars != nil {
		for k, v := range envvars {
			e := []string{k, v}
			env = append(env, strings.Join(e, "="))
		}
	}
	additionalArgs, err := s.Workspace().SerializeArgsForOp(ctx, s.Name())
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to exec command, error getting additional args: %w", err)
	}
	args = append(args, additionalArgs...)
	args = append(args, "--stack", s.Name())

	stdout, stderr, errCode, err := s.workspace.PulumiCommand().Run(
		ctx,
		s.Workspace().WorkDir(),
		nil,
		additionalOutput,
		additionalErrorOutput,
		env,
		args...,
	)
	if err != nil {
		return stdout, stderr, errCode, err
	}
	err = s.Workspace().PostCommandCallback(ctx, s.Name())
	if err != nil {
		return stdout, stderr, errCode, fmt.Errorf("command ran successfully, but error running PostCommandCallback: %w", err)
	}
	return stdout, stderr, errCode, nil
}

func (s *Stack) isRemote() bool {
	var remote bool
	if lws, isLocalWorkspace := s.Workspace().(*LocalWorkspace); isLocalWorkspace {
		remote = lws.remote
	}
	return remote
}

func (s *Stack) remoteArgs() []string {
	var remote bool
	var repo *GitRepo
	var preRunCommands []string
	var envvars map[string]EnvVarValue
	var executorImage *ExecutorImage
	var remoteAgentPoolID string
	var skipInstallDependencies bool
	var inheritSettings bool
	if lws, isLocalWorkspace := s.Workspace().(*LocalWorkspace); isLocalWorkspace {
		remote = lws.remote
		repo = lws.repo
		preRunCommands = lws.preRunCommands
		envvars = lws.remoteEnvVars
		skipInstallDependencies = lws.remoteSkipInstallDependencies
		executorImage = lws.remoteExecutorImage
		remoteAgentPoolID = lws.remoteAgentPoolID
		inheritSettings = lws.remoteInheritSettings
	}
	if !remote {
		return nil
	}

	args := slice.Prealloc[string](len(envvars) + len(preRunCommands))
	args = append(args, "--remote")
	if repo != nil {
		if repo.URL != "" {
			args = append(args, repo.URL)
		}
		if repo.Branch != "" {
			args = append(args, "--remote-git-branch="+repo.Branch)
		}
		if repo.CommitHash != "" {
			args = append(args, "--remote-git-commit="+repo.CommitHash)
		}
		if repo.ProjectPath != "" {
			args = append(args, "--remote-git-repo-dir="+repo.ProjectPath)
		}
		if repo.Auth != nil {
			if repo.Auth.PersonalAccessToken != "" {
				args = append(args, "--remote-git-auth-access-token="+repo.Auth.PersonalAccessToken)
			}
			if repo.Auth.SSHPrivateKey != "" {
				args = append(args, "--remote-git-auth-ssh-private-key="+repo.Auth.SSHPrivateKey)
			}
			if repo.Auth.SSHPrivateKeyPath != "" {
				args = append(args,
					"--remote-git-auth-ssh-private-key-path="+repo.Auth.SSHPrivateKeyPath)
			}
			if repo.Auth.Password != "" {
				args = append(args, "--remote-git-auth-password="+repo.Auth.Password)
			}
			if repo.Auth.Username != "" {
				args = append(args, "--remote-git-auth-username="+repo.Auth.Username)
			}
		}
	}

	for k, v := range envvars {
		flag := "--remote-env"
		if v.Secret {
			flag += "-secret"
		}
		args = append(args, fmt.Sprintf("%s=%s=%s", flag, k, v.Value))
	}

	for _, command := range preRunCommands {
		args = append(args, "--remote-pre-run-command="+command)
	}

	if executorImage != nil {
		args = append(args, "--remote-executor-image="+executorImage.Image)
		if executorImage.Credentials != nil {
			if executorImage.Credentials.Username != "" {
				args = append(args, "--remote-executor-image-username="+executorImage.Credentials.Username)
			}
			if executorImage.Credentials.Password != "" {
				args = append(args, "--remote-executor-image-password="+executorImage.Credentials.Password)
			}
		}
	}

	if remoteAgentPoolID != "" {
		args = append(args, "--remote-agent-pool-id="+remoteAgentPoolID)
	}

	if skipInstallDependencies {
		args = append(args, "--remote-skip-install-dependencies")
	}

	if inheritSettings {
		args = append(args, "--remote-inherit-settings")
	}

	return args
}

const (
	stateWaiting = iota
	stateRunning
	stateCanceled
	stateFinished
)

type languageRuntimeServer struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	m sync.Mutex
	c *sync.Cond

	fn      pulumi.RunFunc
	address string

	state  int
	cancel chan bool
	done   <-chan error
}

// isNestedInvocation returns true if pulumi.RunWithContext is on the stack.
func isNestedInvocation() bool {
	depth, callers := 0, make([]uintptr, 32)
	for {
		n := runtime.Callers(depth, callers)
		if n == 0 {
			return false
		}
		depth += n

		frames := runtime.CallersFrames(callers)
		for f, more := frames.Next(); more; f, more = frames.Next() {
			if f.Function == "github.com/pulumi/pulumi/sdk/v3/go/pulumi.RunWithContext" {
				return true
			}
		}
	}
}

func startLanguageRuntimeServer(fn pulumi.RunFunc) (*languageRuntimeServer, error) {
	if isNestedInvocation() {
		return nil, errors.New("nested stack operations are not supported https://github.com/pulumi/pulumi/issues/5058")
	}

	s := &languageRuntimeServer{
		fn:     fn,
		cancel: make(chan bool),
	}
	s.c = sync.NewCond(&s.m)

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: s.cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, s)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, err
	}
	s.address, s.done = fmt.Sprintf("127.0.0.1:%d", handle.Port), handle.Done
	return s, nil
}

func (s *languageRuntimeServer) Close() error {
	s.m.Lock()
	switch s.state {
	case stateCanceled:
		s.m.Unlock()
		return nil
	case stateWaiting:
		// Not started yet; go ahead and cancel
	default:
		for s.state != stateFinished {
			s.c.Wait()
		}
	}
	s.state = stateCanceled
	s.m.Unlock()

	s.cancel <- true
	close(s.cancel)
	return <-s.done
}

func (s *languageRuntimeServer) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (s *languageRuntimeServer) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	s.m.Lock()
	if s.state == stateCanceled {
		s.m.Unlock()
		return nil, errors.New("program canceled")
	}
	s.state = stateRunning
	s.m.Unlock()

	defer func() {
		s.m.Lock()
		s.state = stateFinished
		s.m.Unlock()
		s.c.Broadcast()
	}()

	var engineAddress string
	if len(req.Args) > 0 {
		engineAddress = req.Args[0]
	}
	runInfo := pulumi.RunInfo{
		EngineAddr:       engineAddress,
		MonitorAddr:      req.GetMonitorAddress(),
		Config:           req.GetConfig(),
		ConfigSecretKeys: req.GetConfigSecretKeys(),
		Project:          req.GetProject(),
		Stack:            req.GetStack(),
		Parallel:         req.GetParallel(),
		DryRun:           req.GetDryRun(),
		Organization:     req.GetOrganization(),
	}

	pulumiCtx, err := pulumi.NewContext(ctx, runInfo)
	if err != nil {
		return nil, err
	}
	defer pulumiCtx.Close()

	err = func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if pErr, ok := r.(error); ok {
					err = fmt.Errorf("go inline source runtime error, an unhandled error occurred: %w", pErr)
				} else {
					err = fmt.Errorf("go inline source runtime error, an unhandled panic occurred: %v", r)
				}
			}
		}()

		return pulumi.RunWithContext(pulumiCtx, s.fn)
	}()
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}
	return &pulumirpc.RunResponse{}, nil
}

func (s *languageRuntimeServer) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func (s *languageRuntimeServer) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

type fileWatcher struct {
	Filename  string
	tail      *tail.Tail
	receivers []chan<- events.EngineEvent
	done      chan bool
}

func watchFile(path string, receivers []chan<- events.EngineEvent) (*fileWatcher, error) {
	t, err := tail.TailFile(path, tail.Config{
		Follow:        true,
		Poll:          runtime.GOOS == "windows", // on Windows poll for file changes instead of using the default inotify
		Logger:        tail.DiscardingLogger,
		CompleteLines: true,
	})
	if err != nil {
		return nil, err
	}
	done := make(chan bool)
	go func(tailedLog *tail.Tail) {
		for line := range tailedLog.Lines {
			if line.Err != nil {
				for _, r := range receivers {
					r <- events.EngineEvent{Error: line.Err}
				}
				continue
			}
			var e apitype.EngineEvent
			err = json.Unmarshal([]byte(line.Text), &e)
			if err != nil {
				for _, r := range receivers {
					r <- events.EngineEvent{Error: err}
				}
				continue
			}
			for _, r := range receivers {
				r <- events.EngineEvent{EngineEvent: e}
			}
		}
		for _, r := range receivers {
			close(r)
		}
		close(done)
	}(t)
	return &fileWatcher{
		Filename:  t.Filename,
		tail:      t,
		receivers: receivers,
		done:      done,
	}, nil
}

func tailLogs(command string, receivers []chan<- events.EngineEvent) (*fileWatcher, error) {
	logDir, err := os.MkdirTemp("", fmt.Sprintf("automation-logs-%s-", command))
	if err != nil {
		return nil, fmt.Errorf("failed to create logdir: %w", err)
	}
	logFile := filepath.Join(logDir, "eventlog.txt")

	t, err := watchFile(logFile, receivers)
	if err != nil {
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	return t, nil
}

func (fw *fileWatcher) Close() {
	if fw.tail == nil {
		return
	}

	// Tell the watcher to end on next EoF, wait for the done event, then cleanup.

	// The tail library we're using is racy when shutting down.
	// If it gets the shutdown signal before reading the data, it
	// will just shut down before finding the EoF.  This problem
	// is exacerbated on Windows, where we use the poller, which
	// polls for changes every 250ms.  Sleep a little bit longer
	// than that to ensure the tail library had a chance to read
	// the whole file.  On OSs that don't use the poller we still
	// want to try to avoid the problem so we sleep for a short
	// amount of time.
	//
	// TODO: remove this once https://github.com/nxadm/tail/issues/67
	// is fixed and we can upgrade nxadm/tail.
	if runtime.GOOS == "windows" {
		time.Sleep(300 * time.Millisecond)
	} else {
		time.Sleep(150 * time.Millisecond)
	}

	//nolint:errcheck
	fw.tail.StopAtEOF()
	<-fw.done
	logDir := filepath.Dir(fw.tail.Filename)
	fw.tail.Cleanup()
	os.RemoveAll(logDir)

	// set to nil so we can safely close again in defer
	fw.tail = nil
}
