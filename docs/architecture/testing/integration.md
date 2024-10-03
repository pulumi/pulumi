(integration-testing)=
## Integration tests

Integration tests use the built binaries for the CLI and language runtimes. We have some helpers that allow us to create test scenarios and invoke CLI commands.

* [ProgramTest](gh-file:pulumi#pkg/testing/integration/program.go#L853): this is typically the easiest way to write an integration test. `ProgramTest` imports a Pulumi program from the `testdata` directory, and runs a series of CLI commands against it.
* [Environment](gh-file:pulumi#sdk/go/common/testing/environment.go#L42): this is lower level than `ProgramTest`. It takes care of creating a temporary directory to run commands in via `Environment.RunCommand`. Use `Environment.ImportDirectory` to import a test scenario into the environment.

### How and when to use
:::{attention}
Before running integration tests, make sure to rebuild the binaries your test will use.
```bash
# from the repostiory root, build and install `pulumi`
make build install
# from sdk/{python,nodejs,go}, build and install the required language runtimes
cd sdk/python
make build install
```
:::

To run a single integration test, run the following command from the repository root.

```bash
go test -tags=all github.com/pulumi/pulumi/tests/integration -run ${MY_TEST_TO_RUN}
```
