import { Output, PulumiFn, execute, inline } from "./utilities";

export interface Options {
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

export interface AboutOptions extends Options {
    /** Emit output as JSON */
    json?: boolean;
    /** The name of the stack to get info on. Defaults to the current stack */
    stack?: string;
    /** Include transitive dependencies */
    transitive?: boolean;
}

export interface AboutEnvOptions extends AboutOptions {
}

/** An overview of the environmental variables used by pulumi */
export function aboutEnv(options?: AboutEnvOptions): Promise<Output> {
    const __args = []
    __args.push('about')
    __args.push('env')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface AiOptions extends Options {
}

export interface AiWebOptions extends AiOptions {
    /** Language to use for the prompt - this defaults to TypeScript. [TypeScript, Python, Go, C#, Java, YAML] */
    language?: string;
    /** Opt-out of automatically submitting the prompt to Pulumi AI */
    noAutoSubmit?: boolean;
}

/**
 * Opens Pulumi AI in your local browser
 *
 * This command opens the Pulumi AI web app in your local default browser.
 * It can be further initialized by providing a prompt to pre-fill in the app,
 * with the default behavior then automatically submitting that prompt to Pulumi AI.
 *
 * If you do not want to submit the prompt to Pulumi AI, you can opt-out of this
 * by passing the --no-auto-submit flag.
 *
 * Example:
 *   pulumi ai web "Create an S3 bucket in Python"
 */
export function aiWeb(prompt: string, options?: AiWebOptions): Promise<Output> {
    const __args = []
    __args.push('ai')
    __args.push('web')
    if (prompt != null) {
        __args.push(prompt)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface CancelOptions extends Options {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts, and proceed with cancellation anyway */
    yes?: boolean;
}

/**
 * Cancel a stack's currently running update, if any.
 *
 * This command cancels the update currently being applied to a stack if any exists.
 * Note that this operation is _very dangerous_, and may leave the stack in an
 * inconsistent state if a resource operation was pending when the update was canceled.
 *
 * After this command completes successfully, the stack will be ready for further
 * updates.
 */
export function cancel(stackName: string, options?: CancelOptions): Promise<Output> {
    const __args = []
    __args.push('cancel')
    if (stackName != null) {
        __args.push(stackName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigOptions extends Options {
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

export interface ConfigCpOptions extends ConfigOptions {
    /** The name of the new stack to copy the config to */
    dest?: string;
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
}

/**
 * Copies the config from the current stack to the destination stack. If `key` is omitted,
 * then all of the config from the current stack will be copied to the destination stack.
 */
export function configCp(key: string, options?: ConfigCpOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('cp')
    if (key != null) {
        __args.push(key)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigEnvOptions extends ConfigOptions {
}

export interface ConfigEnvAddOptions extends ConfigEnvOptions {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/**
 * Adds environments to the end of a stack's import list. Imported environments are merged in order
 * per the ESC merge rules. The list of stacks behaves as if it were the import list in an anonymous
 * environment.
 */
export function configEnvAdd(environmentNames: string[], options?: ConfigEnvAddOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('env')
    __args.push('add')
    __args.push(... environmentNames)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigEnvInitOptions extends ConfigEnvOptions {
    /** The name of the environment to create. Defaults to "<project name>/<stack name>" */
    env?: string;
    /** Do not remove configuration values from the stack after creating the environment */
    keepConfig?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save the created environment without prompting */
    yes?: boolean;
}

/**
 * Creates an environment for a specific stack based on the stack's configuration values,
 * then replaces the stack's configuration values with a reference to that environment.
 * The environment will be created in the same organization as the stack.
 */
export function configEnvInit(options?: ConfigEnvInitOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('env')
    __args.push('init')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigEnvLsOptions extends ConfigEnvOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** Lists the environments imported into a stack's configuration. */
export function configEnvLs(options?: ConfigEnvLsOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('env')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigEnvRmOptions extends ConfigEnvOptions {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/** Removes an environment from a stack's import list. */
export function configEnvRm(environmentName: string, options?: ConfigEnvRmOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('env')
    __args.push('rm')
    __args.push(environmentName)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigGetOptions extends ConfigOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration */
    open?: boolean;
    /** The key contains a path to a property in a map or list to get */
    path?: boolean;
}

/**
 * Get a single configuration value.
 *
 * The `--path` flag can be used to get a value inside a map or list:
 *
 *   - `pulumi config get --path outer.inner` will get the value of the `inner` key, if the value of `outer` is a map `inner: value`.
 *   - `pulumi config get --path 'names[0]'` will get the value of the first item, if the value of `names` is a list.
 */
export function configGet(key: string, options?: ConfigGetOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('get')
    __args.push(key)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigRefreshOptions extends ConfigOptions {
    /** Overwrite configuration file, if it exists, without creating a backup */
    force?: boolean;
}

/** Update the local configuration based on the most recent deployment of the stack */
export function configRefresh(options?: ConfigRefreshOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('refresh')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigRmOptions extends ConfigOptions {
    /** The key contains a path to a property in a map or list to remove */
    path?: boolean;
}

/**
 * Remove configuration value.
 *
 * The `--path` flag can be used to remove a value inside a map or list:
 *
 *   - `pulumi config rm --path outer.inner` will remove the `inner` key, if the value of `outer` is a map `inner: value`.
 *   - `pulumi config rm --path 'names[0]'` will remove the first item, if the value of `names` is a list.
 */
export function configRm(key: string, options?: ConfigRmOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('rm')
    __args.push(key)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigRmAllOptions extends ConfigOptions {
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
}

/**
 * Remove multiple configuration values.
 *
 * The `--path` flag indicates that keys should be parsed within maps or lists:
 *
 *   - `pulumi config rm-all --path  outer.inner 'foo[0]' key1` will remove the 
 *     `inner` key of the `outer` map, the first key of the `foo` list and `key1`.
 *   - `pulumi config rm-all outer.inner 'foo[0]' key1` will remove the literal    `outer.inner`, `foo[0]` and `key1` keys
 */
export function configRmAll(keys: string[], options?: ConfigRmAllOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('rm-all')
    __args.push(... keys)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigSetOptions extends ConfigOptions {
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
    /** Save the value as plaintext (unencrypted) */
    plaintext?: boolean;
    /** Encrypt the value instead of storing it in plaintext */
    secret?: boolean;
    /** Save the value as the given type.  Allowed values are string, bool, int, and float */
    type?: string;
}

/**
 * Configuration values can be accessed when a stack is being deployed and used to configure behavior. 
 * If a value is not present on the command line, pulumi will prompt for the value. Multi-line values
 * may be set by piping a file to standard in.
 *
 * The `--path` flag can be used to set a value inside a map or list:
 *
 *   - `pulumi config set --path 'names[0]' a` will set the value to a list with the first item `a`.
 *   - `pulumi config set --path parent.nested value` will set the value of `parent` to a map `nested: value`.
 *   - `pulumi config set --path '["parent.name"]["nested.name"]' value` will set the value of 
 *     `parent.name` to a map `nested.name: value`.
 *
 * When setting the config for a path, "true" and "false" are treated as boolean values, and
 * integers are treated as numbers. All other values are treated as strings.  Top level entries
 * are always treated as strings.
 */
export function configSet(key: string, value: string, options?: ConfigSetOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('set')
    __args.push(key)
    if (value != null) {
        __args.push(value)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConfigSetAllOptions extends ConfigOptions {
    /** Read values from a JSON string in the format produced by 'pulumi config --json' */
    json?: string;
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
    /** Marks a value as plaintext (unencrypted) */
    plaintext?: string;
    /** Marks a value as secret to be encrypted */
    secret?: string;
}

/**
 * pulumi set-all allows you to set multiple configuration values in one command.
 *
 * Each key-value pair must be preceded by either the `--secret` or the `--plaintext` flag to denote whether 
 * it should be encrypted:
 *
 *   - `pulumi config set-all --secret key1=value1 --plaintext key2=value --secret key3=value3`
 *
 * The `--path` flag can be used to set values inside a map or list:
 *
 *   - `pulumi config set-all --path --plaintext "names[0]"=a --plaintext "names[1]"=b` 
 *     will set the value to a list with the first item `a` and second item `b`.
 *   - `pulumi config set-all --path --plaintext parent.nested=value --plaintext parent.other=value2` 
 *     will set the value of `parent` to a map `{nested: value, other: value2}`.
 *   - `pulumi config set-all --path --plaintext '["parent.name"].["nested.name"]'=value` will set the 
 *     value of `parent.name` to a map `nested.name: value`.
 *
 * The `--json` flag can be used to pass a JSON string from which values should be read.
 * The JSON string should follow the same format as that produced by `pulumi config --json`. If the
 * `--json` option is passed, the `--plaintext`, `--secret` and `--path` flags must not be used.
 */
export function configSetAll(options?: ConfigSetAllOptions): Promise<Output> {
    const __args = []
    __args.push('config')
    __args.push('set-all')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConsoleOptions extends Options {
    /** The name of the stack to view */
    stack?: string;
}

/** Opens the current stack in the Pulumi Console */
export function console(options?: ConsoleOptions): Promise<Output> {
    const __args = []
    __args.push('console')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConvertOptions extends Options {
    /** Which converter plugin to use to read the source program */
    From?: string;
    /** Generate the converted program(s) only; do not install dependencies */
    generateOnly?: boolean;
    /** Which language plugin to use to generate the Pulumi project */
    language?: string;
    /** Any mapping files to use in the conversion */
    mappings?: string;
    /** The name to use for the converted project; defaults to the directory of the source project */
    name?: string;
    /** The output directory to write the converted project to */
    out?: string;
    /** Fail the conversion on errors such as missing variables */
    strict?: boolean;
}

/**
 * Convert Pulumi programs from a supported source program into other supported languages.
 *
 * The source program to convert will default to the current working directory.
 *
 * Valid source languages: yaml, terraform, bicep, arm, kubernetes
 *
 * Valid target languages: typescript, python, csharp, go, java, yaml
 * Example command usage:
 *     pulumi convert --from yaml --language java --out .
 */
export function convert(options?: ConvertOptions): Promise<Output> {
    const __args = []
    __args.push('convert')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ConvertTraceOptions extends Options {
    /** the sample granularity */
    granularity?: string;
    /** true to ignore log spans */
    ignoreLogSpans?: boolean;
    /** true to export to OpenTelemetry */
    otel?: boolean;
}

/**
 * Convert a trace from the Pulumi CLI to Google's pprof format.
 *
 * This command is used to convert execution traces collected by a prior
 * invocation of the Pulumi CLI from their native format to Google's
 * pprof format. The converted trace is written to stdout, and can be
 * inspected using `go tool pprof`.
 */
export function convertTrace(traceFile: string, options?: ConvertTraceOptions): Promise<Output> {
    const __args = []
    __args.push('convert-trace')
    __args.push(traceFile)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentOptions extends Options {
    /** Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml */
    configFile?: string;
}

export interface DeploymentRunOptions extends DeploymentOptions {
    /** The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue will be used. */
    agentPoolId?: string;
    /** Environment variables to use in the remote operation of the form NAME=value (e.g. `--env FOO=bar`) */
    env?: string;
    /** Environment variables with secret values to use in the remote operation of the form NAME=secretvalue (e.g. `--env FOO=secret`) */
    envSecret?: string;
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
    preRunCommand?: string;
    /** Whether to skip the default dependency installation step */
    skipInstallDependencies?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of the state permalink */
    suppressPermalink?: boolean;
    /** Suppress log streaming of the deployment job */
    suppressStreamLogs?: boolean;
}

/**
 * Launch a deployment job on Pulumi Cloud
 *
 * This command queues a new deployment job for any supported operation of type 
 * update, preview, destroy, refresh, detect-drift or remediate-drift.
 */
export function deploymentRun(operation: string, url: string, options?: DeploymentRunOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('run')
    __args.push(operation)
    if (url != null) {
        __args.push(url)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsOptions extends DeploymentOptions {
}

export interface DeploymentSettingsConfigureOptions extends DeploymentSettingsOptions {
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Updates stack's deployment settings secrets */
export function deploymentSettingsConfigure(options?: DeploymentSettingsConfigureOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('configure')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsDestroyOptions extends DeploymentSettingsOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Delete all the stack's deployment settings */
export function deploymentSettingsDestroy(options?: DeploymentSettingsDestroyOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('destroy')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsEnvOptions extends DeploymentSettingsOptions {
    /** whether the key should be removed */
    remove?: boolean;
    /** whether the value should be treated as a secret and be encrypted */
    secret?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Update stack's deployment settings secrets */
export function deploymentSettingsEnv(options?: DeploymentSettingsEnvOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('env')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsInitOptions extends DeploymentSettingsOptions {
    /** Forces content to be generated even if it is already configured */
    force?: boolean;
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Git SSH private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Initialize the stack's deployment.yaml file */
export function deploymentSettingsInit(options?: DeploymentSettingsInitOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('init')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsPullOptions extends DeploymentSettingsOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Pull the stack's deployment settings from Pulumi Cloud into the deployment.yaml file */
export function deploymentSettingsPull(options?: DeploymentSettingsPullOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('pull')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DeploymentSettingsPushOptions extends DeploymentSettingsOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Update stack deployment settings from deployment.yaml */
export function deploymentSettingsPush(options?: DeploymentSettingsPushOptions): Promise<Output> {
    const __args = []
    __args.push('deployment')
    __args.push('settings')
    __args.push('push')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface DestroyOptions extends Options {
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the destroy and save to the stack config file */
    config?: string;
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
    exclude?: string;
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
    target?: string;
    /** Allows destroying of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Automatically approve and perform the destroy after previewing it */
    yes?: boolean;
}

/**
 * Destroy all existing resources in the stack, but not the stack itself
 *
 * Deletes all the resources in the selected stack.  The current state is
 * loaded from the associated state file in the workspace.  After running to completion,
 * all of this stack's resources and associated state are deleted.
 *
 * The stack itself is not deleted. Use `pulumi stack rm` or the 
 * `--remove` flag to delete the stack and its config file.
 *
 * Warning: this command is generally irreversible and should be used with great care.
 */
export function destroy(options?: DestroyOptions): Promise<Output> {
    const __args = []
    __args.push('destroy')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

/**
 * Destroy all existing resources in the stack, but not the stack itself
 *
 * Deletes all the resources in the selected stack.  The current state is
 * loaded from the associated state file in the workspace.  After running to completion,
 * all of this stack's resources and associated state are deleted.
 *
 * The stack itself is not deleted. Use `pulumi stack rm` or the 
 * `--remove` flag to delete the stack and its config file.
 *
 * Warning: this command is generally irreversible and should be used with great care.
 */
export function destroyInline(__program: PulumiFn, options?: DestroyOptions): Promise<Output> {
    const __args = []
    __args.push('destroy')
    if (options != null) {
    }
    return inline(__program, 'pulumi', __args)
}

export interface EnvOptions extends Options {
    /** The name of the environment to operate on. */
    env?: string;
}

export interface EnvCloneOptions extends EnvOptions {
    /** preserve the same team access on the environment being cloned */
    preserveAccess?: boolean;
    /** preserve any tags on the environment being cloned */
    preserveEnvTags?: boolean;
    /** preserve history of the environment being cloned */
    preserveHistory?: boolean;
    /** preserve any tags on the environment revisions being cloned */
    preserveRevTags?: boolean;
}

/**
 * Clone an existing environment into a new environment.
 *
 * This command clones an existing environment with the given identifier into a new environment.
 * If a project is omitted from the new environment identifier the new environment will be created
 * within the same project as the environment being cloned.
 */
export function envClone(options?: EnvCloneOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('clone')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvDiffOptions extends EnvOptions {
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell' */
    format?: string;
    /** Show the diff for a specific path */
    path?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/**
 * Show changes between versions
 *
 * This command displays the changes between two environments or two versions
 * of a single environment.
 *
 * The first argument is the base environment for the diff and the second argument
 * is the comparison environment. If the environment name portion of the second
 * argument is omitted, the name of the base environment is used. If the version portion of
 * the second argument is omitted, the 'latest' tag is used.
 */
export function envDiff(options?: EnvDiffOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('diff')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvEditOptions extends EnvOptions {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** the command to use to edit the environment definition */
    editor?: string;
    /** the file that contains the updated environment, if any. Pass `-` to read from standard input. */
    file?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/**
 * Edit an environment definition
 *
 * This command fetches the current definition for the named environment and opens it
 * for editing in an editor. The editor defaults to the value of the VISUAL environment
 * variable. If VISUAL is not set, EDITOR is used. These values are interpreted as
 * commands to which the name of the temporary file used for the environment is appended.
 * If no editor is specified via the --editor flag or environment variables, edit
 * defaults to `vi`.
 */
export function envEdit(options?: EnvEditOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('edit')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvGetOptions extends EnvOptions {
    /** Set to print just the definition. */
    definition?: boolean;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
    /** Set to print just the value in the given format. May be 'dotenv', 'json', 'detailed', 'shell' or 'string' */
    value?: string;
}

/**
 * Get a value within an environment
 *
 * This command fetches the current definition for the named environment and gets a
 * value within it. The path to the value to set is a Pulumi property path. The value
 * is printed to stdout as YAML.
 */
export function envGet(options?: EnvGetOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('get')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvInitOptions extends EnvOptions {
    /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
    file?: string;
}

/**
 * Create an empty environment with the given name, ready for editing
 *
 * This command creates an empty environment with the given name. It has no definition,
 * but afterwards it can be edited using the `edit` command.
 *
 * To create an environment in an organization when logged in to the Pulumi Cloud,
 * prefix the stack name with the organization name and a slash (e.g. 'acmecorp/dev').
 */
export function envInit(options?: EnvInitOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('init')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvLsOptions extends EnvOptions {
    /** Filter returned environments to those in a specific organization */
    organization?: string;
    /** Filter returned environments to those in a specific project */
    project?: string;
}

/**
 * List environments
 *
 * This command lists environments. All environments you have access to will be listed.
 */
export function envLs(options?: EnvLsOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvOpenOptions extends EnvOptions {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', 'shell' or 'string' */
    format?: string;
    /** the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m) */
    lifetime?: string;
}

/**
 * Open the environment with the given name and return the result
 *
 * This command opens the environment with the given name. The result is written to
 * stdout as JSON. If a property path is specified, only retrieves that property.
 */
export function envOpen(options?: EnvOpenOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('open')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvRmOptions extends EnvOptions {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/**
 * Remove an environment or a value from an environment
 *
 * This command removes an environment or a value from an environment.
 * When removing an environment, the environment will no longer be available
 * once this command completes.
 */
export function envRm(options?: EnvRmOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('rm')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvRotateOptions extends EnvOptions {
}

/**
 * Rotate secrets in an environment
 *
 * Optionally accepts any number of Property Paths as additional arguments. If given any paths, will only rotate secrets at those paths.
 */
export function envRotate(options?: EnvRotateOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('rotate')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvRunOptions extends EnvOptions {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** true to treat the command as interactive and disable output filters */
    interactive?: boolean;
    /** the lifetime of the opened environment */
    lifetime?: string;
}

/**
 * Open the environment with the given name and run a command
 *
 * This command opens the environment with the given name and runs the given command.
 * If the opened environment contains a top-level 'environmentVariables' object, each
 * key-value pair in the object is made available to the command as an environment
 * variable. Note that commands are not run in a subshell, so environment variable
 * references in the command are not expanded by default. You should invoke the command
 * inside a shell if you need environment variable expansion:
 *
 *     run <environment-name> -- zsh -c '"echo $MY_ENV_VAR"'
 *
 * The command to run is assumed to be non-interactive by default and its output
 * streams are filtered to remove any secret values. Use the -i flag to run interactive
 * commands, which will disable filtering.
 *
 * It is not strictly required that you pass `--`. The `--` indicates that any
 * arguments that follow it should be treated as positional arguments instead of flags.
 * It is only required if the arguments to the command you would like to run include
 * flags of the form `--flag` or `-f`.
 */
export function envRun(options?: EnvRunOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('run')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvSetOptions extends EnvOptions {
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

/**
 * Set a value within an environment
 *
 * This command fetches the current definition for the named environment and modifies a
 * value within it. The path to the value to set is a Pulumi property path. The value
 * is interpreted as YAML.
 */
export function envSet(options?: EnvSetOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('set')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvTagOptions extends EnvOptions {
    /** display times in UTC */
    utc?: boolean;
}

export interface EnvTagGetOptions extends EnvTagOptions {
    /** display times in UTC */
    utc?: boolean;
}

/**
 * Get an environment tag
 *
 * This command get a tag with the given name on the specified environment.
 */
export function envTagGet(options?: EnvTagGetOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('tag')
    __args.push('get')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvTagLsOptions extends EnvTagOptions {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/**
 * List environment tags
 *
 * This command lists an environment's tags.
 */
export function envTagLs(options?: EnvTagLsOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('tag')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvTagMvOptions extends EnvTagOptions {
    /** display times in UTC */
    utc?: boolean;
}

/**
 * Move an environment tag
 *
 * This command updates a tag with the given name on the specified environment, changing it's name.
 */
export function envTagMv(options?: EnvTagMvOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('tag')
    __args.push('mv')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvTagRmOptions extends EnvTagOptions {
}

/**
 * Remove an environment tag
 *
 * This command removes an environment tag using the tag name.
 */
export function envTagRm(options?: EnvTagRmOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('tag')
    __args.push('rm')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvVersionOptions extends EnvOptions {
    /** display times in UTC */
    utc?: boolean;
}

export interface EnvVersionHistoryOptions extends EnvVersionOptions {
    /** the command to use to page through the environment's revisions */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/**
 * Show revision history
 *
 * This command shows the revision history for an environment. If a version
 * is present, the logs will start at the corresponding revision.
 */
export function envVersionHistory(options?: EnvVersionHistoryOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('version')
    __args.push('history')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvVersionRetractOptions extends EnvVersionOptions {
    /** the reason for the retraction */
    reason?: string;
    /** the version to use to replace the retracted revision */
    replaceWith?: string;
}

/**
 * Retract a specific revision of an environment
 *
 * This command retracts a specific revision of an environment. A retracted
 * revision can no longer be read or opened. Retracting a revision also updates
 * any tags that point to the retracted revision to instead point to a
 * replacement revision. If no replacement is specified, the latest non-retracted
 * revision preceding the revision being retracted is used as the replacement.
 *
 * The revision pointed to by the `latest` tag may not be retracted. To retract
 * the latest revision of an environment, first update the environment with a new
 * definition.
 */
export function envVersionRetract(options?: EnvVersionRetractOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('version')
    __args.push('retract')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvVersionRollbackOptions extends EnvVersionOptions {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
}

/**
 * Roll back to a specific version
 *
 * This command rolls an environment's definition back to the specified
 * version. The environment's definition will be replaced with the
 * definition at that version, creating a new revision.
 */
export function envVersionRollback(options?: EnvVersionRollbackOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('version')
    __args.push('rollback')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvVersionTagOptions extends EnvVersionOptions {
    /** display times in UTC */
    utc?: boolean;
}

export interface EnvVersionTagLsOptions extends EnvVersionTagOptions {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/**
 * List tagged versions
 *
 * This command lists an environment's tagged versions.
 */
export function envVersionTagLs(options?: EnvVersionTagLsOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('version')
    __args.push('tag')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface EnvVersionTagRmOptions extends EnvVersionTagOptions {
}

/**
 * Remove a tagged version
 *
 * This command removes the tagged version with the given name
 */
export function envVersionTagRm(options?: EnvVersionTagRmOptions): Promise<Output> {
    const __args = []
    __args.push('env')
    __args.push('version')
    __args.push('tag')
    __args.push('rm')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface GenCompletionOptions extends Options {
}

/** Generate completion scripts for the Pulumi CLI */
export function genCompletion(shell: string, options?: GenCompletionOptions): Promise<Output> {
    const __args = []
    __args.push('gen-completion')
    __args.push(shell)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface GenMarkdownOptions extends Options {
}

/** Generate Pulumi CLI documentation as Markdown (one file per command) */
export function genMarkdown(dir: string, options?: GenMarkdownOptions): Promise<Output> {
    const __args = []
    __args.push('gen-markdown')
    __args.push(dir)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface GenerateCliSpecOptions extends Options {
    /** help for generate-cli-spec */
    help?: boolean;
}

/** Generate Pulumi CLI specification as JSON */
export function generateCliSpec(options?: GenerateCliSpecOptions): Promise<Output> {
    const __args = []
    __args.push('generate-cli-spec')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface HelpOptions extends Options {
}

/**
 * Help provides help for any command in the application.
 * Simply type pulumi help [path to command] for full details.
 */
export function help(options?: HelpOptions): Promise<Output> {
    const __args = []
    __args.push('help')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ImportOptions extends Options {
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
    From?: string;
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
    properties?: string;
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

/**
 * Import resources into an existing stack.
 *
 * Resources that are not managed by Pulumi can be imported into a Pulumi stack
 * using this command. A definition for each resource will be printed to stdout
 * in the language used by the project associated with the stack; these definitions
 * should be added to the Pulumi program. The resources are protected from deletion
 * by default.
 *
 * Should you want to import your resource(s) without protection, you can pass
 * `--protect=false` as an argument to the command. This will leave all resources unprotected.
 *
 * A single resource may be specified in the command line arguments or a set of
 * resources may be specified by a JSON file.
 *
 * If using the command line args directly, the type token, name, id and optional flags
 * must be provided.  For example:
 *
 *     pulumi import 'aws:iam/user:User' name id
 *
 * The type token and property used for resource lookup are available in the Import section of
 * the resource's API documentation in the Pulumi Registry (https://www.pulumi.com/registry/).
 * To fully specify parent and/or provider, subsitute the <urn> for each into the following:
 *
 *      pulumi import 'aws:iam/user:User' name id --parent 'parent=<urn>' --provider 'admin=<urn>'
 *
 * When importing multiple resources at once the `--file` option can be used to pass a JSON file
 * containing multiple resources: 
 *      pulumi import --file import.json
 *
 * Where import.json is a file that matches the following JSON format:
 *
 *     {
 *         "resources": [
 *             {
 *                 "type": "aws:ec2/vpc:Vpc",
 *                 "name": "application-vpc",
 *                 "id": "vpc-0ad77710973388316"
 *             },
 *             ...
 *             {
 *                 ...
 *             }
 *         ],
 *     }
 *
 * The full import file schema references can be found in the [import documentation](https://www.pulumi.com/docs/iac/adopting-pulumi/import/#bulk-import-operations).
 *
 * The import JSON file can be generated from a Pulumi program by running
 *
 *     pulumi preview --import-file import.json
 *
 * This will create entries for all resources that need creating from the preview, filling
 * in the name, type, parent and provider information and just requiring you to fill in the
 * resource IDs.
 */
export function Import(type: string, name?: string, id?: string, options?: ImportOptions): Promise<Output> {
    const __args = []
    __args.push('import')
    if (type != null) {
        __args.push(type)
    }
    if (name != null) {
        __args.push(name)
    }
    if (id != null) {
        __args.push(id)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface InstallOptions extends Options {
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

/**
 * Install packages and plugins for the current program or policy pack.
 *
 * This command is used to manually install packages and plugins required by your program or policy pack.
 * If your Pulumi.yaml file contains a 'packages' section, this command will automatically install
 * SDKs for all packages declared in that section, similar to the 'pulumi package add' command.
 */
export function install(options?: InstallOptions): Promise<Output> {
    const __args = []
    __args.push('install')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface LoginOptions extends Options {
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

/**
 * Log in to the Pulumi Cloud.
 *
 * The Pulumi Cloud manages your stack's state reliably. Simply run
 *
 *     $ pulumi login
 *
 * and this command will prompt you for an access token, including a way to launch your web browser to
 * easily obtain one. You can script by using `PULUMI_ACCESS_TOKEN` environment variable.
 *
 * By default, this will log in to the managed Pulumi Cloud backend.
 * If you prefer to log in to a self-hosted Pulumi Cloud backend, specify a URL. For example, run
 *
 *     $ pulumi login https://api.pulumi.acmecorp.com
 *
 * to log in to a self-hosted Pulumi Cloud running at the api.pulumi.acmecorp.com domain.
 *
 * For `https://` URLs, the CLI will speak REST to a Pulumi Cloud that manages state and concurrency control.
 * You can specify a default org to use when logging into the Pulumi Cloud backend or a self-hosted Pulumi Cloud.
 *
 * If you prefer to operate Pulumi independently of a Pulumi Cloud, and entirely local to your computer,
 * pass `file://<path>`, where `<path>` will be where state checkpoints will be stored. For instance,
 *
 *     $ pulumi login file://~
 *
 * will store your state information on your computer underneath `~/.pulumi`. It is then up to you to
 * manage this state, including backing it up, using it in a team environment, and so on.
 *
 * As a shortcut, you may pass --local to use your home directory (this is an alias for `file://~`):
 *
 *     $ pulumi login --local
 *
 * Additionally, you may leverage supported object storage backends from one of the cloud providers to manage the state independent of the Pulumi Cloud. For instance,
 *
 * AWS S3:
 *
 *     $ pulumi login s3://my-pulumi-state-bucket
 *
 * GCP GCS:
 *
 *     $ pulumi login gs://my-pulumi-state-bucket
 *
 * Azure Blob:
 *
 *     $ pulumi login azblob://my-pulumi-state-bucket
 *
 * PostgreSQL:
 *
 *     $ pulumi login postgres://username:password@hostname:5432/database
 */
export function login(url: string, options?: LoginOptions): Promise<Output> {
    const __args = []
    __args.push('login')
    if (url != null) {
        __args.push(url)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface LogoutOptions extends Options {
    /** Logout of all backends */
    all?: boolean;
    /** A cloud URL to log out of (defaults to current cloud) */
    cloudUrl?: string;
    /** Log out of using local mode */
    local?: boolean;
}

/**
 * Log out of the Pulumi Cloud.
 *
 * This command deletes stored credentials on the local machine for a single login.
 *
 * Because you may be logged into multiple backends simultaneously, you can optionally pass
 * a specific URL argument, formatted just as you logged in, to log out of a specific one.
 * If no URL is provided, you will be logged out of the current backend.
 *
 * If you would like to log out of all backends simultaneously, you can pass `--all`,
 *
 *     $ pulumi logout --all
 */
export function logout(url: string, options?: LogoutOptions): Promise<Output> {
    const __args = []
    __args.push('logout')
    if (url != null) {
        __args.push(url)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface LogsOptions extends Options {
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

/**
 * [EXPERIMENTAL] Show aggregated resource logs for a stack
 *
 * This command aggregates log entries associated with the resources in a stack from the corresponding
 * provider. For example, for AWS resources, the `pulumi logs` command will query
 * CloudWatch Logs for log data relevant to resources in a stack.
 */
export function logs(options?: LogsOptions): Promise<Output> {
    const __args = []
    __args.push('logs')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface NewOptions extends Options {
    /** Prompt to use for Pulumi AI */
    ai?: string;
    /** Config to save */
    config?: string;
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
    runtimeOptions?: string;
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
    secretsProvider?: string;
    /** The stack name; either an existing stack or stack to create; if not specified, a prompt will request it */
    stack?: string;
    /** Run in template mode, which will skip prompting for AI or Template functionality */
    templateMode?: boolean;
    /** Skip prompts and proceed with default values */
    yes?: boolean;
}

/**
 * Create a new Pulumi project and stack from a template.
 *
 * To create a project from a specific template, pass the template name (such as `aws-typescript`
 * or `azure-python`). If no template name is provided, a list of suggested templates will be presented
 * which can be selected interactively.
 * For testing, a path to a local template may be passed instead (such as `~/templates/aws-typescript`)
 *
 * By default, a stack created using the pulumi.com backend will use the pulumi.com secrets
 * provider and a stack created using the local or cloud object storage backend will use the
 * `passphrase` secrets provider.  A different secrets provider can be selected by passing the
 * `--secrets-provider` flag.
 *
 * To use the `passphrase` secrets provider with the pulumi.com backend, use:
 * * `pulumi new --secrets-provider=passphrase`
 *
 * To use a cloud secrets provider with any backend, use one of the following:
 * * `pulumi new --secrets-provider="awskms://alias/ExampleAlias?region=us-east-1"`
 * * `pulumi new --secrets-provider="awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1"`
 * * `pulumi new --secrets-provider="azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname"`
 * * `pulumi new --secrets-provider="gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k"`
 * * `pulumi new --secrets-provider="hashivault://mykey"`
 *
 * To create a project from a specific source control location, pass the url as follows e.g.
 * * `pulumi new https://gitlab.com/<user>/<repo>`
 * * `pulumi new https://bitbucket.org/<user>/<repo>`
 * * `pulumi new https://github.com/<user>/<repo>`
 *
 *   Note: If the URL doesn't follow the usual scheme of the given host (e.g. for GitLab subprojects)
 *         you can append `.git` to the repository to disambiguate and point to the correct repository.
 *         For example `https://gitlab.com/<project>/<subproject>/<repository>.git`.
 *
 * To create the project from a branch of a specific source control location, pass the url to the branch, e.g.
 * * `pulumi new https://gitlab.com/<user>/<repo>/tree/<branch>`
 * * `pulumi new https://bitbucket.org/<user>/<repo>/tree/<branch>`
 * * `pulumi new https://github.com/<user>/<repo>/tree/<branch>`
 *
 * To use a private repository as a template source, provide an HTTPS or SSH URL with relevant credentials.
 * Ensure your SSH agent has the correct identity (ssh-add) or you may be prompted for your key's passphrase.
 * * `pulumi new git@github.com:<user>/<private-repo>`
 * * `pulumi new https://<user>:<password>@<hostname>/<project>/<repo>`
 * * `pulumi new <user>@<hostname>:<project>/<repo>`
 * * `PULUMI_GITSSH_PASSPHRASE=<passphrase> pulumi new ssh://<user>@<hostname>/<project>/<repo>`
 * To create a project using Pulumi AI, either select `ai` from the first selection, or provide any of the following:
 * * `pulumi new --ai "<prompt>"`
 * * `pulumi new --language <language>`
 * * `pulumi new --ai "<prompt>" --language <language>`
 * Any missing but required information will be prompted for.
 */
export function New(template: string, options?: NewOptions): Promise<Output> {
    const __args = []
    __args.push('new')
    if (template != null) {
        __args.push(template)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface OrgOptions extends Options {
}

export interface OrgGetDefaultOptions extends OrgOptions {
}

/**
 * Get the default org for the current backend.
 *
 * This command is used to get the default organization for which and stacks are created in the current backend.
 *
 * Currently, only the managed and self-hosted backends support organizations.
 */
export function orgGetDefault(options?: OrgGetDefaultOptions): Promise<Output> {
    const __args = []
    __args.push('org')
    __args.push('get-default')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface OrgSearchOptions extends OrgOptions {
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
    query?: string;
    /** Open the search results in a web browser. */
    web?: boolean;
}

export interface OrgSearchAiOptions extends OrgSearchOptions {
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

/** Search for resources in Pulumi Cloud using Pulumi AI */
export function orgSearchAi(options?: OrgSearchAiOptions): Promise<Output> {
    const __args = []
    __args.push('org')
    __args.push('search')
    __args.push('ai')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface OrgSetDefaultOptions extends OrgOptions {
}

/**
 * Set the local default organization for the current backend.
 *
 * This command is used to set your local default organization in which to create 
 * projects and stacks for the current backend.
 *
 * Currently, only the managed and self-hosted backends support organizations. If you try and set a default organization for a backend that does not 
 * support create organizations, then an error will be returned by the CLI
 */
export function orgSetDefault(name: string, options?: OrgSetDefaultOptions): Promise<Output> {
    const __args = []
    __args.push('org')
    __args.push('set-default')
    __args.push(name)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageOptions extends Options {
}

export interface PackageAddOptions extends PackageOptions {
}

/**
 * Add a package to your Pulumi project or plugin.
 *
 * This command locally generates an SDK in the currently selected Pulumi language,
 * adds the package to your project configuration file (Pulumi.yaml or
 * PulumiPlugin.yaml), and prints instructions on how to use it in your project.
 * The SDK is based on a Pulumi package schema extracted from a given resource
 * plugin or provided directly.
 *
 * The <provider> argument can be specified in one of the following ways:
 *
 * - When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to
 *   resolve a resource plugin first, installing it on-demand, similarly to:
 *
 *     pulumi plugin install resource PLUGIN [VERSION]
 *
 * - When <provider> is specified as a local path, Pulumi executes the provider
 *   binary to extract its package schema:
 *
 *     pulumi package add ./my-provider
 *
 * - When <provider> is a path to a local file with a '.json', '.yml' or '.yaml'
 *   extension, Pulumi package schema is read from it directly:
 *
 *     pulumi package add ./my/schema.json
 *
 * - When <provider> is a reference to a Git repo, Pulumi clones the repo and
 *   executes the source. Optionally a version can be specified.  It can either
 *   be a tag (in semver format), or a Git commit hash.  By default the latest
 *   tag (by semver version), or if not available the latest commit on the
 *   default branch is used. Paths can be disambiguated from the repo name by
 *   appending '.git' to the repo URL, followed by the path to the package:
 *
 *     pulumi package add example.org/org/repo.git/path[@<version>]
 *
 * For parameterized providers, parameters may be specified as additional
 * arguments. The exact format of parameters is provider-specific; consult the
 * provider's documentation for more information. If the parameters include flags
 * that begin with dashes, you may need to use '--' to separate the provider name
 * from the parameters, as in:
 *
 *   pulumi package add <provider> -- --provider-parameter-flag value
 */
export function packageAdd(providers: string[], options?: PackageAddOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('add')
    __args.push(... providers)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageDeleteOptions extends PackageOptions {
    /** Skip confirmation prompts, and proceed with deletion anyway */
    yes?: boolean;
}

/**
 * Delete a package version from the Pulumi Registry.
 *
 * The package version must be specified in the format:
 *   [[<source>/]<publisher>/]<name>[@<version>]
 *
 * Example:
 *   pulumi package delete private/myorg/my-package@1.0.0
 *
 * Warning: If this is the only version of the package, the entire package
 * will be removed. This action cannot be undone.
 *
 * You must have publish permissions for the package to delete it.
 */
export function packageDelete(packageName: string, options?: PackageDeleteOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('delete')
    __args.push(packageName)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageGenSdkOptions extends PackageOptions {
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

/**
 * Generate SDK(s) from a package or schema.
 *
 * <schema_source> can be a package name or the path to a plugin binary or folder.
 * If a folder either the plugin binary must match the folder name (e.g. 'aws' and 'pulumi-resource-aws') or it must have a PulumiPlugin.yaml file specifying the runtime to use.
 */
export function packageGenSdk(schemaSources: string[], options?: PackageGenSdkOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('gen-sdk')
    __args.push(... schemaSources)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageGetMappingOptions extends PackageOptions {
    /** The file to write the mapping data to */
    out?: string;
}

/**
 * Get the mapping information for a given key from a package.
 *
 * <schema_source> can be a package name or the path to a plugin binary. [provider key]
 * is the name of the source provider (e.g. "terraform", if a mapping was being requested
 * from Terraform to Pulumi). If you need to pass parameters, you must provide a provider
 * key. In the event that you wish to pass none, you must therefore explicitly pass an
 * empty string.
 */
export function packageGetMapping(key: string, schemaSource: string, providerKey: string, options?: PackageGetMappingOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('get-mapping')
    __args.push(key)
    __args.push(schemaSource)
    if (providerKey != null) {
        __args.push(providerKey)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageGetSchemaOptions extends PackageOptions {
}

/**
 * Get the schema.json from a package.
 *
 * <schema_source> can be a package name or the path to a plugin binary or folder.
 * If a folder either the plugin binary must match the folder name (e.g. 'aws' and 'pulumi-resource-aws') or it must have a PulumiPlugin.yaml file specifying the runtime to use.
 */
export function packageGetSchema(schemaSources: string[], options?: PackageGetSchemaOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('get-schema')
    __args.push(... schemaSources)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackageInfoOptions extends PackageOptions {
    /** Function name */
    Function?: string;
    /** Module name */
    module?: string;
    /** Resource name */
    resource?: string;
}

/**
 * Show information about a package
 *
 * This command shows information about a package, its modules and detailed resource info.
 *
 * The <provider> argument can be specified in the same way as in 'pulumi package add'.
 */
export function packageInfo(providers: string[], options?: PackageInfoOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('info')
    __args.push(... providers)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackagePackSdkOptions extends PackageOptions {
}

/** Pack a package SDK to a language specific artifact. */
export function packagePackSdk(language: string, path: string, options?: PackagePackSdkOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('pack-sdk')
    __args.push(language)
    __args.push(path)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackagePublishOptions extends PackageOptions {
    /** Path to the installation configuration markdown file */
    installationConfiguration?: string;
    /** The publisher of the package (e.g., 'pulumi'). Defaults to the publisher set in the package schema or the default organization in your pulumi config. */
    publisher?: string;
    /** Path to the package readme/index markdown file */
    readme?: string;
    /** The origin of the package (e.g., 'pulumi', 'private', 'opentofu'). Defaults to 'private'. */
    source?: string;
}

/**
 * Publish a package to the Private Registry.
 *
 * This command publishes a package to the Private Registry. The package can be a provider or a schema.
 *
 * When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to resolve a resource plugin first, installing it on-demand, similarly to:
 *
 *   pulumi plugin install resource PLUGIN [VERSION]
 *
 * When <provider> is specified as a local path, Pulumi executes the provider binary to extract its package schema.
 *
 * For parameterized providers, parameters may be specified as additional arguments. The exact format of parameters is provider-specific; consult the provider's documentation for more information. If the parameters include flags that begin with dashes, you may need to use '--' to separate the provider name from the parameters, as in:
 *
 *   pulumi package publish <provider> --readme ./README.md -- --provider-parameter-flag value
 *
 * When <schema> is a path to a local file with a '.json', '.yml' or '.yaml' extension, Pulumi package schema is read from it directly:
 *
 *   pulumi package publish ./my/schema.json --readme ./README.md
 */
export function packagePublish(providers: string[], options?: PackagePublishOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('publish')
    __args.push(... providers)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PackagePublishSdkOptions extends PackageOptions {
    /**
     * The path to the root of your package.
     * 	Example: ./sdk/nodejs
     * 	
     */
    path?: string;
}

/** Publish a package SDK to supported package registries. */
export function packagePublishSdk(language: string, options?: PackagePublishSdkOptions): Promise<Output> {
    const __args = []
    __args.push('package')
    __args.push('publish-sdk')
    if (language != null) {
        __args.push(language)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PluginOptions extends Options {
}

export interface PluginInstallOptions extends PluginOptions {
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

/**
 * Install one or more plugins.
 *
 * This command is used to manually install plugins required by your program. It
 * may be run with a specific KIND, NAME, and optionally, VERSION, or by omitting
 * these arguments and letting Pulumi compute the set of plugins required by the
 * current project. When Pulumi computes the download set automatically, it may
 * download more plugins than are strictly necessary.
 *
 * If VERSION is specified, it cannot be a range; it must be a specific number.
 * If VERSION is unspecified, Pulumi will attempt to look up the latest version of
 * the plugin, though the result is not guaranteed.
 */
export function pluginInstall(kind: string, name?: string, version?: string, options?: PluginInstallOptions): Promise<Output> {
    const __args = []
    __args.push('plugin')
    __args.push('install')
    if (kind != null) {
        __args.push(kind)
    }
    if (name != null) {
        __args.push(name)
    }
    if (version != null) {
        __args.push(version)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PluginLsOptions extends PluginOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** List only the plugins used by the current project */
    project?: boolean;
}

/** List plugins */
export function pluginLs(options?: PluginLsOptions): Promise<Output> {
    const __args = []
    __args.push('plugin')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PluginRmOptions extends PluginOptions {
    /** Remove all plugins */
    all?: boolean;
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/**
 * Remove one or more plugins from the download cache.
 *
 * Specify KIND, NAME, and/or VERSION to narrow down what will be removed.
 * If none are specified, the entire cache will be cleared.  If only KIND and
 * NAME are specified, but not VERSION, all versions of the plugin with the
 * given KIND and NAME will be removed.  VERSION may be a range.
 *
 * This removal cannot be undone.  If a deleted plugin is subsequently required
 * in order to execute a Pulumi program, it must be re-downloaded and installed
 * using the plugin install command.
 */
export function pluginRm(kind: string, name?: string, version?: string, options?: PluginRmOptions): Promise<Output> {
    const __args = []
    __args.push('plugin')
    __args.push('rm')
    if (kind != null) {
        __args.push(kind)
    }
    if (name != null) {
        __args.push(name)
    }
    if (version != null) {
        __args.push(version)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PluginRunOptions extends PluginOptions {
    /** The plugin kind */
    kind?: string;
}

/**
 * [EXPERIMENTAL] Run a command on a plugin binary.
 *
 * Directly executes a plugin binary, if VERSION is not specified the latest installed plugin will be used.
 */
export function pluginRun(plugin: string, args: string, options?: PluginRunOptions): Promise<Output> {
    const __args = []
    __args.push('plugin')
    __args.push('run')
    __args.push(plugin)
    if (args != null) {
        __args.push(args)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyOptions extends Options {
}

export interface PolicyDisableOptions extends PolicyOptions {
    /** The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
    /** The version of the Policy Pack that will be disabled; if not specified, any enabled version of the Policy Pack will be disabled */
    version?: string;
}

/** Disable a Policy Pack for a Pulumi organization */
export function policyDisable(policyPackName: string, options?: PolicyDisableOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('disable')
    __args.push(policyPackName)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyEnableOptions extends PolicyOptions {
    /** The file path for the Policy Pack configuration file */
    config?: string;
    /** The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
}

/** Enable a Policy Pack for a Pulumi organization. Can specify latest to enable the latest version of the Policy Pack or a specific version number. */
export function policyEnable(policyPackName: string, version: string, options?: PolicyEnableOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('enable')
    __args.push(policyPackName)
    __args.push(version)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyGroupOptions extends PolicyOptions {
}

export interface PolicyGroupLsOptions extends PolicyGroupOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** List all Policy Groups for a Pulumi organization */
export function policyGroupLs(orgName: string, options?: PolicyGroupLsOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('group')
    __args.push('ls')
    if (orgName != null) {
        __args.push(orgName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyLsOptions extends PolicyOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** List all Policy Packs for a Pulumi organization */
export function policyLs(orgName: string, options?: PolicyLsOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('ls')
    if (orgName != null) {
        __args.push(orgName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyNewOptions extends PolicyOptions {
    /** The location to place the generated Policy Pack; if not specified, the current directory is used */
    dir?: string;
    /** Forces content to be generated even if it would change existing files */
    force?: boolean;
    /** Generate the Policy Pack only; do not install dependencies */
    generateOnly?: boolean;
    /** Use locally cached templates without making any network requests */
    offline?: boolean;
}

/**
 * Create a new Pulumi Policy Pack from a template.
 *
 * To create a Policy Pack from a specific template, pass the template name (such as `aws-typescript`
 * or `azure-python`).  If no template name is provided, a list of suggested templates will be presented
 * which can be selected interactively.
 *
 * Once you're done authoring the Policy Pack, you will need to publish the pack to your organization.
 * Only organization administrators can publish a Policy Pack.
 */
export function policyNew(template: string, options?: PolicyNewOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('new')
    if (template != null) {
        __args.push(template)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyPublishOptions extends PolicyOptions {
}

/**
 * Publish a Policy Pack to the Pulumi Cloud
 *
 * If an organization name is not specified, the default org (if set) or the current user account is used.
 */
export function policyPublish(orgName: string, options?: PolicyPublishOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('publish')
    if (orgName != null) {
        __args.push(orgName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyRmOptions extends PolicyOptions {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Removes a Policy Pack from a Pulumi organization. The Policy Pack must be disabled from all Policy Groups before it can be removed. */
export function policyRm(policyPackName: string, version: string, options?: PolicyRmOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('rm')
    __args.push(policyPackName)
    __args.push(version)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PolicyValidateConfigOptions extends PolicyOptions {
    /** The file path for the Policy Pack configuration file */
    config?: string;
}

/** Validate a Policy Pack configuration against the configuration schema of the specified version. */
export function policyValidateConfig(policyPackName: string, version: string, options?: PolicyValidateConfigOptions): Promise<Output> {
    const __args = []
    __args.push('policy')
    __args.push('validate-config')
    __args.push(policyPackName)
    __args.push(version)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface PreviewOptions extends Options {
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string;
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the preview and save to the stack config file */
    config?: string;
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
    exclude?: string;
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
    policyPack?: string;
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string;
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2 */
    replace?: string;
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
    target?: string;
    /** Allow updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string;
}

/**
 * Show a preview of updates to a stack's resources.
 *
 * This command displays a preview of the updates to an existing stack whose state is
 * represented by an existing state file. The new desired state is computed by running
 * a Pulumi program, and extracting all resource allocations from its resulting object graph.
 * These allocations are then compared against the existing state to determine what
 * operations must take place to achieve the desired state. No changes to the stack will
 * actually take place.
 *
 * The program to run is loaded from the project in the current directory. Use the `-C` or
 * `--cwd` flag to use a different directory.
 */
export function preview(options?: PreviewOptions): Promise<Output> {
    const __args = []
    __args.push('preview')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

/**
 * Show a preview of updates to a stack's resources.
 *
 * This command displays a preview of the updates to an existing stack whose state is
 * represented by an existing state file. The new desired state is computed by running
 * a Pulumi program, and extracting all resource allocations from its resulting object graph.
 * These allocations are then compared against the existing state to determine what
 * operations must take place to achieve the desired state. No changes to the stack will
 * actually take place.
 *
 * The program to run is loaded from the project in the current directory. Use the `-C` or
 * `--cwd` flag to use a different directory.
 */
export function previewInline(__program: PulumiFn, options?: PreviewOptions): Promise<Output> {
    const __args = []
    __args.push('preview')
    if (options != null) {
    }
    return inline(__program, 'pulumi', __args)
}

export interface ProjectOptions extends Options {
}

export interface ProjectLsOptions extends ProjectOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** The organization whose projects to list */
    organization?: string;
}

/**
 * List your Pulumi projects.
 *
 * This command lists all Pulumi projects accessible to the current user.
 */
export function projectLs(options?: ProjectLsOptions): Promise<Output> {
    const __args = []
    __args.push('project')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface RefreshOptions extends Options {
    /** Clear all pending creates, dropping them from the state */
    clearPendingCreates?: boolean;
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the refresh and save to the stack config file */
    config?: string;
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
    exclude?: string;
    /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes occur during this refresh. This check happens after the refresh is applied */
    expectNoChanges?: boolean;
    /** A list of form [[URN ID]...] describing the provider IDs of pending creates */
    importPendingCreates?: string;
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
    target?: string;
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Automatically approve and perform the refresh after previewing it */
    yes?: boolean;
}

/**
 * Refresh the resources in a stack.
 *
 * This command compares the current stack's resource state with the state known to exist in
 * the actual cloud provider. Any such changes are adopted into the current stack. Note that if
 * the program text isn't updated accordingly, subsequent updates may still appear to be out of
 * sync with respect to the cloud provider's source of truth.
 *
 * The program to run is loaded from the project in the current directory. Use the `-C` or
 * `--cwd` flag to use a different directory.
 */
export function refresh(options?: RefreshOptions): Promise<Output> {
    const __args = []
    __args.push('refresh')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

/**
 * Refresh the resources in a stack.
 *
 * This command compares the current stack's resource state with the state known to exist in
 * the actual cloud provider. Any such changes are adopted into the current stack. Note that if
 * the program text isn't updated accordingly, subsequent updates may still appear to be out of
 * sync with respect to the cloud provider's source of truth.
 *
 * The program to run is loaded from the project in the current directory. Use the `-C` or
 * `--cwd` flag to use a different directory.
 */
export function refreshInline(__program: PulumiFn, options?: RefreshOptions): Promise<Output> {
    const __args = []
    __args.push('refresh')
    if (options != null) {
    }
    return inline(__program, 'pulumi', __args)
}

export interface ReplayEventsOptions extends Options {
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

/**
 * Replay events from a prior update, refresh, or destroy.
 *
 * This command is used to replay events emitted by a prior
 * invocation of the Pulumi CLI (e.g. `pulumi up --event-log [file]`).
 *
 * This command loads events from the indicated file and renders them
 * using either the progress view or the diff view.
 *
 * The <kind> argument must be one of: update, refresh, destroy, import.
 */
export function replayEvents(kind: string, eventsFile: string, options?: ReplayEventsOptions): Promise<Output> {
    const __args = []
    __args.push('replay-events')
    __args.push(kind)
    __args.push(eventsFile)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface SchemaOptions extends Options {
}

export interface SchemaCheckOptions extends SchemaOptions {
    /** Whether references to nonexistent types should be considered errors */
    allowDanglingReferences?: boolean;
}

/**
 * Check a Pulumi package schema for errors.
 *
 * Ensure that a Pulumi package schema meets the requirements imposed by the
 * schema spec as well as additional requirements imposed by the supported
 * target languages.
 */
export function schemaCheck(schemaFile: string, options?: SchemaCheckOptions): Promise<Output> {
    const __args = []
    __args.push('schema')
    __args.push('check')
    __args.push(schemaFile)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackOptions extends Options {
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

export interface StackChangeSecretsProviderOptions extends StackOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/**
 * Change the secrets provider for a stack. Valid secret providers types are `default`, `passphrase`, `awskms`, `azurekeyvault`, `gcpkms`, `hashivault`.
 *
 * To change to using the Pulumi Default Secrets Provider, use the following:
 *
 * pulumi stack change-secrets-provider default
 *
 * To change the stack to use a cloud secrets backend, use one of the following:
 *
 * * `pulumi stack change-secrets-provider "awskms://alias/ExampleAlias?region=us-east-1"`
 * * `pulumi stack change-secrets-provider "awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1"`
 * * `pulumi stack change-secrets-provider "azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname"`
 * * `pulumi stack change-secrets-provider "gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>"`
 * * `pulumi stack change-secrets-provider "hashivault://mykey"`
 */
export function stackChangeSecretsProvider(newSecretsProvider: string, options?: StackChangeSecretsProviderOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('change-secrets-provider')
    __args.push(newSecretsProvider)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackExportOptions extends StackOptions {
    /** A filename to write stack output to */
    file?: string;
    /** Emit secrets in plaintext in exported stack. Defaults to `false` */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Previous stack version to export. (If unset, will export the latest.) */
    version?: string;
}

/**
 * Export a stack's deployment to standard out.
 *
 * The deployment can then be hand-edited and used to update the stack via
 * `pulumi stack import`. This process may be used to correct inconsistencies
 * in a stack's state due to failed deployments, manual changes to cloud
 * resources, etc.
 */
export function stackExport(options?: StackExportOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('export')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackGraphOptions extends StackOptions {
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

/**
 * Export a stack's dependency graph to a file.
 *
 * This command can be used to view the dependency graph that a Pulumi program
 * emitted when it was run. This graph is output in the DOT format. This command operates
 * on your stack's most recent deployment.
 */
export function stackGraph(filename: string, options?: StackGraphOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('graph')
    __args.push(filename)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackHistoryOptions extends StackOptions {
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

/**
 * Display history for a stack
 *
 * This command displays data about previous updates for a stack.
 */
export function stackHistory(options?: StackHistoryOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('history')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackImportOptions extends StackOptions {
    /** A filename to read stack input from */
    file?: string;
    /** Force the import to occur, even if apparent errors are discovered beforehand (not recommended) */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/**
 * Import a deployment from standard in into an existing stack.
 *
 * A deployment that was exported from a stack using `pulumi stack export` and
 * hand-edited to correct inconsistencies due to failed updates, manual changes
 * to cloud resources, etc. can be reimported to the stack using this command.
 * The updated deployment will be read from standard in.
 */
export function stackImport(options?: StackImportOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('import')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackInitOptions extends StackOptions {
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
    teams?: string;
}

/**
 * Create an empty stack with the given name, ready for updates
 *
 * This command creates an empty stack with the given name.  It has no resources,
 * but afterwards it can become the target of a deployment using the `update` command.
 *
 * To create a stack in an organization when logged in to the Pulumi Cloud,
 * prefix the stack name with the organization name and a slash (e.g. 'acmecorp/dev')
 *
 * By default, a stack created using the pulumi.com backend will use the pulumi.com secrets
 * provider and a stack created using the local or cloud object storage backend will use the
 * `passphrase` secrets provider.  A different secrets provider can be selected by passing the
 * `--secrets-provider` flag.
 *
 * To use the `passphrase` secrets provider with the pulumi.com backend, use:
 *
 * * `pulumi stack init --secrets-provider=passphrase`
 *
 * To use a cloud secrets provider with any backend, use one of the following:
 *
 * * `pulumi stack init --secrets-provider="awskms://alias/ExampleAlias?region=us-east-1"`
 * * `pulumi stack init --secrets-provider="awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1"`
 * * `pulumi stack init --secrets-provider="azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname"`
 * * `pulumi stack init --secrets-provider="gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>"`
 * * `pulumi stack init --secrets-provider="hashivault://mykey"`
 *
 * A stack can be created based on the configuration of an existing stack by passing the
 * `--copy-config-from` flag:
 *
 * * `pulumi stack init --copy-config-from dev`
 */
export function stackInit(stackName: string, options?: StackInitOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('init')
    if (stackName != null) {
        __args.push(stackName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackLsOptions extends StackOptions {
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

/**
 * List stacks
 *
 * This command lists stacks. By default only stacks with the same project name as the
 * current workspace will be returned. By passing --all, all stacks you have access to
 * will be listed.
 *
 * Results may be further filtered by passing additional flags. Tag filters may include
 * the tag name as well as the tag value, separated by an equals sign. For example
 * 'environment=production' or just 'gcp:project'.
 */
export function stackLs(options?: StackLsOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackOutputOptions extends StackOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** Emit output as a shell script */
    shell?: boolean;
    /** Display outputs which are marked as secret in plaintext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/**
 * Show a stack's output properties.
 *
 * By default, this command lists all output properties exported from a stack.
 * If a specific property-name is supplied, just that property's value is shown.
 */
export function stackOutput(propertyName: string, options?: StackOutputOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('output')
    if (propertyName != null) {
        __args.push(propertyName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackRenameOptions extends StackOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/**
 * Rename an existing stack.
 *
 * Note: Because renaming a stack will change the value of `getStack()` inside a Pulumi program, if this
 * name is used as part of a resource's name, the next `pulumi up` will want to delete the old resource and
 * create a new copy. For now, if you don't want these changes to be applied, you should rename your stack
 * back to its previous name.
 * You can also rename the stack's project by passing a fully-qualified stack name as well. For example:
 * 'robot-co/new-project-name/production'. However in order to update the stack again, you would also need
 * to update the name field of Pulumi.yaml, so the project names match.
 */
export function stackRename(newStackName: string, options?: StackRenameOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('rename')
    __args.push(newStackName)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackRmOptions extends StackOptions {
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

/**
 * Remove a stack and its configuration
 *
 * This command removes a stack and its configuration state.  Please refer to the
 * `destroy` command for removing a resources, as this is a distinct operation.
 *
 * After this command completes, the stack will no longer be available for updates.
 */
export function stackRm(stackName: string, options?: StackRmOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('rm')
    if (stackName != null) {
        __args.push(stackName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackSelectOptions extends StackOptions {
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

/**
 * Switch the current workspace to the given stack.
 *
 * Selecting a stack allows you to use commands like `config`, `preview`, and `update`
 * without needing to type the stack name each time.
 *
 * If no <stack> argument is supplied, you will be prompted to select one interactively.
 * If provided stack name is not found you may pass the --create flag to create and select it
 */
export function stackSelect(stack: string, options?: StackSelectOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('select')
    if (stack != null) {
        __args.push(stack)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackTagOptions extends StackOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

export interface StackTagGetOptions extends StackTagOptions {
}

/** Get a single stack tag value */
export function stackTagGet(name: string, options?: StackTagGetOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('tag')
    __args.push('get')
    __args.push(name)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackTagLsOptions extends StackTagOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** List all stack tags */
export function stackTagLs(options?: StackTagLsOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('tag')
    __args.push('ls')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackTagRmOptions extends StackTagOptions {
}

/** Remove a stack tag */
export function stackTagRm(name: string, options?: StackTagRmOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('tag')
    __args.push('rm')
    __args.push(name)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackTagSetOptions extends StackTagOptions {
}

/** Set a stack tag */
export function stackTagSet(name: string, value: string, options?: StackTagSetOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('tag')
    __args.push('set')
    __args.push(name)
    __args.push(value)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StackUnselectOptions extends StackOptions {
}

/**
 * Resets stack selection from the current workspace.
 *
 * This way, next time pulumi needs to execute an operation, the user is prompted with one of the stacks to select
 * from.
 */
export function stackUnselect(options?: StackUnselectOptions): Promise<Output> {
    const __args = []
    __args.push('stack')
    __args.push('unselect')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateOptions extends Options {
}

export interface StateDeleteOptions extends StateOptions {
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

/**
 * Deletes a resource from a stack's state
 *
 * This command deletes a resource from a stack's state, as long as it is safe to do so. The resource is specified
 * by its Pulumi URN. If the URN is omitted, this command will prompt for it.
 *
 * Resources can't be deleted if other resources depend on it or are parented to it. Protected resources
 * will not be deleted unless specifically requested using the --force flag.
 *
 * Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateDelete(urn: string, options?: StateDeleteOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('delete')
    if (urn != null) {
        __args.push(urn)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateEditOptions extends StateOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/**
 * [EXPERIMENTAL] Edit the current stack's state in your EDITOR
 *
 * This command can be used to surgically edit a stack's state in the editor
 * specified by the EDITOR environment variable and will provide the user with
 * a preview showing a diff of the altered state.
 */
export function stateEdit(options?: StateEditOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('edit')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateMoveOptions extends StateOptions {
    /** The name of the stack to move resources to */
    dest?: string;
    /** Include all the parents of the moved resources as well */
    includeParents?: boolean;
    /** The name of the stack to move resources from */
    source?: string;
    /** Automatically approve and perform the move */
    yes?: boolean;
}

/**
 * Move resources from one stack to another
 *
 * This command can be used to move resources from one stack to another. This can be useful when
 * splitting a stack into multiple stacks or when merging multiple stacks into one.
 */
export function stateMove(urns: string[], options?: StateMoveOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('move')
    __args.push(... urns)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateProtectOptions extends StateOptions {
    /** Protect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/**
 * Protect resource in a stack's state
 *
 * This command sets the 'protect' bit on one or more resources, preventing those resources from being deleted.
 *
 * Caution: this command is a low-level operation that directly modifies your stack's state.
 * Setting the 'protect' bit on a resource in your stack's state is not sufficient to protect it in
 * all cases. If your program does not also set the 'protect' resource option, Pulumi will
 * unprotect the resource the next time your program runs (e.g. as part of a `pulumi up`).
 *
 * See https://www.pulumi.com/docs/iac/concepts/options/protect/ for more information on
 * the 'protect' resource option and how it can be used to protect resources in your program.
 *
 * To unprotect a resource, use `pulumi unprotect`on the resource URN.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateProtect(urns: string[], options?: StateProtectOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('protect')
    __args.push(... urns)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateRenameOptions extends StateOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/**
 * Renames a resource from a stack's state
 *
 * This command renames a resource from a stack's state. The resource is specified
 * by its Pulumi URN and the new name of the resource.
 *
 * Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateRename(urn: string, newName?: string, options?: StateRenameOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('rename')
    if (urn != null) {
        __args.push(urn)
    }
    if (newName != null) {
        __args.push(newName)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateRepairOptions extends StateOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically approve and perform the repair */
    yes?: boolean;
}

/**
 * Repair an invalid state,
 *
 * This command can be used to repair an invalid state file. It will attempt to
 * sort resources that appear out of order and remove references to resources that
 * are no longer present in the state. If the state is already valid, this command
 * will not attempt to make or write any changes. If the state is not already
 * valid, and remains invalid after repair has been attempted, this command will
 * not write any changes.
 */
export function stateRepair(options?: StateRepairOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('repair')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateTaintOptions extends StateOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/**
 * Taint one or more resources in the stack's state.
 *
 * This has the effect of ensuring the resources are destroyed and recreated upon the next `pulumi up`.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateTaint(urns: string[], options?: StateTaintOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('taint')
    __args.push(... urns)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateUnprotectOptions extends StateOptions {
    /** Unprotect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/**
 * Unprotect resources in a stack's state
 *
 * This command clears the 'protect' bit on one or more resources, allowing those resources to be deleted.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateUnprotect(urns: string[], options?: StateUnprotectOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('unprotect')
    __args.push(... urns)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateUntaintOptions extends StateOptions {
    /** Untaint all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/**
 * Untaint one or more resources in the stack's state.
 *
 * After running this, the resources will no longer be destroyed and recreated upon the next `pulumi up`.
 *
 * To see the list of URNs in a stack, use `pulumi stack --show-urns`.
 */
export function stateUntaint(urns: string[], options?: StateUntaintOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('untaint')
    __args.push(... urns)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface StateUpgradeOptions extends StateOptions {
    /** Automatically approve and perform the upgrade */
    yes?: boolean;
}

/**
 * Migrates the current backend to the latest supported version
 *
 * This only has an effect on DIY backends.
 */
export function stateUpgrade(options?: StateUpgradeOptions): Promise<Output> {
    const __args = []
    __args.push('state')
    __args.push('upgrade')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface TemplateOptions extends Options {
}

export interface TemplatePublishOptions extends TemplateOptions {
    /** The name of the template (required) */
    name?: string;
    /** The publisher of the template (e.g., 'pulumi'). Defaults to the default organization in your pulumi config. */
    publisher?: string;
    /** The version of the template (required, semver format) */
    version?: string;
}

/**
 * Publish a template to the Private Registry.
 *
 * This command publishes a template directory to the Private Registry.
 */
export function templatePublish(directory: string, options?: TemplatePublishOptions): Promise<Output> {
    const __args = []
    __args.push('template')
    __args.push('publish')
    __args.push(directory)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface UpOptions extends Options {
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string;
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the update and save to the stack config file */
    config?: string;
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
    exclude?: string;
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
    policyPack?: string;
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string;
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2. Wildcards (*, **) are also supported */
    replace?: string;
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
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2. Wildcards (*, **) are also supported */
    target?: string;
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string;
    /** Automatically approve and perform the update after previewing it */
    yes?: boolean;
}

/**
 * Create or update the resources in a stack.
 *
 * This command creates or updates resources in a stack. The new desired goal state for the target stack
 * is computed by running the current Pulumi program and observing all resource allocations to produce a
 * resource graph. This goal state is then compared against the existing state to determine what create,
 * read, update, and/or delete operations must take place to achieve the desired goal state, in the most
 * minimally disruptive way. This command records a full transactional snapshot of the stack's new state
 * afterwards so that the stack may be updated incrementally again later on.
 *
 * The program to run is loaded from the project in the current directory by default. Use the `-C` or
 * `--cwd` flag to use a different directory.
 *
 * Note: An optional template name or URL can be provided to deploy from a template. When used, a temporary
 *  project is created, deployed, and then deleted, leaving only the stack state.
 */
export function up(template: string, options?: UpOptions): Promise<Output> {
    const __args = []
    __args.push('up')
    if (template != null) {
        __args.push(template)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

/**
 * Create or update the resources in a stack.
 *
 * This command creates or updates resources in a stack. The new desired goal state for the target stack
 * is computed by running the current Pulumi program and observing all resource allocations to produce a
 * resource graph. This goal state is then compared against the existing state to determine what create,
 * read, update, and/or delete operations must take place to achieve the desired goal state, in the most
 * minimally disruptive way. This command records a full transactional snapshot of the stack's new state
 * afterwards so that the stack may be updated incrementally again later on.
 *
 * The program to run is loaded from the project in the current directory by default. Use the `-C` or
 * `--cwd` flag to use a different directory.
 *
 * Note: An optional template name or URL can be provided to deploy from a template. When used, a temporary
 *  project is created, deployed, and then deleted, leaving only the stack state.
 */
export function upInline(__program: PulumiFn, template: string, options?: UpOptions): Promise<Output> {
    const __args = []
    __args.push('up')
    if (template != null) {
        __args.push(template)
    }
    if (options != null) {
    }
    return inline(__program, 'pulumi', __args)
}

export interface VersionOptions extends Options {
}

/** Print Pulumi's version number */
export function version(options?: VersionOptions): Promise<Output> {
    const __args = []
    __args.push('version')
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface ViewTraceOptions extends Options {
    /** the port the trace viewer will listen on */
    port?: number;
}

/**
 * Display a trace from the Pulumi CLI.
 *
 * This command is used to display execution traces collected by a prior
 * invocation of the Pulumi CLI.
 *
 * This command loads trace data from the indicated file and starts a
 * webserver to display the trace. By default, this server will listen
 * port 8008; the --port flag can be used to change this if necessary.
 */
export function viewTrace(traceFile: string, options?: ViewTraceOptions): Promise<Output> {
    const __args = []
    __args.push('view-trace')
    __args.push(traceFile)
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface WatchOptions extends Options {
    /** Config to use during the update */
    config?: string;
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
    path?: string;
    /** Run one or more policy packs as part of each update */
    policyPack?: string;
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string;
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

/**
 * [EXPERIMENTAL] Continuously update the resources in a stack.
 *
 * This command watches the working directory or specified paths for the current project and updates
 * the active stack whenever the project changes.  In parallel, logs are collected for all resources
 * in the stack and displayed along with update progress.
 *
 * The program to watch is loaded from the project in the current directory by default. Use the `-C` or
 * `--cwd` flag to use a different directory.
 */
export function watch(path: string, options?: WatchOptions): Promise<Output> {
    const __args = []
    __args.push('watch')
    if (path != null) {
        __args.push(path)
    }
    if (options != null) {
    }
    return execute('pulumi', __args)
}

export interface WhoamiOptions extends Options {
    /** Emit output as JSON */
    json?: boolean;
    /** Print detailed whoami information */
    verbose?: boolean;
}

/**
 * Display the current logged-in user
 *
 * Displays the username of the currently logged in user.
 */
export function whoami(options?: WhoamiOptions): Promise<Output> {
    const __args = []
    __args.push('whoami')
    if (options != null) {
    }
    return execute('pulumi', __args)
}
