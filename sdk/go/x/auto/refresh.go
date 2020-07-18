package auto

import "github.com/pkg/errors"

func (s *stack) Refresh() (RefreshResult, error) {
	var rResult RefreshResult

	err := s.initOrSelectStack()
	if err != nil {
		return rResult, err
	}

	stdout, stderr, code, err := s.runCmd("pulumi", "refresh", "--yes")
	if err != nil {
		return rResult, newAutoError(errors.Wrap(err, "failed to refresh stack"), stdout, stderr, code)
	}

	summary, err := s.summary()
	if err != nil {
		return rResult, err
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
