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
