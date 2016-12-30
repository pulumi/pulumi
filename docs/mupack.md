# Mu Package Metadata (MuPack)

This document describes the overall concepts, capabilities, and serialization format for MuPack.  It also describes the
intermediate language (IL) and type system for MuPack, something we refer to as MuIL.

## Overview

Each MuPack file is called a MuPackage and contains four things:

* Package metadata.
* Symbol names and tokens.
* Module, type, function, and variable definitions.
* Data and computations encoded in an intermediate language (IL).

The metadata section describes attributes about the overall MuPackage, like its name and version.

All data and computations are fully bound to types, and ready for interpretation/execution.  Higher level MetaMu
language compilers are responsible for performing this binding, and encoding the results.  Those results are symbol
names and tokens that are registered and available for lookup within any given MuPackage.  These symbols provide a
quick, and binding logic-free, way of resolving any bound node to its target abstraction (module, type, or function).
From there, any data or computations associated with those abstractions may be retrieved thanks to the definitions.

Mu's type system was designed to be supported by a broad cross-section of modern programming languages.  That said,
it's entirely possible that MuPack exposes a construct that a certain language doesn't support.  Because MuIL is
designed for interpretation, determinism, and predictability -- and not runtime speed -- all type coercions are checked
and fail-fast if an illegal coercion is attempted.  It is obviously a better choice to verify such conversions where
possible in the MetaMu compilers themselves, however this approach naturally accomodates dynamic languages.

MuPack is serialized in JSON/YAML form, although in the future we may explore more efficient file formats.  Examples in
this document will use a YAML syntax for brevity's sake.

TODO: hello, world.

## Metadata

Each package may contain self-describing metadata, such as a name, and optional attributes that are common in package
managers, like a description, author, website, license, and so on.  For example:

    name: acmecorp/elk
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@elk.com>
    website: https://github.com/joesmith/elk

TODO(joe): describe the full informational attributes available.

Most of this information is taken from the source MetaMu program and carried forward during compilation.

Note that a version number is *not* part of the metadata prelude.  The version number is managed by a version management
system outside of the purview of the package contents.  For example, packages checked into Git are managed by SHA1
hashes and tags, while packages registered with a traditional package management system might be versioned manually.

## Names

As with most metadata formats, names are central to how things reference one another.  The two key concepts to
understand in how MuPack and MuIL encode such references are *symbols* and *tokens*.

### Symbols

Each symbol represents one of four kinds of abstractions: module, variable, function, or a class:

* A *module* represents a collection of said abstractions.  No type, function, or const can exist outside of one.  Every
  MuPackage consists of at least one top-level module, with optional nested modules inside of it.

* A *variable* represents a named, typed storage location.

* A *function* represents a named computation with typed parameters and an optional typed return value.

* A *class* is a named abstraction that contains properties (variables) and behavior (functions).  Classes have been
  designed to support both nominal and structural typing, catering to a subset of most dynamic and static languages.

### Tokens

Each symbol is keyed by a token, which is a unique identifier used to reference the symbol from elsewhere inside and/or
outside of the defining module.  Each token is just a string name that encodes the entire context necessary to resolve
it to a concrete module and, within that module, its corresponding definition:

* A protocol (e.g., `https://`).
* A base URL (e.g., `hub.mu.com/`, `github.com/`, etc).
* A fully qualified name (e.g., `acmecorp`, `aws/s3/Bucket`, etc).

For example, `https://hub.mu.com/acmecorp#latest` refers to the latest version of the `acmecorp` module, while
`https://github.com/aws/s3/Bucket` refers to the `Bucket` class exported from the `aws/s3` module (itself exported from
the `aws` package).  The URLs are present so that package managers can download dependencies appropriately.

Each MuPackage contains a concrete list of module dependencies.  For example:

    dependencies:
        - https://hub.mu.com/aws#^1.0.6
        - https://hub.mu.com/github#~1.5.2

Now, throughout the rest of the MuPackage, any symbol tokens prefixed with `https://hub.mu.com/aws` and
`https://hub.mu.com/github` will be resolved to the artifacts exported by the repsective packages.  Note that
dependencies are required to be acyclic.

Notice that dependency names are like ordinary token names, but must also carry a version number:

* An `#` followed by version number (e.g., `#^1.0.6`, `#6f99088`, `#latest`, etc).

This version number ensures that the same dependency used for compilation is used during evaluation.  Mu supports
multiple versioning formats (semantic versioning, Git SHA1 hash versioning, and "tip" (`latest`)).  Please refer to
[Mu Dependencies](deps.md) for more information about token and dependency names and the resolution process.

