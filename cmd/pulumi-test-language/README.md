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

## Running Language Conformance Tests

To run the language conformance tests, for example for Python, run:

```bash
go test github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3 -count 1
```

Note: to run this from the root of the repository, make sure you have an uptodate `go.work` file by running `make work`.

The conformance tests are named `TestLanguage/${TEST_NAME}`, for example `TestLanguage/l1-output-string`.

Python can generate SDKs in multiple ways, either using `setup.py` or `project.toml` to define the python package metadata, as well as using different input types (`classes-and-dicts` or `classes`). Python also supports either `MyPy` or `PyRight` as typechecker. To ensure we cover all of these options, each test is run in 3 variants (the options are orthogonal to each other, so we don't run all combinations):

* `default` (uses `setup.py`, `MyPy` and the default input types)
* `toml` (uses `pyproject.toml`, `PyRight` and `classes-and-dicts`)
* `classes` (uses `setup`, `PyRight` and `classes`)

Test names follow the pattern `TestLanguage/${VARIANT}/${TEST_NAME}`, for example `TestLanguage/classes/l1-output-string`.

For Nodejs we have two variants, using TypeScript (`forceTsc=false`) or plain Javascript (`forceTsc=true`, so named because the test setup runs `tsc` on the project so it's runnable as plain Javascript). Tests are named for example `TestLanguage/forceTsc=true/l1-output-string` or `TestLanguage/forceTsc=false/l1-output-string`.

To run a single test case:

```bash
go test github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3 -count 1 -run TestLanguage/classes/l1-output-string
```

To update the snapshots for generated code, run the tests with the `PULUMI_ACCEPT` environment variable set to a truthy value:

```bash
PULUMI_ACCEPT=1 go test github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3 -count 1
```

## Assertions in Language Conformance Tests

Each language conformance TestRun Assert (l[123]-some-test-name) will take in the following arguments:

    - projectDirectory string
    - err error, any errors that occurred during the test run.
    - snap *deploy.Snapshot, the [snapshot](state-snapshots) of a run.
    - changes display.ResourceChanges, any resource changes as a result of the run

### Stack Resource

The stack of the test run itself will have the URN `pulumi:pulumi:Stack`, and
outputs of this resource are the outputs of the pulumi program written.

You can get this resource specifically (it is **not* always the first
Resource!) by requring a single resource with the helper function
[](tests.RequireSingleResource) like so:

```
r := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
```

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
