(converters)=
# Converters

*Converter plugins*, or simply *converters*, are the means by which Pulumi is
able to import, convert and migrate existing infrastructure as code (IaC)
programs and state from other ecosystems, such as Terraform or CloudFormation,
into Pulumi. Converters implement the [](pulumirpc.Converter) interface, which
defines the [](pulumirpc.Converter.ConvertProgram) method for converting program
source code and the [](pulumirpc.Converter.ConvertState) method for converting
existing state.

(program-conversion)=
## Program conversion

Program conversion is the process of converting an existing infrastructure as
code program written in one language (e.g. Terraform HCL) into a Pulumi program
that when run produces the same result. Program conversion is driven primarily
by the `pulumi convert` command, and typically proceeds as follows:

* A user runs `pulumi convert` on a *source program* written in a supported
  *source language* (e.g. Terraform HCL). Among other things, this command
  specifies a *target language* (e.g. TypeScript) and a *target directory* where
  the converted program will be written. For example, if we run a command like:

  ```bash
  pulumi convert --from tf --language typescript --out converted .
  ```

  then:

  * our source language will be Terraform HCL (as indicated by the `--from tf`
    argument);
  * our source program will be the current directory (`.`). In the case of
    Terraform, this means we will load all the `.tf` files we can find in the
    current directory and its subdirectories;
  * our target language will be TypeScript (as indicated by the `--language
    typescript` argument); and
  * our target directory will be `/converted`.

* `pulumi convert` ("the engine") loads a converter plugin based on the source
  language. In this case, the source language is `terraform`, so Pulumi will
  attempt to boot up a plugin named `pulumi-converter-terraform` (see [plugin
  loading and execution](plugin-loading-execution) for more information on how
  Pulumi loads plugins).

* The converter's [](pulumirpc.Converter.ConvertProgram) method is called to
  convert the source program into [PCL](pcl), Pulumi's intermediate
  representation from which target code can be generated. Like several other
  Pulumi processes, it is expected that the engine and the converter share a
  filesystem and that the converter writes PCL files itself as part of the call;
  the engine will send source and target directory information to the converter
  to facilitate this. Additionally, the engine will send the following
  information:

  * Any additional converter-specific arguments that may have been passed to
    `pulumi convert`
  * An address to a gRPC server that implements the [](codegen.Loader)
    interface, which the converter can use to load [schema](schema) for packages
    in order to facilitate PCL generation. This will typically be the address of
    the calling engine, which will broker calls to the appropriate
    [providers](providers) offering the necessary schema.
  * An address to a gRPC server that implements the [](codegen.Mapper)
    interface, which the converter can use to map names from the source language
    to names which match the relevant Pulumi schema. For instance, if converting
    a Terraform program, the converter might need to map a resource type such as
    `aws_s3_bucket` to the Pulumi type `aws:s3/bucket:Bucket`. Mapping is the
    process by which this occurs. As with schema loading, this address will
    typically be that of the calling engine, which will broker calls to the
    appropriate providers. Mapping is discussed in more detail in [its own
    section](converter-mapping) below.

* The converter is expected to write PCL files to the target directory that
  represent the source program in a language-agnostic way. As indicated above,
  it may use the provided schema loading and mapping services to facilitate
  generating PCL that references the appropriate PCL packages, types, property
  names, and so on.

