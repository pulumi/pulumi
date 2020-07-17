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
		return upResult, errors.Wrap(err, "could not initialize or select stack")
	}

	stdout, stderr, err := s.runCmd("pulumi", "up", "--yes")
	if err != nil {
		return UpResult{
			StdErr: stderr,
			StdOut: stdout,
		}, errors.Wrapf(err, "stderr: %s", stderr)
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
	_, _, err := s.runCmd("pulumi", "stack", "select", s.Name)
	if err != nil {
		_, initStderr, err := s.runCmd("pulumi", "stack", "init", s.Name)
		if err != nil {
			return errors.Wrapf(err, "unable to select or init stack: %s", initStderr)
		}
	}

	return nil
}

const secretSentinel = "[secret]"

func (s *stack) Outputs() (map[string]interface{}, map[string]interface{}, error) {
	err := s.initOrSelectStack()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not initialize or select stack")
	}

	return s.outputs()
}

// outputs returns a set of plain outputs, secret outputs, and an error
func (s *stack) outputs() (map[string]interface{}, map[string]interface{}, error) {
	// standard outputs
	outStdout, outStderr, err := s.runCmd("pulumi", "stack", "output", "--json")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not get outputs: stderr: %s", outStderr)
	}

	// secret outputs
	secretStdout, secretStderr, err := s.runCmd("pulumi", "stack", "output", "--json", "--show-secrets")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not get secret outputs: stderr: %s", secretStderr)
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
	outStdout, outStderr, err := s.runCmd("pulumi", "whoami")
	if err != nil {
		return "", errors.Wrapf(err, "could not detect user: stderr: %s", outStderr)
	}
	return strings.TrimSpace(outStdout), nil
}

func (s *stack) SetConfig(config map[string]string) error {
	err := s.initOrSelectStack()
	if err != nil {
		return errors.Wrap(err, "could not initialize or select stack")
	}

	return s.setConfig(config)
}

func (s *stack) setConfig(config map[string]string) error {
	var stderr bytes.Buffer

	for k, v := range config {
		// TODO verify escaping
		_, errstr, err := s.runCmd("pulumi", "config", "set", k, v)
		stderr.WriteString(errstr)
		if err != nil {
			return errors.Wrapf(err, "unable to set config, stderr: %s", errstr)
		}
	}

	return nil

}

func (s *stack) SetSecrets(secrets map[string]string) error {
	err := s.initOrSelectStack()
	if err != nil {
		return errors.Wrap(err, "could not initialize or select stack")
	}

	return s.setSecrets(secrets)
}

func (s *stack) setSecrets(secrets map[string]string) error {
	var stderr bytes.Buffer

	for k, v := range secrets {
		// TODO verify escaping
		_, errstr, err := s.runCmd("pulumi", "config", "set", "--secret", k, v)
		stderr.WriteString(errstr)
		if err != nil {
			return errors.Wrapf(err, "unable to set secret config, stderr: %s", errstr)
		}
	}

	return nil

}

func (s *stack) Summary() (UpdateSummary, error) {
	var zero UpdateSummary
	err := s.initOrSelectStack()
	if err != nil {
		return zero, errors.Wrap(err, "could not initialize or select stack")
	}

	return s.summary()
}

func (s *stack) summary() (UpdateSummary, error) {
	var res UpdateSummary
	histOut, histErr, err := s.runCmd("pulumi", "history", "--json")
	if err != nil {
		// TODO JSON errors produce diagnostics, not STD err
		return res, errors.Wrapf(err, "could not get outputs: stderr: %s", histErr)
	}

	var history []UpdateSummary
	err = json.Unmarshal([]byte(histOut), &history)
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
// returning stdout, stderr, and an error value
func (s *stack) runCmd(name string, arg ...string) (string, string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = s.SourcePath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
