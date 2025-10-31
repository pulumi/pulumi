(lifecycle-tests)=
# Lifecycle tests

*Lifecycle tests* exercise the Pulumi engine and serve as a specification for
the behaviours and interactions of the various features that define the
lifecycle of a Pulumi program. This includes, but is not limited to:

* The operation(s) being executed (`up`, `preview`, etc.) and the options passed
  to that operation (`--target`, `--target-dependents`, etc.).
* The programs being executed -- their resources, invocations, and the various
  options that might be associated with them (`parent`, `retainOnDelete`, etc.).
* The state of the program before and after operations are executed.

## How and when to use

```{include} /pkg/engine/lifecycletest/fuzzing/README.md
```
