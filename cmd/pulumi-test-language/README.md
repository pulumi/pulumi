pulumi-language-test runs a gRPC interface that language plugins can use to run a suite of standard tests.

# Architecture

pulumi-language-test is used to run a standard suite of tests against 
any compliant language plugin.

The diagram below shows the main interactions and data flows for how this system works.

There are three main actors involved. Firstly `test` which is a test function coordinating the language plugin and pulumi-language-test. Secondly `ptl` which is the pulumi-language-test process. Finally `uut` which is the language plugin actually being tested. This will generally be a grpc server running in the same process as the test method.

```mermaid

sequenceDiagram
    test->>ptl: Start ptl process
    ptl-->>test: Read stdout for ptl address to connect to

    Note right of test: All future calls to ptl are via grpc

    test->>+ptl: GetLanguageTests()
    ptl-->>-test: Returns list of test names

    test->>+uut: Serve
    uut-->>-test: Returns uut server address

    test->>+ptl: PrepareLanguageTests(uut)
    ptl->>+uut: Pack(core)
    uut-->>-ptl: Returns name of core artifact
    ptl-->>-test: Returns `token`

    loop for each test
        test->>+ptl: RunLanguageTest(token, test)
        loop for each sdk used in test
            ptl->>+uut: GeneratePackage(sdk)
            uut-->>-ptl: Write package code to temporary directory
            ptl->>ptl: Verify sdk snapshot
            ptl->>+uut: Pack(sdk)
            uut-->>-ptl: Returns name of sdk artifact
        end

        ptl->>+uut: GenerateProject(test)
        uut-->>-ptl: Write project code to temporary directory
        ptl->>ptl: Verify project snapshot

        ptl->>+uut: InstallDependencies(project)
        uut-->>-ptl: 

        note right of ptl: Execute test with engine
        activate ptl
        ptl->>+uut: Run(project)
        uut-->>-ptl: Return run result
        ptl->>ptl: Run test asserts against run result and snapshot
        deactivate ptl

        ptl-->>-test: Returns test result from asserts
    end

    test->>ptl: sigkill
```

## Meta tests

This module contains a number of `_test.go` files. These are tests of the conformance test system itself. The actual conformance tests are all defined in `tests.go`. 