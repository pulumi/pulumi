# Coconut JavaScript (CocoJS)

CocoJS is a superset of a JavaScript (ECMAScript) subset.  First, take a subset of JavaScript, and then superset it with
optional typing annotations (using TypeScript's syntax and semantics for them).

## Modules

CocoJS uses ES6-style modules.

An [ES6 module is a special kind of script](http://www.ecma-international.org/ecma-262/6.0/#sec-scripts-and-modules),
which is just a file containing a list of top-level statements.  NutIL modules have a bit more structure to them to
facilitate analysis and determinism.  As a result, there is a mapping from CocoJS to NutIL module structure.

The mapping simply records all declarations -- variables and functions -- and then moves all other statements into the
special module initializer (`.init`) function.  In the case of blueprints, the special entrypoint function `.main` is
devoid of logic, because in Node.js-style of programming, the module initializer takes care of entrypoint functionality.

## TODO: document more