MuPackages may export other symbols in the form of modules, types, variables, and functions as members.

### Naming Conventions

TODO: talk about casing.

## Types

This section describes the core aspects of MuIL's type system.

There is a single top-type that may refer to any record or class value: the `any` type.

All instances of classes in MuIL are called *objects*.  They are allocated on the heap, in map-like data structures that
have strong type identity, facilitating dynamic and structural conversions, in addition to classical RTTI and OOP
patterns.  In a sense, this is a lot like how ECMAScript works.  Furthermore, there is no notion of a pointer in MuIL
and so the exact storage location is kept hidden from MetaMu languages and their semantics.

Because all instances are objects, we must talk about `null`.  By default, types do not include the special value `null`
in their domain.  To include it in a type's domain, suffix athat type `T` with a question mark, as in `T?`.

### Primitives

At the core, all types are built out of the primitives:

* The basic primitive types: `bool`, `number`, and `string`.

* Any record `S` can be modified by appending `[]` to make an array type `S[]`: e.g., `number[]` and `string[]`.

* Similarly, two types can be paired up to make a map type using `map[K]V`, where `K` is the type of keys used to
  index into the map and `V` is the type of value inside: e.g., `map[string]number` and `map[string]record`, and so on.
  Note that only the primtive types `bool`, `number`, and `string` can be used as keys for a map type.  A map type with
  a value type `V` that belongs to the `record` subset of types is also a `record`; otherwise, it is a `class`.

