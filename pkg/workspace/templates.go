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

package workspace

import (
	"errors"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

const (
	defaultProjectName = "project"
)

// ValidateProjectName ensures a project name is valid, if it is not it returns an error with a message suitable
// for display to an end user.
func ValidateProjectName(s string) error {
	if err := tokens.ValidateProjectName(s); err != nil {
		return err
	}

	// This is needed to stop cyclic imports in DotNet projects
	if strings.ToLower(s) == "pulumi" || strings.HasPrefix(strings.ToLower(s), "pulumi.") {
		return errors.New("project name must not be `Pulumi` and must not start with the prefix `Pulumi.` " +
			"to avoid collision with standard libraries")
	}

	return nil
}

// ValueOrSanitizedDefaultProjectName returns the value or a sanitized valid project name
// based on defaultNameToSanitize.
func ValueOrSanitizedDefaultProjectName(name string, projectName string, defaultNameToSanitize string) string {
	// If we have a name, use it.
	if name != "" {
		return name
	}

	// If the project already has a name that isn't a replacement string, use it.
	if projectName != "${PROJECT}" {
		return projectName
	}

	// Otherwise, get a sanitized version of `defaultNameToSanitize`.
	return getValidProjectName(defaultNameToSanitize)
}

// ValueOrDefaultProjectDescription returns the value or defaultDescription.
func ValueOrDefaultProjectDescription(
	description string, projectDescription string, defaultDescription string,
) string {
	// If we have a description, use it.
	if description != "" {
		return description
	}

	// If the project already has a description that isn't a replacement string, use it.
	if projectDescription != "${DESCRIPTION}" {
		return projectDescription
	}

	// Otherwise, use the default, which may be an empty string.
	return defaultDescription
}

// getValidProjectName returns a valid project name based on the passed-in name.
func getValidProjectName(name string) string {
	// If the name is valid, return it.
	if ValidateProjectName(name) == nil {
		return name
	}

	// Strip any invalid chars from the name.
	r := regexp.MustCompile("[^a-zA-Z0-9_.-]")
	name = r.ReplaceAllString(name, "")

	// See if the name is now valid
	if ValidateProjectName(name) == nil {
		return name
	}

	// If we couldn't come up with a valid project name, fallback to a default.
	return defaultProjectName
}

// ValidateProjectDescription ensures a project description name is valid, if it is not it returns an error with a
// message suitable for display to an end user.
func ValidateProjectDescription(s string) error {
	const maxTagValueLength = 256

	if len(s) > maxTagValueLength {
		return errors.New("a project description must be 256 characters or less")
	}

	return nil
}
