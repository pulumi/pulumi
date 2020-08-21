// Package auto contains the Pulumi Automation API, the programmatic interface for driving Pulumi programs
// without the CLI.
// Generally this can be thought of as encapsulating the functionality of the CLI but with more flexibility
// (`pulumi up`, `pulumi preview`, pulumi destroy`, `pulumi stack init`, etc.). This still requires a
// CLI binary to be installed and available on your $PATH.
//
// The Automation API provides a natural way to orchestrate multiple stacks,
// feeding the output of one stack as an input to the next as shown in the example below.
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
// 	- Building higher level tools, custom CLIs over pulumi, etc.
//
//  - Using pulumi behind a REST or GRPC API.
package auto

import (
	"bufio"
	"bytes"
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
)

type Stack struct {
	workspace Workspace
	fqsn      string
}

func FullyQualifiedStackName(org, project, stack string) string {
	return fmt.Sprintf("%s/%s/%s", org, project, stack)
}

// NewStack creates a new stack using the given workspace, and fully qualified stack name (org/project/name).
// It fails if a stack with that name already exists
func NewStack(fqsn string, ws Workspace) (Stack, error) {
	var s Stack
	s = Stack{
		workspace: ws,
		fqsn:      fqsn,
	}

	err := ws.CreateStack(fqsn)
	if err != nil {
		return s, err
	}

	return s, nil
}

// SelectStack creates a new stack using the given workspace, and fully qualified stack name (org/project/name).
func SelectStack(fqsn string, ws Workspace) (Stack, error) {
	var s Stack
	s = Stack{
		workspace: ws,
		fqsn:      fqsn,
	}

	err := ws.SelectStack(fqsn)
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
func (s *Stack) Preview() (PreviewResult, error) {
	var res PreviewResult

	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to run preview")
	}

	var stdout, stderr string
	var code int
	if s.Workspace().Program() != nil {
		stdout, stderr, err = s.host(true /*isPreview*/)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run preview"), stdout, stderr, code)
		}
	} else {
		stdout, stderr, code, err = s.runPulumiCmdSync("preview", "--json")
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
func (s *Stack) Up() (UpResult, error) {
	var res UpResult
	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to run update")
	}

	var stdout, stderr string
	var code int
	if s.Workspace().Program() != nil {
		// TODO need to figure out how to get error code...
		stdout, stderr, err = s.host(false /*isPreview*/)
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run update"), stdout, stderr, code)
		}
	} else {
		stdout, stderr, code, err = s.runPulumiCmdSync("up", "--yes")
		if err != nil {
			return res, newAutoError(errors.Wrap(err, "failed to run update"), stdout, stderr, code)
		}
	}

	outs, err := s.Outputs()
	if err != nil {
		return res, err
	}

	history, err := s.History()
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

func (s *Stack) Refresh() (RefreshResult, error) {
	var res RefreshResult

	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to refresh stack")
	}

	stdout, stderr, code, err := s.runPulumiCmdSync("refresh", "--yes")
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to refresh stack"), stdout, stderr, code)
	}

	history, err := s.History()
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

func (s *Stack) Destroy() (DestroyResult, error) {
	var res DestroyResult

	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return res, errors.Wrap(err, "failed to destroy stack")
	}

	stdout, stderr, code, err := s.runPulumiCmdSync("destroy", "--yes")
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "failed to destroy stack"), stdout, stderr, code)
	}

	history, err := s.History()
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

// Outputs get the current set of Stack outputs from the last Stack.Up()
func (s *Stack) Outputs() (OutputMap, error) {
	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stack outputs")
	}

	// standard outputs
	outStdout, outStderr, code, err := s.runPulumiCmdSync("stack", "output", "--json")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not get outputs"), outStdout, outStderr, code)
	}

	// secret outputs
	secretStdout, secretStderr, code, err := s.runPulumiCmdSync("stack", "output", "--json", "--show-secrets")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not get secret outputs"), outStdout, outStderr, code)
	}

	var outputs map[string]string
	var secrets map[string]string

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

// History returns a list summarizing all previous and current results from Stack.Up()
func (s *Stack) History() ([]UpdateSummary, error) {
	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stack history")
	}

	stdout, stderr, errCode, err := s.runPulumiCmdSync("history", "--json", "--show-secrets")
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

// GetConfig returns the config value associated with the specified key
func (s *Stack) GetConfig(key string) (ConfigValue, error) {
	return s.Workspace().GetConfig(s.Name(), key)
}

