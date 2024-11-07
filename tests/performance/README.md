# Performance Tests

This package contains basic performance tests for the Pulumi CLI. These tests are intended to run as part of pull request and prevent us from introducing major performance regressions.

The [cli-performance-metrics](https://github.com/pulumi/cli-performance-metrics) repository contains more comprehensive performance tests that regularly run as a cron job.

## Thresholds

The thresholds used to determine if a test has passed or failed are highly dependent on the GitHub Actions Runners. Initial thresholds have been set by running the tests multiple times, taking the slowest run, and adding 10% (rounded to 100ms). These thresholds are not perfect and may need to be adjusted over time.

## Running the Tests

From the root of the repository, run:

```bash
make test_performance
```
