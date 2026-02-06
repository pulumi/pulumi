// Copyright 2026, Pulumi Corporation.
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

// Semantic exit codes for the Pulumi CLI. These allow callers (scripts, agents,
// CI systems) to distinguish between different error categories without parsing
// error messages.
const (
	ExitSuccess             = 0
	ExitCodeError           = 1 // Named to avoid conflict with ExitError func.
	ExitConfigurationError  = 2
	ExitAuthenticationError = 3
	ExitResourceError       = 4
	ExitPolicyViolation     = 5
	ExitStackNotFound       = 6
	ExitNoChanges           = 7
	ExitCancelled           = 8
	ExitTimeout             = 9
	ExitInternalError       = 255
)

// ExitCodeInfo provides metadata about a semantic exit code.
type ExitCodeInfo struct {
	Code        int
	Name        string
	Description string
	Retryable   bool
}

// ExitCodes maps exit code values to their metadata.
var ExitCodes = map[int]ExitCodeInfo{
	ExitSuccess:             {ExitSuccess, "success", "Command completed successfully", false},
	ExitCodeError:           {ExitCodeError, "error", "General error", true},
	ExitConfigurationError:  {ExitConfigurationError, "config_error", "Invalid configuration or flags", false},
	ExitAuthenticationError: {ExitAuthenticationError, "auth_error", "Authentication/authorization failure", false},
	ExitResourceError:       {ExitResourceError, "resource_error", "Cloud resource operation failed", true},
	ExitPolicyViolation:     {ExitPolicyViolation, "policy_violation", "Policy blocked the operation", false},
	ExitStackNotFound:       {ExitStackNotFound, "stack_not_found", "Specified stack does not exist", false},
	ExitNoChanges:           {ExitNoChanges, "no_changes", "No changes found (--expect-no-changes)", false},
	ExitCancelled:           {ExitCancelled, "cancelled", "User cancelled the operation", false},
	ExitTimeout:             {ExitTimeout, "timeout", "Operation timed out", true},
	ExitInternalError:       {ExitInternalError, "internal_error", "Internal/unexpected error (bug)", false},
}
