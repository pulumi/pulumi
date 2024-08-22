// Copyright 2016-2023, Pulumi Corporation.
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

package result

import (
	"errors"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type bailError struct {
	err error
}

// A BailError represents an expected error or a graceful failure -- that is
// something which is not a bug but a normal (albeit unhappy-path) part of the
// program's execution. A BailError implements the Error interface but will
// prefix its error string with "BAIL: ", which if ever seen in user-facing
// messages indicates that a check for bailing was missed. It also does *not*
// implement Unwrap. To ascertain whether an error is a BailError, use the
// IsBail function.
func BailError(err error) error {
	contract.Requiref(err != nil, "err", "must not be nil")

	return &bailError{err: err}
}

func (b *bailError) Error() string {
	return fmt.Sprintf("BAIL: %v", b.err)
}

// BailErrorf is a helper for BailError(fmt.Errorf(...)).
func BailErrorf(format string, args ...interface{}) error {
	return BailError(fmt.Errorf(format, args...))
}

// FprintBailf writes a formatted string to the given writer and returns a BailError with the same message.
func FprintBailf(w io.Writer, msg string, args ...any) error {
	msg = fmt.Sprintf(msg, args...)
	fmt.Fprintln(w, msg)
	return BailError(errors.New(msg))
}

// IsBail returns true if any error in the given error's tree is a BailError.
func IsBail(err error) bool {
	if err == nil {
		return false
	}

	var bail *bailError
	ok := errors.As(err, &bail)
	return ok
}

// MergeBails accepts a set of errors and returns a single error that is the
// result of merging them according to the following criteria:
//
//   - If all the errors are nil, MergeBails returns nil.
//   - If any of the errors is *not* a BailError, MergeBails returns a single
//     error whose message is the concatenation of the messages of all the
//     errors which are not bails (that is, if any error is unexpected, MergeBails
//     will propagate it).
//   - In the remaining case that all errors are either nil or BailErrors, MergeBails
//     will return a single BailError whose message is the concatenation of the
//     messages of all the BailErrors.
func MergeBails(errs ...error) error {
	allNil := true
	joinableErrs := []error{}
	for _, err := range errs {
		if err == nil {
			continue
		}

		allNil = false
		if IsBail(err) {
			continue
		}

		joinableErrs = append(joinableErrs, err)
	}

	if allNil {
		return nil
	}

	if len(joinableErrs) == 0 {
		return BailError(errors.Join(errs...))
	}

	return errors.Join(joinableErrs...)
}
