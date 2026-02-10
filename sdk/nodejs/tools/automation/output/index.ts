// Copyright 2026-2026, Pulumi Corporation.
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

import { CommandResult, PulumiCommand } from "../../../automation/cmd";

export interface PulumiOptionsBase {
  command: PulumiCommand,
  cwd?: string,
  additionalEnv: { [key: string]: string },
  onOutput?: (output: string) => void,
  onError?: (error: string) => void,
  signal?: AbortSignal,
}

// Execute the given command and return the process output.
async function __run(options: PulumiOptionsBase, args: string[]): Promise<CommandResult> {
  return options.command.run(
    args,
    options.cwd ?? process.cwd(),
    options.additionalEnv,
    options.onOutput,
    options.onError,
    options.signal
  );
}

/** Options for the `pulumi ` command. */
export interface PulumiOptions extends PulumiOptionsBase {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Run pulumi as if it had been started in another directory */
    cwd?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Enable emojis in the output */
    emoji?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Disable interactive mode for all commands */
    nonInteractive?: boolean;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
}

/** Options for the `pulumi about` command. */
export interface PulumiAboutOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** The name of the stack to get info on. Defaults to the current stack */
    stack?: string;
    /** Include transitive dependencies */
    transitive?: boolean;
}

/** Options for the `pulumi about env` command. */
export interface PulumiAboutEnvOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi ai` command. */
export interface PulumiAiOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi ai web` command. */
export interface PulumiAiWebOptions extends PulumiOptionsBase {
    /** Language to use for the prompt - this defaults to TypeScript. [TypeScript, Python, Go, C#, Java, YAML] */
    language?: string;
    /** Opt-out of automatically submitting the prompt to Pulumi AI */
    noAutoSubmit?: boolean;
}

/** Options for the `pulumi cancel` command. */
export interface PulumiCancelOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts, and proceed with cancellation anyway */
    yes?: boolean;
}

