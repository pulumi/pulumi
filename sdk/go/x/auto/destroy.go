package auto

import "github.com/pkg/errors"

func (s *stack) Destroy() (DestroyResult, error) {
	var dResult DestroyResult

	err := s.initOrSelectStack()
	if err != nil {
		return dResult, errors.Wrap(err, "could not initialize or select stack")
	}

	stdout, stderr, err := s.runCmd("pulumi", "destroy", "--yes")
	if err != nil {
		return DestroyResult{
			StdErr: stderr,
			StdOut: stdout,
		}, errors.Wrapf(err, "stderr: %s", stderr)
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
