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

import "errors"

// ValidateProjectName validates that the given string is a valid project name.
// The string must meet the following criteria:
//
//   - must be non-empty
//   - must be at most 100 characters
//   - must contain only alphanumeric characters,
//     hyphens, underscores, and periods (see [IsName])
//
// Returns a descriptive error if the string is not a valid project name.
func ValidateProjectName(s string) error {
	switch {
	case s == "":
		return errors.New("project names may not be empty")
	case len(s) > 100:
		return errors.New("project names are limited to 100 characters")
	case !IsName(s):
		return errors.New("project names may only contain alphanumerics, hyphens, underscores, and periods")
	}
	return nil
}
