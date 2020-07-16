package auto

import "github.com/pkg/errors"

func (s *Stack) Destroy() (DestroyResult, error) {
	var dResult DestroyResult

	// TODO figure out setup method lifecycle
	_, err := s.initOrSelectStack()
	if err != nil {
		return dResult, errors.Wrap(err, "could not initialize or select stack")
	}

	err = s.writeProject()
	if err != nil {
		return dResult, err
	}

	err = s.writeStack()
	if err != nil {
		return dResult, err
	}

	_, cfgStderr, err := s.setConfig()
	if err != nil {
		return dResult, errors.Wrapf(err, "unable to set config: %s", cfgStderr)
	}

	_, secretsStderr, err := s.setSecrets()
	if err != nil {
		return dResult, errors.Wrapf(err, "unable to set secrets: %s", secretsStderr)
	}

	stdout, stderr, err := s.runCmd("pulumi", "destroy", "--yes")
	if err != nil {
		return DestroyResult{
			StdErr: stderr,
			StdOut: stdout,
		}, errors.Wrapf(err, "stderr: %s", stderr)
	}

	lastResult, err := s.lastResult()
	if err != nil {
		return dResult, err
	}

	return DestroyResult{
		StdOut:  stdout,
		StdErr:  stderr,
		Summary: lastResult,
	}, nil
}

type DestroyResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}
