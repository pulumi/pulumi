/** Flags for the `pulumi` command */
export interface PulumiOptions {
  /** Colorize output. Choices are: always, never, raw, auto */
  "color"?: string;
  /** Run pulumi as if it had been started in another directory */
  "cwd"?: string;
  /** Disable integrity checking of checkpoint files */
  "disable-integrity-checking"?: boolean;
  /** Enable emojis in the output */
  "emoji"?: boolean;
  /** Show fully-qualified stack names */
  "fully-qualify-stack-names"?: boolean;
  /** Flow log settings to child processes (like plugins) */
  "logflow"?: boolean;
  /** Log to stderr instead of to files */
  "logtostderr"?: boolean;
  /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
  "memprofilerate"?: number;
  /** Disable interactive mode for all commands */
  "non-interactive"?: boolean;
  /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
  "profiling"?: string;
  /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
  "tracing"?: string;
  /** Include the tracing header with the given contents. */
  "tracing-header"?: string;
  /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
  "verbose"?: number;
}
/** Flags for the `pulumi about` command */
export interface PulumiAboutOptions extends PulumiOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** The name of the stack to get info on. Defaults to the current stack */
  "stack"?: string;
  /** Include transitive dependencies */
  "transitive"?: boolean;
}
/** Flags for the `pulumi about env` command */
export interface PulumiAboutEnvOptions extends PulumiAboutOptions {
}
/** Flags for the `pulumi ai` command */
export interface PulumiAiOptions extends PulumiOptions {
}
/** Flags for the `pulumi ai web` command */
export interface PulumiAiWebOptions extends PulumiAiOptions {
  /** Language to use for the prompt - this defaults to TypeScript. [TypeScript, Python, Go, C#, Java, YAML] */
  "language"?: string;
  /** Opt-out of automatically submitting the prompt to Pulumi AI */
  "no-auto-submit"?: boolean;
}
/** Flags for the `pulumi cancel` command */
export interface PulumiCancelOptions extends PulumiOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts, and proceed with cancellation anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi config` command */
export interface PulumiConfigOptions extends PulumiOptions {
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Emit output as JSON */
  "json"?: boolean;
  /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
  "open"?: boolean;
  /** Show secret values when listing config instead of displaying blinded values */
  "show-secrets"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi config cp` command */
