(languages)=
(language-hosts)=
# Language hosts

*Language hosts*, or *language runtimes*, are the means by which Pulumi is able
to support a variety of programming languages. Officially, the term "language
host" refers to the combination of two parts:

* A runtime, which is a Pulumi [plugin](plugins) that exposes the ability to
  execute programs written in a particular language according to a [standardized
  gRPC interface](pulumirpc.LanguageRuntime). The plugin will be named
  `pulumi-language-<language>` (e.g. `pulumi-language-nodejs` for NodeJS, or
  `pulumi-language-python` for Python).
* An SDK, which is a set of libraries that provide the necessary abstractions
  and utilities for writing Pulumi programs in that language (e.g.
  `@pulumi/pulumi` in NodeJS, or `pulumi` in Python).

Often however, the term "language host" is used to refer to the runtime alone.
Aside from providing the ability to [](pulumirpc.LanguageRuntime.Run) programs,
the runtime also supports a number of other operations:

* *Code generation* methods enable callers to generate both [SDKs](sdkgen)
  ([](pulumirpc.LanguageRuntime.GeneratePackage)) and [programs](programgen)
  ([](pulumirpc.LanguageRuntime.GenerateProject)) in the language.
* *Query* endpoints allow callers to calculate the set of language-specific
  dependencies ([](pulumirpc.LanguageRuntime.GetProgramDependencies)) or Pulumi
  plugins ([](pulumirpc.LanguageRuntime.GetRequiredPlugins)) that might be
  required by a program.
* The *[](pulumirpc.LanguageRuntime.Pack)* method allows callers to package up
  bundles of code written in the language into a format suitable for consumption
  by other code (for instance, packaging an SDK for use as a dependency, or
  packaging a program for execution).

---

## Authoring a New Language Host & SDK

This section details how to implement a `pulumi-language-<lang>` plugin from scratch, including gRPC protocol specifications, code generation schemas, resource registration pipelines, and runtime execution models.

### 1. Architectural Topology

```text
+-------------------------------------------------------------+
|                      Pulumi CLI / Engine                    |
|  - Dependency Graph Resolution                             |
|  - Resource Lifecycle State Machine                         |
|  - Provider Plugin Dispatcher                               |
+------------------------------+------------------------------+
                               | gRPC (LanguageHost)
                               v
+-------------------------------------------------------------+
|               pulumi-language-<lang> Plugin                 |
|  - Executable Binary spawned by Engine                      |
|  - Implements LanguageHost gRPC Protocol Server             |
|  - Orchestrates Language Runtime Execution                  |
+------------------------------+------------------------------+
                               | Subprocess Exec
                               v
+-------------------------------------------------------------+
|                  Target Language Program                    |
|  - Imports Native Language SDK                              |
|  - Constructs Resource Graph Objects                        |
|  - Connects to Engine gRPC (ResourceMonitor)                |
+-------------------------------------------------------------+
```

### 2. The LanguageHost gRPC Protocol

The language host executable must implement the `LanguageHost` protobuf service definition (`pulumirpc/language.proto`).

#### Mandatory RPC Endpoints

1. `GetRequiredPlugins(GetRequiredPluginsRequest) -> (GetRequiredPluginsResponse)`
   - Scans program dependencies or manifest files and returns required provider plugin names and version constraints.

2. `Run(RunRequest) -> (RunResponse)`
   - Spawns the target user script/program.
   - Passes the `engineAddress` (gRPC endpoint for `ResourceMonitor`) via environment variables (`PULUMI_CONFIG`, `PULUMI_PARALLEL`), listening on standard input/output pipes.

3. `GenerateProject(GenerateProjectRequest) -> (GenerateProjectResponse)`
   - Translates a declarative Pulumi YAML or JSON project into native target language source files.

4. `GeneratePackage(GeneratePackageRequest) -> (GeneratePackageResponse)`
   - Reads a Pulumi Schema JSON spec and emits native type-safe SDK classes for a provider package.

### 3. Implementing the Native SDK Core

To complement the language host plugin, a native SDK library must be published in the target language's package ecosystem.

#### Core SDK Modules Required

1. **`Deployment` & `Context` Initialization**:
   - Parses configuration values provided by the environment.
   - Maintains an asynchronous barrier/promise counter ensuring all async resource creations complete before `Run` terminates.

2. **`Output<T>` and `Input<T>` Monad Implementation**:
   - Represents values that will be known only after asynchronous resource provisioning.
   - Implements monadic chaining (`apply`, `map`) and output dependency tracking.

3. **`Resource` Base Classes**:
   - `CustomResource`: ID-addressable cloud infrastructure.
   - `ComponentResource`: Abstract logical groupings of child resources.
   - `ProviderResource`: Configured cloud credential instances.

4. **ResourceMonitor Client Protocol**:
   - Invokes `RegisterResource` via gRPC when a resource constructor is called.
   - Passes properties, parent references, URN hints, and custom options.

### 4. Code Generation Engine (`GeneratePackage`)

When `pulumi package gen-sdk <schema> --language <lang>` is called, your language host receives a PCL (Pulumi Control Language) or JSON schema.

#### Transformation Pipeline:
1. **Schema Parsing**: Deserialize schema types, complex structs, inputs, outputs, and enums.
2. **Type Mapping**: Map primitive schema types (`string`, `integer`, `boolean`, `array`, `object`) to native language primitives and generic types.
3. **Class Generator**: Render class constructors with optional arguments, builder patterns, and method bindings.
4. **Package Manifest**: Generate native package configuration (`package.json`, `Cargo.toml`, `go.mod`, `pom.xml`, etc.).

### 5. Verification & Testing Framework

Validate your language host implementation against Pulumi's standard integration suite:

```bash
# 1. Run unit tests against gRPC interface mock
go test ./pkg/testing/...

# 2. Run engine end-to-end integration suite
pulumi test --language-host ./bin/pulumi-language-<lang>
```
