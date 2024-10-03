(integration-testing)=
## Integration tests

Integration tests use the built binaries for the CLI and language runtimes. We have some helpers that allow us to create test scenarios and invoke CLI commands.

* [ProgramTest](https://github.com/pulumi/pulumi/blob/master/pkg/testing/integration/program.go#L853): this is typically the easiest way to write an integration test. `ProgramTest` imports a Pulumi program from the `testdata` directory, and runs a series of CLI commands against it.
* [Environment](https://github.com/pulumi/pulumi/blob/master/sdk/go/common/testing/environment.go#L42): this is lower level than `ProgramTest`. It takes care of creating a temporary directory to run commands in via `Environment.RunCommand`. Use `Environment.ImportDirectory` to import a test scenario into the environment.

### How and when to use

To run a single integration test, run the following command

```bash
go test -tags=all github.com/pulumi/pulumi/tests/integration -run ${MY_TEST_TO_RUN}
```
