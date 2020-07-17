package auto

import "github.com/pkg/errors"

func (s *stack) Refresh() (RefreshResult, error) {
	var dResult RefreshResult

	err := s.initOrSelectStack()
	if err != nil {
		return dResult, errors.Wrap(err, "could not initialize or select stack")
	}

	stdout, stderr, err := s.runCmd("pulumi", "refresh", "--yes")
	if err != nil {
		return RefreshResult{
			StdErr: stderr,
			StdOut: stdout,
		}, errors.Wrapf(err, "stderr: %s", stderr)
	}

	summary, err := s.summary()
	if err != nil {
		return dResult, err
	}

	return RefreshResult{
		StdOut:  stdout,
		StdErr:  stderr,
		Summary: summary,
	}, nil
}

type RefreshResult struct {
	StdOut  string
	StdErr  string
	Summary UpdateSummary
}
