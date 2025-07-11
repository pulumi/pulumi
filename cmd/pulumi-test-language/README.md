(language-conformance-tests)=
# Language conformance tests

*Language conformance tests* (often just *conformance tests*) are a class of
integration test that assert various properties that should hold true across all
[language runtimes](language-runtimes), in essence providing a specification
that language runtimes must conform to. They are structured as follows:

* The "program under test" is expressed using [PCL](pcl). The program can
  specify resources and functions supplied by one or more test resource
  providers with fixed, known implementations, as well as provider-agnostic
  entities such as stack outputs.
* A set of assertions are made about the [state](state-snapshots) of the
  resources, functions, outputs, etc. in the program before and after it has
  been executed. These assertions should hold true regardless of the language
  being used to define and execute the program.

For each test run and language, then:

* If the test requires one or more providers, SDKs are generated from the
  relevant test providers, exercising SDK generation for the given language.
* The PCL program is converted into a program in the target language, exercising
  program generation for the given language.
* The generated program is executed using a compiled language host running as a
  separate process and the assertions are checked, exercising program execution
  for the given language.
* Generated code is snapshot tested to ensure that it doesn't change
  unexpectedly.

## Running

Since the conformance test is implemented separately for each language, running
them depends on the language you want to test. Conformance tests are typically
implemented for a language in a `language_test.go` file adjacent to the language
host's `main.go` file. For example, the NodeJS conformance tests are implemented
in [](gh-file:pulumi#sdk/nodejs/cmd/pulumi-language-nodejs/language_test.go),
next to the `pulumi-language-nodejs`'s `main.go`; Python at
[](gh-file:pulumi#sdk/python/cmd/pulumi-language-python/language_test.go) next
to `pulumi-language-python`'s `main.go`, and so on. To run the Python tests,
therefore, you can use a command such as:

```bash
go test github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3 -count 1
```

:::{note}
To run the above command from the root of the repository, you will likely need
an up-to-date `go.work` workspace definition. You can run `make work` to ensure
this.
:::

Alternatively, you can `cd` into the relevant directory and use `go test ./...`.
For the NodeJS tests, for instance:

```bash
(cd sdk/nodejs/cmd/pulumi-language-nodejs && go test ./... -count 1)
```

To run a single test, you can use the `-run` flag:

```bash
(cd sdk/python/cmd/pulumi-language-python && go test ./... -count 1 -run TestLanguage/default/l1-output-string)
```

Test names typically follow the pattern `TestLanguage/${VARIANT}/${TEST_NAME}`,
where `TEST_NAME` is the name of the test and `VARIANT` represents
language-specific variations in the conditions under which the test runs. For
instance, Python can generate SDKs in multiple ways, either using `setup.py` or
`pyproject.toml` to hold package metadata, or using different input types
(`classes-and-dicts` or `classes`). Python also supports both Mypy and Pyright
as typecheckers. Each test is thus currently run three times, with the following
variants:

* `default`, which uses `setup.py`, Mypy, and the default input types.
* `toml`, which uses `pyproject.toml`, Pyright, and `classes-and-dicts`.
* `classes`, which uses `setup.py`, Pyright, and `classes`.

Python tests can thus be run as `TestLanguage/default/${TEST_NAME}`,
`TestLanguage/toml/${TEST_NAME}`, and `TestLanguage/classes/${TEST_NAME}`. For
NodeJS, there are two variants: TypeScript (`forceTsc=false`) and plain
JavaScript (`forceTsc=true`; so named because the test setup runs `tsc` on the
project so it's runnable as plain JavaScript). Tests are thus named for example
as `TestLanguage/forceTsc=true/${TEST_NAME}` or
`TestLanguage/forceTsc=false/${TEST_NAME}`.

To update the snapshots for a conformance test, run with the `PULUMI_ACCEPT`
environment variable set to a truthy value:

```bash
PULUMI_ACCEPT=1 go test github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3 -count 1
```

## Debugging

The simplest way to debug one or more conformance tests is probably to use
VSCode with the sample [](gh-file:pulumi#.vscode/launch.json.example) file
copied to your `.vscode/launch.json` directory:

```bash
cp .vscode/launch.json.example .vscode/launch.json
```

With this, upon opening VSCode inside your `pulumi` worktree, you should have
options to run and debug the conformance tests for each language runtime. When
selecting an option, you'll be prompted for a test name. Here you should enter a
valid name or pattern, such as `l1-output-string` or `l1-*` to run all level 1
tests. You can then set and hit breakpoints that will be triggered directly by
the test process, which will include anything the language host runs itself --
code generation, for instance.

## Authoring

As mentioned above, conformance tests are defined using a series of PCL programs
and some Go code that makes assertions about the execution of language-specific
programs generated from that PCL. Presently they are laid out as follows:

* A test has a name, `l<N>-<name>`, where `N` is the "level" of the test (see
  the definitions in the [architecture](language-conformance-tests-arch) section
  below) and `name` is a descriptive name for the test. The `l1-output-string`
  test, for example, has a level of 1 and the name `output-string`.

* PCL programs are defined in `cmd/pulumi-test-language/tests/testdata`. Each test has
  its own directory, `l<N>-<name>`. If the test runs a single program once, this
  directory will typically just contain a `main.pp` file containing a PCL
  program, or several `.pp` files making up a whole. If the test runs multiple
  programs, it will contain subdirectories `0`, `1`, `2`, etc., each containing
  one or more `.pp` files to be run in order.

* Go tests (assertions, validations, etc.) are defined in
  `cmd/pulumi-test-language/tests`. Each test has its own file,
  `l<N>_<name>.go`, where any hyphens in `name` are replaced with underscores.
  So for the `l1-output-string` test, for instance, the file would be
  [](gh-file:pulumi#cmd/pulumi-test-language/tests/l1_output_string.go). Inside
  this file, an [`init`
  function](https://go.dev/ref/spec#Package_initialization) is used to add the
  test's specification to the list of tests that are served by the test server
  (see the [architecture](language-conformance-tests-arch) section below for
  more on the server and how it fits in). Each test is an object which specifies
  the program runs for the test and the assertions made off the back of each
  run.

* To add a new test, then, you would create a new directory in
  `cmd/pulumi-test-language/testdata` with the appropriate PCL program(s) and a
  new file in `cmd/pulumi-test-language/tests` with the assertions for that
  program.

### Writing assertions

`Assert` functions accept the following arguments:

* `projectDirectory string`
* `err error` -- any error that occurred during the run for which assertions are
  being written.

* `snap *deploy.Snapshot` -- the state snapshot produced by the run. This can be
  examined to make assertions about the state of resources and outputs modified
  by the run, for instance.

* `changes display.ResourceChanges` -- a list of resource change events produced
  by the run. This can be used to make assertions about the display relating to
  a run, or to observe intermediate steps (e.g. replacements) that may have lead
  to the final state snapshot.

### Stack resources and stack outputs

The stack resource for a test can be retrieved from the snapshot using the
`RequireSingleResource` helper function, since a stack resource should be the
only one of type `pulumi:pulumi:Stack`. It is *not* in general safe to assume
that the stack resource is the first resource in the snapshot -- you should
always favour using `RequireSingleResource`:

```
r := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
```

Stack outputs can be retrieved by inspect the outputs of the stack resource.

(language-conformance-tests-arch)=
## Architecture

Test providers are defined in
<gh-file:pulumi#cmd/pulumi-test-language/providers>. PCL programs for language
conformance tests are defined in
<gh-file:pulumi#cmd/pulumi-test-language/testdata>.
<gh-file:pulumi#cmd/pulumi-test-language/tests.go> then references these
programs and defines the assertions to be made about each. Tests are categorised
as follows:

* *L1 tests* are those which *do not* exercise provider code paths and use only
  the most basic of features (e.g. stack outputs).
* *L2 tests* are those which *do* exercise provider code paths and use things
  such as custom resources, function invocations, and so on.
* *L3 tests* exercise features that require more advanced language support such
  as first-class functions -- `apply` is a good example of this.

Each language defines a test function (the *language test host*) responsible for
running its conformance test suite, if it implements one. For core languages
whose runtimes are written in Go, this typically lives in a `language_test.go`
file next to the relevant language host executable code -- see for example
<gh-file:pulumi#sdk/nodejs/cmd/pulumi-language-nodejs/language_test.go> for
NodeJS/TypeScript and
<gh-file:pulumi#sdk/python/cmd/pulumi-language-python/language_test.go> for
Python. This function works as follows:

* The relevant [language runtime](language-runtimes) (e.g.
  `pulumi-language-nodejs` for NodeJS) is booted up as a separate process.
* The `pulumi-test-language` executable is booted up, with the language
  runtime's gRPC server address as a parameter. The test language executable
  itself exposes a gRPC server which allows clients to e.g. retrieve a list of
  tests (`GetLanguageTests`) and execute a test (`RunLanguageTest`).
* In preparation for test execution, the language test host retrieves the list
  of tests from the `pulumi-test-language` server. It
  [](pulumirpc.LanguageRuntime.Pack)s the core SDK (e.g. `@pulumi/pulumi` in
  TypeScript/NodeJS).
* For each test:
  * SDKs required by the test are generated by calling the language host's
    [](pulumirpc.LanguageRuntime.GeneratePackage) method. The generated code is
    written to a temporary directory where it is
    [](pulumirpc.LanguageRuntime.Pack)ed for use in the test.
  * The [](pulumirpc.LanguageRuntime.GenerateProject) method is invoked to
    convert the test's PCL code into a program in the target language.
    Dependencies are installed with
    [](pulumirpc.LanguageRuntime.InstallDependencies) and the test program is
    [](pulumirpc.LanguageRuntime.Run).
  * Assertions are verified and the next test is processed until there are no
    more remaining.

```mermaid
:caption: The lifecycle of a language conformance test suite
:zoom:

sequenceDiagram
    participant LTH as Language test host
    participant PTL as pulumi-test-language
    participant LH as Language host

    LTH->>+PTL: Start pulumi-test-language process
    PTL-->>-LTH: Return gRPC server address

    note right of LTH: All future calls to<br>pulumi-test-language are via gRPC

    LTH->>+PTL: GetLanguageTests()
    PTL-->>-LTH: List of test names

    LTH->>+LH: Start language host
    LH-->>-LTH: Return gRPC server address

    note right of LTH: All future calls to<br>language host are via gRPC

    LTH->>+PTL: PrepareLanguageTests(Language host)
    PTL->>+LH: Pack(Core SDK)
    LH-->>-PTL: Name of core SDK artifact
    PTL->>-LTH: Token

    loop For each test
        LTH->>+PTL: RunLanguageTest(Token, Language test host)

        loop For each SDK
            PTL->>+LH: GeneratePackage(SDK)
            LH-->>-PTL: Write package code to temporary directory
            PTL->>PTL: Verify SDK snapshot
            PTL->>+LH: Pack(SDK)
            LH-->>-PTL: Name of SDK artifact
        end

        PTL->>+LH: GenerateProject(Test)
        LH-->>-PTL: Write project code to temporary directory
        PTL->>PTL: Verify project snapshot

        PTL->>+LH: InstallDependencies(Project)
        LH-->>-PTL: Dependencies installed

        note right of PTL: Execute test with engine

        PTL->>+LH: Run(Project)
        LH-->>-PTL: Run result
        PTL->>PTL: Check assertions against run result and snapshot

        PTL-->>-LTH: Test result
    end
```

## Meta tests

This module contains a number of `_test.go` files. These are tests of the
conformance test system itself. The actual conformance tests are all defined in
`tests.go`.
