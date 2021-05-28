// Copyright 2016-2020, Pulumi Corporation.
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
// 1. Programs locally available on-disk and addressed via a filepath (NewStackLocalSource)
//	stack, err := NewStackLocalSource(ctx, "myOrg/myProj/myStack", filepath.Join("..", "path", "to", "project"))
//
// 2. Programs fetched from a Git URL (NewStackRemoteSource)
//	stack, err := NewStackRemoteSource(ctx, "myOrg/myProj/myStack", GitRepo{
//		URL:         "https://github.com/pulumi/test-repo.git",
//		ProjectPath: filepath.Join("project", "path", "repo", "root", "relative"),
//	})
// 3. Programs defined as a function alongside your Automation API code (NewStackInlineSource)
//	 stack, err := NewStackInlineSource(ctx, "myOrg/myProj/myStack", func(pCtx *pulumi.Context) error {
//		bucket, err := s3.NewBucket(pCtx, "bucket", nil)
//		if err != nil {
//			return err
//		}
//		pCtx.Export("bucketName", bucket.Bucket)
//		return nil
//	 })
// Each of these creates a stack with access to the full range of Pulumi lifecycle methods
// (up/preview/refresh/destroy), as well as methods for managing config, stack, and project settings.
//	 err := stack.SetConfig(ctx, "key", ConfigValue{ Value: "value", Secret: true })
//	 preRes, err := stack.Preview(ctx)
//	 // detailed info about results
//	 fmt.Println(preRes.prev.Steps[0].URN)
// The Automation API provides a natural way to orchestrate multiple stacks,
// feeding the output of one stack as an input to the next as shown in the package-level example below.
// The package can be used for a number of use cases:
//
// 	- Driving pulumi deployments within CI/CD workflows
//
// 	- Integration testing
//
// 	- Multi-stage deployments such as blue-green deployment patterns
//
// 	- Deployments involving application code like database migrations
//
// 	- Building higher level tools, custom CLIs over pulumi, etc
//
//	- Using pulumi behind a REST or GRPC API
//
//  - Debugging Pulumi programs (by using a single main entrypoint with "inline" programs)
//
// To enable a broad range of runtime customization the API defines a `Workspace` interface.
// A Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
// Workspaces are used to manage the execution environment, providing various utilities such as plugin
// installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
// Every Stack including those in the above examples are backed by a Workspace which can be accessed via:
//	 w = stack.Workspace()
//	 err := w.InstallPlugin("aws", "v3.2.0")
// Workspaces can be explicitly created and customized beyond the three Stack creation helpers noted above:
//	 w, err := NewLocalWorkspace(ctx, WorkDir(filepath.Join(".", "project", "path"), PulumiHome("~/.pulumi"))
//	 s := NewStack(ctx, "org/proj/stack", w)
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
// 	uRes, err :=stack.Up(ctx)
// 	if err != nil && IsConcurrentUpdateError(err) { /* retry logic here */ }
package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/nxadm/tail"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
// Note that filestate backends (local file, S3, Azure Blob) do not support stack names in this
// format, and instead only use the stack name without an org/user or project to qualify it.
// See: https://github.com/pulumi/pulumi/issues/2522
func FullyQualifiedStackName(org, project, stack string) string {
	return fmt.Sprintf("%s/%s/%s", org, project, stack)
}

