// Copyright 2019-2024, Pulumi Corporation.
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

package backend

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

// ConflictingUpdateError represents an error which occurred while starting an update/destroy operation.
// Another update of the same stack was in progress, so the operation got canceled due to this conflict.
type ConflictingUpdateError struct {
	Err error // The error that occurred while starting the operation.
}

func (c ConflictingUpdateError) Error() string {
	return fmt.Sprintf("%s\nTo learn more about possible reasons and resolution, visit "+
		"https://www.pulumi.com/docs/troubleshooting/#conflict", c.Err)
}

// MissingEnvVarForNonInteractiveError represents a situation where the CLI is run in
// non-interactive mode and that requires certain env vars to be set.
type MissingEnvVarForNonInteractiveError struct {
	Var env.Var
}

func (err MissingEnvVarForNonInteractiveError) Error() string {
	return err.Var.Name() + " must be set for login during non-interactive CLI sessions"
}
