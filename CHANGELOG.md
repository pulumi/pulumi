# Changelog

## 3.94.0 (2023-11-14)


### Features

- [engine] `import` can now create empty component resource to use as the parent of other imported resources.
  [#14467](https://github.com/pulumi/pulumi/pull/14467)

- [engine] `import` can now import a parent resource in the same deployment as a child resource.
  [#14461](https://github.com/pulumi/pulumi/pull/14461)

- [engine] Import files no longer need parent URNs in the name table for resource being imported in the same file.
  [#14524](https://github.com/pulumi/pulumi/pull/14524)

- [cli/config] `config refresh` will now restore secret provider config from the last deployment.
  [#13900](https://github.com/pulumi/pulumi/pull/13900)

- [cli/new] Simplifies URL parsing for pulumi new zip
  [#14546](https://github.com/pulumi/pulumi/pull/14546)


### Bug Fixes

- [components/yaml] Upgrade yaml to 1.4.1
  [#14542](https://github.com/pulumi/pulumi/pull/14542)

- [engine] Ignore spurious error from Kubernetes providers DiffConfig method.
  [#14533](https://github.com/pulumi/pulumi/pull/14533)

- [sdk/python] Maintain old behavior for empty Kubernetes invoke results
  [#14535](https://github.com/pulumi/pulumi/pull/14535)

## 3.92.0 (2023-11-03)


### Features

- [auto] Allow shallow repository clones in NewLocalWorkspace
  [#14312](https://github.com/pulumi/pulumi/pull/14312)

- [cli] Add support for ESC file projection
  [#14447](https://github.com/pulumi/pulumi/pull/14447)

- [cli/new] Adds support for remote zip archive templates to pulumi new
  [#14443](https://github.com/pulumi/pulumi/pull/14443)

- [engine] Support {NAME} in http plugin download URLs.
  [#14435](https://github.com/pulumi/pulumi/pull/14435)

- [yaml] Update pulumi-yaml to 1.4.0
  [#14425](https://github.com/pulumi/pulumi/pull/14425)

- [auto/nodejs] Add `refresh` option for `up`
  [#14306](https://github.com/pulumi/pulumi/pull/14306)


### Bug Fixes

- [cli/new] Adds nested directory support to pulumi new .zip
  [#14473](https://github.com/pulumi/pulumi/pull/14473)

- [auto/nodejs] Pin @grpc/grpc-js to v1.9.6 to resolve automation-api hang in NodeJS.
  [#14445](https://github.com/pulumi/pulumi/pull/14445)

- [engine] Correctly propogate provider errors from DiffConfig.
  [#14436](https://github.com/pulumi/pulumi/pull/14436)

- [engine] Fix parsing of property paths such as "root.[1]" being returned from providers.
  [#14451](https://github.com/pulumi/pulumi/pull/14451)

- [programgen/go] Fix using inline invoke expressions inside resources, objects and arrays
  [#14484](https://github.com/pulumi/pulumi/pull/14484)

- [sdk/python] Fix error on empty invoke returns
  [#14470](https://github.com/pulumi/pulumi/pull/14470)

- [sdk/python] Fix traceback diagnostic from being printed when using Python dynamic providers
  [#14474](https://github.com/pulumi/pulumi/pull/14474)


### Miscellaneous

- [ci] Bump homebrew using pulumi's fork instead of pulumi-bot's
  [#14449](https://github.com/pulumi/pulumi/pull/14449)

- [ci] Additional fixes for the homebrew release job
  [#14482](https://github.com/pulumi/pulumi/pull/14482)

- [cli] Pull in fixes from esc v0.5.7
  [#14430](https://github.com/pulumi/pulumi/pull/14430)

## 3.91.1 (2023-10-27)


### Bug Fixes

- [cli/display] Fix misleading output in stack ls --json
  [#14309](https://github.com/pulumi/pulumi/pull/14309)

- [sdkgen/python] Fix regression where constructing ResourceArgs would fail if required arguments were missing.
  [#14427](https://github.com/pulumi/pulumi/pull/14427)

## 3.91.0 (2023-10-25)


### Features

- [cli] Adds a new `pulumi install` command which will install packages and plugins for a project.
  [#13081](https://github.com/pulumi/pulumi/pull/13081)


### Bug Fixes

- [engine] Fix generation of property paths in diff.
  [#14337](https://github.com/pulumi/pulumi/pull/14337)

## 3.90.1 (2023-10-24)


### Bug Fixes

- [cli/config] Don't crash on empty config values
  [#14328](https://github.com/pulumi/pulumi/pull/14328)

- [sdkgen/python] Fix issue calling nonexistent `_configure` method on external types
  [#14318](https://github.com/pulumi/pulumi/pull/14318)

- [sdkgen/python] Fix calling `_configure` with an Output value
  [#14321](https://github.com/pulumi/pulumi/pull/14321)

## 3.90.0 (2023-10-23)


### Features

- [auto/nodejs] Add support for the path option for config operations
  [#14305](https://github.com/pulumi/pulumi/pull/14305)

- [engine] Converters can return diagnostics from `ConvertState`.
  [#14135](https://github.com/pulumi/pulumi/pull/14135)


### Bug Fixes

- [cli] Tightened the parser for property paths to be less prone to typos
  [#14257](https://github.com/pulumi/pulumi/pull/14257)

- [engine] Fix handling of explicit providers and --target-dependents.
  [#14238](https://github.com/pulumi/pulumi/pull/14238)

- [engine] Fix automatic diffs comparing against output instead of input properties.
  [#14256](https://github.com/pulumi/pulumi/pull/14256)

- [sdkgen/dotnet] Fix codegen with nested modules.
  [#14297](https://github.com/pulumi/pulumi/pull/14297)

- [programgen/go] Fix codegen to correctly output pulumi.Array instead of pulumi.AnyArray
  [#14299](https://github.com/pulumi/pulumi/pull/14299)

- [cli/new] `pulumi new` now allows users to bypass existing project name checks.
  [#14081](https://github.com/pulumi/pulumi/pull/14081)

- [sdk/nodejs] Nodejs now supports unknown resource IDs.
  [#14137](https://github.com/pulumi/pulumi/pull/14137)

- [sdkgen/python] Fix `_configure` failing due to required args mismatch.
  [#14281](https://github.com/pulumi/pulumi/pull/14281)


### Miscellaneous

- [cli] Pull in fixes from esc v0.5.6
  [#14284](https://github.com/pulumi/pulumi/pull/14284)

- [protobuf] Add a config as property map field to RunRequest and pass that to the SDK
  [#14273](https://github.com/pulumi/pulumi/pull/14273)

- [sdk/python] updates grpcio dependency
  [#14259](https://github.com/pulumi/pulumi/pull/14259)

## 3.89.0 (2023-10-16)


### Features

- [engine] Old inputs are sent to provider Delete functions, as well as the old outputs.
  [#14051](https://github.com/pulumi/pulumi/pull/14051)


### Bug Fixes

- [engine] Fix a panic in the engine when same steps failed due to provider errors.
  [#14076](https://github.com/pulumi/pulumi/pull/14076)

- [engine] Engine is now more efficent about starting up provider processes, generally saving at least one process startup per deployment.
  [#14127](https://github.com/pulumi/pulumi/pull/14127)

- [programgen] Fixes panic when binding the signature of output-versioned invokes without input arguments
  [#14234](https://github.com/pulumi/pulumi/pull/14234)

- [sdkgen/python] Python SDK generation _configure now correctly handles original property names for resource arguments (i.e. user provides `propName` instead of `prop_name`).
  [#14235](https://github.com/pulumi/pulumi/pull/14235)

## 3.88.1 (2023-10-11)


### Bug Fixes

- [cli] allow unmarshalling nil as a config value.
  [#14149](https://github.com/pulumi/pulumi/pull/14149)

- [auto/nodejs] Remove unneeded SxS check for inline programs
  [#14154](https://github.com/pulumi/pulumi/pull/14154)


### Miscellaneous

- [cli] Pull in fixes from esc v0.5.2
  [#14155](https://github.com/pulumi/pulumi/pull/14155)


## 3.88.0 (2023-10-10)


### Features

- [engine] Add the new policy remediations feature.
  [#14080](https://github.com/pulumi/pulumi/pull/14080)

- [auto] Added a tracing span for plugin launch
  [#14100](https://github.com/pulumi/pulumi/pull/14100)

### Bug Fixes

- [cli/package] Fix a panic in get-mapping when not passing a provider name.
  [#14124](https://github.com/pulumi/pulumi/pull/14124)

- [engine] Engine will now error earlier if a deployment needs a bundled plugin that is missing.
  [#14103](https://github.com/pulumi/pulumi/pull/14103)

- [sdk/{go,nodejs,python}] Fix MockMonitor reporting DeletedWith wasn't supported
  [#14118](https://github.com/pulumi/pulumi/pull/14118)

- [programgen/python] Fix panic in python program-gen when rewriting index expressions
  [#14099](https://github.com/pulumi/pulumi/pull/14099)

## 3.87.0 (2023-10-06)


### Features

- [cli] Users can now set `PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION` to disable the engine trying to auto install missing plugins.
  [#14083](https://github.com/pulumi/pulumi/pull/14083)

- [pkg] Upgrade pulumi-java to v0.9.8

- [cli/import] Import converters will write out their intermediate import file for manual fixups if needed.
  [#14067](https://github.com/pulumi/pulumi/pull/14067)

- [sdkgen/go] Generate output-versioned invokes for functions without inputs
  [#13694](https://github.com/pulumi/pulumi/pull/13694)

- [sdk/python] Add `default` arg to `Config.get_secret`
  [#12279](https://github.com/pulumi/pulumi/pull/12279)


### Bug Fixes

- [cli] policy publish: default to default-org if possible
  [#14090](https://github.com/pulumi/pulumi/pull/14090)

- [cli] Fix a panic in `whoami` with tokens missing expected information.

- [engine] Calling RegisterResourceOutputs twice no longer panics and returns an error instead.
  [#14062](https://github.com/pulumi/pulumi/pull/14062)

- [engine] ComponentResources now emit resourceOutputEvent on Deletion. This fixes the time elapsed timer not ending when the resource is deleted.
  [#14061](https://github.com/pulumi/pulumi/pull/14061)

- [engine] Lifecycle tests shouldn't reuse a closed plugin host.
  [#14063](https://github.com/pulumi/pulumi/pull/14063)

- [engine] ctrl-c should cause Pulumi to send cancellation signal to providers
  [#14057](https://github.com/pulumi/pulumi/pull/14057)

- [engine] Fix a race condition in the engine access step event data.
  [#14049](https://github.com/pulumi/pulumi/pull/14049)

- [engine] Restore elided asset contents into returned inputs and state from Read operations

- [engine] `DISABLE_AUTOMATIC_PLUGIN_ACQUISITION` is respected for deployment operations now.
  [#14104](https://github.com/pulumi/pulumi/pull/14104)

- [programgen] `pulumi import` supports resources with duplicate names, it will fail if referenced as a provider/parent due to the ambiguity.
  [#13989](https://github.com/pulumi/pulumi/pull/13989)

- [programgen/dotnet] Fixes compiling an empty list of expressions from object properties
  [#14027](https://github.com/pulumi/pulumi/pull/14027)

## 3.86.0 (2023-09-26)


### Features

- [cli/about] `pulumi about` and `pulumi whoami` will now return information about the access token used to login to the service.
  [#13206](https://github.com/pulumi/pulumi/pull/13206)


### Bug Fixes

- [cli] Add filestate variables to `pulumi env`.
  [#14015](https://github.com/pulumi/pulumi/pull/14015)

- [cli] Include a newline in `pulumi whoami`'s output
  [#14025](https://github.com/pulumi/pulumi/pull/14025)

- [cli/import] `import --from=plugin` will now try to auto-install the plugin if missing.
  [#14048](https://github.com/pulumi/pulumi/pull/14048)

- [engine] Fix creation and modification timestamps sometimes not setting.
  [#14038](https://github.com/pulumi/pulumi/pull/14038)

- [engine] Fixes the engine using aliases from old deployments when writing out statefiles.

- [sdk/python] Resource property parameters are now runtime type checked to ensure they are a `Mapping` object.
  [#14030](https://github.com/pulumi/pulumi/pull/14030)

## 3.85.0 (2023-09-22)


### Features

- [engine] Provider mapping information lookups are now more efficient. Providers can also support multiple mappings.
  [#13975](https://github.com/pulumi/pulumi/pull/13975)

- [sdkgen/python] Generate output-versioned invokes for functions without inputs
  [#13685](https://github.com/pulumi/pulumi/pull/13685)


### Bug Fixes

- [sdkgen/dotnet] Fixes potential conflicts when generating resources called System
  [#14011](https://github.com/pulumi/pulumi/pull/14011)

- [cli/engine] Errors writing out snapshots now print error messages to be seen by users.
  [#14016](https://github.com/pulumi/pulumi/pull/14016)

- [sdk/go] Ensure Assets of AssetArchive are non-nil when creating and deserializing
  [#14007](https://github.com/pulumi/pulumi/pull/14007)

- [cli/new] Fix selector wrapping on narrow terminals.
  [#13979](https://github.com/pulumi/pulumi/pull/13979)

- [sdkgen/python] Fix error calling _configure when the value is None
  [#14014](https://github.com/pulumi/pulumi/pull/14014)

## 3.84.0 (2023-09-19)


### Features

- [engine] Program conversion plugins can now be passed extra arguments from `convert`.
  [#13973](https://github.com/pulumi/pulumi/pull/13973)

- [sdkgen/go] Support generating Go SDKs that use generic input and output types
  [#13828](https://github.com/pulumi/pulumi/pull/13828)


### Bug Fixes

- [cli/new] `pulumi new` no longer defaults to a project name of "pulum" if ran in a folder called "pulumi".
  [#13953](https://github.com/pulumi/pulumi/pull/13953)

## 3.83.0 (2023-09-15)


### Features

- [engine] pulumi-test-language can now be used to test language runtimes against a standard suite of tests.
  [#13705](https://github.com/pulumi/pulumi/pull/13705)


### Bug Fixes

- [cli] Fixes panic when default org is not set and no org is provided to org search
  [#13947](https://github.com/pulumi/pulumi/pull/13947)

- [engine] Fix aliases of parents tracking over partial deployments.
  [#13935](https://github.com/pulumi/pulumi/pull/13935)

- [sdkgen/python] Python sdkgen now correctly sets default values on dicts passed as resource arguments.
  [#13825](https://github.com/pulumi/pulumi/pull/13825)

## 3.82.1 (2023-09-12)


### Bug Fixes

- [cli/config] Allows org search for individual accounts
  [#13930](https://github.com/pulumi/pulumi/pull/13930)

- [sdkgen/{go,nodejs,python}] Fix a bug in marshalling enums across gRPC

- [cli/state] `pulumi state edit` now handles multi-part EDITOR env vars (i.e. `emacs -nw`).
  [#13922](https://github.com/pulumi/pulumi/pull/13922)

- [programgen/python] Fix deprecation warning triggering on ResourceArgs with default values.
  [#13890](https://github.com/pulumi/pulumi/pull/13890)

## 3.82.0 (2023-09-12)


### Features

- [cli] Adds `pulumi org search` and `pulumi org search ai` for Pulumi Insights in the CLI. These commands render a table containing all resources in a given organization matching the query provided.

  `-q <query>` will search for resources in the organization using a query provided in Pulumi Query Syntax.

  `-o <json|csv|yaml>` flag customizes the output.

  The `ai` command uses AI Assist to translate a natural language query into Pulumi Query Syntax.

  Default table output will show a count of displayed resources out of the total. Additional output includes the query run, a URL to view and explore search results in the Pulumi Console and the query, and the query run.

  Additional output is suppressed for non-table output formats such that they can be easily piped into other tools.

  The `--web` flag will open the search results in a default browser.
  [#13611](https://github.com/pulumi/pulumi/pull/13611)
  [#13879](https://github.com/pulumi/pulumi/pull/13879)
  [#13888](https://github.com/pulumi/pulumi/pull/13888)
  [#13846](https://github.com/pulumi/pulumi/pull/13846)

- [cli] Adds `pulumi ai` command - currently the only functionality in this group is `pulumi ai web`, which will open the Pulumi AI application in a default browser. An optional `--prompt/-p` flag can be provided with a query to pre-populate the search bar in the Pulumi AI application. By default, that prompt will be submitted automatically, but passing `--no-auto-submit` will prevent that.
  [#13808](https://github.com/pulumi/pulumi/pull/13808)
  [#13846](https://github.com/pulumi/pulumi/pull/13846)

- [engine] Support SDKs sending plugin checksums as part of resource requests.
  [#13789](https://github.com/pulumi/pulumi/pull/13789)


### Bug Fixes

- [cli/new] Fixes `pulumi policy new <template-name>` to not require `--yes` when run non-interactively.
  [#13902](https://github.com/pulumi/pulumi/pull/13902)

## 3.81.0 (2023-09-06)


### Features

- [cli] Pass args from import to state converters.
  [#13862](https://github.com/pulumi/pulumi/pull/13862)

- [cli/config] Removes PULUMI_DEV flag for org search
  [#13888](https://github.com/pulumi/pulumi/pull/13888)

- [sdkgen/python] Opting into pyproject.toml SDK generation no longer generates setup.py, but instead generates a standalone pyproject.toml that can be built with `python -m build .`
  [#13812](https://github.com/pulumi/pulumi/pull/13812)


### Bug Fixes

- [engine] Check for old resources first by URN and then aliases
  [#13883](https://github.com/pulumi/pulumi/pull/13883)

- [sdk/nodejs] Fix a possible panic in running NodeJS programs.
  [#13689](https://github.com/pulumi/pulumi/pull/13689)


### Miscellaneous

- [sdk/go] Support multi-errors built from errors.Join for RunFunc, Exit, and friends.
  [#13813](https://github.com/pulumi/pulumi/pull/13813)

- [sdk/go] Rename Join in pulumix to Flatten.
  [#13882](https://github.com/pulumi/pulumi/pull/13882)

## 3.80.0 (2023-08-31)


### Features

- [sdk/go] Add sdk/go/pulumix package with type-safe generics-based APIs to interact with Pulumi's core functionality.
  [#13509](https://github.com/pulumi/pulumi/pull/13509)

- [sdk/go] Built-in Pulumi types all satisfy `pulumix.Input[T]` for their underlying types.
  [#13509](https://github.com/pulumi/pulumi/pull/13509)

- [sdkgen/go] Generate types that are compatible with sdk/go/pulumix's type-safe APIs.
  [#13510](https://github.com/pulumi/pulumi/pull/13510)

- [sdkgen/{nodejs,python}] NodeJS and Python packages no longer running plugin install scripts on package install.
  [#13800](https://github.com/pulumi/pulumi/pull/13800)


### Bug Fixes

- [cli/new] Fix regression where `pulumi new -s org/project/stack` would fail if the project already exists.
  [#13786](https://github.com/pulumi/pulumi/pull/13786)

- [sdk/nodejs] Fix provider used for resource methods
  [#13796](https://github.com/pulumi/pulumi/pull/13796)


### Miscellaneous

- [cli] Some CLI prompts now support backspace, arrow keys, etc.
  [#13815](https://github.com/pulumi/pulumi/pull/13815)

- [sdk/go] Add cmdutil.TerminateProcessGroup to terminate processes gracefully.
  [#13792](https://github.com/pulumi/pulumi/pull/13792)

- [sdk/{go,nodejs,python}] Language plugins now clean up resources and exit cleanly on receiving SIGINT or CTRL_BREAK signals.
  [#13809](https://github.com/pulumi/pulumi/pull/13809)

## 3.79.0 (2023-08-25)


### Features

- [engine] Support runtime plugins returning plugin checksums from GetRequiredPlugins.
  [#13776](https://github.com/pulumi/pulumi/pull/13776)

- [sdkgen/go] Implement option to override the name of the generated internal/utilities module
  [#13749](https://github.com/pulumi/pulumi/pull/13749)


### Bug Fixes

- [engine] Fix panic when analyzer plugin is not found on PATH.
  [#13767](https://github.com/pulumi/pulumi/pull/13767)

- [programgen/go] Fixes go.mod version requirement
  [#13729](https://github.com/pulumi/pulumi/pull/13729)

- [sdk/nodejs] When using closure serializtion, lookup package.json up from current working directory up to parent directories recursively
  [#13770](https://github.com/pulumi/pulumi/pull/13770)


### Miscellaneous

- [pkg] Update pulumi-yaml (v1.2.1 -> v1.2.2) pulumi-java (v0.9.0 -> v0.9.6) pulumi-dotnet (v3.54.0 -> v3.56.1)
  [#13763](https://github.com/pulumi/pulumi/pull/13763)

- [sdk/python] Update grpc-io from 1.56.0 to 1.56.2
  [#13759](https://github.com/pulumi/pulumi/pull/13759)

## 3.78.1 (2023-08-11)


### Features

- [yaml] Update pulumi-yaml to 1.2.1.
  [#13712](https://github.com/pulumi/pulumi/pull/13712)


### Bug Fixes

- [engine] Fixes some synchronization in plugin shutdown to prevent panics on Ctrl-C.
  [#13682](https://github.com/pulumi/pulumi/pull/13682)

- [sdkgen/python] Fixes rendering v1.2.3-alpha.8 style of versions to valid PyPI versions when respectSchemaVersions option is set in sdkgen.
  [#13707](https://github.com/pulumi/pulumi/pull/13707)


### Miscellaneous

- [ci] Add preliminary support for GitHub's native merge queues.
  [#13681](https://github.com/pulumi/pulumi/pull/13681)

- [engine] Pass Loader gRPC target to converter plugins.
  [#13696](https://github.com/pulumi/pulumi/pull/13696)

- [sdk/go] Updates aws-sdk-go to 1.44.298 to enable support for sso-session link AWS profiles.
  [#13619](https://github.com/pulumi/pulumi/pull/13619)

## 3.78.0 (2023-08-09)


### Features

- [yaml] Update pulumi-yaml to 1.2.0.
  [#13674](https://github.com/pulumi/pulumi/pull/13674)

- [sdkgen/dotnet] Generate output-versioned invokes for functions without inputs.
  [#13669](https://github.com/pulumi/pulumi/pull/13669)

- [sdkgen/nodejs] Generate output-versioned invokes for functions without inputs.
  [#13678](https://github.com/pulumi/pulumi/pull/13678)

- [cli/package] New experimental "pack-sdk" command to pack an SDK into an artifact.
  [#13675](https://github.com/pulumi/pulumi/pull/13675)

- [cli/plugin] CLI will now warn when loading ambient plugins from $PATH.
  [#13670](https://github.com/pulumi/pulumi/pull/13670)


### Bug Fixes

- [programgen/dotnet] Fixes code generation of ForExpressions, both when creating a list or a dictionary.
  [#13620](https://github.com/pulumi/pulumi/pull/13620)

- [programgen/dotnet] Fixes list initializer for plain lists in resource properties.
  [#13630](https://github.com/pulumi/pulumi/pull/13630)

- [programgen/{go,nodejs}] Fix a bug in marshalling type refs across gRPC.
  [#13676](https://github.com/pulumi/pulumi/pull/13676)

- [programgen/nodejs] Fixes parseProxyApply to handle nested outputs within index expressions.
  [#13629](https://github.com/pulumi/pulumi/pull/13629)

- [sdk/nodejs] Fix finding the pulumi package when the runtime wasn't started in the project directory.
  [#13639](https://github.com/pulumi/pulumi/pull/13639)

- [cli/plugin] Improve error message during `pulumi plugin install` if the plugin is bundled with Pulumi.
  [#12575](https://github.com/pulumi/pulumi/pull/12575)


### Miscellaneous

- [sdkgen/nodejs] Remove the pluginVersion and pluginName options from nodejs schema options.
  [#13646](https://github.com/pulumi/pulumi/pull/13646)

## 3.77.1 (2023-08-05)


### Bug Fixes

- [cli] Revert warning about ambient plugins loaded from $PATH #13607.
  [#13657](https://github.com/pulumi/pulumi/pull/13657)

## 3.77.0 (2023-08-04)


### Features

- [programgen/dotnet] Fix typing for optional and complex config variables in main program
  [#13590](https://github.com/pulumi/pulumi/pull/13590)

- [cli/new] Support SSH-style Git URLs, including private template repositories for `pulumi new`
  [#13515](https://github.com/pulumi/pulumi/pull/13515)

- [sdk/nodejs] NodeJS programs will now warn that undefined values will not show as stack outputs.
  [#13608](https://github.com/pulumi/pulumi/pull/13608)

- [cli/plugin] CLI will now warn when loading ambient plugins from $PATH.
  [#13607](https://github.com/pulumi/pulumi/pull/13607)


### Bug Fixes

- [cli] Several fixes for `pulumi logs` including support for first-class providers, support for ambient credentials and improved error reporting.
  [#13588](https://github.com/pulumi/pulumi/pull/13588)

- [cli/state] Fix panic in `pulumi state edit` when no stack is selected.
  [#13638](https://github.com/pulumi/pulumi/pull/13638)

- [engine] Language plugins now defer schema loading to the engine via a gRPC interface.
  [#13605](https://github.com/pulumi/pulumi/pull/13605)

- [programgen/{dotnet,go,nodejs,python}] Normalize the declaration name of generated resource components
  [#13606](https://github.com/pulumi/pulumi/pull/13606)

- [sdk/python] `Output.from_input` now recurses into tuples.
  [#13603](https://github.com/pulumi/pulumi/pull/13603)

- [sdkgen] Fix bug binding provider schema where type default int values could not take integers.
  [#13599](https://github.com/pulumi/pulumi/pull/13599)

- [sdkgen/python] Fixes python external enum types missing the import reference to the external package.
  [#13584](https://github.com/pulumi/pulumi/pull/13584)


### Miscellaneous

- [sdk/go] Move some types to an internal package, re-exporting them from sdk/go/pulumi. This should have no meaningful effect on users of these APIs.
  [#13495](https://github.com/pulumi/pulumi/pull/13495)

- [sdk/go] Bump the minimum required versions of google.golang.org/genproto and google.golang.org/grpc.
  [#13593](https://github.com/pulumi/pulumi/pull/13593)

## 3.76.1 (2023-07-25)


### Bug Fixes

- [engine] Fix --target-dependents from targeting all resources with default providers.
  [#13560](https://github.com/pulumi/pulumi/pull/13560)

- [engine] Fix a panic when trying to construct a remote component with an explicit provider configured with unknown values during preview.
  [#13579](https://github.com/pulumi/pulumi/pull/13579)

- [programgen/go] Fix conflicting imports generated when two imported packages have the same name.
  [#13289](https://github.com/pulumi/pulumi/pull/13289)

- [programgen/nodejs] Fixes issue with javascript program generation where enums would incorrectly reference the package rather than the import alias.
  [#13546](https://github.com/pulumi/pulumi/pull/13546)

## 3.76.0 (2023-07-20)


### Features

- [cli/state] Adds `pulumi state edit` an experimental developer utility for manually editing state files.
  [#13462](https://github.com/pulumi/pulumi/pull/13462)

- [programgen] Allow binding unsupported range and collection types in non-strict mode for pulumi convert
  [#13459](https://github.com/pulumi/pulumi/pull/13459)

- [programgen/nodejs] Improve static typing of config variables in main program
  [#13496](https://github.com/pulumi/pulumi/pull/13496)

- [sdk/{go,nodejs,python}] Add support for reporting resource source positions
  [#13449](https://github.com/pulumi/pulumi/pull/13449)

- [sdk/{nodejs,python}] Support explicit providers for packaged components
  [#13282](https://github.com/pulumi/pulumi/pull/13282)


### Bug Fixes

- [cli/config] Pulumi no longer falls back on old config when config resolution fails (except for `pulumi destroy --stack <stack-name>` where the config may be unavailable).
  [#13511](https://github.com/pulumi/pulumi/pull/13511)

- [cli/new] Fix the use of uninitalized backend when running `new` with --generate-only. When --generate-only is set `new` will skip all checks that require the backend.
  [#13530](https://github.com/pulumi/pulumi/pull/13530)

- [engine] Fix alias resoloution when parent alieses where also aliased.
  [#13480](https://github.com/pulumi/pulumi/pull/13480)

- [engine] Validate URNs passed via ResourceOptions are valid.
  [#13531](https://github.com/pulumi/pulumi/pull/13531)

- [engine] Add a missing lock that could cause a concurrent map read/write panic.
  [#13532](https://github.com/pulumi/pulumi/pull/13532)

- [programgen/go] Fix panic in GenerateProject when version is not set in schema
  [#13488](https://github.com/pulumi/pulumi/pull/13488)

- [sdkgen/{go,nodejs}] Fix ReplaceOnChanges being dropped in Go and NodeJS codegen.
  [#13519](https://github.com/pulumi/pulumi/pull/13519)

- [programgen/nodejs] Fix interpolated strings used as keys of maps
  [#13514](https://github.com/pulumi/pulumi/pull/13514)

- [cli/plugin] Automatically install pulumiverse provider plugins during convert.
  [#13486](https://github.com/pulumi/pulumi/pull/13486)

- [cli/plugin] Fix lookup of side-by-side binaries when PULUMI_IGNORE_AMBIENT_PLUGINS is set.
  [#13521](https://github.com/pulumi/pulumi/pull/13521)

- [sdk/python] Move some global state to context state for parallel updates.
  [#13458](https://github.com/pulumi/pulumi/pull/13458)


### Miscellaneous

- [programgen] Consistently use the same non-strict bind options when applicable
  [#13479](https://github.com/pulumi/pulumi/pull/13479)

- [programgen] Propagate SkipRangeTypechecking option down to program components
  [#13493](https://github.com/pulumi/pulumi/pull/13493)

## 3.75.0 (2023-07-12)


### Features

- [programgen/{dotnet,go,nodejs,python}] Allow generating code for unknown invokes (tf data sources) in non-strict mode
  [#13448](https://github.com/pulumi/pulumi/pull/13448)

- [programgen/go] Adds explicit package versioning to Golang codegen
  [#13136](https://github.com/pulumi/pulumi/pull/13136)


### Bug Fixes

- [sdk/go] Fix downloading of unimported external plugins.
  [#13455](https://github.com/pulumi/pulumi/pull/13455)

- [cli/new] `pulumi new -s 'org/project/stack'` checks the specified organization for project existence rather than the currentUser.
  [#13234](https://github.com/pulumi/pulumi/pull/13234)

- [cli/new] When providing a `--stack` and `--name` to `pulumi new`, the project names must match before creating Pulumi.yaml.
  [#13250](https://github.com/pulumi/pulumi/pull/13250)

- [cli/plugin] Fix interpolation of vesion into http plugin source URLs.
  [#13447](https://github.com/pulumi/pulumi/pull/13447)

- [sdk/nodejs] Add dependency on @opentelemetry/instrumentation
  [#13278](https://github.com/pulumi/pulumi/pull/13278)

- [sdk/nodejs] Node.js dynamic providers mark serialized functions as secret if they capture any secrets
  [#13329](https://github.com/pulumi/pulumi/pull/13329)

- [sdk/python] Python dynamic provider serialized code is now saved to state as secret.
  [#13315](https://github.com/pulumi/pulumi/pull/13315)

## 3.74.0 (2023-06-30)


### Features

- [cli] Improve the CLI stack validation error message
  [#13285](https://github.com/pulumi/pulumi/pull/13285)

- [engine] Old inputs are sent to provider Diff and Update functions, as well as the old outputs.
  [#13139](https://github.com/pulumi/pulumi/pull/13139)

- [sdk/nodejs] Support loading package.json from parent directory. If `package.json` is not found in the Pulumi main directory, Pulumi recursively searches up the directory tree until it is found. If `package.json` provides a `main` field, per the [NPM spec](https://docs.npmjs.com/cli/v6/configuring-npm/package-json#main), that field is relative to the directory containing package.json.
  [#13273](https://github.com/pulumi/pulumi/pull/13273)

- [programgen/{nodejs,python}] Prefer output-versioned invokes in generated programs for nodejs and python
  [#13251](https://github.com/pulumi/pulumi/pull/13251)

- [cli/state] The upgrade command now prompts the user to supply project names for stacks for which the project name could not be automatically guessed.
  [#13078](https://github.com/pulumi/pulumi/pull/13078)

- [cli/state] Add interactive URN selection to `pulumi state {rename,unprotect,delete}`.
  [#13302](https://github.com/pulumi/pulumi/pull/13302)


### Bug Fixes

- [auto/nodejs] Adds a better error message for invalid NodeJS AutoAPI workDir.
  [#13275](https://github.com/pulumi/pulumi/pull/13275)

- [cli] Stack output on the console no longer escapes HTML characters inside JSON strings. This matches the behavior of the `--json` flag.
  [#13257](https://github.com/pulumi/pulumi/pull/13257)

- [engine] Engine marks outputs secret if an output of the same name is marked secret.
  [#13260](https://github.com/pulumi/pulumi/pull/13260)

- [sdkgen] Fix loading schemas from providers on PATH.
  [#13305](https://github.com/pulumi/pulumi/pull/13305)

- [cli/display] Print the summary event for previews that contain non-error level diagnostic messages.
  [#13264](https://github.com/pulumi/pulumi/pull/13264)

- [cli/display] Fix diffs sometimes not showing even in details view.
  [#13311](https://github.com/pulumi/pulumi/pull/13311)

- [cli/package] Fixes resolving plugins when they are not yet installed in plugin cache
  [#13283](https://github.com/pulumi/pulumi/pull/13283)

- [cli/state] Disallow renaming resources to invalid names that will corrupt the state.
  [#13254](https://github.com/pulumi/pulumi/pull/13254)

- [programgen/go] Fix aliasing package names using dashes when schema doesn't include go package info override
  [#13212](https://github.com/pulumi/pulumi/pull/13212)

- [programgen/go] Use raw string literals for long, multi-line strings.
  [#13249](https://github.com/pulumi/pulumi/pull/13249)

- [sdk/{go,nodejs,python}] Missing config error text includes "--secret" if requireSecret was used.
  [#13241](https://github.com/pulumi/pulumi/pull/13241)

- [sdkgen/nodejs] Fix isInstance methods for generated provider types.
  [#13265](https://github.com/pulumi/pulumi/pull/13265)


### Miscellaneous

- [pkg/testing] ProgramTest dropped the CoverProfile option as it's no longer necessary.
  [#13298](https://github.com/pulumi/pulumi/pull/13298)

- [sdk/nodejs] Update @grpc/grpc-js to 1.8.16.
  [#13237](https://github.com/pulumi/pulumi/pull/13237)

## 3.73.0 (2023-06-22)


### Features

- [programgen] Allow traversing unknown properties from resources when skipping resource type checking
  [#13180](https://github.com/pulumi/pulumi/pull/13180)


### Bug Fixes

- [backend/filestate] Fix auto-opt-in to project mode.
  [#13243](https://github.com/pulumi/pulumi/pull/13243)

- [cli] `pulumi convert` will now cleanup temporary pulumi-convert directories when the command is finished.
  [#13185](https://github.com/pulumi/pulumi/pull/13185)

- [cli] Fix Markdown formatting issues in command usage.
  [#13225](https://github.com/pulumi/pulumi/pull/13225)

- [cli] Fix `stack rm` removing config files for the wrong project.
  [#13227](https://github.com/pulumi/pulumi/pull/13227)

- [cli/config] No longer error on directory read permissions when searching for project files.
  [#13211](https://github.com/pulumi/pulumi/pull/13211)

- [cli/display] Fix diff display partially parsing JSON/YAML from strings.

- [cli/display] Fix large integers displaying in scientific notation.
  [#13209](https://github.com/pulumi/pulumi/pull/13209)

- [cli/display] Update summary is now correctly shown when `advisory` and `disabled` policy events are encountered.
  [#13218](https://github.com/pulumi/pulumi/pull/13218)

- [cli/display] Fix formatting bugs in display causing text like (MISSING) showing in output.
  [#13228](https://github.com/pulumi/pulumi/pull/13228)

- [cli/display] On Windows, make `pulumi state unprotect` command suggestion use double-quotes instead of single-quotes.
  [#13236](https://github.com/pulumi/pulumi/pull/13236)

- [cli/new] `pulumi new` now correctly supports numeric stack names.
  [#13220](https://github.com/pulumi/pulumi/pull/13220)

- [cli/new] Fix empty config values being added to the config file as part of `new`.
  [#13233](https://github.com/pulumi/pulumi/pull/13233)

- [cli/plugin] Fixes the output of plugin rm --yes command to explicitly say that plugins were removed
  [#13216](https://github.com/pulumi/pulumi/pull/13216)

- [engine] Fix wildcards in IgnoreChanges.
  [#13005](https://github.com/pulumi/pulumi/pull/13005)

- [engine] Fix ignoreChanges setting ignore array indexes to zero.
  [#13005](https://github.com/pulumi/pulumi/pull/13005)

- [sdk/nodejs] Write port to stdout as a string so Node doesn't colorize the output
  [#13204](https://github.com/pulumi/pulumi/pull/13204)

- [sdk/python] Allow tuples as Sequence input values to resources.
  [#13210](https://github.com/pulumi/pulumi/pull/13210)

- [sdkgen/python] Python SDK only prints a Function Invoke result's deprecation messages when using getters rather than on instantiation.
  [#13213](https://github.com/pulumi/pulumi/pull/13213)


### Miscellaneous

- [cli] Make no retry attempts for the Pulumi new version query. This should speed up the CLI in certain environments.
  [#13215](https://github.com/pulumi/pulumi/pull/13215)

## 3.72.2 (2023-06-17)


### Bug Fixes

- [cli/state] Fix panic caused by an invalid stack when a parent resource is renamed in the state. Now, parent references are also updated when the resource is renamed.
  [#13190](https://github.com/pulumi/pulumi/pull/13190)

## 3.72.1 (2023-06-16)


### Bug Fixes

- [cli] Revert go.cloud update to fixes issues with using azure object store and secrets.
  [#13184](https://github.com/pulumi/pulumi/pull/13184)

## 3.72.0 (2023-06-15)


### Features

- [cli] Don't warn about the CLI version being out of date on every run. The CLI will now only warn once a day, when it queries for the latest version.
  [#12660](https://github.com/pulumi/pulumi/pull/12660)

- [programgen/{dotnet,go,nodejs,python}] Extend SkipResourceTypechecking to allow generating unknown resources
  [#13172](https://github.com/pulumi/pulumi/pull/13172)

- [cli/package] Add a "get-mapping" command to query providers for their mapping information.
  [#13155](https://github.com/pulumi/pulumi/pull/13155)


### Bug Fixes

- [cli/config] `pulumi destroy` now sets the `encryptedkey` every run like the rest of the CLI commands.
  [#13168](https://github.com/pulumi/pulumi/pull/13168)

- [engine] Fix aliasing children
  [#12848](https://github.com/pulumi/pulumi/pull/12848)

- [sdk/nodejs] Fix Parent/NoParent aliases
  [#12848](https://github.com/pulumi/pulumi/pull/12848)

- [sdk/nodejs] Fixes uncaught rejections on the resource monitor terminating causing Automation API programs to exit prematurely.
  [#13070](https://github.com/pulumi/pulumi/pull/13070)


### Miscellaneous

- [backend/filestate] Add an option to the Upgrade operation allowing injection of an external source of project names for stacks where the project name could not be automatically determined.
  [#13077](https://github.com/pulumi/pulumi/pull/13077)

- [sdk/go] Adds `tokens.ValidateProjectName` to validate project names.
  [#13165](https://github.com/pulumi/pulumi/pull/13165)

## 3.71.0 (2023-06-12)


### Features

- [cli] Support for `pulumi convert --from terraform`

- [cli] Make convert errors more clear to users
  [#13126](https://github.com/pulumi/pulumi/pull/13126)

- [programgen/{dotnet,go}] Add support for the singleOrNone intrinsic
  [#13149](https://github.com/pulumi/pulumi/pull/13149)


### Bug Fixes

- [engine] Fix plugin installation when looking up new schemas.
  [#13140](https://github.com/pulumi/pulumi/pull/13140)

- [programgen] Fixes range scoping for PCL components
  [#13131](https://github.com/pulumi/pulumi/pull/13131)

- [programgen] Fixes panic when trying to convert a null literal to a string value
  [#13138](https://github.com/pulumi/pulumi/pull/13138)

- [sdkgen/dotnet] sdkgen no longer sets the UseSharedCompilation project setting.
  [#13146](https://github.com/pulumi/pulumi/pull/13146)

- [programgen/python] Fixes python panic when emiting code for index expressions that aren't typechecked
  [#13137](https://github.com/pulumi/pulumi/pull/13137)

- [sdkgen/python] Fixes python always printing input deprecation messages.
  [#13141](https://github.com/pulumi/pulumi/pull/13141)

## 3.70.0 (2023-06-08)


### Features

- [cli] 'convert' now defaults to be more leniant about program correctness, old behaviour can be toggled back on with --strict.
  [#13120](https://github.com/pulumi/pulumi/pull/13120)

- [engine] DeletedWith ResourceOption is now inherited from its parent across SDKs.
  [#12572](https://github.com/pulumi/pulumi/pull/12572)

- [engine] Add 'pulumi:tags' config option to set stack tags.
  [#12856](https://github.com/pulumi/pulumi/pull/12856)

- [pkg] Upgrade pulumi-java to v0.9.4.
  [#13121](https://github.com/pulumi/pulumi/pull/13121)

- [programgen/nodejs] Allow output variables to have the same identifier as other program nodes
  [#13115](https://github.com/pulumi/pulumi/pull/13115)

- [sdk/nodejs] Add support for asynchronous mock implementations


### Bug Fixes

- [cli/new] Escape special characters in project description
  [#13122](https://github.com/pulumi/pulumi/pull/13122)

- [engine] Fixes a bug where targeted previews would error on deletes of targeted resources.
  [#13010](https://github.com/pulumi/pulumi/pull/13010)

- [programgen/dotnet] Only await task-returning invokes in dotnet program-gen
  [#13092](https://github.com/pulumi/pulumi/pull/13092)

- [programgen/{dotnet,go}] Do not error out when generaing not yet implemented ForExpressions
  [#13083](https://github.com/pulumi/pulumi/pull/13083)

- [cli/plugin] Language plugins respect PULUMI_IGNORE_AMBIENT_PLUGINS.
  [#13086](https://github.com/pulumi/pulumi/pull/13086)

- [programgen/go] Fix conversion of programs with components for Go.
  [#13037](https://github.com/pulumi/pulumi/pull/13037)

- [programgen/go] Fix panic in go program-gen when encountering splat expressions
  [#13116](https://github.com/pulumi/pulumi/pull/13116)

- [programgen/{go,nodejs}] Fix a panic in diagnostics from go/nodejs project generation.
  [#13084](https://github.com/pulumi/pulumi/pull/13084)

- [programgen/nodejs] Only await promise-returning invokes in typescript program-gen
  [#13085](https://github.com/pulumi/pulumi/pull/13085)

## 3.69.0 (2023-06-01)


### Features

- [auto/python] Add support for the path option for config operations
  [#13052](https://github.com/pulumi/pulumi/pull/13052)

- [cli] Replace heap profiles with allocation profiles and add a flag, --memprofilerate, to control the sampling rate. --memprofilerate behaves like the -memprofilerate flag to `go test`; set it to "1" to profile every allocation site.
  [#13026](https://github.com/pulumi/pulumi/pull/13026)

- [cli] The convert and import commands will try to install plugins needed for correct conversions.
  [#13046](https://github.com/pulumi/pulumi/pull/13046)

- [cli/plugin] Plugin install auto-fills the download URL for some known third-party plugins.
  [#13020](https://github.com/pulumi/pulumi/pull/13020)

- [engine] Provider plugins are now loaded as needed, not at startup based on old state information.
  [#12657](https://github.com/pulumi/pulumi/pull/12657)

- [programgen] Include the component source directory in diagnostics when reporting PCL errors
  [#13017](https://github.com/pulumi/pulumi/pull/13017)

- [programgen/{nodejs,python}] Implement singleOrNone intrinsic
  [#13032](https://github.com/pulumi/pulumi/pull/13032)

- [sdkgen/python] Generate a pyproject.toml file. This enables Python providers to build Wheels per PEP 621
  [#12805](https://github.com/pulumi/pulumi/pull/12805)


### Bug Fixes

- [backend] Fixes a bug where Resource instances as stack exports got printed as if it had diff in the end steps
  [#12261](https://github.com/pulumi/pulumi/pull/12261)

- [engine] Fix --replace behavior to be not considered a targeted update (where only --replace resources would be targeted).
  [#13011](https://github.com/pulumi/pulumi/pull/13011)

- [backend/filestate] Fix the project filter when listing stacks from new stores that support per-project stack references.
  [#12994](https://github.com/pulumi/pulumi/pull/12994)

- [backend/filestate] Fix stack rename renaming projects for the self-managed backend.
  [#13047](https://github.com/pulumi/pulumi/pull/13047)

- [programgen/go] Do not error when generated Go code cannot be formatted
  [#13053](https://github.com/pulumi/pulumi/pull/13053)

- [cli/plugin] Fixes PULUMI_DEBUG_GRPC to surface provider errors
  [#12984](https://github.com/pulumi/pulumi/pull/12984)

- [sdkgen/go] For properties with environment variable defaults, differentiate between unset environment variables and empty.
  [#12976](https://github.com/pulumi/pulumi/pull/12976)

- [sdkgen/go] When a property has an environment variable default, and the environment variable is not set, sdkgen would incorrectly set it to the zero value of that property. Fixes by only setting the property if the environment variable is set.
  [#12976](https://github.com/pulumi/pulumi/pull/12976)

- [sdkgen/go] Fix versioned typerefs being marshalled across code generator RPCs.
  [#13006](https://github.com/pulumi/pulumi/pull/13006)

## 3.68.0 (2023-05-18)


### Features

- [backend/service] Improve memory consumption and decrease CPU time required when using snapshot patching
  [#12962](https://github.com/pulumi/pulumi/pull/12962)


### Bug Fixes

- [engine] Step generation now uses old inputs for untargeted resources and does not send current inputs to `Check()` on providers.
  [#12973](https://github.com/pulumi/pulumi/pull/12973)

- [sdk/go] Fix regression disallowing placing a Pulumi program in a subdirectory of a Go module.
  [#12967](https://github.com/pulumi/pulumi/pull/12967)

- [programgen/nodejs] Allow iterating dynamic entries in TypeScript
  [#12961](https://github.com/pulumi/pulumi/pull/12961)

## 3.67.1 (2023-05-15)


### Features

- [programgen/go] Module support as component resources
  [#12840](https://github.com/pulumi/pulumi/pull/12840)


### Bug Fixes

- [engine] Non-targeted resources are now added to internal update plans fixing a bug where the step_executor would error due to missing resources in the plan.
  [#12939](https://github.com/pulumi/pulumi/pull/12939)

- [programgen] Fix stack overflow panic when pretty printing recursive types
  [#12866](https://github.com/pulumi/pulumi/pull/12866)

- [sdk/nodejs] Revert recursive package.json lookup.
  [#12944](https://github.com/pulumi/pulumi/pull/12944)


### Miscellaneous

- [sdk/go] testing.Environment now tolerates errors in deleting the test environment.
  [#12927](https://github.com/pulumi/pulumi/pull/12927)

- [sdk/nodejs] Replaces empty interfaces with type aliases. Empty interfaces are equivalent to their supertype; this change expresses these type definitions using type aliases instead of interface extention to provide better clarity. This change will not affect type-checking.
  [#12865](https://github.com/pulumi/pulumi/pull/12865)

## 3.67.0 (2023-05-11)


### Features

- [sdk/nodejs] Support loading package.json from parent directory. If `package.json` is not found in the Pulumi main directory, Pulumi recursively searches up the directory tree until it is found. If `package.json` provides a `main` field, per the [NPM spec](https://docs.npmjs.com/cli/v6/configuring-npm/package-json#main), that field is relative to the directory containing package.json.
  [#12759](https://github.com/pulumi/pulumi/pull/12759)


### Bug Fixes

- [build] Fixes race condition in building Go sdk.
  [#12821](https://github.com/pulumi/pulumi/pull/12821)

- [cli] Convert to PCL will recover from panics in program binding.
  [#12827](https://github.com/pulumi/pulumi/pull/12827)

- [engine] Fix bug with targeting and plans where root stack resource and target-replaces were not being marked targeted.
  [#12834](https://github.com/pulumi/pulumi/pull/12834)

- [engine] Fix the engine trying to install the pulumi-resource-pulumi plugin which is builtin.
  [#12858](https://github.com/pulumi/pulumi/pull/12858)

- [programgen] Allow null literal as a default value for config variables
  [#12817](https://github.com/pulumi/pulumi/pull/12817)

- [programgen] Fix panic on component type traversal
  [#12828](https://github.com/pulumi/pulumi/pull/12828)

- [sdk/python] Fix hang due to component children cycles
  [#12855](https://github.com/pulumi/pulumi/pull/12855)


### Miscellaneous

- [sdk/nodejs] With Node14 sunset on April 30, the minimum version of Node is now Node 16.
  [#12648](https://github.com/pulumi/pulumi/pull/12648)

## 3.66.0 (2023-05-03)


### Features

- [cli] `convert` now prints all diagnostics from program conversion
  [#12808](https://github.com/pulumi/pulumi/pull/12808)

- [programgen/nodejs] Support range expressions that are of type output
  [#12749](https://github.com/pulumi/pulumi/pull/12749)

- [programgen/python] Support range expressions that are of type output
  [#12804](https://github.com/pulumi/pulumi/pull/12804)


### Bug Fixes

- [cli] Fix destroy without project file.
  [#12766](https://github.com/pulumi/pulumi/pull/12766)

- [engine] Fix bug where non-default providers are created even when not specified as a target.
  [#12628](https://github.com/pulumi/pulumi/pull/12628)


### Miscellaneous

- [backend/filestate] Improve performance of project-existence check.
  [#12798](https://github.com/pulumi/pulumi/pull/12798)

## 3.65.1 (2023-04-27)


### Bug Fixes

- [backend/filestate] Revert change causing `provided project name "" doesn't match Pulumi.yaml` error
  [#12761](https://github.com/pulumi/pulumi/pull/12761)

## 3.65.0 (2023-04-26)


### Features

- [auto/nodejs] Add `excludeProtected` option for `destroy`
  [#12734](https://github.com/pulumi/pulumi/pull/12734)

- [auto/nodejs] Add `refresh` option for `preview`
  [#12743](https://github.com/pulumi/pulumi/pull/12743)

- [cli] Speed up conversion mapping lookups for the common case of Pulumi names matching external ecosystem names.
  [#12711](https://github.com/pulumi/pulumi/pull/12711)

- [engine] Support propagating more resource options to packaged components.
  [#12682](https://github.com/pulumi/pulumi/pull/12682)

- [cli/display] Pulumi CLI can now display messages provided by the service.
  [#12671](https://github.com/pulumi/pulumi/pull/12671)

- [sdk/go] Support new options on packaged components (MLCs), including: AdditionalSecretOutputs, Timeouts, DeletedWith, DeleteBeforeReplace, IgnoreChanges, ReplaceOnChanges, and RetainOnDelete.
  [#12701](https://github.com/pulumi/pulumi/pull/12701)

- [sdk/go] Support vendored dependencies for Pulumi programs.
  [#12727](https://github.com/pulumi/pulumi/pull/12727)


### Bug Fixes

- [cli] Fix destroy without project file.
  [#12728](https://github.com/pulumi/pulumi/pull/12728)

- [programgen] Allow using option(T) in range expressions
  [#12717](https://github.com/pulumi/pulumi/pull/12717)

- [sdk/go] Ensure that dependency searches happen in the Pulumi program directory.
  [#12732](https://github.com/pulumi/pulumi/pull/12732)

- [pkg/testing] Fix failure in writing a package.json for test overrides.
  [#12700](https://github.com/pulumi/pulumi/pull/12700)


### Miscellaneous

- [pkg/testing] ProgramTest now supports --exclude-protected during stack cleanup.
  [#12699](https://github.com/pulumi/pulumi/pull/12699)

## 3.64.0 (2023-04-18)


### Features

- [cli/display] Adds an indicator for resources that are being deleted/replaced with `retainOnDelete` set as well as an itemized warning.
  [#12157](https://github.com/pulumi/pulumi/pull/12157)

- [backend/{filestate,service}] Add more information to `pulumi stack history` (Update CLI Args, Environment Variables, Pulumi Version, OS, Architecture).
  [#12574](https://github.com/pulumi/pulumi/pull/12574)


### Bug Fixes

- [pkg/testing] deploytest: Fix nil custom timeouts and timeouts smaller than a minute being ignored.
  [#12681](https://github.com/pulumi/pulumi/pull/12681)

- [programgen] Do not panic when PCL attribute type or PCL resource variable type isn't fully bound
  [#12661](https://github.com/pulumi/pulumi/pull/12661)

- [sdk/go] Fixed NewResourceOptions dropping MLC dependencies from the options preview.
  [#12683](https://github.com/pulumi/pulumi/pull/12683)

- [programgen/nodejs] Linearize component resource nodes
  [#12676](https://github.com/pulumi/pulumi/pull/12676)

- [sdk/python] Fix component resources not correctly propagating the `provider` option to their children.
This is a re-application of #12292, which was previously reverted in #12522.

  [#12639](https://github.com/pulumi/pulumi/pull/12639)

- [sdk/python] Fix multi-language components dropping the `provider` option intended for their descendants.
  [#12639](https://github.com/pulumi/pulumi/pull/12639)

- [sdkgen/python] Fix referencing local types with a different package name
  [#12669](https://github.com/pulumi/pulumi/pull/12669)


### Miscellaneous

- [pkg] Bump pulumi-terraform-bridge
  [#12625](https://github.com/pulumi/pulumi/pull/12625)

- [programgen] Do not panic when the type of PCL local variable isn't known
  [#12670](https://github.com/pulumi/pulumi/pull/12670)

## 3.63.0 (2023-04-12)


### Bug Fixes

- [cli/config] Fix config set-all not saving secret provider configuration.
  [#12643](https://github.com/pulumi/pulumi/pull/12643)

- [cli/display] Fix a panic when diffing empty archives.
  [#12656](https://github.com/pulumi/pulumi/pull/12656)

- [programgen] Suppport the `any` type in config and outputs.
  [#12619](https://github.com/pulumi/pulumi/pull/12619)

- [sdk/go] Fix hang due to component children cycles
  [#12516](https://github.com/pulumi/pulumi/pull/12516)

- [sdk/nodejs] Fix hang due to component children cycles
  [#12515](https://github.com/pulumi/pulumi/pull/12515)

- [sdk/python] Fix hang due to component children cycles
  [#12462](https://github.com/pulumi/pulumi/pull/12462)


### Miscellaneous

- [backend/filestate] Propagate request context to external call sites.
  [#12638](https://github.com/pulumi/pulumi/pull/12638)

## 3.62.0 (2023-04-06)


### Features

- [yaml] Updates Pulumi YAML to v1.1.0.
  [#12612](https://github.com/pulumi/pulumi/pull/12612)

## 3.61.1 (2023-04-06)


### Features

- [programgen/python] Impleneted python program-gen for PCL components
  [#12555](https://github.com/pulumi/pulumi/pull/12555)


### Bug Fixes

- [programgen/{nodejs,python}] Fixes the type signature of PCL function "entries" to return list of key-value pair objects
  [#12607](https://github.com/pulumi/pulumi/pull/12607)

- [cli/package] Fix bug in `package get-schema` subcommand caused it to bail on certain providers.
  [#12459](https://github.com/pulumi/pulumi/pull/12459)

- [cli/state] Fixes panic when renaming providers in `pulumi state rename`.
  [#12599](https://github.com/pulumi/pulumi/pull/12599)

## 3.61.0 (2023-04-03)


### Features

- [backend/filestate] Add support for project-scoped stacks.
  Newly initialized backends will automatically use this mode.
  Set PULUMI_SELF_MANAGED_STATE_LEGACY_LAYOUT=1 to opt-out of this.
  This mode needs write access to the root of the .pulumi directory;
  if you're using a cloud storage, be sure to update your ACLs.

  [#12437](https://github.com/pulumi/pulumi/pull/12437)

- [cli/state] Add 'upgrade' subcommand to upgrade a Pulumi self-managed state to use project layout.
  [#12438](https://github.com/pulumi/pulumi/pull/12438)


### Bug Fixes

- [cli/display] Fix a bug in the interactive update tree display where small terminals would cause the Pulumi CLI to panic.
  [#12571](https://github.com/pulumi/pulumi/pull/12571)

- [sdkgen/dotnet] Fix a whitespace error in generated .csproj files.
  [#12577](https://github.com/pulumi/pulumi/pull/12577)


### Miscellaneous

- [backend/filestate] Print a warning if a project-scoped backend has non-project stacks in it.
Disable this warning by setting PULUMI_SELF_MANAGED_STATE_NO_LEGACY_WARNING=1.

  [#12437](https://github.com/pulumi/pulumi/pull/12437)

## 3.60.1 (2023-03-30)


### Features

- [sdkgen/python] In codegen, use 3.7 as a default if not provided.
  [#12287](https://github.com/pulumi/pulumi/pull/12287)


### Bug Fixes

- [backend/filestate] Don't write a state metadata file for legacy layouts.
This should prevent permissioning issues for users
with tight access control to the storage backend.

  [#12537](https://github.com/pulumi/pulumi/pull/12537)

- [docs] Fix filename clashes between resources and functions on case-insensitive filesystems in docsgen.
  [#12453](https://github.com/pulumi/pulumi/pull/12453)

- [engine] Fix updating a resource from a component to custom resource.
  [#12561](https://github.com/pulumi/pulumi/pull/12561)

- [engine] Revert PR moving deletedWith inheritance logic to the engine as `get` resources and packaged components are incompatible.
  [#12564](https://github.com/pulumi/pulumi/pull/12564)

- [sdk] Fix multiplied retries when downloading plugins.
  [#12504](https://github.com/pulumi/pulumi/pull/12504)

- [auto/go] Added support for the path option for config operations
  [#12265](https://github.com/pulumi/pulumi/pull/12265)


### Miscellaneous

- [backend/filestate] Rename state metadata file from .pulumi/Pulumi.yaml to .pulumi/meta.yaml.
This is an internal detail to the self-managed backend's storage format
intended to avoid confusion with Pulumi project files,
and should not affect most users.

  [#12538](https://github.com/pulumi/pulumi/pull/12538)

## 3.60.0 (2023-03-27)


### Features

- [engine] Enhances the state schema to track fields `Created`, `Modified` per each resource. The timestamp is captured in RFC3339. It pertains to timestamps of state modification done by the engine.
  [#12082](https://github.com/pulumi/pulumi/pull/12082)

- [engine] DeletedWith ResourceOption is now inherited from its parent across SDKs.
  [#12446](https://github.com/pulumi/pulumi/pull/12446)

- [programgen/{dotnet,nodejs}] Object-typed config variables for components
  [#12488](https://github.com/pulumi/pulumi/pull/12488)


### Bug Fixes

- [sdk] common: Fix extraneous backoff during retries.
  [#12502](https://github.com/pulumi/pulumi/pull/12502)

- [sdkgen/dotnet] respectSchemaVersion now writes the package version to the .csproj file.
  [#12518](https://github.com/pulumi/pulumi/pull/12518)

- [sdk/python] Revert #12292 to unbreak some users.
  [#12522](https://github.com/pulumi/pulumi/pull/12522)

- [sdkgen/{dotnet,go,nodejs,python}] Add respectSchemaVersion to schema.
  [#12511](https://github.com/pulumi/pulumi/pull/12511)

## 3.59.1 (2023-03-24)


### Bug Fixes

- [sdk] Make default logger thread-safe.
  [#12485](https://github.com/pulumi/pulumi/pull/12485)

- [sdk/go] Track rehydrated components as dependencies.
  [#12494](https://github.com/pulumi/pulumi/pull/12494)

- [sdkgen/go] Fixes emission of dup types breaking Go compilation when chunking >500 helper types.
  [#12484](https://github.com/pulumi/pulumi/pull/12484)


### Miscellaneous

- [cli] Improve CLI upgrade instructions for macOS.
  [#12483](https://github.com/pulumi/pulumi/pull/12483)

## 3.59.0 (2023-03-22)


### Features

- [programgen] PCL program.WriteSource(afero.Fs) writes the full directory tree of PCL source files.
  [#12428](https://github.com/pulumi/pulumi/pull/12428)

- [programgen/{dotnet,go,nodejs,python}] Implement description as comments or docstring for config variables in program-gen
  [#12464](https://github.com/pulumi/pulumi/pull/12464)

- [programgen/{dotnet,nodejs}] Component resources implementation including nested components
  [#12398](https://github.com/pulumi/pulumi/pull/12398)

- [backend/service] Add "--teams" flag to assign team name to stack during init
  [#11974](https://github.com/pulumi/pulumi/pull/11974)


### Bug Fixes

- [auto/go] Fix memory leak in stack.Up() in Automation API.
  [#12475](https://github.com/pulumi/pulumi/pull/12475)

- [auto/{go,nodejs,python}] Fix calling WhoAmI against pre 3.58 CLIs.
  [#12466](https://github.com/pulumi/pulumi/pull/12466)

- [engine] Fixed automatic plugin downloads for third-party plugins.
  [#12441](https://github.com/pulumi/pulumi/pull/12441)

- [programgen/python] Fix handling of reserved words in imports.
  [#12447](https://github.com/pulumi/pulumi/pull/12447)


### Miscellaneous

- [ci] Bumps python version in matrix to 3.11
  [#11238](https://github.com/pulumi/pulumi/pull/11238)

## 3.58.0 (2023-03-15)


### Features

- [auto/go] Add WhoAmIDetails which includes user, url and organizations to Go Automation API
  [#12374](https://github.com/pulumi/pulumi/pull/12374)

- [auto/nodejs] Add url and organizations to WhoAmIResult for NodeJS Automation API
  [#12374](https://github.com/pulumi/pulumi/pull/12374)

- [auto/python] Add url and organizations to WhoAmIResult for Python Automation API
  [#12374](https://github.com/pulumi/pulumi/pull/12374)

- [cli] Add `--json` flag to `pulumi whoami` to emit output as JSON
  [#12374](https://github.com/pulumi/pulumi/pull/12374)

- [cli/display] Add a view in browser shortcut to the interactive display.
  [#12412](https://github.com/pulumi/pulumi/pull/12412)
  [#12380](https://github.com/pulumi/pulumi/pull/12380)

- [programgen/dotnet] PCL components and dotnet program-gen implementation
  [#12361](https://github.com/pulumi/pulumi/pull/12361)

- [programgen/{dotnet,go,nodejs,python}] Add "NotImplemented" PCL function intrinsic
  [#12409](https://github.com/pulumi/pulumi/pull/12409)

- [sdk/go] Adds `NewInvokeOptions` to preview the effect of a list of `InvokeOption` values.
  [#12128](https://github.com/pulumi/pulumi/pull/12128)


### Bug Fixes

- [cli/display] Do not treat single-line strings as YAML values
  [#12406](https://github.com/pulumi/pulumi/pull/12406)

- [sdk/go] Fixes an ID handling bug in provider_server Read implementation
  [#12410](https://github.com/pulumi/pulumi/pull/12410)

- [sdk/go] Fixes use of Provider option from parent resources with mismatched packages.
  [#12433](https://github.com/pulumi/pulumi/pull/12433)

## 3.57.1 (2023-03-09)


### Bug Fixes

- [cli/plugin] Fix sending empty tokens to GitHub API.
  [#12392](https://github.com/pulumi/pulumi/pull/12392)

## 3.57.0 (2023-03-08)


### Features

- [cli/display] Autoscroll the interactive display and support pgup/pgdown
  [#12363](https://github.com/pulumi/pulumi/pull/12363)

- [programgen] Support `options.retainOnDelete` on resources in PCL.
  [#12305](https://github.com/pulumi/pulumi/pull/12305)

- [sdkgen/dotnet] Update sdkgen to target dotnet 6.
  [#12333](https://github.com/pulumi/pulumi/pull/12333)

- [programgen/{dotnet,go,nodejs,python}] Adds support for generating RetainOnDelete options.
  [#12306](https://github.com/pulumi/pulumi/pull/12306)

- [auto/go] Enable programmatic tagging of stacks (Go only)
  [#12329](https://github.com/pulumi/pulumi/pull/12329)

- [auto/python] Enable programmatic tagging of stacks (Python only)
  [#12275](https://github.com/pulumi/pulumi/pull/12275)

- [sdk/go] Adds `NewResourceOptions` to preview the effect of a list of `ResourceOption` values.
  [#12124](https://github.com/pulumi/pulumi/pull/12124)

- [sdk/python] Added support for shimless Python plugins.
  [#12362](https://github.com/pulumi/pulumi/pull/12362)


### Bug Fixes

- [cli/display] Reorder options to handle pending creates. Users can now hold enter to select the clear option which should be more ergonomic.
  [#12375](https://github.com/pulumi/pulumi/pull/12375)

- [auto/{dotnet,go,nodejs,python}] Fix support for specifying a git commit for remote workspaces
  [#11716](https://github.com/pulumi/pulumi/pull/11716)

- [auto/go] Fetch commits before checkout
  [#12331](https://github.com/pulumi/pulumi/pull/12331)

- [auto/go] The various workspace load routines (e.g. LoadProject) are no longer singularly cached.
  [#12370](https://github.com/pulumi/pulumi/pull/12370)

- [sdk/go] Fixes overwrite of the Provider option by the Providers option due to ordering.
  [#12296](https://github.com/pulumi/pulumi/pull/12296)

- [auto/nodejs] Fixes issue with specifying a git username for remote workspaces
  [#12269](https://github.com/pulumi/pulumi/pull/12269)

- [sdk/python] Fixes Component Resources not correctly propagating the provider option to its children.
  [#12292](https://github.com/pulumi/pulumi/pull/12292)


### Miscellaneous

- [sdk/go] common/util/contract: Deprecate functions that don't accept printf-style arguments.
  [#12350](https://github.com/pulumi/pulumi/pull/12350)

## 3.56.0 (2023-03-02)


### Features

- [cli/display] Display now shows default colorized stacktraces in NodeJS.
  [#10410](https://github.com/pulumi/pulumi/pull/10410)

- [cli/plugin] Plugin download urls now support GitLab as a first class url schema. For example "gitlab://gitlab.com/43429536".
  [#12145](https://github.com/pulumi/pulumi/pull/12145)


### Bug Fixes

- [backend/service] Reduce retrieval-validation latency for update tokens
  [#12323](https://github.com/pulumi/pulumi/pull/12323)

- [sdk/go] Fix panic from attempting to create a resource with an uninitialized parent resource.
  [#12303](https://github.com/pulumi/pulumi/pull/12303)

- [cli/import] Fixes panic on incomplete resources in JSON file.
  [#12182](https://github.com/pulumi/pulumi/pull/12182)

- [sdk/nodejs] Cleanup temporary pulumi-node-pipes folders after running.
  [#12294](https://github.com/pulumi/pulumi/pull/12294)

- [sdk/nodejs] Fix stack outputs picking up co-located JSON files.
  [#12302](https://github.com/pulumi/pulumi/pull/12302)

- [cli/plugin] Remove temporary files from plugin downloads.
  [#12146](https://github.com/pulumi/pulumi/pull/12146)


### Miscellaneous

- [sdk/go] common/resource/testing: Returns strongly typed generators instead of `interface{}` generators.
  [#12197](https://github.com/pulumi/pulumi/pull/12197)

- [sdk/python] grpc 1.51.3 Python SDK contains native arm64 binaries (universal2)
  [#12313](https://github.com/pulumi/pulumi/pull/12313)

## 3.55.0 (2023-02-14)


### Features

- [cli] Remove the `[experimental] yes, using Update Plans` prompt.
  [#12135](https://github.com/pulumi/pulumi/pull/12135)

- [backend/filestate] pulumi login gs:// to support google oauth access tokens via environment variable for Google Cloud Storage backends
  [#12102](https://github.com/pulumi/pulumi/pull/12102)

- [sdk/go] Adds StackReference.GetOutputDetails to retrieve outputs from StackReferences as plain objects.
  [#12034](https://github.com/pulumi/pulumi/pull/12034)

- [sdk/nodejs] Adds StackReference.getOutputDetails to retrieve outputs from StackReferences as plain objects.
  [#12072](https://github.com/pulumi/pulumi/pull/12072)

- [sdk/python] Adds StackReference.get_output_details to retrieve outputs from StackReferences as plain objects.
  [#12071](https://github.com/pulumi/pulumi/pull/12071)


### Bug Fixes

- [cli] Fix verbose logging to filter secrets.
  [#12079](https://github.com/pulumi/pulumi/pull/12079)

- [engine] This fixes an issue where 'pulumi state delete ' would prompt the user to disambiguate between multiple resources in state with the same URN and proceed to delete all of them. With this change, dependency checks are performed only if the deletion will lead to no resources possessing the URN. The targetDependents flag will only target dependents if the deleted resource will orphan the dependents.
  [#12111](https://github.com/pulumi/pulumi/pull/12111)

- [engine] Fixed issue where pulumi displays multiline secrets when the newlines('
') are escaped.
  [#12140](https://github.com/pulumi/pulumi/pull/12140)

- [sdkgen/go] Prevent defaults from overriding set values.
  [#12099](https://github.com/pulumi/pulumi/pull/12099)


### Miscellaneous

- [pkg] Raise 'go' directive to 1.18.
  [#11807](https://github.com/pulumi/pulumi/pull/11807)

- [sdk/go] Raise 'go' directive to 1.18.
  [#11807](https://github.com/pulumi/pulumi/pull/11807)

## 3.54.0 (2023-02-06)


### Features

- [cli] Add `--shell` flag to `pulumi stack output` to print outputs as a shell script.
  [#11956](https://github.com/pulumi/pulumi/pull/11956)

- [cli] Add `--insecure` flag to `pulumi login` which disables https certificate checks
  [#9159](https://github.com/pulumi/pulumi/pull/9159)

- [programgen] Add a new `unsecret` intrinsic function to PCL.
  [#12026](https://github.com/pulumi/pulumi/pull/12026)

- [sdkgen/go] Go SDKs now use `errors.New` instead of `github.com/pkg/errors.New`.
  [#12046](https://github.com/pulumi/pulumi/pull/12046)


### Bug Fixes

- [auto] Add support for cloning from Azure DevOps
  [#12001](https://github.com/pulumi/pulumi/pull/12001)

- [sdkgen] Correctly error on resource using the reserved name "provider".
  [#11996](https://github.com/pulumi/pulumi/pull/11996)

- [sdk/python] Fix handling of Output keys in dicts passed to Output.from_input.
  [#11968](https://github.com/pulumi/pulumi/pull/11968)


### Miscellaneous

- [sdk/go] Delegate alias computation to the engine
  [#12025](https://github.com/pulumi/pulumi/pull/12025)

- [sdk/python] Delegate alias computation to the engine
  [#12015](https://github.com/pulumi/pulumi/pull/12015)

## 3.53.1 (2023-01-25)


### Bug Fixes

- [engine] Revert go-cloud upgrade to fix issues with Azure secrets.
  [#11984](https://github.com/pulumi/pulumi/pull/11984)

## v3.53.0 (2023-01-25)


### Features

- [auto/nodejs] Enable programmatic tagging of stacks (Nodejs only)
  [#11659](https://github.com/pulumi/pulumi/pull/11659)

- [sdk/go] Coerces output values in ApplyT calls if the types are equivalent.
  [#11903](https://github.com/pulumi/pulumi/pull/11903)

- [sdk/nodejs] Add optional / backwards compatible generic types to pulumi.dynamic.ResourceProvider.
  [#11881](https://github.com/pulumi/pulumi/pull/11881)


### Bug Fixes

- [auto/nodejs] Fix NodeJS automation api always setting the PULUMI_CONFIG environment variable.
  [#11943](https://github.com/pulumi/pulumi/pull/11943)

- [cli/display] Display text-based diff if yaml/json diff is semantically equal
  [#11803](https://github.com/pulumi/pulumi/pull/11803)

- [sdk/go] Fixes data race in provider plugin resulting in weakly typed secrets.
  [#11975](https://github.com/pulumi/pulumi/pull/11975)

- [sdk/nodejs] Fix handling of recursive symlinks in node_modules.
  [#11950](https://github.com/pulumi/pulumi/pull/11950)

## 3.52.1 (2023-01-19)


### Bug Fixes

- [engine] Fix launching non-Go plugins on Windows.
  [#11915](https://github.com/pulumi/pulumi/pull/11915)

## 3.52.0 (2023-01-18)


### Features

- [sdk/go] Allows users to discover if their program is being run with a mock monitor
  [#11788](https://github.com/pulumi/pulumi/pull/11788)

- [sdk/nodejs] Add support for custom naming of dynamic provider resource.
  [#11873](https://github.com/pulumi/pulumi/pull/11873)

- [sdkgen/{dotnet,nodejs}] Initial implementation of simplified invokes for dotnet and nodejs.
  [#11753](https://github.com/pulumi/pulumi/pull/11753)


### Bug Fixes

- [cli/display] Fixes #11864. Pulumi panics before main when Pulumi.yaml provider plugin does not have a path provided.
  [#11892](https://github.com/pulumi/pulumi/pull/11892)

- [sdk/{go,nodejs,python}] Fix DeletedWith resource option
  [#11883](https://github.com/pulumi/pulumi/pull/11883)

- [sdk/python] Fix a TypeError in Output.from_input.
  [#11852](https://github.com/pulumi/pulumi/pull/11852)

## 3.51.1 (2023-01-11)


### Features

- [sdk/go] Add JSONUnmarshal to go sdk.
  [#11745](https://github.com/pulumi/pulumi/pull/11745)

- [sdk/python] Add output json_loads using json.loads.
  [#11741](https://github.com/pulumi/pulumi/pull/11741)


### Bug Fixes

- [cli/new] Allow running inside new VCS repositories.
  [#11804](https://github.com/pulumi/pulumi/pull/11804)

- [auto/python] Fix issue specifying log_verbosity
  [#11778](https://github.com/pulumi/pulumi/pull/11778)

- [protobuf] Downstream implementers of the RPC server interfaces must embed UnimplementedServer structs or opt out of forward compatibility.
  [#11652](https://github.com/pulumi/pulumi/pull/11652)

## 3.51.0 (2023-01-04)

Happy New Years from the Pulumi team!  This is our first release of 2023, and we're very excited for all the things to come this year.

### Features

- [sdk/nodejs] Add output jsonParse using JSON.parse.
  [#11735](https://github.com/pulumi/pulumi/pull/11735)

## 3.50.1 (2022-12-21)


### Bug Fixes

- [cli/display] Fix flickering in the interactive display
  [#11695](https://github.com/pulumi/pulumi/pull/11695)

- [cli/plugin] Fix check of executable bits on Windows.
  [#11692](https://github.com/pulumi/pulumi/pull/11692)

- [codegen] Revert change to codegen schema spec.
   [#11701](https://github.com/pulumi/pulumi/pull/11701)

## 3.50.0 (2022-12-19)

We're approaching the end of 2022, and this is the final minor release scheduled for the year! 🎸
Thank you very much to our wonderful community for your many contributions! ❤️

### Features

- [auto/{go,nodejs,python}] Adds SkipInstallDependencies option for Remote Workspaces
  [#11674](https://github.com/pulumi/pulumi/pull/11674)

- [ci] GitHub release artifacts are now signed using [cosign](https://github.com/sigstore/cosign) and signatures are uploaded to the [Rekor transparency log](https://rekor.tlog.dev/).
  [#11310](https://github.com/pulumi/pulumi/pull/11310)

- [cli] Adds a flag that allows user to set the node label as the resource name instead of full URN in the stack graph
  [#11383](https://github.com/pulumi/pulumi/pull/11383)

- [cli] pulumi destroy --remove will now delete the stack config file
  [#11394](https://github.com/pulumi/pulumi/pull/11394)

- [cli] Allow rotating the encrpytion key for cloud secrets.
  [#11554](https://github.com/pulumi/pulumi/pull/11554)

- [cli/{config,new,package}] Preserve comments on editing of project and config files.
  [#11456](https://github.com/pulumi/pulumi/pull/11456)

- [sdk/dotnet] Add Output.JsonSerialize using System.Text.Json.
  [#11556](https://github.com/pulumi/pulumi/pull/11556)

- [sdk/go] Add JSONMarshal to go sdk.
  [#11609](https://github.com/pulumi/pulumi/pull/11609)

- [sdkgen/{dotnet,nodejs}] Initial implementation of simplified invokes for dotnet and nodejs.
  [#11418](https://github.com/pulumi/pulumi/pull/11418)

- [sdk/nodejs] Delegates alias computation to engine for Node SDK
  [#11206](https://github.com/pulumi/pulumi/pull/11206)

- [sdk/nodejs] Emit closure requires in global scope for improved cold start on Lambda
  [#11481](https://github.com/pulumi/pulumi/pull/11481)

- [sdk/nodejs] Add output jsonStringify using JSON.stringify.
  [#11605](https://github.com/pulumi/pulumi/pull/11605)

- [sdk/python] Add json_dumps to python sdk.
  [#11607](https://github.com/pulumi/pulumi/pull/11607)


### Bug Fixes

- [backend/service] Fixes out-of-memory issues when using PULUMI_OPTIMIZED_CHECKPOINT_PATCH protocol
  [#11666](https://github.com/pulumi/pulumi/pull/11666)

- [cli] Improve performance of convert to not try and load so many provider plugins.
  [#11639](https://github.com/pulumi/pulumi/pull/11639)

- [programgen] Don't panic on some empty objects
  [#11660](https://github.com/pulumi/pulumi/pull/11660)

- [cli/display] Fixes negative durations on update display.
  [#11631](https://github.com/pulumi/pulumi/pull/11631)

- [programgen/go] Check for optional/ Ptr types within Union types. This fixes a bug in Go programgen where optional outputs are not returned as pointers.
  [#11635](https://github.com/pulumi/pulumi/pull/11635)

- [sdkgen/{dotnet,go,nodejs,python}] Do not generate Result types for functions with empty outputs
  [#11596](https://github.com/pulumi/pulumi/pull/11596)

- [sdk/python] Fix a deadlock on provider-side error with automation api
  [#11595](https://github.com/pulumi/pulumi/pull/11595)

- [sdkgen/{dotnet,nodejs}] Fix imports when a component is using another component from the same schema as a property
  [#11606](https://github.com/pulumi/pulumi/pull/11606)
  [#11467](https://github.com/pulumi/pulumi/pull/11467)

- [sdkgen/go] Illegal cast in resource constructors when secret-wrapping input arguments.
  [#11673](https://github.com/pulumi/pulumi/pull/11673)


### Miscellaneous

- [sdk/nodejs] Remove function serialization code for out of suppport NodeJS versions.
  [#11551](https://github.com/pulumi/pulumi/pull/11551)
  queue-merge: true
  run-dispatch-commands: true
  version-set: {
  "dotnet": "6.0.x",
  "go": "1.18.x",
  "nodejs": "14.x",
  "python": "3.9.x"
}


## 3.49.0 (2022-12-08)


### Features

- [sdk] Add methods to cast pointer types to corresponding Pulumi Ptr types
  [#11539](https://github.com/pulumi/pulumi/pull/11539)

- [yaml] [Updates Pulumi YAML to v1.0.4](https://github.com/pulumi/pulumi-yaml/releases/tag/v1.0.4) unblocking Docker Image resource support in a future Docker provider release.
  [#11583](https://github.com/pulumi/pulumi/pull/11583)

- [backend/service] Allows the service to opt into a bandwidth-optimized DIFF protocol for storing checkpoints. Previously this required setting the PULUMI_OPTIMIZED_CHECKPOINT_PATCH env variable on the client. This env variable is now deprecated.
  [#11421](https://github.com/pulumi/pulumi/pull/11421)

- [cli/about] Add fully qualified stack name to current stack.
  [#11387](https://github.com/pulumi/pulumi/pull/11387)

- [sdk/{dotnet,nodejs}] Add InvokeSingle variants to dotnet and nodejs SDKs
  [#11564](https://github.com/pulumi/pulumi/pull/11564)


### Bug Fixes

- [docs] Exclude id output property for component resources
  [#11469](https://github.com/pulumi/pulumi/pull/11469)

- [engine] Fix an assert for resources being replaced but also pending deletion.
  [#11475](https://github.com/pulumi/pulumi/pull/11475)

- [pkg] Fixes codegen/python generation of non-string secrets in provider properties
  [#11494](https://github.com/pulumi/pulumi/pull/11494)

- [pkg/testing] Optionally caches python venvs for testing
  [#11532](https://github.com/pulumi/pulumi/pull/11532)

- [programgen] Improve error message for invalid enum values on `pulumi convert`.
  [#11019](https://github.com/pulumi/pulumi/pull/11019)

- [programgen] Interpret schema.Asset as pcl.AssetOrArchive.
  [#11593](https://github.com/pulumi/pulumi/pull/11593)

- [programgen/go] Convert the result of immediate invokes to ouputs when necessary.
  [#11480](https://github.com/pulumi/pulumi/pull/11480)

- [programgen/nodejs] Add `.` between `?` and `[`.
  [#11477](https://github.com/pulumi/pulumi/pull/11477)

- [programgen/nodejs] Fix capitalization when generating `fs.readdirSync`.
  [#11478](https://github.com/pulumi/pulumi/pull/11478)

- [sdk/nodejs] Fix regression when passing a provider to a MLC
  [#11509](https://github.com/pulumi/pulumi/pull/11509)

- [sdk/python] Allows for duplicate output values in python
  [#11559](https://github.com/pulumi/pulumi/pull/11559)

- [sdkgen/go] Fixes superfluous newline being added between documentation comment and package statement in doc.go
  [#11492](https://github.com/pulumi/pulumi/pull/11492)

- [sdkgen/nodejs] Generate JS doc comments for output-versioned invokes and use explicit any type.
  [#11511](https://github.com/pulumi/pulumi/pull/11511)

## 3.48.0 (2022-11-23)


### Bug Fixes

- [cli] Don't print update plan message with --json.
  [#11454](https://github.com/pulumi/pulumi/pull/11454)

- [cli] `up --yes` should not use update plans.
  [#11445](https://github.com/pulumi/pulumi/pull/11445)

## 3.47.2 (2022-11-22)


### Features

- [cli] Add prompt to `up` to use experimental update plans.
  [#11353](https://github.com/pulumi/pulumi/pull/11353)


### Bug Fixes

- [sdk/python] Don't error on type mismatches when using input values for outputs
  [#11422](https://github.com/pulumi/pulumi/pull/11422)

## 3.47.1 (2022-11-18)


### Bug Fixes

- [sdk/{dotnet,go,nodejs}] Attempt to select stack then create as fallback on 'createOrSelect'
  [#11402](https://github.com/pulumi/pulumi/pull/11402)

## 3.47.0 (2022-11-17)


### Features

- [cli] Added "--from=tf" to pulumi convert.
  [#11341](https://github.com/pulumi/pulumi/pull/11341)

- [engine] Engine and Golang support for language plugins starting providers directly.
  [#10916](https://github.com/pulumi/pulumi/pull/10916)

- [sdk/dotnet] Add DictionaryInvokeArgs for dynamically constructing invoke input bag of properties.
  [#11335](https://github.com/pulumi/pulumi/pull/11335)

- [sdk/go] Allow sane conversions for `As*Map*` and `As*Array*` conversions.
  [#11351](https://github.com/pulumi/pulumi/pull/11351)

- [sdkgen/{dotnet,nodejs}] Add default dependencies for generated SDKs.
  [#11315](https://github.com/pulumi/pulumi/pull/11315)

- [sdkgen/nodejs] Splits input and output definitions into multiple files.
  [#10831](https://github.com/pulumi/pulumi/pull/10831)


### Bug Fixes

- [cli] Fix stack selection prompt.
  [#11354](https://github.com/pulumi/pulumi/pull/11354)

- [engine] Always keep resources when pulumi:pulumi:getResource is invoked
  [#11382](https://github.com/pulumi/pulumi/pull/11382)

- [pkg] Fix a panic in codegen for an edge case involving object expressions without corresponding function arguments.
  [#11311](https://github.com/pulumi/pulumi/pull/11311)

- [programgen] Enable type checking for resource attributes
  [#11371](https://github.com/pulumi/pulumi/pull/11371)

- [cli/display] Fix text cutting off prior to the edge of the terminal
  [#11202](https://github.com/pulumi/pulumi/pull/11202)

- [programgen/{dotnet,go,nodejs,python}] Don't generate traverse errors when typechecking a dynamic type
  [#11359](https://github.com/pulumi/pulumi/pull/11359)

- [sdk/{go,nodejs,python}] Set acceptResources when invoking pulumi:pulumi:getResource
  [#11382](https://github.com/pulumi/pulumi/pull/11382)

- [sdk/python] Copy ResourceOptions correctly during a merge.
  [#11327](https://github.com/pulumi/pulumi/pull/11327)

## 3.46.1 (2022-11-09)


### Features

- [cli] Enables debug tracing of Pulumi gRPC internals: `PULUMI_DEBUG_GRPC=$PWD/grpc.json pulumi up`
  [#11085](https://github.com/pulumi/pulumi/pull/11085)

- [cli/display] Improve the usability of the interactive dipslay by making the treetable scrollable
  [#11200](https://github.com/pulumi/pulumi/pull/11200)

- [pkg] Add `DeletedWith` as a resource option.
  [#11095](https://github.com/pulumi/pulumi/pull/11095)

- [programgen] More programs can be converted to Pulumi when using `pulumi convert`, provider bridging, and conversion tools by allowing property accesses and field names to fall back to a case insensitive lookup.
  [#11266](https://github.com/pulumi/pulumi/pull/11266)


### Bug Fixes

- [engine] Disable auto parenting to see if that fixes #10950.
  [#11272](https://github.com/pulumi/pulumi/pull/11272)

- [yaml] [Updates Pulumi YAML to v1.0.2](https://github.com/pulumi/pulumi-yaml/releases/tag/v1.0.2) which fixes a bug encountered using templates with project level config.
  [#11296](https://github.com/pulumi/pulumi/pull/11296)

- [sdkgen/go] Allow resource names that conflict with additional types.
  [#11244](https://github.com/pulumi/pulumi/pull/11244)

- [sdkgen/go] Guard against conflicting field names.
  [#11262](https://github.com/pulumi/pulumi/pull/11262)

- [sdk/python] Handle None being passed to register_resource_outputs.
  [#11226](https://github.com/pulumi/pulumi/pull/11226)

## 3.46.0 (2022-11-02)


### Features

- [programgen/{dotnet,go,java,nodejs,python}] Support a logical name for config vars
  [#11231](https://github.com/pulumi/pulumi/pull/11231)

- [sdk/dotnet] Make the `LocalSerializer` class public.
  [#11106](https://github.com/pulumi/pulumi/pull/11106)

- [sdk/yaml] [Updates Pulumi YAML to v1.0.0](https://github.com/pulumi/pulumi-yaml/releases/tag/v1.0.0) containing runtime support for external config.
  [#11222](https://github.com/pulumi/pulumi/pull/11222)


### Bug Fixes

- [engine] Fix a bug in update plans handling resources being replaced due to other resources being deleted before replacement.
  [#11009](https://github.com/pulumi/pulumi/pull/11009)

- [engine] Pending deletes are no longer executed before everything else. This correctly handles dependencies for resource graphs that were partially deleted.
  [#11027](https://github.com/pulumi/pulumi/pull/11027)

- [engine] Expand duplicate URN checks across direct URNs and aliases.
  [#11212](https://github.com/pulumi/pulumi/pull/11212)

## 3.45.0 (2022-10-31)


### Features

- [auto/dotnet] Support for remote operations
  [#11194](https://github.com/pulumi/pulumi/pull/11194)

- [cli/config] Typing made optional, extended short-hand values to arrays and correctly pass stack name to config validator
  [#11192](https://github.com/pulumi/pulumi/pull/11192)

- [auto/go] Support for remote operations
  [#11168](https://github.com/pulumi/pulumi/pull/11168)

- [auto/nodejs] Support for remote operations
  [#11170](https://github.com/pulumi/pulumi/pull/11170)

- [auto/python] Support for remote operations
  [#11174](https://github.com/pulumi/pulumi/pull/11174)


### Bug Fixes

- [sdk/{go,yaml}] Block IsSecret until secretness is known
  [#11189](https://github.com/pulumi/pulumi/pull/11189)

- [sdk/{go,yaml}] Prevent race on resource output
  [#11186](https://github.com/pulumi/pulumi/pull/11186)

## 3.44.3 (2022-10-28)


### Features

- [cli/state] Add the --target-dependents flag to `pulumi state delete`
  [#11164](https://github.com/pulumi/pulumi/pull/11164)


### Bug Fixes

- [cli] Hard reset the templates checkout to work around a go-git issue with ignored files.
  [#11175](https://github.com/pulumi/pulumi/pull/11175)

- [auto/dotnet] allow deserializing complex stack config values.
  [#11143](https://github.com/pulumi/pulumi/pull/11143)

- [auto/{dotnet,go,nodejs,python}] detect concurrent update error from local backend.
  [#11146](https://github.com/pulumi/pulumi/pull/11146)

## 3.44.2 (2022-10-26)


### Features

- [cli] Allow globbing for resources that do not yet exist
  [#11150](https://github.com/pulumi/pulumi/pull/11150)

- [auto/dotnet] Add Json option to UpdateOptions.
  [#11148](https://github.com/pulumi/pulumi/pull/11148)


### Bug Fixes

- [build] Fix release build to continue to use MacOS 11.
  [#11155](https://github.com/pulumi/pulumi/pull/11155)

- [engine] Prevent concurrent read/writes to the component providers map.
  [#11151](https://github.com/pulumi/pulumi/pull/11151)

## 3.44.1 (2022-10-25)


### Bug Fixes

- [engine] Fix an invalid cast in analyzer plugins.
  [#11141](https://github.com/pulumi/pulumi/pull/11141)

## 3.44.0 (2022-10-24)


### Features

- [auto/go] Add InstallPluginFromServer method
  [#10955](https://github.com/pulumi/pulumi/pull/10955)

- [auto/nodejs] Add InstallPluginFromServer
  [#10955](https://github.com/pulumi/pulumi/pull/10955)

- [auto/python] Add install_plugin_from_server
  [#10955](https://github.com/pulumi/pulumi/pull/10955)

- [cli] Implement initial MVP for hierarchical and structured project configuration.
  [#10832](https://github.com/pulumi/pulumi/pull/10832)

- [cli] Allow rotating the passphrase non-interactively
  [#11094](https://github.com/pulumi/pulumi/pull/11094)

- [programgen] Add error reporting infrastructure
  [#11032](https://github.com/pulumi/pulumi/pull/11032)


### Bug Fixes

- [ci] Fix pull request URLs in Pulumi changelogs
  [#11060](https://github.com/pulumi/pulumi/pull/11060)

- [engine] Fix type validation of stack config with secure values.
  [#11084](https://github.com/pulumi/pulumi/pull/11084)

- [cli/engine] Component Resources inherit thier parents providers map
  [#10933](https://github.com/pulumi/pulumi/pull/10933)

- [cli/import] Only trigger an import when necessary during refresh.
  [#11100](https://github.com/pulumi/pulumi/pull/11100)

- [sdk/go] Allow decoding *asset and *archive values
  [#11053](https://github.com/pulumi/pulumi/pull/11053)

- [sdkgen/{go,python}] Handle hypheneated names in go and python
  [#11049](https://github.com/pulumi/pulumi/pull/11049)

- [sdk/nodejs] Fixes loss of undefined type case in `all()`
  [#11048](https://github.com/pulumi/pulumi/pull/11048)

- [sdk/python] pulumi.automation.create_or_select_stack() attempts to select the stack before attempting to create
  [#11115](https://github.com/pulumi/pulumi/pull/11115)

- [sdk/python] Python runtime now respects the --parallel flag.
  [#11122](https://github.com/pulumi/pulumi/pull/11122)


### Miscellaneous

- [protobuf] Bumps python grpcio version
  [#11067](https://github.com/pulumi/pulumi/pull/11067)

- [sdk/go] Update notes, update the deprecated functions, make some lint.
  [#11002](https://github.com/pulumi/pulumi/pull/11002)

## 3.43.1 (2022-10-15)


### Bug Fixes

- [sdkgen/{go,python}] Revert 10738, fixing python class generation
  [#11033](https://github.com/pulumi/pulumi/pull/11033)

## 3.43.0 (2022-10-14)


### Features

- [auto/nodejs] Adds support for parallel programs in NodeJS Automation API
  [#10568](https://github.com/pulumi/pulumi/pull/10568)

- [backend/service] Implements diff-based snapshot saving protocol that reduces bandwidth on large stacks. To opt into this feature, set the environment variable and value `PULUMI_OPTIMIZED_CHECKPOINT_PATCH=true`.
  [#10788](https://github.com/pulumi/pulumi/pull/10788)

- [engine] Adds structured alias support to the engine
  [#10819](https://github.com/pulumi/pulumi/pull/10819)

- [cli/display] Displays time elapsed when modifying a resource.
  [#10953](https://github.com/pulumi/pulumi/pull/10953)

- [sdk/go] Modifies built-in As-ArrayOutput methods to attempt to convert []interface{} to []T.
  [#10991](https://github.com/pulumi/pulumi/pull/10991)

- [sdkgen/go] Add `modulePath` to go, allowing accurate `go.mod` files for prerelease packages
  [#10944](https://github.com/pulumi/pulumi/pull/10944)

- [cli/new] Add --remove flag to`pulumi destroy`
  [#10943](https://github.com/pulumi/pulumi/pull/10943)


### Bug Fixes

- [cli] Project path is included in error messages when a project can't be loaded.
  [#10973](https://github.com/pulumi/pulumi/pull/10973)

- [cli/display] Fix gocloud unconditonally writing to stderr.
  [#11007](https://github.com/pulumi/pulumi/pull/11007)

- [cli/{display,engine}] Use of unsupported ResourceOptions on components will no longer raise resource warnings, instead they are just logged to the diagnostic error stream.
  [#11010](https://github.com/pulumi/pulumi/pull/11010)

- [cli/import] Handle importing resource properties that are typed as a union
  [#10995](https://github.com/pulumi/pulumi/pull/10995)

- [cli/package] Require a path separator for path based binaries. This allows us to distinguish between ./myProvider (execute the binary at path) and myProvider (execute the installed plugin).
  [#11015](https://github.com/pulumi/pulumi/pull/11015)

- [programgen/dotnet] Annotate deeply nested objects with their schema types and apply property name overrides
  [#10976](https://github.com/pulumi/pulumi/pull/10976)

- [programgen/go] Fixes int constant range expressions for go
  [#10979](https://github.com/pulumi/pulumi/pull/10979)

- [programgen/go] Missing default case handling when generating local variables
  [#10978](https://github.com/pulumi/pulumi/pull/10978)

- [sdk/go] Avoid backfilling property deps for Go
  [#11021](https://github.com/pulumi/pulumi/pull/11021)

- [sdkgen] Re-enables caching the schemas of versioned provider plugins.
  [#10971](https://github.com/pulumi/pulumi/pull/10971)

- [programgen/python] Recursively annotate expressions under invoke calls with their associated schema types
  [#10958](https://github.com/pulumi/pulumi/pull/10958)


### Miscellaneous

- [yaml] "[Updates Pulumi YAML to v0.5.10](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.10) containing bug fixes and improvements primarily for `pulumi convert` from YAML."
  [#11018](https://github.com/pulumi/pulumi/pull/11018)

## 3.42.0 (2022-10-07)


### Bug Fixes

- [cli/new] Fix cloning templates from Azure DevOps repos.
  [#10954](https://github.com/pulumi/pulumi/pull/10954)

- [docs] Allow more flexible parsing when extracting examples from doc comments
  [#10913](https://github.com/pulumi/pulumi/pull/10913)

- [sdkgen/python] Fixes dangling type-refs generated under compatibility=tfbridge20 for schemas that refer to types aross modules.
  [#10935](https://github.com/pulumi/pulumi/pull/10935)

## 3.41.1 (2022-10-05)


### Features

- [backend] Allows CLI auth for Azure blob storage
  [#10900](https://github.com/pulumi/pulumi/pull/10900)

- [cli/{about,plugin}] Remove experimental feature for plugins from private github releases. This is now supported by `github:` plugin urls, see https://www.pulumi.com/docs/guides/pulumi-packages/how-to-author/#support-for-github-releases.
  [#10896](https://github.com/pulumi/pulumi/pull/10896)

- [sdk/go] Adds an `UnsafeAwaitOutput` function to the Go SDK. This permits a workaround for component providers and other advanced scenarios where resources created are conditional on an output.
  [#10877](https://github.com/pulumi/pulumi/pull/10877)

- [sdk/python] Add invoke to Provider interface.
  [#10906](https://github.com/pulumi/pulumi/pull/10906)

- [sdk/python] Add Output.format to the python SDK.
  [#10919](https://github.com/pulumi/pulumi/pull/10919)

- [yaml] [Updates Pulumi YAML to v0.5.9](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.9)
  [#10937](https://github.com/pulumi/pulumi/pull/10937)


### Bug Fixes

- [cli] Prevent sending on a closed channel panics during 'pulumi convert'
  [#10893](https://github.com/pulumi/pulumi/pull/10893)

- [programgen/go] Fix codegen for `__apply` functions
  [#10775](https://github.com/pulumi/pulumi/pull/10775)

- [sdk/go] Go programs run with Go 1.17 or below failed due to go mod tidy being run with -compat=1.18. The change is reverted.
  [#10865](https://github.com/pulumi/pulumi/pull/10865)

- [sdk/go] Fixed bug in (ours, theirs) to (theirs, theirs)
  [#10881](https://github.com/pulumi/pulumi/pull/10881)

- [sdk/python] Fix KeyError in UpdateSummary.
  [#10907](https://github.com/pulumi/pulumi/pull/10907)

- [sdkgen/nodejs] Fixes a bug with lazy-loaded modules that caused mixins to observe unexpected null values.
  [#10871](https://github.com/pulumi/pulumi/pull/10871)

## 3.40.2 (2022-09-27)


### Features

- [cli] Allow per-execution override of the cloud secrets provider url via the `PULUMI_CLOUD_SECRET_OVERRIDE` environment variable. This allows a temporary replacement without updating the stack config, such a during CI. This does not effect stacks using service secrets or passphrases.
  [#10749](https://github.com/pulumi/pulumi/pull/10749)
  [#10749](https://github.com/pulumi/pulumi/pull/10749)
  [#10749](https://github.com/pulumi/pulumi/pull/10749)

- [cli/new] Enables `pulumi new` to use templates from Azure DevOps(currently limited to master/main branches and does not support providing subdirectories).
  [#10789](https://github.com/pulumi/pulumi/pull/10789)

- [engine] 'pulumi policy new' now uses the same system as 'pulumi new' to install dependencies.
  [#10797](https://github.com/pulumi/pulumi/pull/10797)

- [programgen] Support resource option "version" in `pulumi convert` to select specific provider SDK versions.
  [#10194](https://github.com/pulumi/pulumi/pull/10194)

- [yaml] [Updates Pulumi YAML to v0.5.8](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.8)
  [#10856](https://github.com/pulumi/pulumi/pull/10856)

- [cli/plugin] Don't retry plugin downloads in 403 and 404 responses
  [#10803](https://github.com/pulumi/pulumi/pull/10803)

- [sdk/dotnet] Added `Deployment.OrganizationName` to return the current organization if available.
  [#10564](https://github.com/pulumi/pulumi/pull/10564)

- [sdk/go] Pulumi Go Programs now support a Pulumi.yaml option `buildTarget: path/to/binary` to compile/recompile a Go binary to that location.
  [#10731](https://github.com/pulumi/pulumi/pull/10731)

- [sdk/go] Added `Context.Organization` to return the current organization if available.
  [#10811](https://github.com/pulumi/pulumi/pull/10811)


### Bug Fixes

- [ci] Re-enable Homebrew Tap publishing.
  [#10796](https://github.com/pulumi/pulumi/pull/10796)

- [cli] Fixes --tracing to account for response parsing in HTTP api/* spans.
  [#10828](https://github.com/pulumi/pulumi/pull/10828)

- [cli] Fixes Pulumi.yaml validation error when the `refresh: always` option is specified
  [#10833](https://github.com/pulumi/pulumi/pull/10833)

- [engine] Mark pulumi-analyzer-policy and pulumi-analyzer-policy-python as bundled plugins.
  [#10817](https://github.com/pulumi/pulumi/pull/10817)

- [engine] Fix node and python MLCs on Windows.
  [#10827](https://github.com/pulumi/pulumi/pull/10827)

- [sdkgen/dotnet] Fixes a .NET SDK codegen bug when emitting functions with secret parameters.
  [#10840](https://github.com/pulumi/pulumi/pull/10840)

- [sdkgen/dotnet] Fix the type emitted for `ImmutableArray.Create` and `ImmutableDictionary.Create` for secret properties.
  [#10850](https://github.com/pulumi/pulumi/pull/10850)

- [sdk/nodejs] The `@pulumi/pulumi` package is now interoperable with ESModules.
  [#10622](https://github.com/pulumi/pulumi/pull/10622)

- [sdk/{nodejs,python}] `getOrganization` now returns "organization" by default.
  [#10820](https://github.com/pulumi/pulumi/pull/10820)

- [programgen/yaml] Fix incorrect import for non-pulumi owned package on convert
  [#10727](https://github.com/pulumi/pulumi/pull/10727)

## 3.40.1 (2022-09-17)


### Features

- [backend] Adds a flag `PULUMI_SKIP_CHECKPOINTS=true` that makes Pulumi skip saving state checkpoints as it modifies resources and only save the final state of a deployment. 
  [#10750](https://github.com/pulumi/pulumi/pull/10750)

   This is an experimental feature that also requires `PULUMI_EXPERIMENTAL=true` to be set.

   Using the feature introduces risk that in the case of network disconnect or crash state edits will be lost and may require manual recovery. When this risk is acceptable, using the feature can speed up Pulumi deployments.

   See also:

     - [Checkpoints](https://www.pulumi.com/docs/intro/concepts/state/#checkpoints)
     - [#10668](https://github.com/pulumi/pulumi/issues/10668)

- [ci] Improves first-time contributor developer experience and reduces test execution time by defaulting `integration.ProgramTest` to a filestate backend. Tests that require running against a service should set `RequireService` to `true`.
  [#10720](https://github.com/pulumi/pulumi/pull/10720)

- [cli] Add a package author focused subcommand: `pulumi package` with subcommands `pulumi package gen-sdk` and `pulumi package get-schema`.
  [#10711](https://github.com/pulumi/pulumi/pull/10711)

- [cli] Use "https://github.com/pulumi/pulumi/blob/master/sdk/go/common/workspace/project.json" to validate loaded project files.
  [#10596](https://github.com/pulumi/pulumi/pull/10596)


### Bug Fixes

- [sdk/go] Correctly handle nil resource references in the RPC layer.
  [#10739](https://github.com/pulumi/pulumi/pull/10739)

## 3.40.0 (2022-09-14)

### Bug fixes

- [engine] Plugin resolution now automatically installs any missing plugins as they are encountered.
   [#10691](https://github.com/pulumi/pulumi/pull/10691)

### Miscellaneous

- [ci] Miscellaneous improvements to release process.

## 3.39.4 (2022-09-14)


### Improvements

- [provider/go]: Added support for token authentication in the go providers which use git.
  [#10628](https://github.com/pulumi/pulumi/pull/10628)

- [codegen/go] Chunk the `pulumiTypes.go` file to reduce max file size.
  [#10666](https://github.com/pulumi/pulumi/pull/10666)

### Bug Fixes

- Fix invalid resource type on `pulumi convert` to Go
  [#10670](https://github.com/pulumi/pulumi/pull/10670)

- [auto/nodejs] `onOutput` is now called incrementally as the
  underyling Pulumi process produces data, instead of being called
  once at the end of the process execution. This restores behavior
  that regressed since 3.39.0.
  [#10678](https://github.com/pulumi/pulumi/pull/10678)

### Miscellaneous

- [ci] Migrate to merge queues for more reliable builds
  [#10644](https://github.com/pulumi/pulumi/pull/10644)


## 3.39.3 (2022-09-07)

### Improvements

- [sdk/python] Improve error message when pulumi-python cannot find a main program.
  [#10617](https://github.com/pulumi/pulumi/pull/10617)

- [cli] provide info message to user if a pulumi program contains no resources
  [#10461](https://github.com/pulumi/pulumi/issues/10461)

### Bug Fixes

- [engine/plugins]: Revert change causing third party provider packages to prevent deployment commands (`up`, `preview`, ...)
  when used with the nodejs runtime. Reverts #10530.
  [#10650](https://github.com/pulumi/pulumi/pull/10650)

## 3.39.2 (2022-09-07)

### Improvements

- [sdk/go] Pulumi Go programs, on failure, now log a single error message.
  [#10347](https://github.com/pulumi/pulumi/pull/10347)

- [sdk/nodejs] Updated the vendored version of TypeScript in the NodeJS SDK and runtime from v3.7.3 to v3.8.3
  [#10618](https://github.com/pulumi/pulumi/pull/10618)

### Bug Fixes

- [sdk/nodejs] Calls onOutput in runPulumiCmd
  [#10631](https://github.com/pulumi/pulumi/pull/10631)

## 3.39.1 (2022-09-02)

### Improvements

- [cli] Display outputs last in diff view.
  [#10535](https://github.com/pulumi/pulumi/pull/10535)

- [sdk/python] Dropped support for Python 3.6.
  [#10529](https://github.com/pulumi/pulumi/pull/10529)

- [codegen/nodejs] Support lazy-loading Node modules.
  [#10538](https://github.com/pulumi/pulumi/pull/10538)

- [cli/backend] Gzip compress HTTPS payloads for `pulumi import` and secret decryption against
  the Pulumi Service backend.
  [#10558](https://github.com/pulumi/pulumi/pull/10558)

### Bug Fixes

- [cli] Fix the "no Pulumi.yaml project file found" error message.
  [#10592](https://github.com/pulumi/pulumi/pull/10592)

- [cli/refresh] Do not panic when snapshot is `nil`.
  [#10593](https://github.com/pulumi/pulumi/pull/10593)

- [sdk/{python,nodejs}] Fix the use of `getOrganization` in policy packs.
  [#10574](https://github.com/pulumi/pulumi/pull/10574)

## 3.39.0 (2022-09-01)

### Improvements

- [yaml] [Updates Pulumi YAML to v0.5.5](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.5)

- [cli] Allow `pulumi refresh` to interactively resolve pending creates.
  [#10394](https://github.com/pulumi/pulumi/pull/10394)

- [cli] Clarify highlighting of confirmation text in `confirmPrompt`.
  [#10413](https://github.com/pulumi/pulumi/pull/10413)

- [provider/python]: Improved exception display. The traceback is now shorter and it always starts with user code.
  [#10336](https://github.com/pulumi/pulumi/pull/10336)

- [sdk/python] Update PyYAML to 6.0

- [cli/watch] `pulumi watch` now uses relies on a program built on [`watchexec`](https://github.com/watchexec/watchexec)
  to implement recursive file watching, improving performance and cross-platform compatibility.
  This `pulumi-watch` program is now included in releases.
  [#10213](https://github.com/pulumi/pulumi/issues/10213)

- [codegen] Reduce time to execute `pulumi convert` and some YAML programs, depending on providers used, by up to 3 seconds.
  [#10444](https://github.com/pulumi/pulumi/pull/10444)


- [dotnet/sdk] Implement `Deployment.TestAsync` overloads which accept functions that create resources without requiring a stack definition.
  [#10458](https://github.com/pulumi/pulumi/pull/10458)

- [sdk/nodejs] Added stack truncation to `SyntaxError` in nodejs.
  [#10465](https://github.com/pulumi/pulumi/pull/10465)

- [sdk/python] Makes global SETTINGS values context-aware to not leak state between Pulumi programs running in parallel
  [#10402](https://github.com/pulumi/pulumi/pull/10402)

- [sdk/python] Makes global ROOT, CONFIG, _SECRET_KEYS ContextVars to not leak state between parallel inline Pulumi programs
  [#10472](https://github.com/pulumi/pulumi/pull/10472)

- [sdk/go] Improve error messages for `StackReference`s
  [#10477](https://github.com/pulumi/pulumi/pull/10477)

- [sdk/dotnet] Added `Output.CreateSecret<T>(Output<T> value)` to set the secret bit on an output value.
  [#10467](https://github.com/pulumi/pulumi/pull/10467)

- [policy] `pulumi policy publish` now takes into account `.gitignore` files higher in the file tree.
  [#10493](https://github.com/pulumi/pulumi/pull/10493)

- [sdk/go] enable direct compilation via `go build`(set `PULUMI_GO_USE_RUN=true` to opt out)
  [#10375](https://github.com/pulumi/pulumi/pull/10375)

- [cli/backend] Updates no longer immediately renew the token but wait
  until the token is halfway through its expiration period. Currently
  it is assumed tokens expire in 5 minutes, so with this change the
  first lease renewal now happens approximately 2.5 minutes after the
  start of the update. The change optimizes startup latency for Pulumi
  CLI. [#10462](https://github.com/pulumi/pulumi/pull/10462)

- [cli/plugin] `plugin install` now supports a `--checksum` option.
  [#10528](https://github.com/pulumi/pulumi/pull/10528)

- [sdk/{nodejs/python}] Added `getOrganization()` to return the current organization if available.
  [#10504](https://github.com/pulumi/pulumi/pull/10504)

### Bug Fixes

- [codegen/go] Fix StackReference codegen.
  [#10260](https://github.com/pulumi/pulumi/pull/10260

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)

- [cli] Fix VCS detection for domains other than .com and .org.
  [#10415](https://github.com/pulumi/pulumi/pull/10415)

- [codegen/go] Fix incorrect method call for reading floating point values from configuration.
  [#10445](https://github.com/pulumi/pulumi/pull/10445)

- [engine]: HTML characters are no longer escaped in JSON output.
  [#10440](https://github.com/pulumi/pulumi/pull/10440)

- [codegen/go] Ensure consistency between go docs information and package name
  [#10452](https://github.com/pulumi/pulumi/pull/10452)

- [auto/go] Clone non-default branches (and tags).
  [#10285](https://github.com/pulumi/pulumi/pull/10285)

- [cli] Fixes `survey.v1` panics in Terminal UI introduced in
  [#10130](https://github.com/pulumi/pulumi/issues/10130) in v3.38.0.
  [#10475](https://github.com/pulumi/pulumi/pull/10475)

- [codegen/ts] Fix non-pulumi owned provider import alias.
  [#10447](https://github.com/pulumi/pulumi/pull/10447)

- [codegen/go] Fix import path for non-pulumi owner providers
  [#10485](https://github.com/pulumi/pulumi/pull/10485)

- [cli] Fixes panics on repeat Ctrl+C invocation during long-running updates
  [#10489](https://github.com/pulumi/pulumi/pull/10489)

- [cli] Improve Windows reliability with dependency update to ssh-agent
  [#10486](https://github.com/pulumi/pulumi/pull/10486)

- [sdk/{dotnet,nodejs,python}] Dynamic providers and automation API will not trigger a firewall
  permission prompt, will only accept network requests via loopback address.
  [#10498](https://github.com/pulumi/pulumi/pull/10498)
  [#10502](https://github.com/pulumi/pulumi/pull/10502)
  [#10503](https://github.com/pulumi/pulumi/pull/10503)

- [cli] Fix `pulumi console` command to follow documented behavior in help message/docs.
  [#10509](https://github.com/pulumi/pulumi/pull/10509)

- [sdk/nodejs] Fixes an issue which would occur when multiple processes were spawned and some would receive no stdout/stderr
  [10522](https://github.com/pulumi/pulumi/pull/10522)

- [engine] Plugin resolution now automatically installs any missing plugins as they are encountered.
  [#10530](https://github.com/pulumi/pulumi/pull/10530)

- [python] put python version check after installing dependencies to resolve `fork/exec` warning
  [#10524](https://github.com/pulumi/pulumi/pull/10524)

- [go/codegen] Fix generating invalid Go code when derivatives of input types collide with existing resource types
  [#10551](https://github.com/pulumi/pulumi/pull/10551)

## 3.38.0 (2022-08-16)

### Improvements

- [cli] Updated to the latest version of go-git.
  [#10330](https://github.com/pulumi/pulumi/pull/10330)

- [sdk] Merge python error message and traceback into single error message.
   [#10348](https://github.com/pulumi/pulumi/pull/10348)

- [sdk/python] Support optional default parameters in pulumi.Config
  [#10344](https://github.com/pulumi/pulumi/pull/10344)

- [sdk/nodejs] Adds warning message when entrypoint resolution is ambiguous
  [#3582](https://github.com/pulumi/pulumi/issues/3582)

- [automation] Add options for Automation API in each SDK to control logging and tracing, allowing
  automation API to run with the equivalent of CLI arguments `--logflow`, `--verbose`,
  `--logtostderr`, `--tracing` and `--debug`
  [#10338](https://github.com/pulumi/pulumi/pull/10338)

- [yaml] [Updates Pulumi YAML to v0.5.4](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.4)

- [java] [Updates Pulumi Java to v0.5.4](https://github.com/pulumi/pulumi-java/releases/tag/v0.5.4)

- [cli] `pulumi about` now queries language plugins for information, rather than having hardcoded language logic.
  [#10392](https://github.com/pulumi/pulumi/pull/10392)

- [sdk/python] Deprecate PULUMI_TEST_MODE
  [#10400](https://github.com/pulumi/pulumi/pull/10400)


### Bug Fixes

- [cli] Paginate template options
  [#10130](https://github.com/pulumi/pulumi/issues/10130)

- [sdk/dotnet] Fix serialization of non-generic list types.
  [#10277](https://github.com/pulumi/pulumi/pull/10277)

- [codegen/nodejs] Correctly reference external enums.
  [#10286](https://github.com/pulumi/pulumi/pull/10286)

- [sdk/python] Support deeply nested protobuf objects.
  [#10284](https://github.com/pulumi/pulumi/pull/10284)

- Revert [Remove api/renewLease from startup crit path](pulumi/pulumi#10168) to fix #10293.
  [#10294](https://github.com/pulumi/pulumi/pull/10294)

- [codegen/go] Remove superfluous double forward slash from doc.go
  [#10317](https://github.com/pulumi/pulumi/pull/10317)

- [codegen/go] Add an external package cache option to `GenerateProgramOptions`.
  [#10340](https://github.com/pulumi/pulumi/pull/10340)

- [cli/plugins] Don't retry plugin downloads that failed due to local file errors.
  [#10341](https://github.com/pulumi/pulumi/pull/10341)

- [dotnet] Set environment exit code during `Deployment.RunAsync` in case users don't bubble it the program entry point themselves
  [#10217](https://github.com/pulumi/pulumi/pull/10217)

- [cli/display] Fix a panic in the color logic.
  [#10354](https://github.com/pulumi/pulumi/pull/10354

## 3.37.2 (2022-07-29)

### Bug Fixes

- [sdk/dotnet] Fix serialization of non-generic list types.
  [#10277](https://github.com/pulumi/pulumi/pull/10277)

- [codegen/nodejs] Correctly reference external enums.
  [#10286](https://github.com/pulumi/pulumi/pull/10286)

- [sdk/python] Support deeply nested protobuf objects.
  [#10284](https://github.com/pulumi/pulumi/pull/10284)

- Revert [Remove api/renewLease from startup crit path](pulumi/pulumi#10168) to fix #10293.
  [#10294](https://github.com/pulumi/pulumi/pull/10294)


## 3.37.1 (2022-07-27)

### Improvements

- [cli] Groups `pulumi help` commands by category
  [#10170](https://github.com/pulumi/pulumi/pull/10170)

- [sdk/nodejs] Removed stack trace output for Typescript compilation errors
  [#10259](https://github.com/pulumi/pulumi/pull/10259)

### Bug Fixes

- [cli] Fix installation failures on Windows due to release artifacts shipped omitting a folder, `pulumi/*.exe` instead
  of `pulumi/bin/*.exe`.
  [#10264](https://github.com/pulumi/pulumi/pull/10264)

## 3.37.0 (2022-07-27)

### Breaking

- [engine] Engine now always encrypts secret return values in Invoke.
           May break `Language SDKs<3.0.0`
  [#10239](https://github.com/pulumi/pulumi/pull/10239)

### Improvements

- [auto/go] Adds the ability to capture incremental `stderr`
  via the new option `ErrorProgressStreams`.
  [#10179](https://github.com/pulumi/pulumi/pull/10179)

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

- [dotnet/codegen] code generation for csharp Pulumi programs now targets .NET 6
  [#10143](https://github.com/pulumi/pulumi/pull/10143)

- [cli] Allow `pulumi plugin install <type> <pkg> -f <path>` to target a binary
  file or a folder.
  [#10094](https://github.com/pulumi/pulumi/pull/10094)

- [cli/config] Allow `pulumi config cp --path` between objects.
  [#10147](https://github.com/pulumi/pulumi/pull/10147)

- [codegen/schema] Support stack reference as a resource
  [#10174](https://github.com/pulumi/pulumi/pull/10174)

- [backends] When logging in to a file backend, validate that the bucket is accessible.
  [#10012](https://github.com/pulumi/pulumi/pull/10012)

- [cli] Add flag to specify whether to install dependencies on `pulumi convert`.
  [#10198](https://github.com/pulumi/pulumi/pull/10198)

- [cli] Expose `gen-completion` command when running `pulumi --help`.
  [#10218](https://github.com/pulumi/pulumi/pull/10218)

- [sdk/go] Expose context.Context from pulumi.Context.
  [#10190](https://github.com/pulumi/pulumi/pull/10190)

- [cli/plugins] Add local plugin linkage in `Pulumi.yaml`.
  [#10146](https://github.com/pulumi/pulumi/pull/10146)

- [engine] Remove sequence numbers from the engine and provider interfaces.
  [#10203](https://github.com/pulumi/pulumi/pull/10203)

- [engine] The engine will retry plugin downloads that error.
  [#10248](https://github.com/pulumi/pulumi/pull/10248)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [sdk/go] `config.Encrypter` and `config.Decrypter` interfaces now
  require explicit `Context`. This is a minor breaking change to the
  SDK. The change fixes parenting of opentracing spans that decorate
  calls to the Pulumi Service crypter.

  [#10037](https://github.com/pulumi/pulumi/pull/10037)

- [codegen/go] Support program generation, `pulumi convert` for programs that create explicit
  provider resources.
  [#10132](https://github.com/pulumi/pulumi/issues/10132)

- [sdk/go] Remove the `AsName` and `AsQName` asserting functions.
  [#10156](https://github.com/pulumi/pulumi/pull/10156)

- [python] PULUMI_PYTHON_CMD is checked for deciding what python binary to use in a virtual environment.
  [#10155](https://github.com/pulumi/pulumi/pull/10155)

- [cli] Reduced the noisiness of `pulumi new --help` by replacing the list of available templates to just the number.
  [#10164](https://github.com/pulumi/pulumi/pull/10164)

- [cli] Revert "Add last status to `pulumi stack ls` output #10059"
  [#10221](https://github.com/pulumi/pulumi/pull/10221)

- [python] Fix overriding of PATH on Windows.
  [#10236](https://github.com/pulumi/pulumi/pull/10236)

- [dotnet/codgen] Allow specified root namespace to be suffixed with "Pulumi" when generating packages
  [#10245](https://github.com/pulumi/pulumi/pull/10245)

- [dotnet/sdk] Serialize immutable arrays initialized by default.
  [#10247](https://github.com/pulumi/pulumi/pull/10247)

- [dotnet/codegen] Override static `Empty` property to return concrete argument types
  [#10250](https://github.com/pulumi/pulumi/pull/10250)

## 3.36.0 (2022-07-13)

### Improvements

- [cli] Display outputs during the very first preview.
  [#10031](https://github.com/pulumi/pulumi/pull/10031)

- [cli] Add Last Status to `pulumi stack ls` output.
  [#6148](https://github.com/pulumi/pulumi/pull/6148)

- [cli] Allow pulumi `destroy -s <stack>` if not in a Pulumi project dir
  [#9918](https://github.com/pulumi/pulumi/pull/9918)

- [protobuf] Pulumi protobuf messages are now namespaced under "pulumi".
  [#10074](https://github.com/pulumi/pulumi/pull/10074)

- [cli] Truncate long stack outputs
  [#9905](https://github.com/pulumi/pulumi/issues/9905)

- [sdk/go] Add `As*Output` methods to `AnyOutput`
  [#10085](https://github.com/pulumi/pulumi/pull/10085)

- [yaml] [Updates Pulumi YAML to v0.5.3](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.3)

- [sdk/go] Accept a short branch name in `GitRepo.Branch`
  [#10118](https://github.com/pulumi/pulumi/pull/10118)

### Bug Fixes

- [cli] `pulumi convert` help text is wrong
  [#9892](https://github.com/pulumi/pulumi/issues/9892)

- [go/codegen] fix error assignment when creating a new resource in generated go code
  [#10049](https://github.com/pulumi/pulumi/pull/10049)

- [cli] `pulumi convert` generates incorrect input parameter names for C#
  [#10042](https://github.com/pulumi/pulumi/issues/10042)

- [engine] Un-parent child resource when a resource is deleted during a refresh.
  [#10073](https://github.com/pulumi/pulumi/pull/10073)

- [cli] `pulumi state change-secrets-provider` now takes `--stack` into account
  [#10075](https://github.com/pulumi/pulumi/pull/10075)

- [nodejs/sdkgen] Default set `pulumi.name` in package.json to the pulumi package name.
  [#10088](https://github.com/pulumi/pulumi/pull/10088)

- [sdk/python] update protobuf library to v4 which speeds up pulumi CLI dramatically on M1 machines
  [#10063](https://github.com/pulumi/pulumi/pull/10063)

- [engine] Fix data races discovered in CLI and Go SDK that could cause nondeterministic behavior
  or a panic.
  [#10081](https://github.com/pulumi/pulumi/pull/10081),
  [#10100](https://github.com/pulumi/pulumi/pull/10100)

- [go] Fix panic when returning pulumi.Bool, .String, .Int, and .Float64 in the argument to
  ApplyT and casting the result to the corresponding output, e.g.: BoolOutput.
  [#10103](https://github.com/pulumi/pulumi/pull/10103)

## 3.35.3 (2022-07-01)

### Improvements

- [plugins] Plugin download urls now support GitHub as a first class url schema. For example "github://api.github.com/pulumiverse".
  [#9984](https://github.com/pulumi/pulumi/pull/9984)

- [nodejs] No longer roundtrips requests for the stack URN via the engine.
  [#9680](https://github.com/pulumi/pulumi/pull/9680)

### Bug Fixes

- [cli] `pulumi convert` supports provider packages without a version.
  [#9976](https://github.com/pulumi/pulumi/pull/9976)

- [cli] Revert changes to how --target works. This means that non-targeted resources do need enough valid inputs to pass Check.
  [#10024](https://github.com/pulumi/pulumi/pull/10024)

## 3.35.2 (2022-06-29)

### Improvements

- [sdk/go] Added `PreviewDigest` for third party tools to be able to ingest the preview json
  [#9886](https://github.com/pulumi/pulumi/pull/9886)

- [cli] Do not require the `--yes` option if the `--skip-preview` option is set.
  [#9972](https://github.com/pulumi/pulumi/pull/9972)

- [yaml] [Updates Pulumi YAML to v0.5.2](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.2),
  containing bug fixes and improvements primarily for `pulumi convert` from YAML.
  [#9993](https://github.com/pulumi/pulumi/pull/9993)

### Bug Fixes

- [engine] Filter out non-targeted resources much earlier in the engine cycle.
  [#9960](https://github.com/pulumi/pulumi/pull/9960)

- [cli] Fix a panic in `pulumi plugin ls --project --json`.
  [#9998](https://github.com/pulumi/pulumi/pull/9998)

- [engine] Revert support for aliases in the engine.
  [#9999](https://github.com/pulumi/pulumi/pull/9999)

## 3.35.1 (2022-06-23)

### Improvements

- [cli] The engine will now warn when a resource option is applied to a Component resource when that option will have no effect. This extends [#9863](https://github.com/pulumi/pulumi/pull/9863) which only warns for the `ignoreChanges` resource options.
  [#9921](https://github.com/pulumi/pulumi/pull/9921)

- [auto/*] Add a option to control the `--show-secrets` flag in the automation API.
  [#9879](https://github.com/pulumi/pulumi/pull/9879)

### Bug Fixes

- [auto/go] Fix passing of the color option.
  [#9940](https://github.com/pulumi/pulumi/pull/9940)

- [engine] Fix panic from unexpected resource name formats.
  [#9950](https://github.com/pulumi/pulumi/pull/9950)


## 3.35.0 (2022-06-22)

### Improvements

- [auto/*] Add `--policy-pack` and `--policy-pack-configs` options to automation API.
  [#9872](https://github.com/pulumi/pulumi/pull/9872)

- [cli] The engine now produces a warning when the 'ignoreChanges' option is applied to a Component resource.
  [#9863](https://github.com/pulumi/pulumi/pull/9863)

- [sdk/python] Changed `Output[T].__str__()` to return an informative message rather than "<pulumi.output.Output object at 0x012345ABCDEF>".
  [#9848](https://github.com/pulumi/pulumi/pull/9848)

- [cli] The engine will now default resource parent to the root stack if it exists.
  [#9481](https://github.com/pulumi/pulumi/pull/9481)

- [engine] Reduce memory usage in convert and yaml programs by caching of package schemas.
  [#9684](https://github.com/pulumi/pulumi/issues/9684)

- [sdk/go] Added `NewUniqueName` for providers to use for generating autonames.
  [#9852](https://github.com/pulumi/pulumi/pull/9852)

- [engine] The engine now understands alias objects which it can resolve to URNs, requiring less logic in SDKs.
  [#9731](https://github.com/pulumi/pulumi/pull/9731)

- [sdk/dotnet] The dotnet SDK will now send alias objects rather than URNs to the engine.
  [#9731](https://github.com/pulumi/pulumi/pull/9731)

- [cli] Add java to `pulumi convert`
  [#9917](https://github.com/pulumi/pulumi/pull/9917)

### Bug Fixes

- [sdk/go] Handle nils in mapper encoding.
  [#9810](https://github.com/pulumi/pulumi/pull/9810)

- [engine] Explicit providers use the same plugin as default providers unless otherwise requested.
  [#9708](https://github.com/pulumi/pulumi/pull/9708)

- [sdk/go] Correctly parse nested git projects in GitLab.
  [#9354](https://github.com/pulumi/pulumi/issues/9354)

- [sdk/go] Mark StackReference keys that don't exist as unknown. Error when converting unknown keys to strings.
  [#9855](https://github.com/pulumi/pulumi/pull/9855)

- [sdk/go] Precisely mark values obtained via stack reference `Get...Output(key)` methods as secret or not.
  [#9842](https://github.com/pulumi/pulumi/pull/9842)

- [codegen/go] Import external Enum types as external.
  [#9920](https://github.com/pulumi/pulumi/pull/9920)

- [codegen/go] Correctly generate nested `Input` and `Ouput` collection types.
  [#9887](https://github.com/pulumi/pulumi/pull/9887)

- [engine] Revert the additional secret outputs warning until the engine can understand optional outputs.
  [#9922](https://github.com/pulumi/pulumi/pull/9922)


## 3.34.1 (2022-06-10)

### Improvements

- [cli/python] Respond to SIGINT when installing plugins.
  [#9793](https://github.com/pulumi/pulumi/pull/9793)

- [codegen/go] Always chose the correct version when `respectSchemaVersion` is set.
  [#9798](https://github.com/pulumi/pulumi/pull/9798)

### Bug Fixes

- [sdk/python] Better explain the keyword arguments to create(etc)_stack.
  [#9794](https://github.com/pulumi/pulumi/pull/9794)

- [cli] Revert changes causing a panic in `pulumi destroy` that tried to operate without config files.
  [#9821](https://github.com/pulumi/pulumi/pull/9821)

- [cli] Revert to statically linked binaries on Windows and Linux,
  fixing a regression introduced in 3.34.0
  [#9816](https://github.com/pulumi/pulumi/issues/9816)

- [codegen/python] ResourceOptions are no longer mutated by resources.
  [#9802](https://github.com/pulumi/pulumi/pull/9802)

## 3.34.0 (2022-06-08)

### Improvements

- [codegen/go] Handle long and complicated traversals in a nicer way.
  [#9726](https://github.com/pulumi/pulumi/pull/9726)

- [cli] Allow pulumi `destroy -s <stack>` if not in a Pulumi project dir
  [#9613](https://github.com/pulumi/pulumi/pull/9613)

- [cli] Plugins will now shut themselves down if they can't contact the engine that started them.
  [#9735](https://github.com/pulumi/pulumi/pull/9735)

- [cli/engine] The engine will emit a warning of a key given in additional secret outputs doesn't match any of the property keys on the resources.
  [#9750](https://github.com/pulumi/pulumi/pull/9750)

- [sdk/go] Add `CompositeInvoke` function, like `Composite` but for `InvokeOption`.
  [#9752](https://github.com/pulumi/pulumi/pull/9752)

- [cli] The `gcpkms://` now supports the same credentials resolution mechanism as the state store, including the `GOOGLE_CREDENTIALS`.`
  [#6379](https://github.com/pulumi/pulumi/pull/6379)

- [yaml] [Updates Pulumi YAML to v0.5.1](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.1),
  containing bug fixes, new functions, diagnostics and validation. Fixes support for using providers
  such as `awsx`.
  [#9797](https://github.com/pulumi/pulumi/pull/9797)

### Bug Fixes

- [sdk/nodejs] Fix a crash due to dependency cycles from component resources.
  [#9683](https://github.com/pulumi/pulumi/pull/9683)

- [cli/about] Make `pulumi about` aware of the YAML and Java runtimes.
  [#9745](https://github.com/pulumi/pulumi/pull/9745)

- [cli/engine] Fix a panic deserializing resource plans without goals.
  [#9749](https://github.com/pulumi/pulumi/pull/9749)

- [cli/engine] Provide a sorting for plugins of equivalent version.
  [#9761](https://github.com/pulumi/pulumi/pull/9761)

- [cli/backend] Fix degraded performance in filestate backend
  [#9777](https://github.com/pulumi/pulumi/pull/9777)
  [#9796](https://github.com/pulumi/pulumi/pull/9796)

- [engine] Engine correctly the number of bytes written to a TAR archive is what was expected.
  [#9792](https://github.com/pulumi/pulumi/pull/9792)

## 3.33.2 (2022-05-27)

### Improvements

- [cli] `pulumi logout` now prints a confirmation message that it logged out.
  [#9641](https://github.com/pulumi/pulumi/pull/9641)

- [cli/backend] Add gzip compression to filestate backend. Compression can be enabled via `PULUMI_SELF_MANAGED_STATE_GZIP=true`. Special thanks to @awoimbee for the initial PR.
  [#9610](https://github.com/pulumi/pulumi/pull/9610)

- [sdk/nodejs] Lazy load inflight context to remove module import side-effect.
  [#9375](https://github.com/pulumi/pulumi/issues/9375)

### Bug Fixes

- [sdk/python] Fix spurious diffs causing an "update" on resources created by dynamic providers.
  [#9656](https://github.com/pulumi/pulumi/pull/9656)

- [sdk/python] Do not depend on the children of remote component resources.
  [#9665](https://github.com/pulumi/pulumi/pull/9665)

- [codegen/nodejs] Emit the "package.json".pulumi.server as "server" correctly. Previously, "pluginDownloadURL" was emitted but never read.
  [#9662](https://github.com/pulumi/pulumi/pull/9662)

- [cli] Fix panic in `pulumi console` when no stack is selected.
  [#9594](https://github.com/pulumi/pulumi/pull/9594)

- [cli] Engine now correctly tracks that resource reads have unique URNs.
  [#9516](https://github.com/pulumi/pulumi/pull/9516)

- [sdk/python] Fixed bug in automation API that invoked Pulumi with malformed arguments.
  [#9607](https://github.com/pulumi/pulumi/pull/9607)

- [cli/backend] Fix a panic in the filestate backend when renaming history files.
  [#9673](https://github.com/pulumi/pulumi/pull/9673)

- [sdk/python] Pin protobuf library to <4.
  [#9696](https://github.com/pulumi/pulumi/pull/9696)

- [sdk/proto] Inline dockerfile used to generate protobuf code.
  [#9700](https://github.com/pulumi/pulumi/pull/9700)

## 3.33.1 (2022-05-19)

### Improvements

- [cli] Add `--stack` to `pulumi about`.
  [#9518](https://github.com/pulumi/pulumi/pull/9518)

- [sdk/dotnet] Bumped several dependency versions to avoid pulling packages with known vulnerabilities.
  [#9591](https://github.com/pulumi/pulumi/pull/9591)

- [cli] Updated gocloud.dev to 0.24.0, which adds support for using AWS SDK v2. It enables users to pass an AWS profile to the `awskms` secrets provider url (i.e. `awskms://alias/pulumi?awssdk=v2&region=eu-west-1&profile=aws-prod`)
  [#9590](https://github.com/pulumi/pulumi/pull/9590)

- [sdk/nodejs] Lazy load V8 inspector session to remove module import side-effect [#9375](https://github.com/pulumi/pulumi/issues/9375)

### Bug Fixes

- [cli] The PULUMI_CONFIG_PASSPHRASE environment variables can be empty, this is treated different to being unset.
  [#9568](https://github.com/pulumi/pulumi/pull/9568)

- [codegen/python] Fix importing of enum types from other packages.
  [#9579](https://github.com/pulumi/pulumi/pull/9579)

- [cli] Fix panic in `pulumi console` when no stack is selected.
  [#9594](https://github.com/pulumi/pulumi/pull/9594)


- [auto/python] - Fix text color argument being ignored during stack     operations.
  [#9615](https://github.com/pulumi/pulumi/pull/9615)


## 3.33.0 (2022-05-19)

Replaced by 3.33.1 during release process.

## 3.32.1 (2022-05-05)

### Improvements

- [cli/plugins] The engine will try to lookup the latest version of plugins if the program doesn't specify a version to use.
  [#9537](https://github.com/pulumi/pulumi/pull/9537)
### Bug Fixes

- [cli] Fix an issue using PULUMI_CONFIG_PASSPHRASE_FILE.
  [#9540](https://github.com/pulumi/pulumi/pull/9540)

- [cli/display] Avoid an assert in the table display logic.
  [#9543](https://github.com/pulumi/pulumi/pull/9543)

## 3.32.0 (2022-05-04)

### Improvements

- Pulumi Java support
- Pulumi YAML support

## 3.31.1 (2022-05-03)

### Improvements

- [dotnet] No longer roundtrips requests for the stack URN via the engine.
  [#9515](https://github.com/pulumi/pulumi/pull/9515)

### Bug Fixes

- [codegen/go] Enable obtaining resource outputs off a ResourceOutput.
  [#9513](https://github.com/pulumi/pulumi/pull/9513)

- [codegen/go] Ensure that "plain" generates shallowly plain types.
  [#9512](https://github.com/pulumi/pulumi/pull/9512)

- [codegen/nodejs] Fix enum naming when the enum name starts with `_`.
  [#9453](https://github.com/pulumi/pulumi/pull/9453)

- [cli] Empty passphrases environment variables are now treated as if the variable was not set.
  [#9490](https://github.com/pulumi/pulumi/pull/9490)

- [sdk/go] Fix awaits for outputs containing resources.
  [#9106](https://github.com/pulumi/pulumi/pull/9106)

- [cli] Decode YAML mappings with numeric keys during diff.
  [#9502](https://github.com/pulumi/pulumi/pull/9503)

- [cli] Fix an issue with explicit and default organization names in `pulumi new`
  [#9514](https://github.com/pulumi/pulumi/pull/9514)

## 3.31.0 (2022-04-29)

### Improvements

- [auto/*] Add `--save-plan` and `--plan` options to automation API.
  [#9391](https://github.com/pulumi/pulumi/pull/9391)

- [cli] "down" is now treated as an alias of "destroy".
  [#9458](https://github.com/pulumi/pulumi/pull/9458)

- [go] Add `Composite` resource option allowing several options to be encapsulated into a "single" option.
  [#9459](https://github.com/pulumi/pulumi/pull/9459)

- [codegen] Support all [Asset and Archive](https://www.pulumi.com/docs/intro/concepts/assets-archives/) types.
  [#9463](https://github.com/pulumi/pulumi/pull/9463)

- [cli] Display JSON/YAML property values as objects for creates, sames, and deletes.
  [#9484](https://github.com/pulumi/pulumi/pull/9484)

### Bug Fixes

- [codegen/go] Ensure that plain properties are plain.
  [#9430](https://github.com/pulumi/pulumi/pull/9430)
  [#9488](https://github.com/pulumi/pulumi/pull/9488)

- [cli] Fixed some context leaks where shutdown code wasn't correctly called.
  [#9438](https://github.com/pulumi/pulumi/pull/9438)

- [cli] Do not render array diffs for unchanged elements without recorded values.
  [#9448](https://github.com/pulumi/pulumi/pull/9448)

- [auto/go] Fixed the exit code reported by `runPulumiCommandSync` to be zero if the command runs successfully. Previously it returned -2 which could lead to confusing messages if the exit code was used for other errors, such as in `Stack.Preview`.
  [#9443](https://github.com/pulumi/pulumi/pull/9443)

- [auto/go] Fixed a race condition that could cause `Preview` to fail with "failed to get preview summary".
  [#9467](https://github.com/pulumi/pulumi/pull/9467)

- [backend/filestate] Fix a bug creating `stack.json.bak` files.
  [#9476](https://github.com/pulumi/pulumi/pull/9476)

## 3.30.0 (2022-04-20)

### Improvements

- [cli] Split invoke request protobufs, as monitors and providers take different arguments.
  [#9323](https://github.com/pulumi/pulumi/pull/9323)

- [providers] - gRPC providers can now support an Attach method for debugging. The engine will attach to providers listed in the PULUMI_DEBUG_PROVIDERS environment variable. This should be of the form "providerName:port,otherProvider:port".
  [#8979](https://github.com/pulumi/pulumi/pull/8979)

### Bug Fixes

- [cli/plugin] - Dynamic provider binaries will now be found even if pulumi/bin is not on $PATH.
  [#9396](https://github.com/pulumi/pulumi/pull/9396)

- [sdk/go] - Fail appropriatly for `config.Try*` and `config.Require*` where the
  key is present but of the wrong type.
  [#9407](https://github.com/pulumi/pulumi/pull/9407)

## 3.29.1 (2022-04-13)

### Improvements

- [cli] - Installing of language specific project dependencies is now managed by the language plugins, not the pulumi cli.
  [#9294](https://github.com/pulumi/pulumi/pull/9294)

- [cli] Warn users when there are pending operations but proceed with deployment
  [#9293](https://github.com/pulumi/pulumi/pull/9293)

- [cli] Display more useful diffs for secrets that are not primitive values
  [#9351](https://github.com/pulumi/pulumi/pull/9351)

- [cli] - Warn when `additionalSecretOutputs` is used to mark the `id` property as secret.
  [#9360](https://github.com/pulumi/pulumi/pull/9360)

- [cli] Display richer diffs for texutal property values.
  [#9376](https://github.com/pulumi/pulumi/pull/9376)

- [cli] Display richer diffs for JSON/YAML property values.
  [#9380](https://github.com/pulumi/pulumi/pull/9380)

### Bug Fixes

- [codegen/node] - Fix an issue with escaping deprecation messages.
  [#9371](https://github.com/pulumi/pulumi/pull/9371)

- [cli] - StackReferences will now correctly use the service bulk decryption end point.
  [#9373](https://github.com/pulumi/pulumi/pull/9373)

## 3.28.0 (2022-04-01)

### Improvements

- When a resource is aliased to an existing resource with a different URN, only store
  the alias of the existing resource in the statefile rather than storing all possible
  aliases.
  [#9288](https://github.com/pulumi/pulumi/pull/9288)

- Clear pending operations during `pulumi refresh` or `pulumi up -r`.
  [#8435](https://github.com/pulumi/pulumi/pull/8435)

- [cli] - `pulumi whoami --verbose` and `pulumi about` include a list of the current users organizations.
  [#9211](https://github.com/pulumi/pulumi/pull/9211)

### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)

- [cli/engine] - Fix a panic due to `Check` returning nil while using update plans.
  [#9304](https://github.com/pulumi/pulumi/pull/9304)


## 3.27.0 (2022-03-24)

### Improvements

- [cli] - Implement `pulumi stack unselect`.
  [#9179](https://github.com/pulumi/pulumi/pull/9179)

- [language/dotnet] - Updated Pulumi dotnet packages to use grpc-dotnet instead of grpc.
  [#9149](https://github.com/pulumi/pulumi/pull/9149)

- [cli/config] - Rename the `config` property in `Pulumi.yaml` to `stackConfigDir`. The `config` key will continue to be supported.
  [#9145](https://github.com/pulumi/pulumi/pull/9145)

- [cli/plugins] Add support for downloading plugin from private Pulumi GitHub releases. This also drops the use of the `GITHUB_ACTOR` and `GITHUB_PERSONAL_ACCESS_TOKEN` environment variables for authenticating to github. Only `GITHUB_TOKEN` is now used, but this can be set to a personal access token.
  [#9185](https://github.com/pulumi/pulumi/pull/9185)

- [cli] - Speed up `pulumi stack --show-name` by skipping unneeded snapshot loading.
  [#9199](https://github.com/pulumi/pulumi/pull/9199)

- [cli/plugins] - Improved error message for missing plugins.
  [#5208](https://github.com/pulumi/pulumi/pull/5208)

- [sdk/nodejs] - Take engines property into account when engine-strict appear in npmrc file
  [#9249](https://github.com/pulumi/pulumi/pull/9249)

### Bug Fixes

- [sdk/nodejs] - Fix uncaught error "ENOENT: no such file or directory" when an error occurs during the stack up.
  [#9065](https://github.com/pulumi/pulumi/issues/9065)

- [sdk/nodejs] - Fix uncaught error "ENOENT: no such file or directory" when an error occurs during the stack preview.
  [#9272](https://github.com/pulumi/pulumi/issues/9272)

- [sdk/go] - Fix a panic in `pulumi.All` when using pointer inputs.
  [#9197](https://github.com/pulumi/pulumi/issues/9197)

- [cli/engine] - Fix a panic due to passing `""` as the ID for a resource read.
  [#9243](https://github.com/pulumi/pulumi/pull/9243)

- [cli/engine] - Fix a panic due to `Check` failing while using update plans.
  [#9254](https://github.com/pulumi/pulumi/pull/9254)

- [cli] - Stack names correctly take `org set-default` into account when printing.
  [#9240](https://github.com/pulumi/pulumi/pull/9240)


## 3.26.1 (2022-03-09)

### Improvements

### Bug Fixes

- [cli/new] Fix an error message when the project name picked by default was already used.
  [#9156](https://github.com/pulumi/pulumi/pull/9156)

## 3.26.0 (2022-03-09)

### Improvements

- [area/cli] - Implemented `state rename` command.
  [#9098](https://github.com/pulumi/pulumi/pull/9098)

- [cli/plugins] `pulumi plugin install` can now look up the latest version of plugins on GitHub releases.
  [#9012](https://github.com/pulumi/pulumi/pull/9012)

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

- [cli/import] - The import command no longer errors if resource properties do not validate. Instead the
  engine warns about property issues returned by the provider but then continues with the import and codegen
  as best it can. This should result in more resources being imported to the pulumi state and being able to
  generate some code, at the cost that the generated code may not work as is in an update. Users will have to
  edit the code to successfully run.
  [#8922](https://github.com/pulumi/pulumi/pull/8922)

- [cli/import] - Code generation in `pulumi import` can now be disabled with the `--generate-code=false` flag.
  [#9141](https://github.com/pulumi/pulumi/pull/9141)

### Bug Fixes

- [sdk/python] - Fix build warnings. See
  [#9011](https://github.com/pulumi/pulumi/issues/9011) for more details.
  [#9139](https://github.com/pulumi/pulumi/pull/9139)

- [cli/backend] - Fixed an issue with non-atomicity when saving file state stacks.
  [#9122](https://github.com/pulumi/pulumi/pull/9122)

- [sdk/go] - Fixed an issue where the RetainOnDelete resource option is not applied.
  [#9147](https://github.com/pulumi/pulumi/pull/9147)

## 3.25.1 (2022-03-2)

### Improvements

### Bug Fixes

- [sdk/nodejs] - Fix Node `fs.rmdir` DeprecationWarning for Node JS 15.X+
  [#9044](https://github.com/pulumi/pulumi/pull/9044)

- [engine] - Fix deny default provider handling for Invokes and Reads.
  [#9067](https://github.com/pulumi/pulumi/pull/9067)

- [codegen/go] - Fix secret codegen for input properties
  [#9052](https://github.com/pulumi/pulumi/pull/9052)

- [sdk/nodejs] - `PULUMI_NODEJS_TSCONFIG_PATH` is now explicitly passed to tsnode for the tsconfig file.
  [#9062](https://github.com/pulumi/pulumi/pull/9062)

## 3.25.0 (2022-02-23)

### Improvements

- [codegen/go] - Add GenerateProgramWithOpts function to enable configurable codegen options.
  [#8997](https://github.com/pulumi/pulumi/pull/8997)

- [cli] -  Enabled dot spinner for non-interactive mode
  [#8996](https://github.com/pulumi/pulumi/pull/8996)

- [sdk] - Add `RetainOnDelete` as a resource option.
  [#8746](https://github.com/pulumi/pulumi/pull/8746)

- [cli] - Adding `completion` as an alias to `gen-completion`
  [#9006](https://github.com/pulumi/pulumi/pull/9006)

- [cli/plugins] Add support for downloading plugin from private GitHub releases.
  [#8944](https://github.com/pulumi/pulumi/pull/8944)

### Bug Fixes

- [sdk/go] - Normalize merge behavior for `ResourceOptions`, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8882](https://github.com/pulumi/pulumi/pull/8882)

- [sdk/go] - Correctly parse GoLang version.
  [#8920](https://github.com/pulumi/pulumi/pull/8920)

- [sdk/go] - Fix git initialization in git_test.go
  [#8924](https://github.com/pulumi/pulumi/pull/8924)

- [cli/go] - Fix git initialization in util_test.go
  [#8924](https://github.com/pulumi/pulumi/pull/8924)

- [sdk/nodejs] - Fix nodejs function serialization module path to comply with package.json
  exports if exports is specified.
  [#8893](https://github.com/pulumi/pulumi/pull/8893)

- [cli/python] - Parse a larger subset of PEP440 when guessing Pulumi package versions.
  [#8958](https://github.com/pulumi/pulumi/pull/8958)

- [sdk/nodejs] - Allow disabling TypeScript typechecking
  [#8981](https://github.com/pulumi/pulumi/pull/8981)

- [cli/backend] - Revert a change to file state locking that was causing stacks to stay locked.
  [#8995](https://github.com/pulumi/pulumi/pull/8995)

- [cli] - Fix passphrase secrets provider prompting.
  [#8986](https://github.com/pulumi/pulumi/pull/8986)

- [cli] - Fix an assert when replacing protected resources.
  [#9004](https://github.com/pulumi/pulumi/pull/9004)

## 3.24.1 (2022-02-4)

### Bug Fixes

- [release] - Update .gitignore to allow for a clean git repository for release.
  [#8932](https://github.com/pulumi/pulumi/pull/8932)

## 3.24.0 (2022-02-4)

### Improvements

- [codegen/go] - Implement go type conversions for optional string, boolean, int, and float32
  arguments, and changes our behavior for optional spilling from variable declaration hoisting to
  instead rewrite as calls to these functions. Fixes #8821.
  [#8839](https://github.com/pulumi/pulumi/pull/8839)

- [sdk/go] Added new conversion functions for Read methods on resources exported at the top level
  of the Pulumi sdk. They are `StringRef`, `BoolRef`, `IntRef`, and `Float64Ref`. They are used for
  creating a pointer to the type they name, e.g.: StringRef takes `string` and returns `*string`.
  Data source methods which take optional strings, bools, ints, and float64 values can be set to
  the return value of these functions. These functions will appear in generated programs as well as
  future docs updates.

- [sdk/nodejs] - Fix resource plugins advertising a `pluginDownloadURL` not being downloaded. This
  should allow resource plugins published via boilerplates to find and consume plugins published
  outside the registry. See: https://github.com/pulumi/pulumi/issues/8890 for the tracking issue to
  document this feature.

- [cli] Experimental support for update plans. Only enabled when PULUMI_EXPERIMENTAL is
  set. This enables preview to save a plan of what the engine expects to happen in a file
  with --save-plan. That plan can then be read in by up with --plan and is used to ensure
  only the expected operations happen.
  [#8448](https://github.com/pulumi/pulumi/pull/8448)

- [codegen] - Add language option to make codegen respect the `Version` field in
  the Pulumi package schema.
  [#8881](https://github.com/pulumi/pulumi/pull/8881)

- [cli] - Support wildcards for `pulumi up --target <urn>` and similar commands.
  [#8883](https://github.com/pulumi/pulumi/pull/8883).

- [cli/import] - The import command now takes an extra argument --properties to instruct the engine which
  properties to use for the import. This can be used to import resources which the engine couldn't automaticly
  infer the correct property set for.
  [#8846](https://github.com/pulumi/pulumi/pull/8846)

- [cli] Ensure defaultOrg is used as part of any stack name
  [#8903](https://github.com/pulumi/pulumi/pull/8903)

### Bug Fixes

- [codegen] - Correctly handle third party resource imports.
  [#8861](https://github.com/pulumi/pulumi/pull/8861)

- [sdk/dotnet] - Normalize merge behavior for ComponentResourceOptions, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8838](https://github.com/pulumi/pulumi/pull/8838)

- [codegen/nodejs] - Respect compat modes when referencing external types.
  [#8850](https://github.com/pulumi/pulumi/pull/8850)

- [cli] The engine will allow a resource to be replaced if either it's old or new state
  (or both) is not protected.
  [#8873](https://github.com/pulumi/pulumi/pull/8873)

- [cli] - Fixed CLI duplicating prompt question.
  [#8858](https://github.com/pulumi/pulumi/pull/8858)

- [cli] - `pulumi plugin install --reinstall` now always reinstalls plugins.
  [#8892](https://github.com/pulumi/pulumi/pull/8892)

- [codegen/go] - Honor import aliases for external types/resources.
  [#8833](https://github.com/pulumi/pulumi/pull/8833)

- [codegen/python] - Correctly reference external types/resources with same module name.
  [#8910](https://github.com/pulumi/pulumi/pull/8910)

- [sdk/nodejs] - Correctly pickup provider as a member of providers.
  [#8923](https://github.com/pulumi/pulumi/pull/8923)

## 3.23.2 (2022-01-28)

## Bug Fixes

- [sdk/{nodejs,python}] - Remove sequence numbers from the dynamic provider interfaces.
  [#8849](https://github.com/pulumi/pulumi/pull/8849)

## 3.23.0 (2022-01-26)

### Improvements

- [codegen/dotnet] - Add C# extension `rootNamespace`, allowing the user to
  replace `Pulumi` as the default C# global namespace in generated programs.
  The `Company` and `Author` fields of the .csproj file are now driven by
  `schema.publisher`.
  [#8735](https://github.com/pulumi/pulumi/pull/8735)

- [cli] Download provider plugins from GitHub Releases
  [#8785](https://github.com/pulumi/pulumi/pull/8785)

- [cli] Using a decryptAll functionality when deserializing a deployment. This will allow
  decryption of secrets stored in the Pulumi Service backend to happen in bulk for
  performance increase
  [#8676](https://github.com/pulumi/pulumi/pull/8676)

- [sdk/dotnet] - Changed `Output<T>.ToString()` to return an informative message rather than just "Output`1[X]"
  [#8767](https://github.com/pulumi/pulumi/pull/8767)

- [cli] Add the concept of sequence numbers to the engine and resource provider interface.
  [#8631](https://github.com/pulumi/pulumi/pull/8631)

- [common] Allow names with hyphens.

- [cli] - Add support for overriding plugin download URLs.
  [#8798](https://github.com/pulumi/pulumi/pull/8798)

- [automation] - Add `color` option to stack up, preview, refresh, and destroy commands.
  [#8811](https://github.com/pulumi/pulumi/pull/8811)

- [sdk/nodejs] - Support top-level default exports in ESM.
  [#8766](https://github.com/pulumi/pulumi/pull/8766)

- [cli] - Allow disabling default providers via the Pulumi config.
  [#8829](https://github.com/pulumi/pulumi/pull/8829)

- [cli] Add better error message for pulumi service rate limit responses
  [#7963](https://github.com/pulumi/pulumi/issues/7963)

### Bug Fixes

- [sdk/{python,nodejs}] - Prevent `ResourceOptions.merge` from promoting between the
  `.provider` and `.providers` fields. This changes the general behavior of merging
  for `.provider` and `.providers`, as described in [#8796](https://github.com/pulumi/pulumi/issues/8796).
  Note that this is a breaking change in two ways:
    1. Passing a provider to a custom resource of the wrong package now
       produces a `ValueError`. In the past it would send to the provider, and
       generally crash the provider.
    2. Merging two `ResourceOptions` with `provider` set no longer hoists to `providers`.
       One `provider` will now take priority over the other. The new behavior reflects the
       common case for `ResourceOptions.merge`. To restore the old behavior, replace
       `ResourceOptions(provider=FooProvider).merge(ResourceOptions(provider=BarProvider))`
       with `ResourceOptions(providers=[FooProvider]).merge(ResourceOptions(providers=[BarProvider]))`.
  [#8770](https://github.com/pulumi/pulumi/pull/8770)

- [codegen/nodejs] - Generate an install script that runs `pulumi plugin install` with
  the `--server` flag when necessary.
  [#8730](https://github.com/pulumi/pulumi/pull/8730)

- [cli] The engine will no longer try to replace resources that are protected as that entails a delete.
  [#8810](https://github.com/pulumi/pulumi/pull/8810)

- [codegen/pcl] - Fix handling of function invokes without args
  [#8805](https://github.com/pulumi/pulumi/pull/8805)

## 3.22.1 (2022-01-14)

### Improvements

- [sdk/dotnet] - Add `PluginDownloadURL` as a resource option. When provided by
  the schema, `PluginDownloadURL` will be baked into `new Resource` and `Invoke`
  requests in generated SDKs.
  [#8739](https://github.com/pulumi/pulumi/pull/8739)

- [sdk] - Allow property paths to accept `[*]` as sugar for `["*"]`.
  [#8743](https://github.com/pulumi/pulumi/pull/8743)

- [sdk/dotnet] Add `Union.Bimap` function for converting both sides of a union at once.
  [#8733](https://github.com/pulumi/pulumi/pull/8733)

### Bug Fixes

- [sdk/dotnet] Allow `Output<Union>` to be converted to `InputUnion`.
  [#8733](https://github.com/pulumi/pulumi/pull/8733)

- [cli/config] - Revert number handling in `pulumi config`.
  [#8754](https://github.com/pulumi/pulumi/pull/8754)

## 3.22.0 (2022-01-12)

### Improvements

- [sdk/{nodejs,go,python}] - Add `PluginDownloadURL` as a resource option. When provided by
  the schema, `PluginDownloadURL` will be baked into `new Resource` and `Invoke`
  requests in generated SDKs.
  [#8698](https://github.com/pulumi/pulumi/pull/8698)
  [#8690](https://github.com/pulumi/pulumi/pull/8690)
  [#8692](https://github.com/pulumi/pulumi/pull/8692)

### Bug Fixes

- [auto/python] - Fixes an issue with exception isolation in a
  sequence of inline programs that caused all inline programs to fail
  after the first one failed
  [#8693](https://github.com/pulumi/pulumi/pull/8693)


## 3.21.1 (2022-01-07)

### Improvements

- [sdk/go] - Add `PluginDownloadURL` as a resource option.
  [#8555](https://github.com/pulumi/pulumi/pull/8555)

- [sdk/go] - Allow users to override enviromental variables for `GetCommandResults`.
  [#8610](https://github.com/pulumi/pulumi/pull/8610)

- [sdk/nodejs] Support using native ES modules as Pulumi scripts
  [#7764](https://github.com/pulumi/pulumi/pull/7764)

- [sdk/nodejs] Support a `nodeargs` option for passing `node` arguments to the Node language host
  [#8655](https://github.com/pulumi/pulumi/pull/8655)

### Bug Fixes

- [cli/engine] - Fix [#3982](https://github.com/pulumi/pulumi/issues/3982), a bug
  where the engine ignored the final line of stdout/stderr if it didn't terminate
  with a newline.
  [#8671](https://github.com/pulumi/pulumi/pull/8671)

- [nodejs/sdk] - GetRequiredPlugins: Return plugins even when there're errors.
  [#8699](https://github.com/pulumi/pulumi/pull/8699)


## 3.21.0 (2021-12-29)

### Improvements

- [engine] - Interpret `pluginDownloadURL` as the provider host url when
  downloading plugins.
  [#8544](https://github.com/pulumi/pulumi/pull/8544)

- [sdk/dotnet] - `InputMap` and `InputList` can now be initialized
  with any value that implicitly converts to the collection type.
  These values are then automatically appended, for example:

        var list = new InputList<string>
        {
            "V1",
            Output.Create("V2"),
            new[] { "V3", "V4" },
            new List<string> { "V5", "V6" },
            Output.Create(ImmutableArray.Create("V7", "V8"))
        };

  This feature simplifies the syntax for constructing resources and
  specifying resource options such as the `DependsOn` option.

  [#8498](https://github.com/pulumi/pulumi/pull/8498)

### Bug Fixes

- [sdk/python] - Fixes an issue with stack outputs persisting after
  they are removed from the Pulumi program
  [#8583](https://github.com/pulumi/pulumi/pull/8583)

- [auto/*] - Fixes `stack.setConfig()` breaking when trying to set
  values that look like flags (such as `-value`)
  [#8518](https://github.com/pulumi/pulumi/pull/8614)

- [sdk/dotnet] - Don't throw converting value types that don't match schema
  [#8628](https://github.com/pulumi/pulumi/pull/8628)

- [sdk/{go,nodejs,dotnet,python}] - Compute full set of aliases when both parent and child are aliased.
  [#8627](https://github.com/pulumi/pulumi/pull/8627)

- [cli/import] - Fix import of resource with non-identifier map keys
  [#8645](https://github.com/pulumi/pulumi/pull/8645)

- [backend/filestate] - Allow preview on locked stack
  [#8642](https://github.com/pulumi/pulumi/pull/8642)

## 3.20.0 (2021-12-16)

### Improvements

- [codegen/go] - Do not generate unreferenced input types by default.
  [#7943](https://github.com/pulumi/pulumi/pull/7943)

- [codegen/go] - Simplify the application of object defaults in generated SDKs.
  [#8539](https://github.com/pulumi/pulumi/pull/8539)

- [codegen/{python,dotnet}] - Emit `pulumi-plugin.json` unconditionally.
  [#8527](https://github.com/pulumi/pulumi/pull/8527)
  [#8532](https://github.com/pulumi/pulumi/pull/8532)

- [sdk/python] - Lookup Pulumi packages by searching for `pulumi-plugin.json`.
  Pulumi packages need not be prefixed by `pulumi-` anymore.
  [#8515](https://github.com/pulumi/pulumi/pull/8515)

- [sdk/go] - Lookup packages by searching for `pulumi-plugin.json`.
  Pulumi packages need not be prefixed by `github.com/pulumi/pulumi-` anymore.
  [#8516](https://github.com/pulumi/pulumi/pull/8516)

- [sdk/dotnet] - Lookup packages by searching for `pulumi-plugin.json`.
  Pulumi packages need not be prefixed by `Pulumi.` anymore.
  [#8517](https://github.com/pulumi/pulumi/pull/8517)

- [sdk/go] - Emit `pulumi-plugin.json`
  [#8530](https://github.com/pulumi/pulumi/pull/8530)

- [cli] - Always use locking in filestate backends. This feature was
  previously disabled by default and activated by setting the
  `PULUMI_SELF_MANAGED_STATE_LOCKING=1` environment variable.
  [#8565](https://github.com/pulumi/pulumi/pull/8565)

- [{cli,auto}] - Exclude language plugins from `PULUMI_IGNORE_AMBIENT_PLUGINS`.
  [#8576](https://github.com/pulumi/pulumi/pull/8576)

- [sdk/dotnet] - Fixes a rare race condition that sporadically caused
  NullReferenceException to be raised when constructing resources
  [#8495](https://github.com/pulumi/pulumi/pull/8495)

- [cli] Log secret decryption events when a project uses the Pulumi Service and a 3rd party secrets provider
  [#8563](https://github.com/pulumi/pulumi/pull/8563)

- [schema] Do not validate against the metaschema in ImportSpec. Clients that need to
  validate input schemas should use the BindSpec API instead.
  [#8543](https://github.com/pulumi/pulumi/pull/8543)

### Bug Fixes

- [codegen/schema] - Error on type token names that are not allowed (schema.Name
  or specified in allowedPackageNames).
  [#8538](https://github.com/pulumi/pulumi/pull/8538)
  [#8558](https://github.com/pulumi/pulumi/pull/8558)

- [codegen/go] - Fix `ElementType` for nested collection input and output types.
  [#8535](https://github.com/pulumi/pulumi/pull/8535)

- [{codegen,sdk}/{python,dotnet,go}] - Use `pulumi-plugin.json` rather than `pulumiplugin.json`.
  [#8593](https://github.com/pulumi/pulumi/pull/8593)

## 3.19.0 (2021-12-01)

### Improvements

- [codegen/go] - Remove `ResourcePtr` types from generated SDKs. Besides being
  unnecessary--`Resource` types already accommodate `nil` to indicate the lack of
  a value--the implementation of `Ptr` types for resources was incorrect, making
  these types virtually unusable in practice.
  [#8449](https://github.com/pulumi/pulumi/pull/8449)

- [cli] - Allow interpolating plugin custom server URLs.
  [#8507](https://github.com/pulumi/pulumi/pull/8507)

### Bug Fixes

- [cli/engine] - Accurately computes the fields changed when diffing with unhelpful providers. This
  allows the `replaceOnChanges` feature to be respected for all providers.
  [#8488](https://github.com/pulumi/pulumi/pull/8488)

- [codegen/go] - Respect default values in Pulumi object types.
  [#8411](https://github.com/pulumi/pulumi/pull/8400)

## 3.18.1 (2021-11-22)

### Improvements

- [cli] - When running `pulumi new https://github.com/name/repo`, check
  for branch `main` if branch `master` doesn't exist.
  [#8463](https://github.com/pulumi/pulumi/pull/8463)

- [codegen/python] - Program generator now uses `fn_output` forms where
  appropriate, simplifying auto-generated examples.
  [#8433](https://github.com/pulumi/pulumi/pull/8433)

- [codegen/go] - Program generator now uses fnOutput forms where
  appropriate, simplifying auto-generated examples.
  [#8431](https://github.com/pulumi/pulumi/pull/8431)

- [codegen/dotnet] - Program generator now uses `Invoke` forms where
  appropriate, simplifying auto-generated examples.
  [#8432](https://github.com/pulumi/pulumi/pull/8432)

### Bug Fixes

- [cli/nodejs] - Allow specifying the tsconfig file used in Pulumi.yaml.
  [#8452](https://github.com/pulumi/pulumi/pull/8452)

- [codegen/nodejs] - Respect default values in Pulumi object types.
  [#8400](https://github.com/pulumi/pulumi/pull/8400)

- [sdk/python] - Correctly handle version checking python virtual environments.
  [#8465](https://github.com/pulumi/pulumi/pull/8465)

- [cli] - Catch expected errors in stacks with filestate backends.
  [#8455](https://github.com/pulumi/pulumi/pull/8455)

- [sdk/dotnet] - Do not attempt to serialize unknown values.
  [#8475](https://github.com/pulumi/pulumi/pull/8475)

## 3.18.0 (2021-11-17)

### Improvements
- [ci] - Adds CI detector for Buildkite
  [#7933](https://github.com/pulumi/pulumi/pull/7933)

- [cli] - Add `--exclude-protected` flag to `pulumi destroy`.
  [#8359](https://github.com/pulumi/pulumi/pull/8359)

- [cli] Add the ability to use `pulumi org set [name]` to set a default org
  to use when creating a stacks in the Pulumi Service backend or self-hosted Service.
  [#8352](https://github.com/pulumi/pulumi/pull/8352)

- [schema] Add IsOverlay option to disable codegen for particular types.
  [#8338](https://github.com/pulumi/pulumi/pull/8338)
  [#8425](https://github.com/pulumi/pulumi/pull/8425)

- [sdk/dotnet] - Marshal output values.
  [#8316](https://github.com/pulumi/pulumi/pull/8316)

- [sdk/python] - Unmarshal output values in component provider.
  [#8212](https://github.com/pulumi/pulumi/pull/8212)

- [sdk/nodejs] - Unmarshal output values in component provider.
  [#8205](https://github.com/pulumi/pulumi/pull/8205)

- [sdk/nodejs] - Allow returning failures from Call in the provider without setting result outputs.
  [#8424](https://github.com/pulumi/pulumi/pull/8424)

- [sdk/go] - Allow specifying Call failures from the provider.
  [#8424](https://github.com/pulumi/pulumi/pull/8424)

- [codegen/nodejs] - Program generator now uses `fnOutput` forms where
  appropriate, simplifying auto-generated examples.
  [#8434](https://github.com/pulumi/pulumi/pull/8434)

### Bug Fixes

- [engine] - Compute dependents correctly during targeted deletes.
  [#8360](https://github.com/pulumi/pulumi/pull/8360)

- [cli/engine] - Update command respects `--target-dependents`.
  [#8395](https://github.com/pulumi/pulumi/pull/8395)

- [docs] - Fix broken lists in dotnet docs.
  [docs#6558](https://github.com/pulumi/docs/issues/6558)

## 3.17.1 (2021-11-09)

### Improvements

- [codegen/docs] Edit docs codegen to document `$fnOutput` function
  invoke forms in API documentation.
  [#8287](https://github.com/pulumi/pulumi/pull/8287)

### Bug Fixes

- [automation/python] - Fix deserialization of events.
  [#8375](https://github.com/pulumi/pulumi/pull/8375)

- [sdk/dotnet] - Fixes failing preview for programs that call data
  sources (`F.Invoke`) with unknown outputs.
  [#8339](https://github.com/pulumi/pulumi/pull/8339)

- [programgen/go] - Don't change imported resource names.
  [#8353](https://github.com/pulumi/pulumi/pull/8353)


## 3.17.0 (2021-11-03)

### Improvements

- [cli] - Reformat error message string in `sdk/go/common/diag/errors.go`.
  [#8284](https://github.com/pulumi/pulumi/pull/8284)

- [cli] - Add `--json` flag to `up`, `destroy` and `refresh`.

  Passing the `--json` flag to `up`, `destroy` and `refresh` will stream JSON events from the engine to stdout.
  For `preview`, the existing functionality of outputting a JSON object at the end of preview is maintained.
  However, the streaming output can be extended to `preview` by using the `PULUMI_ENABLE_STREAMING_JSON_PREVIEW` environment variable.

  [#8275](https://github.com/pulumi/pulumi/pull/8275)

### Bug Fixes

- [sdk/go] - Respect implicit parents in alias resolution.
  [#8288](https://github.com/pulumi/pulumi/pull/8288)

- [sdk/python] - Expand dependencies when marshaling output values.
  [#8301](https://github.com/pulumi/pulumi/pull/8301)

- [codegen/go] - Interaction between the `plain` and `default` tags of a type.
  [#8254](https://github.com/pulumi/pulumi/pull/8254)

- [sdk/dotnet] - Fix a race condition when detecting exceptions in stack creation.
  [#8294](https://github.com/pulumi/pulumi/pull/8294)

- [sdk/go] - Fix regression marshaling assets/archives.
  [#8290](https://github.com/pulumi/pulumi/pull/8290)

- [sdk/dotnet] - Don't panic on schema mismatches.
  [#8286](https://github.com/pulumi/pulumi/pull/8286)

- [codegen/python] - Fixes issue with `$fn_output` functions failing in
  preview when called with unknown arguments.
  [#8320](https://github.com/pulumi/pulumi/pull/8320)


## 3.16.0 (2021-10-20)

### Improvements

- [codegen/dotnet] - Add helper function forms `$fn.Invoke` that
  accept `Input`s, return an `Output`, and wrap the underlying
  `$fn.InvokeAsync` call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for .NET, making
  it easier to compose functions/datasources with Pulumi resources.
  NOTE for resource providers: the generated code requires Pulumi .NET
  SDK 3.15 or higher.

  [#7899](https://github.com/pulumi/pulumi/pull/7899)

- [auto/dotnet] - Add `pulumi state delete` and `pulumi state unprotect` functionality
  [#8202](https://github.com/pulumi/pulumi/pull/8202)


## 3.15.0 (2021-10-14)

### Improvements

- [automation/python] - Use `rstrip` rather than `strip` for the sake of indentation
  [#8160](https://github.com/pulumi/pulumi/pull/8160)

- [codegen/nodejs] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/5758) for NodeJS,
  making it easier to compose functions/datasources with Pulumi
  resources.
  [#8047](https://github.com/pulumi/pulumi/pull/8047)

- [sdk/dotnet] - Update SDK to support the upcoming codegen feature that
  will enable functions to accept Outputs
  ([5758](https://github.com/pulumi/pulumi/issues/5758)). Specifically
  add `Pulumi.DeploymentInstance.Invoke` and remove the now redundant
  `Pulumi.Utilities.CodegenUtilities`.
  [#8142](https://github.com/pulumi/pulumi/pull/8142)

- [cli] - Upgrade CLI to go1.17
  [#8171](https://github.com/pulumi/pulumi/pull/8171)

- [codegen/go] Register input types for schema object types.
  [#7959](https://github.com/pulumi/pulumi/pull/7959)

- [codegen/go] Register input types for schema resource and enum types.
  [#8204]((https://github.com/pulumi/pulumi/pull/8204))

- [codegen/go] Add schema flag to disable registering input types.
  [#8198](https://github.com/pulumi/pulumi/pull/8198)

### Bug Fixes

- [codegen/go] - Use `importBasePath` before `name` if specified for name
  and path.
  [#8159](https://github.com/pulumi/pulumi/pull/8159)
  [#8187](https://github.com/pulumi/pulumi/pull/8187)

- [auto/go] - Mark entire exported map as secret if key in map is secret.
  [#8179](https://github.com/pulumi/pulumi/pull/8179)


## 3.14.0 (2021-10-06)

### Improvements

- [cli] - Differentiate in-progress actions by bolding output.
  [#7918](https://github.com/pulumi/pulumi/pull/7918)

- [CLI] Adding the ability to set `refresh: always` in an options object at a Pulumi.yaml level
  to allow a user to be able to always refresh their derivative stacks by default
  [#8071](https://github.com/pulumi/pulumi/pull/8071)

### Bug Fixes

- [codegen/go] - Fix generation of cyclic struct types.
  [#8049](https://github.com/pulumi/pulumi/pull/8049)

- [codegen/nodejs] - Fix type literal generation by adding
  disambiguating parens; previously nested types such as arrays of
  unions and optionals generated type literals that were incorrectly
  parsed by TypeScript precedence rules.

  NOTE for providers: using updated codegen may result in API changes
  that break existing working programs built against the older
  (incorrect) API declarations.

  [#8116](https://github.com/pulumi/pulumi/pull/8116)

- [auto/go] - Fix --target / --replace args
  [#8109](https://github.com/pulumi/pulumi/pull/8109)

- [sdk/python] - Fix deprecation warning when using python 3.10
  [#8129](https://github.com/pulumi/pulumi/pull/8129)


## 3.13.2 (2021-09-27)

**Please Note:** The v3.13.1 release failed in our build pipeline and was re-released as v3.13.2.

### Improvements

- [CLI] - Enable output values in the engine by default.
  [#8014](https://github.com/pulumi/pulumi/pull/8014)

### Bug Fixes

- [automation/python] - Fix a bug in printing `Stack` if no program is provided.
  [#8032](https://github.com/pulumi/pulumi/pull/8032)

- [codegen/schema] - Revert #7938.
  [#8035](https://github.com/pulumi/pulumi/pull/8035)

- [codegen/nodejs] - Correctly determine imports for functions.
  [#8038](https://github.com/pulumi/pulumi/pull/8038)

- [codegen/go] - Fix resolution of enum naming collisions.
  [#7985](https://github.com/pulumi/pulumi/pull/7985)

- [sdk/{nodejs,python}] - Fix errors when testing remote components with mocks.
  [#8053](https://github.com/pulumi/pulumi/pull/8053)

- [codegen/nodejs] - Fix generation of provider enum with environment variables.
  [#8051](https://github.com/pulumi/pulumi/pull/8051)

## 3.13.0 (2021-09-22)

### Improvements

- [sdk/go] - Improve error messages for (un)marshalling properties.
  [#7936](https://github.com/pulumi/pulumi/pull/7936)

- [sdk/go] - Initial support for (un)marshalling output values.
  [#7861](https://github.com/pulumi/pulumi/pull/7861)

- [sdk/go] - Add `RegisterInputType` and register built-in types.
  [#7928](https://github.com/pulumi/pulumi/pull/7928)

- [codegen] - Packages include `Package.Version` when provided.
  [#7938](https://github.com/pulumi/pulumi/pull/7938)

- [auto/*] - Fix escaped HTML characters from color directives in event stream.

  E.g. `"<{%reset%}>debug: <{%reset%}>"` -> `"<{%reset%}>debug: <{%reset%}>"`
  [#7998](https://github.com/pulumi/pulumi/pull/7998)

- [auto/*] - Allow eliding color directives from event logs by passing `NO_COLOR` env var.

  E.g. `"<{%reset%}>debug: <{%reset%}>"` -> `"debug: "`
  [#7998](https://github.com/pulumi/pulumi/pull/7998)

- [schema] The syntactical well-formedness of a package schema is now described
  and checked by a JSON schema metaschema.
  [#7952](https://github.com/pulumi/pulumi/pull/7952)

### Bug Fixes

- [codegen/schema] - Correct validation for Package
  [#7896](https://github.com/pulumi/pulumi/pull/7896)

- [cli] Use json.Unmarshal instead of custom parser
  [#7954](https://github.com/pulumi/pulumi/pull/7954)

- [sdk/{go,dotnet}] - Thread replaceOnChanges through Go and .NET
  [#7967](https://github.com/pulumi/pulumi/pull/7967)

- [codegen/nodejs] - Correctly handle hyphenated imports
  [#7993](https://github.com/pulumi/pulumi/pull/7993)

## 3.12.0 (2021-09-08)

### Improvements

- [build] - make lint returns an accurate status code
  [#7844](https://github.com/pulumi/pulumi/pull/7844)

- [codegen/python] - Add helper function forms `$fn_output` that
  accept `Input`s, return an `Output`, and wrap the underlying `$fn`
  call. This change addresses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Python,
  making it easier to compose functions/datasources with Pulumi
  resources. [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [codegen] - Add `replaceOnChange` to schema.
  [#7874](https://github.com/pulumi/pulumi/pull/7874)

- [cli/about] - Add command for debug information
  [#7817](https://github.com/pulumi/pulumi/pull/7817)

- [codegen/schema] Add a `pulumi schema check` command to validate package schemas.
  [#7865](https://github.com/pulumi/pulumi/pull/7865)

### Bug Fixes

- [sdk/python] - Fix Pulumi programs hanging when dependency graph
  forms a cycle, as when `eks.NodeGroup` declaring `eks.Cluster` as a
  parent while also depending on it indirectly via properties
  [#7887](https://github.com/pulumi/pulumi/pull/7887)

- [sdk/python] Fix a regression in Python dynamic providers introduced in #7755.

- [automation/go] Fix loading of stack settings/configs from yaml files.
  [#pulumi-kubernetes-operator/183](https://github.com/pulumi/pulumi-kubernetes-operator/issues/183)

- [codegen/python] - Fix invalid Python docstring generation for enums
  that contain doc comments with double quotes
  [#7914](https://github.com/pulumi/pulumi/pull/7914)

## 3.11.0 (2021-08-25)

### Improvements

- [auto/dotnet] - Add support for `--exact` and `--server` with `pulumi plugin install` via Automation API. BREAKING NOTE: If you are subclassing `Workspace` your `InstallPluginAsync` implementation will need to be updated to reflect the new `PluginInstallOptions` parameter.
  [#7762](https://github.com/pulumi/pulumi/pull/7796)

- [codegen/go] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Go, making it
  easier to compose functions/datasources with Pulumi resources.
  [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [sdk/python] - Speed up `pulumi up` on Python projects by optimizing
  `pip` invocations
  [#7819](https://github.com/pulumi/pulumi/pull/7819)

- [sdk/dotnet] - Support for calling methods.
  [#7582](https://github.com/pulumi/pulumi/pull/7582)

### Bug Fixes

- [cli] - Avoid `missing go.sum entry for module` for new Go projects.
  [#7808](https://github.com/pulumi/pulumi/pull/7808)

- [codegen/schema] - Allow hyphen in schema path reference.
  [#7824](https://github.com/pulumi/pulumi/pull/7824)

## 3.10.3 (2021-08-19)

### Improvements

- [sdk/python] - Add support for custom naming of dynamic provider resource.
  [#7633](https://github.com/pulumi/pulumi/pull/7633)

### Bug Fixes

- [codegen/go] - Fix nested collection type generation.
  [#7779](https://github.com/pulumi/pulumi/pull/7779)

- [sdk/dotnet] - Fix an exception when passing an unknown `Output` to
  the `DependsOn` resource option.
  [#7762](https://github.com/pulumi/pulumi/pull/7762)

- [engine] Include transitive children in dependency list for deletes.
  [#7788](https://github.com/pulumi/pulumi/pull/7788)


## 3.10.2 (2021-08-16)

### Improvements

- [cli] Stop printing secret value on `pulumi config set` if it looks like a secret.
  [#7327](https://github.com/pulumi/pulumi/pull/7327)

- [sdk/nodejs] Prevent Pulumi from overriding tsconfig.json options.
  [#7068](https://github.com/pulumi/pulumi/pull/7068)

- [sdk/go] - Permit declaring explicit resource dependencies via
  `ResourceInput` values.
  [#7584](https://github.com/pulumi/pulumi/pull/7584)

### Bug Fixes

- [sdk/go] - Fix marshaling behavior for undefined properties.
  [#7768](https://github.com/pulumi/pulumi/pull/7768)

- [sdk/python] - Fix program hangs when monitor becomes unavailable.
  [#7734](https://github.com/pulumi/pulumi/pull/7734)

- [sdk/python] Allow Python dynamic provider resources to be constructed outside of `__main__`.
  [#7755](https://github.com/pulumi/pulumi/pull/7755)

## 3.10.1 (2021-08-12)

### Improvements

- [sdk/go] - Depending on a component now depends on the transitive closure of its
  child resources.
  [#7732](https://github.com/pulumi/pulumi/pull/7732)

- [sdk/python] - Depending on a component now depends on the transitive closure of its
  child resources.
  [#7732](https://github.com/pulumi/pulumi/pull/7732)

## 3.10.0 (2021-08-11)

### Improvements

- [cli] - Fix the preview experience for unconfigured providers. Rather than returning the
  inputs of a resource managed by an unconfigured provider as its outputs, the engine will treat all outputs as unknown. Most
  programs will not be affected by these changes: in general, the only programs that will
  see differences are programs that:

      1. pass unknown values to provider instances
      2. use these provider instances to manage resources
      3. pass values from these resources to resources that are managed by other providers

  These kinds of programs are most common in scenarios that deploy managed Kubernetes
  clusters and Kubernetes apps within the same program, then flow values from those apps
  into other resources.

  The legacy behavior can be re-enabled by setting the `PULUMI_LEGACY_PROVIDER_PREVIEW` to
  a truthy value (e.g. `1`, `true`, etc.).

  [#7560](https://github.com/pulumi/pulumi/pull/7560)

- [automation] - Add force flag for RemoveStack in workspace
  [#7523](https://github.com/pulumi/pulumi/pull/7523)

### Bug Fixes

- [cli] - Properly parse Git remotes with periods or hyphens.
  [#7386](https://github.com/pulumi/pulumi/pull/7386)

- [codegen/python] - Recover good IDE completion experience over
  module imports that was compromised when introducing the lazy import
  optimization.
  [#7487](https://github.com/pulumi/pulumi/pull/7487)

- [sdk/python] - Use `Sequence[T]` instead of `List[T]` for several `Resource`
  parameters.
  [#7698](https://github.com/pulumi/pulumi/pull/7698)

- [auto/nodejs] - Fix a case where inline programs could exit with outstanding async work.
  [#7704](https://github.com/pulumi/pulumi/pull/7704)

- [sdk/nodejs] - Use ESlint instead of TSlint
  [#7719](https://github.com/pulumi/pulumi/pull/7719)

- [sdk/python] - Fix pulumi.property's default value handling.
  [#7736](https://github.com/pulumi/pulumi/pull/7736)

## 3.9.1 (2021-07-29)

### Bug Fixes

- [cli] - Respect provider aliases
  [#7166](https://github.com/pulumi/pulumi/pull/7166)

- [cli] - `pulumi stack ls` now returns all accessible stacks (removing
  earlier cap imposed by the httpstate backend).
  [#3620](https://github.com/pulumi/pulumi/issues/3620)

- [sdk/go] - Fix panics caused by logging from `ApplyT`, affecting
  `pulumi-docker` and potentially other providers
  [#7661](https://github.com/pulumi/pulumi/pull/7661)

- [sdk/python] - Handle unknown results from methods.
  [#7677](https://github.com/pulumi/pulumi/pull/7677)

## 3.9.0 (2021-07-28)

### Improvements

- [sdk/go] - Add stack output helpers for numeric types.
  [#7410](https://github.com/pulumi/pulumi/pull/7410)

- [sdk/python] - Permit `Input[Resource]` values in `depends_on`.
  [#7559](https://github.com/pulumi/pulumi/pull/7559)

- [backend/filestate] - Allow pulumi stack ls to see all stacks regardless of passphrase.
  [#7660](https://github.com/pulumi/pulumi/pull/7660)

### Bug Fixes

- [sdk/{go,python,nodejs}] - Rehydrate provider resources in `Construct`.
  [#7624](https://github.com/pulumi/pulumi/pull/7624)

- [engine] - Include children when targeting components.
  [#7605](https://github.com/pulumi/pulumi/pull/7605)

- [cli] - Restore passing log options to providers when `--logflow` is specified
  https://github.com/pulumi/pulumi/pull/7640

- [sdk/nodejs] - Fix `pulumi up --logflow` causing Node multi-lang components to hang
  [#7644](https://github.com/pulumi/pulumi/pull/)

- [sdk/{dotnet,python,nodejs}] - Set the package on DependencyProviderResource.
  [#7630](https://github.com/pulumi/pulumi/pull/7630)


## 3.8.0 (2021-07-22)

### Improvements

- [sdk/dotnet] - Fix async await warnings.
  [#7537](https://github.com/pulumi/pulumi/pull/7537)

- [codegen/dotnet] - Emit dynamic config-getters.
  [#7549](https://github.com/pulumi/pulumi/pull/7549)

- [sdk/python] - Support for authoring resource methods in Python.
  [#7555](https://github.com/pulumi/pulumi/pull/7555)

- [sdk/{go,dotnet}] - Admit non-asset/archive values when unmarshalling into assets and archives.
  [#7579](https://github.com/pulumi/pulumi/pull/7579)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`.
  [#7529](https://github.com/pulumi/pulumi/pull/7529)

- [sdk/go] - Fix target and replace options for the Automation API.
  [#7426](https://github.com/pulumi/pulumi/pull/7426)

- [cli] - Don't escape special characters when printing JSON.
  [#7593](https://github.com/pulumi/pulumi/pull/7593)

- [sdk/go] - Fix panic when marshaling `self` in a method.
  [#7604](https://github.com/pulumi/pulumi/pull/7604)

## 3.7.1 (2021-07-19)

### Improvements

- [codegen/python,nodejs] Emit dynamic config-getters.
  [#7447](https://github.com/pulumi/pulumi/pull/7447), [#7530](https://github.com/pulumi/pulumi/pull/7530)

- [sdk/python] Make `Output[T]` covariant
  [#7483](https://github.com/pulumi/pulumi/pull/7483)

### Bug Fixes

- [sdk/nodejs] Fix a bug in closure serialization.
  [#6999](https://github.com/pulumi/pulumi/pull/6999)

- [cli] Normalize cloud URL during login
  [#7544](https://github.com/pulumi/pulumi/pull/7544)

- [sdk/nodejs,dotnet] Wait on remote component dependencies
  [#7541](https://github.com/pulumi/pulumi/pull/7541)

## 3.7.0 (2021-07-13)

### Improvements

- [sdk/nodejs] Support for calling resource methods.
  [#7377](https://github.com/pulumi/pulumi/pull/7377)

- [sdk/go] Support for calling resource methods.
  [#7437](https://github.com/pulumi/pulumi/pull/7437)

### Bug Fixes

- [codegen/go] Reimplement strict go enums to be Inputs.
  [#7383](https://github.com/pulumi/pulumi/pull/7383)

- [codegen/go] Emit To[ElementType]Output methods for go enum output types.
  [#7499](https://github.com/pulumi/pulumi/pull/7499)

## 3.6.1 (2021-07-07)

### Improvements

- [sdk] Add `replaceOnChanges` resource option.
  [#7226](https://github.com/pulumi/pulumi/pull/7226)

- [sdk/go] Support for authoring resource methods in Go.
  [#7379](https://github.com/pulumi/pulumi/pull/7379)

### Bug Fixes

- [sdk/python] Fix an issue where dependency keys were incorrectly translates to camelcase.
  [#7443](https://github.com/pulumi/pulumi/pull/7443)

- [cli] Fix rendering of diffs for resource without DetailedDiffs.
  [#7500](https://github.com/pulumi/pulumi/pull/7500)

## 3.6.0 (2021-06-30)

### Improvements

- [cli] Added support for passing custom paths that need
  to be watched by the `pulumi watch` command.
  [#7115](https://github.com/pulumi/pulumi/pull/7247)

- [auto/nodejs] Fail early when multiple versions of `@pulumi/pulumi` are detected in nodejs inline programs.
  [#7349](https://github.com/pulumi/pulumi/pull/7349)

- [sdk/go] Add preliminary support for unmarshaling plain arrays and maps of output values.
  [#7369](https://github.com/pulumi/pulumi/pull/7369)

- Initial support for resource methods (Node.js authoring, Python calling).
  [#7363](https://github.com/pulumi/pulumi/pull/7363)

### Bug Fixes

- [sdk/dotnet] Fix swallowed nested exceptions with inline program, so they correctly bubble to the consumer.
  [#7323](https://github.com/pulumi/pulumi/pull/7323)

- [sdk/go] Specify known when creating outputs for `construct`.
  [#7343](https://github.com/pulumi/pulumi/pull/7343)

- [cli] Fix passphrase rotation.
  [#7347](https://github.com/pulumi/pulumi/pull/7347)

- [multilang/python] Fix nested module generation.
  [#7353](https://github.com/pulumi/pulumi/pull/7353)

- [multilang/nodejs] Fix a hang when an error is thrown within an apply in a remote component.
  [#7365](https://github.com/pulumi/pulumi/pull/7365)

- [codegen/python] Include enum docstrings for python.
  [#7374](https://github.com/pulumi/pulumi/pull/7374)

## 3.5.1 (2021-06-16)

**Please Note:** The v3.5.0 release did not complete and was re-released at the same commit as v3.5.1.

### Improvements

- [dotnet/sdk] Support microsoft logging extensions with inline programs.
  [#7117](https://github.com/pulumi/pulumi/pull/7117)

- [dotnet/sdk] Add create unknown to output utilities.
  [#7173](https://github.com/pulumi/pulumi/pull/7173)

- [dotnet] Fix Resharper code issues.
  [#7178](https://github.com/pulumi/pulumi/pull/7178)

- [codegen] Include properties with an underlying type of string on Go provider instances.
  [#7230](https://github.com/pulumi/pulumi/pull/7230)

- [cli] Provide a more helpful error instead of panicking when codegen fails during import.
  [#7265](https://github.com/pulumi/pulumi/pull/7265)

- [codegen/python] Cache package version for improved performance.
  [#7293](https://github.com/pulumi/pulumi/pull/7293)

- [sdk/python] Reduce `log.debug` calls for improved performance.
  [#7295](https://github.com/pulumi/pulumi/pull/7295)

### Bug Fixes

- [sdk/dotnet] Fix resources destroyed after exception thrown during inline program.
  [#7299](https://github.com/pulumi/pulumi/pull/7299)

- [sdk/python] Fix regression in behaviour for `Output.from_input({})`.
  [#7254](https://github.com/pulumi/pulumi/pull/7254)

- [sdk/python] Prevent infinite loops when iterating `Output` objects.
  [#7288](https://github.com/pulumi/pulumi/pull/7288)

- [codegen/python] Rename conflicting ResourceArgs classes.
  [#7171](https://github.com/pulumi/pulumi/pull/7171)

## 3.4.0 (2021-06-05)

### Improvements

- [dotnet/sdk] Add get value async to output utilities.
  [#7170](https://github.com/pulumi/pulumi/pull/7170)

### Bug Fixes

- [CLI] Fix broken venv for Python projects started from templates.
  [#6624](https://github.com/pulumi/pulumi/pull/6623)

- [cli] Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)

- [cli] Protect against panics when using the wrong resource type with `pulumi import`.
  [#7202](https://github.com/pulumi/pulumi/pull/7202)

- [auto/nodejs] Emit warning instead of breaking on parsing JSON events for automation API.
  [#7162](https://github.com/pulumi/pulumi/pull/7162)

- [sdk/python] Improve performance of `Output.from_input` and `Output.all` on nested objects.
  [#7175](https://github.com/pulumi/pulumi/pull/7175)

### Misc
- [cli] Update version of go-cloud used by Pulumi to `0.23.0`.
  [#7204](https://github.com/pulumi/pulumi/pull/7204)

## 3.3.1 (2021-05-25)

### Improvements

- [dotnet/sdk] Use source context with serilog.
  [#7095](https://github.com/pulumi/pulumi/pull/7095)

- [auto/dotnet] Make StackDeployment.FromJsonString public.
  [#7067](https://github.com/pulumi/pulumi/pull/7067)

- [sdk/python] Generated SDKs may now be installed from in-tree source.
  [#7097](https://github.com/pulumi/pulumi/pull/7097)

### Bug Fixes

- [auto/nodejs] Fix an intermittent bug in parsing JSON events.
  [#7032](https://github.com/pulumi/pulumi/pull/7032)

- [auto/dotnet] Fix deserialization of CancelEvent in .NET 5.
  [#7051](https://github.com/pulumi/pulumi/pull/7051)

- Temporarily disable warning when a secret config is read as a non-secret.
  [#7129](https://github.com/pulumi/pulumi/pull/7129)

## 3.3.0 (2021-05-20)

### Improvements

- [cli] Provide user information when protected resources are not able to be deleted
  [#7055](https://github.com/pulumi/pulumi/pull/7055)

- [cli] Error instead of panic on invalid state file import
  [#7065](https://github.com/pulumi/pulumi/pull/7065)

- Warn when a secret config is read as a non-secret
  [#6896](https://github.com/pulumi/pulumi/pull/6896)
  [#7078](https://github.com/pulumi/pulumi/pull/7078)
  [#7079](https://github.com/pulumi/pulumi/pull/7079)
  [#7080](https://github.com/pulumi/pulumi/pull/7080)

- [sdk/nodejs|python] Add GetSchema support to providers
  [#6892](https://github.com/pulumi/pulumi/pull/6892)

- [auto/dotnet] Provide PulumiFn implementation that allows runtime stack type
  [#6910](https://github.com/pulumi/pulumi/pull/6910)

- [auto/go] Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

### Bug Fixes

- [sdk/python] Fix relative `runtime:options:virtualenv` path resolution to ignore `main` project attribute
  [#6966](https://github.com/pulumi/pulumi/pull/6966)

- [auto/dotnet] Disable Language Server Host logging and checking appsettings.json config
  [#7023](https://github.com/pulumi/pulumi/pull/7023)

- [auto/python] Export missing `ProjectBackend` type
  [#6984](https://github.com/pulumi/pulumi/pull/6984)

- [sdk/nodejs] Fix noisy errors.
  [#6995](https://github.com/pulumi/pulumi/pull/6995)

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- [codegen/python] Fix issue with lazy_import affecting pulumi-eks
  [#7024](https://github.com/pulumi/pulumi/pull/7024)

- Ensure that all outstanding asynchronous work is awaited before returning from a .NET
  Pulumi program.
  [#6993](https://github.com/pulumi/pulumi/pull/6993)

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- Build: Add vs code dev container
  [#7052](https://github.com/pulumi/pulumi/pull/7052)

- Ensure that all outstanding asynchronous work is awaited before returning from a Go
  Pulumi program. Note that this may require changes to programs that use the
  `pulumi.NewOutput` API.
  [#6983](https://github.com/pulumi/pulumi/pull/6983)

## 3.2.1 (2021-05-06)

### Bug Fixes

- [cli] Fix a regression caused by [#6893](https://github.com/pulumi/pulumi/pull/6893) that stopped stacks created
  with empty passphrases from completing successful pulumi commands when loading the passphrase secrets provider.
  [#6976](https://github.com/pulumi/pulumi/pull/6976)

## 3.2.0 (2021-05-05)

### Enhancements

- [auto/go] Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

- [automation/*] Add support for getting stack outputs using Workspace
  [#6859](https://github.com/pulumi/pulumi/pull/6859)

- [automation/*] Optionally skip Automation API version check
  [#6882](https://github.com/pulumi/pulumi/pull/6882)
  The version check can be skipped by passing a non-empty value to the `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK` environment variable.

- [auto/go,nodejs] Add UserAgent to update/pre/refresh/destroy options.
  [#6935](https://github.com/pulumi/pulumi/pull/6935)

### Bug Fixes

- [cli] Return an appropriate error when a user has not set `PULUMI_CONFIG_PASSPHRASE` nor `PULUMI_CONFIG_PASSPHRASE_FILE`
  when trying to access the Passphrase Secrets Manager
  [#6893](https://github.com/pulumi/pulumi/pull/6893)

- [cli] Prevent against panic when using a ResourceReference as a program output
  [#6962](https://github.com/pulumi/pulumi/pull/6962)

- [sdk/python] Fix bug in MockResourceArgs.
  [#6863](https://github.com/pulumi/pulumi/pull/6863)

- [sdk/python] Address issues when using resource subclasses.
  [#6890](https://github.com/pulumi/pulumi/pull/6890)

- [sdk/python] Fix type-related regression on Python 3.6.
  [#6942](https://github.com/pulumi/pulumi/pull/6942)

- [sdk/python] Don't error when a dict input value has a mismatched type annotation.
  [#6949](https://github.com/pulumi/pulumi/pull/6949)

- [automation/dotnet] Fix EventLogWatcher failing to read events after an exception was thrown
  [#6821](https://github.com/pulumi/pulumi/pull/6821)

- [automation/dotnet] Use stackName in ImportStack
  [#6858](https://github.com/pulumi/pulumi/pull/6858)

- [automation/go] Improve autoError message formatting
  [#6924](https://github.com/pulumi/pulumi/pull/6924)

### Misc.

- [auto/dotnet] Bump YamlDotNet to 11.1.1
  [#6915](https://github.com/pulumi/pulumi/pull/6915)

- [sdk/dotnet] Enable deterministic builds
  [#6917](https://github.com/pulumi/pulumi/pull/6917)

- [auto/*] Bump minimum version to v3.1.0.
  [#6852](https://github.com/pulumi/pulumi/pull/6852)

## 3.1.0 (2021-04-22)

### Breaking Changes

Please note, the following 2 breaking changes were included in our [3.0 changlog](https://www.pulumi.com/docs/get-started/install/migrating-3.0/#updated-cli-behavior-in-pulumi-30)
Unfortunately, the initial release did not include that change. We apologize for any confusion or inconvenience this may have included the addressed behaviour.

- [cli] Standardize stack select behavior to ensure that passing `--stack` does not make that the current stack.
  [#6840](https://github.com/pulumi/pulumi/pull/6840)

- [cli] Set pagination defaults for `pulumi stack history` to 10 entries.
  [#6841](https://github.com/pulumi/pulumi/pull/6841)

### Enhancements

- [sdk/nodejs] Handle providers for RegisterResourceRequest
  [#6795](https://github.com/pulumi/pulumi/pull/6795)

- [automation/dotnet] Remove dependency on Gprc.Tools for F# / Paket compatibility
  [#6793](https://github.com/pulumi/pulumi/pull/6793)

### Bug Fixes

- [codegen] Fix codegen for types that are used by both resources and functions.
  [#6811](https://github.com/pulumi/pulumi/pull/6811)

- [sdk/python] Fix bug in `get_resource_module` affecting resource hydration.
  [#6833](https://github.com/pulumi/pulumi/pull/6833)

- [automation/python] Fix bug in UpdateSummary deserialization for nested config values.
  [#6838](https://github.com/pulumi/pulumi/pull/6838)


## 3.0.0 (2021-04-19)

### Breaking Changes

- [sdk/cli] Bump version of Pulumi CLI and SDK to v3
  [#6554](https://github.com/pulumi/pulumi/pull/6554)

- Dropped support for NodeJS < v11.x

- [CLI] Standardize the `--stack` flag to *not* set the stack as current (i.e. setStack=false) across CLI commands.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [CLI] Set pagination defaults for `pulumi stack history` to 10 entries.
  [#6739](https://github.com/pulumi/pulumi/pull/6739)

- [CLI] Remove `pulumi history` command. This was previously deprecated and replaced by `pulumi stack history`
  [#6724](https://github.com/pulumi/pulumi/pull/6724)

- [sdk/*] Refactor Mocks newResource and call to accept an argument struct for future extensibility rather than individual args
  [#6672](https://github.com/pulumi/pulumi/pull/6672)

- [sdk/nodejs] Enable nodejs dynamic provider caching by default on program side.
  [#6704](https://github.com/pulumi/pulumi/pull/6704)

- [sdk/python] Improved dict key translation support (3.0-based providers will opt-in to the improved behavior)
  [#6695](https://github.com/pulumi/pulumi/pull/6695)

- [sdk/python] Allow using Python to build resource providers for multi-lang components.
  [#6715](https://github.com/pulumi/pulumi/pull/6715)

- [sdk/go] Simplify `Apply` method options to reduce binary size
  [#6607](https://github.com/pulumi/pulumi/pull/6607)

- [Automation/*] All operations use `--stack` to specify the stack instead of running `select stack` before the operation.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/nodejs] Moving NodeJS automation API package from sdk/nodejs/x/automation -> sdk/nodejs/automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/python] Moving Python automation API package from pulumi.x.automation -> pulumi.automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)


### Enhancements

- [sdk/nodejs] Add support for multiple V8 VM contexts in closure serialization.
  [#6648](https://github.com/pulumi/pulumi/pull/6648)

- [sdk] Handle providers for RegisterResourceRequest
  [#6771](https://github.com/pulumi/pulumi/pull/6771)
  [#6781](https://github.com/pulumi/pulumi/pull/6781)
  [#6786](https://github.com/pulumi/pulumi/pull/6786)

- [sdk/go] Support defining remote components in Go.
  [#6403](https://github.com/pulumi/pulumi/pull/6403)


### Bug Fixes

- [CLI] Clean the template cache if the repo remote has changed.
  [#6784](https://github.com/pulumi/pulumi/pull/6784)


## 2.25.2 (2021-04-17)

### Bug Fixes

- [cli] Fix a bug that prevented copying checkpoint files when using Azure Blob Storage
  as the backend provider. [#6794](https://github.com/pulumi/pulumi/pull/6794)

## 2.25.1 (2021-04-15)

### Bug Fixes

- [automation/python] Fix serialization bug in `StackSettings`
  [#6776](https://github.com/pulumi/pulumi/pull/6776)

## 2.25.0 (2021-04-14)

### Breaking

- [automation/dotnet] Rename (Get,Set,Remove)Config(Value)
  [#6731](https://github.com/pulumi/pulumi/pull/6731)

  The following methods on Workspace and WorkspaceStack classes have
  been renamed. Please update your code (before -> after):

  * GetConfigValue -> GetConfig
  * SetConfigValue -> SetConfig
  * RemoveConfigValue -> RemoveConfig
  * GetConfig -> GetAllConfig
  * SetConfig -> SetAllConfig
  * RemoveConfig -> RemoveAllConfig

  This change was made to align with the other Pulumi language SDKs.

### Improvements

- [cli] Add option to print absolute rather than relative dates in stack history
  [#6742](https://github.com/pulumi/pulumi/pull/6742)

  Example:
  ```bash
  pulumi stack history --full-dates
  ```

- [cli] Enable absolute and relative parent paths for pulumi main
  [#6734](https://github.com/pulumi/pulumi/pull/6734)

- [sdk/dotnet] Thread-safe concurrency-friendly global state
  [#6139](https://github.com/pulumi/pulumi/pull/6139)

- [tooling] Update pulumi python docker image to python 3.9
  [#6706](https://github.com/pulumi/pulumi/pull/6706)

- [sdk/nodejs] Add program side caching for dynamic provider serialization behind env var
  [#6673](https://github.com/pulumi/pulumi/pull/6673)

- [sdk/nodejs] Allow prompt values in `construct` for multi-lang components.
  [#6522](https://github.com/pulumi/pulumi/pull/6522)

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

- [automation/dotnet] Expose WorkspaceStack.GetOutputsAsync
  [#6699](https://github.com/pulumi/pulumi/pull/6699)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  await stack.SetConfigAsync(config);
  var initialOutputs = await stack.GetOutputsAsync();
  ```

- [automation/dotnet] Implement (Import,Export)StackAsync methods on LocalWorkspace and WorkspaceStack and expose StackDeployment helper class.
  [#6728](https://github.com/pulumi/pulumi/pull/6728)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  var upResult = await stack.UpAsync();
  deployment = await workspace.ExportStackAsync(stackName);
  ```

- [automation/dotnet] Implement CancelAsync method on WorkspaceStack
  [#6729](https://github.com/pulumi/pulumi/pull/6729)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  var cancelTask = stack.CancelAsync();
  ```

- [automation/python] Expose structured logging for Stack.up/preview/refresh/destroy.
  [#6527](https://github.com/pulumi/pulumi/pull/6527)

  You can now pass in an `on_event` callback function as a keyword arg to `up`, `preview`, `refresh`
  and `destroy` to process streaming json events defined in `automation/events.py`

  Example:
  ```python
  stack.up(on_event=print)
  ```

### Bug Fixes

- [sdk/go] Fix wrongly named Go modules
  [#6775](https://github.com/pulumi/pulumi/issues/6775)

- [cli] Handle non-existent creds file in `pulumi logout --all`
  [#6741](https://github.com/pulumi/pulumi/pull/6741)

- [automation/nodejs] Do not run the promise leak checker if an inline program has errored.
  [#6758](https://github.com/pulumi/pulumi/pull/6758)

- [sdk/nodejs] Explicitly create event log file for NodeJS Automation API.
  [#6730](https://github.com/pulumi/pulumi/pull/6730)

- [sdk/nodejs] Fix error handling for failed logging statements
  [#6714](https://github.com/pulumi/pulumi/pull/6714)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452)

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)

- [automation] Set default value for 'main' for inline programs to support relative paths, assets, and closure serialization.
  [#6743](https://github.com/pulumi/pulumi/pull/6743)

- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [automation/dotnet] Fix GetConfigValueAsync failing to deserialize
  [#6698](https://github.com/pulumi/pulumi/pull/6698)

- [automation] Fix (de)serialization of StackSettings in .NET, Node, and Python.
  [#6752](https://github.com/pulumi/pulumi/pull/6752)
  [#6754](https://github.com/pulumi/pulumi/pull/6754)
  [#6749](https://github.com/pulumi/pulumi/pull/6749)


## 2.24.1 (2021-04-01)

### Bug Fixes

- [cli] Revert the swapping out of the YAML parser library
  [#6681](https://github.com/pulumi/pulumi/pull/6681)

- [automation/go,python,nodejs] Respect pre-existing Pulumi.yaml for inline programs.
  [#6655](https://github.com/pulumi/pulumi/pull/6655)

## 2.24.0 (2021-03-31)

### Improvements

- [sdk/nodejs] Add provider side caching for dynamic provider deserialization
  [#6657](https://github.com/pulumi/pulumi/pull/6657)

- [automation/dotnet] Expose structured logging
  [#6572](https://github.com/pulumi/pulumi/pull/6572)

- [cli] Support full fidelity YAML round-tripping
  - Strip Byte-order Mark (BOM) from YAML configs during load. [#6636](https://github.com/pulumi/pulumi/pull/6636)
  - Swap out YAML parser library [#6642](https://github.com/pulumi/pulumi/pull/6642)

- [sdk/python] Ensure all async tasks are awaited prior to exit.
  [#6606](https://github.com/pulumi/pulumi/pull/6606)

### Bug Fixes

- [sdk/nodejs] Fix error propagation in registerResource and other resource methods.
  [#6644](https://github.com/pulumi/pulumi/pull/6644)

- [automation/python] Fix passing of additional environment variables.
  [#6639](https://github.com/pulumi/pulumi/pull/6639)

- [sdk/python] Make exceptions raised by calls to provider functions (e.g. data sources) catchable.
  [#6504](https://github.com/pulumi/pulumi/pull/6504)

- [automation/go,python,nodejs] Respect pre-existing Pulumi.yaml for inline programs.
  [#6655](https://github.com/pulumi/pulumi/pull/6655)

## 2.23.2 (2021-03-25)

### Improvements

- [cli] Improve diff displays during `pulumi refresh`
  [#6568](https://github.com/pulumi/pulumi/pull/6568)

- [sdk/go] Cache loaded configuration files.
  [#6576](https://github.com/pulumi/pulumi/pull/6576)

- [sdk/nodejs] Allow `Mocks::newResource` to determine whether the created resource is a `CustomResource`.
  [#6551](https://github.com/pulumi/pulumi/pull/6551)

- [automation/*] Implement minimum version checking and add:
  - Go: `LocalWorkspace.PulumiVersion()` [#6577](https://github.com/pulumi/pulumi/pull/6577)
  - Nodejs: `LocalWorkspace.pulumiVersion` [#6580](https://github.com/pulumi/pulumi/pull/6580)
  - Python: `LocalWorkspace.pulumi_version` [#6589](https://github.com/pulumi/pulumi/pull/6589)
  - Dotnet: `LocalWorkspace.PulumiVersion` [#6590](https://github.com/pulumi/pulumi/pull/6590)

### Bug Fixes

- [sdk/python] Fix automatic venv creation
  [#6599](https://github.com/pulumi/pulumi/pull/6599)

- [automation/python] Fix Settings file save
  [#6605](https://github.com/pulumi/pulumi/pull/6605)

- [sdk/dotnet] Remove MaybeNull from Output/Input.Create to avoid spurious warnings
  [#6600](https://github.com/pulumi/pulumi/pull/6600)

## 2.23.1 (2021-03-17)

### Bug Fixes

- [cli] Fix a bug where a version wasn't passed to go install commands as part of `make brew` installs from homebrew
  [#6566](https://github.com/pulumi/pulumi/pull/6566)

## 2.23.0 (2021-03-17)

### Breaking

- [automation/go] Expose structured logging for Stack.Up/Preview/Refresh/Destroy.
  [#6436](https://github.com/pulumi/pulumi/pull/6436)

This change is marked breaking because it changes the shape of the `PreviewResult` struct.

**Before**

```go
type PreviewResult struct {
  Steps         []PreviewStep  `json:"steps"`
  ChangeSummary map[string]int `json:"changeSummary"`
}
```

**After**

```go
type PreviewResult struct {
  StdOut        string
  StdErr        string
  ChangeSummary map[apitype.OpType]int
}
```

- [automation/dotnet] Add ability to capture stderr
  [#6513](https://github.com/pulumi/pulumi/pull/6513)

This change is marked breaking because it also renames `OnOutput` to `OnStandardOutput`.

### Improvements

- [sdk/go] Add helpers to convert raw Go maps and arrays to Pulumi `Map` and `Array` inputs.
  [#6337](https://github.com/pulumi/pulumi/pull/6337)

- [sdk/go] Return zero values instead of panicing in `Index` and `Elem` methods.
  [#6338](https://github.com/pulumi/pulumi/pull/6338)

- [sdk/go] Support multiple folders in GOPATH.
  [#6228](https://github.com/pulumi/pulumi/pull/6228

- [cli] Add ability to download arm64 provider plugins
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [build] Updating Pulumi to use Go 1.16
  [#6470](https://github.com/pulumi/pulumi/pull/6470)

- [build] Adding a Pulumi arm64 binary for use on new macOS hardware.
  Please note that `pulumi watch` will not be supported on darwin/arm64 builds.
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [automation/nodejs] Expose structured logging for Stack.up/preview/refresh/destroy.
  [#6454](https://github.com/pulumi/pulumi/pull/6454)

- [automation/nodejs] Add `onOutput` event handler to `PreviewOptions`.
  [#6507](https://github.com/pulumi/pulumi/pull/6507)

- [cli] Add locking support to the self-managed backends using the `PULUMI_SELF_MANAGED_STATE_LOCKING=1` environment variable.
  [#2697](https://github.com/pulumi/pulumi/pull/2697)

### Bug Fixes

- [sdk/python] Fix mocks issue when passing a resource more than once.
  [#6479](https://github.com/pulumi/pulumi/pull/6479)

- [automation/dotnet] Add ReadDiscard OperationType
  [#6493](https://github.com/pulumi/pulumi/pull/6493)

- [cli] Ensure the user has the correct access to the secrets manager before using it as part of
  `pulumi stack export --show-secrets`.
  [#6215](https://github.com/pulumi/pulumi/pull/6210)

- [sdk/go] Implement getResource in the mock monitor.
  [#5923](https://github.com/pulumi/pulumi/pull/5923)

## 2.22.0 (2021-03-03)

### Improvements

- [#6410](https://github.com/pulumi/pulumi/pull/6410) Add `diff` option to Automation API's `preview` and `up`

### Bug Fixes

- [automation/dotnet] resolve issue with OnOutput delegate not being called properly during pulumi process execution.
  [#6435](https://github.com/pulumi/pulumi/pull/6435)

- [automation/python,nodejs,dotnet] BREAKING Remove `summary` property from `PreviewResult`.
  The `summary` property on `PreviewResult` returns a result that is always incorrect and is being removed.
  [#6405](https://github.com/pulumi/pulumi/pull/6405)

- [automation/python] Fix Windows error caused by use of NamedTemporaryFile in automation api.
  [#6421](https://github.com/pulumi/pulumi/pull/6421)

- [sdk/nodejs] Serialize default parameters correctly. [#6397](https://github.com/pulumi/pulumi/pull/6397)

- [cli] Respect provider aliases while diffing resources.
  [#6453](https://github.com/pulumi/pulumi/pull/6453)

## 2.21.2 (2021-02-22)

### Improvements

- [cli] Disable permalinks to the update details page when using self-managed backends (S3, Azure, GCS). Should the user
  want to get permalinks when using a self backend, they can pass a flag:
      `pulumi up --suppress-permalink false`.
  Permalinks for these self-managed backends will be suppressed on `update`, `preview`, `destroy`, `import` and `refresh`
  operations.
  [#6251](https://github.com/pulumi/pulumi/pull/6251)

- [cli] Added commands `config set-all` and `config rm-all` to set and remove multiple configuration keys.
  [#6373](https://github.com/pulumi/pulumi/pull/6373)

- [automation/*] Consume `config set-all` and `config rm-all` from automation API.
  [#6388](https://github.com/pulumi/pulumi/pull/6388)

- [sdk/dotnet] C# Automation API.
  [#5761](https://github.com/pulumi/pulumi/pull/5761)

- [sdk/dotnet] F# API to specify stack options.
  [#5077](https://github.com/pulumi/pulumi/pull/5077)

### Bug Fixes

- [sdk/nodejs] Don't error when loading multiple copies of the same version of a Node.js
  component package. [#6387](https://github.com/pulumi/pulumi/pull/6387)

- [cli] Skip unnecessary state file writes to address performance regression introduced in 2.16.2.
  [#6396](https://github.com/pulumi/pulumi/pull/6396)

## 2.21.1 (2021-02-18)

### Bug Fixes

- [sdk/python] Fixed a change to `Output.all()` that raised an error if no inputs are passed in.
  [#6381](https://github.com/pulumi/pulumi/pull/6381)

## 2.21.0 (2021-02-17)

### Improvements

- [cli] Added pagination options to `pulumi stack history` [#6292](https://github.com/pulumi/pulumi/pull/6292)
  This is used as follows:
  `pulumi stack history --page-size=20 --page=1`

- [automation/*] Added pagination options for stack history in Automation API SDKs to improve
  performance of stack updates. [#6257](https://github.com/pulumi/pulumi/pull/6257)
  This is used similar to the following example in go:
```go
  func ExampleStack_History() {
	ctx := context.Background()
	stackName := FullyQualifiedStackName("org", "project", "stack")
	stack, _ := SelectStackLocalSource(ctx, stackName, filepath.Join(".", "program"))
	pageSize := 0
	page := 0
	hist, _ := stack.History(ctx, pageSize, page)
	fmt.Println(hist[0].StartTime)
  }
```

- [pkg/testing/integration] Changed the default behavior for Python test projects to use `UseAutomaticVirtualEnv` by
  default. `UsePipenv` is now the way to use pipenv with tests.
  [#6318](https://github.com/pulumi/pulumi/pull/6318)

### Bug Fixes

- [automation/go] Exposed the version in the UpdateSummary for use in understanding the version of a stack update
  [#6339](https://github.com/pulumi/pulumi/pull/6339)

- [cli] Changed the behavior for Python on Windows to look for `python` binary first instead of `python3`.
  [#6317](https://github.com/pulumi/pulumi/pull/6317)

- [sdk/python] Gracefully handle monitor shutdown in the python runtime without exiting the process.
  [#6249](https://github.com/pulumi/pulumi/pull/6249)

- [sdk/python] Fixed a bug in `contains_unknowns` where outputs with a property named "values" failed with a TypeError.
  [#6264](https://github.com/pulumi/pulumi/pull/6264)

- [sdk/python] Allowed keyword args in Output.all() to create a dict.
  [#6269](https://github.com/pulumi/pulumi/pull/6269)

- [sdk/python] Defined `__all__` in modules for better IDE autocomplete.
  [#6351](https://github.com/pulumi/pulumi/pull/6351)

- [automation/python] Fixed a bug in nested configuration parsing.
  [#6349](https://github.com/pulumi/pulumi/pull/6349)

## 2.20.0 (2021-02-03)

- [sdk/python] Fix `Output.from_input` to unwrap nested output values in input types (args classes), which addresses
  an issue that was preventing passing instances of args classes with nested output values to Provider resources.
  [#6221](https://github.com/pulumi/pulumi/pull/6221)

## 2.19.0 (2021-01-27)

- [sdk/nodejs] Always read and write NodeJS runtime options from the environment.
  [#6076](https://github.com/pulumi/pulumi/pull/6076)

- [sdk/go] Take a breaking change to remove unidiomatic numerical types and drastically improve build performance (binary size and compilation time).
  [#6143](https://github.com/pulumi/pulumi/pull/6143)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key from hashivault to passphrase provider
  [#6210](https://github.com/pulumi/pulumi/pull/6210)

## 2.18.2 (2021-01-22)

- [CLI] Fix malformed resource value bug.
  [#6164](https://github.com/pulumi/pulumi/pull/6164)

- [sdk/dotnet] Fix `RegisterResourceOutputs` to serialize resources as resource references
  only when the monitor reports that resource references are supported.
  [#6172](https://github.com/pulumi/pulumi/pull/6172)

- [CLI] Avoid panic for diffs with invalid property paths.
  [#6159](https://github.com/pulumi/pulumi/pull/6159)

- Enable resource reference feature by default.
  [#6202](https://github.com/pulumi/pulumi/pull/6202)

## 2.18.1 (2021-01-21)

- Revert [#6125](https://github.com/pulumi/pulumi/pull/6125) as it caused a which introduced a bug with serializing resource IDs

## 2.18.0 (2021-01-20)

- [CLI] Add the ability to log out of all Pulumi backends at once.
  [#6101](https://github.com/pulumi/pulumi/pull/6101)

- [sdk/go] Added `pulumi.Unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.IsSecret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6085](https://github.com/pulumi/pulumi/pull/6085)

## 2.17.2 (2021-01-14)

- .NET: Allow `IMock.NewResourceAsync` to return a null ID for component resources.
  Note that this may require mocks written in C# to be updated to account for the
  change in nullability.
  [#6104](https://github.com/pulumi/pulumi/pull/6104)

- [automation/go] Add debug logging settings for common automation API operations
  [#6095](https://github.com/pulumi/pulumi/pull/6095)

- [automation/go] Set DryRun on previews so unknowns are identified correctly.
  [#6099](https://github.com/pulumi/pulumi/pull/6099)

- [sdk/python] Fix python 3.6 support by removing annotations import.
  [#6109](https://github.com/pulumi/pulumi/pull/6109)

- [sdk/nodejs] Added `pulumi.unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.isSecret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6086](https://github.com/pulumi/pulumi/pull/6086)

- [sdk/python] Added `pulumi.unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.is_secret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6111](https://github.com/pulumi/pulumi/pull/6111)

## 2.17.1 (2021-01-13)

- Fix an issue with go sdk generation where optional strict enum values
  could not be omitted. Note - this is a breaking change to go sdk's enum
  values. However we currently only support strict enums in the azure-nextgen provider's schema.
  [#6069](https://github.com/pulumi/pulumi/pull/6069)

- Fix an issue where python debug messages print unexpectedly.
  [#6967](https://github.com/pulumi/pulumi/pull/6067)

- [CLI] Add `version` to the stack history output to be able to
  correlate events back to the Pulumi SaaS
  [#6063](https://github.com/pulumi/pulumi/pull/6063)

- Fix a typo in the unit testing mocks to get the outputs
  while registering them
  [#6040](https://github.com/pulumi/pulumi/pull/6040)

- [sdk/dotnet] Moved urn value retrieval into if statement
  for MockMonitor
  [#6081](https://github.com/pulumi/pulumi/pull/6081)

- [sdk/dotnet] Added `Pulumi.Output.Unsecret` which will
  take an existing secret output and
  create a non-secret variant with an unwrapped secret value.
  [#6092](https://github.com/pulumi/pulumi/pull/6092)

- [sdk/dotnet] Added `Pulumi.Output.IsSecretAsync` which will
  take an existing output and
  determine if an output has a secret within the output.
  [#6092](https://github.com/pulumi/pulumi/pull/6092)

- [sdk/dotnet] Fix looking up empty version in
  `ResourcePackages.TryGetResourceType`.
  [#6084](https://github.com/pulumi/pulumi/pull/6084)

- Python Automation API.
  [#5979](https://github.com/pulumi/pulumi/pull/5979)

- Support recovery workflow (import/export/cancel) in Python Automation API.
  [#6037](https://github.com/pulumi/pulumi/pull/6037)

## 2.17.0 (2021-01-06)

- Respect the `version` resource option for provider resources.
  [#6055](https://github.com/pulumi/pulumi/pull/6055)

- Allow `serializeFunction` to capture secrets.
  [#6013](https://github.com/pulumi/pulumi/pull/6013)

- [CLI] Allow `pulumi console` to accept a stack name
  [#6031](https://github.com/pulumi/pulumi/pull/6031)

- Support recovery workflow (import/export/cancel) in NodeJS Automation API.
  [#6038](https://github.com/pulumi/pulumi/pull/6038)

- [CLI] Add a confirmation prompt when using `pulumi policy rm`
  [#6034](https://github.com/pulumi/pulumi/pull/6034)

- [CLI] Ensure errors with the Pulumi credentials file
  give the user some information on how to resolve the problem
  [#6044](https://github.com/pulumi/pulumi/pull/6044)

- [sdk/go] Support maps in Invoke outputs and Read inputs
  [#6014](https://github.com/pulumi/pulumi/pull/6014)

## 2.16.2 (2020-12-23)

- Fix a bug in the core engine that could cause previews to fail if a resource with changes had
  unknown output property values.
  [#6006](https://github.com/pulumi/pulumi/pull/6006)

## 2.16.1 (2020-12-22)

- Fix a panic due to unsafe concurrent map access.
  [#5995](https://github.com/pulumi/pulumi/pull/5995)

- Fix regression in `venv` creation for python policy packs.
  [#5992](https://github.com/pulumi/pulumi/pull/5992)

## 2.16.0 (2020-12-21)

- Do not read plugins and policy packs into memory prior to extraction, as doing so can exhaust
  the available memory on lower-end systems.
  [#5983](https://github.com/pulumi/pulumi/pull/5983)

- Fix a bug in the core engine where deleting/renaming a resource would panic on update + refresh.
  [#5980](https://github.com/pulumi/pulumi/pull/5980)

- Fix a bug in the core engine that caused `ignoreChanges` to fail for resources being imported.
  [#5976](https://github.com/pulumi/pulumi/pull/5976)

- Fix a bug in the core engine that could cause resources references to marshal improperly
  during preview.
  [#5960](https://github.com/pulumi/pulumi/pull/5960)

- [sdk/dotnet] Add collection initializers for smooth support of Union<T, U> as element type
  [#5938](https://github.com/pulumi/pulumi/pull/5938)

- Fix a bug in the core engine where ComponentResource state would be accessed before initialization.
  [#5949](https://github.com/pulumi/pulumi/pull/5949)

- Prevent a panic by not attempting to show progress for zero width/height terminals.
  [#5957](https://github.com/pulumi/pulumi/issues/5957)

## 2.15.6 (2020-12-12)

- Fix a bug in the Go SDK that could result in dropped resource dependencies.
  [#5930](https://github.com/pulumi/pulumi/pull/5930)

- Temporarily disable resource ref feature.
  [#5932](https://github.com/pulumi/pulumi/pull/5932)

## 2.15.5 (2020-12-11)

- Re-apply fix for running multiple `pulumi` processes concurrently.
  [#5893](https://github.com/pulumi/pulumi/issues/5893)

- [cli] Prevent a panic when using `pulumi import` with local filesystems
  [#5906](https://github.com/pulumi/pulumi/issues/5906)

- [sdk/nodejs] Fix issue that would cause unit tests using mocks to fail with unhandled errors when
  a resource references another resources that's been registered with `registerResourceModule`.
  [#5914](https://github.com/pulumi/pulumi/pull/5914)

- Enable resource reference feature by default.
  [#5905](https://github.com/pulumi/pulumi/pull/5905)

- [codegen/go] Fix Input/Output methods for Go resources.
  [#5916](https://github.com/pulumi/pulumi/pull/5916)

- [sdk/python] Implement getResource in the mock monitor.
  [#5919](https://github.com/pulumi/pulumi/pull/5919)

- [sdk/dotnet] Implement getResource in the mock monitor and fix some issues around
  deserializing resources.
  [#5921](https://github.com/pulumi/pulumi/pull/5921)

## 2.15.4 (2020-12-08)

- Fix a problem where `pulumi import` could panic on an import error due to missing error message.
  [#5884](https://github.com/pulumi/pulumi/pull/5884)
- Correct the system name detected for Jenkins CI. [#5891](https://github.com/pulumi/pulumi/pull/5891)

- Fix python execution for users running Python installed through the Windows App Store
  on Windows 10 [#5874](https://github.com/pulumi/pulumi/pull/5874)

## 2.15.3 (2020-12-07)

- Fix errors when running `pulumi` in Windows-based CI environments.
  [#5879](https://github.com/pulumi/pulumi/issues/5879)

## 2.15.2 (2020-12-07)

- Fix a problem where `pulumi import` could panic on importing arrays and sets, due to
  incorrect array resizing logic. [#5872](https://github.com/pulumi/pulumi/pull/5872).

## 2.15.1 (2020-12-04)

- Address potential issues when running multiple `pulumi` processes concurrently.
  [#5857](https://github.com/pulumi/pulumi/pull/5857)

- Automatically install missing Python dependencies.
  [#5787](https://github.com/pulumi/pulumi/pull/5787)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key for a passphrase provider
  [#5865](https://github.com/pulumi/pulumi/pull/5865/)

## 2.15.0 (2020-12-02)

- [sdk/python] Add deserialization support for enums.
  [#5615](https://github.com/pulumi/pulumi/pull/5615)

- Correctly rename `Pulumi.*.yaml` stack files during a rename that includes an
  organization in its name [#5812](https://github.com/pulumi/pulumi/pull/5812).

- Respect `PULUMI_PYTHON_CMD` in scripts.
  [#5782](https://github.com/pulumi/pulumi/pull/5782)

- Add `PULUMI_BACKEND_URL` environment variable to configure the state backend.
  [#5789](https://github.com/pulumi/pulumi/pull/5789)

- [sdk/dotnet] Add support for dependency injection into TStack instance by adding an overload to `Deployment.RunAsync`. The overload accepts an `IServiceProvider` that is used to create the instance of TStack. Also added a new method `Deployment.TestWithServiceProviderAsync` for testing stacks that use dependency injection.
  [#5723](https://github.com/pulumi/pulumi/pull/5723/)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key in Azure KeyVault
  [#5842](https://github.com/pulumi/pulumi/pull/5842/)

## 2.14.0 (2020-11-18)

- Propagate secretness of provider configuration through to the statefile. This ensures
  that any configuration values marked as secret (i.e. values set with
  `pulumi config set --secret`) that are used as inputs to providers are encrypted
  before they are stored.
  [#5742](https://github.com/pulumi/pulumi/pull/5742)

- Fix a bug that could prevent `pulumi import` from succeeding.
  [#5730](https://github.com/pulumi/pulumi/pull/5730)

- [Docs] Add support for the generation of Import documentation in the resource docs.
  This documentation will only be available if the resource is importable.
  [#5667](https://github.com/pulumi/pulumi/pull/5667)

- [codegen/go] Add support for ResourceType and isComponent to enable multi-language
  components in Go. This change also generates Input/Output types for all resources
  in downstream Go SDKs.
  [#5497](https://github.com/pulumi/pulumi/pull/5497)

- Support python 3.9 on Windows.
  [#5739](https://github.com/pulumi/pulumi/pull/5739)

- `pulumi-language-go` and `pulumi new` now explicitly requires Go 1.14.0 or greater.
  [#5741](https://github.com/pulumi/pulumi/pull/5741)

- Update .NET `Grpc` libraries to 2.33.1 and `Protobuf` to 3.13.0 (forked to increase
  the recursion limit) [#5757](https://github.com/pulumi/pulumi/pull/5757)

- Fix plugin install failures on Windows.
  [#5759](https://github.com/pulumi/pulumi/pull/5759)

- .NET: Report plugin install errors during `pulumi new`.
  [#5760](https://github.com/pulumi/pulumi/pull/5760)

- Correct error message on KeyNotFoundException against StackReference.
  [#5740](https://github.com/pulumi/pulumi/pull/5740)

- [cli] Small UX change on the policy violations output to render as `type: name`
  [#5773](https://github.com/pulumi/pulumi/pull/5773)

## 2.13.2 (2020-11-06)

- Fix a bug that was causing errors when (de)serializing custom resources.
  [#5709](https://github.com/pulumi/pulumi/pull/5709)

## 2.13.1 (2020-11-06)

- [cli] Ensure `pulumi history` annotes when secrets are unable to be decrypted
  [#5701](https://github.com/pulumi/pulumi/pull/5701)

- Fix a bug in the Python SDK that caused incompatibilities with versions of the CLI prior to
  2.13.0.
  [#5702](https://github.com/pulumi/pulumi/pull/5702)

## 2.13.0 (2020-11-04)

- Add internal scaffolding for using cross-language components from Go.
  [#5558](https://github.com/pulumi/pulumi/pull/5558)

- Support python 3.9.
  [#5669](https://github.com/pulumi/pulumi/pull/5669)

- [cli] Ensure that the CLI doesn't panic when using pulumi watch and using ComponentResources with non-standard naming
  [#5675](https://github.com/pulumi/pulumi/pull/5675)

- [cli] Ensure that the CLI doesn't panic when trying to assemble a graph on a stack that has no snapshot available
  [#5678](https://github.com/pulumi/pulumi/pull/5678)

- Add boolean values to Go SDK
  [#5687](https://github.com/pulumi/pulumi/pull/5687)

## 2.12.1 (2020-10-23)

- [cli] Ensure that the CLI doesn't panic when using pulumi watch and policies are enabled
  [#5569](https://github.com/pulumi/pulumi/pull/5569)

- [cli] Ensure that the CLI doesn't panic when using the JSON output as part of previews
  and policies are enabled
  [#5610](https://github.com/pulumi/pulumi/pull/5610)


## 2.12.0 (2020-10-14)

- NodeJS Automation API.
  [#5347](https://github.com/pulumi/pulumi/pull/5347)

- Improve the accuracy of previews by allowing providers to participate in determining what
  the impact of a change will be on output properties. Previously, Pulumi previews
  conservatively assumed that any output-only properties changed their values when an update
  occurred. For many properties, this was guaranteed to not be the case (because those
  properties are immutable, for example), and by suggesting the value might change, this could
  lead to the preview suggesting additional transitive updates of even replaces that would not
  actually happen during an update. Pulumi now allows the provider to specify the details of
  what properties will change during a preview, allowing them to expose more accurate
  provider-specific knowledge. This change is less conservative than the previous behavior,
  and so in case it causes preview results which are not deemed correct in some case - the
  `PULUMI_DISABLE_PROVIDER_PREVIEW` flag can be set to a truthy value (e.g. `1`) to enable the
  previous and more conservative behavior for previews.
  [#5443](https://github.com/pulumi/pulumi/pull/5443).

- Add an import command to the Pulumi CLI. This command can be used to import existing resources
  into a Pulumi stack.
  [#4765](https://github.com/pulumi/pulumi/pull/4765)

- [cli] Remove eternal loop if a configured passphrase is invalid.
  [#5507](https://github.com/pulumi/pulumi/pull/5507)

- Correctly validate project names during 'pulumi new'
  [#5504](https://github.com/pulumi/pulumi/pull/5504)

- Fixing gzip compression for alternative backends.
  [#5484](https://github.com/pulumi/pulumi/pull/5484)

- Add internal scaffolding for using cross-language components from .NET.
  [#5485](https://github.com/pulumi/pulumi/pull/5485)

- Support self-contained executables as binary option for .NET programs.
  [#5519](https://github.com/pulumi/pulumi/pull/5519)

- [cli] Ensure old secret provider variables are cleaned up when changing between secret providers
  [#5545](https://github.com/pulumi/pulumi/pull/5545)

- [cli] Respect logging verbosity as part of pulumi plugin install commands
  [#5549](https://github.com/pulumi/pulumi/pull/5549)

- [cli] Accept `-f` as a shorthand for `--skip-preview` on `pulumi up`, `pulumi refresh` and `pulumi destroy` operations
  [#5556](https://github.com/pulumi/pulumi/pull/5556)

- [cli] Validate cloudUrl formats before `pulumi login` and throw an error if incorrect format specified
  [#5550](https://github.com/pulumi/pulumi/pull/5545)

- [automation api] Add support for passing a private ssh key for git authentication that doesn't rely on a file path
  [#5557](https://github.com/pulumi/pulumi/pull/5557)

- [cli] Improve user experience when pulumi plugin rm --all finds no plugins
  to remove. The previous behaviour was an error and should not be so.
  [#5547](https://github.com/pulumi/pulumi/pull/5547)

- [sdk/python] Fix ResourceOptions annotations and doc strings.
  [#5559](https://github.com/pulumi/pulumi/pull/5559)

- [sdk/dotnet] Fix HashSet concurrency issue.
  [#5563](https://github.com/pulumi/pulumi/pull/5563)

## 2.11.2 (2020-10-01)

- feat(autoapi): expose EnvVars LocalWorkspaceOption to set in ctor
  [#5499](https://github.com/pulumi/pulumi/pull/5499)

- [sdk/python] Fix secret regression: ensure unwrapped secrets during deserialization
  are rewrapped before being returned.
  [#5496](https://github.com/pulumi/pulumi/pull/5496)

## 2.11.1 (2020-09-30)

- Add internal scaffolding for using cross-language components from Python.
  [#5375](https://github.com/pulumi/pulumi/pull/5375)

## 2.11.0 (2020-09-30)

- Do not oversimplify types for display when running an update or preview.
  [#5440](https://github.com/pulumi/pulumi/pull/5440)

- Pulumi Windows CLI now uploads all VCS information to console
  (fixes [#5014](https://github.com/pulumi/pulumi/issues/5014))
  [#5406](https://github.com/pulumi/pulumi/pull/5406)

- .NET SDK: Support `Output<object>` for resource output properties
  (fixes [#5446](https://github.com/pulumi/pulumi/issues/5446))
  [#5465](https://github.com/pulumi/pulumi/pull/5465)

## 2.10.2 (2020-09-21)

- [sdk/go] Add missing Version field to invokeOptions
  [#5401](https://github.com/pulumi/pulumi/pull/5401)

- Add `pulumi console` command which opens the currently selected stack in the Pulumi console.
  [#5368](https://github.com/pulumi/pulumi/pull/5368)

- Python SDK: Cast numbers intended to be integers to `int`.
  [#5419](https://github.com/pulumi/pulumi/pull/5419)

## 2.10.1 (2020-09-16)

- feat(autoapi): add GetPermalink for operation result
  [#5363](https://github.com/pulumi/pulumi/pull/5363)

- Relax stack name validations for Automation API [#5337](https://github.com/pulumi/pulumi/pull/5337)

- Allow Pulumi to read a passphrase file, via `PULUMI_CONFIG_PASSPHRASE_FILE` to interact
  with the passphrase secrets provider. Pulumi will first try and use the `PULUMI_CONFIG_PASSPHRASE`
  to get the passphrase then will check `PULUMI_CONFIG_PASSPHRASE_FILE` and then all through to
  asking interactively as the final option.
  [#5327](https://github.com/pulumi/pulumi/pull/5327)

- feat(autoapi): Add support for working with private Git repos. Either `SSHPrivateKeyPath`,
  `PersonalAccessToken` or `UserName` and `Password` can be pushed to the `auto.GitRepo` struct
  when interacting with a private repo
  [#5333](https://github.com/pulumi/pulumi/pull/5333)

- Revise the design for connecting an existing language runtime to a CLI invocation.
  Note that this is a protocol breaking change for the Automation API, so both the
  API and the CLI must be updated together.
  [#5317](https://github.com/pulumi/pulumi/pull/5317)

- Automation API - support streaming output for Up/Refresh/Destroy operations.
  [#5367](https://github.com/pulumi/pulumi/pull/5367)

- Automation API - add recovery APIs (cancel/export/import)
  [#5369](https://github.com/pulumi/pulumi/pull/5369)

## 2.10.0 (2020-09-10)

- feat(autoapi): add Upsert methods for stacks
  [#5316](https://github.com/pulumi/pulumi/pull/5316)

- Add IsSelectStack404Error and IsCreateStack409Error
  [#5314](https://github.com/pulumi/pulumi/pull/5314)

- Add internal scaffolding for cross-language components.
  [#5280](https://github.com/pulumi/pulumi/pull/5280)

- feat(autoapi): add workspace scoped envvars to LocalWorkspace and Stack
  [#5275](https://github.com/pulumi/pulumi/pull/5275)

- refactor(autoapi-gitrepo): use Workspace in SetupFn callback
  [#5279](https://github.com/pulumi/pulumi/pull/5279)

- Fix Go SDK plugin acquisition for programs with vendored dependencies
  [#5286](https://github.com/pulumi/pulumi/pull/5286)

- Python SDK: Add support for `Sequence[T]` for array types
  [#5282](https://github.com/pulumi/pulumi/pull/5282)

- feat(autoapi): Add support for non default secret providers in local workspaces
  [#5320](https://github.com/pulumi/pulumi/pull/5320)

- .NET SDK: Prevent a task completion race condition
  [#5324](https://github.com/pulumi/pulumi/pull/5324)

## 2.9.2 (2020-08-31)

- Alpha version of the Automation API for Go
  [#4977](https://github.com/pulumi/pulumi/pull/4977)

- Python SDK: Avoid raising an error when internal properties don't match the
  expected type.
  [#5251](https://github.com/pulumi/pulumi/pull/5251)

- Added `--suppress-permalink` option to suppress the permalink output
  (fixes [#4103](https://github.com/pulumi/pulumi/issues/4103))
  [#5191](https://github.com/pulumi/pulumi/pull/5191)

## 2.9.1 (2020-08-27)

- Python SDK: Avoid raising an error when an output has a type annotation of Any
  and the value is a list or dict.
  [#5238](https://github.com/pulumi/pulumi/pull/5238)

## 2.9.0 (2020-08-19)

- Fix support for CheckFailures in Python Dynamic Providers
  [#5138](https://github.com/pulumi/pulumi/pull/5138)

- Upgrade version of `gocloud.dev`. This ensures that 'AWSKMS' secrets
  providers can now be used with full ARNs rather than just Aliases
  [#5138](https://github.com/pulumi/pulumi/pull/5138)

- Ensure the 'history' command is a subcommand of 'stack'.
  This means that `pulumi history` has been deprecated in favour
  of `pulumi stack history`.
  [#5158](https://github.com/pulumi/pulumi/pull/5158)

- Add support for extracting jar files in archive resources
  [#5150](https://github.com/pulumi/pulumi/pull/5150)

- SDK changes to support Python input/output classes
  [#5033](https://github.com/pulumi/pulumi/pull/5033)

## 2.8.2 (2020-08-07)

- Add nuget badge to README [#5117](https://github.com/pulumi/pulumi/pull/5117)

- Support publishing and consuming Policy Packs using any runtime
  [#5102](https://github.com/pulumi/pulumi/pull/5102)

- Fix regression where any CLI integration for any stack with a default
  secrets provider would sort the config alphabetically and new stacks created
  would get created with an empty map `{}` in the config file
  [#5132](https://github.com/pulumi/pulumi/pull/5132)

## 2.8.1 (2020-08-05)

- Fix a bug where passphrase managers were not being
  recognised correctly when getting the configuration
  for the current stack.
  **Please Note:**
  This specific bug may have caused the stack config
  file to remove the password encryption salt.
  [#5110](https://github.com/pulumi/pulumi/pull/5110)

## 2.8.0 (2020-08-04)

- Add missing MapMap and ArrayArray types to Go SDK
  [#5092](https://github.com/pulumi/pulumi/pull/5092)

- Switch os/user package with luser drop in replacement
  [#5065](https://github.com/pulumi/pulumi/pull/5065)

- Update pip/setuptools/wheel in virtual environment before installing dependencies
  [#5042](https://github.com/pulumi/pulumi/pull/5042)

- Add ability to change a secrets provider for the current stack
  [#5031](https://github.com/pulumi/pulumi/pull/5031)

- Add ability to create a stack based on the config from an existing stack
  [#5062](https://github.com/pulumi/pulumi/pull/5062)

- Python: Improved error message when `virtualenv` doesn't exist
  [#5069](https://github.com/pulumi/pulumi/pull/5069)

- Enable pushing to Artifact Registry in actions
  [#5075](https://github.com/pulumi/pulumi/pull/5075)

## 2.7.1 (2020-07-22)

- Fix logic to parse pulumi venv on github action
  [5038](https://github.com/pulumi/pulumi/pull/5038)

## 2.7.0 (2020-07-22)

- Add pluginDownloadURL field to package definition
  [#4947](https://github.com/pulumi/pulumi/pull/4947)

- Add support for streamInvoke during update
  [#4990](https://github.com/pulumi/pulumi/pull/4990)

- Add ability to copy configuration values between stacks
  [#4971](https://github.com/pulumi/pulumi/pull/4971)

- Add logic to parse pulumi venv on github action
  [#4994](https://github.com/pulumi/pulumi/pull/4994)

- Better performance for stacks with many resources using the .NET SDK
  [#5015](https://github.com/pulumi/pulumi/pull/5015)

- Output PDB files and enable SourceLink integration for .NET assemblies
  [#4967](https://github.com/pulumi/pulumi/pull/4967)

## 2.6.1 (2020-07-09)

- Fix a panic in the display during CLI operations
  [#4987](https://github.com/pulumi/pulumi/pull/4987)

## 2.6.0 (2020-07-08)

- Go program gen: Improved handling for pulumi.Map types
  [#4914](https://github.com/pulumi/pulumi/pull/4914)

- Go SDK: Input type interfaces should declare pointer type impls where appropriate
  [#4911](https://github.com/pulumi/pulumi/pull/4911)

- Fixes issue where base64-encoded GOOGLE_CREDENTIALS causes problems with other commands
  [#4972](https://github.com/pulumi/pulumi/pull/4972)

## 2.5.0 (2020-06-25)

- Go program gen: prompt array conversion, unused range vars, id handling
  [#4884](https://github.com/pulumi/pulumi/pull/4884)

- Go program gen handling for prompt optional primitives
  [#4875](https://github.com/pulumi/pulumi/pull/4875)

- Go program gen All().Apply rewriter
  [#4858](https://github.com/pulumi/pulumi/pull/4858)

- Go program gen improvements (multiline strings, get/lookup disambiguation, invoke improvements)
  [#4850](https://github.com/pulumi/pulumi/pull/4850)

- Go program gen improvements (splat, all, index, traversal, range)
  [#4831](https://github.com/pulumi/pulumi/pull/4831)

- Go program gen improvements (resource range, readDir, fileArchive)
  [#4818](https://github.com/pulumi/pulumi/pull/4818)

- Set default config namespace for Get/Try/Require methods in Go SDK.
  [#4802](https://github.com/pulumi/pulumi/pull/4802)

- Handle invalid UTF-8 characters before RPC calls
  [#4816](https://github.com/pulumi/pulumi/pull/4816)

- Improve typing for Go SDK secret config values
  [#4800](https://github.com/pulumi/pulumi/pull/4800)

- Fix panic on `pulumi up` prompt after preview when filtering and hitting arrow keys.
  [#4808](https://github.com/pulumi/pulumi/pull/4808)

- Ensure GitHub Action authenticates to GCR when `$GOOGLE_CREDENTIALS` specified
  [#4812](https://github.com/pulumi/pulumi/pull/4812)

- Fix `pylint(no-member)` when accessing `resource.id`.
  [#4813](https://github.com/pulumi/pulumi/pull/4813)

- Fix GitHub Actions environment detection for PRs.
  [#4817](https://github.com/pulumi/pulumi/pull/4817)

- Adding language sdk specific docker containers.
  [#4837](https://github.com/pulumi/pulumi/pull/4837)

- Workaround bug in grcpio v1.30.0 by excluding this version from required dependencies.
  [#4883](https://github.com/pulumi/pulumi/pull/4883)

## 2.4.0 (2020-06-10)
- Turn program generation NYIs into diagnostic errors
  [#4794](https://github.com/pulumi/pulumi/pull/4794)

- Improve dev version detection logic
  [#4732](https://github.com/pulumi/pulumi/pull/4732)

- Export `CustomTimeouts` in the Python SDK
  [#4747](https://github.com/pulumi/pulumi/pull/4747)

- Add GitHub Actions CI detection
  [#4758](https://github.com/pulumi/pulumi/pull/4758)

- Allow users to specify base64 encoded strings as GOOGLE_CREDENTIALS
  [#4773](https://github.com/pulumi/pulumi/pull/4773)

- Install and use dependencies automatically for new Python projects.
  [#4775](https://github.com/pulumi/pulumi/pull/4775)

## 2.3.0 (2020-05-27)
- Add F# operators for InputUnion.
  [#4699](https://github.com/pulumi/pulumi/pull/4699)

- Add support for untagged outputs in Go SDK.
  [#4640](https://github.com/pulumi/pulumi/pull/4640)

- Update go-cloud to support all Azure regions
  [#4643](https://github.com/pulumi/pulumi/pull/4643)

- Fix a Regression in .NET unit testing.
  [#4656](https://github.com/pulumi/pulumi/pull/4656)

- Allow `pulumi.export` calls from Python unit tests.
  [#4670](https://github.com/pulumi/pulumi/pull/4670)

- Add support for publishing Python policy packs.
  [#4644](https://github.com/pulumi/pulumi/pull/4644)

- Improve download perf by fetching plugins from a CDN.
  [#4692](https://github.com/pulumi/pulumi/pull/4692)

## 2.2.1 (2020-05-13)
- Add new brew target to fix homebrew builds
  [#4633](https://github.com/pulumi/pulumi/pull/4633)

## 2.2.0 (2020-05-13)

- Fixed ResourceOptions issue with stack references in Python SDK
  [#4553](https://github.com/pulumi/pulumi/pull/4553)

- Add runTask to F# Deployment module
  [#3858](https://github.com/pulumi/pulumi/pull/3858)

- Add support for generating Fish completions
  [#4401](https://github.com/pulumi/pulumi/pull/4401)

- Support map-typed inputs in RegisterResource for Go SDK
  [#4522](https://github.com/pulumi/pulumi/pull/4522)

- Don't call IMocks.NewResourceAsync for the root stack resource
  [#4527](https://github.com/pulumi/pulumi/pull/4527)

- Add ResourceOutput type to Go SDK
  [#4575](https://github.com/pulumi/pulumi/pull/4575)

- Allow secrets to be decrypted when exporting a stack
  [#4046](https://github.com/pulumi/pulumi/pull/4046)

- Commands checking for a confirmation or requiring a `--yes` flag can now be
  skipped by setting `PULUMI_SKIP_CONFIRMATIONS` to `1` or `true`.
  [#4477](https://github.com/pulumi/pulumi/pull/4477)

## 2.1.1 (2020-05-11)

- Add retry support when writing to state buckets
  [#4494](https://github.com/pulumi/pulumi/pull/4494)

## 2.1.0 (2020-04-28)

- Fix infinite recursion bug for Go SDK
  [#4516](https://github.com/pulumi/pulumi/pull/4516)

- Order secretOutputNames when used in stack references
  [#4489](https://github.com/pulumi/pulumi/pull/4489)

- Add support for a `PULUMI_CONSOLE_DOMAIN` environment variable to override the
  behavior for how URLs to the Pulumi Console are generated.
  [#4410](https://github.com/pulumi/pulumi/pull/4410)

- Protect against panic when unprotecting non-existant resources
  [#4441](https://github.com/pulumi/pulumi/pull/4441)

- Add flag to `pulumi stack` to output only the stack name
  [#4450](https://github.com/pulumi/pulumi/pull/4450)

- Ensure Go accessor methods correctly support nested fields of optional outputs
  [#4456](https://github.com/pulumi/pulumi/pull/4456)

- Improve `ResourceOptions.merge` type in Python SDK
  [#4484](https://github.com/pulumi/pulumi/pull/4484)

- Ensure generated Python module names are keyword-safe.
  [#4473](https://github.com/pulumi/pulumi/pull/4473)

- Explicitly set XDG_CONFIG_HOME and XDG_CACHE_HOME env vars for helm in the
  pulumi docker image
  [#4474](https://github.com/pulumi/pulumi/pull/4474)

- Increase the MaxCallRecvMsgSize for all RPC calls.
  [#4455](https://github.com/pulumi/pulumi/pull/4455)

## 2.0.0 (2020-04-16)

- CLI behavior change.  Commands in non-interactive mode (i.e. when `pulumi` has its output piped to
  another process or running on CI) will not default to assuming that `--yes` was passed in.  `--yes` is now
  explicitly required to proceed in non-interactive scenarios. This affects:
  * `pulumi destroy`
  * `pulumi new`
  * `pulumi refresh`
  * `pulumi up`

- Fixed [crashes and hangs](https://github.com/pulumi/pulumi/issues/3528) introduced by usage of
  another library.

- @pulumi/pulumi now requires Node.js version >=10.10.0.

- All data-source invocations are now asynchronous (Promise-returning) by default.

- C# code generation switched to schema.

- .NET API: replace `IDeployment` interface with `DeploymentInstance` class.

- Fix Go SDK secret propagation for Resource inputs/outputs.
  [#4387](https://github.com/pulumi/pulumi/pull/4387)

- Fix Go codegen to emit config packages
  [#4388](https://github.com/pulumi/pulumi/pull/4388)

- Treat config values set with `--path` that start with '0' as strings rather than numbers.
  [#4393](https://github.com/pulumi/pulumi/pull/4393)

- Switch .NET projects to .NET Core 3.1
  [#4400](https://github.com/pulumi/pulumi/pull/4400)

- Avoid unexpected replace on resource with `import` applied on second update.
  [#4403](https://github.com/pulumi/pulumi/pull/4403)

## 1.14.1 (2020-04-13)
- Propagate `additionalSecretOutputs` opt to Read in NodeJS.
  [#4307](https://github.com/pulumi/pulumi/pull/4307)

- Fix handling of `nil` values in Outputs in Go.
  [#4268](https://github.com/pulumi/pulumi/pull/4268)

- Include usage hints for Input types in Go SDK
  [#4279](https://github.com/pulumi/pulumi/pull/4279)

- Fix secretness propagation in Python `apply`.
  [#4273](https://github.com/pulumi/pulumi/pull/4273)

- Fix the `call` mock in Python.
  [#4274](https://github.com/pulumi/pulumi/pull/4274)

- Fix handling of secret values in mock-based tests.
  [#4272](https://github.com/pulumi/pulumi/pull/4272)

- Automatic plugin acquisition for Go
  [#4297](https://github.com/pulumi/pulumi/pull/4297)

- Define merge behavior for resource options in Go SDK
  [#4316](https://github.com/pulumi/pulumi/pull/4316)

- Add overloads to Output.All in .NET
  [#4321](https://github.com/pulumi/pulumi/pull/4321)

- Make prebuilt executables opt-in only for the Go SDK
  [#4338](https://github.com/pulumi/pulumi/pull/4338)

- Support the `binary` option (prebuilt executables) for the .NET SDK
  [#4355](https://github.com/pulumi/pulumi/pull/4355)

- Add helper methods for stack outputs in the Go SDK
  [#4341](https://github.com/pulumi/pulumi/pull/4341)

- Add additional overloads to Deployment.RunAsync in .NET API.
  [#4286](https://github.com/pulumi/pulumi/pull/4286)

- Automate execution of `go mod download` for `pulumi new` Go templates
  [#4353](https://github.com/pulumi/pulumi/pull/4353)

- Fix `pulumi up -r -t $URN` not refreshing only the target
  [#4217](https://github.com/pulumi/pulumi/pull/4217)

- Fix logout with file backend when state is deleted
  [#4218](https://github.com/pulumi/pulumi/pull/4218)

- Fix specific flags for `pulumi stack` being global
  [#4294](https://github.com/pulumi/pulumi/pull/4294)

- Fix error when setting config without value in non-interactive mode
  [#4358](https://github.com/pulumi/pulumi/pull/4358)

- Propagate unknowns in Go SDK during marshal operations
  [#4369](https://github.com/pulumi/pulumi/pull/4369/files)

- Fix Go SDK stack reference helpers to handle nil values
  [#4370](https://github.com/pulumi/pulumi/pull/4370)

- Fix propagation of unknown status for secrets
  [#4377](https://github.com/pulumi/pulumi/pull/4377)

## 1.14.0 (2020-04-01)
- Fix error related to side-by-side versions of `@pulumi/pulumi`.
  [#4235](https://github.com/pulumi/pulumi/pull/4235)

- Allow users to specify an alternate backend URL when using the GitHub Actions container with the env var `PULUMI_BACKEND_URL`.
  [#4243](https://github.com/pulumi/pulumi/pull/4243)

## 1.13.1 (2020-03-27)
- Move to a multi-module repo to enable modules for the Go SDK
  [#4109](https://github.com/pulumi/pulumi/pull/4109)

- Report compile time errors for Go programs during plugin acquisition.
  [#4141](https://github.com/pulumi/pulumi/pull/4141)

- Add missing builtin `MapArray` to Go SDK.
  [#4144](https://github.com/pulumi/pulumi/pull/4144)

- Add aliases to Go SDK codegen pkg.
  [#4157](https://github.com/pulumi/pulumi/pull/4157)

- Discontinue testing on Node 8 (which has been end-of-life since January 2020), and start testing on Node 13.
  [#4156](https://github.com/pulumi/pulumi/pull/4156)

- Add support for enabling Policy Packs with configuration.
  [#3756](https://github.com/pulumi/pulumi/pull/4127)

- Remove obsolete .NET serialization attributes.
  [#4190](https://github.com/pulumi/pulumi/pull/4190)

- Add support for validating Policy Pack configuration.
  [#4179](https://github.com/pulumi/pulumi/pull/4186)

## 1.13.0 (2020-03-18)
- Add support for plugin acquisition for Go programs
  [#4060](https://github.com/pulumi/pulumi/pull/4060)

- Display resource type in PAC violation output
  [#4061](https://github.com/pulumi/pulumi/issues/4061)

- Update to Helm v3 in pulumi Docker image
  [#4090](https://github.com/pulumi/pulumi/pull/4090)

- Add ArrayMap builtin types to Go SDK
  [#4086](https://github.com/pulumi/pulumi/pull/4086)

- Improve documentation of URL formats for `pulumi login`
  [#4059](https://github.com/pulumi/pulumi/pull/4059)

- Add support for stack transformations in the .NET SDK.
  [#4008](https://github.com/pulumi/pulumi/pull/4008)

- Fix `pulumi stack ls` on Windows
  [#4094](https://github.com/pulumi/pulumi/pull/4094)

- Add support for running Python policy packs.
  [#4057](https://github.com/pulumi/pulumi/pull/4057)

## 1.12.1 (2020-03-11)
- Fix Kubernetes YAML parsing error in .NET.
  [#4023](https://github.com/pulumi/pulumi/pull/4023)

- Avoid projects beginning with `Pulumi` to stop cyclic imports
  [#4013](https://github.com/pulumi/pulumi/pull/4013)

- Ensure we can locate Go created application binaries on Windows
  [#4030](https://github.com/pulumi/pulumi/pull/4030)

- Ensure Python overlays work as part of our SDK generation
  [#4043](https://github.com/pulumi/pulumi/pull/4043)

- Fix terminal gets into a state where UP/DOWN don't work with prompts.
  [#4042](https://github.com/pulumi/pulumi/pull/4042)

- Ensure old provider is not used when configuration has changed
  [#4051](https://github.com/pulumi/pulumi/pull/4051)

- Support for unit testing and mocking in the .NET SDK.
  [#3696](https://github.com/pulumi/pulumi/pull/3696)

## 1.12.0 (2020-03-04)
- Avoid Configuring providers which are not used during preview.
  [#4004](https://github.com/pulumi/pulumi/pull/4004)

- Fix missing module import on Windows platform.
  [#3983](https://github.com/pulumi/pulumi/pull/3983)

- Add support for mocking the resource monitor to the NodeJS and Python SDKs.
  [#3738](https://github.com/pulumi/pulumi/pull/3738)

- Reinstate caching of TypeScript compilation.
  [#4007](https://github.com/pulumi/pulumi/pull/4007)

- Remove the need to set PULUMI_EXPERIMENTAL to use the policy and watch commands.
  [#4001](https://github.com/pulumi/pulumi/pull/4001)

- Fix type annotations for `Output.all` and `Output.concat` in Python SDK.
  [#4016](https://github.com/pulumi/pulumi/pull/4016)

- Add support for configuring policies.
  [#4015](https://github.com/pulumi/pulumi/pull/4015)

## 1.11.1 (2020-02-26)
- Fix a regression for CustomTimeouts in Python SDK.
  [#3964](https://github.com/pulumi/pulumi/pull/3964)

- Avoid panic when displaying failed stack policies.
  [#3960](https://github.com/pulumi/pulumi/pull/3960)

- Add support for secrets in the Go SDK.
  [3938](https://github.com/pulumi/pulumi/pull/3938)

- Add support for transformations in the Go SDK.
  [3978](https://github.com/pulumi/pulumi/pull/3938)

## 1.11.0 (2020-02-19)
- Allow oversize protocol buffers for Python SDK.
  [#3895](https://github.com/pulumi/pulumi/pull/3895)

- Avoid duplicated messages in preview/update progress display.
  [#3890](https://github.com/pulumi/pulumi/pull/3890)

- Improve CPU utilization in the Python SDK when waiting for resource operations.
  [#3892](https://github.com/pulumi/pulumi/pull/3892)

- Expose resource options, parent, dependencies, and provider config to policies.
  [#3862](https://github.com/pulumi/pulumi/pull/3862)

- Move .NET SDK attributes to the root namespace.
  [#3902](https://github.com/pulumi/pulumi/pull/3902)

- Support exporting older stack versions.
  [#3906](https://github.com/pulumi/pulumi/pull/3906)

- Disable interactive progress display when no terminal size is available.
  [#3936](https://github.com/pulumi/pulumi/pull/3936)

- Mark `ResourceOptions` class as abstract in the .NET SDK. Require the use of derived classes.
  [#3943](https://github.com/pulumi/pulumi/pull/3943)

## 1.10.1 (2020-02-06)
- Support stack references in the Go SDK.
  [#3829](https://github.com/pulumi/pulumi/pull/3829)

- Fix the Windows release process.
  [#3875](https://github.com/pulumi/pulumi/pull/3875)

## 1.10.0 (2020-02-05)
- Avoid writing checkpoints to backend storage in common case where no changes are being made.
  [#3860](https://github.com/pulumi/pulumi/pull/3860)

- Add information about an in-flight operation to the stack command output, if applicable.
  [#3822](https://github.com/pulumi/pulumi/pull/3822)

- Update `SummaryEvent` to include the actual name and local file path for locally-executed policy packs.

- Add support for aliases in the Go SDK
  [3853](https://github.com/pulumi/pulumi/pull/3853)

- Fix Python Dynamic Providers on Windows.
  [#3855](https://github.com/pulumi/pulumi/pull/3855)

## 1.9.1 (2020-01-27)
- Fix a stack reference regression in the Python SDK.
  [#3798](https://github.com/pulumi/pulumi/pull/3798)

- Fix a buggy assertion in the Go SDK.
  [#3794](https://github.com/pulumi/pulumi/pull/3794)

- Add `--latest` flag to `pulumi policy enable`.

- Breaking change for Policy which removes requirement for version when running `pulumi policy disable`. Add `--version` flag if user wants to specify version of Policy Pack to disable.

- Fix rendering of Policy Packs to ensure they are always displayed.

- Primitive input types in the Go SDK (e.g. Int, String, etc.) now implement the corresponding Ptr type e.g. IntPtr,
  StringPtr, etc.). This is consistent with the output of the Go code generator and is much more ergonomic for
  optional inputs than manually converting to pointer types.
  [#3806](https://github.com/pulumi/pulumi/pull/3806)

- Add ability to specify all versions when removing a Policy Pack.

- Breaking change to Policy command: Change enable command to use `pulumi policy enable <org-name>/<policy-pack-name> latest` instead of a `--latest` flag.

## 1.9.0 (2020-01-22)
- Publish python types for PEP 561
  [#3704](https://github.com/pulumi/pulumi/pull/3704)

- Lock dep ts-node to v8.5.4
  [#3733](https://github.com/pulumi/pulumi/pull/3733)

- Improvements to `pulumi policy` functionality. Add ability to remove & disable Policy Packs.

- Breaking change for Policy which is in Public Preview: Change `pulumi policy apply` to `pulumi policy enable`, and allow users to specify the Policy Group.

- Add Permalink to output when publishing a Policy Pack.

- Add `pulumi policy ls` and `pulumi policy group ls` to list Policy related resources.

- Add `BuildNumber` to CI vars and backend metadata property bag for CI systems that have separate ID and a user-friendly number. [#3766](https://github.com/pulumi/pulumi/pull/3766)

- Breaking changes for the Go SDK. Complete details are in [#3506](https://github.com/pulumi/pulumi/pull/3506).

## 1.8.1 (2019-12-20)

- Fix a panic in `pulumi stack select`.
  [#3687](https://github.com/pulumi/pulumi/pull/3687)

## 1.8.0 (2019-12-19)

- Update version of TypeScript used by Pulumi to `3.7.3`.
  [#3627](https://github.com/pulumi/pulumi/pull/3627)

- Add support for GOOGLE_CREDENTIALS when using Google Cloud Storage backend.
  [#2906](https://github.com/pulumi/pulumi/pull/2906)

  ```sh
   export GOOGLE_CREDENTIALS="$(cat ~/service-account-credentials.json)"
   pulumi login gs://my-bucket
  ```


- Support for using `Config`, `getProject()`, `getStack()`, and `isDryRun()` from Policy Packs.
  [#3612](https://github.com/pulumi/pulumi/pull/3612)

- Top-level Stack component in the .NET SDK.
  [#3618](https://github.com/pulumi/pulumi/pull/3618)

- Add the .NET Core 3.0 runtime to the `pulumi/pulumi` container.
  [#3616](https://github.com/pulumi/pulumi/pull/3616)

- Add `pulumi preview` support for `--refresh`, `--target`, `--replace`, `--target-replace` and
  `--target-dependents` to align with `pulumi up`.
  [#3675](https://github.com/pulumi/pulumi/pull/3675)

- `ComponentResource`s now have built-in support for asynchronously constructing their children.
  [#3676](https://github.com/pulumi/pulumi/pull/3676)

- `Output.apply` (for the JS, Python and .Net sdks) has updated semantics, and will lift dependencies from inner Outputs to the returned Output.
  [#3663](https://github.com/pulumi/pulumi/pull/3663)

- Fix bug in determining PRNumber and BuildURL for an Azure Pipelines CI environment.
  [#3677](https://github.com/pulumi/pulumi/pull/3677)

- Improvements to `pulumi policy` functionality. Add ability to remove & disable Policy Packs.

- Breaking change for Policy which is in Public Preview: Change `pulumi policy apply` to `pulumi policy enable`, and allow users to specify the Policy Group.

## 1.7.1 (2019-12-13)

- Fix [SxS issue](https://github.com/pulumi/pulumi/issues/3652) introduced in 1.7.0 when assigning
  `Output`s across different versions of the `@pulumi/pulumi` SDK.
  [#3658](https://github.com/pulumi/pulumi/pull/3658)

## 1.7.0 (2019-12-11)

- A Pulumi JavaScript/TypeScript program can now consist of a single exported top level function. This
  allows for an easy approach to create a Pulumi program that needs to perform `async`/`await`
  operations at the top-level.
  [#3321](https://github.com/pulumi/pulumi/pull/3321)

  ```ts
  // JavaScript
  module.exports = async () => {
  }

  //TypeScript
  export = async () => {
  }
  ```

## 1.6.1 (2019-11-26)

- Support passing a parent and providers for `ReadResource`, `RegisterResource`, and `Invoke` in the go SDK. [#3563](https://github.com/pulumi/pulumi/pull/3563)

- Fix go SDK ReadResource.
  [#3581](https://github.com/pulumi/pulumi/pull/3581)

- Fix go SDK DeleteBeforeReplace.
  [#3572](https://github.com/pulumi/pulumi/pull/3572)

- Support for setting the `PULUMI_PREFER_YARN` environment variable to opt-in to using `yarn` instead of `npm` for
  installing Node.js dependencies.
  [#3556](https://github.com/pulumi/pulumi/pull/3556)

- Fix regression that prevented relative paths passed to `--policy-pack` from working.
  [#3565](https://github.com/pulumi/pulumi/issues/3564)

## 1.6.0 (2019-11-20)

- Support for config.GetObject and related variants for Golang.
  [#3526](https://github.com/pulumi/pulumi/pull/3526)

- Add support for IgnoreChanges in the go SDK.
  [#3514](https://github.com/pulumi/pulumi/pull/3514)

- Support for a `go run` style workflow. Building or installing a pulumi program written in go is
  now optional.
  [#3503](https://github.com/pulumi/pulumi/pull/3503)

- Re-apply "propagate resource inputs to resource state during preview, including first-class unknown values." The new
  set of changes have additional fixes to ensure backwards compatibility with earlier code. This allows the preview to
  better estimate the state of a resource after an update, including property values that were populated using defaults
  calculated by the provider.
  [#3327](https://github.com/pulumi/pulumi/pull/3327)

- Validate StackName when passing a non-default secrets provider to `pulumi stack init`

- Add support for go1.13.x

- `pulumi update --target` and `pulumi destroy --target` will both error if they determine a
  dependent resource needs to be updated, destroyed, or created that was not was specified in the
  `--target` list.  To proceed with an `update/destroy` after this error, either specify all the
  reported resources as `--target`s, or pass the `--target-dependents` flag to allow necessary
  changes to unspecified dependent targets.

- Support for node 13.x, building with gcc 8 and newer.
  [#3512] (https://github.com/pulumi/pulumi/pull/3512)

- Codepaths which could result in a hang will print a message to the console indicating the problem, along with a link
  to documentation on how to restructure code to best address it.

### Compatibility

- `StackReference.getOutputSync` and `requireOutputSync` are deprecated as they may cause hangs on
  some combinations of Node and certain OS platforms. `StackReference.getOutput` and `requireOutput`
  should be used instead.

## 1.5.2 (2019-11-13)

- `pulumi policy publish` now determines the Policy Pack name from the Policy Pack, and the
  the `org-name` CLI argument is now optional. If not specified; the current user account is
  used.
  [#3459](https://github.com/pulumi/pulumi/pull/3459)

- Refactor the Output API in the Go SDK.
  [#3496](https://github.com/pulumi/pulumi/pull/3496)

## 1.5.1 (2019-11-06)

- Include the .NET language provider in the Windows SDK.

## 1.5.0 (2019-11-06)

- Gracefully handle errors when resources use duplicate aliases.

- Use the update token for renew_lease calls and update the API version to 5.
  [#3348](https://github.com/pulumi/pulumi/pull/3348)

- Improve startup time performance by 0.5-1s by checking for a newer CLI release in parallel.
  [#3441](https://github.com/pulumi/pulumi/pull/3441)

- Add an experimental `pulumi watch` command.
  [#3391](https://github.com/pulumi/pulumi/pull/3391)

## 1.4.1 (2019-11-01)

- Adds a **preview** of .NET support for Pulumi. This code is an preview state and is subject
  to change at any point.

- Fix another colorizer issue that could cause garbled output for messages that did not end in colorization tags.
  [#3417](https://github.com/pulumi/pulumi/pull/3417)

- Verify deployment integrity during import and issue an error if verification fails. The state file can still be
  imported by passing the `--force` flag.
  [#3422](https://github.com/pulumi/pulumi/pull/3422)

- Omit unknowns in resources in stack outputs during preview.
  [#3427](https://github.com/pulumi/pulumi/pull/3427)

- `pulumi update` can now be instructed that a set of resources should be replaced by adding a
  `--replace urn` argument.  Multiple resources can be specified using `--replace urn1 --replace urn2`. In order to
  replace exactly one resource and leave other resources unchanged, invoke `pulumi update --replace urn --target urn`,
  or `pulumi update --target-replace urn` for short.
  [#3418](https://github.com/pulumi/pulumi/pull/3418)

- `pulumi stack` now renders the stack as a tree view.
  [#3430](https://github.com/pulumi/pulumi/pull/3430)

- Support for lists and maps in config.
  [#3342](https://github.com/pulumi/pulumi/pull/3342)

- `ResourceProvider#StreamInvoke` implemented, will be the basis for streaming
  APIs in `pulumi query`.
  [#3424](https://github.com/pulumi/pulumi/pull/3424)

## 1.4.0 (2019-10-24)

- `FileAsset` in the Python SDK now accepts anything implementing `os.PathLike` in addition to `str`.
  [#3368](https://github.com/pulumi/pulumi/pull/3368)

- Fix colorization on Windows 10, and fix a colorizer bug that could cause garbled output for resources with long
  status messages.
  [#3385](https://github.com/pulumi/pulumi/pull/3385)

## 1.3.4 (2019-10-18)

- Remove unintentional console outupt introduced in 1.3.3.

## 1.3.3 (2019-10-17)

- Fix an issue with first-class providers introduced in 1.3.2.

## 1.3.2 (2019-10-16)

- Fix hangs and crashes related to use of `getResource` (i.e. `aws.ec2.getSubnetIds(...)`) methods,
  including frequent hangs on Node.js 12. This fixes https://github.com/pulumi/pulumi/issues/3260)
  and [hangs](https://github.com/pulumi/pulumi/issues/3309).

  Some less common existing styles of using `getResource` calls are also deprecated as part of this
  change, and users should see https://www.pulumi.com/docs/troubleshooting/#synchronous-call for
  details on adjusting their code if needed.

## 1.3.1 (2019-10-09)

- Revert "propagate resource inputs to resource state during preview". These changes had a critical issue that needs
  further investigation.

## 1.3.0 (2019-10-09)

- Propagate resource inputs to resource state during preview, including first-class unknown values. This allows the
  preview to better estimate the state of a resource after an update, including property values that were populated
  using defaults calculated by the provider.
  [#3245](https://github.com/pulumi/pulumi/pull/3245)

- Fetch version information from the Homebrew JSON API for CLIs installed using `brew`.
  [#3290](https://github.com/pulumi/pulumi/pull/3290)

- Support renaming stack projects via `pulumi stack rename`.
  [#3292](https://github.com/pulumi/pulumi/pull/3292)

- Add `helm` to `pulumi/pulumi` Dockerhub container
  [#3294](https://github.com/pulumi/pulumi/pull/3294)

- Make the location of `.pulumi` folder configurable with an environment variable.
  [#3300](https://github.com/pulumi/pulumi/pull/3300) (Fixes [#2966](https://github.com/pulumi/pulumi/issues/2966))

- `pulumi update` can now be scoped to update a single resource by adding a `--target urn` or `-t urn`
  argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- Adds the ability to provide transformations to modify the properties and resource options that
  will be used for any child resource of a component or stack.
  [#3174](https://github.com/pulumi/pulumi/pull/3174)

- Add resource transformations support in Python. [#3319](https://github.com/pulumi/pulumi/pull/3319)

## 1.2.0 (2019-09-26)

- Support emitting high-level execution trace data to a file and add a debug-only command to view trace data.
  [#3238](https://github.com/pulumi/pulumi/pull/3238)

- Fix parsing of GitLab urls with subgroups.
  [#3239](https://github.com/pulumi/pulumi/pull/3239)

- `pulumi refresh` can now be scoped to refresh a subset of resources by adding a `--target urn` or
  `-t urn` argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- `pulumi destroy` can now be scoped to delete a single resource (and its dependents) by adding a
  `--target urn` or `-t urn` argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- Avoid re-encrypting secret values on each checkpoint write. These changes should improve update times for stacks
  that contain secret values.
  [#3183](https://github.com/pulumi/pulumi/pull/3183)

- Add Codefresh CI detection.

- Add `-c` (config array) flag to the `preview` command.

## 1.1.0 (2019-09-11)

- Fix a bug that caused the Python runtime to ignore unhandled exceptions and erroneously report that a Pulumi program executed successfully.
  [#3170](https://github.com/pulumi/pulumi/pull/3170)

- Read operations are no longer considered changes for the purposes of `--expect-no-changes`.
  [#3197](https://github.com/pulumi/pulumi/pull/3197)

- Increase the MaxCallRecvMsgSize for interacting with the gRPC server.
  [#3201](https://github.com/pulumi/pulumi/pull/3201)

- Do not ask for a passphrase in non-interactive sessions (fix [#2758](https://github.com/pulumi/pulumi/issues/2758)).
  [#3204](https://github.com/pulumi/pulumi/pull/3204)

- Support combining the filestate backend (local or remote storage) with the cloud-backed secrets providers (KMS, etc.).
  [#3198](https://github.com/pulumi/pulumi/pull/3198)

- Moved `@pulumi/pulumi` to target `es2016` instead of `es6`.  As `@pulumi/pulumi` programs run
  inside Nodejs, this should not change anything externally as Nodejs already provides es2016
  support. Internally, this makes more APIs available for `@pulumi/pulumi` to use in its implementation.

- Fix the --stack option of the `pulumi new` command.
  ([#3131](https://github.com/pulumi/pulumi/pull/3131) fixes [#2880](https://github.com/pulumi/pulumi/issues/2880))

## 1.0.0 (2019-09-03)

- No significant changes.

## 1.0.0-rc.1 (2019-08-28)

- Print a Welcome to Pulumi message for users during interactive logins to the Pulumi CLI.
  [#3145](https://github.com/pulumi/pulumi/pull/3145)

- Filter the list of templates shown by default during `pulumi new`.
  [#3147](https://github.com/pulumi/pulumi/pull/3147)

## 1.0.0-beta.4 (2019-08-22)

- Fix a crash when using StackReference from the `1.0.0-beta.3` version of
  `@pulumi/pulumi` and `1.0.0-beta.2` or earlier of the CLI.

- Allow Un/MashalProperties to reject Asset and AssetArchive types. (partial fix
  for https://github.com/pulumi/pulumi-kubernetes/issues/737)

## 1.0.0-beta.3 (2019-08-21)

- When using StackReference to fetch output values from another stack, do not mark a value as secret if it was not
  secret in the stack you referenced. (fixes [#2744](https://github.com/pulumi/pulumi/issues/2744)).

- Allow resource IDs to be changed during `pulumi refresh` operations

- Do not crash when renaming a stack that has never been updated, when using the local backend. (fixes
  [#2654](https://github.com/pulumi/pulumi/issues/2654))

- Fix intermittet "NoSuchKey" issues when using the S3 based backend. (fixes [#2714](https://github.com/pulumi/pulumi/issues/2714)).

- Support filting stacks by organization or tags when using `pulumi stack ls`. (fixes [#2712](https://github.com/pulumi/pulumi/issues/),
  [#2769](https://github.com/pulumi/pulumi/issues/2769)

- Explicitly setting `deleteBeforeReplace` to `false` now overrides the provider's decision.
  [#3118](https://github.com/pulumi/pulumi/pull/3118)

- Fail read steps (e.g. the step generated by a call to `aws.s3.Bucket.get()`) if the requested resource does not exist.
  [#3123](https://github.com/pulumi/pulumi/pull/3123)

## 1.0.0-beta.2 (2019-08-13)

- Fix the package version compatibility checks in the NodeJS language host.
  [#3083](https://github.com/pulumi/pulumi/pull/3083)

## 1.0.0-beta.1 (2019-08-13)

- Do not propagate input properties to missing output properties during preview. The old behavior can cause issues that
  are difficult to diagnose in cases where the actual value of the output property differs from the value of the input
  property, and can cause `apply`s to run at unexpected times. If this change causes issues in a Pulumi program, the
  original behavior can be enabled by setting the `PULUMI_ENABLE_LEGACY_APPLY` environment variable to `true`.

- Fix a bug in the GitHub Actions program preventing errors from being rendered in the Actions log on github.com.
  [#3036](https://github.com/pulumi/pulumi/pull/3036)

- Fix a bug in the Node.JS SDK that caused failure details for provider functions to go unreported.
  [#3048](https://github.com/pulumi/pulumi/pull/3048)

- Fix a bug in the Python SDK that caused crashes when using asynchronous data sources.
  [#3056](https://github.com/pulumi/pulumi/pull/3056)

- Fix crash when exporting secrets from a pulumi app
  [#2962](https://github.com/pulumi/pulumi/issues/2962)

- Fix a panic in logger when a secret contains non-printable characters
  [#3074](https://github.com/pulumi/pulumi/pull/3074)

- Check the uniqueness of the project name during pulumi new
  [#3065](https://github.com/pulumi/pulumi/pull/3065)

## 0.17.28 (2019-08-05)

- Retry renaming a temporary folder during plugin installation
  [#3008](https://github.com/pulumi/pulumi/pull/3008)

- Add support for additional Pulumi secrets providers using AWS KMS, Azure KeyVault, Google Cloud
  KMS and HashiCorp Vault.  These secrets providers can be configured at stack creation time using
  `pulumi stack init b --secrets-provider="awskms://alias/LukeTesting?region=us-west-2"`, and ensure
  that all encrypted data associated with the stack is encrypted using the target cloud platform
  encryption keys.  This augments the previous choice between using the app.pulumi.com-managed
  secrets encryption or a fully-client-side local passphrase encryption.
  [#2994](https://github.com/pulumi/pulumi/pull/2994)

- Add `Output.concat` to Python SDK [#3006](https://github.com/pulumi/pulumi/pull/3006)

- Add `requireOutput` to `StackReference` [#3007](https://github.com/pulumi/pulumi/pull/3007)

- Arbitrary values can now be exported from a Python app. This includes dictionaries, lists, class
  instances, and the like. Values are treated as "plain old python data" and generally kept as
  simple values (like strings, numbers, etc.) or the simple collections supported by the Pulumi data model (specifically, dictionaries and lists).

- Fix `get_secret` in Python SDK always returning None.

- Make `pulumi.runtime.invoke` synchronous in the Python SDK [#3019](https://github.com/pulumi/pulumi/pull/3019)

- Fix a bug in the Python SDK that caused input properties that are coroutines to be awaited twice.
  [#3024](https://github.com/pulumi/pulumi/pull/3024)

### Compatibility

- Deprecated functions in `@pulumi/pulumi` will now issue warnings if you call them.  Please migrate
  off of these functions as they will be removed in a future release.  The deprecated functions are.
  1. `function computeCodePaths(extraIncludePaths?: string[], ...)`.  Use the `computeCodePaths`
     overload that takes a `CodePathOptions` instead.
  2. `function serializeFunctionAsync`. Please use `serializeFunction` instead.

## 0.17.27 (2019-07-29)

- Fix an error message from the logging subsystem which was introduced in v0.17.26
  [#2989](https://github.com/pulumi/pulumi/pull/2997)

- Add support for property paths in `ignoreChanges`, and pass `ignoreChanges` to providers
  [#3005](https://github.com/pulumi/pulumi/pull/3005). This allows differences between the actual and desired
  state of the resource that are not captured by differences in the resource's inputs to be ignored (including
  differences that may occur due to resource provider bugs).

## 0.17.26 (2019-07-26)

- Add `get_object`, `require_object`, `get_secret_object` and `require_secret_object` APIs to Python
  `config` module [#2959](https://github.com/pulumi/pulumi/pull/2959)

- Fix unexpected provider replacements when upgrading from older CLIs and older providers
  [pulumi/pulumi-kubernetes#645](https://github.com/pulumi/pulumi-kubernetes/issues/645)

- Add *Python* support for renaming resources via the `aliases` resource option.  Adding aliases
  allows new resources to match resources from previous deployments which used different names,
  maintaining the identity of the resource and avoiding replacements or re-creation of the resource.
  This was previously added to the *JavaScript* sdk in 0.17.15.
  [#2974](https://github.com/pulumi/pulumi/pull/2974)

## 0.17.25 (2019-07-19)

- Support for Dynamic Providers in Python [#2900](https://github.com/pulumi/pulumi/pull/2900)

## 0.17.24 (2019-07-19)

- Fix a crash when two different versions of `@pulumi/pulumi` are used in the same Pulumi program
  [#2942](https://github.com/pulumi/pulumi/issues/2942)

## 0.17.23 (2019-07-16)
- `pulumi new` allows specifying a local path to templates (resolves
  [#2672](https://github.com/pulumi/pulumi/issues/2672))

- Fix an issue where a file archive created on Windows would contain back-slashes
  [#2784](https://github.com/pulumi/pulumi/issues/2784)

- Fix an issue where output values of a resource would not be present when they
  contained secret values, when using Python.

- Fix an issue where emojis are printed in non-interactive mode. (fixes
  [#2871](https://github.com/pulumi/pulumi/issues/2871))

- Promises/Outputs can now be directly exported as the top-level (i.e. not-named) output of a Stack.
  (fixes [#2910](https://github.com/pulumi/pulumi/issues/2910))

- Add support for importing existing resources to be managed using Pulumi. A resource can be imported
  by setting the `import` property in the resource options bag when instantiating a resource. In order to
  successfully import a resource, its desired configuration (i.e. its inputs) must not differ from its
  actual configuration (i.e. its state) as calculated by the resource's provider.
- Better error message for missing npm on `pulumi new` (fixes [#1511](https://github.com/pulumi/pulumi/issues/1511))

- Add the ability to pass a customTimeouts object from the providers across the engine
  to resource management. (fixes [#2655](https://github.com/pulumi/pulumi/issues/2655))

### Breaking Changes

- Defer to resource providers in all cases where the engine must determine whether or not a resource
  has changed. Note that this can expose bugs in the resources providers that cause diffs to be
  present even if the desired configuration matches the actual state of the resource: in these cases,
  users can set the `PULUMI_ENABLE_LEGACY_DIFF` environment variable to `1` or `true` to enable the
  old diff behavior. https://github.com/pulumi/pulumi/issues/2971 lists the known provider bugs
  exposed by these changes and links to appropriate workarounds or tracking issues.

## 0.17.22 (2019-07-11)

- Improve update performance in cases where a large number of log messages are
  reported during an update.

## 0.17.21 (2019-06-26)

- Python SDK fix for a crash resulting from a KeyError if secrets were used in configuration.

- Fix an issue where a secret would not be encrypted in the state file if it was
  a property of a resource which was used as a stack output (fixes
  [#2862](https://github.com/pulumi/pulumi/issues/2862))

## 0.17.20 (2019-06-23)

- SDK fix for crash that could occasionally happen if there were multiple identical aliases to the
  same Resource.

## 0.17.19 (2019-06-23)

- Engine fix for crash that could occasionally happen if there were multiple identical aliases to
  the same Resource.

## 0.17.18 (2019-06-20)

- Allow setting backend URL explicitly in `Pulumi.yaml` file

- `StackReference` now has a `.getOutputSync` function to retrieve exported values from an existing
  stack synchronously.  This can be valuable when creating another stack that wants to base
  flow-control off of the values of an existing stack (i.e. importing the information about all AZs
  and basing logic off of that in a new stack). Note: this only works for importing values from
  Stacks that have not exported `secrets`.

- When the environment variable `PULUMI_TEST_MODE` is set to `true`, the
  Python runtime will now behave as if
  `pulumi.runtime.settings._set_test_mode_enabled(True)` had been called. This
  mirrors the behavior for NodeJS programs (fixes [#2818](https://github.com/pulumi/pulumi/issues/2818)).

- Resources that are only 'read' will no longer be displayed in the terminal tree-display anymore.
  These ended up heavily cluttering the display and often meant that programs without updates still
  showed a bunch of resources that weren't important.  There will still be a message displayed
  indicating that a 'read' has happened to help know that these are going on and that the program is making progress.

## 0.17.17 (2019-06-12)

### Improvements

- docs(login): escape codeblocks, and add object store state instructions
  [#2810](https://github.com/pulumi/pulumi/pull/2810)
- The API for passing along a custom provider to a ComponentResource has been simplified.  You can
  now just say `new SomeComponentResource(name, props, { provider: awsProvider })` instead of `new
  SomeComponentResource(name, props, { providers: { "aws" : awsProvider } })`
- Fix a bug where the path provided to a URL in `pulumi login` is lost are dropped, so if you `pulumi login
  s3://bucketname/afolder`, the Pulumi files will be inside of `s3://bucketname/afolder/.pulumi` rather than
  `s3://bucketname/.pulumi` (thanks, [@bigkraig](https://github.com/bigkraig)!).  **NOTE**: If you have been
  logging in to the s3 backend with a path after the bucket name, you will need to either move the .pulumi
  folder in the bucket to the correct location or log in again without the path prefix to see your previous
  stacks.
- Fix a crash that would happen if you ran `pulumi stack output` against an empty stack (fixes
  [pulumi/pulumi#2792](https://github.com/pulumi/pulumi/issues/2792)).
- Unparented Pulumi `CustomResource`s now support calling `.getProvider(...)` on them.

## 0.17.16 (2019-06-06)

### Improvements

- Fixed a bug that caused an assertion when dealing with unchanged resources across version upgrades.

## 0.17.15 (2019-06-05)

### Improvements

- Pulumi now allows Python programs to "read" existing resources instead of just creating them. This feature enables
  Pulumi Python packages to expose ".get()" methods that allow for reading of resources that already exist.
- Support for referencing the outputs of other Pulumi stacks has been added to the Pulumi Python libraries via the
  `StackReference` type.
- Add CI system detection for Bitbucket Pipelines.
- Pulumi now tolerates changes in default providers in certain cases, which fixes an issue where users would see
  unexpected replaces when upgrading a Pulumi package.
- Add support for renaming resources via the `aliases` resource option.  Adding aliases allows new resources to match
  resources from previous deployments which used different names, maintaining the identity of the resource and avoiding
  replacements or re-creation of the resource.
- `pulumi plugin install` gained a new optional argument `--server` which can be used to provide a custom server to be
  used when downloading a plugin.

## 0.17.14 (2019-05-28)

### Improvements

- `pulumi refresh` now tries to install any missing plugins automatically like
  `pulumi destroy` and `pulumi update` do (fixes [pulumi/pulumi#2669](https://github.com/pulumi/pulumi/issues/2669)).
- `pulumi whoami` now outputs the URL of the currently connected backend.
- Correctly suppress stack outputs when serializing previews to JSON, i.e. `pulumi preview --json --suppress-outputs`.
  Fixes [pulumi/pulumi#2765](https://github.com/pulumi/pulumi/issues/2765).

## 0.17.13 (2019-05-21)

### Improvements

- Fix an issue where creating a first class provider would fail if any of the
  configuration values for the providers were secrets. (fixes [pulumi/pulumi#2741](https://github.com/pulumi/pulumi/issues/2741)).
- Fix an issue where when using `--diff` or looking at details for a proposed
  updated, the CLI might print text like: `<{%reset%}>
  --outputs:--<{%reset%}>` instead of just `--outputs:--`.
- Fixes local login on Windows.  Specifically, windows local paths are properly understood and
  backslashes `\` are not converted to `__5c__` in paths.
- Fix an issue where some operations would fail with `error: could not deserialize deployment: unknown secrets provider type`.
- Fix an issue where pulumi might try to replace existing resources when upgrading to the newest version of some resource providers.

## 0.17.12 (2019-05-15)

### Improvements

- Pulumi now tells you much earlier when the `--secrets-provider` argument to
  `up` `init` or `new` has the wrong value. In addition, supported values are
  now listed in the help text. (fixes [pulumi/pulumi#2727](https://github.com/pulumi/pulumi/issues/2727)).
- Pulumi no longer prompts for your passphrase twice during operations when you
  are using the passphrase based secrets provider. (fixes [pulumi/pulumi#2729](https://github.com/pulumi/pulumi/issues/2729)).
- Fix an issue where complex inputs to a resource which contained secret values
  would not be stored correctly.
- Fix a panic during property diffing when comparing two secret arrays.

## 0.17.11 (2019-05-13)

### Major Changes

#### Secrets and Pluggable Encryption

- The Pulumi engine and Python and NodeJS SDKs now have support for tracking values as "secret" to ensure they are
  encrypted when being persisted in a state file. `[pulumi/pulumi#397](https://github.com/pulumi/pulumi/issues/397)`

  Any existing value may be turned into a secret by calling `pulumi.secret(<value>)` (NodeJS) or
  `Output.secret(<value>`) (Python).  In both cases, the returned value is an output which may be passed around
  like any other.  If this value flows into a resource, the plaintext will not be stored in the state file, but instead
  It will be encrypted, just like values added to config with `pulumi config set --secret`.

  You can verify that values are being stored as you expect by running `pulumi stack export`, When values are encrypted
  in the state file, they appear as an object with a special signature key and a ciphertext property.

  When outputs of a stack are secrets, `pulumi stack output` will show `[secret]` as the value, by default.  You can
  pass `--show-secrets` to `pulumi stack output` in order to see the actual raw value.

- When storing state with the Pulumi Service, you may now elect to use the passphrase based encryption for both secret
  configuration values and values that are encrypted in a state file.  To use this new feature, pass
  `--secrets-provider passphrase` to `pulumi new` or `pulumi stack init` when you initally create the stack. When you
  create the stack, you will be prompted for a passphrase (or if `PULUMI_CONFIG_PASSPHRASE` is set, it will be used).
  This passphrase is used to generate a unique key for your stack, and config values and encrypted state values are
  encrypted using AES-256-GCM. The key is derived from your passphrase, and while information to re-create it when
  provided with your passphrase is stored in both the `Pulumi.<stack-name>.yaml` file and the state file for your stack,
  this information can not be used to recover the key. When using this mode, the Pulumi Service is unable to decrypt
  either your secret configuration values or and secret values in your state file.

  We will be adding gestures to move existing stacks managed by the service to use passphrase based encryption soon
  as well as gestures to change the passphrase for an existing stack.

** Note **

Stacks with encrypted secrets in their state files can only be managed by 0.17.11 or later of the CLI. Attempting
to use a previous version of the CLI with these stacks will result in an error.

Fixes #397

### Improvements

- Add support for Azure Pipelines in CI environment detection.
- Minor fix to how Azure repository information is extracted to allow proper grouping of Azure
  repositories when various remote URLs are used to pull the repository.

## 0.17.10 (2019-05-02)

### Improvements

- Fixes issue introduced in 0.17.9 where local-login broke on Windows due to the new support for
  `s3://`, `azblob://` and `gs://` save locations.
- Minor contributing document improvement.
- Warnings from `npm` about missing description, repository, and license fields in package.json are
  now suppressed when `npm install` is run from `pulumi new` (via `npm install --loglevel=error`).
- Depend on newer version of gRPC package in the NodeJS SDK. This version has
  prebuilt binaries for Node 12, which should make installing `@pulumi/pulumi`
  more reliable when running on Node 12.

## 0.17.9 (2019-04-30)

### Improvements

- `pulumi login` now supports `s3://`, `azblob://` and `gs://` paths (on top of `file://`) for
  storing stack information. These are passed the location of a desired bucket for each respective
  cloud provider (i.e. `pulumi login s3://mybucket`).  Pulumi artifacts (like the
  `xxx.checkpoint.json` file) will then be stored in that bucket.  Credentials for accessing the
  bucket operate in the normal manner for each cloud provider.  i.e. for AWS this can come from the
  environment, or your `.aws/credentials` file, etc.
- The pulumi version update check can be skipped by setting the environment variable
  `PULUMI_SKIP_UPDATE_CHECK` to `1` or `true`.
- Fix an issue where the stack would not be selected when an existing stack is specified when running
  `pulumi new <template> -s <existing-stack>`.
- Add a `--json` flag (`-j` for short) to the `preview` command. This allows basic serialization of a plan,
  including the anticipated set of deployment steps, list of diagnostics messages, and summary information.
  Each step includes deeply serialized information about the resource state and step metadata itself. This
  is part of ongoing work tracked in [pulumi/pulumi#2390](https://github.com/pulumi/pulumi/issues/2390).

## 0.17.8 (2019-04-23)

### Improvements

- Add a new `ignoreChanges` option to resource options to allow specifying a list of properties to
  ignore for purposes of updates or replacements.  [#2657](https://github.com/pulumi/pulumi/pull/2657)
- Fix an engine bug that could lead to incorrect interpretation of the previous state of a resource leading to
  unexpected Update, Replace or Delete operations being scheduled. [#2650]https://github.com/pulumi/pulumi/issues/2650)
- Build/push `pulumi/actions` container to [DockerHub](https://hub.docker.com/r/pulumi/actions) with new SDK releases [#2646](https://github.com/pulumi/pulumi/pull/2646)

## 0.17.7 (2019-04-17)

### Improvements

- A new "test mode" can be enabled by setting the `PULUMI_TEST_MODE` environment variable to
  `true` in either the Node.js or Python SDK. This new mode allows you to unit test your Pulumi programs
  using standard test harnesses, without needing to run the program using the Pulumi CLI. In this mode, limited
  functionality is available, however basic resource object allocation with input properties will work.
  Note that no actual engine operations will occur in this mode, and that you'll need to use the
  `PULUMI_CONFIG`, `PULUMI_NODEJS_PROJECT`, and `PULUMI_NODEJS_STACK` environment variables to control settings
  the CLI would have otherwise managed for you.

## 0.17.6 (2019-04-11)

### Improvements

- `refresh` will now warn instead of returning an error when it notices a resource is in an
  unhealthy state. This is in service of https://github.com/pulumi/pulumi/issues/2633.

## 0.17.5 (2019-04-08)

### Improvements
- Correctly handle the case where we would fail to detect an archive type if the filename included a dot in it. (fixes [pulumi/pulumi#2589](https://github.com/pulumi/pulumi/issues/2589))
- Make `Config`'s constructor's `name` argument optional in Python, for consistency with our Node.js SDK. If it isn't
  supplied, the current project name is used as the default.
- `pulumi logs` will now display log messages from Google Cloud Functions.

## 0.17.4 (2019-03-26)

### Improvements

- Don't print the `error:` prefix when Pulumi exists because of a declined confirmation prompt (fixes [pulumi/pulumi#458](https://github.com/pulumi/pulumi/issues/2070))
- Fix issue where `Outputs` produced by `pulumi.interpolate` might have values which could
  cause validation errors due to them containing the text `<computed>` during previews.

## 0.17.3 (2019-03-26)

### Improvements

- A new command, `pulumi stack rename` was added. This allows you to change the name of an existing stack in a project. Note: When a stack is renamed, the `pulumi.getStack` function in the SDK will now return a new value. If a stack name is used as part of a resource name, the next `pulumi up` will not understand that the old and new resources are logically the same. We plan to support adding aliases to individual resources so you can handle these cases. See [pulumi/pulumi#458](https://github.com/pulumi/pulumi/issues/458) for discussion on this new feature. For now, if you are unwilling to have `pulumi up` create and destroy these resources, you can rename your stack back to the old name. (fixes [pulumi/pulumi#2402](https://github.com/pulumi/pulumi/issues/2402))
- Fix two warnings that were printed when using a dynamic provider about missing method handlers.
- A bug in the previous version of the Pulumi CLI occasionally caused the Pulumi Engine to load the incorrect resource
  plugin when processing an update. This bug has been fixed in 0.17.3 by performing a deterministic selection of the
  best set of plugins available to the engine before starting up. See
- Add support for serializing JavaScript function that capture [BigInts](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt).
- Support serializing arrow-functions with deconstructed parameters.

## 0.17.2 (2019-03-15)

### Improvements

- Show `brew upgrade pulumi` as the upgrade message when the currently running `pulumi` executable
  is running on macOS from the brew install directory.
- Resource diffs that are rendered to the console are now filtered to properties that have semantically-meaningful
  changes where possible.
- `pulumi new` no longer runs an initial deployment after a project is generated for nodejs projects.
  Instead, instructions are printed indicating that `pulumi up` can be used to deploy the project.
- Differences between the state of a refreshed resource and the state described in a Pulumi program are now properly
  detected when using newer providers.
- Differences between a resource's provider-internal properties are no longer displayed in the CLI.
- Pulumi will now install missing plugins on startup. Previously, Pulumi would error if a required plugin was not
  present and a bug in the Pulumi CLI made it common for users using Pulumi in their continuous integration setup to have problems with missing plugins. Starting with 0.17.2, if Pulumi detects that required plugins are missing, it will make an attempt to install the missing plugins before proceeding with the update.

## 0.17.1 (2019-03-06)

### Improvements

- Slight tweak to `Output.apply` signature to help TypeScript infer types better.

## 0.17.0 (2019-03-05)

This update includes several changes to core `@pulumi/pulumi` constructs that will not play nicely
in side-by-side applications that pull in prior versions of this package.  As such, we are rev'ing
the minor version of the package from 0.16 to 0.17.  Recent version of `pulumi` will now detect,
and warn, if different versions of `@pulumi/pulumi` are loaded into the same application.  If you
encounter this warning, it is recommended you move to versions of the `@pulumi/...` packages that
are compatible.  i.e. keep everything on 0.16.x until you are ready to move everything to 0.17.x.

### Improvements

- `Output<T>` now 'lifts' property members from the value it wraps, simplifying common coding patterns.
  For example:

```ts
interface Widget { text: string, x: number, y: number };
var v: Output<Widget>;

var widgetX = v.x;
// `widgetX` has the type Output<number>.
// This is equivalent to writing `v.apply(w => w.x)`
```

Note: this 'lifting' only occurs for POJO values.  It does not happen for `Output<Resource>`s.
Similarly, this only happens for properties.  Functions are not lifted.

- Depending on a **Component** Resource will now depend on all other Resources parented by that
  Resource. This will help out the programming model for Component Resources as your consumers can
  just depend on a Component and have that automatically depend on all the child Resources created
  by that Component.  Note: this does not apply to a **Custom** resource.  Depending on a
  CustomResource will still only wait on that single resource being created, not any other Resources
  that consider that CustomResource to be a parent.


## 0.16.19 (2019-03-04)

- Rolled back change where calling toString/toJSON on an Output would cause a message
  to be logged to the `pulumi` diagnostics stream.

## 0.16.18 (2019-03-01)

- Fix an issue where the Pulumi CLI would load the newest plugin for a resource provider instead of the version that was
  requested, which could result in the Pulumi CLI loading a resource provider plugin that is incompatible with the
  program. This has the potential to disrupt users that previously had working configurations; if you are experiencing
  problems after upgrading to 0.16.17, you can opt-in to the legacy plugin load behavior by setting the environnment
  variable `PULUMI_ENABLE_LEGACY_PLUGIN_SEARCH=1`. You can also install plugins that are missing with the command
  `pulumi plugin install resource <name> <version> --exact`.

### Improvements

- Attempting to convert an [Output<T>] to a string or to JSON will now result in a warning
  message being printed, as well as information on how to rectify the situation.  This is
  to help with diagnosing cryptic problems that can occur when Outputs are accidentally
  concatenated into a string in some part of the program.

- Fixes incorrect closure serialization issue (https://github.com/pulumi/pulumi/pull/2497)

- `pulumi` will now check that all versions of `@pulumi/pulumi` are compatible in your node_modules
  folder, and will issue a warning message if not.  To be compatible, the versions of
  `@pulumi/pulumi` must agree on their major and minor versions.  Running incompatible versions is
  not something that will be blocked, but it is discouraged as it may lead to subtle problems if one
  version of `@pulumi/pulumi` is loaded and passes objects to/from an incompatible version.

## 0.16.17 (2019-02-27)

### Improvements

- Rolling back the change:
  "Depending on a Resource will now depend on all other Resource's parented by that Resource."

  Unforeseen problems cropped up that caused deadlocks.  Removing this change until we can
  have a high quality solution without these issues.

## 0.16.16 (2019-02-24)

### Improvements

- Fix deadlock with resource dependencies (https://github.com/pulumi/pulumi/issues/2470)

## 0.16.15 (2019-02-22)

### Improvements

- When trying to `stack rm` a stack managed by pulumi.com that has resources, the error message now informs you to pass `--force` if you really want to remove a stack that still has resources under management, as this would orphan these resources (fixes [pulumi/pulumi#2431](https://github.com/pulumi/pulumi/issues/2431)).
- Enabled Python programs to delete resources in parallel (fixes [pulumi/pulumi#2382](https://github.com/pulumi/pulumi/issues/2382)). If you are using Python 2, you should upgrade to Python 3 or else you may experience problems when deleting resources.
- Fixed an issue where Python programs would occasionally fail during preview with errors about empty IDs being passed
  to resources. ([pulumi/pulumi#2450](https://github.com/pulumi/pulumi/issues/2450))
- Return an error from `pulumi stack tag` commands when using the `--local` mode.
- Depending on a Resource will now depend on all other Resource's parented by that Resource.
  This will help out the programming model for Component Resources as your consumers can just
  depend on a Component and have that automatically depend on all the child Resources created
  by that Component.

## 0.16.14 (2019-01-31)

### Improvements

- Fix a regression in `@pulumi/pulumi` introduced by 0.16.13 where an update could fail with an error like:

```
Diagnostics:
  pulumi:pulumi:Stack (my-great-stack):
    TypeError: resproto.InvokeRequest is not a constructor
        at Object.<anonymous> (.../node_modules/@pulumi/pulumi/runtime/invoke.js:58:25)
        at Generator.next (<anonymous>)
        at fulfilled (.../node_modules/@pulumi/pulumi/runtime/invoke.js:17:58)
        at <anonymous>
```

We appologize for the regression.  (fixes [pulumi/pulumi#2414](https://github.com/pulumi/pulumi/issues/2414))

### Improvements

- Individual resources may now be explicitly marked as requiring delete-before-replace behavior. This can be used e.g. to handle explicitly-named resources that may not be able to be replaced in the usual manner.

## 0.16.13 (2019-01-31)

### Major Changes

- When used in conjunction with the latest versions of the various language SDKs, the Pulumi CLI is now more precise about the dependent resources that must be deleted when a given resource must be deleted before it can be replaced (fixes [pulumi/pulumi#2167](https://github.com/pulumi/pulumi/issues/2167)).

**NOTE**: As part of the above change, once a stack is updated with v0.16.13, previous versions of `pulumi` will be unable to manage it.

### Improvements

- Issue a more prescriptive error when using StackReference and the name of the stack to reference is not of the form `<organization>/<project>/<stack>`.

## 0.16.12 (2019-01-25)

### Major Changes

- When using the cloud backend, stack names now must only be unique within a project, instead of across your entire account. Starting with version of 0.16.12 the CLI, you can create stacks with duplicate names. If an account has multiple stacks with the same name across different projects, you must use 0.16.12 or later of the CLI to manage them.

**BREAKING CHANGE NOTICE**: As part of the above change, when using the 0.16.12 CLI (or a later version) the names passed to `StackReference` must be updated to be of the form (`<organization>/<project>/<stack>`) e.g. `acmecorp/infra/dev` to refer to the `dev` stack of the `infra` project in the `acmecorp` organization.

### Improvements

- Add `--json` to `pulumi config`, `pulumi config get`, `pulumi history` and `pulumi plugin ls` to request the output be in JSON.

- Changes to `pulumi new`'s output to improve the experience.

## 0.16.11 (2019-01-16)

### Improvements

- In the nodejs SDK, `pulumi.interpolate` and `pulumi.concat` have been added as convenient ways to combine Output values into strings.

- Added `pulumi history` to show information about the history of updates to a stack.

- When creating a project with `pulumi new` the generated `Pulumi.yaml` file no longer contains the template section, which was unused after creating a project

- In the Python SDK, the `is_dry_run` function just always returned `true`, even when an update (and not a preview) was being preformed. This has been fixed.

- Python programs will no longer deadlock due to exceptions in functions run during applies.

## 0.16.10 (2019-01-11)

### Improvements

- Support for first-class providers in Python.

- Fix a bug where `StackReference` outputs were not updated when changes occured in the referenced stack.

- Added `pulumi stack tag` commands for managing stack tags stored in the cloud backend.

- Link directly to /account/tokens when prompting for an access token.

- Exporting a Resource from an application Stack now exports it as a rich recursive pojo instead of just being an opaque URN (fixes https://github.com/pulumi/pulumi/issues/1858).

## 0.16.9 (2018-12-24)

### Improvements

- Update the error message when When `pulumi` commands fail to detect your project to mention that `pulumi new` can be used to create a new project (fixes [pulumi/pulumi#2234](https://github.com/pulumi/pulumi/issues/2234))

- Added a `--stack` argument (short form `-s`) to `pulumi stack`, `pulumi stack init`, `pulumi state delete` and `pulumi state unprotect` to allow operating on a different stack than the currently selected stack. This brings these commands in line with the other commands that operate on stacks and already provided a `--stack` option (fixes [pulumi/pulumi#1648](https://github.com/pulumi/pulumi/issues/1648))

- Added `Output.all` and `Output.from_input` to the Python SDK.

- During previews and updates, read operations (i.e. calls to `.get` methods) are no longer shown in the output unless they cause any changes.

- Fix a performance regression where `pulumi preview` and `pulumi up` would hang for a few moments at the end of a preview or update, in addition to the overall operation being slower.

## 0.16.8 (2018-12-14)

### Improvements

- Fix an issue that caused panics due to shutting the Jaeger tracing infrastructure down before all traces had finished ([pulumi/pulumi#1850](https://github.com/pulumi/pulumi/issues/1850))

## 0.16.7 (2018-12-05)

### Improvements

- Configuration and stack commands now take a `--config-file` options. This option allows the user to override the file used to fetch and store config information for a stack during the execution of a command.

- Fix an issue where ANSI escape codes would appear in messages printed from the CLI when running on Windows.

- Fix an error about a bad icotl when trying to read sensitive input from the console and standard in was not connected to a terminal.

- The dynamic provider would fail to launch if your `node_modules` folder was non in the default location or had a non standard layout. This has been fixed so we correctly find your `node_modules` folder in the same way node does. (fixes [pulumi/pulumi#2261](https://github.com/pulumi/pulumi/issues/2261))

## 0.16.6 (2018-11-28)

### Major Changes

- When running a Python program, pulumi will now run `python3` instead of `python`, since `python` often points at Python 2.7 binary, and Pulumi requires Python 3.6 or later. The environment variable `PULUMI_PYTHON_CMD` can be used to provide a different binary to run.

### Improvements

- Allow `Output`s in the dependsOn property of `ResourceOptions` (fixes [pulumi/pulumi#991](https://github.com/pulumi/pulumi/issues/991))

- Add a new `StackReference` type to the node SDK which allows referencing an output of another stack (fixes [pulumi/pulumi#109](https://github.com/pulumi/pulumi/issues/109))

- Fix an issue where `pulumi` would not respect common `NO_PROXY` settings (fixes [pulumi/pulumi#2134](https://github.com/pulumi/pulumi/issues/2134))

- The CLI wil now correctly report any output from a Python program which writes to `sys.stderr` (fixes [pulumi/pulumi#1542](https://github.com/pulumi/pulumi/issues/1542))

- Don't install packages by default for Python projects when creating a new project from a template using `pulumi new`. Previously, `pulumi` would install these packages using `pip install` and they would be installed globally when `pulumi` was run outside a virtualenv.

- Fix an issue where `pulumi` could panic during a peview when using a first class provider which was constructed using an output property of another resource (fixes [pulumi/pulumi#2223](https://github.com/pulumi/pulumi/issues/2223))

- Fix an issue where `pulumi` would fail to load resource plugins for newer dev builds.

- Fix an issue where running two copies of `pulumi plugin install` in parallel for the same plugin version could cause one to fail with an error about renaming a directory.

- Fix an issue where if the directory containing the `pulumi` executable was not on the `$PATH` we would fail to load language plugins. We now will also search next to the current running copy of Pulumi (fixes [pulumi/pulumi#1956](https://github.com/pulumi/pulumi/issues/1956))

- Fix an issue where passing a key of the form `foo:config:bar:baz` to `pulumi config set` would succeed but cause errors later when trying to interact with the stack. Setting this value is now blocked eagerly (fixes [pulumi/pulumi#2171](https://github.com/pulumi/pulumi/issues/2171))

## 0.16.5 (2018-11-16)

### Improvements

- Fix an issue where `pulumi plugin install` would fail on Windows with an access deined message.

## 0.16.4 (2018-11-12)

### Major Changes

- If you're using Pulumi with Python, this release removes Python 2.7 support in favor of Python 3.6 and greater. In addition, some members have been renamed. For example the `stack_output` function has been renamed to `export`. All major features of Pulumi work with this release, including parallelism!

### Improvements

- Download plugins to a temporary folder during `pulumi plugin install` to ensure if the operation is canceled, the have downloaded plugin is not used.

- If an update is in progress when `pulumi stack ls` is run, don't show its last update time as "a long time ago".

- Add `--preserve-config` to `pulumi stack rm` which causes Pulumi to keep the `Pulumi.<stack-name>.yaml` when removing a stack.

- Support passing template names to `pulumi up` the same as `pulumi new` does.

- When `-g` or `--generate-only` is passed to `pulumi new`, don't show a confusing message that says it will update a stack.

- Fix an issue where an output property of a resource would change its type during an update in some cases.

- Provide richer detail on the properties during a multi-stage replace.

- Fix `pulumi logs` so it can collect log messages from Lambdas on AWS.

- Pulumi now reports metadata during CI runs on CircleCI, for later display on app.pulumi.com.

- Fix an assert that could fire if a checkpoint had multiple resources with the same URN (which could happen in cases where a delete operation was pending on an old copy of a resource).

- When `$TERM` is set to `dumb`, Pulumi should no longer try to use interactive reading from the terminal, which would fail.

- When displaying elapsed time for an update, round to the nearest second.

- Add the `--json` flag to the `pulumi logs` command.

- Add an `iterable` module to `@pulumi/pulumi` with two helpful combinators `toObject` and `groupBy` to help combine multiple `Output<T>`'s into a single object.

- Pulumi no longer prompts you for confirmation when `--skip-preview` is passed to `pulumi up`. Instead, it just preforms the update as requested.

- Add the `--json` flag to the `pulumi stack ls` command.

- The `--color=always` flag should now be respected in all cases.

- Pulumi now reports metadata about GitLab repositories when doing an update, so they can be shown on app.pulumi.com.

- Pulumi now uses compression when uploading your checkpoint file to the Pulumi service, which should speed up updates where your stack has many resources.

- "First Class" providers used to be shown as changing during previews. This is no longer the case.

## 0.16.3 (2018-11-06)

### Improvements

- Fully support Node 11 [pulumi/pulumi#2101](https://github.com/pulumi/pulumi/pull/2101)

## 0.16.2 (2018-10-29)

### Improvements

- Fix a regression that would cause resource operations to not be processed in parallel when using the latest CLI with a `@pulumi/pulumi` older than 0.16.1 [pulumi/pulumi#2123](https://github.com/pulumi/pulumi/pull/2123)

- Fail with a better error message (and in fewer cases) on Node 11. We hope to have complete support for Node 11 later this week, but for now recommend using Node 10 or earlier. [pulumi/pulumi#2098](https://github.com/pulumi/pulumi/pull/2098)

## 0.16.1 (2018-10-23)

### Improvements

- A new top-level CLI command “pulumi state” was added to assist in making targeted edits to the state of a stack. Two subcommands, “pulumi state delete” and “pulumi state unprotect”, can be used to delete or unprotect individual resources respectively within a Pulumi stack. [pulumi/pulumi#2024](https://github.com/pulumi/pulumi/pull/2024)
- Default to allowing as many parallel operations as possible [pulumi/pulumi#2065](https://github.com/pulumi/pulumi/pull/2065)
- Fixed an issue with the generated type for an Unwrap expression when using TypeScript [pulumi/pulumi#2061](https://github.com/pulumi/pulumi/pull/2061)
- Improve error messages when resource plugins can't be loaded or when a checkpoint is invalid [pulumi/pulumi#2078](https://github.com/pulumi/pulumi/pull/2078)
- Fix link to the Pulumi Web Console in the CLI for a stack [pulumi/pulumi#2075](https://github.com/pulumi/pulumi/pull/2075)
- Attach git commit metadata to Pulumi updates in some additional cases [pulumi/pulumi#2062](https://github.com/pulumi/pulumi/pull/2062) and [pulumi/pulumi#2069](https://github.com/pulumi/pulumi/pull/2069)

## 0.16.0 (2018-10-15)

### Major Changes

#### Improvements to CLI output

Default colors that fit better for both light and dark terminals.  Overall updates to rendering of previews/updates for consistency and simplicity of the display.

#### Parallelized resource deletion

Parallel resource creation and updates were added in `0.15`.  In `0.16`, this has been extended to include deletions, which are now conservatively parallelized based on dependency information. [pulumi/pulumi#1963](https://github.com/pulumi/pulumi/pull/1963)

#### Support for any CI system in the Pulumi GitHub App

The Pulumi GitHub App previously supported just TravisCI.  With this release the `pulumi` CLI now supports configurable CI providers via environment variables.  Thanks [@jen20](https://github.com/jen20)!

### Improvements

In addition to the above features, we've made a handfull of day to day improvements in the CLI:
- Support for `zsh` completions. Thanks to [@Tirke](https://github.com/Tirke)!) [pulumi/pulumi#1967](https://github.com/pulumi/pulumi/pull/1967)
- JSON formatting support for `pulumi stack output`. [pulumi/pulumi#2000](https://github.com/pulumi/pulumi/pull/2000)
- Added a `Dockerfile` for the Pulumi CLI and development environment for use in hosted environments.
- Many improvements for Go development.  Thanks to [@justone](https://github.com/justone)!. [pulumi/pulumi#1954](https://github.com/pulumi/pulumi/pull/1954) [pulumi/pulumi#1955](https://github.com/pulumi/pulumi/pull/1955) [pulumi/pulumi#1965](https://github.com/pulumi/pulumi/pull/1965)
- Extend `pulumi.output` to deeply unwrap `Input`s.  This significantly simplifies working with `Inputs` when building Pulumi components.  [pulumi/pulumi#1915](https://github.com/pulumi/pulumi/pull/1915)

## 0.15.4 (2018-09-28)

### Improvements

- Fix an assert in display code when a resource property transitions from an asset to an archive or the other way around

## 0.15.3 (2018-09-18)

### Improvements

- Improved performance of `pulumi stack ls`
- Fix build authoring so the dynamic provider works for the CLI built by Homebrew (thanks to **[@Tirke](https://github.com/Tirke)**!)

## 0.15.2 (2018-09-11)

### Major Changes

Major features of this release include:

#### Ephemeral status messages

Providers are now able to register "ephmeral" update messages which are shown in the "Info" column in the CLI during an update, but which are not printed at the end of the update. The new version of the `@pulumi/kubernetes` package uses this when printing messages about resource initialization.

#### Local backend

The local backend (which stores your deployment's state file locally, instead of on pulumi.com) has been improved. You can now use `pulumi login --local` or `pulumi login file://<path-to-storage-root>` to select the local backend and control where state files are stored. In addition, older versions of the CLI would behave slightly differently when using the local backend vs pulumi.com, for example, some operations would not show previews before running.  This has been fixed.  When using the local backend, updates print the on disk location of the checkpoint file that was written. The local backend is covered in more detail in [here](https://www.pulumi.com/docs/reference/state/).

#### `pulumi refresh`

We've made a bunch of improvements in `pulumi refresh`. Some of these improve the UI during a refresh (for example, clarifying text about the underyling operations) as well fixing bugs with refreshing certain types of objects (for example CloudFront CDNs).

#### `pulumi up` and `pulumi new`

You can now pass a URL to a Git repository to `pulumi up <url>` to deploy a project without having to manage its source code locally. This works like `pulumi new <url>`, but configures and deploys the project from a temporary directory that will be cleaned up automatically after the update.

`pulumi new` now outputs an error when the current working directory (or directory specified explicitly via the `--dir` flag) is not empty. Additionally, `pulumi new` now runs a preview of an initial update at the end of its operation and asks if you would like to perform the update.

Both `pulumi up` and `pulumi new` now support `-c` flags for specifying config values as arguments (e.g. `pulumi up <url> -c aws:region=us-east-1`).

### Improvements

In addition to the above features, we've made a handfull of day to day improvements in the CLI:

- Support `pulumi` in a projects in a Yarn workspaces. [pulumi/pulumi#1893](https://github.com/pulumi/pulumi/pull/1893)
- Improve error message when there are errors decrypting secret configuration values. [pulumi/pulumi#1815](https://github.com/pulumi/pulumi/pull/1815)
- Don't fail `pulumi up` when plugin discovery fails. [pulumi/pulumi#1745](https://github.com/pulumi/pulumi/pull/1745)
- New helpers for extracting and validating values from `pulumi.Config`. [pulumi/pulumi#1843](https://github.com/pulumi/pulumi/pull/1843)
- Support serializng "factory" functions. [pulumi/pulumi#1804](https://github.com/pulumi/pulumi/pull/1804)

## 0.15.0 (2018-08-13)

### Major Changes

#### Parallelism

Pulumi now performs resource creates and updates in parallel, driven by dependencies in the resource graph. (Parallel deletes are coming in a future release.) If your program has implicit dependencies that Pulumi does not already see as dependencies, it's possible parallel will cause ordering issues. If this happens, you may set the `dependsOn` on property in the `resourceOptions` parameter to any resource. By default, Pulumi allows 10 parallel operations, but the `-p` flag can be used to override this. `-p=1` disables parallelism altogether. Parallelism is supported for Node.js and Go programs, and Python support will come in a future release.

#### First Class Providers

Pulumi now allows creation and configuration of resource providers programmatically. In addition to the default provider instance for each resource, you can also create an explicit version of the provider and configure it explicitly. This can be used to create some resources in a different region from your main deployment, or deploy resources to a programmatically configured Kubernetes cluster, for example. We have [a multi-region deployment example](https://github.com/pulumi/pulumi-aws/blob/master/examples/multiple-regions/index.ts) for illustrative purposes.

#### Status Rich Updates

The Pulumi CLI is now able to report more detailed information from individual resources during an update. This is used, for instance, in the Kubernetes provider, to provide incremental progress output for steps that may take a while to comeplete (such as deployment orchestration). We anticipate leveraging this feature in more places over time.

#### Improved Templating Support

You can now pass a URL to a Git repository to `pulumi new` to install a custom template, enabling you to share common templates across your team. If you pass a simple name, or omit arguments altogether, `pulumi new` behaves as before, using the [templates hosted by Pulumi](https://github.com/pulumi/templates).

#### Native TypeScript support

By default, Pulumi now natively supports TypeScript, so you do not need to run `tsc` explicitly before deploying. (We often forget to do this too!) Simply run `pulumi up`, and the program will be recompiled on the fly before running it.

To use this new support, upgrade your `@pulumi/pulumi` version to 0.15.0, in addition to the CLI. Pulumi prefers JavaScript source to TypeScript source, so if you had been using TypeScript previously, we recommend you make the following changes:

1. Remove the `main` and `typings` directives from `package.json`, as well as the `build` script.
2. Remove the `bin` folder that contained your previously compiled code.
3. You may remove the dependency on `typescript` from your `package.json` as well, since `@pulumi/pulumi` has one.

While a `tsconfig.json` file is no longer required, as Pulumi uses intelligent defaults, other tools like VS Code behave
better when it is present, so you'll probably want to keep it.

#### Closure capturing improvements

We've improved our closure capturing logic, which should allow you to write more idiomatic code in lambda functions that are uploaded to the cloud. Previously, if you wanted to use a module, we required you to write either `require('module')` or `await import('module')` inside your lambda function. In addition, if you wanted to use a helper you defined in another file, you had to require that module in your function as well. With these changes, the following code now works:

```typescript
import * as axios from "axios";
import * as cloud from "@pulumi/cloud-aws";

const api = new cloud.API("api");
api.get("/", async (req, res) => {
    const statusText = (await axios.default.get("https://www.pulumi.com")).statusText;
    res.write(`GET https://www.pulumi.com/ == ${statusText}`).end();
});
```

#### Default value for configuration package

The `pulumi.Config` object can now be created without an argument. When no argument is supplied, the value of the current project is used. This means that application level code can simply do `new pulumi.Confg()` without passing any argument. For library authors, you should continue to pass the name of your package as an argument.

#### Pulumi GitHub App (preview)

The Pulumi GitHub application bridges the gap between GitHub (source code, pull requests) and Pulumi (cloud resources, stack updates). By installing
the Pulumi GitHub application into your GitHub organization, and then running Pulumi as part of your CI build process, you can now see the results of
stack updates and previews as part of pull requests. This allows you to see the potential impact a change would have on your cloud infrastructure before
merging the code.

The Pulumi GitHub application is still in preview as we work to support more CI systems and provide richer output. For information on how to install the
GitHub application and configure it with your CI system, please [visit our documentation](https://www.pulumi.com/docs/reference/cd-github/) page.

### Improvements

- The CLI no longer emits warnings if it can't detect metadata about your git enlistement (for example, what GitHub project it coresponds to).
- The CLI now only warns about adding a plaintext configuration in cases where it appears likely you may be storing a secret.

## 0.14.3 (2018-07-20)

### Improvements

#### Fixed

- Support empty text assets ([pulumi/pulumi#1599](https://github.com/pulumi/pulumi/pull/1599)).

- When printing message in non-interactive mode, do not keep printing out the worst diagnostic ([pulumi/pulumi#1640](https://github.com/pulumi/pulumi/pull/1640)). When run in non interactive environments (e.g. docker) Pulumi would print duplicate messages to the screen related to a resource when the running Pulumi program was writing to standard out (e.g. if it was invoking a docker build). This no longer happens. The full output from the program continues to be printed at the end of execution.

- Work around a potentially bad assert in the engine ([pulumi/pulumi#1640](https://github.com/pulumi/pulumi/pull/1644)). In some cases, when Pulumi failed to delete a resource as part of an update, future updates would crash with an assert message. This is no longer the case and Pulumi will try to delete the resource it had marked as should be deleted.

#### Added

- Print out a 'still working' message every 20 seconds when in non-interactive mode ([pulumi/pulumi#1616](https://github.com/pulumi/pulumi/pull/1616)). When Pulumi is waiting for a long running resource operation to create (e.g. waiting for an ECS service to become stable after creation), print some output to the console even when running non-interactively. This helps for cases like TravsCI where if output is not written for a while the job is assumed to have hung and is aborted.

- Support the NO_COLOR env variable to suppress any colored output ([pulumi/pulumi#1594](https://github.com/pulumi/pulumi/pull/1594)). Pulumi now respects the `NO_COLOR` environment variable. When set to a truthy value, colors are suppressed from the CLI. In addition, the `--color` flag can now be passed to all `pulumi` commands.

## 0.14.2 (2018-07-03)

### Improvements

- Support -s in `stack {export, graph, import, output}` ([pulumi/pulumi#1572](https://github.com/pulumi/pulumi/pull/1574)). `pulumi stack export`, `pulumi stack graph`, `pulumi stack import` and `pulumi stack output` now support a `-s` or `--stack` flag, which allows them to operate on a different stack that the currently selected one.

## 0.14.1 (2018-06-29)

### Improvements

#### Added

- Add `pulumi whoami` ([pulumi/pulumi#1572](https://github.com/pulumi/pulumi/pull/1572)). `pulumi whoami` will report the account name of the current logged in user. In addition, we now display the name of the current user after `pulumi login`.

#### Fixed

- Don't require `PULUMI_DEBUG_COMMANDS` to be set to use local backend ([pulumi/pulumi#1575](https://github.com/pulumi/pulumi/pull/1575)).

- Improve misleading `pulumi new` summary message ([pulumi/pulumi#1571](https://github.com/pulumi/pulumi/pull/1571)).

- Fix printing out outputs in a pulumi program ([pulumi/pulumi#1531](https://github.com/pulumi/pulumi/pull/1531)). Pulumi now shows the values of output properties after a `pulumi up` instead of requiring you to run `pulumi stack output`.

- Do a better job preventing serialization of unnecessary objects in closure serialization ([pulumi/pulumi#1543](https://github.com/pulumi/pulumi/pull/1543)).  We've improved our analysis when serializing functions. This yeilds smaller code when a function is serialized and prevents errors around unused native code being captured in some cases.

## 0.14.0 (2018-06-15)

### Improvements

#### Added

- Publish to pypi.org ([pulumi/pulumi#1497](https://github.com/pulumi/pulumi/pull/1497)). Pulumi packages are now public on pypi.org!

- Add optional `--dir` flag to `pulumi new` ([pulumi/pulumi#1459](https://github.com/pulumi/pulumi/pull/1459)). The `pulumi new` command now has an optional flag `--dir`, for the directory to place the generated project. If it doesn't exist, it will be created.

- Support Pulumi programs written in Go ([pulumi/pulumi#1456](https://github.com/pulumi/pulumi/pull/1456)). Initial version for Pulumi programs written in Go. While it is not complete, basic resource registration works.

- Allow overriding config location ([pulumi/pulumi#1379](https://github.com/pulumi/pulumi/pull/1379)). Support a new `config` member in `Pulumi.yaml`, which specifies a relative path to a folder where per-stack configuration is stored. The path is relative to the location of `Pulumi.yaml` itself.

- Delete existing resources before replacing, for resources that must be singletons ([pulumi/pulumi#1365](https://github.com/pulumi/pulumi/pull/1365)). For resources where the cloud vendor does not allow multiple resources to exist, such as a mount target in EFS, Pulumi now deletes the existing resource before creating a replacement resource.

#### Changed

- Compute required packages during closure serialization ([pulumi/pulumi#1457](https://github.com/pulumi/pulumi/pull/1457)). Closure serialization now keeps track of the `require`'d packages it sees in the function bodies that are serialized during a call to `serializeFunction`. So, only required packages are uploaded to Lambda.

- Support browser based logins to the CLI ([pulumi/pulumi#1439](https://github.com/pulumi/pulumi/pull/1439)). The Pulumi CLI now has an option to login via a browser. When you are prompted for an access token, you can just hit enter. The CLI then opens a browser to [app.pulumi.com](https://app.pulumi.com) so that you can authenticate.

#### Fixed

- Support better previews in Python by mocking out Unknown values ([pulumi/pulumi#1482](https://github.com/pulumi/pulumi/pull/1482)). During the preview phase of a deployment, computed values were `Unknown` in Python, causing the preview to be empty. This issue is now resolved.

- Issue a better error message if you capture a V8 intrinsic ([pulumi/pulumi#1423](https://github.com/pulumi/pulumi/pull/1423)). It's possible to accidentally take a dependency on a Pulumi deployment-time library, which causes problems when creating a runtime function for AWS Lambda. There is now a better error message when this situation occurs.

## 0.12.2 (2018-05-19)

### Improvements

- Improve the promise leak experience ([pulumi/pulumi#1374](https://github.com/pulumi/pulumi/pull/1374)). Fixes an issue where a promise leak could be erroneously reported. Also, show simple error message by default, unless the environment variable `PULUMI_DEBUG_PROMISE_LEAKS` is set.

## 0.12.1 (2018-05-09)

### Added

- A new all-in-one installer script is now available at [https://get.pulumi.com](https://get.pulumi.com).

- Many enhancements to `pulumi new` ([pulumi/pulumi#1307](https://github.com/pulumi/pulumi/pull/1307)).  The command now interactively walks through creating everything needed to deploy a new stack, including selecting a template, providing a name, creating a stack, setting default configuration, and installing dependencies.

- Several improvements to the `pulumi up` CLI experience ([pulumi/pulumi#1260](https://github.com/pulumi/pulumi/pull/1260)): a tree view display, more details from logs during deployments, and rendering of stack outputs at the end of updates.

### Changed

- (**Breaking**) Remove the `--preview` flag in `pulumi up`, in favor of reintroducing `pulumi preview` ([pulumi/pulumi#1290](https://github.com/pulumi/pulumi/pull/1290)). Also, to accept an update without the interactive prompt, use the `--yes` flag, rather than `--force`.

### Fixed

- Significant performance improvements for `pulumi up` ([pulumi/pulumi#1319](https://github.com/pulumi/pulumi/pull/1319)).

- JavaScript `async` functions in Node 7.6+ now work with Pulumi function serialization ([pulumi/pulumi#1311](https://github.com/pulumi/pulumi/pull/1311).

- Support installation on Windows in folders which contain spaces in their name ([pulumi/pulumi#1300](https://github.com/pulumi/pulumi/pull/1300)).


## 0.12.0 (2018-04-26)

### Added

- Add a `pulumi cancel` command ([pulumi/pulumi#1230](https://github.com/pulumi/pulumi/pull/1230)). This command cancels any in-progress operation for the current stack.

### Changed

- (**Breaking**) Eliminate `pulumi init` requirement ([pulumi/pulumi#1226](https://github.com/pulumi/pulumi/pull/1226)). The `pulumi init` command is no longer required and should not be used for new stacks. For stacks created prior to the v0.12.0 SDK, `pulumi init` should still be run in the project directory if you are connecting to an existing stack. For new projects, stacks will be created under the currently logged in account. After upgrading the CLI, it is necessary to run `pulumi stack select`, as the location of bookkeeping files has been changed. For more information, see [Creating Stacks](https://www.pulumi.com/docs/reference/stack/#create-stack).

- (**Breaking**) Remove the explicit 'pulumi preview' command ([pulumi/pulumi#1170](https://github.com/pulumi/pulumi/pull/1170)). The `pulumi preview` output has now been merged in to the `pulumi up` command. Before an update is run, the preview is shown and you can choose whether to proceed or see more update details. To see just the preview operation, run `pulumi up --preview`.

- Switch to a more streamlined view for property diffs in `pulumi up` ([pulumi/pulumi#1212](https://github.com/pulumi/pulumi/pull/1212)).

- Allow multiple versions of the `@pulumi/pulumi` package to be loaded ([pulumi/pulumi#1209](https://github.com/pulumi/pulumi/pull/1209)). This allows packages and dependencies to be versioned independently.

### Fixed
- When running a `pulumi up` or `destroy` operation, a single ctrl-c will cancel the current operation, waiting for it to complete. A second ctrl-c will terminate the operation immediately. ([pulumi/pulumi#1231](https://github.com/pulumi/pulumi/pull/1231)).

- When getting update logs, get all results ([pulumi/pulumi#1220](https://github.com/pulumi/pulumi/pull/1220)). Fixes a bug where logs could sometimes be truncated in the pulumi.com console.

## 0.11.3 (2018-04-13)

- Switch to a resource-progress oriented view for pulumi preview, update, and destroy operations ([pulumi/pulumi#1116](https://github.com/pulumi/pulumi/pull/1116)). The operations `pulumi preview`, `update` and `destroy` have far simpler output by default, and show a progress view of ongoing operations. In addition, there is a structured component view, showing a parent operation as complete only when all child resources have been created.

- Remove strict dependency on Node v6.10.x ([pulumi/pulumi#1139](https://github.com/pulumi/pulumi/pull/1139)). It is now no longer necessary to use a specific version of Node to run Pulumi programs. Node versions after 6.10.x are supported, as long as they are under **Active LTS** or are the **Current** stable release.

## 0.11.2 (2018-04-06)

### Changed

- (Breaking) Require `pulumi login` before commands that need a backend ([pulumi/pulumi#1114](https://github.com/pulumi/pulumi/pull/1114)). The `pulumi` CLI now requires you to log in to pulumi.com for most operations.

### Fixed

- Improve the error message arising from missing required configurations for resource providers ([pulumi/pulumi#1097](https://github.com/pulumi/pulumi/pull/1097)). The error message now prints all missing configuration keys, along with their descriptions.

## 0.11.0 (2018-03-20)

### Added

- Add a `pulumi new` command to scaffold a project ([pulumi/pulumi#1008](https://github.com/pulumi/pulumi/pull/1008)). Usage is `pulumi new [templateName]`. If template name is not specified, the CLI will prompt with a list of templates. Currently, the templates `javascript`, `python` and `typescript` are available. Templates are defined in the GitHub repo [pulumi/templates](https://github.com/pulumi/templates) and contributions are welcome!

- Python is now a supported language in Pulumi ([pulumi/pulumi#800](https://github.com/pulumi/pulumi/pull/800)). For more information, see [Python documentation](https://www.pulumi.com/docs/reference/python/).

### Changed

- (Breaking) Change the way that configuration is stored ([pulumi/pulumi#986](https://github.com/pulumi/pulumi/pull/986)). To simplify the configuration model, there is no longer a separate notion of project and workspace settings, but only stack settings. The switches `--all` and `--save` are no longer supported; any common settings across stacks must be set on each stack directly. Settings for a stack are stored in a file that is a sibling to `Pulumi.yaml`, named `Pulumi.<stack-name>.yaml`. On first run `pulumi`, will migrate projects from the previous configuration format to the new one. The recommended practice is that developer stacks that are not shared between team members should be added to `.gitignore`, while stack setting files for shared stacks should be checked in to source control. For more information, see the section [Defining and setting stack settings](https://www.pulumi.com/docs/reference/config/#config-stack).

- (Breaking) Eliminate the superfluous `:config` part of configuration keys ([pulumi/pulumi#995](https://github.com/pulumi/pulumi/pull/995)). `pulumi` no longer requires configuration keys to have the string `:config` in them. Using the `:config` string in keys for the object `@pulumi/pulumi.Config` is deprecated and `preview` and `update` show warnings when it is used. Additionally, it is preferred to set keys in the form `aws:region` rather than `aws:config:region`. For compatibility, the old behavior is also supported, but will be removed in a future release. For more information, see the article [Configuration](https://www.pulumi.com/docs/reference/config/).

- (Breaking) Modules are treated as normal values when serialized ([pulumi/pulumi#1030](https://github.com/pulumi/pulumi/pull/1030)). If you need to use a module at runtime, consider either using `require` or `await import` at runtime, or pre-compute what you need and capture the resulting data or objects.

<!-- NOTE: the programming-model article below is still all todos. -->
- (Breaking) Serialize resource registration after inputs resolve ([pulumi/pulumi#964](https://github.com/pulumi/pulumi/pull/964)). Previously, resources were most often created/updated in the order they were seen during the Pulumi program execution. In preparation for supporting parallel resource operations, these operations now run in an order that respects the dependencies between resources (via [`Output`](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/#Output)), but may not match the order of program execution. This is mostly transparent to Pulumi program authors, but does mean that any missing dependencies will cause your program to fail in unexpected ways. For more information on how such failures manifest and what to do about them, see the article [Programming Model](https://www.pulumi.com/docs/reference/programming-model/).

- Hide secrets from CLI output ([pulumi/pulumi#1002](https://github.com/pulumi/pulumi/pull/1002)). To prevent secret values from being accidentally disclosed in command output or logs, `pulumi` replaces secret values with the string `[secret]`. Inspired by the behavior of [Travis CI](https://travis-ci.org/).

- Change default of where stacks are created ([pulumi/pulumi#971](https://github.com/pulumi/pulumi/pull/971)). If currently logged in to the Pulumi CLI, `stack init` creates a managed stack; otherwise, it creates a local stack. To force a local or remote stack, use the flags `--local` or `--remote`.

### Fixed

- Improve error messages output by the CLI ([pulumi/pulumi#1011](https://github.com/pulumi/pulumi/pull/1011)). RPC endpoint errors have been improved. Errors such as "catastrophic error" and "fatal error" are no longer duplicated in the output.

- Produce better error messages when the main module is not found ([pulumi/pulumi#976](https://github.com/pulumi/pulumi/pull/976)). If you're running TypeScript but have not run `tsc` or your main JavaScript file does not exist, the CLI will print a helpful `info:` message that points to the possible source of the error.

## 0.10.0 (2018-02-27)

> **Note:** The v0.10.0 SDK has a strict dependency on Node.js 6.10.2.

### Added

- Support "force" option when deleting a managed stack.

- Add a `pulumi history` command ([pulumi#636](https://github.com/pulumi/pulumi/issues/636)). For a managed stack, use the `pulumi history` to view deployments of that stack's resources.

### Changed

- (Breaking) Use `npm install` instead of `npm link` to reference the Pulumi SDK `@pulumi/aws`, `@pulumi/cloud`, `@pulumi/cloud-aws`. For more information, see [Pulumi npm packages](https://www.pulumi.com/docs/reference/pkg/nodejs/).

- (Breaking) Explicitly track resource dependencies via `Input` and `Output` types. This enables future improvements to the Pulumi development experience, such as parallel resource creation and enhanced dependency visualization. When a resource is created, all of its output properties are instances of a new type [`pulumi.Output<T>`](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/#Output). `Output<T>` contains both the value of the resource property and metadata that tracks resource dependencies. Inputs to a resource now accept `Output<T>` in addition to `T` and `Promise<T>`.

### Fixed

- Managed stacks sometimes return a 500 error when requesting logs
- Error when using `float64` attributes using SDK v0.9.9 ([pulumi-terraform#95](https://github.com/pulumi/pulumi-terraform/issues/95))
- `pulumi logs` entries only return first line ([pulumi#857](https://github.com/pulumi/pulumi/issues/857))

## 0.9.13 (2018-02-07)

### Added

- Added the ability to control the upload context to the Pulumi Service. You may now set a `context` property in `Pulumi.yaml`, which is combined with the location of `Pulumi.yaml`. This new path is the root of what is uploaded and can be used during deployment. This allows you to, for example, share common code that is located in a folder in your source tree above the directory `Pulumi.yaml` for the project you are deploying.

- Added additional configuration for docker builds for a container. The `build` property of a container may now either be a string (which is treated as a path to the folder to do a `docker build` in) or an object with properties `context`, `dockerfile` and `args`, which are passed to `docker build`. If unset, `context` defaults to the current working directory, `dockerfile` defaults to `Dockerfile` and `args` default to no arguments.

## 0.9.11 (2018-01-22)

### Added

- Added the ability to import or export a stack's deployment in the Pulumi CLI. This command can be used for either local or managed stacks. There are two new verbs under the command `stack`:
  - `export` writes the current stack's latest deployment to stdout in JSON format.
  - `import` reads a new JSON deployment from stdin and applies it to the current stack.

- A basic progress spinner is displayed during deployment operations.
  - When the Pulumi CLI is run in interactive mode, it displays an animated ASCII spinner
  - When run in non-interactive mode, CLI prints a message that it is still working. For CI systems that kill jobs when there is no CLI output (such as TravisCI), this eliminates the need to create shell scripts that periodically print output.

### Changed

- To make the behavior of local and managed stacks consistent, the Pulumi CLI uses a separate encryption key for each stack, rather than one shared for all stacks. You can now use a different passphrase for different stacks. Similar to managed stacks, you cannot copy and paste an encrypted value from one stack to another in `Pulumi.yaml`. Instead you must manage the value via `pulumi config`.

- The default behavior for `--color` is now `always`. To change this, specify `--color always` or `--color never`. Previously, the value was based on the presence of the flag `--debug`.

- The command `pulumi logs` now defaults to returning one hour of logs and outputs the start time that is  used.

### Fixed

- When a stack is removed, `pulumi` now deletes any configuration it had saved in either the `Pulumi.yaml` file or the workspace.

## 0.9.8 (2017-12-28)

### Added

#### Pulumi Console and managed stacks

New in this release is the [Pulumi Console](https://app.pulumi.com) and stacks that are managed by Pulumi. This is the recommended way to safely deploy cloud applications.
- `pulumi stack init` now creates a Pulumi managed stack. For a local stack, use `--local`.
- All Pulumi CLI commands now work with managed stacks. Login to Pulumi via `pulumi login`.
- The [Pulumi Console](https://app.pulumi.com) provides a management experience for stacks. You can view the currently deployed resources (along with the AWS ARNs) and see logs from the last update operation.

#### Components and output properties

- Support for component resources([pulumi #340](https://github.com/pulumi/pulumi/issues/340)), enabling grouping of resources into logical components. This provides an improved view of resources during `preview` and `update` operations in the CLI ([pulumi #417](https://github.com/pulumi/pulumi/issues/417)).

  ```
  + pulumi:pulumi:Stack: (create)
     [urn=urn:pulumi:donna-testing::url-shortener::pulumi:pulumi:Stack::url-shortener-donna-testing]
     + cloud:table:Table: (create)
        [urn=urn:pulumi:donna-testing::url-shortener::cloud:table:Table::urls]
        + aws:dynamodb/table:Table: (create)
              [urn=urn:pulumi:donna-testing::url-shortener::cloud:table:Table$aws:dynamodb/table:Table::urls]
  ```
- A stack can have *output properties*, defined as `export let varName = val`. You can view the last deployed value for the output property using `pulumi stack output varName` or in the Pulumi Console.

#### Resource naming

Resource naming is now more consistent, but there is a new file format for checkpoint files for both local and managed stacks.

> If you created stacks in the 0.8 release, you should destroy them with the 0.8 CLI, then recreate with the 0.9.x CLI.

#### Support for configuration secrets
- Store secrets securely in configuration via `pulumi config set --secret`. chris
- The verbs for `config` are now consistent, via `get`, `set`, and `rm`. See [Consistent config verbs #552](https://github.com/pulumi/pulumi/issues/552).

#### Logging

- [**experimental**] Support for the `pulumi logs` command ([pulumi #527](https://github.com/pulumi/pulumi/issues/527)). These features now work:
  - To see new logs as they arrive, use `--follow`
  - Use `--since` to limit to recent logs, such as `pulumi logs --since=1h`
  - Filter to specific resources with `--resource`. This filters to a particular component and its child resources (if any), such as `pulumi logs --resource examples-todoc57917fa --since 1h`

#### Other features

- Support for `.pulumiignore`, for files that should not be uploaded when deploying a managed stack through Pulumi.
- [Allow overriding a `Pulumi.yaml`'s entry point #575](https://github.com/pulumi/pulumi/issues/575). To specify the entry directory, specify `main` in `Pulumi.yaml`. For instance, `main: a/path/to/main/`.
- Support for *protected* resources. A resource can be marked as `protect: true`, which prevents deletion of the resource. For example, `let res = new MyResource("precious", { .. }, { protect: true });`. To "unprotect" the resource, change `protect: false` then run `pulumi up`. See [Allow resources to be flagged "protected" #689](https://github.com/pulumi/pulumi/issues/689).
- Changed defaults for workspace and stack configuration. See [Workspace configuration is error prone #714](https://github.com/pulumi/pulumi/issues/714).
- [Save configuration under the stack by default](https://github.com/pulumi/pulumi/issues/693).

### Fixed
- Improved SDK installer. It automatically creates directories as needed, configures node modules, and prints out friendly error messages.
- [Better diffing in CLI output, especially for Lambdas \#454](https://github.com/pulumi/pulumi/issues/454)
- [`main` does not set working dir correctly for Lambda zip #667](https://github.com/pulumi/pulumi/issues/667)
- [Better error when invalid access token is used in `pulumi login` #640](https://github.com/pulumi/pulumi/issues/640)
- [Eliminate the top-level Stack from all URNs #647](https://github.com/pulumi/pulumi/issues/647)
- Service API for Encrypting and Decrypting secrets
- [Make CLI resilient to network flakiness #763](https://github.com/pulumi/pulumi/issues/763)
- Support --since and --resource on `pulumi logs` when targeting the service
- [Pulumi unable to serialize non-integers #694](https://github.com/pulumi/pulumi/issues/694)
- [Stop buffering CLI output #660](https://github.com/pulumi/pulumi/issues/660)