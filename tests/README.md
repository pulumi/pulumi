# Integration Tests

This module provides integration tests for the Pulumi CLI. 

The tests can be run via:

``` sh
make test_all
```

## Usage of Go build tags

In order to speed up integration tests in GitHub actions, Go build tags are used to conditionally compile the desired test cases.

```go
// integration_nodejs_test.go
//go:build (nodejs || all) && !xplatform-acceptance

// integration_nodejs_acceptance_test.go
//go:build nodejs || all
```
