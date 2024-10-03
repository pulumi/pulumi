(unit-testing)=
## Unit tests

*Unit tests* generally fit the widely agreed definition of testing a single
"unit" of code in isolation. In the context of Pulumi, this might be a single
function, class, or module. Unit tests are applied judiciously in both the
engine (as standard `*_test.go` files) and the various language SDKs (using
language-specific frameworks such as Mocha for TypeScript and `unittest` for
Python).

### How and when to use