// NewStack creates a new stack using the given workspace, and stack name.
// It fails if a stack with that name already exists
func NewStack(ctx context.Context, stackName string, ws Workspace) (Stack, error) {
	var s Stack
	s = Stack{
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
	var s Stack
	s = Stack{
		workspace: ws,
		stackName: stackName,
	}

	err := ws.SelectStack(ctx, stackName)
	if err != nil {
		return s, err
	}

	return s, nil
}

// UpsertStack tries to create a new stack using the given workspace and
// stack name if the stack does not already exist,
// or falls back to selecting the existing stack. If the stack does not exist,
// it will be created and selected.
func UpsertStack(ctx context.Context, stackName string, ws Workspace) (Stack, error) {
	s, err := NewStack(ctx, stackName, ws)
	// error for all failures except if the stack already exists, as we'll
	// just select the stack if it exists.
	if err != nil && !IsCreateStack409Error(err) {
		return s, err
	}

	err = ws.SelectStack(ctx, stackName)
	if err != nil {
		return s, err
	}

	return s, nil
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

// Preview preforms a dry-run update to a stack, returning pending changes.
// https://www.pulumi.com/docs/reference/cli/pulumi_preview/
func (s *Stack) Preview(ctx context.Context, opts ...optpreview.Option) (PreviewResult, error) {
	var res PreviewResult

	preOpts := &optpreview.Options{}
	for _, o := range opts {
		o.ApplyOption(preOpts)
	}

	var sharedArgs []string

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
		sharedArgs = append(sharedArgs, "--replace %s", rURN)
	}
	for _, tURN := range preOpts.Target {
		sharedArgs = append(sharedArgs, "--target %s", tURN)
	}
	if preOpts.TargetDependents {
		sharedArgs = append(sharedArgs, "--target-dependents")
	}
	if preOpts.Parallel > 0 {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--parallel=%d", preOpts.Parallel))
	}
	if preOpts.UserAgent != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--exec-agent=%s", preOpts.UserAgent))
	}

	kind, args := constant.ExecKindAutoLocal, []string{"preview"}
	if program := s.Workspace().Program(); program != nil {
		server, err := startLanguageRuntimeServer(program)
		if err != nil {
			return res, err
		}
		defer contract.IgnoreClose(server)

		kind, args = constant.ExecKindAutoInline, append(args, "--client="+server.address)
	}

	args = append(args, fmt.Sprintf("--exec-kind=%s", kind))
	args = append(args, sharedArgs...)

	var summaryEvents []apitype.SummaryEvent
	eventChannel := make(chan events.EngineEvent)
	go func(ch chan events.EngineEvent, events *[]apitype.SummaryEvent) {
		for {
			event, ok := <-eventChannel
			if !ok {
				return
			}
			if event.SummaryEvent != nil {
				summaryEvents = append(summaryEvents, *event.SummaryEvent)
			}
		}
	}(eventChannel, &summaryEvents)

	eventChannels := []chan<- events.EngineEvent{eventChannel}
	eventChannels = append(eventChannels, preOpts.EventStreams...)

	t, err := tailLogs("preview", eventChannels)
	if err != nil {
		return res, errors.Wrap(err, "failed to tail logs")
	}
	defer cleanup(t, eventChannels)
	args = append(args, "--event-log", t.Filename)

	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, preOpts.ProgressStreams /* additionalOutput */, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to run preview"), stdout, stderr, code)
	}

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
// https://www.pulumi.com/docs/reference/cli/pulumi_up/
func (s *Stack) Up(ctx context.Context, opts ...optup.Option) (UpResult, error) {
	var res UpResult

	upOpts := &optup.Options{}
	for _, o := range opts {
		o.ApplyOption(upOpts)
	}

	var sharedArgs []string

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
		sharedArgs = append(sharedArgs, "--replace %s", rURN)
	}
	for _, tURN := range upOpts.Target {
		sharedArgs = append(sharedArgs, "--target %s", tURN)
	}
	if upOpts.TargetDependents {
		sharedArgs = append(sharedArgs, "--target-dependents")
	}
	if upOpts.Parallel > 0 {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--parallel=%d", upOpts.Parallel))
	}
	if upOpts.UserAgent != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--exec-agent=%s", upOpts.UserAgent))
	}

	kind, args := constant.ExecKindAutoLocal, []string{"up", "--yes", "--skip-preview"}
	if program := s.Workspace().Program(); program != nil {
		server, err := startLanguageRuntimeServer(program)
		if err != nil {
			return res, err
		}
		defer contract.IgnoreClose(server)

		kind, args = constant.ExecKindAutoInline, append(args, "--client="+server.address)
	}
	args = append(args, fmt.Sprintf("--exec-kind=%s", kind))

	if len(upOpts.EventStreams) > 0 {
		eventChannels := upOpts.EventStreams
		t, err := tailLogs("up", eventChannels)
		if err != nil {
			return res, errors.Wrap(err, "failed to tail logs")
		}
		defer cleanup(t, eventChannels)
		args = append(args, "--event-log", t.Filename)
	}

	args = append(args, sharedArgs...)
	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, upOpts.ProgressStreams, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to run update"), stdout, stderr, code)
	}

	outs, err := s.Outputs(ctx)
	if err != nil {
		return res, err
	}

	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/)
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

