package auto

import "github.com/pkg/errors"

func (s *stack) Destroy() (DestroyResult, error) {
	var dResult DestroyResult

	err := s.initOrSelectStack()
	if err != nil {
		return dResult, err
	}

	stdout, stderr, code, err := s.runCmd("pulumi", "destroy", "--yes")
	if err != nil {
		return dResult,
			newAutoError(errors.Wrap(err, "failed to destroy stack"), stdout, stderr, code)
	}

	summary, err := s.summary()
	if err != nil {
		return dResult, err
	}

	return DestroyResult{
		StdOut:  stdout,
		StdErr:  stderr,
		Summary: summary,
	}, nil
}

type DestroyResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}
