package auto

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func (s *stack) Up() (UpResult, error) {
	var upResult UpResult

	err := s.initOrSelectStack()
	if err != nil {
		return upResult, err
	}

	stdout, stderr, code, err := s.runCmd("pulumi", "up", "--yes")
	if err != nil {
		return upResult, newAutoError(err, stdout, stderr, code)
	}

	outs, secrets, err := s.outputs()
	if err != nil {
		return upResult, err
	}

	summary, err := s.summary()
	if err != nil {
		return upResult, err
	}

	return UpResult{
		StdOut:        stdout,
		StdErr:        stderr,
		Outputs:       outs,
		SecretOutputs: secrets,
		Summary:       summary,
	}, nil
}

type UpResult struct {
	StdOut        string
	StdErr        string
	Outputs       map[string]interface{}
	SecretOutputs map[string]interface{}
	Summary       UpdateSummary
}

func (s *stack) initOrSelectStack() error {
	_, _, _, err := s.runCmd("pulumi", "stack", "select", s.Name)
	if err != nil {
		stdout, stderr, code, err := s.runCmd("pulumi", "stack", "init", s.Name)
		if err != nil {
			return newAutoError(errors.Wrap(err, "unable to select or init stack"), stdout, stderr, code)
		}
	}

	return nil
}

const secretSentinel = "[secret]"

func (s *stack) Outputs() (map[string]interface{}, map[string]interface{}, error) {
	err := s.initOrSelectStack()
	if err != nil {
		return nil, nil, err
	}

	return s.outputs()
}

// outputs returns a set of plain outputs, secret outputs, and an error
func (s *stack) outputs() (map[string]interface{}, map[string]interface{}, error) {
	// standard outputs
	outStdout, outStderr, code, err := s.runCmd("pulumi", "stack", "output", "--json")
	if err != nil {
		return nil, nil, newAutoError(errors.Wrap(err, "could not get outputs"), outStdout, outStderr, code)
	}

	// secret outputs
	secretStdout, secretStderr, code, err := s.runCmd("pulumi", "stack", "output", "--json", "--show-secrets")
	if err != nil {
		return nil, nil, newAutoError(errors.Wrap(err, "could not get secret outputs"), outStdout, outStderr, code)
	}

	var outputs map[string]interface{}
	var secrets map[string]interface{}

	if err = json.Unmarshal([]byte(outStdout), &outputs); err != nil {
		return nil, nil, errors.Wrapf(err, "error unmarshalling outputs: %s", secretStderr)
	}

	if err = json.Unmarshal([]byte(secretStdout), &secrets); err != nil {
		return nil, nil, errors.Wrapf(err, "error unmarshalling secret outputs: %s", secretStderr)
	}

	for k, v := range outputs {
		if v == secretSentinel {
			delete(outputs, k)
		} else {
			delete(secrets, k)
		}
	}

	return outputs, secrets, nil
}

func (s *stack) User() (string, error) {
	err := s.initOrSelectStack()
	if err != nil {
		return "", errors.Wrap(err, "could not initialize or select stack")
	}
	outStdout, outStderr, code, err := s.runCmd("pulumi", "whoami")
	if err != nil {
		return "", newAutoError(errors.Wrap(err, "could not detect user"), outStdout, outStderr, code)
	}
	return strings.TrimSpace(outStdout), nil
}

func (s *stack) SetConfig(config map[string]string) error {
	err := s.initOrSelectStack()
	if err != nil {
		return err
	}

	return s.setConfig(config)
}

func (s *stack) setConfig(config map[string]string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for k, v := range config {
		// TODO verify escaping
		outstr, errstr, code, err := s.runCmd("pulumi", "config", "set", k, v)
		stdout.WriteString(outstr)
		stderr.WriteString(errstr)
		if err != nil {
			return newAutoError(errors.Wrap(err, "unable to set config"), stdout.String(), stderr.String(), code)
		}
	}

	return nil

}

func (s *stack) SetSecrets(secrets map[string]string) error {
	err := s.initOrSelectStack()
	if err != nil {
		return err
	}

	return s.setSecrets(secrets)
}

func (s *stack) setSecrets(secrets map[string]string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for k, v := range secrets {
		// TODO verify escaping
		outstr, errstr, code, err := s.runCmd("pulumi", "config", "set", k, v)
		stdout.WriteString(outstr)
		stderr.WriteString(errstr)
		if err != nil {
			return newAutoError(
				errors.Wrap(err, "unable to set secret config"), stdout.String(), stderr.String(), code,
			)
		}
	}

	return nil

}

func (s *stack) Summary() (UpdateSummary, error) {
	var zero UpdateSummary
	err := s.initOrSelectStack()
	if err != nil {
		return zero, err
	}

	return s.summary()
}

func (s *stack) summary() (UpdateSummary, error) {
	var res UpdateSummary
	stdout, stderr, code, err := s.runCmd("pulumi", "history", "--json")
	if err != nil {
		return res, newAutoError(errors.Wrap(err, "could not get outputs"), stdout, stderr, code)
	}

	var history []UpdateSummary
	err = json.Unmarshal([]byte(stdout), &history)
	if err != nil {
		return res, errors.Wrap(err, "unable to unmarshal history result")
	}

	if len(history) != 0 {
		res = history[0]
	}

	return res, nil
}

// lifted from:
// https://github.com/pulumi/pulumi/blob/66bd3f4aa8f9a90d3de667828dda4bed6e115f6b/pkg/cmd/pulumi/history.go#L91

type UpdateSummary struct {
	Kind        string                 `json:"kind"`
	StartTime   string                 `json:"startTime"`
	Message     string                 `json:"message"`
	Environment map[string]string      `json:"environment"`
	Config      map[string]interface{} `json:"config"`
	Result      string                 `json:"result,omitempty"`

	// These values are only present once the update finishes
	EndTime         *string         `json:"endTime,omitempty"`
	ResourceChanges *map[string]int `json:"resourceChanges,omitempty"`
}

// runCmd execs the given command with appropriate stack context
// returning stdout, stderr, exitcode, and an error value
func (s *stack) runCmd(name string, arg ...string) (string, string, int, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = s.SourcePath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := -2 // unknown
	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		code = exitError.ExitCode()
	}
	return stdout.String(), stderr.String(), code, err
}