// GetAllConfig returns the full config map.
func (s *Stack) GetAllConfig() (ConfigMap, error) {
	return s.Workspace().GetAllConfig(s.Name())
}

// SetConfig sets the specified config KVP.
func (s *Stack) SetConfig(key string, val ConfigValue) error {
	return s.Workspace().SetConfig(s.Name(), key, val)
}

// SetAllConfig sets all values in the provided config map.
func (s *Stack) SetAllConfig(config ConfigMap) error {
	return s.Workspace().SetAllConfig(s.Name(), config)
}

// RemoveConfig removes the specified config KVP.
func (s *Stack) RemoveConfig(key string) error {
	return s.Workspace().RemoveConfig(s.Name(), key)
}

// RemoveAllConfig removes all values in the provided list of keys.
func (s *Stack) RemoveAllConfig(keys []string) error {
	return s.Workspace().RemoveAllConfig(s.Name(), keys)
}

// RefreshConfig gets and sets the config map used with the last Update.
func (s *Stack) RefreshConfig() (ConfigMap, error) {
	return s.Workspace().RefreshConfig(s.Name())
}

// Info returns a summary of the Stack including its URL
func (s *Stack) Info() (StackSummary, error) {
	var info StackSummary
	err := s.Workspace().SelectStack(s.Name())
	if err != nil {
		return info, errors.Wrap(err, "failed to fetch stack info")
	}

	summary, err := s.Workspace().Stack()
	if err != nil {
		return info, errors.Wrap(err, "failed to fetch stack info")
	}

	if summary != nil {
		info = *summary
	}

	return info, nil
}

// UpdateSummary provides an summary of a Stack.Up() operation
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

// OutputValue models a Pulumi Stack output
type OutputValue struct {
	Value  string
	Secret bool
}

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

type RefreshResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

type DestroyResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}

// secretSentinel represents the CLI response for an output marked as "secret"
const secretSentinel = "[secret]"

func (s *Stack) runPulumiCmdSync(args ...string) (string, string, int, error) {
	var env []string
	if s.Workspace().PulumiHome() != nil {
		env = append(env, *s.Workspace().PulumiHome())
	}
	return runPulumiCommandSync(s.Workspace().WorkDir(), env, args...)
}

// TODO set pulumi home env var, etc
func (s *Stack) host(isPreview bool) (string, string, error) {
	proj, err := s.Workspace().ProjectSettings()
	if err != nil {
		return "", "", errors.Wrap(err, "could not start run program, failed to start host")
	}

	var stdout bytes.Buffer
	var errBuff bytes.Buffer
	args := []string{"host"}
	if isPreview {
		args = append(args, "preview")
	}
	cmd := exec.Command("pulumi", args...)
	cmd.Dir = s.Workspace().WorkDir()

	cmd.Stdout = &stdout
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
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
		case <-failChan:
			return stdout.String(), errBuff.String(), errors.New("failed to launch host")
		case monitorAddr = <-resMonAddrChan:
		case engineAddr = <-engineAddrChan:
		}
	}

	os.Setenv(pulumi.EnvMonitor, monitorAddr)
	os.Setenv(pulumi.EnvProject, proj.Name.String())
	os.Setenv(pulumi.EnvStack, s.Name())
	os.Setenv(pulumi.EnvEngine, engineAddr)

	cfg, err := s.GetAllConfig()
	if err != nil {
		return stdout.String(), errBuff.String(), errors.Wrap(err, "failed to serialize config for inline program")
	}
	cfgMap := make(map[string]string)
	for k, v := range cfg {
		cfgMap[k] = v.Value
	}
	cfgStr, err := json.Marshal(cfgMap)
	if err != nil {
		return stdout.String(), errBuff.String(), errors.Wrap(err, "unable to marshal config")
	}
	os.Setenv(pulumi.EnvConfig, string(cfgStr))
	err = execUserCode(s.Workspace().Program())
	if err != nil {
		cmd.Process.Signal(os.Interrupt)
		waitErr := cmd.Wait()
		if waitErr != nil {
			return stdout.String(), errBuff.String(), errors.Wrap(err, "failed to run inline program and shutdown gracefully")
		}
		return stdout.String(), errBuff.String(), errors.Wrap(err, "error running inline pulumi program")
	}

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return stdout.String(), errBuff.String(), errors.Wrap(err, "failed to shutdown host gracefully")
	}
	cmd.Wait()

	return stdout.String(), errBuff.String(), nil
}

func execUserCode(fn pulumi.RunFunc) (err error) {
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

	err = pulumi.RunErr(fn)
	return err
}
