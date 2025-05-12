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

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

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

var AutoUpdateCLI = env.Bool("AUTO_UPDATE_CLI",
	"Enable automatic updates of the CLI. Ony works for dev versions of the CLI")

var Dev = env.Bool("DEV", "Enable features for hacking on pulumi itself.")

var SkipCheckpoints = env.Bool("SKIP_CHECKPOINTS", "Skip saving state checkpoints and only save "+
	"the final deployment. See #10668.")

var DebugCommands = env.Bool("DEBUG_COMMANDS", "List commands helpful for debugging pulumi itself.")

var EnableLegacyDiff = env.Bool("ENABLE_LEGACY_DIFF", "")

var EnableLegacyRefreshDiff = env.Bool("ENABLE_LEGACY_REFRESH_DIFF",
	"Use legacy refresh diff behaviour, in which only output changes are "+
		"reported and changes against the desired state are not calculated.")

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

var SkipVersionCheck = env.Bool("AUTOMATION_API_SKIP_VERSION_CHECK",
	"If set skip validating the version number reported by the CLI.")

var ContinueOnError = env.Bool("CONTINUE_ON_ERROR",
	"Continue to perform the update/destroy operation despite the occurrence of errors.")

var BackendURL = env.String("BACKEND_URL",
	"Set the backend that will be used instead of the currently logged in backend or the current project's backend.")

var SuppressCopilotLink = env.Bool("SUPPRESS_COPILOT_LINK",
	"Suppress showing the 'explainFailure' link to Copilot in the CLI output.")

var CopilotEnabled = env.Bool("COPILOT",
	"Enable Pulumi Copilot's assistance for improved CLI experience and insights.")

// TODO: This is a soft-release feature and will be removed after the feature flag is launched
// https://github.com/pulumi/pulumi/issues/19065
var CopilotSummaryModel = env.String("COPILOT_SUMMARY_MODEL",
	"The LLM model to use for the Copilot summary in diagnostics. Allowed values: 'gpt-4o-mini', 'gpt-4o'.")

// TODO: This is a soft-release feature and will be removed after the feature flag is launched
// https://github.com/pulumi/pulumi/issues/19065
var CopilotSummaryMaxLen = env.Int("COPILOT_SUMMARY_MAXLEN",
	"Max allowed length of Copilot summary in diagnostics. Allowed values are from 20 to 1920.")

var FallbackToStateSecretsManager = env.Bool("FALLBACK_TO_STATE_SECRETS_MANAGER",
	"Use the snapshot secrets manager as a fallback when the stack configuration is missing or incomplete.")

var Parallel = env.Int("PARALLEL",
	"Allow P resource operations to run in parallel at once (1 for no parallelism)")

var AccessToken = env.String("ACCESS_TOKEN",
	"The access token used to authenticate with the Pulumi Service.")

var DisableSecretCache = env.Bool("DISABLE_SECRET_CACHE",
	"Disable caching encryption operations for unchanged stack secrets.")

var ParallelDiff = env.Bool("PARALLEL_DIFF",
	"Enable running diff calculations in parallel.")

var RunProgram = env.Bool("RUN_PROGRAM",
	"Run the Pulumi program for refresh and destroy operations. This is the same as passing --run-program=true.")

// List of overrides for Plugin Download URLs. The expected format is `regexp=URL`, and multiple pairs can
// be specified separated by commas, e.g. `regexp1=URL1,regexp2=URL2`
//
// For example, when set to "^https://foo=https://bar,^github://=https://buzz", HTTPS plugin URLs that start with
// "foo" will use https://bar as the download URL and plugins hosted on github will use https://buzz
//
// Note that named regular expression groups can be used to capture parts of URLs and then reused for building
// redirects. For example
// ^github://api.github.com/(?P<org>[^/]+)/(?P<repo>[^/]+)=https://foo.com/downloads/${org}/${repo}
// will capture any GitHub-hosted plugin and redirect to its corresponding folder under https://foo.com/downloads
var PluginDownloadURLOverrides = env.String("PLUGIN_DOWNLOAD_URL_OVERRIDES", "")

// Environment variables that affect the DIY backend.
var (
	DIYBackendNoLegacyWarning = env.Bool("DIY_BACKEND_NO_LEGACY_WARNING",
		"Disables the warning about legacy stack files mixed with project-scoped stack files.",
		env.Alternative("SELF_MANAGED_STATE_NO_LEGACY_WARNING"))

	DIYBackendLegacyLayout = env.Bool("DIY_BACKEND_LEGACY_LAYOUT",
		"Uses the legacy layout for new buckets, which currently default to project-scoped stacks.",
		env.Alternative("SELF_MANAGED_STATE_LEGACY_LAYOUT"))

	DIYBackendGzip = env.Bool("DIY_BACKEND_GZIP",
		"Enables gzip compression when writing state files.",
		env.Alternative("SELF_MANAGED_STATE_GZIP"))

	DIYBackendRetainCheckpoints = env.Bool("DIY_BACKEND_RETAIN_CHECKPOINTS",
		"If set every checkpoint will be duplicated to a timestamped file.",
		env.Alternative("RETAIN_CHECKPOINTS"))

	DIYBackendDisableCheckpointBackups = env.Bool("DIY_BACKEND_DISABLE_CHECKPOINT_BACKUPS",
		"If set checkpoint backups will not be written the to the backup folder.",
		env.Alternative("DISABLE_CHECKPOINT_BACKUPS"))

	DIYBackendParallel = env.Int("DIY_BACKEND_PARALLEL",
		"Number of parallel operations when fetching stacks and resources from the DIY backend.")
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
