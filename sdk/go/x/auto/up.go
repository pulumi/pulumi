package auto

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// TODO JSON errors produce diagnostics, not STD err

func (s *Stack) Up() (UpResult, error) {
	var upResult UpResult

	res, err := s.initOrSelectStack()
	if err != nil {
		return res, errors.Wrap(err, "could not initialize or select stack")
	}

	err = s.writeProject()
	if err != nil {
		return upResult, err
	}

	// Open question -
	// when to do run setup methods?
	// Should this be done for each lifecycle method?
	// Perhaps just once upon NewStack()...?
	err = s.writeStack()
	if err != nil {
		return upResult, err
	}

	_, cfgStderr, err := s.setConfig()
	if err != nil {
		return upResult, errors.Wrapf(err, "unable to set config: %s", cfgStderr)
	}

	_, secretsStderr, err := s.setSecrets()
	if err != nil {
		return upResult, errors.Wrapf(err, "unable to set secrets: %s", secretsStderr)
	}

	stdout, stderr, err := s.runCmd("pulumi", "up", "--yes")
	if err != nil {
		return UpResult{
			StdErr: stderr,
			StdOut: stdout,
		}, errors.Wrapf(err, "stderr: %s", stderr)
	}

	outs, secrets, err := s.getOutputs()
	if err != nil {
		return upResult, err
	}

	lastResult, err := s.lastResult()
	if err != nil {
		return upResult, err
	}

	return UpResult{
		StdOut:        stdout,
		StdErr:        stderr,
		Outputs:       outs,
		SecretOutputs: secrets,
		Summary:       lastResult,
	}, nil
}

type UpResult struct {
	StdOut        string
	StdErr        string
	Outputs       map[string]interface{}
	SecretOutputs map[string]interface{}
	Summary       UpdateSummary
}

func (s *Stack) initOrSelectStack() (UpResult, error) {
	var upResult UpResult

	_, _, err := s.runCmd("pulumi", "stack", "select", s.Name)
	if err != nil {
		initStdout, initStderr, err := s.runCmd("pulumi", "stack", "init", s.Name)
		if err != nil {
			// TODO this is not the right type
			return UpResult{
				StdErr: initStderr,
				StdOut: initStdout,
			}, errors.Wrapf(err, "unable to select or init stack: %s", initStderr)
		}
	}

	// now parse pulumi.yaml, pulumi.stack.yaml,
	// merge with our model
	// re-write
	// pulumi config set, config set --secrets`

	// TODO consider storing hash of the yaml files so we don't have to do this for every 'up'
	return upResult, nil
}

const secretSentinel = "[secret]"

func (s *Stack) GetOutputs() (map[string]interface{}, map[string]interface{}, error) {
	_, err := s.initOrSelectStack()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not initialize or select stack")
	}

	return s.getOutputs()
}

//getOutputs returns a set of plain outputs, secret outputs, and an error
func (s *Stack) getOutputs() (map[string]interface{}, map[string]interface{}, error) {
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

func (s *Stack) GetUser() (string, error) {
	_, err := s.initOrSelectStack()
	if err != nil {
		return "", errors.Wrap(err, "could not initialize or select stack")
	}
	outStdout, outStderr, err := s.runCmd("pulumi", "whoami")
	if err != nil {
		return "", errors.Wrapf(err, "could not detect user: stderr: %s", outStderr)
	}
	return strings.TrimSpace(outStdout), nil
}

// TODO need to refactor to have a public and private endpoint for construction vs everyday use
func (s *Stack) setConfig() (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for k, v := range s.Overrides.Config {
		// TODO verify escaping
		outstr, errstr, err := s.runCmd("pulumi", "config", "set", k, v)
		stdout.WriteString(outstr)
		stderr.WriteString(errstr)
		if err != nil {
			return stdout.String(), stderr.String(), err
		}
	}

	return stdout.String(), stderr.String(), nil

}

// TODO need to refactor to have a public and private endpoint for construction vs everyday use
func (s *Stack) setSecrets() (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for k, v := range s.Overrides.Secrets {
		// TODO verify escaping
		outstr, errstr, err := s.runCmd("pulumi", "config", "set", "--secret", k, v)
		stdout.WriteString(outstr)
		stderr.WriteString(errstr)
		if err != nil {
			return stdout.String(), stderr.String(), err
		}
	}

	return stdout.String(), stderr.String(), nil

}

func (s *Stack) LastResult() (UpdateSummary, error) {
	var zero UpdateSummary
	_, err := s.initOrSelectStack()
	if err != nil {
		return zero, errors.Wrap(err, "could not initialize or select stack")
	}

	return s.lastResult()
}

// lifted from https://github.com/pulumi/pulumi/blob/66bd3f4aa8f9a90d3de667828dda4bed6e115f6b/pkg/cmd/pulumi/history.go#L91
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

func (s *Stack) lastResult() (UpdateSummary, error) {
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

// runCmd execs the given command with appropriate stack context
// returning stdout, stderr, and an error value
func (s *Stack) runCmd(name string, arg ...string) (string, string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = s.Project.SourcePath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
