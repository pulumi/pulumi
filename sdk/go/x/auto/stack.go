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
// Generally this can be thought of as encapsulating the functionality of the CLI but with more flexibility
// (`pulumi up`, `pulumi preview`, pulumi destroy`, `pulumi stack init`, etc.). This still requires a
// CLI binary to be installed and available on your $PATH. The Automation API is in Alpha (experimental package/x)
// breaking changes (mostly additive) will be made. You can pin to a specific commit version if you need stability.
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
// 3. Programs defined as a `func` alongside your Automation API code (NewStackInlineSource)
//	 stack, err := NewStackInlineSource(ctx, "myOrg/myProj/myStack", func(pCtx *pulumi.Context) error {
//		bucket, err := s3.NewBucket(pCtx, "bucket", nil)
//		if err != nil {
//			return err
//		}
//		pCtx.Export("bucketName", bucket.Bucket)
//		return nil
//	 })
// Each of these creates a stack with access to the full range of Pulumi lifecycle methods
// (up/preview/refresh/destroy), as well as methods for managing config, stack, and project settings:
//	 err := stack.SetConfig(ctx, "key", ConfigValue{ Value: "value", Secret: true })
//	 preRes, err := stack.Preview(ctx)
//	 // detailed info about results
//	 fmt.Println(preRes.prev.Steps[0].URN)
// The Automation API provides a natural way to orchestrate multiple stacks,
// feeding the output of one stack as an input to the next as shown in the package level example.
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
// A default implemenatation of workspace is provided as LocalWorkspace. This implementation relies on pulumi.yaml
// and pulumi.<stack>.yaml as the intermediate format for Project and Stack settings. Modifying ProjectSettings will
// alter the Workspace pulumi.yaml file, and setting config on a Stack will modify the pulumi.<stack>.yaml file.
// This is identical to the behavior of Pulumi CLI driven workspaces. Custom Workspace
// implementations can be used to store Project and Stack settings as well as Config in a different format,
// such as an in-memory data structure, a shared persistend SQL database, or cloud object storage. Regardless of
// the backing Workspace implementation, the Pulumi SaaS Console will still be able to display configuration
// applied to updates as it does with the local version of the Workspace today.
//
// The Automation API also provides error handling utilities to detect common cases such as concurrent update
// conflicts:
// 	uRes, err :=stack.Up(ctx)
// 	if err != nil && IsConcurrentUpdateError(err) { /* retry logic here */ }
package auto

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optup"
)

// Stack is an isolated, independently configurable instance of a Pulumi program.
// Stack exposes methods for the full pulumi lifecycle (up/preview/refresh/destroy), as well as managing configuration.
// Automation API stacks are addressed by a fully qualified stack name (fqsn) in the form "org/project/stack".
// Multiple Stacks are commonly used to denote different phases of development
// (such as development, staging and production) or feature branches (such as feature-x-dev, jane-feature-x-dev).
type Stack struct {
	workspace Workspace
	fqsn      string
}

// FullyQualifiedStackName returns an appropriately formatted name to be used for Stack creation/selection.
func FullyQualifiedStackName(org, project, stack string) string {
	return fmt.Sprintf("%s/%s/%s", org, project, stack)
}

// NewStack creates a new stack using the given workspace, and fully qualified stack name (org/project/name).
// It fails if a stack with that name already exists
func NewStack(ctx context.Context, fqsn string, ws Workspace) (Stack, error) {
	var s Stack
	s = Stack{
		workspace: ws,
		fqsn:      fqsn,
	}

	err := ws.CreateStack(ctx, fqsn)
	if err != nil {
		return s, err
	}

	return s, nil
}

// SelectStack selects stack using the given workspace, and fully qualified stack name (org/project/name).
// It returns an error if the given Stack does not exist. All LocalWorkspace operations will call SelectStack()
// before running.
func SelectStack(ctx context.Context, fqsn string, ws Workspace) (Stack, error) {
	var s Stack
	s = Stack{
		workspace: ws,
		fqsn:      fqsn,
	}

	err := ws.SelectStack(ctx, fqsn)
	if err != nil {
		return s, err
	}

	return s, nil
}

