(sdkgen)=
# SDKs

*[Provider](providers) SDKs* ("software development kits") are generated from a
[Pulumi Schema](schema) definition. Often referred to as "SDKgen", this process
is used by the myriad providers supported by Pulumi to expose their resources,
components, and functions in an idiomatic way for a given language. SDKgen is
generally exposed through the [](pulumirpc.LanguageRuntime.GeneratePackage)
method of a [language host](language-hosts), which in turn is exposed by the
CLI's [`pulumi package
gen-sdk`](https://www.pulumi.com/docs/cli/commands/pulumi_package_gen-sdk/)
command. At a code level, SDKgen starts with the relevant `GeneratePackage` Go
function in `gen.go` -- see <gh-file:pulumi#pkg/codegen/nodejs/gen.go> for
NodeJS, <gh-file:pulumi#pkg/codegen/python/gen.go> for Python, and so on.

:::{note}
The `pulumi package gen-sdk` command is not really intended to be used by
external users or customers, and instead offers a convenient interface for
generating provider SDKs as part of e.g. the various provider CI jobs used
to automate provider build and release processes.
:::