// Refresh compares the current stack’s resource state with the state known to exist in the actual
// cloud provider. Any such changes are adopted into the current stack.
func (s *Stack) Refresh(ctx context.Context, opts ...optrefresh.Option) (RefreshResult, error) {
	var res RefreshResult

	refreshOpts := &optrefresh.Options{}
	for _, o := range opts {
		o.ApplyOption(refreshOpts)
	}

	var args []string

	args = debug.AddArgs(&refreshOpts.DebugLogOpts, args)
	args = append(args, "refresh", "--yes", "--skip-preview")
	if refreshOpts.Message != "" {
		args = append(args, fmt.Sprintf("--message=%q", refreshOpts.Message))
	}
	if refreshOpts.ExpectNoChanges {
		args = append(args, "--expect-no-changes")
	}
	for _, tURN := range refreshOpts.Target {
		args = append(args, "--target %s", tURN)
	}
	if refreshOpts.Parallel > 0 {
		args = append(args, fmt.Sprintf("--parallel=%d", refreshOpts.Parallel))
	}
	if refreshOpts.UserAgent != "" {
		args = append(args, fmt.Sprintf("--exec-agent=%s", refreshOpts.UserAgent))
	}
	execKind := constant.ExecKindAutoLocal
	if s.Workspace().Program() != nil {
		execKind = constant.ExecKindAutoInline
	}
	args = append(args, fmt.Sprintf("--exec-kind=%s", execKind))

	if len(refreshOpts.EventStreams) > 0 {
		eventChannels := refreshOpts.EventStreams
		t, err := tailLogs("refresh", eventChannels)
		if err != nil {
			return res, errors.Wrap(err, "failed to tail logs")
		}
		defer cleanup(t, eventChannels)
		args = append(args, "--event-log", t.Filename)
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, refreshOpts.ProgressStreams, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to refresh stack"), stdout, stderr, code)
	}

	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/)
	if err != nil {
		return res, errors.Wrap(err, "failed to refresh stack")
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

// Destroy deletes all resources in a stack, leaving all history and configuration intact.
func (s *Stack) Destroy(ctx context.Context, opts ...optdestroy.Option) (DestroyResult, error) {
	var res DestroyResult

	destroyOpts := &optdestroy.Options{}
	for _, o := range opts {
		o.ApplyOption(destroyOpts)
	}

	var args []string

	args = debug.AddArgs(&destroyOpts.DebugLogOpts, args)
	args = append(args, "destroy", "--yes", "--skip-preview")
	if destroyOpts.Message != "" {
		args = append(args, fmt.Sprintf("--message=%q", destroyOpts.Message))
	}
	for _, tURN := range destroyOpts.Target {
		args = append(args, "--target %s", tURN)
	}
	if destroyOpts.TargetDependents {
		args = append(args, "--target-dependents")
	}
	if destroyOpts.Parallel > 0 {
		args = append(args, fmt.Sprintf("--parallel=%d", destroyOpts.Parallel))
	}
	if destroyOpts.UserAgent != "" {
		args = append(args, fmt.Sprintf("--exec-agent=%s", destroyOpts.UserAgent))
	}
	execKind := constant.ExecKindAutoLocal
	if s.Workspace().Program() != nil {
		execKind = constant.ExecKindAutoInline
	}
	args = append(args, fmt.Sprintf("--exec-kind=%s", execKind))

	if len(destroyOpts.EventStreams) > 0 {
		eventChannels := destroyOpts.EventStreams
		t, err := tailLogs("destroy", eventChannels)
		if err != nil {
			return res, errors.Wrap(err, "failed to tail logs")
		}
		defer cleanup(t, eventChannels)
		args = append(args, "--event-log", t.Filename)
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, destroyOpts.ProgressStreams, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to destroy stack"), stdout, stderr, code)
	}

	history, err := s.History(ctx, 1 /*pageSize*/, 1 /*page*/)
	if err != nil {
		return res, errors.Wrap(err, "failed to destroy stack")
	}

	var summary UpdateSummary
	if len(history) > 0 {
		summary = history[0]
	}

	res = DestroyResult{
		Summary: summary,
		StdOut:  stdout,
		StdErr:  stderr,
	}

	return res, nil
}

// Outputs get the current set of Stack outputs from the last Stack.Up().
func (s *Stack) Outputs(ctx context.Context) (OutputMap, error) {
	return s.Workspace().StackOutputs(ctx, s.Name())
}

// History returns a list summarizing all previous and current results from Stack lifecycle operations
// (up/preview/refresh/destroy).
func (s *Stack) History(ctx context.Context, pageSize int, page int) ([]UpdateSummary, error) {
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stack history")
	}
	args := []string{"stack", "history", "--json", "--show-secrets"}
	if pageSize > 0 {
		// default page=1 if unset when pageSize is set
		if page < 1 {
			page = 1
		}
		args = append(args, "--page-size", fmt.Sprintf("%d", pageSize), "--page", fmt.Sprintf("%d", page))
	}

	stdout, stderr, errCode, err := s.runPulumiCmdSync(ctx, nil /* additionalOutputs */, args...)
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "failed to get stack history"), stdout, stderr, errCode)
	}

	var history []UpdateSummary
	err = json.Unmarshal([]byte(stdout), &history)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal history result")
	}

	return history, nil
}

