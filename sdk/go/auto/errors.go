// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auto

import (
	"fmt"
	"regexp"
	"strings"
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
	return fmt.Sprintf("%s\ncode: %d\nstdout: %s\nstderr: %s\n", ae.err.Error(), ae.code, ae.stdout, ae.stderr)
}

// IsConcurrentUpdateError returns true if the error was a result of a conflicting update locking the stack.
func IsConcurrentUpdateError(e error) bool {
	ae, ok := e.(autoError)
	if !ok {
		return false
	}

	return strings.Contains(ae.stderr, "[409] Conflict: Another update is currently in progress.")
}

// IsSelectStack404Error returns true if the error was a result of selecting a stack that does not exist.
func IsSelectStack404Error(e error) bool {
	ae, ok := e.(autoError)
	if !ok {
		return false
	}

	regex := regexp.MustCompile(`no stack named.*found`)
	return regex.MatchString(ae.stderr)
}

// IsCreateStack409Error returns true if the error was a result of creating a stack that already exists.
func IsCreateStack409Error(e error) bool {
	ae, ok := e.(autoError)
	if !ok {
		return false
	}

	regex := regexp.MustCompile(`stack.*already exists`)
	return regex.MatchString(ae.stderr)
}

// IsCompilationError returns true if the program failed at the build/run step (only Typescript, Go, .NET)
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
