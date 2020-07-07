package auto

import (
	"bytes"
	"encoding/json"
	"os/exec"

	"github.com/pkg/errors"
)

func (s *Stack) Up() (UpResult, error) {
	// TODO setup - merge pulumi.yaml, set config, etc.

	var upResult UpResult
	cmd := exec.Command("pulumi", "up", "--yes")
	cmd.Dir = s.Project.SourcePath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return upResult, errors.Wrapf(err, "stderr: %s", stderr.String())
	}

	outs, secrets, err := s.getOutputs()
	if err != nil {
		return upResult, err
	}

	// TODO - last histroy item.

	return UpResult{
		StdOut:        stdout.String(),
		StdErr:        stderr.String(),
		Outputs:       outs,
		SecretOutputs: secrets,
	}, nil
}

type UpResult struct {
	StdOut        string
	StdErr        string
	Outputs       map[string]string
	SecretOutputs map[string]string
	Summary       map[string]interface{}
}

const secretSentinel = "[secret]"

//getOutputs returns a set of plain outputs, secret outputs, and an error
func (s *Stack) getOutputs() (map[string]string, map[string]string, error) {
	// standard outputs
	outCmd := exec.Command("pulumi", "stack", "output", "--json")
	outCmd.Dir = s.Project.SourcePath
	var outStdout bytes.Buffer
	var outStderr bytes.Buffer
	outCmd.Stdout = &outStdout
	outCmd.Stderr = &outStderr
	err := outCmd.Run()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not get outputs: stderr: %s", outStderr.String())
	}

	// secret outputs
	secretOutCmd := exec.Command("pulumi", "stack", "output", "--json", "--show-secrets")
	secretOutCmd.Dir = s.Project.SourcePath
	var secretStdout bytes.Buffer
	var secretStderr bytes.Buffer
	secretOutCmd.Stdout = &secretStdout
	secretOutCmd.Stderr = &secretStderr
	err = secretOutCmd.Run()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not get secret outputs: stderr: %s", secretStderr.String())
	}

	var outputs map[string]string
	var secrets map[string]string

	if err = json.Unmarshal(outStdout.Bytes(), &outputs); err != nil {
		return nil, nil, errors.Wrapf(err, "error unmarshalling outputs: %s", secretStderr.String())
	}

	if err = json.Unmarshal(secretStdout.Bytes(), &secrets); err != nil {
		return nil, nil, errors.Wrapf(err, "error unmarshalling secret outputs: %s", secretStderr.String())
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

//summary - call history and get last item

// configure - write the pulumi.yaml, pulumi.<stack>.yaml, set config & secrets
