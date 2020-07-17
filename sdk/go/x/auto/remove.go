package auto

import "github.com/pkg/errors"

func (s *stack) Remove() error {
	err := s.initOrSelectStack()
	if err != nil {
		return errors.Wrap(err, "could not initialize or select stack")
	}

	_, stderr, err := s.runCmd("pulumi", "stack", "rm", "--yes")
	if err != nil {
		return errors.Wrapf(err, "failed to remove stack: %s", stderr)
	}

	return nil
}