As with JSON, all numbers are [IEEE 754 64-bit floating point numbers](
https://en.wikipedia.org/wiki/IEEE_floating_point).

TODO(joe): we likely want ints/longs.  Perhaps not in the JSON-like subset, however.  Maybe even bignum.

## Definitions

Each package contains a definitions map containing all modules, variables, functions, and classes:

    definitions:

This map is laid out in a hierarchical manner, so that any types belonging to a module are nested underneath it, etc.,
making the name resolution process straightforward.  It contains both internal and exported members.

### Modules

Because each package has an implicit top-level module, the `definitions:` element itself is actually a module
specification.  Every module specification may contain up to the four kinds of members underneath it listed earlier:

    definitions:
        modules:
            # submodules, keyed by name
        variables:
            # variables, keyed by name
        functions:
            # functions, keyed by name
        classes:
            # classes, keyed by name

Each of these elements may be made accessible outside of the package by attaching the `export: true` attribute:

    export: true

A module may contain definitions that aren't exported simply by leaving off `export: true` or explicitly marking a
definition as `export: false`.  These are for use within the package only.  No additional accessibility level is
available at the MuPack level of abstraction, although of course MetaMu languages may project things however they wish.

Modules may contain a single special function, called its "initializer", to run code at module load time.  It is denoted
by the special name `.init`.  Any variables with complex initialization must be written to from this initializer.

The sole top-level module for an executable MuPackage may also contain a special entrypoint function that is used to
perform graph evaluation.  It is denoted by the special name `.main`.  It is illegal for non-executable MuPackages to
contain such a function, just as it is for a submodule to contain one (versus the MuPackage's top-level module).

### Variables

A variable is a typed, named storage location.  As we will see, variables show up in several places: as module
properties, struct and class properties, function parameters, and function local variables.

Each variable definition, no matter which of the above places it appears, shares the following attributes:

* A `name` (its key).
* An required `type` token.
* An optional `default` value.
* An optional `readonly` indicator.
* An optional informative `description`.

TODO(joe): `secret` keyword for Amazon NoEcho-like cases.

The following is an example variable that demonstrates several of these attributes:

    availabilityZoneCount:
        description: A map from AZ to the count of its subzones.
        type: map[string]number
        default:
            "us-east-1": 3
            "us-east-2": 3
            "us-west-1": 3
            "us-west-2": 3
            "eu-west-1": 3
            "eu-central-1": 2
            "ap-northeast-1": 2
            "ap-southeast-1": 2
            "ap-southeast-2": 3

A variable can be given a `default` value if it can be represented using a simple serialized literal value.  For more
complex cases, such as using arbitrary expressions, initialization must happen in a function somewhere.  For module
properties, for instance, this occurs in the module initialzer; for class properties, in its constructor; and so on.

By default, each variable is mutable.  The `readonly` attribute indicates that it isn't:

    availabilityZoneCount:
        type: map[string]number
        readonly: true
        # ...

As with most uses of `readonly` in other programming languages, it is shallow (that is, the property value cannot be
changed by if the target is a mutable record, properties on *that* record can be).

All variables are initialized to `null` by default.  Problems may arise if the type is not nullable (more on this
later).  All loads guard against this possibility, however loading a `null` value from a non-null location will lead to
runtime failure (which can be difficult to diagnose).  It is better if MetaMu compilers ensure this cannot happen.

### Functions

A function is a named executable computation, with typed parameters, and an optional typed return value.  As we will
see, functions show up in a few places: as module functions, class functions, and lambdas.

All function definitions have two following common attributes:

* An optional list of parameters, each of which is a variable.
* An optional return type.

Module and class functions are required to also carry a `name` (the function's key).  Lambdas do not have one.

Module functions and lambdas are required to have a MuIL `body`.  Class functions often have one, but it can be omitted,
in which case the enclosing class must be abstract and concrete subclasses must provide a `body`.

### Classes

New named `class` types can be created by composing primitives and other `class` types.  Classes are capable of
representing an array of constructs, from pure-data, to pure-interfaces, to everything in between.  Because Mu's
primitive type system is restricted to JSON-like types, all instances can be serialized as [JSON](
https://tools.ietf.org/html/rfc7159), ensuring interoperability with languages and Internet protocols.  There may be
additional constraints placed on classes to enforce contracts, but this does not alter their runtime representation.

Each custom `class` type has the following attributes:

* A `name` (its key).
* An optional `base` type.
* An optional informative `description`.
* An optional set of `properties` and/or `functions`.

For instance, here is an example of a pure data-only `Person` class:

    Person:
        description: A person.
        properties:
            firstName:
                type: string
                description: The person's given name.
            lastName:
                type: string
                description: The person's family name.
            age:
                type: number
                description: The person's current age.

All properties are variable definitions and functions are function definitions, per the earlier descriptions.

Class properties are special in one important way: each property is, by default, a *primary property*.  Although classes
can have constructors, primary properties are set by the calling object initializer *before* invoking that constructor.
In fact, unless the type of a property is nullable, a value will be *required* at initialization time.  As a result,
data-only classes like the above `Person` class may omit a constructor altogether.  A constructor can, however, run
afterwards to initialize more complex properties, possibly based on constructor arguments.  Properties that will be
initialized in this manner can be marked `computed: true` so that callers need not manually initialize them.

#### Subtyping

Classes may subtype other record type using the `base:` element.

For example, imagine we want an `Employee` which is a special kind of `Person`:

    Employee:
        description: An employee person.
        base: Person
        properties:
            company:
                type: string
                description: The employee's current employer.
            title:
                type: string
                description: The employee's current title.

This facilitates easy conversion from an `employee` value to a `person`.  Because MuIL leverages a nominal type system,
the parent/child relationship between these types is preserved at runtime for purposes of RTTI.  This caters to MetaMu
languages that use nominal type systems as well as MetaMu languages that use structural ones.

At the moment, there is support for covariance (i.e., strengthening properties).  All base-type properties are simply
inherited "as-is".

### Conversions

Although schemas are nominal, they also enjoy convenient structural conversions in the language without undue ceremony.

TODO: RTTI, type identity, structual conversions, isinst vs type (Python-esque), etc.

### Lambda Types

### Advanced Types

MuIL supports some additional "advanced" type system features.

#### Constraints

To support rich validation, even in the presence of representations that faciliate data interoperability, MuIL supports
additional constraints on `number`, `string`, and array types, inspired by [JSON Schema](http://json-schema.org/):

* For `number`s:
    - Minimum: `number<M:>`, where `M` is a constant `number` of the minimum (inclusive) value.
    - Maximum: `number<:N>`, where `N` is a constant `number` of the maximum (inclusive) value.
    - Both minimum and maximum: `number<N:M>`.
* For `string`s:
    - Exact length in characters: `string<L>`, where `L` is a constant `number` of the exact length.
    - Minimum length in characters: `string<M:>`, where `M` is a constant `number` of the minimum (inclusive) length.
    - Minimum length in characters: `string<:N>`, where `N` is a constant `number` of the maximum (inclusive) length.
    - Both minimum and maximum: `string<N:M>`.
    - A regex pattern for legal values: `string<"R">`, where `"R"` is a regex representing valid patterns.
* For arrays:
    - Exact length: `T[L]`, where `L` is a constant `number` of the exact length.
    - Minimum length: `T[M:]`, where `M` is a constant `number` of the minimum (inclusive) length.
    - Maximum length: `T[:N]`, where `N` is a constant `number` of the maximum (inclusive) length.
    - Both minimum and maximum: `T[N:M]`.

As examples of these, consider:

    number<512:>            // min 512 (incl)
    number<:1024>           // max 1024 (incl)
    number<512:1024>        // min 512, max 1024 (both incl)
    
    string<8>               // exactly 8 chars
    string<2:>              // min 2 chars (incl)
    string<:8>              // max 8 chars (incl)
    string<2:8>             // min 2, max 8 chars (incl)
    
    string<"a-zA-Z0-9">     // regex only permits alphanumerics
    
    number[16]              // exactly 16 numbers
    number[8:]              // min 8 numbers (incl)
    number[:16]             // max 16 numbers (incl)
    number[8:16]            // min 8, max 16 numbers (incl)

These constructs are frequently useful for validating properties of schemas without needing custom code.

#### Union and Literal Types

A union type is simply an array containing all possible types that a value might resolve to.  For example, the type
`[ string, number ]` resolves to either a string or number value at runtime.

A literal type is a type with an arbitrary string value.  A literal type is silly to use on its own, however, when
combined with union types, this provides everything we need for strongly typed enums.

For example, imaging we wish our `state` property to be confined to the 50 states:

    properties:
        state:
            type: [ "AL", "AK", ..., "WI", "WY" ]

A compiler should check that any value for the `state` property has one of the legal string values.  If it doesn't,
MuIL runtime validation will ensure that it is the case.

#### Type Aliases

Any type `A` can be used as an alias for another type `B`, simply by listing `B` as `A`'s base type.

For instance, imagine we want to alias `Employees` to mean an array of `Employee` records, or `Employee[]`:

    Employees:
        base: Employee[]

This can be particularly useful for union/literal enum types, such as our state example above:

    State:
        base: [ "AL", "AK", ..., "WI", "WY" ]

Now, given this new `State` type, we can simplify our `state` property example from the previous section:

    properties:
        state:
            type: State

## Mu Intermediate Language (MuIL)

Loads/stores
    Load constants (null, number, string)
    Load/store variable (modvar, field, local)
    Load/store map element (same as variable?)
    Load/store array element
    Array and map intrinsics (ldlen)
    Different for static vs. dynamic load?
Branches (ble, bge, lt)
Calls
Lambdas
New (records and classes) / init
Conversion, isinst, casts(structural plus nominal)
Throw
Try/Catch/Finally
Operators

## Possibly-Controversial Decisions

It's worth describing for a moment some possibly-controversial decisions about MuPack and MuIL.

These might come as a surprise to higher level programmers, however, it is worth remembering that MuIL is attempting to
strike a balance between high- and low-level multi-language representations.  In doing so, some opinions had to be
discard, while others were strengthened.  And some of them aren't set in stone and may be something we revisit later.

### Generics

MuIL does not support generics.  MetaMu languages are free to, however they must be erased at compile-time.

This admittedly sacrifices some amount of generality.  But it does so at the benefit of simplicity.  Some MetaMu
languages simply do not support generics, and so admitting them to the core would be problematic.  Furthermore,
languages like Go demonstrate that modern cloud programs of considerable complexity can be written without them.

Perhaps the most unfortunate and apparent aspect of MuIL's lack of generics is the consequently missing composable
collection types.  To soften the blow of this, MuIL has built-in array, map, and enumerable object types.

### Operators

MuIL does come with a number of built-in operators.

MuIL does not care about operator precedence, however.  All expressions are evaluated in the exact order in which they
appear in the tree.  Parenthesis nodes may be used to group expressions so that they are evaluated in a specific order.

MuIL does not support operator overloading.  The set of operators is fixed and cannot be overridden, although a
higher-level MetaMu compiler may decide to emit calls to intrinsic functions rather than depending on MuIL operators.

### Smaller Items

MuIL doesn't currently support "attributes" (a.k.a., decorators).  This isn't for any principled reason other than the
lack of a need for them and, as such, attributes may be something we consider adding at a later date.

## Open Questions

AST shapes

Exporting classes: how much do you get?  E.g., Go is a good litmus test.

Exceptions: fail-fast

Abstract

Virtuals

Inheritance

RTTI/Casting/Conversion

Lambda types
Numeric types (long, int, etc)

Main entrypoint (vs. open-ended code)

Boxing/unboxing?

Async/await

Static variables

Module variable initialization

Varargs

Overloading generally

