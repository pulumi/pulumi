# Automatic Automation

## Background

The automation API is a Pulumi project that allows users to write programs that
interface with Pulumi directly, rather than via the CLI. This allows you to
build truly dynamic infrastructure setups.

Under the hood, most of the automation API is just passing commands through to
the CLI behind a programmatic interface. However, there are exceptions: we can
define inline programs within the automation API, which means we have to
interface directly with the Pulumi gRPC. However, even this is very mechanical.

In fact, most of the Automation API is entirely predictable, given the shape of
the CLI. When we add a command to the CLI, we have to write more predictable
code for each backend. The plan with this project was to auto-generate that
code based on a JSON API that could be shared across all Automation API
provider languages, so that the cost of adding a new command to the Automation
API in all languages becomes editing a JSON file, rather than understanding and
updating seven different codebases in seven different languages.

## The future

I'd love to generate the JSON for this from Cobra directly, and thus the whole
process would be entirely automatic, but Cobra is lacking. The principal issue
is that I can't generate a specification for positional arguments. Apart from
that, everything else is reasonably well-structured. However, arguments are
validated using a predicate function, with a bunch supplied by the library for
simple cases such as `AtLeast` and `AtMost`.

We could potentially try to do something with function pointer equality, but
it's A) not guaranteed to work and B) not useful when users specify their own
custom function for parameter validation. We'd need to create a wrapper around
Cobra that turns these into a much more concrete spec, and then compiles it
down to predicate functions when we want to run the Cobra CLI.