// GetConfig returns the config value associated with the specified key.
func (s *Stack) GetConfig(ctx context.Context, key string) (ConfigValue, error) {
	return s.Workspace().GetConfig(ctx, s.Name(), key)
}

// GetAllConfig returns the full config map.
func (s *Stack) GetAllConfig(ctx context.Context) (ConfigMap, error) {
	return s.Workspace().GetAllConfig(ctx, s.Name())
}

// SetConfig sets the specified config key-value pair.
func (s *Stack) SetConfig(ctx context.Context, key string, val ConfigValue) error {
	return s.Workspace().SetConfig(ctx, s.Name(), key, val)
}

// SetAllConfig sets all values in the provided config map.
func (s *Stack) SetAllConfig(ctx context.Context, config ConfigMap) error {
	return s.Workspace().SetAllConfig(ctx, s.Name(), config)
}

// RemoveConfig removes the specified config key-value pair.
func (s *Stack) RemoveConfig(ctx context.Context, key string) error {
	return s.Workspace().RemoveConfig(ctx, s.Name(), key)
}

// RemoveAllConfig removes all values in the provided list of keys.
func (s *Stack) RemoveAllConfig(ctx context.Context, keys []string) error {
	return s.Workspace().RemoveAllConfig(ctx, s.Name(), keys)
}

// RefreshConfig gets and sets the config map used with the last Update.
func (s *Stack) RefreshConfig(ctx context.Context) (ConfigMap, error) {
	return s.Workspace().RefreshConfig(ctx, s.Name())
}

