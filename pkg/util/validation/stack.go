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

package validation

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// validateStackTagName checks if s is a valid stack tag name, otherwise returns a descriptive error.
// This should match the stack naming rules enforced by the Pulumi Service.
func validateStackTagName(s string) error {
	const maxTagName = 40

	if len(s) == 0 {
		return fmt.Errorf("invalid stack tag %q", s)
	}
	if len(s) > maxTagName {
		return fmt.Errorf("stack tag %q is too long (max length %d characters)", s, maxTagName)
	}

	tagNameRE := regexp.MustCompile("^[a-zA-Z0-9-_.:]{1,40}$")
	if tagNameRE.MatchString(s) {
		return nil
	}
	return errors.New("stack tag names may only contain alphanumerics, hyphens, underscores, periods, or colons")
}

// ValidateStackTags validates the tag names and values.
func ValidateStackTags(tags map[apitype.StackTagName]string) error {
	const maxTagValue = 256

	for t, v := range tags {
		if err := validateStackTagName(t); err != nil {
			return err
		}
		if len(v) > maxTagValue {
			return fmt.Errorf("stack tag %q value is too long (max length %d characters)", t, maxTagValue)
		}
	}

	return nil
}
