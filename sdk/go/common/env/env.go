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

// A small library for creating consistent and documented environmental variable accesses.
//
// Public environmental variables should be declared as a module level variable.

package env

import "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"

// Re-export some types and functions from the env library.

type Env = env.Env

type MapStore = env.MapStore

func NewEnv(s env.Store) env.Env { return env.NewEnv(s) }

// Global is the environment defined by environmental variables.
func Global() env.Env {
	return env.NewEnv(env.Global)
}

// That Pulumi is running in experimental mode.
//
// This is our standard gate for an existing feature that's not quite ready to be stable
// and publicly consumed.
var Experimental = env.Bool("EXPERIMENTAL", "Enable experimental options and commands.")

var SkipUpdateCheck = env.Bool("SKIP_UPDATE_CHECK", "Disable checking for a new version of pulumi.")

var Dev = env.Bool("DEV", "Enable features for hacking on pulumi itself.")

var SkipCheckpoints = env.Bool("SKIP_CHECKPOINTS", "Experimental flag to skip saving state "+
	"checkpoints and only save the final deployment. See #10668.", env.Needs(Experimental))

var DebugCommands = env.Bool("DEBUG_COMMANDS", "List commands helpful for debugging pulumi itself.")

var EnableLegacyDiff = env.Bool("ENABLE_LEGACY_DIFF", "")

var DisableProviderPreview = env.Bool("DISABLE_PROVIDER_PREVIEW", "")

var DisableResourceReferences = env.Bool("DISABLE_RESOURCE_REFERENCES", "")

var DisableOutputValues = env.Bool("DISABLE_OUTPUT_VALUES", "")

var ErrorOutputString = env.Bool("ERROR_OUTPUT_STRING", "Throw an error instead "+
	"of returning a string on attempting to convert an Output to a string")

var IgnoreAmbientPlugins = env.Bool("IGNORE_AMBIENT_PLUGINS",
	"Discover additional plugins by examining $PATH.")

var DisableAutomaticPluginAcquisition = env.Bool("DISABLE_AUTOMATIC_PLUGIN_ACQUISITION",
	"Disables the automatic installation of missing plugins.")

var SkipConfirmations = env.Bool("SKIP_CONFIRMATIONS",
	`Whether or not confirmation prompts should be skipped. This should be used by pass any requirement
that a --yes parameter has been set for non-interactive scenarios.

This should NOT be used to bypass protections for destructive operations, such as those that will
fail without a --force parameter.`)

var DebugGRPC = env.String("DEBUG_GRPC", `Enables debug tracing of Pulumi gRPC internals.
The variable should be set to the log file to which gRPC debug traces will be sent.`)

var GitSSHPassphrase = env.String("GITSSH_PASSPHRASE",
	"The passphrase to use with Git operations that use SSH.", env.Secret)

var ErrorOnDependencyCycles = env.Bool("ERROR_ON_DEPENDENCY_CYCLES",
	"Whether or not to error when dependency cycles are detected.")

// Environment variables that affect the self-managed backend.
var (
	SelfManagedStateNoLegacyWarning = env.Bool("SELF_MANAGED_STATE_NO_LEGACY_WARNING",
		"Disables the warning about legacy stack files mixed with project-scoped stack files.")

	SelfManagedStateLegacyLayout = env.Bool("SELF_MANAGED_STATE_LEGACY_LAYOUT",
		"Uses the legacy layout for new buckets, which currently default to project-scoped stacks.")

	SelfManagedGzip = env.Bool("SELF_MANAGED_STATE_GZIP",
		"Enables gzip compression when writing state files.")

	SelfManagedRetainCheckpoints = env.Bool("RETAIN_CHECKPOINTS",
		"If set every checkpoint will be duplicated to a timestamped file.")

	SelfManagedDisableCheckpointBackups = env.Bool("DISABLE_CHECKPOINT_BACKUPS",
		"If set checkpoint backups will not be written the to the backup folder.")
)

// Environment variables which affect Pulumi AI integrations
var (
	AIServiceEndpoint = env.String("AI_SERVICE_ENDPOINT", "Endpoint for Pulumi AI service")
)

var DisableValidation = env.Bool(
	"DISABLE_VALIDATION",
	`Disables format validation of system inputs.

Currently this disables validation of the following formats:
	- Stack names

This should only be used in cases where current data does not conform to the format and either cannot be migrated
without using the system itself, or show that the validation is too strict. Over time entries in the list above will be
removed and enforced to be validated.`)
