package auto

import (
	"fmt"
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
	return errors.Wrap(
		ae.err, fmt.Sprintf("code: %d\n, stdout: %s\n, stderr: %s\n", ae.code, ae.stdout, ae.stderr),
	).Error()
}

func IsConcurrentUpdateError(e error) bool {
	ae, ok := e.(autoError)
	if !ok {
		return false
	}

	return strings.Contains(ae.stderr, "[409] Conflict: Another update is currently in progress.")
}

// IsCompilationError returns true if there was an error compiling the user program (only typescript, go, dotnet)
func IsCompilationError(e error) bool {
	as, ok := e.(autoError)
	if !ok {
		return false
	}

	// dotnet
	if strings.Contains(as.stdout, "Build FAILED.") {
		return true
	}

	// go
	// TODO: flimsy for go
	if strings.Contains(as.stdout, ": syntax error:") {
		return true
	}

	if strings.Contains(as.stdout, ": undefined:") {
		return true
	}

	// typescript
	if strings.Contains(as.stdout, "Unable to compile TypeScript") {
		return true
	}

	return false
}

// IsRuntimeError returns true if there was an error in the user program at during execution.
func IsRuntimeError(e error) bool {
	as, ok := e.(autoError)
	if !ok {
		return false
	}

	if IsCompilationError(e) {
		return false
	}

	// js/ts/dotnet/python
	if strings.Contains(as.stdout, "failed with an unhandled exception:") {
		return true
	}

	// go
	if strings.Contains(as.stdout, "panic: runtime error:") {
		return true
	}
	if strings.Contains(as.stdout, "an unhandled error occurred:") {
		return true
	}

	if strings.Contains(as.Error(), "go inline source runtime error") {
		return true
	}

	return false
}

// IsUnexpectedEngineError returns true if the pulumi core engine encountered an error (most likely a bug).
func IsUnexpectedEngineError(e error) bool {
	// TODO: figure out how to write a test for this
	as, ok := e.(autoError)
	if !ok {
		return false
	}

	return strings.Contains(as.stdout, "The Pulumi CLI encountered a fatal error. This is a bug!")
}

// TODO IsProviderError
// errors are not consistently labeled, will likely need a new strategy.
// Error creating S3 bucket: BucketAlreadyExists: The requested bucket name is not available.
// could not make instance of 'aws:s3/bucket:Bucket': name '...' plus 7 random chars is longer than maximum length 63
// Error creating S3 bucket: IllegalLocationConstraintException