* Once the call to [](pulumirpc.Converter.ConvertProgram) has completed, the
  engine will have a set of PCL files that represent the source program in a
  language-agnostic way. The engine will then call the appropriate [language
  host's](language-hosts) ["programgen"](programgen) features to generate a
  Pulumi program in the target language.

The following diagram illustrates this process:

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    participant P as Provider
    box Engine
        participant E as Engine
        participant SL as Schema loader
        participant M as Mapper
    end
    participant C as Converter

    Note right of E: Conversion starts
    Note right of E: PCL generation starts

    E->>+C: ConvertProgram(source, target, args, SL, M)

    loop As necessary
        C->>+SL: GetSchema(package)
        SL->>+P: GetSchema(package)
        P->>-SL: Schema(package)
        SL->>-C: Schema(package)

        C->>+M: GetMapping(name, hint)

        Note right of M: If supplied, use hint to find an appropriate plugin

        M->>+P: GetMappings(name)
        P->>-M: Mappings(name)
        M->>+P: GetMapping(name)
        P->>-M: Mapping(name)
        M->>-C: Mapping(name)
    end

    C->>-E: Done(target PCL)

    Note right of E: PCL generation completes
    Note right of E: Target language code generation starts

    E->>+LH: GenerateProject(target PCL)

    loop As necessary
        LH->>+E: GetSchema(package)
        E->>+P: GetSchema(package)
        P->>-E: Schema(package)
        E->>-LH: Schema(package)
    end

    LH->>-E: Done(target program)

    Note right of E: Target language code generation completes
    Note right of E: Conversion completes
```

(state-conversion)=
## State conversion

(converter-schema-mapping)=
(converter-schema)=
(converter-mapping)=
## Schema loading and mapping

Suppose we want to convert the following Terraform program:

```hcl
resource "aws_s3_bucket" "foo" {
    bucket_name = "my-bucket"
}
```

We might expect the following PCL output:

```hcl
resource "aws:s3/bucket:Bucket" "foo" {
    bucketName = "my-bucket"
}
```

We know from other parts of this document that the `pulumi-converter-terraform`
plugin will almost certainly be involved, but how is the plugin to know that:

* The `aws_s3_bucket` resource has an equivalent provided by the Pulumi `aws`
  package?
* In the Pulumi `aws` package, the `aws:s3/bucket:Bucket` type is used to
  represent an S3 bucket?

This is where *schema loading* and *mapping* come in. Converting programs and
state requires knowledge of various Pulumi identifiers and conventions, such as
which Pulumi package maps to a given Terraform provider (e.g. Pulumi's `gcp` to
Terraform's `google`), or how some source resource name must be transformed to
produce a Pulumi equivalent (e.g. turning Terraform's `aws_s3_bucket` into
Pulumi's `aws:s3/bucket:Bucket`). Rather than baking this knowledge into each
converters, plugins are furnished with the addresses of [](codegen.Loader) and
[](codegen.Mapper) gRPC servers that they can use to load schema and map names
respectively. Since in general this sort of information is provider-specific
(e.g. the `gcp` provider knows specifically that Terraform resources named
`gcp_*` can be mapped to Pulumi types of the form `gpc:*/*:*`, etc.), the engine
exposes implementations of these services that broker calls to the appropriate
provider. Specifically, [providers](providers) (through the
[](pulumirpc.ResourceProvider) interface) offer the following methods:

* [](pulumirpc.ResourceProvider.GetSchema), which returns the schema for the
  package offered by the provider
* [](pulumirpc.ResourceProvider.GetMappings), which accepts a "conversion key"
  (the source language being converted from, such as `terraform`) and returns a
  list of names representing packages *in the source language* for which
  mappings are available. So for instance, if the Pulumi `gcp` provider maps to
  the Terraform `google` provider, calling `GetMappings("terraform")` on an
  instance of the `gcp` plugin might return the list `["google"]`.
* [](pulumirpc.ResourceProvider.GetMapping), which accepts a conversion key and
  a *source language provider name*, and returns mapping information for that
  name. So, for instance, calling `GetMapping("terraform", "google")` on the
  `gcp` plugin would return mapping information for transforming Terraform
  programs using `google_*` resources into Pulumi programs using the `gcp`
  package. In this context, "mapping information" is converter-specific, but
  will typically be some map from the various kinds of names in the source
  language to their Pulumi equivalents.

When the engine receives a request to load a schema or mapping from a converter,
it will iterate through the set of plugins it knows about and do its best to
pass back the most suitable match to the converter.

### Parameterized plugins and hints

On the surface, a request of the form "find me the mapping for the `aws`
Terraform provider" is an easy one to satisfy -- the engine simply needs to find
a plugin called `pulumi-resource-aws`, boot it up, call
`GetMappings("terraform")`, see that `aws` is in the resulting list, call
`GetMapping("terraform", "aws")`, and return the result. In the presence of
[parameterized providers](parameterized-providers), however, things are not
always so straightforward. For instance, if the `aws` package is provided by an
instance of the `terraform-provider` plugin, which is [dynamically
bridging](https://www.pulumi.com/blog/any-terraform-provider/) the Terraform AWS
provider, then the plugin the engine needs to load is in fact
`pulumi-resource-terraform-provider`. Moreover, after loading that plugin, the
engine will need to make a [](pulumirpc.ResourceProvider.Parameterize) call to
trigger the requisite bridging before requesting schema or mappings. We might
ask then, how can the engine know whether to look for a package name literally,
or to find another plugin and parameterize it?

For loading schema, this problem is already solved with *package descriptors*,
which can capture both a plugin name and an optional parameterization. For
mappings, a slightly different approach is used, in the form of *hints* (e.g.
[](codegen.MapperParameterizationHint) at the gRPC layer). Hints are
instructions from a converter plugin that tell the engine where it might find
the mappings being requested. These look similar to package descriptors, in that
a hint may contain a plugin name and an optional parameterization. If the engine
finds a plugin whose name matches that in the given hint, it will use the hint's
parameterization (if present) before making the mapping request.
