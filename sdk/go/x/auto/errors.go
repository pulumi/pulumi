package auto

import (
	"strings"

	"github.com/pkg/errors"
)

type autoError struct {
	stdout string
	stderr string
	code   int
	err    error
}

func newAutoError(err error, stdout, stderr string, code int) autoError {
	return autoError{
		stdout,
		stderr,
		code,
		err,
	}
}

func (ae autoError) Error() string {
	return errors.Wrap(ae.err, ae.stderr).Error()
}

func IsConflict(e error) bool {
	ae, ok := e.(autoError)
	if !ok {
		return false
	}

	return strings.Contains(ae.stderr, "[409] Conflict: Another update is currently in progress.")
}
