// Copyright 2016-2019, Pulumi Corporation.
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

package validation

import (
	"regexp"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
)

// validateStackName checks if s is a valid stack name, otherwise returns a descriptive error.
// This should match the stack naming rules enforced by the Pulumi Service.
func validateStackName(s string) error {
	stackNameRE := regexp.MustCompile("^[a-zA-Z0-9-_.]{1,100}$")
	if stackNameRE.MatchString(s) {
		return nil
	}
	return errors.New("a stack name may only contain alphanumeric, hyphens, underscores, or periods")
}

// ValidateStackTags validates the tag names and values.
func ValidateStackTags(tags map[apitype.StackTagName]string) error {
	const maxTagName = 40
	const maxTagValue = 256

	for t, v := range tags {
		if len(t) == 0 {
			return errors.Errorf("invalid stack tag %q", t)
		}
		if len(t) > maxTagName {
			return errors.Errorf("stack tag %q is too long (max length %d characters)", t, maxTagName)
		}
		if len(v) > maxTagValue {
			return errors.Errorf("stack tag %q value is too long (max length %d characters)", t, maxTagValue)
		}
	}

	return nil
}

// ValidateStackProperties validates the stack name and its tags to confirm they adhear to various
// naming and length restrictions.
func ValidateStackProperties(stack string, tags map[apitype.StackTagName]string) error {
	const maxStackName = 100 // Derived from the regex in validateStackName.
	if len(stack) > maxStackName {
		return errors.Errorf("stack name too long (max length %d characters)", maxStackName)
	}
	if err := validateStackName(stack); err != nil {
		return errors.Wrapf(err, "invalid stack name")
	}

	// Ensure tag values won't be rejected by the Pulumi Service. We do not validate that their
	// values make sense, e.g. ProjectRuntimeTag is a supported runtime.
	return ValidateStackTags(tags)
}