// Name returns the fully qualified stack name in the form "org/project/stack"
func (s *Stack) Name() string {
	return s.fqsn
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

	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to run preview")
	}

	preOpts := &optpreview.Options{}
	for _, o := range opts {
		o.ApplyOption(preOpts)
	}

	var sharedArgs []string
	if preOpts.Message != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--message=%q", preOpts.Message))
	}
	if preOpts.ExpectNoChanges {
		sharedArgs = append(sharedArgs, "--expect-no-changes")
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

	var stdout, stderr string
	var code int
	if s.Workspace().Program() != nil {
		hostArgs := []string{"preview"}
		hostArgs = append(hostArgs, sharedArgs...)
		stdout, stderr, err = s.host(ctx, hostArgs, preOpts.Parallel)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run preview"), stdout, stderr, code)
		}
	} else {
		args := []string{"preview", "--json"}
		args = append(args, sharedArgs...)
		stdout, stderr, code, err = s.runPulumiCmdSync(ctx, args...)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run preview"), stdout, stderr, code)
		}
	}

	err = json.Unmarshal([]byte(stdout), &res)
	if err != nil {
		return res, errors.Wrap(err, "unable to unmarshal preview result")
	}

	return res, nil
}

// Up creates or updates the resources in a stack by executing the program in the Workspace.
// https://www.pulumi.com/docs/reference/cli/pulumi_up/
func (s *Stack) Up(ctx context.Context, opts ...optup.Option) (UpResult, error) {
	var res UpResult
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to run update")
	}

	upOpts := &optup.Options{}
	for _, o := range opts {
		o.ApplyOption(upOpts)
	}

	var sharedArgs []string
	if upOpts.Message != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--message=%q", upOpts.Message))
	}
	if upOpts.ExpectNoChanges {
		sharedArgs = append(sharedArgs, "--expect-no-changes")
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

	var stdout, stderr string
	var code int
	if s.Workspace().Program() != nil {
		// TODO need to figure out how to get error code...
		stdout, stderr, err = s.host(ctx, sharedArgs, upOpts.Parallel)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run update"), stdout, stderr, code)
		}
	} else {
		args := []string{"up", "--yes", "--skip-preview"}
		args = append(args, sharedArgs...)
		if upOpts.Parallel > 0 {
			args = append(args, fmt.Sprintf("--parallel=%d", upOpts.Parallel))
		}

		stdout, stderr, code, err = s.runPulumiCmdSync(ctx, args...)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run update"), stdout, stderr, code)
		}
	}

	outs, err := s.Outputs(ctx)
	if err != nil {
		return res, err
	}

	history, err := s.History(ctx)
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

	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to refresh stack")
	}

	refreshOpts := &optrefresh.Options{}
	for _, o := range opts {
		o.ApplyOption(refreshOpts)
	}

	args := []string{"refresh", "--yes", "--skip-preview"}
	if refreshOpts.Message != "" {
		args = append(args, fmt.Sprintf("--message=%q", refreshOpts.Message))
	}
	for _, tURN := range refreshOpts.Target {
		args = append(args, "--target %s", tURN)
	}
	if refreshOpts.Parallel > 0 {
		args = append(args, fmt.Sprintf("--parallel=%d", refreshOpts.Parallel))
	}

	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to refresh stack"), stdout, stderr, code)
	}

	history, err := s.History(ctx)
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

	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to destroy stack")
	}

	destroyOpts := &optdestroy.Options{}
	for _, o := range opts {
		o.ApplyOption(destroyOpts)
	}

	args := []string{"destroy", "--yes", "--skip-preview"}
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

	stdout, stderr, code, err := s.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to destroy stack"), stdout, stderr, code)
	}

	history, err := s.History(ctx)
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
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stack outputs")
	}

	// standard outputs
	outStdout, outStderr, code, err := s.runPulumiCmdSync(ctx, "stack", "output", "--json")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not get outputs"), outStdout, outStderr, code)
	}

	// secret outputs
	secretStdout, secretStderr, code, err := s.runPulumiCmdSync(ctx, "stack", "output", "--json", "--show-secrets")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not get secret outputs"), outStdout, outStderr, code)
	}

	var outputs map[string]interface{}
	var secrets map[string]interface{}

	if err = json.Unmarshal([]byte(outStdout), &outputs); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling outputs: %s", secretStderr)
	}

	if err = json.Unmarshal([]byte(secretStdout), &secrets); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling secret outputs: %s", secretStderr)
	}

	res := make(OutputMap)
	for k, v := range secrets {
		isSecret := outputs[k] == secretSentinel
		res[k] = OutputValue{
			Value:  v,
			Secret: isSecret,
		}
	}

	return res, nil
}

