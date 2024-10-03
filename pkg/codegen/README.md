(codegen)=
(crosscode)=
# Code generation

Code generation is essential to Pulumi's ability to support both a variety of
programming languages and a variety of cloud providers. This package defines the
core components of Pulumi's code generation functionality (known as [Pulumi
CrossCode](https://www.pulumi.com/crosscode/)). At a high level, code generation
is used to manage three categories of output: [SDKs](sdkgen),
[programs](programgen), and [documentation](docsgen). At a lower level, these
all make use of a number of shared concepts such as [schema](schema) and [Pulumi
Configuration Language (PCL)](pcl).

:::{toctree}
:maxdepth: 1
:titlesonly:

/pkg/codegen/sdks.md
/pkg/codegen/programs.md
/pkg/codegen/docs/README
/pkg/codegen/schema/README
/pkg/codegen/pcl/README
:::
