package main

type HelpText struct {
	Use   string
	Short string
	Long  string
}

var aboutText = HelpText{
	Use:   "about",
	Short: "Print information about the Pulumi environment.\n",
	Long: "Print information about the Pulumi environment.\n" +
		"\n" +
		"Prints out information helpful for debugging the Pulumi CLI." +
		"\n" +
		"This includes information about:\n" +
		" - the CLI and how it was built\n" +
		" - which OS Pulumi was run from\n" +
		" - the current project\n" +
		" - the current stack\n" +
		" - the current backend\n",
}
var cancelText = HelpText{
	Use:   "cancel [<stack-name>]",
	Short: "Cancel a stack's currently running update, if any",
	Long: "Cancel a stack's currently running update, if any.\n" +
		"\n" +
		"This command cancels the update currently being applied to a stack if any exists.\n" +
		"Note that this operation is _very dangerous_, and may leave the stack in an\n" +
		"inconsistent state if a resource operation was pending when the update was canceled.\n" +
		"\n" +
		"After this command completes successfully, the stack will be ready for further\n" +
		"updates.",
}
var configText = HelpText{
	Use:   "config",
	Short: "Manage configuration",
	Long: "Lists all configuration values for a specific stack. To add a new configuration value, run\n" +
		"`pulumi config set`. To remove and existing value run `pulumi config rm`. To get the value of\n" +
		"for a specific configuration key, use `pulumi config get <key-name>`.",
}
var configCpText = HelpText{
	Use:   "cp [key]",
	Short: "Copy config to another stack",
	Long: "Copies the config from the current stack to the destination stack. If `key` is omitted,\n" +
		"then all of the config from the current stack will be copied to the destination stack.",
}
var configGetText = HelpText{
	Use:   "get <key>",
	Short: "Get a single configuration value",
	Long: "Get a single configuration value.\n\n" +
		"The `--path` flag can be used to get a value inside a map or list:\n\n" +
		"  - `pulumi config get --path outer.inner` will get the value of the `inner` key, " +
		"if the value of `outer` is a map `inner: value`.\n" +
		"  - `pulumi config get --path names[0]` will get the value of the first item, " +
		"if the value of `names` is a list.",
}
var configRmText = HelpText{
	Use:   "rm <key>",
	Short: "Remove configuration value",
	Long: "Remove configuration value.\n\n" +
		"The `--path` flag can be used to remove a value inside a map or list:\n\n" +
		"  - `pulumi config rm --path outer.inner` will remove the `inner` key, " +
		"if the value of `outer` is a map `inner: value`.\n" +
		"  - `pulumi config rm --path names[0]` will remove the first item, " +
		"if the value of `names` is a list.",
}
var configRmAllText = HelpText{
	Use:   "rm-all <key1> <key2> <key3> ...",
	Short: "Remove multiple configuration values",
	Long: "Remove multiple configuration values.\n\n" +
		"The `--path` flag indicates that keys should be parsed within maps or lists:\n\n" +
		"  - `pulumi config rm-all --path  outer.inner foo[0] key1` will remove the \n" +
		"    `inner` key of the `outer` map, the first key of the `foo` list and `key1`.\n" +
		"  - `pulumi config rm-all outer.inner foo[0] key1` will remove the literal" +
		"    `outer.inner`, `foo[0]` and `key1` keys",
}
var configRefreshText = HelpText{
	Use:   "refresh",
	Short: "Update the local configuration based on the most recent deployment of the stack",
}
var configSetText = HelpText{
	Use:   "set <key> [value]",
	Short: "Set configuration value",
	Long: "Configuration values can be accessed when a stack is being deployed and used to configure behavior. \n" +
		"If a value is not present on the command line, pulumi will prompt for the value. Multi-line values\n" +
		"may be set by piping a file to standard in.\n\n" +
		"The `--path` flag can be used to set a value inside a map or list:\n\n" +
		"  - `pulumi config set --path names[0] a` " +
		"will set the value to a list with the first item `a`.\n" +
		"  - `pulumi config set --path parent.nested value` " +
		"will set the value of `parent` to a map `nested: value`.\n" +
		"  - `pulumi config set --path '[\"parent.name\"].[\"nested.name\"]' value` will set the value of \n" +
		"    `parent.name` to a map `nested.name: value`.",
}
var configSetAllText = HelpText{
	Use:   "set-all --plaintext key1=value1 --plaintext key2=value2 --secret key3=value3",
	Short: "Set multiple configuration values",
	Long: "pulumi set-all allows you to set multiple configuration values in one command.\n\n" +
		"Each key-value pair must be preceded by either the `--secret` or the `--plaintext` flag to denote whether \n" +
		"it should be encrypted:\n\n" +
		"  - `pulumi config set-all --secret key1=value1 --plaintext key2=value --secret key3=value3`\n\n" +
		"The `--path` flag can be used to set values inside a map or list:\n\n" +
		"  - `pulumi config set-all --path --plaintext \"names[0]\"=a --plaintext \"names[1]\"=b` \n" +
		"    will set the value to a list with the first item `a` and second item `b`.\n" +
		"  - `pulumi config set-all --path --plaintext parent.nested=value --plaintext parent.other=value2` \n" +
		"    will set the value of `parent` to a map `{nested: value, other: value2}`.\n" +
		"  - `pulumi config set-all --path --plaintext '[\"parent.name\"].[\"nested.name\"]'=value` will set the \n" +
		"    value of `parent.name` to a map `nested.name: value`.",
}
var consoleText = HelpText{
	Use:   "console",
	Short: "Opens the current stack in the Pulumi Console",
}
var convert = HelpText{
	Use:   "convert",
	Short: "Convert resource declarations into a pulumi program",
	Long: "Convert resource declarations into a pulumi program.\n" +
		"\n" +
		"The YAML program to convert will default to the manifest in the current working directory.\n" +
		"You may also specify '-f' for the file path or '-d' for the directory path containing the manifests.\n",
}
var convertTraceText = HelpText{
	Use:   "convert-trace [trace-file]",
	Short: "Convert a trace from the Pulumi CLI to Google's pprof format",
	Long: "Convert a trace from the Pulumi CLI to Google's pprof format.\n" +
		"\n" +
		"This command is used to convert execution traces collected by a prior\n" +
		"invocation of the Pulumi CLI from their native format to Google's\n" +
		"pprof format. The converted trace is written to stdout, and can be\n" +
		"inspected using `go tool pprof`.",
}
var destroy = HelpText{
	Use:   "destroy",
	Short: "Destroy all existing resources in the stack",
	Long: "Destroy all existing resources in the stack, but not the stack itself\n" +
		"\n" +
		"Deletes all the resources in the selected stack.  The current state is\n" +
		"loaded from the associated state file in the workspace.  After running to completion,\n" +
		"all of this stack's resources and associated state are deleted.\n" +
		"\n" +
		"The stack itself is not deleted. Use `pulumi stack rm` to delete the stack.\n" +
		"\n" +
		"Warning: this command is generally irreversible and should be used with great care.",
}
var genCompletionText = HelpText{
	Use:   "gen-completion <SHELL>",
	Short: "Generate completion scripts for the Pulumi CLI",
}
var genMarkdownText = HelpText{
	Use:   "gen-markdown <DIR>",
	Short: "Generate Pulumi CLI documentation as Markdown (one file per command)",
}
var importText = HelpText{
	Use:   "import [type] [name] [id]",
	Short: "Import resources into an existing stack",
	Long: "Import resources into an existing stack.\n" +
		"\n" +
		"Resources that are not managed by Pulumi can be imported into a Pulumi stack\n" +
		"using this command. A definition for each resource will be printed to stdout\n" +
		"in the language used by the project associated with the stack; these definitions\n" +
		"should be added to the Pulumi program. The resources are protected from deletion\n" +
		"by default.\n" +
		"\n" +
		"Should you want to import your resource(s) without protection, you can pass\n" +
		"`--protect=false` as an argument to the command. This will leave all resources unprotected." +
		"\n" +
		"\n" +
		"A single resource may be specified in the command line arguments or a set of\n" +
		"resources may be specified by a JSON file. This file must contain an object\n" +
		"of the following form:\n" +
		"\n" +
		"    {\n" +
		"        \"nameTable\": {\n" +
		"            \"provider-or-parent-name-0\": \"provider-or-parent-urn-0\",\n" +
		"            ...\n" +
		"            \"provider-or-parent-name-n\": \"provider-or-parent-urn-n\",\n" +
		"        },\n" +
		"        \"resources\": [\n" +
		"            {\n" +
		"                \"type\": \"type-token\",\n" +
		"                \"name\": \"name\",\n" +
		"                \"id\": \"resource-id\",\n" +
		"                \"parent\": \"optional-parent-name\",\n" +
		"                \"provider\": \"optional-provider-name\",\n" +
		"                \"version\": \"optional-provider-version\",\n" +
		"                \"properties\": [\"optional-property-names\"],\n" +
		"            },\n" +
		"            ...\n" +
		"            {\n" +
		"                ...\n" +
		"            }\n" +
		"        ]\n" +
		"    }\n" +
		"\n" +
		"The name table maps language names to parent and provider URNs. These names are\n" +
		"used in the generated definitions, and should match the corresponding declarations\n" +
		"in the source program. This table is required if any parents or providers are\n" +
		"specified by the resources to import.\n" +
		"\n" +
		"The resources list contains the set of resources to import. Each resource is\n" +
		"specified as a triple of its type, name, and ID. The format of the ID is specific\n" +
		"to the resource type. Each resource may specify the name of a parent or provider;\n" +
		"these names must correspond to entries in the name table. If a resource does not\n" +
		"specify a provider, it will be imported using the default provider for its type. A\n" +
		"resource that does specify a provider may specify the version of the provider\n" +
		"that will be used for its import.\n" +
		"Each resource may specify which input properties to import with;\n" +
		"If a resource does not specify any properties the default behaviour is to\n" +
		"import using all required properties.\n",
}
var loginText = HelpText{
	Use:   "login [<url>]",
	Short: "Log in to the Pulumi service",
	Long: "Log in to the Pulumi service.\n" +
		"\n" +
		"The service manages your stack's state reliably. Simply run\n" +
		"\n" +
		"    $ pulumi login\n" +
		"\n" +
		"and this command will prompt you for an access token, including a way to launch your web browser to\n" +
		"easily obtain one. You can script by using `PULUMI_ACCESS_TOKEN` environment variable.\n" +
		"\n" +
		"By default, this will log in to the managed Pulumi service backend.\n" +
		"If you prefer to log in to a self-hosted Pulumi service backend, specify a URL. For example, run\n" +
		"\n" +
		"    $ pulumi login https://api.pulumi.acmecorp.com\n" +
		"\n" +
		"to log in to a self-hosted Pulumi service running at the api.pulumi.acmecorp.com domain.\n" +
		"\n" +
		"For `https://` URLs, the CLI will speak REST to a service that manages state and concurrency control.\n" +
		"You can specify a default org to use when logging into the Pulumi service backend or a " +
		"self-hosted Pulumi service.\n" +
		"\n" +
		"[PREVIEW] If you prefer to operate Pulumi independently of a service, and entirely local to your computer,\n" +
		"pass `file://<path>`, where `<path>` will be where state checkpoints will be stored. For instance,\n" +
		"\n" +
		"    $ pulumi login file://~\n" +
		"\n" +
		"will store your state information on your computer underneath `~/.pulumi`. It is then up to you to\n" +
		"manage this state, including backing it up, using it in a team environment, and so on.\n" +
		"\n" +
		"As a shortcut, you may pass --local to use your home directory (this is an alias for `file://~`):\n" +
		"\n" +
		"    $ pulumi login --local\n" +
		"\n" +
		"[PREVIEW] Additionally, you may leverage supported object storage backends from one of the cloud providers " +
		"to manage the state independent of the service. For instance,\n" +
		"\n" +
		"AWS S3:\n" +
		"\n" +
		"    $ pulumi login s3://my-pulumi-state-bucket\n" +
		"\n" +
		"GCP GCS:\n" +
		"\n" +
		"    $ pulumi login gs://my-pulumi-state-bucket\n" +
		"\n" +
		"Azure Blob:\n" +
		"\n" +
		"    $ pulumi login azblob://my-pulumi-state-bucket\n",
}
var logoutText = HelpText{
	Use:   "logout <url>",
	Short: "Log out of the Pulumi service",
	Long: "Log out of the Pulumi service.\n" +
		"\n" +
		"This command deletes stored credentials on the local machine for a single login.\n" +
		"\n" +
		"Because you may be logged into multiple backends simultaneously, you can optionally pass\n" +
		"a specific URL argument, formatted just as you logged in, to log out of a specific one.\n" +
		"If no URL is provided, you will be logged out of the current backend." +
		"\n\n" +
		"If you would like to log out of all backends simultaneously, you can pass `--all`,\n" +
		"    $ pulumi logout --all",
}
var logs = HelpText{
	Use:   "logs",
	Short: "Show aggregated resource logs for a stack",
	Long: "[EXPERIMENTAL] Show aggregated resource logs for a stack\n" +
		"\n" +
		"This command aggregates log entries associated with the resources in a stack from the corresponding\n" +
		"provider. For example, for AWS resources, the `pulumi logs` command will query\n" +
		"CloudWatch Logs for log data relevant to resources in a stack.\n",
}
var newText = HelpText{
	Use:   "new [template|url]",
	Short: "Create a new Pulumi project",
	Long: "Create a new Pulumi project and stack from a template.\n" +
		"\n" +
		"To create a project from a specific template, pass the template name (such as `aws-typescript`\n" +
		"or `azure-python`).  If no template name is provided, a list of suggested templates will be presented\n" +
		"which can be selected interactively.\n" +
		"\n" +
		"By default, a stack created using the pulumi.com backend will use the pulumi.com secrets\n" +
		"provider and a stack created using the local or cloud object storage backend will use the\n" +
		"`passphrase` secrets provider.  A different secrets provider can be selected by passing the\n" +
		"`--secrets-provider` flag.\n" +
		"\n" +
		"To use the `passphrase` secrets provider with the pulumi.com backend, use:\n" +
		"* `pulumi new --secrets-provider=passphrase`\n" +
		"\n" +
		"To use a cloud secrets provider with any backend, use one of the following:\n" +
		"* `pulumi new --secrets-provider=\"awskms://alias/ExampleAlias?region=us-east-1\"`\n" +
		"* `pulumi new --secrets-provider=\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
		"* `pulumi new --secrets-provider=\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
		"* `pulumi new --secrets-provider=\"gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k\"`\n" +
		"* `pulumi new --secrets-provider=\"hashivault://mykey\"`" +
		"\n\n" +
		"To create a project from a specific source control location, pass the url as follows e.g.\n" +
		"* `pulumi new https://gitlab.com/<user>/<repo>`\n" +
		"* `pulumi new https://bitbucket.org/<user>/<repo>`\n" +
		"* `pulumi new https://github.com/<user>/<repo>`\n" +
		"\n" +
		"To create the project from a branch of a specific source control location, pass the url to the branch, e.g.\n" +
		"* `pulumi new https://gitlab.com/<user>/<repo>/tree/<branch>`\n" +
		"* `pulumi new https://bitbucket.org/<user>/<repo>/tree/<branch>`\n" +
		"* `pulumi new https://github.com/<user>/<repo>/tree/<branch>`\n",
}
var org = HelpText{
	Use:   "org",
	Short: "Manage Organization configuration",
	Long: "Manage Organization configuration.\n" +
		"\n" +
		"Use this command to manage organization configuration, " +
		"e.g. setting the default organization for a backend",
}
var orgSetDefaultText = HelpText{
	Use:   "set-default [NAME]",
	Short: "Set the default organization for the current backend",
	Long: "Set the default organization for the current backend.\n" +
		"\n" +
		"This command is used to set the default organization in which to create \n" +
		"projects and stacks for the current backend.\n" +
		"\n" +
		"Currently, only the managed and self-hosted backends support organizations. " +
		"If you try and set a default organization for a backend that does not \n" +
		"support create organizations, then an error will be returned by the CLI",
}
var orgGetDefault = HelpText{
	Use:   "get-default",
	Short: "Get the default org for the current backend",
	Long: "Get the default org for the current backend.\n" +
		"\n" +
		"This command is used to get the default organization for which and stacks are created in " +
		"the current backend.\n" +
		"\n" +
		"Currently, only the managed and self-hosted backends support organizations.",
}
var pluginText = HelpText{
	Use:   "plugin",
	Short: "Manage language and resource provider plugins",
	Long: "Manage language and resource provider plugins.\n" +
		"\n" +
		"Pulumi uses dynamically loaded plugins as an extensibility mechanism for\n" +
		"supporting any number of languages and resource providers.  These plugins are\n" +
		"distributed out of band and must be installed manually.  Attempting to use a\n" +
		"package that provisions resources without the corresponding plugin will fail.\n" +
		"\n" +
		"You may write your own plugins, for example to implement custom languages or\n" +
		"resources, although most people will never need to do this.  To understand how to\n" +
		"write and distribute your own plugins, please consult the relevant documentation.\n" +
		"\n" +
		"The plugin family of commands provides a way of explicitly managing plugins.",
}
var pluginInstallText = HelpText{
	Use:   "install [KIND NAME [VERSION]]",
	Short: "Install one or more plugins",
	Long: "Install one or more plugins.\n" +
		"\n" +
		"This command is used manually install plugins required by your program.  It may\n" +
		"be run either with a specific KIND, NAME, and VERSION, or by omitting these and\n" +
		"letting Pulumi compute the set of plugins that may be required by the current\n" +
		"project. If specified VERSION cannot be a range: it must be a specific number.\n" +
		"\n" +
		"If you let Pulumi compute the set to download, it is conservative and may end up\n" +
		"downloading more plugins than is strictly necessary.",
}
var pluginLsText = HelpText{
	Use:   "ls",
	Short: "List plugins",
}
var pluginRmText = HelpText{
	Use:   "rm [KIND [NAME [VERSION]]]",
	Short: "Remove one or more plugins from the download cache",
	Long: "Remove one or more plugins from the download cache.\n" +
		"\n" +
		"Specify KIND, NAME, and/or VERSION to narrow down what will be removed.\n" +
		"If none are specified, the entire cache will be cleared.  If only KIND and\n" +
		"NAME are specified, but not VERSION, all versions of the plugin with the\n" +
		"given KIND and NAME will be removed.  VERSION may be a range.\n" +
		"\n" +
		"This removal cannot be undone.  If a deleted plugin is subsequently required\n" +
		"in order to execute a Pulumi program, it must be re-downloaded and installed\n" +
		"using the plugin install command.",
}
var policyDisableText = HelpText{
	Use:   "disable <org-name>/<policy-pack-name>",
	Short: "Disable a Policy Pack for a Pulumi organization",
	Long:  "Disable a Policy Pack for a Pulumi organization",
}
var policyEnableText = HelpText{
	Use:   "enable <org-name>/<policy-pack-name> <latest|version>",
	Short: "Enable a Policy Pack for a Pulumi organization",
	Long: "Enable a Policy Pack for a Pulumi organization. " +
		"Can specify latest to enable the latest version of the Policy Pack or a specific version number.",
}
var policyLsText = HelpText{
	Use:   "ls [org-name]",
	Short: "List all Policy Packs for a Pulumi organization",
	Long:  "List all Policy Packs for a Pulumi organization",
}
var policyNewText = HelpText{
	Use:   "new [template|url]",
	Short: "Create a new Pulumi Policy Pack",
	Long: "Create a new Pulumi Policy Pack from a template.\n" +
		"\n" +
		"To create a Policy Pack from a specific template, pass the template name (such as `aws-typescript`\n" +
		"or `azure-python`).  If no template name is provided, a list of suggested templates will be presented\n" +
		"which can be selected interactively.\n" +
		"\n" +
		"Once you're done authoring the Policy Pack, you will need to publish the pack to your organization.\n" +
		"Only organization administrators can publish a Policy Pack.",
}
var policyPublishText = HelpText{
	Use:   "publish [org-name]",
	Short: "Publish a Policy Pack to the Pulumi service",
	Long: "Publish a Policy Pack to the Pulumi service\n" +
		"\n" +
		"If an organization name is not specified, the current user account is used.",
}
var policyRmText = HelpText{
	Use:   "rm <org-name>/<policy-pack-name> <all|version>",
	Short: "Removes a Policy Pack from a Pulumi organization",
	Long: "Removes a Policy Pack from a Pulumi organization. " +
		"The Policy Pack must be disabled from all Policy Groups before it can be removed.",
}
var policyValidateConfigText = HelpText{
	Use:   "validate-config <org-name>/<policy-pack-name> <version>",
	Short: "Validate a Policy Pack configuration",
	Long:  "Validate a Policy Pack configuration against the configuration schema of the specified version.",
}
var previewText = HelpText{
	Use:   "preview",
	Short: "Show a preview of updates to a stack's resources",
	Long: "Show a preview of updates a stack's resources.\n" +
		"\n" +
		"This command displays a preview of the updates to an existing stack whose state is\n" +
		"represented by an existing state file. The new desired state is computed by running\n" +
		"a Pulumi program, and extracting all resource allocations from its resulting object graph.\n" +
		"These allocations are then compared against the existing state to determine what\n" +
		"operations must take place to achieve the desired state. No changes to the stack will\n" +
		"actually take place.\n" +
		"\n" +
		"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
		"`--cwd` flag to use a different directory.",
}
var pulumiText = HelpText{
	Use:   "pulumi",
	Short: "Pulumi command line",
	Long: "Pulumi - Modern Infrastructure as Code\n" +
		"\n" +
		"To begin working with Pulumi, run the `pulumi new` command:\n" +
		"\n" +
		"    $ pulumi new\n" +
		"\n" +
		"This will prompt you to create a new project for your cloud and language of choice.\n" +
		"\n" +
		"The most common commands from there are:\n" +
		"\n" +
		"    - pulumi up       : Deploy code and/or resource changes\n" +
		"    - pulumi stack    : Manage instances of your project\n" +
		"    - pulumi config   : Alter your stack's configuration or secrets\n" +
		"    - pulumi destroy  : Tear down your stack's resources entirely\n" +
		"\n" +
		"For more information, please visit the project page: https://www.pulumi.com/docs/",
}
var queryText = HelpText{
	Use:   "query",
	Short: "Run query program against cloud resources",
	Long: "[EXPERIMENTAL] Run query program against cloud resources.\n" +
		"\n" +
		"This command loads a Pulumi query program and executes it. In \"query mode\", Pulumi provides various\n" +
		"useful data sources for querying, such as the resource outputs for a stack. Query mode also disallows\n" +
		"all resource operations, so users cannot declare resource definitions as they would in normal Pulumi\n" +
		"programs.\n" +
		"\n" +
		"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
		"`--cwd` flag to use a different directory.",
}
var refreshText = HelpText{
	Use:   "refresh",
	Short: "Refresh the resources in a stack",
	Long: "Refresh the resources in a stack.\n" +
		"\n" +
		"This command compares the current stack's resource state with the state known to exist in\n" +
		"the actual cloud provider. Any such changes are adopted into the current stack. Note that if\n" +
		"the program text isn't updated accordingly, subsequent updates may still appear to be out of\n" +
		"synch with respect to the cloud provider's source of truth.\n" +
		"\n" +
		"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
		"`--cwd` flag to use a different directory.",
}
var replayEventsText = HelpText{
	Use:   "replay-events [kind] [events-file]",
	Short: "Replay events from a prior update, refresh, or destroy",
	Long: "Replay events from a prior update, refresh, or destroy.\n" +
		"\n" +
		"This command is used to replay events emitted by a prior\n" +
		"invocation of the Pulumi CLI (e.g. `pulumi up --event-log [file]`).\n" +
		"\n" +
		"This command loads events from the indicated file and renders them\n" +
		"using either the progress view or the diff view.\n",
}
var checkText = HelpText{
	Use:   "check",
	Short: "Check a Pulumi package schema for errors",
	Long: "Check a Pulumi package schema for errors.\n" +
		"\n" +
		"Ensure that a Pulumi package schema meets the requirements imposed by the\n" +
		"schema spec as well as additional requirements imposed by the supported\n" +
		"target languages.",
}
var schemaText = HelpText{
	Use:   "schema",
	Short: "Analyze package schemas",
	Long: `Analyze package schemas

	Subcommands of this command can be used to analyze Pulumi package schemas. This can be useful to check hand-authored
	package schemas for errors.`,
}
var changeSecretsProviderText = HelpText{
	Use:   "change-secrets-provider <new-secrets-provider>",
	Short: "Change the secrets provider for a stack",
	Long: "Change the secrets provider for a stack. " +
		"Valid secret providers types are `default`, `passphrase`, `awskms`, `azurekeyvault`, `gcpkms`, `hashivault`.\n\n" +
		"To change to using the Pulumi Default Secrets Provider, use the following:\n" +
		"\n" +
		"pulumi stack change-secrets-provider default" +
		"\n" +
		"\n" +
		"To change the stack to use a cloud secrets backend, use one of the following:\n" +
		"\n" +
		"* `pulumi stack change-secrets-provider \"awskms://alias/ExampleAlias?region=us-east-1\"" +
		"`\n" +
		"* `pulumi stack change-secrets-provider " +
		"\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
		"* `pulumi stack change-secrets-provider " +
		"\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
		"* `pulumi stack change-secrets-provider " +
		"\"gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>\"`\n" +
		"* `pulumi stack change-secrets-provider \"hashivault://mykey\"`",
}
var exportText = HelpText{
	Use:   "export",
	Short: "Export a stack's deployment to standard out",
	Long: "Export a stack's deployment to standard out.\n" +
		"\n" +
		"The deployment can then be hand-edited and used to update the stack via\n" +
		"`pulumi stack import`. This process may be used to correct inconsistencies\n" +
		"in a stack's state due to failed deployments, manual changes to cloud\n" +
		"resources, etc.",
}
var stackText = HelpText{
	Use:   "stack",
	Short: "Manage stacks",
	Long: "Manage stacks\n" +
		"\n" +
		"A stack is a named update target, and a single project may have many of them.\n" +
		"Each stack has a configuration and update history associated with it, stored in\n" +
		"the workspace, in addition to a full checkpoint of the last known good update.\n",
}
var graphText = HelpText{
	Use:   "graph [filename]",
	Short: "Export a stack's dependency graph to a file",
	Long: "Export a stack's dependency graph to a file.\n" +
		"\n" +
		"This command can be used to view the dependency graph that a Pulumi program\n" +
		"emitted when it was run. This graph is output in the DOT format. This command operates\n" +
		"on your stack's most recent deployment.",
}
var historyText = HelpText{
	Use:   "history",
	Short: "Display history for a stack",
	Long: `Display history for a stack

	This command displays data about previous updates for a stack.`,
}
var stackImportText = HelpText{
	Use:   "import",
	Short: "Import a deployment from standard in into an existing stack",
	Long: "Import a deployment from standard in into an existing stack.\n" +
		"\n" +
		"A deployment that was exported from a stack using `pulumi stack export` and\n" +
		"hand-edited to correct inconsistencies due to failed updates, manual changes\n" +
		"to cloud resources, etc. can be reimported to the stack using this command.\n" +
		"The updated deployment will be read from standard in.",
}
var stackInitText = HelpText{
	Use:   "init [<org-name>/]<stack-name>",
	Short: "Create an empty stack with the given name, ready for updates",
	Long: "Create an empty stack with the given name, ready for updates\n" +
		"\n" +
		"This command creates an empty stack with the given name.  It has no resources,\n" +
		"but afterwards it can become the target of a deployment using the `update` command.\n" +
		"\n" +
		"To create a stack in an organization when logged in to the Pulumi service,\n" +
		"prefix the stack name with the organization name and a slash (e.g. 'acmecorp/dev')\n" +
		"\n" +
		"By default, a stack created using the pulumi.com backend will use the pulumi.com secrets\n" +
		"provider and a stack created using the local or cloud object storage backend will use the\n" +
		"`passphrase` secrets provider.  A different secrets provider can be selected by passing the\n" +
		"`--secrets-provider` flag.\n" +
		"\n" +
		"To use the `passphrase` secrets provider with the pulumi.com backend, use:\n" +
		"\n" +
		"* `pulumi stack init --secrets-provider=passphrase`\n" +
		"\n" +
		"To use a cloud secrets provider with any backend, use one of the following:\n" +
		"\n" +
		"* `pulumi stack init --secrets-provider=\"awskms://alias/ExampleAlias?region=us-east-1\"`\n" +
		"* `pulumi stack init --secrets-provider=\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
		"* `pulumi stack init --secrets-provider=\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
		"* `pulumi stack init --secrets-provider=\"gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>\"`\n" +
		"* `pulumi stack init --secrets-provider=\"hashivault://mykey\"\n`" +
		"\n" +
		"A stack can be created based on the configuration of an existing stack by passing the\n" +
		"`--copy-config-from` flag.\n" +
		"* `pulumi stack init --copy-config-from dev`",
}
var stackLsText = HelpText{
	Use:   "ls",
	Short: "List stacks",
	Long: "List stacks\n" +
		"\n" +
		"This command lists stacks. By default only stacks with the same project name as the\n" +
		"current workspace will be returned. By passing --all, all stacks you have access to\n" +
		"will be listed.\n" +
		"\n" +
		"Results may be further filtered by passing additional flags. Tag filters may include\n" +
		"the tag name as well as the tag value, separated by an equals sign. For example\n" +
		"'environment=production' or just 'gcp:project'.",
}
var stackOutputText = HelpText{
	Use:   "output [property-name]",
	Short: "Show a stack's output properties",
	Long: "Show a stack's output properties.\n" +
		"\n" +
		"By default, this command lists all output properties exported from a stack.\n" +
		"If a specific property-name is supplied, just that property's value is shown.",
}
var stackRenameText = HelpText{
	Use:   "rename <new-stack-name>",
	Short: "Rename an existing stack",
	Long: "Rename an existing stack.\n" +
		"\n" +
		"Note: Because renaming a stack will change the value of `getStack()` inside a Pulumi program, if this\n" +
		"name is used as part of a resource's name, the next `pulumi up` will want to delete the old resource and\n" +
		"create a new copy. For now, if you don't want these changes to be applied, you should rename your stack\n" +
		"back to its previous name." +
		"\n" +
		"You can also rename the stack's project by passing a fully-qualified stack name as well. For example:\n" +
		"'robot-co/new-project-name/production'. However in order to update the stack again, you would also need\n" +
		"to update the name field of Pulumi.yaml, so the project names match.",
}
var stackRmText = HelpText{
	Use:   "rm [<stack-name>]",
	Short: "Remove a stack and its configuration",
	Long: "Remove a stack and its configuration\n" +
		"\n" +
		"This command removes a stack and its configuration state.  Please refer to the\n" +
		"`destroy` command for removing a resources, as this is a distinct operation.\n" +
		"\n" +
		"After this command completes, the stack will no longer be available for updates.",
}
var stackSelectText = HelpText{
	Use:   "select [<stack>]",
	Short: "Switch the current workspace to the given stack",
	Long: "Switch the current workspace to the given stack.\n" +
		"\n" +
		"Selecting a stack allows you to use commands like `config`, `preview`, and `update`\n" +
		"without needing to type the stack name each time.\n" +
		"\n" +
		"If no <stack> argument is supplied, you will be prompted to select one interactively.\n" +
		"If provided stack name is not found you may pass the --create flag to create and select it",
}
var stackTagText = HelpText{
	Use:   "tag",
	Short: "Manage stack tags",
	Long: "Manage stack tags\n" +
		"\n" +
		"Stacks have associated metadata in the form of tags. Each tag consists of a name\n" +
		"and value. The `get`, `ls`, `rm`, and `set` commands can be used to manage tags.\n" +
		"Some tags are automatically assigned based on the environment each time a stack\n" +
		"is updated.\n",
}
var stackTagGetText = HelpText{
	Use:   "get <name>",
	Short: "Get a single stack tag value",
}
var stackTagLsText = HelpText{
	Use:   "ls",
	Short: "List all stack tags",
}
var stackTagRmText = HelpText{
	Use:   "rm <name>",
	Short: "Remove a stack tag",
}
var stackTagSetText = HelpText{
	Use:   "set <name> <value>",
	Short: "Set a stack tag",
}
var stackUnselectText = HelpText{
	Use:   "unselect",
	Short: "Resets stack selection from the current workspace",
	Long: "Resets stack selection from the current workspace.\n" +
		"\n" +
		"This way, next time pulumi needs to execute an operation, the user is prompted with one of the stacks to select\n" +
		"from.\n",
}
var stateDeleteText = HelpText{
	Use:   "delete <resource URN>",
	Short: "Deletes a resource from a stack's state",
	Long: `Deletes a resource from a stack's state

	This command deletes a resource from a stack's state, as long as it is safe to do so. The resource is specified 
	by its Pulumi URN (use ` + "`pulumi stack --show-urns`" + ` to get it).

	Resources can't be deleted if there exist other resources that depend on it or are parented to it. Protected resources 
	will not be deleted unless it is specifically requested using the --force flag.

	Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.

	Example:
	pulumi state delete 'urn:pulumi:stage::demo::eks:index:Cluster$pulumi:providers:kubernetes::eks-provider'
	`,
}
var stateText = HelpText{
	Use:   "state",
	Short: "Edit the current stack's state",
	Long: `Edit the current stack's state

	Subcommands of this command can be used to surgically edit parts of a stack's state. These can be useful when
	troubleshooting a stack or when performing specific edits that otherwise would require editing the state file by hand.`,
}
var stateRenameText = HelpText{
	Use:   "rename <resource URN> <new name>",
	Short: "Renames a resource from a stack's state",
	Long: `Renames a resource from a stack's state

	This command renames a resource from a stack's state. The resource is specified 
	by its Pulumi URN (use ` + "`pulumi stack --show-urns`" + ` to get it) and the new name of the resource.

	Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.

	Example:
	pulumi state rename 'urn:pulumi:stage::demo::eks:index:Cluster$pulumi:providers:kubernetes::eks-provider' new-name-here
	`,
}
var stateUnprotectText = HelpText{
	Use:   "unprotect <resource URN>",
	Short: "Unprotect resources in a stack's state",
	Long: `Unprotect resource in a stack's state

	This command clears the 'protect' bit on one or more resources, allowing those resources to be deleted.`,
}
var upText = HelpText{
	Use:   "up [template|url]",
	Short: "Create or update the resources in a stack",
	Long: "Create or update the resources in a stack.\n" +
		"\n" +
		"This command creates or updates resources in a stack. The new desired goal state for the target stack\n" +
		"is computed by running the current Pulumi program and observing all resource allocations to produce a\n" +
		"resource graph. This goal state is then compared against the existing state to determine what create,\n" +
		"read, update, and/or delete operations must take place to achieve the desired goal state, in the most\n" +
		"minimally disruptive way. This command records a full transactional snapshot of the stack's new state\n" +
		"afterwards so that the stack may be updated incrementally again later on.\n" +
		"\n" +
		"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
		"`--cwd` flag to use a different directory.",
}
var versionText = HelpText{
	Use:   "version",
	Short: "Print Pulumi's version number",
}
var viewTraceText = HelpText{
	Use:   "view-trace [trace-file]",
	Short: "Display a trace from the Pulumi CLI",
	Long: "Display a trace from the Pulumi CLI.\n" +
		"\n" +
		"This command is used to display execution traces collected by a prior\n" +
		"invocation of the Pulumi CLI.\n" +
		"\n" +
		"This command loads trace data from the indicated file and starts a\n" +
		"webserver to display the trace. By default, this server will listen\n" +
		"port 8008; the --port flag can be used to change this if necessary.",
}
var watchText = HelpText{
	Use:   "watch",
	Short: "Continuously update the resources in a stack",
	Long: "[EXPERIMENTAL] Continuously update the resources in a stack.\n" +
		"\n" +
		"This command watches the working directory or specified paths for the current project and updates\n" +
		"the active stack whenever the project changes.  In parallel, logs are collected for all resources\n" +
		"in the stack and displayed along with update progress.\n" +
		"\n" +
		"The program to watch is loaded from the project in the current directory by default. Use the `-C` or\n" +
		"`--cwd` flag to use a different directory.",
}
var whoamiText = HelpText{
	Use:   "whoami",
	Short: "Display the current logged-in user",
	Long: "Display the current logged-in user\n" +
		"\n" +
		"Displays the username of the currently logged in user.",
}
