package auto

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func (s *Stack) Up() (UpResult, error) {
	var upResult UpResult

	// TODO setup - merge pulumi.yaml, set config, etc.
	res, err := s.initOrSelectStack()
	if err != nil {
		return res, errors.Wrap(err, "could not initialize or select stack")
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

	// TODO - last histroy item.

	return UpResult{
		StdOut:        stdout,
		StdErr:        stderr,
		Outputs:       outs,
		SecretOutputs: secrets,
	}, nil
}

type UpResult struct {
	StdOut        string
	StdErr        string
	Outputs       map[string]interface{}
	SecretOutputs map[string]interface{}
	Summary       map[string]interface{}
}

func (s *Stack) initOrSelectStack() (UpResult, error) {
	var upResult UpResult

	_, _, err := s.runCmd("pulumi", "stack", "select", s.Name)
	if err != nil {
		initStdout, initStderr, err := s.runCmd("pulumi", "stack", "init", s.Name)
		if err != nil {
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
	outStdout, outStderr, err := s.runCmd("pulumi", "whoami")
	if err != nil {
		return "", errors.Wrapf(err, "could not detect user: stderr: %s", outStderr)
	}
	return strings.TrimSpace(outStdout), nil
}

//summary - call history and get last item

// configure - write the pulumi.yaml, pulumi.<stack>.yaml, set config & secrets

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
