// Copyright 2016-2025, Pulumi Corporation.
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

package cmdutil

import (
	"errors"
	"fmt"
)

type ExitCoder interface {
	error
	ExitCode() int
}

type CLIError struct {
	Err  error
	Code int
	Msg  string
}

func (e *CLIError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

func (e *CLIError) ExitCode() int {
	return e.Code
}

func (e *CLIError) Unwrap() error {
	return e.Err
}

func NewCLIError(code int, msg string) *CLIError {
	return &CLIError{
		Code: code,
		Msg:  msg,
	}
}

func WrapError(code int, err error) *CLIError {
	return &CLIError{
		Err:  err,
		Code: code,
	}
}

func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitCoder ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}

	return ExitCodeError
}
