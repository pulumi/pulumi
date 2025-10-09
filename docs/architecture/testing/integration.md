(integration-testing)=
## Integration tests

Integration tests use the built binaries for the CLI and language runtimes. We have some helpers that allow us to create test scenarios and invoke CLI commands.

* [ProgramTest](gh-file:pulumi#pkg/testing/integration/program.go#L853): this is typically the easiest way to write an integration test. `ProgramTest` imports a Pulumi program from the `testdata` directory, and runs a series of CLI commands against it.
* [Environment](gh-file:pulumi#sdk/go/common/testing/environment.go#L42): this is lower level than `ProgramTest`. It takes care of creating a temporary directory to run commands in via `Environment.RunCommand`. Use `Environment.ImportDirectory` to import a test scenario into the environment.

### How and when to use

Prefer [language conformance tests](language-conformance-tests) over integration tests where possible. Language conformance tests ensure that we test features across all of our languages in a uniform manner. For functionality that is not exercised during a pulumi operation (preview, up, destroy, ...), an integration test is appropriate.

:::{attention}

Integration tests should have a TestMain which calls the `testutils.SetupPulumiBinary()` method to set an explicit path to the binaries under test to avoid reliance on the `$PATH` which can cause the wrong binary to be used in tests, resulting in incorrect test results.

:::

You can set `PULUMI_INTEGRATION_REBUILD_BINARIES=true` in your environment to automatically re-build the binaries locally to your repository and have the integration tests use them automatically.

To test an alternative `pulumi` binary, set the environment variable `PULUMI_INTEGRATION_BINARY_PATH` to the absolute path of the binary you want to test.

### Organization

Integration tests are located in [tests](gh-file:pulumi#tests) and loosely organized:

[Smoke tests](gh-file:pulumi#tests/smoke) are intended to be small tests that exercise a high level pulumi command, matrixed over our supported languages where applicable.

[Performance tests](gh-file:pulumi#tests/performance) are basic performance tests that run as part of pull request and prevent us from introducing major performance regressions.

The bulk of the tests are in [tests/integration](gh-file:pulumi#tests/integration), but some pulumi commands have their own top level directories, for example `pulumi config` is tested in [tests/config](gh-file:pulumi#tests/config). Within [tests/integration](gh-file:pulumi#tests/integration) tests are either split by language [integration_python_test.go](gh-file:pulumi#tests/integration/integration_python_test.go), [integration_nodejs_test.go](gh-file:pulumi#tests/integration/integration_nodejs_test.go), ... or by feature in separate directories, for example [tests/integration/transforms](gh-file:pulumi#tests/integration/transforms/transforms_test.go).

#### Build the required binaries

```bash
# From the repository root, build the Pulumi CLI and the Go, Python and Node.js language runtimes.
make build
# You can also build individual SDKs
SDKS="nodejs python" make build
# or just the main Pulumi CLI
SDKS= make build
# The Node.js TypeScript SDK needs to be built separatly
cd sdks/nodejs && make build install
```

To run a single integration test, run the following command from the repository root.

```bash
go test -tags=all github.com/pulumi/pulumi/tests/integration -run ${MY_TEST_TO_RUN}
```