/** Options for the `pulumi config` command. */
export interface PulumiConfigOptions extends PulumiOptionsBase {
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config cp` command. */
export interface PulumiConfigCpOptions extends PulumiOptionsBase {
    /** The name of the new stack to copy the config to */
    dest?: string;
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
}

/** Options for the `pulumi config env` command. */
export interface PulumiConfigEnvOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi config env add` command. */
export interface PulumiConfigEnvAddOptions extends PulumiOptionsBase {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config env init` command. */
export interface PulumiConfigEnvInitOptions extends PulumiOptionsBase {
    /** The name of the environment to create. Defaults to "<project name>/<stack name>" */
    env?: string;
    /** Do not remove configuration values from the stack after creating the environment */
    keepConfig?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save the created environment without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config env ls` command. */
export interface PulumiConfigEnvLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi config env rm` command. */
export interface PulumiConfigEnvRmOptions extends PulumiOptionsBase {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config get` command. */
export interface PulumiConfigGetOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration */
    open?: boolean;
    /** The key contains a path to a property in a map or list to get */
    path?: boolean;
}

/** Options for the `pulumi config refresh` command. */
export interface PulumiConfigRefreshOptions extends PulumiOptionsBase {
    /** Overwrite configuration file, if it exists, without creating a backup */
    force?: boolean;
}

/** Options for the `pulumi config rm` command. */
export interface PulumiConfigRmOptions extends PulumiOptionsBase {
    /** The key contains a path to a property in a map or list to remove */
    path?: boolean;
}

/** Options for the `pulumi config rm-all` command. */
export interface PulumiConfigRmAllOptions extends PulumiOptionsBase {
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
}

/** Options for the `pulumi config set` command. */
export interface PulumiConfigSetOptions extends PulumiOptionsBase {
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
    /** Save the value as plaintext (unencrypted) */
    plaintext?: boolean;
    /** Encrypt the value instead of storing it in plaintext */
    secret?: boolean;
    /** Save the value as the given type.  Allowed values are string, bool, int, and float */
    __type?: string;
}

/** Options for the `pulumi config set-all` command. */
export interface PulumiConfigSetAllOptions extends PulumiOptionsBase {
    /** Read values from a JSON string in the format produced by 'pulumi config --json' */
    json?: string;
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
    /** Marks a value as plaintext (unencrypted) */
    plaintext?: string[];
    /** Marks a value as secret to be encrypted */
    secret?: string[];
}

/** Options for the `pulumi console` command. */
export interface PulumiConsoleOptions extends PulumiOptionsBase {
    /** The name of the stack to view */
    stack?: string;
}

/** Options for the `pulumi convert` command. */
export interface PulumiConvertOptions extends PulumiOptionsBase {
    /** Which converter plugin to use to read the source program */
    from?: string;
    /** Generate the converted program(s) only; do not install dependencies */
    generateOnly?: boolean;
    /** Which language plugin to use to generate the Pulumi project */
    language?: string;
    /** Any mapping files to use in the conversion */
    mappings?: string[];
    /** The name to use for the converted project; defaults to the directory of the source project */
    name?: string;
    /** The output directory to write the converted project to */
    out?: string;
    /** Fail the conversion on errors such as missing variables */
    strict?: boolean;
}

/** Options for the `pulumi convert-trace` command. */
export interface PulumiConvertTraceOptions extends PulumiOptionsBase {
    /** the sample granularity */
    granularity?: string;
    /** true to ignore log spans */
    ignoreLogSpans?: boolean;
    /** true to export to OpenTelemetry */
    otel?: boolean;
}

/** Options for the `pulumi deployment` command. */
export interface PulumiDeploymentOptions extends PulumiOptionsBase {
    /** Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml */
    configFile?: string;
}

/** Options for the `pulumi deployment run` command. */
export interface PulumiDeploymentRunOptions extends PulumiOptionsBase {
    /** The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue will be used. */
    agentPoolId?: string;
    /** Environment variables to use in the remote operation of the form NAME=value (e.g. `--env FOO=bar`) */
    env?: string[];
    /** Environment variables with secret values to use in the remote operation of the form NAME=secretvalue (e.g. `--env FOO=secret`) */
    envSecret?: string[];
    /** The Docker image to use for the executor */
    executorImage?: string;
    /** The password for the credentials with access to the Docker image to use for the executor */
    executorImagePassword?: string;
    /** The username for the credentials with access to the Docker image to use for the executor */
    executorImageUsername?: string;
    /** Git personal access token */
    gitAuthAccessToken?: string;
    /** Git password; for use with username or with an SSH private key */
    gitAuthPassword?: string;
    /** Git SSH private key; use --git-auth-password for the password, if needed */
    gitAuthSshPrivateKey?: string;
    /** Git SSH private key path; use --git-auth-password for the password, if needed */
    gitAuthSshPrivateKeyPath?: string;
    /** Git username */
    gitAuthUsername?: string;
    /** Git branch to deploy; this is mutually exclusive with --git-commit; either value needs to be specified */
    gitBranch?: string;
    /** Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); this is mutually exclusive with --git-branch; either value needs to be specified */
    gitCommit?: string;
    /** The directory to work from in the project's source repository where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root */
    gitRepoDir?: string;
    /** Inherit deployment settings from the current stack */
    inheritSettings?: boolean;
    /** Commands to run before the remote operation */
    preRunCommand?: string[];
    /** Whether to skip the default dependency installation step */
    skipInstallDependencies?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of the state permalink */
    suppressPermalink?: boolean;
    /** Suppress log streaming of the deployment job */
    suppressStreamLogs?: boolean;
}

/** Options for the `pulumi deployment settings` command. */
export interface PulumiDeploymentSettingsOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi deployment settings configure` command. */
export interface PulumiDeploymentSettingsConfigureOptions extends PulumiOptionsBase {
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings destroy` command. */
export interface PulumiDeploymentSettingsDestroyOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Options for the `pulumi deployment settings env` command. */
export interface PulumiDeploymentSettingsEnvOptions extends PulumiOptionsBase {
    /** whether the key should be removed */
    remove?: boolean;
    /** whether the value should be treated as a secret and be encrypted */
    secret?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings init` command. */
export interface PulumiDeploymentSettingsInitOptions extends PulumiOptionsBase {
    /** Forces content to be generated even if it is already configured */
    force?: boolean;
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Git SSH private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings pull` command. */
export interface PulumiDeploymentSettingsPullOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings push` command. */
export interface PulumiDeploymentSettingsPushOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Options for the `pulumi destroy` command. */
export interface PulumiDestroyOptions extends PulumiOptionsBase {
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the destroy and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Continue to perform the destroy operation despite the occurrence of errors (can also be set with PULUMI_CONTINUE_ON_ERROR env var) */
    continueOnError?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Do not destroy protected resources. Destroy all other resources. */
    excludeProtected?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Serialize the destroy diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the destroy operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Only show a preview of the destroy, but don't perform the destroy itself */
    previewOnly?: boolean;
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Remove the stack and its config file after all resources in the stack have been deleted */
    remove?: boolean;
    /** Run the program to determine up-to-date state for providers to destroy resources */
    runProgram?: boolean;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need to be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Do not calculate a preview before performing the destroy */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed. Multiple resources can be specified using: --target urn1 --target urn2. Wildcards (*, **) are also supported */
    target?: string[];
    /** Allows destroying of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Automatically approve and perform the destroy after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi env` command. */
export interface PulumiEnvOptions extends PulumiOptionsBase {
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env clone` command. */
export interface PulumiEnvCloneOptions extends PulumiOptionsBase {
    /** preserve the same team access on the environment being cloned */
    preserveAccess?: boolean;
    /** preserve any tags on the environment being cloned */
    preserveEnvTags?: boolean;
    /** preserve history of the environment being cloned */
    preserveHistory?: boolean;
    /** preserve any tags on the environment revisions being cloned */
    preserveRevTags?: boolean;
}

/** Options for the `pulumi env diff` command. */
export interface PulumiEnvDiffOptions extends PulumiOptionsBase {
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell' */
    format?: string;
    /** Show the diff for a specific path */
    path?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/** Options for the `pulumi env edit` command. */
export interface PulumiEnvEditOptions extends PulumiOptionsBase {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** the command to use to edit the environment definition */
    editor?: string;
    /** the file that contains the updated environment, if any. Pass `-` to read from standard input. */
    file?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/** Options for the `pulumi env get` command. */
export interface PulumiEnvGetOptions extends PulumiOptionsBase {
    /** Set to print just the definition. */
    definition?: boolean;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
    /** Set to print just the value in the given format. May be 'dotenv', 'json', 'detailed', 'shell' or 'string' */
    value?: string;
}

/** Options for the `pulumi env init` command. */
export interface PulumiEnvInitOptions extends PulumiOptionsBase {
    /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
    file?: string;
}

/** Options for the `pulumi env ls` command. */
export interface PulumiEnvLsOptions extends PulumiOptionsBase {
    /** Filter returned environments to those in a specific organization */
    organization?: string;
    /** Filter returned environments to those in a specific project */
    project?: string;
}

/** Options for the `pulumi env open` command. */
export interface PulumiEnvOpenOptions extends PulumiOptionsBase {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', 'shell' or 'string' */
    format?: string;
    /** the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m) */
    lifetime?: string;
}

/** Options for the `pulumi env rm` command. */
export interface PulumiEnvRmOptions extends PulumiOptionsBase {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi env rotate` command. */
export interface PulumiEnvRotateOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi env run` command. */
export interface PulumiEnvRunOptions extends PulumiOptionsBase {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** true to treat the command as interactive and disable output filters */
    interactive?: boolean;
    /** the lifetime of the opened environment */
    lifetime?: string;
}

/** Options for the `pulumi env set` command. */
export interface PulumiEnvSetOptions extends PulumiOptionsBase {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** If set, the value is read from the specified file. Pass `-` to read from standard input. */
    file?: string;
    /** true to leave the value in plaintext */
    plaintext?: boolean;
    /** true to mark the value as secret */
    secret?: boolean;
    /** true to treat the value as a string rather than attempting to parse it as YAML */
    string?: boolean;
}

/** Options for the `pulumi env tag` command. */
export interface PulumiEnvTagOptions extends PulumiOptionsBase {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag get` command. */
export interface PulumiEnvTagGetOptions extends PulumiOptionsBase {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag ls` command. */
export interface PulumiEnvTagLsOptions extends PulumiOptionsBase {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag mv` command. */
export interface PulumiEnvTagMvOptions extends PulumiOptionsBase {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag rm` command. */
export interface PulumiEnvTagRmOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi env version` command. */
export interface PulumiEnvVersionOptions extends PulumiOptionsBase {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version history` command. */
export interface PulumiEnvVersionHistoryOptions extends PulumiOptionsBase {
    /** the command to use to page through the environment's revisions */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version retract` command. */
export interface PulumiEnvVersionRetractOptions extends PulumiOptionsBase {
    /** the reason for the retraction */
    reason?: string;
    /** the version to use to replace the retracted revision */
    replaceWith?: string;
}

/** Options for the `pulumi env version rollback` command. */
export interface PulumiEnvVersionRollbackOptions extends PulumiOptionsBase {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
}

/** Options for the `pulumi env version tag` command. */
export interface PulumiEnvVersionTagOptions extends PulumiOptionsBase {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag ls` command. */
export interface PulumiEnvVersionTagLsOptions extends PulumiOptionsBase {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag rm` command. */
export interface PulumiEnvVersionTagRmOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi gen-completion` command. */
export interface PulumiGenCompletionOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi gen-markdown` command. */
export interface PulumiGenMarkdownOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi generate-cli-spec` command. */
export interface PulumiGenerateCliSpecOptions extends PulumiOptionsBase {
    /** help for generate-cli-spec */
    help?: boolean;
}

/** Options for the `pulumi help` command. */
export interface PulumiHelpOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi import` command. */
export interface PulumiImportOptions extends PulumiOptionsBase {
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    execAgent?: string;
    execKind?: string;
    /** The path to a JSON-encoded file containing a list of resources to import */
    file?: string;
    /** Invoke a converter to import the resources */
    from?: string;
    /** Generate resource declaration code for the imported resources */
    generateCode?: boolean;
    /** When used with --from, always write a JSON-encoded file containing a list of importable resources discovered by conversion to the specified path */
    generateResources?: string;
    /** Serialize the import diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** The path to the file that will contain the generated resource declarations */
    out?: string;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** The name and URN of the parent resource in the format name=urn, where name is the variable name of the parent resource */
    parent?: string;
    /** Only show a preview of the import, but don't perform the import itself */
    previewOnly?: boolean;
    /** The property names to use for the import in the format name1,name2 */
    properties?: string[];
    /** Allow resources to be imported with protection from deletion enabled */
    protect?: boolean;
    /** The name and URN of the provider to use for the import in the format name=urn, where name is the variable name for the provider resource */
    provider?: string;
    /** Do not calculate a preview before performing the import */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Automatically approve and perform the import after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi install` command. */
export interface PulumiInstallOptions extends PulumiOptionsBase {
    /** Skip installing dependencies */
    noDependencies?: boolean;
    /** Skip installing plugins */
    noPlugins?: boolean;
    /** The max number of concurrent installs to perform. Parallelism of less then 1 implies unbounded parallelism */
    parallel?: number;
    /** Reinstall a plugin even if it already exists */
    reinstall?: boolean;
    /** Use language version tools to setup and install the language runtime */
    useLanguageVersionTools?: boolean;
}

/** Options for the `pulumi login` command. */
export interface PulumiLoginOptions extends PulumiOptionsBase {
    /** A cloud URL to log in to */
    cloudUrl?: string;
    /** A default org to associate with the login. Please note, currently, only the managed and self-hosted backends support organizations */
    defaultOrg?: string;
    /** Allow insecure server connections when using SSL */
    insecure?: boolean;
    /** Show interactive login options based on known accounts */
    interactive?: boolean;
    /** Use Pulumi in local-only mode */
    local?: boolean;
    /** The expiration for the cloud backend access token in duration format (e.g. '15m', '24h') */
    oidcExpiration?: string;
    /** The organization to use for OIDC token exchange audience */
    oidcOrg?: string;
    /** The team when exchanging for a team token */
    oidcTeam?: string;
    /** An OIDC token to exchange for a cloud backend access token. Can be either a raw token or a file path prefixed with 'file://'. */
    oidcToken?: string;
    /** The user when exchanging for a personal token */
    oidcUser?: string;
}

/** Options for the `pulumi logout` command. */
export interface PulumiLogoutOptions extends PulumiOptionsBase {
    /** Logout of all backends */
    all?: boolean;
    /** A cloud URL to log out of (defaults to current cloud) */
    cloudUrl?: string;
    /** Log out of using local mode */
    local?: boolean;
}

/** Options for the `pulumi logs` command. */
export interface PulumiLogsOptions extends PulumiOptionsBase {
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Follow the log stream in real time (like tail -f) */
    follow?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Only return logs for the requested resource ('name', 'type::name' or full URN).  Defaults to returning all logs. */
    resource?: string;
    /** Only return logs newer than a relative duration ('5s', '2m', '3h') or absolute timestamp.  Defaults to returning the last 1 hour of logs. */
    since?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi new` command. */
export interface PulumiNewOptions extends PulumiOptionsBase {
    /** Prompt to use for Pulumi AI */
    ai?: string;
    /** Config to save */
    config?: string[];
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** The project description; if not specified, a prompt will request it */
    description?: string;
    /** The location to place the generated project; if not specified, the current directory is used */
    dir?: string;
    /** Forces content to be generated even if it would change existing files */
    force?: boolean;
    /** Generate the project only; do not create a stack, save config, or install dependencies */
    generateOnly?: boolean;
    /** Language to use for Pulumi AI (must be one of TypeScript, JavaScript, Python, Go, C#, Java, or YAML) */
    language?: string;
    /** List locally installed templates and exit */
    listTemplates?: boolean;
    /** The project name; if not specified, a prompt will request it */
    name?: string;
    /** Use locally cached templates without making any network requests */
    offline?: boolean;
    /** Store stack configuration remotely */
    remoteStackConfig?: boolean;
    /** Additional options for the language runtime (format: key1=value1,key2=value2) */
    runtimeOptions?: string[];
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
    secretsProvider?: string;
    /** The stack name; either an existing stack or stack to create; if not specified, a prompt will request it */
    stack?: string;
    /** Run in template mode, which will skip prompting for AI or Template functionality */
    templateMode?: boolean;
    /** Skip prompts and proceed with default values */
    yes?: boolean;
}

/** Options for the `pulumi org` command. */
export interface PulumiOrgOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi org get-default` command. */
export interface PulumiOrgGetDefaultOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi org search` command. */
export interface PulumiOrgSearchOptions extends PulumiOptionsBase {
    /** Delimiter to use when rendering CSV output. */
    delimiter?: string;
    /** Name of the organization to search. Defaults to the current user's default organization. */
    org?: string;
    /** Output format. Supported formats are 'table', 'json', 'csv', and 'yaml'. */
    output?: string;
    /**
     * A Pulumi Query to send to Pulumi Cloud for resource search.May be formatted as a single query, or multiple:
     * 	-q "type:aws:s3/bucketv2:BucketV2 modified:>=2023-09-01"
     * 	-q "type:aws:s3/bucketv2:BucketV2" -q "modified:>=2023-09-01"
     * See https://www.pulumi.com/docs/pulumi-cloud/insights/search/#query-syntax for syntax reference.
     */
    query?: string[];
    /** Open the search results in a web browser. */
    web?: boolean;
}

/** Options for the `pulumi org search ai` command. */
export interface PulumiOrgSearchAiOptions extends PulumiOptionsBase {
    /** Delimiter to use when rendering CSV output. */
    delimiter?: string;
    /** Organization name to search within */
    org?: string;
    /** Output format. Supported formats are 'table', 'json', 'csv' and 'yaml'. */
    output?: string;
    /** Plaintext natural language query */
    query?: string;
    /** Open the search results in a web browser. */
    web?: boolean;
}

/** Options for the `pulumi org set-default` command. */
export interface PulumiOrgSetDefaultOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi package` command. */
export interface PulumiPackageOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi package add` command. */
export interface PulumiPackageAddOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi package delete` command. */
export interface PulumiPackageDeleteOptions extends PulumiOptionsBase {
    /** Skip confirmation prompts, and proceed with deletion anyway */
    yes?: boolean;
}

/** Options for the `pulumi package gen-sdk` command. */
export interface PulumiPackageGenSdkOptions extends PulumiOptionsBase {
    /** The SDK language to generate: [nodejs|python|go|dotnet|java|all] */
    language?: string;
    /** Generate an SDK appropriate for local usage */
    local?: boolean;
    /** The directory to write the SDK to */
    out?: string;
    /** A folder of extra overlay files to copy to the generated SDK */
    overlays?: string;
    /** The provider plugin version to generate the SDK for */
    version?: string;
}

/** Options for the `pulumi package get-mapping` command. */
export interface PulumiPackageGetMappingOptions extends PulumiOptionsBase {
    /** The file to write the mapping data to */
    out?: string;
}

/** Options for the `pulumi package get-schema` command. */
export interface PulumiPackageGetSchemaOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi package info` command. */
export interface PulumiPackageInfoOptions extends PulumiOptionsBase {
    /** Function name */
    function?: string;
    /** Module name */
    module?: string;
    /** Resource name */
    resource?: string;
}

/** Options for the `pulumi package pack-sdk` command. */
export interface PulumiPackagePackSdkOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi package publish` command. */
export interface PulumiPackagePublishOptions extends PulumiOptionsBase {
    /** Path to the installation configuration markdown file */
    installationConfiguration?: string;
    /** The publisher of the package (e.g., 'pulumi'). Defaults to the publisher set in the package schema or the default organization in your pulumi config. */
    publisher?: string;
    /** Path to the package readme/index markdown file */
    readme?: string;
    /** The origin of the package (e.g., 'pulumi', 'private', 'opentofu'). Defaults to 'private'. */
    source?: string;
}

/** Options for the `pulumi package publish-sdk` command. */
export interface PulumiPackagePublishSdkOptions extends PulumiOptionsBase {
    /**
     * The path to the root of your package.
     * 	Example: ./sdk/nodejs
     * 	
     */
    path?: string;
}

/** Options for the `pulumi plugin` command. */
export interface PulumiPluginOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi plugin install` command. */
export interface PulumiPluginInstallOptions extends PulumiOptionsBase {
    /** The expected SHA256 checksum for the plugin archive */
    checksum?: string;
    /** Force installation of an exact version match (usually >= is accepted) */
    exact?: boolean;
    /** Install a plugin from a binary, folder or tarball, instead of downloading it */
    file?: string;
    /** Reinstall a plugin even if it already exists */
    reinstall?: boolean;
    /** A URL to download plugins from */
    server?: string;
}

/** Options for the `pulumi plugin ls` command. */
export interface PulumiPluginLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** List only the plugins used by the current project */
    project?: boolean;
}

/** Options for the `pulumi plugin rm` command. */
export interface PulumiPluginRmOptions extends PulumiOptionsBase {
    /** Remove all plugins */
    all?: boolean;
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi plugin run` command. */
export interface PulumiPluginRunOptions extends PulumiOptionsBase {
    /** The plugin kind */
    kind?: string;
}

/** Options for the `pulumi policy` command. */
export interface PulumiPolicyOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi policy disable` command. */
export interface PulumiPolicyDisableOptions extends PulumiOptionsBase {
    /** The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
    /** The version of the Policy Pack that will be disabled; if not specified, any enabled version of the Policy Pack will be disabled */
    version?: string;
}

/** Options for the `pulumi policy enable` command. */
export interface PulumiPolicyEnableOptions extends PulumiOptionsBase {
    /** The file path for the Policy Pack configuration file */
    config?: string;
    /** The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
}

/** Options for the `pulumi policy group` command. */
export interface PulumiPolicyGroupOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi policy group ls` command. */
export interface PulumiPolicyGroupLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi policy install` command. */
export interface PulumiPolicyInstallOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi policy ls` command. */
export interface PulumiPolicyLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi policy new` command. */
export interface PulumiPolicyNewOptions extends PulumiOptionsBase {
    /** The location to place the generated Policy Pack; if not specified, the current directory is used */
    dir?: string;
    /** Forces content to be generated even if it would change existing files */
    force?: boolean;
    /** Generate the Policy Pack only; do not install dependencies */
    generateOnly?: boolean;
    /** Use locally cached templates without making any network requests */
    offline?: boolean;
}

/** Options for the `pulumi policy publish` command. */
export interface PulumiPolicyPublishOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi policy rm` command. */
export interface PulumiPolicyRmOptions extends PulumiOptionsBase {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi policy validate-config` command. */
export interface PulumiPolicyValidateConfigOptions extends PulumiOptionsBase {
    /** The file path for the Policy Pack configuration file */
    config?: string;
}

/** Options for the `pulumi preview` command. */
export interface PulumiPreviewOptions extends PulumiOptionsBase {
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string[];
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the preview and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allow ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes are proposed by this preview */
    expectNoChanges?: boolean;
    /** Save any creates seen during the preview into an import file to use with 'pulumi import' */
    importFile?: string;
    /** Serialize the preview diffs, operations, and overall output as JSON. Set PULUMI_ENABLE_STREAMING_JSON_PREVIEW to stream JSON events instead. */
    json?: boolean;
    /** Optional message to associate with the preview operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Run one or more policy packs as part of this update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2 */
    replace?: string[];
    /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
    runProgram?: boolean;
    /** [PREVIEW] Save the operations proposed by the preview to a plan file at the given path */
    savePlan?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show per-resource policy remediation details instead of a summary */
    showPolicyRemediations?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Show secrets in plaintext in the CLI output, if used with --save-plan the secrets will also be shown in the plan file. Defaults to `false` */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2 */
    target?: string[];
    /** Allow updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string[];
}

/** Options for the `pulumi project` command. */
export interface PulumiProjectOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi project ls` command. */
export interface PulumiProjectLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** The organization whose projects to list */
    organization?: string;
}

/** Options for the `pulumi refresh` command. */
export interface PulumiRefreshOptions extends PulumiOptionsBase {
    /** Clear all pending creates, dropping them from the state */
    clearPendingCreates?: boolean;
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the refresh and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be refreshed. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes occur during this refresh. This check happens after the refresh is applied */
    expectNoChanges?: boolean;
    /** A list of form [[URN ID]...] describing the provider IDs of pending creates */
    importPendingCreates?: string[];
    /** Serialize the refresh diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Only show a preview of the refresh, but don't perform the refresh itself */
    previewOnly?: boolean;
    /** Run the program to determine up-to-date state for providers to refresh resources */
    runProgram?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Skip importing pending creates in interactive mode */
    skipPendingCreates?: boolean;
    /** Do not calculate a preview before performing the refresh */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to refresh. Multiple resource can be specified using: --target urn1 --target urn2 */
    target?: string[];
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Automatically approve and perform the refresh after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi replay-events` command. */
export interface PulumiReplayEventsOptions extends PulumiOptionsBase {
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Delay display by the given duration. Useful for attaching a debugger. */
    delay?: string;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Serialize the preview diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Delay each event by the given duration. */
    period?: string;
    /** Must be set for events from a `pulumi preview`. */
    preview?: boolean;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
}

/** Options for the `pulumi schema` command. */
export interface PulumiSchemaOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi schema check` command. */
export interface PulumiSchemaCheckOptions extends PulumiOptionsBase {
    /** Whether references to nonexistent types should be considered errors */
    allowDanglingReferences?: boolean;
}

/** Options for the `pulumi stack` command. */
export interface PulumiStackOptions extends PulumiOptionsBase {
    /** Display each resource's provider-assigned unique ID */
    showIds?: boolean;
    /** Display only the stack name */
    showName?: boolean;
    /** Display stack outputs which are marked as secret in plaintext */
    showSecrets?: boolean;
    /** Display each resource's Pulumi-assigned globally unique URN */
    showUrns?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack change-secrets-provider` command. */
export interface PulumiStackChangeSecretsProviderOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack export` command. */
export interface PulumiStackExportOptions extends PulumiOptionsBase {
    /** A filename to write stack output to */
    file?: string;
    /** Emit secrets in plaintext in exported stack. Defaults to `false` */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Previous stack version to export. (If unset, will export the latest.) */
    version?: string;
}

/** Options for the `pulumi stack graph` command. */
export interface PulumiStackGraphOptions extends PulumiOptionsBase {
    /** Sets the color of dependency edges in the graph */
    dependencyEdgeColor?: string;
    /** An optional DOT fragment that will be inserted at the top of the digraph element. This can be used for styling the graph elements, setting graph properties etc. */
    dotFragment?: string;
    /** Ignores edges introduced by dependency resource relationships */
    ignoreDependencyEdges?: boolean;
    /** Ignores edges introduced by parent/child resource relationships */
    ignoreParentEdges?: boolean;
    /** Sets the color of parent edges in the graph */
    parentEdgeColor?: string;
    /** Sets the resource name as the node label for each node of the graph */
    shortNodeName?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack history` command. */
export interface PulumiStackHistoryOptions extends PulumiOptionsBase {
    /** Show full dates, instead of relative dates */
    fullDates?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Used with 'page-size' to paginate results */
    page?: number;
    /** Used with 'page' to control number of results returned */
    pageSize?: number;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** Choose a stack other than the currently selected one */
    stack?: string;
}

/** Options for the `pulumi stack import` command. */
export interface PulumiStackImportOptions extends PulumiOptionsBase {
    /** A filename to read stack input from */
    file?: string;
    /** Force the import to occur, even if apparent errors are discovered beforehand (not recommended) */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack init` command. */
export interface PulumiStackInitOptions extends PulumiOptionsBase {
    /** The name of the stack to copy existing config from */
    copyConfigFrom?: string;
    /** Do not select the stack */
    noSelect?: boolean;
    /** Store stack configuration remotely */
    remoteConfig?: boolean;
    /**
     * The type of the provider that should be used to encrypt and decrypt secrets
     * (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)
     */
    secretsProvider?: string;
    /** The name of the stack to create */
    stack?: string;
    /** A list of team names that should have permission to read and update this stack, once created */
    teams?: string[];
}

/** Options for the `pulumi stack ls` command. */
export interface PulumiStackLsOptions extends PulumiOptionsBase {
    /** List all stacks instead of just stacks for the current project */
    all?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Filter returned stacks to those in a specific organization */
    organization?: string;
    /** Filter returned stacks to those with a specific project name */
    project?: string;
    /** Filter returned stacks to those in a specific tag (tag-name or tag-name=tag-value) */
    tag?: string;
}

/** Options for the `pulumi stack output` command. */
export interface PulumiStackOutputOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** Emit output as a shell script */
    shell?: boolean;
    /** Display outputs which are marked as secret in plaintext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack rename` command. */
export interface PulumiStackRenameOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack rm` command. */
export interface PulumiStackRmOptions extends PulumiOptionsBase {
    /** Forces deletion of the stack, leaving behind any resources managed by the stack */
    force?: boolean;
    /** Do not delete the corresponding Pulumi.<stack-name>.yaml configuration file for the stack */
    preserveConfig?: boolean;
    /** Additionally remove backups of the stack, if using the DIY backend */
    removeBackups?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi stack select` command. */
export interface PulumiStackSelectOptions extends PulumiOptionsBase {
    /** If selected stack does not exist, create it */
    create?: boolean;
    /**
     * Use with --create flag, The type of the provider that should be used to encrypt and decrypt secrets
     * (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)
     */
    secretsProvider?: string;
    /** The name of the stack to select */
    stack?: string;
}

/** Options for the `pulumi stack tag` command. */
export interface PulumiStackTagOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack tag get` command. */
export interface PulumiStackTagGetOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi stack tag ls` command. */
export interface PulumiStackTagLsOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi stack tag rm` command. */
export interface PulumiStackTagRmOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi stack tag set` command. */
export interface PulumiStackTagSetOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi stack unselect` command. */
export interface PulumiStackUnselectOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi state` command. */
export interface PulumiStateOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi state delete` command. */
export interface PulumiStateDeleteOptions extends PulumiOptionsBase {
    /** Delete all resources in the stack */
    all?: boolean;
    /** Force deletion of protected resources */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Delete the URN and all its dependents */
    targetDependents?: boolean;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state edit` command. */
export interface PulumiStateEditOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state move` command. */
export interface PulumiStateMoveOptions extends PulumiOptionsBase {
    /** The name of the stack to move resources to */
    dest?: string;
    /** Include all the parents of the moved resources as well */
    includeParents?: boolean;
    /** The name of the stack to move resources from */
    source?: string;
    /** Automatically approve and perform the move */
    yes?: boolean;
}

/** Options for the `pulumi state protect` command. */
export interface PulumiStateProtectOptions extends PulumiOptionsBase {
    /** Protect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state rename` command. */
export interface PulumiStateRenameOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state repair` command. */
export interface PulumiStateRepairOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically approve and perform the repair */
    yes?: boolean;
}

/** Options for the `pulumi state taint` command. */
export interface PulumiStateTaintOptions extends PulumiOptionsBase {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state unprotect` command. */
export interface PulumiStateUnprotectOptions extends PulumiOptionsBase {
    /** Unprotect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state untaint` command. */
export interface PulumiStateUntaintOptions extends PulumiOptionsBase {
    /** Untaint all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state upgrade` command. */
export interface PulumiStateUpgradeOptions extends PulumiOptionsBase {
    /** Automatically approve and perform the upgrade */
    yes?: boolean;
}

/** Options for the `pulumi template` command. */
export interface PulumiTemplateOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi template publish` command. */
export interface PulumiTemplatePublishOptions extends PulumiOptionsBase {
    /** The name of the template (required) */
    name?: string;
    /** The publisher of the template (e.g., 'pulumi'). Defaults to the default organization in your pulumi config. */
    publisher?: string;
    /** The version of the template (required, semver format) */
    version?: string;
}

/** Options for the `pulumi up` command. */
export interface PulumiUpOptions extends PulumiOptionsBase {
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string[];
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the update and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Continue updating resources even if an error is encountered (can also be set with PULUMI_CONTINUE_ON_ERROR environment variable) */
    continueOnError?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes occur during this update. This check happens after the update is applied */
    expectNoChanges?: boolean;
    /** Serialize the update diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** [EXPERIMENTAL] Path to a plan file to use for the update. The update will not perform operations that exceed its plan (e.g. replacements instead of updates, or updates insteadof sames). */
    plan?: string;
    /** Run one or more policy packs as part of this update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2. Wildcards (*, **) are also supported */
    replace?: string[];
    /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
    runProgram?: boolean;
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
    secretsProvider?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show per-resource policy remediation details instead of a summary */
    showPolicyRemediations?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Show secret outputs in the CLI output */
    showSecrets?: boolean;
    /** Do not calculate a preview before performing the update */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** [EXPERIMENTAL] Enable strict plan behavior: generate a plan during preview and constrain the update to that plan (opt-in). Cannot be used with --skip-preview. */
    strict?: boolean;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2. Wildcards (*, **) are also supported */
    target?: string[];
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string[];
    /** Automatically approve and perform the update after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi version` command. */
export interface PulumiVersionOptions extends PulumiOptionsBase {
}

/** Options for the `pulumi view-trace` command. */
export interface PulumiViewTraceOptions extends PulumiOptionsBase {
    /** the port the trace viewer will listen on */
    port?: number;
}

/** Options for the `pulumi watch` command. */
export interface PulumiWatchOptions extends PulumiOptionsBase {
    /** Config to use during the update */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    execKind?: string;
    /** Optional message to associate with each update operation */
    message?: string;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Specify one or more relative or absolute paths that need to be watched. A path can point to a folder or a file. Defaults to working directory */
    path?: string[];
    /** Run one or more policy packs as part of each update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before each update */
    refresh?: boolean;
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
    secretsProvider?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi whoami` command. */
export interface PulumiWhoamiOptions extends PulumiOptionsBase {
    /** Emit output as JSON */
    json?: boolean;
    /** Print detailed whoami information */
    verbose?: boolean;
}

export function aboutEnv(__options: PulumiAboutEnvOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['about', 'env', ...__arguments]);
}

export function about(__options: PulumiAboutOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.transitive != null) {
        if (__options.transitive) {
            __arguments.push('--transitive');
        }
    }

    return __run(__options, ['about', ...__arguments]);
}

export function aiWeb(__options: PulumiAiWebOptions, prompt?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (prompt != null) {
        __arguments.push('' + prompt);
    }

    if (__options.language != null) {
        __arguments.push('--language', '' + __options.language);
    }

    if (__options.noAutoSubmit != null) {
        if (__options.noAutoSubmit) {
            __arguments.push('--no-auto-submit');
        }
    }

    return __run(__options, ['ai', 'web', ...__arguments]);
}

export function ai(__options: PulumiAiOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['ai', ...__arguments]);
}

export function cancel(__options: PulumiCancelOptions, stackName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (stackName != null) {
        __arguments.push('' + stackName);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['cancel', ...__arguments]);
}

export function configCp(__options: PulumiConfigCpOptions, key?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (key != null) {
        __arguments.push('' + key);
    }

    if (__options.dest != null) {
        __arguments.push('--dest', '' + __options.dest);
    }

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    return __run(__options, ['config', 'cp', ...__arguments]);
}

export function configEnvAdd(__options: PulumiConfigEnvAddOptions, ...environmentName: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + environmentName);

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['config', 'env', 'add', ...__arguments]);
}

export function configEnvInit(__options: PulumiConfigEnvInitOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.env != null) {
        __arguments.push('--env', '' + __options.env);
    }

    if (__options.keepConfig != null) {
        if (__options.keepConfig) {
            __arguments.push('--keep-config');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['config', 'env', 'init', ...__arguments]);
}

export function configEnvLs(__options: PulumiConfigEnvLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    return __run(__options, ['config', 'env', 'ls', ...__arguments]);
}

export function configEnvRm(__options: PulumiConfigEnvRmOptions, environmentName: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + environmentName);

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['config', 'env', 'rm', ...__arguments]);
}

export function configGet(__options: PulumiConfigGetOptions, key: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.open != null) {
        if (__options.open) {
            __arguments.push('--open');
        }
    }

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    return __run(__options, ['config', 'get', ...__arguments]);
}

export function configRefresh(__options: PulumiConfigRefreshOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    return __run(__options, ['config', 'refresh', ...__arguments]);
}

export function configRm(__options: PulumiConfigRmOptions, key: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    return __run(__options, ['config', 'rm', ...__arguments]);
}

export function configRmAll(__options: PulumiConfigRmAllOptions, ...key: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    return __run(__options, ['config', 'rm-all', ...__arguments]);
}

export function configSet(__options: PulumiConfigSetOptions, key: string, value?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    if (value != null) {
        __arguments.push('' + value);
    }

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    if (__options.plaintext != null) {
        if (__options.plaintext) {
            __arguments.push('--plaintext');
        }
    }

    if (__options.secret != null) {
        if (__options.secret) {
            __arguments.push('--secret');
        }
    }

    if (__options.__type != null) {
        __arguments.push('--type', '' + __options.__type);
    }

    return __run(__options, ['config', 'set', ...__arguments]);
}

export function configSetAll(__options: PulumiConfigSetAllOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        __arguments.push('--json', '' + __options.json);
    }

    if (__options.path != null) {
        if (__options.path) {
            __arguments.push('--path');
        }
    }

    if (__options.plaintext != null) {
        for (const __item of __options.plaintext) {
            __arguments.push('--plaintext', '' + __item);
        }
    }

    if (__options.secret != null) {
        for (const __item of __options.secret) {
            __arguments.push('--secret', '' + __item);
        }
    }

    return __run(__options, ['config', 'set-all', ...__arguments]);
}

export function config(__options: PulumiConfigOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.open != null) {
        if (__options.open) {
            __arguments.push('--open');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['config', ...__arguments]);
}

export function __console(__options: PulumiConsoleOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['console', ...__arguments]);
}

export function convert(__options: PulumiConvertOptions, ...arg: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (arg != null) {
        for (const __item of arg) {
            __arguments.push('' + __item);
        }
    }

    if (__options.from != null) {
        __arguments.push('--from', '' + __options.from);
    }

    if (__options.generateOnly != null) {
        if (__options.generateOnly) {
            __arguments.push('--generate-only');
        }
    }

    if (__options.language != null) {
        __arguments.push('--language', '' + __options.language);
    }

    if (__options.mappings != null) {
        for (const __item of __options.mappings) {
            __arguments.push('--mappings', '' + __item);
        }
    }

    if (__options.name != null) {
        __arguments.push('--name', '' + __options.name);
    }

    if (__options.out != null) {
        __arguments.push('--out', '' + __options.out);
    }

    if (__options.strict != null) {
        if (__options.strict) {
            __arguments.push('--strict');
        }
    }

    return __run(__options, ['convert', ...__arguments]);
}

export function convertTrace(__options: PulumiConvertTraceOptions, traceFile: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + traceFile);

    if (__options.granularity != null) {
        __arguments.push('--granularity', '' + __options.granularity);
    }

    if (__options.ignoreLogSpans != null) {
        if (__options.ignoreLogSpans) {
            __arguments.push('--ignore-log-spans');
        }
    }

    if (__options.otel != null) {
        if (__options.otel) {
            __arguments.push('--otel');
        }
    }

    return __run(__options, ['convert-trace', ...__arguments]);
}

export function deploymentRun(__options: PulumiDeploymentRunOptions, operation: string, url?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + operation);

    if (url != null) {
        __arguments.push('' + url);
    }

    if (__options.agentPoolId != null) {
        __arguments.push('--agent-pool-id', '' + __options.agentPoolId);
    }

    if (__options.env != null) {
        for (const __item of __options.env) {
            __arguments.push('--env', '' + __item);
        }
    }

    if (__options.envSecret != null) {
        for (const __item of __options.envSecret) {
            __arguments.push('--env-secret', '' + __item);
        }
    }

    if (__options.executorImage != null) {
        __arguments.push('--executor-image', '' + __options.executorImage);
    }

    if (__options.executorImagePassword != null) {
        __arguments.push('--executor-image-password', '' + __options.executorImagePassword);
    }

    if (__options.executorImageUsername != null) {
        __arguments.push('--executor-image-username', '' + __options.executorImageUsername);
    }

    if (__options.gitAuthAccessToken != null) {
        __arguments.push('--git-auth-access-token', '' + __options.gitAuthAccessToken);
    }

    if (__options.gitAuthPassword != null) {
        __arguments.push('--git-auth-password', '' + __options.gitAuthPassword);
    }

    if (__options.gitAuthSshPrivateKey != null) {
        __arguments.push('--git-auth-ssh-private-key', '' + __options.gitAuthSshPrivateKey);
    }

    if (__options.gitAuthSshPrivateKeyPath != null) {
        __arguments.push('--git-auth-ssh-private-key-path', '' + __options.gitAuthSshPrivateKeyPath);
    }

    if (__options.gitAuthUsername != null) {
        __arguments.push('--git-auth-username', '' + __options.gitAuthUsername);
    }

    if (__options.gitBranch != null) {
        __arguments.push('--git-branch', '' + __options.gitBranch);
    }

    if (__options.gitCommit != null) {
        __arguments.push('--git-commit', '' + __options.gitCommit);
    }

    if (__options.gitRepoDir != null) {
        __arguments.push('--git-repo-dir', '' + __options.gitRepoDir);
    }

    if (__options.inheritSettings != null) {
        if (__options.inheritSettings) {
            __arguments.push('--inherit-settings');
        }
    }

    if (__options.preRunCommand != null) {
        for (const __item of __options.preRunCommand) {
            __arguments.push('--pre-run-command', '' + __item);
        }
    }

    if (__options.skipInstallDependencies != null) {
        if (__options.skipInstallDependencies) {
            __arguments.push('--skip-install-dependencies');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.suppressPermalink != null) {
        if (__options.suppressPermalink) {
            __arguments.push('--suppress-permalink');
        }
    }

    if (__options.suppressStreamLogs != null) {
        if (__options.suppressStreamLogs) {
            __arguments.push('--suppress-stream-logs');
        }
    }

    return __run(__options, ['deployment', 'run', ...__arguments]);
}

export function deploymentSettingsConfigure(__options: PulumiDeploymentSettingsConfigureOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.gitAuthSshPrivateKey != null) {
        __arguments.push('--git-auth-ssh-private-key', '' + __options.gitAuthSshPrivateKey);
    }

    if (__options.gitAuthSshPrivateKeyPath != null) {
        __arguments.push('--git-auth-ssh-private-key-path', '' + __options.gitAuthSshPrivateKeyPath);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['deployment', 'settings', 'configure', ...__arguments]);
}

export function deploymentSettingsDestroy(__options: PulumiDeploymentSettingsDestroyOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['deployment', 'settings', 'destroy', ...__arguments]);
}

export function deploymentSettingsEnv(__options: PulumiDeploymentSettingsEnvOptions, key: string, value?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    if (value != null) {
        __arguments.push('' + value);
    }

    if (__options.remove != null) {
        if (__options.remove) {
            __arguments.push('--remove');
        }
    }

    if (__options.secret != null) {
        if (__options.secret) {
            __arguments.push('--secret');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['deployment', 'settings', 'env', ...__arguments]);
}

export function deploymentSettingsInit(__options: PulumiDeploymentSettingsInitOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.gitAuthSshPrivateKey != null) {
        __arguments.push('--git-auth-ssh-private-key', '' + __options.gitAuthSshPrivateKey);
    }

    if (__options.gitAuthSshPrivateKeyPath != null) {
        __arguments.push('--git-auth-ssh-private-key-path', '' + __options.gitAuthSshPrivateKeyPath);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['deployment', 'settings', 'init', ...__arguments]);
}

export function deploymentSettingsPull(__options: PulumiDeploymentSettingsPullOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['deployment', 'settings', 'pull', ...__arguments]);
}

export function deploymentSettingsPush(__options: PulumiDeploymentSettingsPushOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['deployment', 'settings', 'push', ...__arguments]);
}

export function deploymentSettings(__options: PulumiDeploymentSettingsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['deployment', 'settings', ...__arguments]);
}

export function deployment(__options: PulumiDeploymentOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    return __run(__options, ['deployment', ...__arguments]);
}

export function destroy(__options: PulumiDestroyOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.client != null) {
        __arguments.push('--client', '' + __options.client);
    }

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.continueOnError != null) {
        if (__options.continueOnError) {
            __arguments.push('--continue-on-error');
        }
    }

    if (__options.copilot != null) {
        if (__options.copilot) {
            __arguments.push('--copilot');
        }
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.exclude != null) {
        for (const __item of __options.exclude) {
            __arguments.push('--exclude', '' + __item);
        }
    }

    if (__options.excludeProtected != null) {
        if (__options.excludeProtected) {
            __arguments.push('--exclude-protected');
        }
    }

    if (__options.execAgent != null) {
        __arguments.push('--exec-agent', '' + __options.execAgent);
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.neo != null) {
        if (__options.neo) {
            __arguments.push('--neo');
        }
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.previewOnly != null) {
        if (__options.previewOnly) {
            __arguments.push('--preview-only');
        }
    }

    if (__options.refresh != null) {
        __arguments.push('--refresh', '' + __options.refresh);
    }

    if (__options.remove != null) {
        if (__options.remove) {
            __arguments.push('--remove');
        }
    }

    if (__options.runProgram != null) {
        if (__options.runProgram) {
            __arguments.push('--run-program');
        }
    }

    if (__options.showConfig != null) {
        if (__options.showConfig) {
            __arguments.push('--show-config');
        }
    }

    if (__options.showFullOutput != null) {
        if (__options.showFullOutput) {
            __arguments.push('--show-full-output');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.skipPreview != null) {
        if (__options.skipPreview) {
            __arguments.push('--skip-preview');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressPermalink != null) {
        __arguments.push('--suppress-permalink', '' + __options.suppressPermalink);
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    if (__options.target != null) {
        for (const __item of __options.target) {
            __arguments.push('--target', '' + __item);
        }
    }

    if (__options.targetDependents != null) {
        if (__options.targetDependents) {
            __arguments.push('--target-dependents');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['destroy', ...__arguments]);
}

export function envClone(__options: PulumiEnvCloneOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.preserveAccess != null) {
        if (__options.preserveAccess) {
            __arguments.push('--preserve-access');
        }
    }

    if (__options.preserveEnvTags != null) {
        if (__options.preserveEnvTags) {
            __arguments.push('--preserve-env-tags');
        }
    }

    if (__options.preserveHistory != null) {
        if (__options.preserveHistory) {
            __arguments.push('--preserve-history');
        }
    }

    if (__options.preserveRevTags != null) {
        if (__options.preserveRevTags) {
            __arguments.push('--preserve-rev-tags');
        }
    }

    return __run(__options, ['env', 'clone', ...__arguments]);
}

export function envDiff(__options: PulumiEnvDiffOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.format != null) {
        __arguments.push('--format', '' + __options.format);
    }

    if (__options.path != null) {
        __arguments.push('--path', '' + __options.path);
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    return __run(__options, ['env', 'diff', ...__arguments]);
}

export function envEdit(__options: PulumiEnvEditOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.draft != null) {
        __arguments.push('--draft', '' + __options.draft);
    }

    if (__options.editor != null) {
        __arguments.push('--editor', '' + __options.editor);
    }

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    return __run(__options, ['env', 'edit', ...__arguments]);
}

export function envGet(__options: PulumiEnvGetOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.definition != null) {
        if (__options.definition) {
            __arguments.push('--definition');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.value != null) {
        __arguments.push('--value', '' + __options.value);
    }

    return __run(__options, ['env', 'get', ...__arguments]);
}

export function envInit(__options: PulumiEnvInitOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    return __run(__options, ['env', 'init', ...__arguments]);
}

export function envLs(__options: PulumiEnvLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.organization != null) {
        __arguments.push('--organization', '' + __options.organization);
    }

    if (__options.project != null) {
        __arguments.push('--project', '' + __options.project);
    }

    return __run(__options, ['env', 'ls', ...__arguments]);
}

export function envOpen(__options: PulumiEnvOpenOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.draft != null) {
        __arguments.push('--draft', '' + __options.draft);
    }

    if (__options.format != null) {
        __arguments.push('--format', '' + __options.format);
    }

    if (__options.lifetime != null) {
        __arguments.push('--lifetime', '' + __options.lifetime);
    }

    return __run(__options, ['env', 'open', ...__arguments]);
}

export function envRm(__options: PulumiEnvRmOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['env', 'rm', ...__arguments]);
}

export function envRotate(__options: PulumiEnvRotateOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['env', 'rotate', ...__arguments]);
}

export function envRun(__options: PulumiEnvRunOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.draft != null) {
        __arguments.push('--draft', '' + __options.draft);
    }

    if (__options.interactive != null) {
        if (__options.interactive) {
            __arguments.push('--interactive');
        }
    }

    if (__options.lifetime != null) {
        __arguments.push('--lifetime', '' + __options.lifetime);
    }

    return __run(__options, ['env', 'run', ...__arguments]);
}

export function envSet(__options: PulumiEnvSetOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.draft != null) {
        __arguments.push('--draft', '' + __options.draft);
    }

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.plaintext != null) {
        if (__options.plaintext) {
            __arguments.push('--plaintext');
        }
    }

    if (__options.secret != null) {
        if (__options.secret) {
            __arguments.push('--secret');
        }
    }

    if (__options.string != null) {
        if (__options.string) {
            __arguments.push('--string');
        }
    }

    return __run(__options, ['env', 'set', ...__arguments]);
}

export function envTagGet(__options: PulumiEnvTagGetOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'tag', 'get', ...__arguments]);
}

export function envTagLs(__options: PulumiEnvTagLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.pager != null) {
        __arguments.push('--pager', '' + __options.pager);
    }

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'tag', 'ls', ...__arguments]);
}

export function envTagMv(__options: PulumiEnvTagMvOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'tag', 'mv', ...__arguments]);
}

export function envTagRm(__options: PulumiEnvTagRmOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['env', 'tag', 'rm', ...__arguments]);
}

export function envTag(__options: PulumiEnvTagOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'tag', ...__arguments]);
}

export function envVersionHistory(__options: PulumiEnvVersionHistoryOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.pager != null) {
        __arguments.push('--pager', '' + __options.pager);
    }

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'version', 'history', ...__arguments]);
}

export function envVersionRetract(__options: PulumiEnvVersionRetractOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.reason != null) {
        __arguments.push('--reason', '' + __options.reason);
    }

    if (__options.replaceWith != null) {
        __arguments.push('--replace-with', '' + __options.replaceWith);
    }

    return __run(__options, ['env', 'version', 'retract', ...__arguments]);
}

export function envVersionRollback(__options: PulumiEnvVersionRollbackOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.draft != null) {
        __arguments.push('--draft', '' + __options.draft);
    }

    return __run(__options, ['env', 'version', 'rollback', ...__arguments]);
}

export function envVersionTagLs(__options: PulumiEnvVersionTagLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.pager != null) {
        __arguments.push('--pager', '' + __options.pager);
    }

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'version', 'tag', 'ls', ...__arguments]);
}

export function envVersionTagRm(__options: PulumiEnvVersionTagRmOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['env', 'version', 'tag', 'rm', ...__arguments]);
}

export function envVersionTag(__options: PulumiEnvVersionTagOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'version', 'tag', ...__arguments]);
}

export function envVersion(__options: PulumiEnvVersionOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.utc != null) {
        if (__options.utc) {
            __arguments.push('--utc');
        }
    }

    return __run(__options, ['env', 'version', ...__arguments]);
}

export function genCompletion(__options: PulumiGenCompletionOptions, shell: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + shell);

    return __run(__options, ['gen-completion', ...__arguments]);
}

export function genMarkdown(__options: PulumiGenMarkdownOptions, dir: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + dir);

    return __run(__options, ['gen-markdown', ...__arguments]);
}

export function generateCliSpec(__options: PulumiGenerateCliSpecOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.help != null) {
        if (__options.help) {
            __arguments.push('--help');
        }
    }

    return __run(__options, ['generate-cli-spec', ...__arguments]);
}

export function help(__options: PulumiHelpOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['help', ...__arguments]);
}

export function __import(__options: PulumiImportOptions, ...arg: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (arg != null) {
        for (const __item of arg) {
            __arguments.push('' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.execAgent != null) {
        __arguments.push('--exec-agent', '' + __options.execAgent);
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.from != null) {
        __arguments.push('--from', '' + __options.from);
    }

    if (__options.generateCode != null) {
        if (__options.generateCode) {
            __arguments.push('--generate-code');
        }
    }

    if (__options.generateResources != null) {
        __arguments.push('--generate-resources', '' + __options.generateResources);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.out != null) {
        __arguments.push('--out', '' + __options.out);
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.parent != null) {
        __arguments.push('--parent', '' + __options.parent);
    }

    if (__options.previewOnly != null) {
        if (__options.previewOnly) {
            __arguments.push('--preview-only');
        }
    }

    if (__options.properties != null) {
        for (const __item of __options.properties) {
            __arguments.push('--properties', '' + __item);
        }
    }

    if (__options.protect != null) {
        if (__options.protect) {
            __arguments.push('--protect');
        }
    }

    if (__options.provider != null) {
        __arguments.push('--provider', '' + __options.provider);
    }

    if (__options.skipPreview != null) {
        if (__options.skipPreview) {
            __arguments.push('--skip-preview');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressPermalink != null) {
        __arguments.push('--suppress-permalink', '' + __options.suppressPermalink);
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['import', ...__arguments]);
}

export function install(__options: PulumiInstallOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.noDependencies != null) {
        if (__options.noDependencies) {
            __arguments.push('--no-dependencies');
        }
    }

    if (__options.noPlugins != null) {
        if (__options.noPlugins) {
            __arguments.push('--no-plugins');
        }
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.reinstall != null) {
        if (__options.reinstall) {
            __arguments.push('--reinstall');
        }
    }

    if (__options.useLanguageVersionTools != null) {
        if (__options.useLanguageVersionTools) {
            __arguments.push('--use-language-version-tools');
        }
    }

    return __run(__options, ['install', ...__arguments]);
}

export function login(__options: PulumiLoginOptions, url?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (url != null) {
        __arguments.push('' + url);
    }

    if (__options.cloudUrl != null) {
        __arguments.push('--cloud-url', '' + __options.cloudUrl);
    }

    if (__options.defaultOrg != null) {
        __arguments.push('--default-org', '' + __options.defaultOrg);
    }

    if (__options.insecure != null) {
        if (__options.insecure) {
            __arguments.push('--insecure');
        }
    }

    if (__options.interactive != null) {
        if (__options.interactive) {
            __arguments.push('--interactive');
        }
    }

    if (__options.local != null) {
        if (__options.local) {
            __arguments.push('--local');
        }
    }

    if (__options.oidcExpiration != null) {
        __arguments.push('--oidc-expiration', '' + __options.oidcExpiration);
    }

    if (__options.oidcOrg != null) {
        __arguments.push('--oidc-org', '' + __options.oidcOrg);
    }

    if (__options.oidcTeam != null) {
        __arguments.push('--oidc-team', '' + __options.oidcTeam);
    }

    if (__options.oidcToken != null) {
        __arguments.push('--oidc-token', '' + __options.oidcToken);
    }

    if (__options.oidcUser != null) {
        __arguments.push('--oidc-user', '' + __options.oidcUser);
    }

    return __run(__options, ['login', ...__arguments]);
}

export function logout(__options: PulumiLogoutOptions, url?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (url != null) {
        __arguments.push('' + url);
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.cloudUrl != null) {
        __arguments.push('--cloud-url', '' + __options.cloudUrl);
    }

    if (__options.local != null) {
        if (__options.local) {
            __arguments.push('--local');
        }
    }

    return __run(__options, ['logout', ...__arguments]);
}

export function logs(__options: PulumiLogsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.follow != null) {
        if (__options.follow) {
            __arguments.push('--follow');
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.resource != null) {
        __arguments.push('--resource', '' + __options.resource);
    }

    if (__options.since != null) {
        __arguments.push('--since', '' + __options.since);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['logs', ...__arguments]);
}

export function __new(__options: PulumiNewOptions, templateOrUrl?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (templateOrUrl != null) {
        __arguments.push('' + templateOrUrl);
    }

    if (__options.ai != null) {
        __arguments.push('--ai', '' + __options.ai);
    }

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.description != null) {
        __arguments.push('--description', '' + __options.description);
    }

    if (__options.dir != null) {
        __arguments.push('--dir', '' + __options.dir);
    }

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.generateOnly != null) {
        if (__options.generateOnly) {
            __arguments.push('--generate-only');
        }
    }

    if (__options.language != null) {
        __arguments.push('--language', '' + __options.language);
    }

    if (__options.listTemplates != null) {
        if (__options.listTemplates) {
            __arguments.push('--list-templates');
        }
    }

    if (__options.name != null) {
        __arguments.push('--name', '' + __options.name);
    }

    if (__options.offline != null) {
        if (__options.offline) {
            __arguments.push('--offline');
        }
    }

    if (__options.remoteStackConfig != null) {
        if (__options.remoteStackConfig) {
            __arguments.push('--remote-stack-config');
        }
    }

    if (__options.runtimeOptions != null) {
        for (const __item of __options.runtimeOptions) {
            __arguments.push('--runtime-options', '' + __item);
        }
    }

    if (__options.secretsProvider != null) {
        __arguments.push('--secrets-provider', '' + __options.secretsProvider);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.templateMode != null) {
        if (__options.templateMode) {
            __arguments.push('--template-mode');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['new', ...__arguments]);
}

export function orgGetDefault(__options: PulumiOrgGetDefaultOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['org', 'get-default', ...__arguments]);
}

export function orgSearchAi(__options: PulumiOrgSearchAiOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.delimiter != null) {
        __arguments.push('--delimiter', '' + __options.delimiter);
    }

    if (__options.org != null) {
        __arguments.push('--org', '' + __options.org);
    }

    if (__options.output != null) {
        __arguments.push('--output', '' + __options.output);
    }

    if (__options.query != null) {
        __arguments.push('--query', '' + __options.query);
    }

    if (__options.web != null) {
        if (__options.web) {
            __arguments.push('--web');
        }
    }

    return __run(__options, ['org', 'search', 'ai', ...__arguments]);
}

export function orgSearch(__options: PulumiOrgSearchOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.delimiter != null) {
        __arguments.push('--delimiter', '' + __options.delimiter);
    }

    if (__options.org != null) {
        __arguments.push('--org', '' + __options.org);
    }

    if (__options.output != null) {
        __arguments.push('--output', '' + __options.output);
    }

    if (__options.query != null) {
        for (const __item of __options.query) {
            __arguments.push('--query', '' + __item);
        }
    }

    if (__options.web != null) {
        if (__options.web) {
            __arguments.push('--web');
        }
    }

    return __run(__options, ['org', 'search', ...__arguments]);
}

export function orgSetDefault(__options: PulumiOrgSetDefaultOptions, name: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + name);

    return __run(__options, ['org', 'set-default', ...__arguments]);
}

export function org(__options: PulumiOrgOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['org', ...__arguments]);
}

export function packageAdd(__options: PulumiPackageAddOptions, provider: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + provider);

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    return __run(__options, ['package', 'add', ...__arguments]);
}

export function packageDelete(__options: PulumiPackageDeleteOptions, __package: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + __package);

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['package', 'delete', ...__arguments]);
}

export function packageGenSdk(__options: PulumiPackageGenSdkOptions, schemaSource: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + schemaSource);

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    if (__options.language != null) {
        __arguments.push('--language', '' + __options.language);
    }

    if (__options.local != null) {
        if (__options.local) {
            __arguments.push('--local');
        }
    }

    if (__options.out != null) {
        __arguments.push('--out', '' + __options.out);
    }

    if (__options.overlays != null) {
        __arguments.push('--overlays', '' + __options.overlays);
    }

    if (__options.version != null) {
        __arguments.push('--version', '' + __options.version);
    }

    return __run(__options, ['package', 'gen-sdk', ...__arguments]);
}

export function packageGetMapping(__options: PulumiPackageGetMappingOptions, key: string, schemaSource: string, providerKey?: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + key);

    __arguments.push('' + schemaSource);

    if (providerKey != null) {
        __arguments.push('' + providerKey);
    }

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    if (__options.out != null) {
        __arguments.push('--out', '' + __options.out);
    }

    return __run(__options, ['package', 'get-mapping', ...__arguments]);
}

export function packageGetSchema(__options: PulumiPackageGetSchemaOptions, schemaSource: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + schemaSource);

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    return __run(__options, ['package', 'get-schema', ...__arguments]);
}

export function packageInfo(__options: PulumiPackageInfoOptions, provider: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + provider);

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    if (__options.function != null) {
        __arguments.push('--function', '' + __options.function);
    }

    if (__options.module != null) {
        __arguments.push('--module', '' + __options.module);
    }

    if (__options.resource != null) {
        __arguments.push('--resource', '' + __options.resource);
    }

    return __run(__options, ['package', 'info', ...__arguments]);
}

export function packagePackSdk(__options: PulumiPackagePackSdkOptions, language: string, path: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + language);

    __arguments.push('' + path);

    return __run(__options, ['package', 'pack-sdk', ...__arguments]);
}

export function packagePublish(__options: PulumiPackagePublishOptions, provider: string, ...providerParameter: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + provider);

    if (providerParameter != null) {
        for (const __item of providerParameter) {
            __arguments.push('' + __item);
        }
    }

    if (__options.installationConfiguration != null) {
        __arguments.push('--installation-configuration', '' + __options.installationConfiguration);
    }

    if (__options.publisher != null) {
        __arguments.push('--publisher', '' + __options.publisher);
    }

    if (__options.readme != null) {
        __arguments.push('--readme', '' + __options.readme);
    }

    if (__options.source != null) {
        __arguments.push('--source', '' + __options.source);
    }

    return __run(__options, ['package', 'publish', ...__arguments]);
}

export function packagePublishSdk(__options: PulumiPackagePublishSdkOptions, language?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (language != null) {
        __arguments.push('' + language);
    }

    if (__options.path != null) {
        __arguments.push('--path', '' + __options.path);
    }

    return __run(__options, ['package', 'publish-sdk', ...__arguments]);
}

export function pluginInstall(__options: PulumiPluginInstallOptions, kind?: string, name?: string, version?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (kind != null) {
        __arguments.push('' + kind);
    }

    if (name != null) {
        __arguments.push('' + name);
    }

    if (version != null) {
        __arguments.push('' + version);
    }

    if (__options.checksum != null) {
        __arguments.push('--checksum', '' + __options.checksum);
    }

    if (__options.exact != null) {
        if (__options.exact) {
            __arguments.push('--exact');
        }
    }

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.reinstall != null) {
        if (__options.reinstall) {
            __arguments.push('--reinstall');
        }
    }

    if (__options.server != null) {
        __arguments.push('--server', '' + __options.server);
    }

    return __run(__options, ['plugin', 'install', ...__arguments]);
}

export function pluginLs(__options: PulumiPluginLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.project != null) {
        if (__options.project) {
            __arguments.push('--project');
        }
    }

    return __run(__options, ['plugin', 'ls', ...__arguments]);
}

export function pluginRm(__options: PulumiPluginRmOptions, kind?: string, name?: string, version?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (kind != null) {
        __arguments.push('' + kind);
    }

    if (name != null) {
        __arguments.push('' + name);
    }

    if (version != null) {
        __arguments.push('' + version);
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['plugin', 'rm', ...__arguments]);
}

export function pluginRun(__options: PulumiPluginRunOptions, nameOrPath: string, ...args: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + nameOrPath);

    if (args != null) {
        for (const __item of args) {
            __arguments.push('' + __item);
        }
    }

    if (__options.kind != null) {
        __arguments.push('--kind', '' + __options.kind);
    }

    return __run(__options, ['plugin', 'run', ...__arguments]);
}

export function policyDisable(__options: PulumiPolicyDisableOptions, policyPack: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + policyPack);

    if (__options.policyGroup != null) {
        __arguments.push('--policy-group', '' + __options.policyGroup);
    }

    if (__options.version != null) {
        __arguments.push('--version', '' + __options.version);
    }

    return __run(__options, ['policy', 'disable', ...__arguments]);
}

export function policyEnable(__options: PulumiPolicyEnableOptions, policyPack: string, version: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + policyPack);

    __arguments.push('' + version);

    if (__options.config != null) {
        __arguments.push('--config', '' + __options.config);
    }

    if (__options.policyGroup != null) {
        __arguments.push('--policy-group', '' + __options.policyGroup);
    }

    return __run(__options, ['policy', 'enable', ...__arguments]);
}

export function policyGroupLs(__options: PulumiPolicyGroupLsOptions, orgName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (orgName != null) {
        __arguments.push('' + orgName);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    return __run(__options, ['policy', 'group', 'ls', ...__arguments]);
}

export function policyInstall(__options: PulumiPolicyInstallOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['policy', 'install', ...__arguments]);
}

export function policyLs(__options: PulumiPolicyLsOptions, orgName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (orgName != null) {
        __arguments.push('' + orgName);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    return __run(__options, ['policy', 'ls', ...__arguments]);
}

export function policyNew(__options: PulumiPolicyNewOptions, template?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (template != null) {
        __arguments.push('' + template);
    }

    if (__options.dir != null) {
        __arguments.push('--dir', '' + __options.dir);
    }

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.generateOnly != null) {
        if (__options.generateOnly) {
            __arguments.push('--generate-only');
        }
    }

    if (__options.offline != null) {
        if (__options.offline) {
            __arguments.push('--offline');
        }
    }

    return __run(__options, ['policy', 'new', ...__arguments]);
}

export function policyPublish(__options: PulumiPolicyPublishOptions, orgName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (orgName != null) {
        __arguments.push('' + orgName);
    }

    return __run(__options, ['policy', 'publish', ...__arguments]);
}

export function policyRm(__options: PulumiPolicyRmOptions, policyPack: string, version: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + policyPack);

    __arguments.push('' + version);

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['policy', 'rm', ...__arguments]);
}

export function policyValidateConfig(__options: PulumiPolicyValidateConfigOptions, policyPack: string, version: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + policyPack);

    __arguments.push('' + version);

    if (__options.config != null) {
        __arguments.push('--config', '' + __options.config);
    }

    return __run(__options, ['policy', 'validate-config', ...__arguments]);
}

export function preview(__options: PulumiPreviewOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.attachDebugger != null) {
        for (const __item of __options.attachDebugger) {
            __arguments.push('--attach-debugger', '' + __item);
        }
    }

    if (__options.client != null) {
        __arguments.push('--client', '' + __options.client);
    }

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.copilot != null) {
        if (__options.copilot) {
            __arguments.push('--copilot');
        }
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.exclude != null) {
        for (const __item of __options.exclude) {
            __arguments.push('--exclude', '' + __item);
        }
    }

    if (__options.excludeDependents != null) {
        if (__options.excludeDependents) {
            __arguments.push('--exclude-dependents');
        }
    }

    if (__options.execAgent != null) {
        __arguments.push('--exec-agent', '' + __options.execAgent);
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.expectNoChanges != null) {
        if (__options.expectNoChanges) {
            __arguments.push('--expect-no-changes');
        }
    }

    if (__options.importFile != null) {
        __arguments.push('--import-file', '' + __options.importFile);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.neo != null) {
        if (__options.neo) {
            __arguments.push('--neo');
        }
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.policyPack != null) {
        for (const __item of __options.policyPack) {
            __arguments.push('--policy-pack', '' + __item);
        }
    }

    if (__options.policyPackConfig != null) {
        for (const __item of __options.policyPackConfig) {
            __arguments.push('--policy-pack-config', '' + __item);
        }
    }

    if (__options.refresh != null) {
        __arguments.push('--refresh', '' + __options.refresh);
    }

    if (__options.replace != null) {
        for (const __item of __options.replace) {
            __arguments.push('--replace', '' + __item);
        }
    }

    if (__options.runProgram != null) {
        if (__options.runProgram) {
            __arguments.push('--run-program');
        }
    }

    if (__options.savePlan != null) {
        __arguments.push('--save-plan', '' + __options.savePlan);
    }

    if (__options.showConfig != null) {
        if (__options.showConfig) {
            __arguments.push('--show-config');
        }
    }

    if (__options.showFullOutput != null) {
        if (__options.showFullOutput) {
            __arguments.push('--show-full-output');
        }
    }

    if (__options.showPolicyRemediations != null) {
        if (__options.showPolicyRemediations) {
            __arguments.push('--show-policy-remediations');
        }
    }

    if (__options.showReads != null) {
        if (__options.showReads) {
            __arguments.push('--show-reads');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressPermalink != null) {
        __arguments.push('--suppress-permalink', '' + __options.suppressPermalink);
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    if (__options.target != null) {
        for (const __item of __options.target) {
            __arguments.push('--target', '' + __item);
        }
    }

    if (__options.targetDependents != null) {
        if (__options.targetDependents) {
            __arguments.push('--target-dependents');
        }
    }

    if (__options.targetReplace != null) {
        for (const __item of __options.targetReplace) {
            __arguments.push('--target-replace', '' + __item);
        }
    }

    return __run(__options, ['preview', ...__arguments]);
}

export function projectLs(__options: PulumiProjectLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.organization != null) {
        __arguments.push('--organization', '' + __options.organization);
    }

    return __run(__options, ['project', 'ls', ...__arguments]);
}

export function refresh(__options: PulumiRefreshOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.clearPendingCreates != null) {
        if (__options.clearPendingCreates) {
            __arguments.push('--clear-pending-creates');
        }
    }

    if (__options.client != null) {
        __arguments.push('--client', '' + __options.client);
    }

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.copilot != null) {
        if (__options.copilot) {
            __arguments.push('--copilot');
        }
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.exclude != null) {
        for (const __item of __options.exclude) {
            __arguments.push('--exclude', '' + __item);
        }
    }

    if (__options.excludeDependents != null) {
        if (__options.excludeDependents) {
            __arguments.push('--exclude-dependents');
        }
    }

    if (__options.execAgent != null) {
        __arguments.push('--exec-agent', '' + __options.execAgent);
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.expectNoChanges != null) {
        if (__options.expectNoChanges) {
            __arguments.push('--expect-no-changes');
        }
    }

    if (__options.importPendingCreates != null) {
        for (const __item of __options.importPendingCreates) {
            __arguments.push('--import-pending-creates', '' + __item);
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.neo != null) {
        if (__options.neo) {
            __arguments.push('--neo');
        }
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.previewOnly != null) {
        if (__options.previewOnly) {
            __arguments.push('--preview-only');
        }
    }

    if (__options.runProgram != null) {
        if (__options.runProgram) {
            __arguments.push('--run-program');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.skipPendingCreates != null) {
        if (__options.skipPendingCreates) {
            __arguments.push('--skip-pending-creates');
        }
    }

    if (__options.skipPreview != null) {
        if (__options.skipPreview) {
            __arguments.push('--skip-preview');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressPermalink != null) {
        __arguments.push('--suppress-permalink', '' + __options.suppressPermalink);
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    if (__options.target != null) {
        for (const __item of __options.target) {
            __arguments.push('--target', '' + __item);
        }
    }

    if (__options.targetDependents != null) {
        if (__options.targetDependents) {
            __arguments.push('--target-dependents');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['refresh', ...__arguments]);
}

export function replayEvents(__options: PulumiReplayEventsOptions, kind: string, eventsFile: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + kind);

    __arguments.push('' + eventsFile);

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.delay != null) {
        __arguments.push('--delay', '' + __options.delay);
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.period != null) {
        __arguments.push('--period', '' + __options.period);
    }

    if (__options.preview != null) {
        if (__options.preview) {
            __arguments.push('--preview');
        }
    }

    if (__options.showConfig != null) {
        if (__options.showConfig) {
            __arguments.push('--show-config');
        }
    }

    if (__options.showReads != null) {
        if (__options.showReads) {
            __arguments.push('--show-reads');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    return __run(__options, ['replay-events', ...__arguments]);
}

export function schemaCheck(__options: PulumiSchemaCheckOptions, file: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + file);

    if (__options.allowDanglingReferences != null) {
        if (__options.allowDanglingReferences) {
            __arguments.push('--allow-dangling-references');
        }
    }

    return __run(__options, ['schema', 'check', ...__arguments]);
}

export function stackChangeSecretsProvider(__options: PulumiStackChangeSecretsProviderOptions, newSecretsProvider: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + newSecretsProvider);

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'change-secrets-provider', ...__arguments]);
}

export function stackExport(__options: PulumiStackExportOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.version != null) {
        __arguments.push('--version', '' + __options.version);
    }

    return __run(__options, ['stack', 'export', ...__arguments]);
}

export function stackGraph(__options: PulumiStackGraphOptions, filename: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + filename);

    if (__options.dependencyEdgeColor != null) {
        __arguments.push('--dependency-edge-color', '' + __options.dependencyEdgeColor);
    }

    if (__options.dotFragment != null) {
        __arguments.push('--dot-fragment', '' + __options.dotFragment);
    }

    if (__options.ignoreDependencyEdges != null) {
        if (__options.ignoreDependencyEdges) {
            __arguments.push('--ignore-dependency-edges');
        }
    }

    if (__options.ignoreParentEdges != null) {
        if (__options.ignoreParentEdges) {
            __arguments.push('--ignore-parent-edges');
        }
    }

    if (__options.parentEdgeColor != null) {
        __arguments.push('--parent-edge-color', '' + __options.parentEdgeColor);
    }

    if (__options.shortNodeName != null) {
        if (__options.shortNodeName) {
            __arguments.push('--short-node-name');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'graph', ...__arguments]);
}

export function stackHistory(__options: PulumiStackHistoryOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.fullDates != null) {
        if (__options.fullDates) {
            __arguments.push('--full-dates');
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.page != null) {
        __arguments.push('--page', '' + __options.page);
    }

    if (__options.pageSize != null) {
        __arguments.push('--page-size', '' + __options.pageSize);
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'history', ...__arguments]);
}

export function stackImport(__options: PulumiStackImportOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.file != null) {
        __arguments.push('--file', '' + __options.file);
    }

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'import', ...__arguments]);
}

export function stackInit(__options: PulumiStackInitOptions, stackName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (stackName != null) {
        __arguments.push('' + stackName);
    }

    if (__options.copyConfigFrom != null) {
        __arguments.push('--copy-config-from', '' + __options.copyConfigFrom);
    }

    if (__options.noSelect != null) {
        if (__options.noSelect) {
            __arguments.push('--no-select');
        }
    }

    if (__options.remoteConfig != null) {
        if (__options.remoteConfig) {
            __arguments.push('--remote-config');
        }
    }

    if (__options.secretsProvider != null) {
        __arguments.push('--secrets-provider', '' + __options.secretsProvider);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.teams != null) {
        for (const __item of __options.teams) {
            __arguments.push('--teams', '' + __item);
        }
    }

    return __run(__options, ['stack', 'init', ...__arguments]);
}

export function stackLs(__options: PulumiStackLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.organization != null) {
        __arguments.push('--organization', '' + __options.organization);
    }

    if (__options.project != null) {
        __arguments.push('--project', '' + __options.project);
    }

    if (__options.tag != null) {
        __arguments.push('--tag', '' + __options.tag);
    }

    return __run(__options, ['stack', 'ls', ...__arguments]);
}

export function stackOutput(__options: PulumiStackOutputOptions, propertyName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (propertyName != null) {
        __arguments.push('' + propertyName);
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.shell != null) {
        if (__options.shell) {
            __arguments.push('--shell');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'output', ...__arguments]);
}

export function stackRename(__options: PulumiStackRenameOptions, newStackName: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + newStackName);

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'rename', ...__arguments]);
}

export function stackRm(__options: PulumiStackRmOptions, stackName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (stackName != null) {
        __arguments.push('' + stackName);
    }

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.preserveConfig != null) {
        if (__options.preserveConfig) {
            __arguments.push('--preserve-config');
        }
    }

    if (__options.removeBackups != null) {
        if (__options.removeBackups) {
            __arguments.push('--remove-backups');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['stack', 'rm', ...__arguments]);
}

export function stackSelect(__options: PulumiStackSelectOptions, stack?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (stack != null) {
        __arguments.push('' + stack);
    }

    if (__options.create != null) {
        if (__options.create) {
            __arguments.push('--create');
        }
    }

    if (__options.secretsProvider != null) {
        __arguments.push('--secrets-provider', '' + __options.secretsProvider);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', 'select', ...__arguments]);
}

export function stackTagGet(__options: PulumiStackTagGetOptions, name: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + name);

    return __run(__options, ['stack', 'tag', 'get', ...__arguments]);
}

export function stackTagLs(__options: PulumiStackTagLsOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    return __run(__options, ['stack', 'tag', 'ls', ...__arguments]);
}

export function stackTagRm(__options: PulumiStackTagRmOptions, name: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + name);

    return __run(__options, ['stack', 'tag', 'rm', ...__arguments]);
}

export function stackTagSet(__options: PulumiStackTagSetOptions, name: string, value: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + name);

    __arguments.push('' + value);

    return __run(__options, ['stack', 'tag', 'set', ...__arguments]);
}

export function stackUnselect(__options: PulumiStackUnselectOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['stack', 'unselect', ...__arguments]);
}

export function stack(__options: PulumiStackOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.showIds != null) {
        if (__options.showIds) {
            __arguments.push('--show-ids');
        }
    }

    if (__options.showName != null) {
        if (__options.showName) {
            __arguments.push('--show-name');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.showUrns != null) {
        if (__options.showUrns) {
            __arguments.push('--show-urns');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['stack', ...__arguments]);
}

export function stateDelete(__options: PulumiStateDeleteOptions, resourceUrn?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        __arguments.push('' + resourceUrn);
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.force != null) {
        if (__options.force) {
            __arguments.push('--force');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.targetDependents != null) {
        if (__options.targetDependents) {
            __arguments.push('--target-dependents');
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'delete', ...__arguments]);
}

export function stateEdit(__options: PulumiStateEditOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['state', 'edit', ...__arguments]);
}

export function stateMove(__options: PulumiStateMoveOptions, ...urn: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + urn);

    if (__options.dest != null) {
        __arguments.push('--dest', '' + __options.dest);
    }

    if (__options.includeParents != null) {
        if (__options.includeParents) {
            __arguments.push('--include-parents');
        }
    }

    if (__options.source != null) {
        __arguments.push('--source', '' + __options.source);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'move', ...__arguments]);
}

export function stateProtect(__options: PulumiStateProtectOptions, ...resourceUrn: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        for (const __item of resourceUrn) {
            __arguments.push('' + __item);
        }
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'protect', ...__arguments]);
}

export function stateRename(__options: PulumiStateRenameOptions, resourceUrn?: string, newName?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        __arguments.push('' + resourceUrn);
    }

    if (newName != null) {
        __arguments.push('' + newName);
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'rename', ...__arguments]);
}

export function stateRepair(__options: PulumiStateRepairOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'repair', ...__arguments]);
}

export function stateTaint(__options: PulumiStateTaintOptions, ...resourceUrn: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        for (const __item of resourceUrn) {
            __arguments.push('' + __item);
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'taint', ...__arguments]);
}

export function stateUnprotect(__options: PulumiStateUnprotectOptions, ...resourceUrn: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        for (const __item of resourceUrn) {
            __arguments.push('' + __item);
        }
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'unprotect', ...__arguments]);
}

export function stateUntaint(__options: PulumiStateUntaintOptions, ...resourceUrn: string[]): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (resourceUrn != null) {
        for (const __item of resourceUrn) {
            __arguments.push('' + __item);
        }
    }

    if (__options.all != null) {
        if (__options.all) {
            __arguments.push('--all');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'untaint', ...__arguments]);
}

export function stateUpgrade(__options: PulumiStateUpgradeOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['state', 'upgrade', ...__arguments]);
}

export function templatePublish(__options: PulumiTemplatePublishOptions, directory: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + directory);

    if (__options.name != null) {
        __arguments.push('--name', '' + __options.name);
    }

    if (__options.publisher != null) {
        __arguments.push('--publisher', '' + __options.publisher);
    }

    if (__options.version != null) {
        __arguments.push('--version', '' + __options.version);
    }

    return __run(__options, ['template', 'publish', ...__arguments]);
}

export function up(__options: PulumiUpOptions, templateOrUrl?: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (templateOrUrl != null) {
        __arguments.push('' + templateOrUrl);
    }

    if (__options.attachDebugger != null) {
        for (const __item of __options.attachDebugger) {
            __arguments.push('--attach-debugger', '' + __item);
        }
    }

    if (__options.client != null) {
        __arguments.push('--client', '' + __options.client);
    }

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.continueOnError != null) {
        if (__options.continueOnError) {
            __arguments.push('--continue-on-error');
        }
    }

    if (__options.copilot != null) {
        if (__options.copilot) {
            __arguments.push('--copilot');
        }
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.diff != null) {
        if (__options.diff) {
            __arguments.push('--diff');
        }
    }

    if (__options.exclude != null) {
        for (const __item of __options.exclude) {
            __arguments.push('--exclude', '' + __item);
        }
    }

    if (__options.excludeDependents != null) {
        if (__options.excludeDependents) {
            __arguments.push('--exclude-dependents');
        }
    }

    if (__options.execAgent != null) {
        __arguments.push('--exec-agent', '' + __options.execAgent);
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.expectNoChanges != null) {
        if (__options.expectNoChanges) {
            __arguments.push('--expect-no-changes');
        }
    }

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.neo != null) {
        if (__options.neo) {
            __arguments.push('--neo');
        }
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.plan != null) {
        __arguments.push('--plan', '' + __options.plan);
    }

    if (__options.policyPack != null) {
        for (const __item of __options.policyPack) {
            __arguments.push('--policy-pack', '' + __item);
        }
    }

    if (__options.policyPackConfig != null) {
        for (const __item of __options.policyPackConfig) {
            __arguments.push('--policy-pack-config', '' + __item);
        }
    }

    if (__options.refresh != null) {
        __arguments.push('--refresh', '' + __options.refresh);
    }

    if (__options.replace != null) {
        for (const __item of __options.replace) {
            __arguments.push('--replace', '' + __item);
        }
    }

    if (__options.runProgram != null) {
        if (__options.runProgram) {
            __arguments.push('--run-program');
        }
    }

    if (__options.secretsProvider != null) {
        __arguments.push('--secrets-provider', '' + __options.secretsProvider);
    }

    if (__options.showConfig != null) {
        if (__options.showConfig) {
            __arguments.push('--show-config');
        }
    }

    if (__options.showFullOutput != null) {
        if (__options.showFullOutput) {
            __arguments.push('--show-full-output');
        }
    }

    if (__options.showPolicyRemediations != null) {
        if (__options.showPolicyRemediations) {
            __arguments.push('--show-policy-remediations');
        }
    }

    if (__options.showReads != null) {
        if (__options.showReads) {
            __arguments.push('--show-reads');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.showSecrets != null) {
        if (__options.showSecrets) {
            __arguments.push('--show-secrets');
        }
    }

    if (__options.skipPreview != null) {
        if (__options.skipPreview) {
            __arguments.push('--skip-preview');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    if (__options.strict != null) {
        if (__options.strict) {
            __arguments.push('--strict');
        }
    }

    if (__options.suppressOutputs != null) {
        if (__options.suppressOutputs) {
            __arguments.push('--suppress-outputs');
        }
    }

    if (__options.suppressPermalink != null) {
        __arguments.push('--suppress-permalink', '' + __options.suppressPermalink);
    }

    if (__options.suppressProgress != null) {
        if (__options.suppressProgress) {
            __arguments.push('--suppress-progress');
        }
    }

    if (__options.target != null) {
        for (const __item of __options.target) {
            __arguments.push('--target', '' + __item);
        }
    }

    if (__options.targetDependents != null) {
        if (__options.targetDependents) {
            __arguments.push('--target-dependents');
        }
    }

    if (__options.targetReplace != null) {
        for (const __item of __options.targetReplace) {
            __arguments.push('--target-replace', '' + __item);
        }
    }

    if (__options.yes != null) {
        if (__options.yes) {
            __arguments.push('--yes');
        }
    }

    return __run(__options, ['up', ...__arguments]);
}

export function version(__options: PulumiVersionOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    return __run(__options, ['version', ...__arguments]);
}

export function viewTrace(__options: PulumiViewTraceOptions, traceFile: string): Promise<CommandResult> {
    const __arguments: string[] = [];

    __arguments.push('' + traceFile);

    if (__options.port != null) {
        __arguments.push('--port', '' + __options.port);
    }

    return __run(__options, ['view-trace', ...__arguments]);
}

export function watch(__options: PulumiWatchOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.config != null) {
        for (const __item of __options.config) {
            __arguments.push('--config', '' + __item);
        }
    }

    if (__options.configFile != null) {
        __arguments.push('--config-file', '' + __options.configFile);
    }

    if (__options.configPath != null) {
        if (__options.configPath) {
            __arguments.push('--config-path');
        }
    }

    if (__options.debug != null) {
        if (__options.debug) {
            __arguments.push('--debug');
        }
    }

    if (__options.execKind != null) {
        __arguments.push('--exec-kind', '' + __options.execKind);
    }

    if (__options.message != null) {
        __arguments.push('--message', '' + __options.message);
    }

    if (__options.parallel != null) {
        __arguments.push('--parallel', '' + __options.parallel);
    }

    if (__options.path != null) {
        for (const __item of __options.path) {
            __arguments.push('--path', '' + __item);
        }
    }

    if (__options.policyPack != null) {
        for (const __item of __options.policyPack) {
            __arguments.push('--policy-pack', '' + __item);
        }
    }

    if (__options.policyPackConfig != null) {
        for (const __item of __options.policyPackConfig) {
            __arguments.push('--policy-pack-config', '' + __item);
        }
    }

    if (__options.refresh != null) {
        if (__options.refresh) {
            __arguments.push('--refresh');
        }
    }

    if (__options.secretsProvider != null) {
        __arguments.push('--secrets-provider', '' + __options.secretsProvider);
    }

    if (__options.showConfig != null) {
        if (__options.showConfig) {
            __arguments.push('--show-config');
        }
    }

    if (__options.showReplacementSteps != null) {
        if (__options.showReplacementSteps) {
            __arguments.push('--show-replacement-steps');
        }
    }

    if (__options.showSames != null) {
        if (__options.showSames) {
            __arguments.push('--show-sames');
        }
    }

    if (__options.stack != null) {
        __arguments.push('--stack', '' + __options.stack);
    }

    return __run(__options, ['watch', ...__arguments]);
}

export function whoami(__options: PulumiWhoamiOptions): Promise<CommandResult> {
    const __arguments: string[] = [];

    if (__options.json != null) {
        if (__options.json) {
            __arguments.push('--json');
        }
    }

    if (__options.verbose != null) {
        if (__options.verbose) {
            __arguments.push('--verbose');
        }
    }

    return __run(__options, ['whoami', ...__arguments]);
}