// Info returns a summary of the Stack including its URL.
func (s *Stack) Info(ctx context.Context) (StackSummary, error) {
	var info StackSummary
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return info, errors.Wrap(err, "failed to fetch stack info")
	}

	summary, err := s.Workspace().Stack(ctx)
	if err != nil {
		return info, errors.Wrap(err, "failed to fetch stack info")
	}

	if summary != nil {
		info = *summary
	}

	return info, nil
}

// Cancel stops a stack's currently running update. It returns an error if no update is currently running.
// Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
// if a resource operation was pending when the update was canceled.
// This command is not supported for local backends.
func (s *Stack) Cancel(ctx context.Context) error {
	stdout, stderr, errCode, err := s.runPulumiCmdSync(
		ctx,
		nil, /* additionalOutput */
		"cancel", "--yes")
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to cancel update"), stdout, stderr, errCode)
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

// GetPermalink returns the permalink URL in the Pulumi Console for the update operation.
func (ur *UpResult) GetPermalink() (string, error) {
	return GetPermalink(ur.StdOut)
}

// ErrParsePermalinkFailed occurs when the the generated permalink URL can't be found in the op result
var ErrParsePermalinkFailed = errors.New("failed to get permalink")

// GetPermalink returns the permalink URL in the Pulumi Console for the update
// or refresh operation. This will error for alternate, local backends.
func GetPermalink(stdout string) (string, error) {
	const permalinkSearchStr = "View Live: |Permalink: "
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
	args ...string,
) (string, string, int, error) {
	var env []string
	debugEnv := fmt.Sprintf("%s=%s", "PULUMI_DEBUG_COMMANDS", "true")
	env = append(env, debugEnv)
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
		return "", "", -1, errors.Wrap(err, "failed to exec command, error getting additional args")
	}
	args = append(args, additionalArgs...)
	args = append(args, "--stack", s.Name())

	stdout, stderr, errCode, err := runPulumiCommandSync(ctx, s.Workspace().WorkDir(), additionalOutput, env, args...)
	if err != nil {
		return stdout, stderr, errCode, err
	}
	err = s.Workspace().PostCommandCallback(ctx, s.Name())
	if err != nil {
		return stdout, stderr, errCode, errors.Wrap(err, "command ran successfully, but error running PostCommandCallback")
	}
	return stdout, stderr, errCode, nil
}

const (
	stateWaiting = iota
	stateRunning
	stateCanceled
	stateFinished
)

type languageRuntimeServer struct {
	m sync.Mutex
	c *sync.Cond

	fn      pulumi.RunFunc
	address string

	state  int
	cancel chan bool
	done   chan error
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

	port, done, err := rpcutil.Serve(0, s.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, s)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	s.address, s.done = fmt.Sprintf("127.0.0.1:%d", port), done
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
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (s *languageRuntimeServer) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	s.m.Lock()
	if s.state == stateCanceled {
		s.m.Unlock()
		return nil, errors.Errorf("program canceled")
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
		Parallel:         int(req.GetParallel()),
		DryRun:           req.GetDryRun(),
	}

	pulumiCtx, err := pulumi.NewContext(ctx, runInfo)
	if err != nil {
		return nil, err
	}

	err = func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if pErr, ok := r.(error); ok {
					err = errors.Wrap(pErr, "go inline source runtime error, an unhandled error occurred:")
				} else {
					err = errors.New("go inline source runtime error, an unhandled error occurred: unknown error")
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

func (s *languageRuntimeServer) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func tailLogs(command string, receivers []chan<- events.EngineEvent) (*tail.Tail, error) {
	logDir, err := ioutil.TempDir("", fmt.Sprintf("automation-logs-%s-", command))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create logdir")
	}
	logFile := filepath.Join(logDir, "eventlog.txt")

	t, err := watchFile(logFile, receivers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to watch file")
	}

	return t, nil
}

func cleanup(t *tail.Tail, channels []chan<- events.EngineEvent) {
	logDir := filepath.Dir(t.Filename)
	t.Cleanup()
	os.RemoveAll(logDir)
	for _, ch := range channels {
		close(ch)
	}
}
