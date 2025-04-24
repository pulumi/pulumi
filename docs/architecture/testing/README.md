(testing)=
# Testing

The surface are of `pulumi/pulumi` is *large*. A single release of "Pulumi"
encapsulates both a version of the deployment engine and CLI, for multiple
platforms (e.g. Linux, macOS and Windows), and a full set of language SDKs
(across TypeScript/NodeJS, Python, Go, .Net, Java, YAML, and maybe more by the
time you are reading this). Automated testing is a critical part of making sure
that things work as expected without requiring undue manual intervention. Within
the repository there are a number of different types of tests that are run as
part of the development, CI/CD and release processes.

(codegen-tests)=
## Code generation tests

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/architecture/testing/unit
/docs/architecture/testing/integration
/pkg/engine/lifecycletest/README
/cmd/pulumi-test-language/README
:::