export interface PulumiConfigCpOptions extends PulumiConfigOptions {
  /** The name of the new stack to copy the config to */
  "dest"?: string;
  /** The key contains a path to a property in a map or list to set */
  "path"?: boolean;
}
/** Flags for the `pulumi config env` command */
export interface PulumiConfigEnvOptions extends PulumiConfigOptions {
}
/** Flags for the `pulumi config env add` command */
export interface PulumiConfigEnvAddOptions extends PulumiConfigEnvOptions {
  /** Show secret values in plaintext instead of ciphertext */
  "show-secrets"?: boolean;
  /** True to save changes without prompting */
  "yes"?: boolean;
}
/** Flags for the `pulumi config env init` command */
export interface PulumiConfigEnvInitOptions extends PulumiConfigEnvOptions {
  /** The name of the environment to create. Defaults to "<project name>/<stack name>" */
  "env"?: string;
  /** Do not remove configuration values from the stack after creating the environment */
  "keep-config"?: boolean;
  /** Show secret values in plaintext instead of ciphertext */
  "show-secrets"?: boolean;
  /** True to save the created environment without prompting */
  "yes"?: boolean;
}
/** Flags for the `pulumi config env ls` command */
export interface PulumiConfigEnvLsOptions extends PulumiConfigEnvOptions {
  /** Emit output as JSON */
  "json"?: boolean;
}
/** Flags for the `pulumi config env rm` command */
export interface PulumiConfigEnvRmOptions extends PulumiConfigEnvOptions {
  /** Show secret values in plaintext instead of ciphertext */
  "show-secrets"?: boolean;
  /** True to save changes without prompting */
  "yes"?: boolean;
}
/** Flags for the `pulumi config get` command */
export interface PulumiConfigGetOptions extends PulumiConfigOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** Open and resolve any environments listed in the stack configuration */
  "open"?: boolean;
  /** The key contains a path to a property in a map or list to get */
  "path"?: boolean;
}
/** Flags for the `pulumi config refresh` command */
export interface PulumiConfigRefreshOptions extends PulumiConfigOptions {
  /** Overwrite configuration file, if it exists, without creating a backup */
  "force"?: boolean;
}
/** Flags for the `pulumi config rm` command */
export interface PulumiConfigRmOptions extends PulumiConfigOptions {
  /** The key contains a path to a property in a map or list to remove */
  "path"?: boolean;
}
/** Flags for the `pulumi config rm-all` command */
export interface PulumiConfigRmAllOptions extends PulumiConfigOptions {
  /** Parse the keys as paths in a map or list rather than raw strings */
  "path"?: boolean;
}
/** Flags for the `pulumi config set` command */
export interface PulumiConfigSetOptions extends PulumiConfigOptions {
  /** The key contains a path to a property in a map or list to set */
  "path"?: boolean;
  /** Save the value as plaintext (unencrypted) */
  "plaintext"?: boolean;
  /** Encrypt the value instead of storing it in plaintext */
  "secret"?: boolean;
  /** Save the value as the given type.  Allowed values are string, bool, int, and float */
  "type"?: string;
}
/** Flags for the `pulumi config set-all` command */
export interface PulumiConfigSetAllOptions extends PulumiConfigOptions {
  /** Read values from a JSON string in the format produced by 'pulumi config --json' */
  "json"?: string;
  /** Parse the keys as paths in a map or list rather than raw strings */
  "path"?: boolean;
  /** Marks a value as plaintext (unencrypted) */
  "plaintext"?: string[];
  /** Marks a value as secret to be encrypted */
  "secret"?: string[];
}
/** Flags for the `pulumi console` command */
export interface PulumiConsoleOptions extends PulumiOptions {
  /** The name of the stack to view */
  "stack"?: string;
}
/** Flags for the `pulumi convert` command */
export interface PulumiConvertOptions extends PulumiOptions {
  /** Which converter plugin to use to read the source program */
  "from"?: string;
  /** Generate the converted program(s) only; do not install dependencies */
  "generate-only"?: boolean;
  /** Which language plugin to use to generate the Pulumi project */
  "language"?: string;
  /** Any mapping files to use in the conversion */
  "mappings"?: string[];
  /** The name to use for the converted project; defaults to the directory of the source project */
  "name"?: string;
  /** The output directory to write the converted project to */
  "out"?: string;
  /** Fail the conversion on errors such as missing variables */
  "strict"?: boolean;
}
/** Flags for the `pulumi convert-trace` command */
export interface PulumiConvertTraceOptions extends PulumiOptions {
  /** the sample granularity */
  "granularity"?: string;
  /** true to ignore log spans */
  "ignore-log-spans"?: boolean;
  /** true to export to OpenTelemetry */
  "otel"?: boolean;
}
/** Flags for the `pulumi deployment` command */
export interface PulumiDeploymentOptions extends PulumiOptions {
  /** Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml */
  "config-file"?: string;
}
/** Flags for the `pulumi deployment run` command */
export interface PulumiDeploymentRunOptions extends PulumiDeploymentOptions {
  /** The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue will be used. */
  "agent-pool-id"?: string;
  /** Environment variables to use in the remote operation of the form NAME=value (e.g. `--env FOO=bar`) */
  "env"?: string[];
  /** Environment variables with secret values to use in the remote operation of the form NAME=secretvalue (e.g. `--env FOO=secret`) */
  "env-secret"?: string[];
  /** The Docker image to use for the executor */
  "executor-image"?: string;
  /** The password for the credentials with access to the Docker image to use for the executor */
  "executor-image-password"?: string;
  /** The username for the credentials with access to the Docker image to use for the executor */
  "executor-image-username"?: string;
  /** Git personal access token */
  "git-auth-access-token"?: string;
  /** Git password; for use with username or with an SSH private key */
  "git-auth-password"?: string;
  /** Git SSH private key; use --git-auth-password for the password, if needed */
  "git-auth-ssh-private-key"?: string;
  /** Git SSH private key path; use --git-auth-password for the password, if needed */
  "git-auth-ssh-private-key-path"?: string;
  /** Git username */
  "git-auth-username"?: string;
  /** Git branch to deploy; this is mutually exclusive with --git-commit; either value needs to be specified */
  "git-branch"?: string;
  /** Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); this is mutually exclusive with --git-branch; either value needs to be specified */
  "git-commit"?: string;
  /** The directory to work from in the project's source repository where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root */
  "git-repo-dir"?: string;
  /** Inherit deployment settings from the current stack */
  "inherit-settings"?: boolean;
  /** Commands to run before the remote operation */
  "pre-run-command"?: string[];
  /** Whether to skip the default dependency installation step */
  "skip-install-dependencies"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: boolean;
  /** Suppress log streaming of the deployment job */
  "suppress-stream-logs"?: boolean;
}
/** Flags for the `pulumi deployment settings` command */
export interface PulumiDeploymentSettingsOptions extends PulumiDeploymentOptions {
}
/** Flags for the `pulumi deployment settings configure` command */
export interface PulumiDeploymentSettingsConfigureOptions extends PulumiDeploymentSettingsOptions {
  /** Git SSH private key */
  "git-auth-ssh-private-key"?: string;
  /** Private key path */
  "git-auth-ssh-private-key-path"?: string;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi deployment settings destroy` command */
export interface PulumiDeploymentSettingsDestroyOptions extends PulumiDeploymentSettingsOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Automatically confirm every confirmation prompt */
  "yes"?: boolean;
}
/** Flags for the `pulumi deployment settings env` command */
export interface PulumiDeploymentSettingsEnvOptions extends PulumiDeploymentSettingsOptions {
  /** whether the key should be removed */
  "remove"?: boolean;
  /** whether the value should be treated as a secret and be encrypted */
  "secret"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi deployment settings init` command */
export interface PulumiDeploymentSettingsInitOptions extends PulumiDeploymentSettingsOptions {
  /** Forces content to be generated even if it is already configured */
  "force"?: boolean;
  /** Git SSH private key */
  "git-auth-ssh-private-key"?: string;
  /** Git SSH private key path */
  "git-auth-ssh-private-key-path"?: string;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi deployment settings pull` command */
export interface PulumiDeploymentSettingsPullOptions extends PulumiDeploymentSettingsOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi deployment settings push` command */
export interface PulumiDeploymentSettingsPushOptions extends PulumiDeploymentSettingsOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Automatically confirm every confirmation prompt */
  "yes"?: boolean;
}
/** Flags for the `pulumi destroy` command */
export interface PulumiDestroyOptions extends PulumiOptions {
  /** The address of an existing language runtime host to connect to */
  "client"?: string;
  /** Config to use during the destroy and save to the stack config file */
  "config"?: string[];
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** Continue to perform the destroy operation despite the occurrence of errors (can also be set with PULUMI_CONTINUE_ON_ERROR env var) */
  "continue-on-error"?: boolean;
  /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
  "copilot"?: boolean;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
  "exclude"?: string[];
  /** Do not destroy protected resources. Destroy all other resources. */
  "exclude-protected"?: boolean;
  "exec-agent"?: string;
  "exec-kind"?: string;
  /** Serialize the destroy diffs, operations, and overall output as JSON */
  "json"?: boolean;
  /** Optional message to associate with the destroy operation */
  "message"?: string;
  /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
  "neo"?: boolean;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** Only show a preview of the destroy, but don't perform the destroy itself */
  "preview-only"?: boolean;
  /** Refresh the state of the stack's resources before this update */
  "refresh"?: string;
  /** Remove the stack and its config file after all resources in the stack have been deleted */
  "remove"?: boolean;
  /** Run the program to determine up-to-date state for providers to destroy resources */
  "run-program"?: boolean;
  /** Show configuration keys and variables */
  "show-config"?: boolean;
  /** Display full length of inputs & outputs */
  "show-full-output"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that don't need to be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** Do not calculate a preview before performing the destroy */
  "skip-preview"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: string;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
  /** Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed. Multiple resources can be specified using: --target urn1 --target urn2. Wildcards (*, **) are also supported */
  "target"?: string[];
  /** Allows destroying of dependent targets discovered but not specified in --target list */
  "target-dependents"?: boolean;
  /** Automatically approve and perform the destroy after previewing it */
  "yes"?: boolean;
}
/** Flags for the `pulumi env` command */
export interface PulumiEnvOptions extends PulumiOptions {
  /** The name of the environment to operate on. */
  "env"?: string;
}
/** Flags for the `pulumi env clone` command */
export interface PulumiEnvCloneOptions extends PulumiEnvOptions {
  /** preserve the same team access on the environment being cloned */
  "preserve-access"?: boolean;
  /** preserve any tags on the environment being cloned */
  "preserve-env-tags"?: boolean;
  /** preserve history of the environment being cloned */
  "preserve-history"?: boolean;
  /** preserve any tags on the environment revisions being cloned */
  "preserve-rev-tags"?: boolean;
}
/** Flags for the `pulumi env diff` command */
export interface PulumiEnvDiffOptions extends PulumiEnvOptions {
  /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell' */
  "format"?: string;
  /** Show the diff for a specific path */
  "path"?: string;
  /** Show static secrets in plaintext rather than ciphertext */
  "show-secrets"?: boolean;
}
/** Flags for the `pulumi env edit` command */
export interface PulumiEnvEditOptions extends PulumiEnvOptions {
  /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
  "draft"?: string;
  /** the command to use to edit the environment definition */
  "editor"?: string;
  /** the file that contains the updated environment, if any. Pass `-` to read from standard input. */
  "file"?: string;
  /** Show static secrets in plaintext rather than ciphertext */
  "show-secrets"?: boolean;
}
/** Flags for the `pulumi env get` command */
export interface PulumiEnvGetOptions extends PulumiEnvOptions {
  /** Set to print just the definition. */
  "definition"?: boolean;
  /** Show static secrets in plaintext rather than ciphertext */
  "show-secrets"?: boolean;
  /** Set to print just the value in the given format. May be 'dotenv', 'json', 'detailed', 'shell' or 'string' */
  "value"?: string;
}
/** Flags for the `pulumi env init` command */
export interface PulumiEnvInitOptions extends PulumiEnvOptions {
  /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
  "file"?: string;
}
/** Flags for the `pulumi env ls` command */
export interface PulumiEnvLsOptions extends PulumiEnvOptions {
  /** Filter returned environments to those in a specific organization */
  "organization"?: string;
  /** Filter returned environments to those in a specific project */
  "project"?: string;
}
/** Flags for the `pulumi env open` command */
export interface PulumiEnvOpenOptions extends PulumiEnvOptions {
  /** open an environment draft with --draft=<change-request-id> */
  "draft"?: string;
  /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', 'shell' or 'string' */
  "format"?: string;
  /** the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m) */
  "lifetime"?: string;
}
/** Flags for the `pulumi env rm` command */
export interface PulumiEnvRmOptions extends PulumiEnvOptions {
  /** Skip confirmation prompts, and proceed with removal anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi env rotate` command */
export interface PulumiEnvRotateOptions extends PulumiEnvOptions {
}
/** Flags for the `pulumi env run` command */
export interface PulumiEnvRunOptions extends PulumiEnvOptions {
  /** open an environment draft with --draft=<change-request-id> */
  "draft"?: string;
  /** true to treat the command as interactive and disable output filters */
  "interactive"?: boolean;
  /** the lifetime of the opened environment */
  "lifetime"?: string;
}
/** Flags for the `pulumi env set` command */
export interface PulumiEnvSetOptions extends PulumiEnvOptions {
  /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
  "draft"?: string;
  /** If set, the value is read from the specified file. Pass `-` to read from standard input. */
  "file"?: string;
  /** true to leave the value in plaintext */
  "plaintext"?: boolean;
  /** true to mark the value as secret */
  "secret"?: boolean;
  /** true to treat the value as a string rather than attempting to parse it as YAML */
  "string"?: boolean;
}
/** Flags for the `pulumi env tag` command */
export interface PulumiEnvTagOptions extends PulumiEnvOptions {
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env tag get` command */
export interface PulumiEnvTagGetOptions extends PulumiEnvTagOptions {
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env tag ls` command */
export interface PulumiEnvTagLsOptions extends PulumiEnvTagOptions {
  /** the command to use to page through the environment's version tags */
  "pager"?: string;
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env tag mv` command */
export interface PulumiEnvTagMvOptions extends PulumiEnvTagOptions {
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env tag rm` command */
export interface PulumiEnvTagRmOptions extends PulumiEnvTagOptions {
}
/** Flags for the `pulumi env version` command */
export interface PulumiEnvVersionOptions extends PulumiEnvOptions {
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env version history` command */
export interface PulumiEnvVersionHistoryOptions extends PulumiEnvVersionOptions {
  /** the command to use to page through the environment's revisions */
  "pager"?: string;
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env version retract` command */
export interface PulumiEnvVersionRetractOptions extends PulumiEnvVersionOptions {
  /** the reason for the retraction */
  "reason"?: string;
  /** the version to use to replace the retracted revision */
  "replace-with"?: string;
}
/** Flags for the `pulumi env version rollback` command */
export interface PulumiEnvVersionRollbackOptions extends PulumiEnvVersionOptions {
  /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
  "draft"?: string;
}
/** Flags for the `pulumi env version tag` command */
export interface PulumiEnvVersionTagOptions extends PulumiEnvVersionOptions {
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env version tag ls` command */
export interface PulumiEnvVersionTagLsOptions extends PulumiEnvVersionTagOptions {
  /** the command to use to page through the environment's version tags */
  "pager"?: string;
  /** display times in UTC */
  "utc"?: boolean;
}
/** Flags for the `pulumi env version tag rm` command */
export interface PulumiEnvVersionTagRmOptions extends PulumiEnvVersionTagOptions {
}
/** Flags for the `pulumi gen-completion` command */
export interface PulumiGenCompletionOptions extends PulumiOptions {
}
/** Flags for the `pulumi gen-markdown` command */
export interface PulumiGenMarkdownOptions extends PulumiOptions {
}
/** Flags for the `pulumi generate-cli-spec` command */
export interface PulumiGenerateCliSpecOptions extends PulumiOptions {
  /** help for generate-cli-spec */
  "help"?: boolean;
}
/** Flags for the `pulumi help` command */
export interface PulumiHelpOptions extends PulumiOptions {
}
/** Flags for the `pulumi import` command */
export interface PulumiImportOptions extends PulumiOptions {
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  "exec-agent"?: string;
  "exec-kind"?: string;
  /** The path to a JSON-encoded file containing a list of resources to import */
  "file"?: string;
  /** Invoke a converter to import the resources */
  "from"?: string;
  /** Generate resource declaration code for the imported resources */
  "generate-code"?: boolean;
  /** When used with --from, always write a JSON-encoded file containing a list of importable resources discovered by conversion to the specified path */
  "generate-resources"?: string;
  /** Serialize the import diffs, operations, and overall output as JSON */
  "json"?: boolean;
  /** Optional message to associate with the update operation */
  "message"?: string;
  /** The path to the file that will contain the generated resource declarations */
  "out"?: string;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** The name and URN of the parent resource in the format name=urn, where name is the variable name of the parent resource */
  "parent"?: string;
  /** Only show a preview of the import, but don't perform the import itself */
  "preview-only"?: boolean;
  /** The property names to use for the import in the format name1,name2 */
  "properties"?: string[];
  /** Allow resources to be imported with protection from deletion enabled */
  "protect"?: boolean;
  /** The name and URN of the provider to use for the import in the format name=urn, where name is the variable name for the provider resource */
  "provider"?: string;
  /** Do not calculate a preview before performing the import */
  "skip-preview"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: string;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
  /** Automatically approve and perform the import after previewing it */
  "yes"?: boolean;
}
/** Flags for the `pulumi install` command */
export interface PulumiInstallOptions extends PulumiOptions {
  /** Skip installing dependencies */
  "no-dependencies"?: boolean;
  /** Skip installing plugins */
  "no-plugins"?: boolean;
  /** The max number of concurrent installs to perform. Parallelism of less then 1 implies unbounded parallelism */
  "parallel"?: number;
  /** Reinstall a plugin even if it already exists */
  "reinstall"?: boolean;
  /** Use language version tools to setup and install the language runtime */
  "use-language-version-tools"?: boolean;
}
/** Flags for the `pulumi login` command */
export interface PulumiLoginOptions extends PulumiOptions {
  /** A cloud URL to log in to */
  "cloud-url"?: string;
  /** A default org to associate with the login. Please note, currently, only the managed and self-hosted backends support organizations */
  "default-org"?: string;
  /** Allow insecure server connections when using SSL */
  "insecure"?: boolean;
  /** Show interactive login options based on known accounts */
  "interactive"?: boolean;
  /** Use Pulumi in local-only mode */
  "local"?: boolean;
  /** The expiration for the cloud backend access token in duration format (e.g. '15m', '24h') */
  "oidc-expiration"?: string;
  /** The organization to use for OIDC token exchange audience */
  "oidc-org"?: string;
  /** The team when exchanging for a team token */
  "oidc-team"?: string;
  /** An OIDC token to exchange for a cloud backend access token. Can be either a raw token or a file path prefixed with 'file://'. */
  "oidc-token"?: string;
  /** The user when exchanging for a personal token */
  "oidc-user"?: string;
}
/** Flags for the `pulumi logout` command */
export interface PulumiLogoutOptions extends PulumiOptions {
  /** Logout of all backends */
  "all"?: boolean;
  /** A cloud URL to log out of (defaults to current cloud) */
  "cloud-url"?: string;
  /** Log out of using local mode */
  "local"?: boolean;
}
/** Flags for the `pulumi logs` command */
export interface PulumiLogsOptions extends PulumiOptions {
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Follow the log stream in real time (like tail -f) */
  "follow"?: boolean;
  /** Emit output as JSON */
  "json"?: boolean;
  /** Only return logs for the requested resource ('name', 'type::name' or full URN).  Defaults to returning all logs. */
  "resource"?: string;
  /** Only return logs newer than a relative duration ('5s', '2m', '3h') or absolute timestamp.  Defaults to returning the last 1 hour of logs. */
  "since"?: string;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi new` command */
export interface PulumiNewOptions extends PulumiOptions {
  /** Prompt to use for Pulumi AI */
  "ai"?: string;
  /** Config to save */
  "config"?: string[];
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** The project description; if not specified, a prompt will request it */
  "description"?: string;
  /** The location to place the generated project; if not specified, the current directory is used */
  "dir"?: string;
  /** Forces content to be generated even if it would change existing files */
  "force"?: boolean;
  /** Generate the project only; do not create a stack, save config, or install dependencies */
  "generate-only"?: boolean;
  /** Language to use for Pulumi AI (must be one of TypeScript, JavaScript, Python, Go, C#, Java, or YAML) */
  "language"?: string;
  /** List locally installed templates and exit */
  "list-templates"?: boolean;
  /** The project name; if not specified, a prompt will request it */
  "name"?: string;
  /** Use locally cached templates without making any network requests */
  "offline"?: boolean;
  /** Store stack configuration remotely */
  "remote-stack-config"?: boolean;
  /** Additional options for the language runtime (format: key1=value1,key2=value2) */
  "runtime-options"?: string[];
  /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
  "secrets-provider"?: string;
  /** The stack name; either an existing stack or stack to create; if not specified, a prompt will request it */
  "stack"?: string;
  /** Run in template mode, which will skip prompting for AI or Template functionality */
  "template-mode"?: boolean;
  /** Skip prompts and proceed with default values */
  "yes"?: boolean;
}
/** Flags for the `pulumi org` command */
export interface PulumiOrgOptions extends PulumiOptions {
}
/** Flags for the `pulumi org get-default` command */
export interface PulumiOrgGetDefaultOptions extends PulumiOrgOptions {
}
/** Flags for the `pulumi org search` command */
export interface PulumiOrgSearchOptions extends PulumiOrgOptions {
  /** Delimiter to use when rendering CSV output. */
  "delimiter"?: string;
  /** Name of the organization to search. Defaults to the current user's default organization. */
  "org"?: string;
  /** Output format. Supported formats are 'table', 'json', 'csv', and 'yaml'. */
  "output"?: string;
  /** A Pulumi Query to send to Pulumi Cloud for resource search.May be formatted as a single query, or multiple: 	-q "type:aws:s3/bucketv2:BucketV2 modified:>=2023-09-01" 	-q "type:aws:s3/bucketv2:BucketV2" -q "modified:>=2023-09-01" See https://www.pulumi.com/docs/pulumi-cloud/insights/search/#query-syntax for syntax reference. */
  "query"?: string[];
  /** Open the search results in a web browser. */
  "web"?: boolean;
}
/** Flags for the `pulumi org search ai` command */
export interface PulumiOrgSearchAiOptions extends PulumiOrgSearchOptions {
  /** Delimiter to use when rendering CSV output. */
  "delimiter"?: string;
  /** Organization name to search within */
  "org"?: string;
  /** Output format. Supported formats are 'table', 'json', 'csv' and 'yaml'. */
  "output"?: string;
  /** Plaintext natural language query */
  "query"?: string;
  /** Open the search results in a web browser. */
  "web"?: boolean;
}
/** Flags for the `pulumi org set-default` command */
export interface PulumiOrgSetDefaultOptions extends PulumiOrgOptions {
}
/** Flags for the `pulumi package` command */
export interface PulumiPackageOptions extends PulumiOptions {
}
/** Flags for the `pulumi package add` command */
export interface PulumiPackageAddOptions extends PulumiPackageOptions {
}
/** Flags for the `pulumi package delete` command */
export interface PulumiPackageDeleteOptions extends PulumiPackageOptions {
  /** Skip confirmation prompts, and proceed with deletion anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi package gen-sdk` command */
export interface PulumiPackageGenSdkOptions extends PulumiPackageOptions {
  /** The SDK language to generate: [nodejs|python|go|dotnet|java|all] */
  "language"?: string;
  /** Generate an SDK appropriate for local usage */
  "local"?: boolean;
  /** The directory to write the SDK to */
  "out"?: string;
  /** A folder of extra overlay files to copy to the generated SDK */
  "overlays"?: string;
  /** The provider plugin version to generate the SDK for */
  "version"?: string;
}
/** Flags for the `pulumi package get-mapping` command */
export interface PulumiPackageGetMappingOptions extends PulumiPackageOptions {
  /** The file to write the mapping data to */
  "out"?: string;
}
/** Flags for the `pulumi package get-schema` command */
export interface PulumiPackageGetSchemaOptions extends PulumiPackageOptions {
}
/** Flags for the `pulumi package info` command */
export interface PulumiPackageInfoOptions extends PulumiPackageOptions {
  /** Function name */
  "function"?: string;
  /** Module name */
  "module"?: string;
  /** Resource name */
  "resource"?: string;
}
/** Flags for the `pulumi package pack-sdk` command */
export interface PulumiPackagePackSdkOptions extends PulumiPackageOptions {
}
/** Flags for the `pulumi package publish` command */
export interface PulumiPackagePublishOptions extends PulumiPackageOptions {
  /** Path to the installation configuration markdown file */
  "installation-configuration"?: string;
  /** The publisher of the package (e.g., 'pulumi'). Defaults to the publisher set in the package schema or the default organization in your pulumi config. */
  "publisher"?: string;
  /** Path to the package readme/index markdown file */
  "readme"?: string;
  /** The origin of the package (e.g., 'pulumi', 'private', 'opentofu'). Defaults to 'private'. */
  "source"?: string;
}
/** Flags for the `pulumi package publish-sdk` command */
export interface PulumiPackagePublishSdkOptions extends PulumiPackageOptions {
  /** The path to the root of your package. 	Example: ./sdk/nodejs */
  "path"?: string;
}
/** Flags for the `pulumi plugin` command */
export interface PulumiPluginOptions extends PulumiOptions {
}
/** Flags for the `pulumi plugin install` command */
export interface PulumiPluginInstallOptions extends PulumiPluginOptions {
  /** The expected SHA256 checksum for the plugin archive */
  "checksum"?: string;
  /** Force installation of an exact version match (usually >= is accepted) */
  "exact"?: boolean;
  /** Install a plugin from a binary, folder or tarball, instead of downloading it */
  "file"?: string;
  /** Reinstall a plugin even if it already exists */
  "reinstall"?: boolean;
  /** A URL to download plugins from */
  "server"?: string;
}
/** Flags for the `pulumi plugin ls` command */
export interface PulumiPluginLsOptions extends PulumiPluginOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** List only the plugins used by the current project */
  "project"?: boolean;
}
/** Flags for the `pulumi plugin rm` command */
export interface PulumiPluginRmOptions extends PulumiPluginOptions {
  /** Remove all plugins */
  "all"?: boolean;
  /** Skip confirmation prompts, and proceed with removal anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi plugin run` command */
export interface PulumiPluginRunOptions extends PulumiPluginOptions {
  /** The plugin kind */
  "kind"?: string;
}
/** Flags for the `pulumi policy` command */
export interface PulumiPolicyOptions extends PulumiOptions {
}
/** Flags for the `pulumi policy disable` command */
export interface PulumiPolicyDisableOptions extends PulumiPolicyOptions {
  /** The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used */
  "policy-group"?: string;
  /** The version of the Policy Pack that will be disabled; if not specified, any enabled version of the Policy Pack will be disabled */
  "version"?: string;
}
/** Flags for the `pulumi policy enable` command */
export interface PulumiPolicyEnableOptions extends PulumiPolicyOptions {
  /** The file path for the Policy Pack configuration file */
  "config"?: string;
  /** The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used */
  "policy-group"?: string;
}
/** Flags for the `pulumi policy group` command */
export interface PulumiPolicyGroupOptions extends PulumiPolicyOptions {
}
/** Flags for the `pulumi policy group ls` command */
export interface PulumiPolicyGroupLsOptions extends PulumiPolicyGroupOptions {
  /** Emit output as JSON */
  "json"?: boolean;
}
/** Flags for the `pulumi policy ls` command */
export interface PulumiPolicyLsOptions extends PulumiPolicyOptions {
  /** Emit output as JSON */
  "json"?: boolean;
}
/** Flags for the `pulumi policy new` command */
export interface PulumiPolicyNewOptions extends PulumiPolicyOptions {
  /** The location to place the generated Policy Pack; if not specified, the current directory is used */
  "dir"?: string;
  /** Forces content to be generated even if it would change existing files */
  "force"?: boolean;
  /** Generate the Policy Pack only; do not install dependencies */
  "generate-only"?: boolean;
  /** Use locally cached templates without making any network requests */
  "offline"?: boolean;
}
/** Flags for the `pulumi policy publish` command */
export interface PulumiPolicyPublishOptions extends PulumiPolicyOptions {
}
/** Flags for the `pulumi policy rm` command */
export interface PulumiPolicyRmOptions extends PulumiPolicyOptions {
  /** Skip confirmation prompts, and proceed with removal anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi policy validate-config` command */
export interface PulumiPolicyValidateConfigOptions extends PulumiPolicyOptions {
  /** The file path for the Policy Pack configuration file */
  "config"?: string;
}
/** Flags for the `pulumi preview` command */
export interface PulumiPreviewOptions extends PulumiOptions {
  /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
  "attach-debugger"?: string[];
  /** The address of an existing language runtime host to connect to */
  "client"?: string;
  /** Config to use during the preview and save to the stack config file */
  "config"?: string[];
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
  "copilot"?: boolean;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
  "exclude"?: string[];
  /** Allow ignoring of dependent targets discovered but not specified in --exclude list */
  "exclude-dependents"?: boolean;
  "exec-agent"?: string;
  "exec-kind"?: string;
  /** Return an error if any changes are proposed by this preview */
  "expect-no-changes"?: boolean;
  /** Save any creates seen during the preview into an import file to use with 'pulumi import' */
  "import-file"?: string;
  /** Serialize the preview diffs, operations, and overall output as JSON. Set PULUMI_ENABLE_STREAMING_JSON_PREVIEW to stream JSON events instead. */
  "json"?: boolean;
  /** Optional message to associate with the preview operation */
  "message"?: string;
  /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
  "neo"?: boolean;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** Run one or more policy packs as part of this update */
  "policy-pack"?: string[];
  /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
  "policy-pack-config"?: string[];
  /** Refresh the state of the stack's resources before this update */
  "refresh"?: string;
  /** Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2 */
  "replace"?: string[];
  /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
  "run-program"?: boolean;
  /** [PREVIEW] Save the operations proposed by the preview to a plan file at the given path */
  "save-plan"?: string;
  /** Show configuration keys and variables */
  "show-config"?: boolean;
  /** Display full length of inputs & outputs */
  "show-full-output"?: boolean;
  /** Show per-resource policy remediation details instead of a summary */
  "show-policy-remediations"?: boolean;
  /** Show resources that are being read in, alongside those being managed directly in the stack */
  "show-reads"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that needn't be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** Show secrets in plaintext in the CLI output, if used with --save-plan the secrets will also be shown in the plan file. Defaults to `false` */
  "show-secrets"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: string;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
  /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2 */
  "target"?: string[];
  /** Allow updating of dependent targets discovered but not specified in --target list */
  "target-dependents"?: boolean;
  /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
  "target-replace"?: string[];
}
/** Flags for the `pulumi project` command */
export interface PulumiProjectOptions extends PulumiOptions {
}
/** Flags for the `pulumi project ls` command */
export interface PulumiProjectLsOptions extends PulumiProjectOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** The organization whose projects to list */
  "organization"?: string;
}
/** Flags for the `pulumi refresh` command */
export interface PulumiRefreshOptions extends PulumiOptions {
  /** Clear all pending creates, dropping them from the state */
  "clear-pending-creates"?: boolean;
  /** The address of an existing language runtime host to connect to */
  "client"?: string;
  /** Config to use during the refresh and save to the stack config file */
  "config"?: string[];
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
  "copilot"?: boolean;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  /** Specify a resource URN to ignore. These resources will not be refreshed. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
  "exclude"?: string[];
  /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
  "exclude-dependents"?: boolean;
  "exec-agent"?: string;
  "exec-kind"?: string;
  /** Return an error if any changes occur during this refresh. This check happens after the refresh is applied */
  "expect-no-changes"?: boolean;
  /** A list of form [[URN ID]...] describing the provider IDs of pending creates */
  "import-pending-creates"?: string[];
  /** Serialize the refresh diffs, operations, and overall output as JSON */
  "json"?: boolean;
  /** Optional message to associate with the update operation */
  "message"?: string;
  /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
  "neo"?: boolean;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** Only show a preview of the refresh, but don't perform the refresh itself */
  "preview-only"?: boolean;
  /** Run the program to determine up-to-date state for providers to refresh resources */
  "run-program"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that needn't be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** Skip importing pending creates in interactive mode */
  "skip-pending-creates"?: boolean;
  /** Do not calculate a preview before performing the refresh */
  "skip-preview"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: string;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
  /** Specify a single resource URN to refresh. Multiple resource can be specified using: --target urn1 --target urn2 */
  "target"?: string[];
  /** Allows updating of dependent targets discovered but not specified in --target list */
  "target-dependents"?: boolean;
  /** Automatically approve and perform the refresh after previewing it */
  "yes"?: boolean;
}
/** Flags for the `pulumi replay-events` command */
export interface PulumiReplayEventsOptions extends PulumiOptions {
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Delay display by the given duration. Useful for attaching a debugger. */
  "delay"?: string;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  /** Serialize the preview diffs, operations, and overall output as JSON */
  "json"?: boolean;
  /** Delay each event by the given duration. */
  "period"?: string;
  /** Must be set for events from a `pulumi preview`. */
  "preview"?: boolean;
  /** Show configuration keys and variables */
  "show-config"?: boolean;
  /** Show resources that are being read in, alongside those being managed directly in the stack */
  "show-reads"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that needn't be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
}
/** Flags for the `pulumi schema` command */
export interface PulumiSchemaOptions extends PulumiOptions {
}
/** Flags for the `pulumi schema check` command */
export interface PulumiSchemaCheckOptions extends PulumiSchemaOptions {
  /** Whether references to nonexistent types should be considered errors */
  "allow-dangling-references"?: boolean;
}
/** Flags for the `pulumi stack` command */
export interface PulumiStackOptions extends PulumiOptions {
  /** Display each resource's provider-assigned unique ID */
  "show-ids"?: boolean;
  /** Display only the stack name */
  "show-name"?: boolean;
  /** Display stack outputs which are marked as secret in plaintext */
  "show-secrets"?: boolean;
  /** Display each resource's Pulumi-assigned globally unique URN */
  "show-urns"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack change-secrets-provider` command */
export interface PulumiStackChangeSecretsProviderOptions extends PulumiStackOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack export` command */
export interface PulumiStackExportOptions extends PulumiStackOptions {
  /** A filename to write stack output to */
  "file"?: string;
  /** Emit secrets in plaintext in exported stack. Defaults to `false` */
  "show-secrets"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Previous stack version to export. (If unset, will export the latest.) */
  "version"?: string;
}
/** Flags for the `pulumi stack graph` command */
export interface PulumiStackGraphOptions extends PulumiStackOptions {
  /** Sets the color of dependency edges in the graph */
  "dependency-edge-color"?: string;
  /** An optional DOT fragment that will be inserted at the top of the digraph element. This can be used for styling the graph elements, setting graph properties etc. */
  "dot-fragment"?: string;
  /** Ignores edges introduced by dependency resource relationships */
  "ignore-dependency-edges"?: boolean;
  /** Ignores edges introduced by parent/child resource relationships */
  "ignore-parent-edges"?: boolean;
  /** Sets the color of parent edges in the graph */
  "parent-edge-color"?: string;
  /** Sets the resource name as the node label for each node of the graph */
  "short-node-name"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack history` command */
export interface PulumiStackHistoryOptions extends PulumiStackOptions {
  /** Show full dates, instead of relative dates */
  "full-dates"?: boolean;
  /** Emit output as JSON */
  "json"?: boolean;
  /** Used with 'page-size' to paginate results */
  "page"?: number;
  /** Used with 'page' to control number of results returned */
  "page-size"?: number;
  /** Show secret values when listing config instead of displaying blinded values */
  "show-secrets"?: boolean;
  /** Choose a stack other than the currently selected one */
  "stack"?: string;
}
/** Flags for the `pulumi stack import` command */
export interface PulumiStackImportOptions extends PulumiStackOptions {
  /** A filename to read stack input from */
  "file"?: string;
  /** Force the import to occur, even if apparent errors are discovered beforehand (not recommended) */
  "force"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack init` command */
export interface PulumiStackInitOptions extends PulumiStackOptions {
  /** The name of the stack to copy existing config from */
  "copy-config-from"?: string;
  /** Do not select the stack */
  "no-select"?: boolean;
  /** Store stack configuration remotely */
  "remote-config"?: boolean;
  /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
  "secrets-provider"?: string;
  /** The name of the stack to create */
  "stack"?: string;
  /** A list of team names that should have permission to read and update this stack, once created */
  "teams"?: string[];
}
/** Flags for the `pulumi stack ls` command */
export interface PulumiStackLsOptions extends PulumiStackOptions {
  /** List all stacks instead of just stacks for the current project */
  "all"?: boolean;
  /** Emit output as JSON */
  "json"?: boolean;
  /** Filter returned stacks to those in a specific organization */
  "organization"?: string;
  /** Filter returned stacks to those with a specific project name */
  "project"?: string;
  /** Filter returned stacks to those in a specific tag (tag-name or tag-name=tag-value) */
  "tag"?: string;
}
/** Flags for the `pulumi stack output` command */
export interface PulumiStackOutputOptions extends PulumiStackOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** Emit output as a shell script */
  "shell"?: boolean;
  /** Display outputs which are marked as secret in plaintext */
  "show-secrets"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack rename` command */
export interface PulumiStackRenameOptions extends PulumiStackOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack rm` command */
export interface PulumiStackRmOptions extends PulumiStackOptions {
  /** Forces deletion of the stack, leaving behind any resources managed by the stack */
  "force"?: boolean;
  /** Do not delete the corresponding Pulumi.<stack-name>.yaml configuration file for the stack */
  "preserve-config"?: boolean;
  /** Additionally remove backups of the stack, if using the DIY backend */
  "remove-backups"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts, and proceed with removal anyway */
  "yes"?: boolean;
}
/** Flags for the `pulumi stack select` command */
export interface PulumiStackSelectOptions extends PulumiStackOptions {
  /** If selected stack does not exist, create it */
  "create"?: boolean;
  /** Use with --create flag, The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
  "secrets-provider"?: string;
  /** The name of the stack to select */
  "stack"?: string;
}
/** Flags for the `pulumi stack tag` command */
export interface PulumiStackTagOptions extends PulumiStackOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi stack tag get` command */
export interface PulumiStackTagGetOptions extends PulumiStackTagOptions {
}
/** Flags for the `pulumi stack tag ls` command */
export interface PulumiStackTagLsOptions extends PulumiStackTagOptions {
  /** Emit output as JSON */
  "json"?: boolean;
}
/** Flags for the `pulumi stack tag rm` command */
export interface PulumiStackTagRmOptions extends PulumiStackTagOptions {
}
/** Flags for the `pulumi stack tag set` command */
export interface PulumiStackTagSetOptions extends PulumiStackTagOptions {
}
/** Flags for the `pulumi stack unselect` command */
export interface PulumiStackUnselectOptions extends PulumiStackOptions {
}
/** Flags for the `pulumi state` command */
export interface PulumiStateOptions extends PulumiOptions {
}
/** Flags for the `pulumi state delete` command */
export interface PulumiStateDeleteOptions extends PulumiStateOptions {
  /** Delete all resources in the stack */
  "all"?: boolean;
  /** Force deletion of protected resources */
  "force"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Delete the URN and all its dependents */
  "target-dependents"?: boolean;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state edit` command */
export interface PulumiStateEditOptions extends PulumiStateOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi state move` command */
export interface PulumiStateMoveOptions extends PulumiStateOptions {
  /** The name of the stack to move resources to */
  "dest"?: string;
  /** Include all the parents of the moved resources as well */
  "include-parents"?: boolean;
  /** The name of the stack to move resources from */
  "source"?: string;
  /** Automatically approve and perform the move */
  "yes"?: boolean;
}
/** Flags for the `pulumi state protect` command */
export interface PulumiStateProtectOptions extends PulumiStateOptions {
  /** Protect all resources in the checkpoint */
  "all"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state rename` command */
export interface PulumiStateRenameOptions extends PulumiStateOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state repair` command */
export interface PulumiStateRepairOptions extends PulumiStateOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Automatically approve and perform the repair */
  "yes"?: boolean;
}
/** Flags for the `pulumi state taint` command */
export interface PulumiStateTaintOptions extends PulumiStateOptions {
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state unprotect` command */
export interface PulumiStateUnprotectOptions extends PulumiStateOptions {
  /** Unprotect all resources in the checkpoint */
  "all"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state untaint` command */
export interface PulumiStateUntaintOptions extends PulumiStateOptions {
  /** Untaint all resources in the checkpoint */
  "all"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** Skip confirmation prompts */
  "yes"?: boolean;
}
/** Flags for the `pulumi state upgrade` command */
export interface PulumiStateUpgradeOptions extends PulumiStateOptions {
  /** Automatically approve and perform the upgrade */
  "yes"?: boolean;
}
/** Flags for the `pulumi template` command */
export interface PulumiTemplateOptions extends PulumiOptions {
}
/** Flags for the `pulumi template publish` command */
export interface PulumiTemplatePublishOptions extends PulumiTemplateOptions {
  /** The name of the template (required) */
  "name"?: string;
  /** The publisher of the template (e.g., 'pulumi'). Defaults to the default organization in your pulumi config. */
  "publisher"?: string;
  /** The version of the template (required, semver format) */
  "version"?: string;
}
/** Flags for the `pulumi up` command */
export interface PulumiUpOptions extends PulumiOptions {
  /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
  "attach-debugger"?: string[];
  /** The address of an existing language runtime host to connect to */
  "client"?: string;
  /** Config to use during the update and save to the stack config file */
  "config"?: string[];
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** Continue updating resources even if an error is encountered (can also be set with PULUMI_CONTINUE_ON_ERROR environment variable) */
  "continue-on-error"?: boolean;
  /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
  "copilot"?: boolean;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  /** Display operation as a rich diff showing the overall change */
  "diff"?: boolean;
  /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
  "exclude"?: string[];
  /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
  "exclude-dependents"?: boolean;
  "exec-agent"?: string;
  "exec-kind"?: string;
  /** Return an error if any changes occur during this update. This check happens after the update is applied */
  "expect-no-changes"?: boolean;
  /** Serialize the update diffs, operations, and overall output as JSON */
  "json"?: boolean;
  /** Optional message to associate with the update operation */
  "message"?: string;
  /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
  "neo"?: boolean;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** [EXPERIMENTAL] Path to a plan file to use for the update. The update will not perform operations that exceed its plan (e.g. replacements instead of updates, or updates insteadof sames). */
  "plan"?: string;
  /** Run one or more policy packs as part of this update */
  "policy-pack"?: string[];
  /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
  "policy-pack-config"?: string[];
  /** Refresh the state of the stack's resources before this update */
  "refresh"?: string;
  /** Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2. Wildcards (*, **) are also supported */
  "replace"?: string[];
  /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
  "run-program"?: boolean;
  /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
  "secrets-provider"?: string;
  /** Show configuration keys and variables */
  "show-config"?: boolean;
  /** Display full length of inputs & outputs */
  "show-full-output"?: boolean;
  /** Show per-resource policy remediation details instead of a summary */
  "show-policy-remediations"?: boolean;
  /** Show resources that are being read in, alongside those being managed directly in the stack */
  "show-reads"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that don't need be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** Show secret outputs in the CLI output */
  "show-secrets"?: boolean;
  /** Do not calculate a preview before performing the update */
  "skip-preview"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
  /** [EXPERIMENTAL] Enable strict plan behavior: generate a plan during preview and constrain the update to that plan (opt-in). Cannot be used with --skip-preview. */
  "strict"?: boolean;
  /** Suppress display of stack outputs (in case they contain sensitive values) */
  "suppress-outputs"?: boolean;
  /** Suppress display of the state permalink */
  "suppress-permalink"?: string;
  /** Suppress display of periodic progress dots */
  "suppress-progress"?: boolean;
  /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2. Wildcards (*, **) are also supported */
  "target"?: string[];
  /** Allows updating of dependent targets discovered but not specified in --target list */
  "target-dependents"?: boolean;
  /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
  "target-replace"?: string[];
  /** Automatically approve and perform the update after previewing it */
  "yes"?: boolean;
}
/** Flags for the `pulumi version` command */
export interface PulumiVersionOptions extends PulumiOptions {
}
/** Flags for the `pulumi view-trace` command */
export interface PulumiViewTraceOptions extends PulumiOptions {
  /** the port the trace viewer will listen on */
  "port"?: number;
}
/** Flags for the `pulumi watch` command */
export interface PulumiWatchOptions extends PulumiOptions {
  /** Config to use during the update */
  "config"?: string[];
  /** Use the configuration values in the specified file rather than detecting the file name */
  "config-file"?: string;
  /** Config keys contain a path to a property in a map or list to set */
  "config-path"?: boolean;
  /** Print detailed debugging output during resource operations */
  "debug"?: boolean;
  "exec-kind"?: string;
  /** Optional message to associate with each update operation */
  "message"?: string;
  /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
  "parallel"?: number;
  /** Specify one or more relative or absolute paths that need to be watched. A path can point to a folder or a file. Defaults to working directory */
  "path"?: string[];
  /** Run one or more policy packs as part of each update */
  "policy-pack"?: string[];
  /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
  "policy-pack-config"?: string[];
  /** Refresh the state of the stack's resources before each update */
  "refresh"?: boolean;
  /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
  "secrets-provider"?: string;
  /** Show configuration keys and variables */
  "show-config"?: boolean;
  /** Show detailed resource replacement creates and deletes instead of a single step */
  "show-replacement-steps"?: boolean;
  /** Show resources that don't need be updated because they haven't changed, alongside those that do */
  "show-sames"?: boolean;
  /** The name of the stack to operate on. Defaults to the current stack */
  "stack"?: string;
}
/** Flags for the `pulumi whoami` command */
export interface PulumiWhoamiOptions extends PulumiOptions {
  /** Emit output as JSON */
  "json"?: boolean;
  /** Print detailed whoami information */
  "verbose"?: boolean;
}
