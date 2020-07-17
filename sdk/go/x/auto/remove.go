package auto

import "github.com/pkg/errors"

func (s *Stack) Remove() error {

	// TODO figure out setup method lifecycle
	_, err := s.initOrSelectStack()
	if err != nil {
		return errors.Wrap(err, "could not initialize or select stack")
	}

	_, stderr, err := s.runCmd("pulumi", "stack", "rm", "--yes")
	if err != nil {
		return errors.Wrapf(err, "failed to remove stack: %s", stderr)
	}

	return nil
}
