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

// Exit codes for the Pulumi CLI.
// These codes are stable and documented for programmatic consumption.
// While new codes may be added, existing codes will not change meaning.
const (
	ExitSuccess             = 0   // Command completed successfully
	ExitCodeError           = 1   // General error occurred
	ExitConfigurationError  = 2   // Invalid configuration or flags
	ExitAuthenticationError = 3   // Authentication/authorization failure
	ExitResourceError       = 4   // Cloud resource operation failed
	ExitPolicyViolation     = 5   // Policy blocked the operation
	ExitStackNotFound       = 6   // Specified stack does not exist
	ExitNoChanges           = 7   // No changes found (with --expect-no-changes)
	ExitCancelled           = 8   // User cancelled the operation
	ExitTimeout             = 9   // Operation timed out
	ExitInternalError       = 255 // Internal/unexpected error (bug)
)

// ExitCodeInfo provides metadata about an exit code for documentation and JSON output.
type ExitCodeInfo struct {
	Code        int
	Name        string
	Description string
	Retryable   bool
}

// ExitCodes maps exit codes to their metadata.
var ExitCodes = map[int]ExitCodeInfo{
	ExitSuccess:             {ExitSuccess, "success", "Command completed successfully", false},
	ExitCodeError:           {ExitCodeError, "error", "General error occurred", true},
	ExitConfigurationError:  {ExitConfigurationError, "config_error", "Invalid configuration or flags", false},
	ExitAuthenticationError: {ExitAuthenticationError, "auth_error", "Authentication or authorization failure", false},
	ExitResourceError:       {ExitResourceError, "resource_error", "Cloud resource operation failed", true},
	ExitPolicyViolation:     {ExitPolicyViolation, "policy_violation", "Policy blocked the operation", false},
	ExitStackNotFound:       {ExitStackNotFound, "stack_not_found", "Specified stack does not exist", false},
	ExitNoChanges:           {ExitNoChanges, "no_changes", "No changes found", false},
	ExitCancelled:           {ExitCancelled, "cancelled", "User cancelled the operation", false},
	ExitTimeout:             {ExitTimeout, "timeout", "Operation timed out", true},
	ExitInternalError:       {ExitInternalError, "internal_error", "Internal or unexpected error", false},
}
