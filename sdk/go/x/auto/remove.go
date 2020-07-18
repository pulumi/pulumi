package auto

import "github.com/pkg/errors"

func (s *stack) Remove() error {
	err := s.initOrSelectStack()
	if err != nil {
		return errors.Wrap(err, "could not initialize or select stack")
	}

	stdout, stderr, code, err := s.runCmd("pulumi", "stack", "rm", "--yes")
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to remove stack"), stdout, stderr, code)
	}

	return nil
}