// History returns a list summarizing all previous and current results from Stack lifecycle operations
// (up/preview/refresh/destroy).
func (s *Stack) History(ctx context.Context) ([]UpdateSummary, error) {
	err := s.Workspace().SelectStack(ctx, s.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stack history")
	}

	stdout, stderr, errCode, err := s.runPulumiCmdSync(ctx, "history", "--json", "--show-secrets")
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

// SetConfig sets the specified config KVP.
func (s *Stack) SetConfig(ctx context.Context, key string, val ConfigValue) error {
	return s.Workspace().SetConfig(ctx, s.Name(), key, val)
}

// SetAllConfig sets all values in the provided config map.
func (s *Stack) SetAllConfig(ctx context.Context, config ConfigMap) error {
	return s.Workspace().SetAllConfig(ctx, s.Name(), config)
}

// RemoveConfig removes the specified config KVP.
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

// UpdateSummary provides an summary of a Stack lifecycle operation (up/preview/refresh/destroy).
type UpdateSummary struct {
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

// OutputValue models a Pulumi Stack output, providing the plaintext value and a market indicating secretness.
type OutputValue struct {
	Value  interface{}
	Secret bool
}

// UpResult contains information about a Stack.Up operation,
// including Outputs, and a summary of the deployed changes.
// TODO: remote StdOut in favor of structured info https://github.com/pulumi/pulumi/issues/5218
type UpResult struct {
	StdOut  string
	StdErr  string
	Outputs OutputMap
	Summary UpdateSummary
}

// OutputMap is the output result of running a Pulumi program
type OutputMap map[string]OutputValue

// PreviewStep is summary of the expected state transition of a given resource based on running the current program
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
	Steps         []PreviewStep  `json:"steps"`
	ChangeSummary map[string]int `json:"changeSummary"`
}

// RefreshResult is the output of a successful Stack.Refresh operation
// TODO: replace StdOut with structured output https://github.com/pulumi/pulumi/issues/5220
type RefreshResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

// DestroyResult is the output of a successful Stack.Destroy operation
// TODO: replace StdOut with structured output https://github.com/pulumi/pulumi/issues/5219
type DestroyResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

// secretSentinel represents the CLI response for an output marked as "secret"
const secretSentinel = "[secret]"

func (s *Stack) runPulumiCmdSync(ctx context.Context, args ...string) (string, string, int, error) {
	var env []string
	if s.Workspace().PulumiHome() != nil {
		homeEnv := fmt.Sprintf("%s=%s", pulumiHomeEnv, *s.Workspace().PulumiHome())
		env = append(env, homeEnv)
	}
	additionalArgs, err := s.Workspace().SerializeArgsForOp(ctx, s.Name())
	if err != nil {
		return "", "", -1, errors.Wrap(err, "failed to exec command, error getting additional args")
	}
	args = append(args, additionalArgs...)
	stdout, stderr, errCode, err := runPulumiCommandSync(ctx, s.Workspace().WorkDir(), env, args...)
	if err != nil {
		return stdout, stderr, errCode, err
	}
	err = s.Workspace().PostOpCallback(ctx, s.Name())
	if err != nil {
		return stdout, stderr, errCode, errors.Wrap(err, "command ran successfully, but error running PostOpCallback")
	}
	return stdout, stderr, errCode, nil
}

func (s *Stack) host(ctx context.Context, additionalArgs []string, parallel int) (string, string, error) {
	proj, err := s.Workspace().ProjectSettings(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "could not start run program, failed to start host")
	}

	var stdout bytes.Buffer
	var errBuff bytes.Buffer
	args := []string{"host"}
	args = append(args, additionalArgs...)
	workspaceArgs, err := s.Workspace().SerializeArgsForOp(ctx, s.Name())
	if err != nil {
		return "", "", errors.Wrap(err, "failed to exec command, error getting additional args")
	}
	args = append(args, workspaceArgs...)
	cmd := exec.CommandContext(ctx, "pulumi", args...)
	cmd.Dir = s.Workspace().WorkDir()
	if s.Workspace().PulumiHome() != nil {
		homeEnv := fmt.Sprintf("%s=%s", pulumiHomeEnv, *s.Workspace().PulumiHome())
		cmd.Env = append(os.Environ(), homeEnv)
	}

	cmd.Stdout = &stdout
	stderr, _ := cmd.StderrPipe()
	err = cmd.Start()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to start host command")
	}
	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)

	resMonAddrChan := make(chan string)
	engineAddrChan := make(chan string)
	failChan := make(chan bool)
	go func() {
		numAddrs := 0
		for scanner.Scan() {
			m := scanner.Text()
			errBuff.WriteString(m)
			if strings.HasPrefix(m, "resmon: ") {
				numAddrs++
				// resmon: 127.0.0.1:23423
				resMonAddrChan <- strings.Split(m, " ")[1]
			}
			if strings.HasPrefix(m, "engine: ") {
				numAddrs++
				// engine: 127.0.0.1:23423
				engineAddrChan <- strings.Split(m, " ")[1]
			}
		}
		if numAddrs < 2 {
			failChan <- true
		}
	}()
	var monitorAddr string
	var engineAddr string
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return stdout.String(), errBuff.String(), ctx.Err()
		case <-failChan:
			return stdout.String(), errBuff.String(), errors.New("failed to launch host")
		case monitorAddr = <-resMonAddrChan:
		case engineAddr = <-engineAddrChan:
		}
	}

	cfg, err := s.GetAllConfig(ctx)
	if err != nil {
		return stdout.String(), errBuff.String(), errors.Wrap(err, "failed to serialize config for inline program")
	}
	cfgMap := make(map[string]string)
	for k, v := range cfg {
		cfgMap[k] = v.Value
	}

	runInfo := pulumi.RunInfo{
		EngineAddr:  engineAddr,
		MonitorAddr: monitorAddr,
		Config:      cfgMap,
		Project:     proj.Name.String(),
		Stack:       s.Name(),
	}
	if parallel > 0 {
		runInfo.Parallel = parallel
	}
	err = execUserCode(ctx, s.Workspace().Program(), runInfo)
	if err != nil {
		interruptErr := cmd.Process.Signal(os.Interrupt)
		if interruptErr != nil {
			return stdout.String(), errBuff.String(),
				errors.Wrap(err, "failed to run inline program and shutdown gracefully, could not kill host")
		}
		waitErr := cmd.Wait()
		if waitErr != nil {
			return stdout.String(), errBuff.String(),
				errors.Wrap(err, "failed to run inline program and shutdown gracefully")
		}
		return stdout.String(), errBuff.String(), errors.Wrap(err, "error running inline pulumi program")
	}

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return stdout.String(), errBuff.String(), errors.Wrap(err, "failed to shutdown host gracefully")
	}
	err = cmd.Wait()

	if err != nil {
		return stdout.String(), errBuff.String(), err
	}

	err = s.Workspace().PostOpCallback(ctx, s.Name())
	if err != nil {
		return stdout.String(), errBuff.String(),
			errors.Wrap(err, "command ran successfully, but error running PostOpCallback")
	}

	return stdout.String(), errBuff.String(), nil
}

func execUserCode(ctx context.Context, fn pulumi.RunFunc, info pulumi.RunInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if pErr, ok := r.(error); ok {
				err = errors.Wrap(pErr, "go inline source runtime error, an unhandled error occurred:")
			} else {
				err = errors.New("go inline source runtime error, an unhandled error occurred: unknown error")
			}
		}
	}()
	stack := string(debug.Stack())
	if strings.Contains(stack, "github.com/pulumi/pulumi/sdk/go/pulumi/run.go") {
		return errors.New("nested stack operations are not supported https://github.com/pulumi/pulumi/issues/5058")
	}
	pulumiCtx, err := pulumi.NewContext(ctx, info)
	if err != nil {
		return err
	}
	return pulumi.RunWithContext(pulumiCtx, fn)
}
