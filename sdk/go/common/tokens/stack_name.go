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

package tokens

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// StackName is a valid stack name. It should always be initialised via ParseStackName, the use of it's zero
// value will panic.
type StackName struct {
	str string
}

// IsEmpty returns true if the stack name is empty.
func (sn StackName) IsEmpty() bool {
	return sn.str == ""
}

// String implements fmt.Stringer. This method panics if StackName was zero initialized.
func (sn StackName) String() string {
	if !env.DisableValidation.Value() {
		contract.Assertf(sn.str != "", "stack name must not be empty")
	}
	return sn.str
}

// Q is a convenience method that returns the stack name as a QName. This method panics if StackName was zero
// initialized.
func (sn StackName) Q() QName {
	return QName(sn.String())
}

var stackNameRegex = regexp.MustCompile("^[A-Za-z0-9_.-]*")

// ParseStackName parses a stack name from a string.
func ParseStackName(s string) (StackName, error) {
	// Temporary flag to allow stack names validation to be disabled for the time being. Be sure to update the
	// DisableValidation help text when this is removed.
	if env.DisableValidation.Value() {
		return StackName{s}, nil
	}

	if s == "" {
		return StackName{}, errors.New("a stack name may not be empty")
	}
	if len(s) > 100 {
		return StackName{}, errors.New("a stack name cannot exceed 100 characters")
	}

	failure := -1
	if match := stackNameRegex.FindStringIndex(s); match == nil {
		// We have failed to find any match, so the first char must be invalid.
		failure = 0
	} else if match[1] != len(s) {
		// Our match did not extend to the end, so the invalid char must be the
		// first char not matched.
		failure = match[1]
	}

	if failure != -1 {
		return StackName{}, fmt.Errorf(
			"a stack name may only contain alphanumeric, hyphens, underscores, or periods: "+
				"invalid character %q at position %d", s[failure], failure)
	}

	return StackName{s}, nil
}

// MustParseStackName parses a stack name from a string.
func MustParseStackName(s string) StackName {
	n, err := ParseStackName(s)
	contract.AssertNoErrorf(err, "failed to parse stack name %q", s)
	return n
}
