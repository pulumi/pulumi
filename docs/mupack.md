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

All data and computation AST nodes are fully bound, and ready for interpretation/execution.  Higher level MetaMu
language compilers are responsible for performing this binding, and encoding the results.  Those results are symbol
names and tokens that are registered and available for lookup within any given MuPackage.  These symbols provide a
quick, and binding logic-free, way of resolving any bound node to its target abstraction (module, type, or function).
From there, any data or computations associated with those abstractions may be retrieved thanks to the definitions.

MuPack is serialized in JSON/YAML form, although in the future we may explore more efficient file formats.  Examples in
this document will use a YAML syntax for brevity's sake.

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

## Symbols

Each symbol refepresents one of four kinds of abstractions: module, variable, function, and type:

* A *module* represents a collection of said abstractions.  No type, function, or const can exist outside of one.  Every
  MuPackage consists of at least one top-level module, with optional nested modules inside of it.

* A *variable* represents a named, typed storage location.

* A *function* represents a named computation with typed parameters and an optional typed return value.

* A *type* is either a *record* or *class*.  A record type is pure data and is restricted to a "JSON-like" subset of
  interoperable data types.  A class type, on the other hand, can contain "behavior" by way of functions, although Mu
  severely restricts this to ensure cross-language interoperability with languages that don't support OOP.

### Tokens

Each symbol is keyed by a token, which is a unique identifier used to reference the symbol from elsewhere inside and/or
outside of the defining module.  Each token is just a string name that encodes the entire context necessary to resolve
it to a concrete module and, within that module, a symbol definition:

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

## Definitions

Each package contains a definitions map containing all modules, types, variables, and functions:

    declarations:

This map is laid out in a hierarchical manner, so that any types belonging to a module are nested underneath it, etc.,
making the name resolution process straightforward.  It contains both internal and exported members.

### Modules

Because each package has an implicit top-level module, the `declarations:` element itself is actually a module
specification.  Every module specification may contain up to the four kinds of members underneath it listed earlier:

    declarations:
        functions:
            # functions, keyed by name
        modules:
            # submodules, keyed by name
        types:
            # types, keyed by name
        variables:
            # variables, keyed by name

Each of these elements may be made accessible outside of the package by attaching the `export: true` attribute:

    export: true

A module may contain definitions that aren't exported simply by leaving off `export: true` or explicitly marking a
definition as `export: false`.  These are for use within the package only.  No additional accessibility level is
available at the MuPack level of abstraction, although of course MetaMu languages may project things however they wish.

Modules cannot contain statements or expressions outside of functions and types.  This is unlike some programming
languages that permit "global" code that runs at module load time.  The only code that is permitted at this level is
variable initializers, which is run in a deterministic order at module load time.  Such initializers might depend on
other variable initializers, in which case, this dependency tree must provably form a DAG.  This ensures determinism.

### Types

This section describes MuIL's type system, plus the type definition metadata formats.

MuPack's type system was designed to be supported by a broad cross-section of modern programming languages.  That said,
it's entirely possible that MuPack exposes a construct that a certain language doesn't support.  Because MuIL is
designed for interpretation, determinism, and predictability -- and not runtime speed -- all type coercions are checked
and fail-fast if an illegal coercion is attempted.  It is obviously a better choice to verify such conversions where
possible in the MetaMu compilers themselves, however this approach naturally accomodates dynamic languages.

There is a single top-type that may refer to any record or class value: the `any` type.

All instances of records and classes in MuIL are called *objects*.  They are allocated on the heap, in map-like data
structures that have strong type identity, facilitating dynamic and structural conversions, in addition to classical
RTTI and OOP patterns.  In a sense, this is a lot like how ECMAScript works.  Furthermore, there is no notion of a
pointer in MuIL and so the exact storage location is kept hidden from MetaMu languages and their semantics.

Because all instances are objects, we must talk about `null`.  By default, types do not include the special value `null`
in their domain.  To include it in a type's domain, suffix athat type `T` with a question mark, as in `T?`.

#### Primitives

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

#### Records

Records are pure data, and instances are representable in [JSON](https://tools.ietf.org/html/rfc7159), ensuring
interoperability with languages and Internet protocols.  This is in contrast to objects which may represent types with
invariants that make them unappealing to serialize and deserialize.  There may be additional constraints placed on
records, to enforce contracts, but this does not alter their runtime representation.

The special type `record` may refer to any record type.  This represents the JSON-like subset of types.

It is of course possible to define new `record` types, comprised solely out of primitives and other `record` types.

Each custom `record` type has the following attributes:

* A `name` (its key).
* An optional `base` type.
* An optional informative `description`.
* Either of these:
    - An optional set of properties; or,
    - An optional set of value constraints.

For instance, here is an example of a custom `Person` record type:

    Person:
        description: A record describing a person.
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

##### Properties

In the case of properties, each property has the following attributes:

* A `name` (its key).
* An required `type`, indicating its primitive or `record` type.
* An optional `default` value.
* An optional `optional` indicator.
* An optional `readonly` indicator.
* An optional informative `description`.

By default, each property is mutable.  The `readonly` attribute on a property indicates that it isn't:

    Person:
        properties:
            firstName:
                type: string
                readonly: true
            # ...

As with most uses of `readonly` in other programming languages, it is shallow (that is, the property value cannot be
changed by if the target is a mutable record, properties on *that* record can be).

By default, each property is also required.  The `optional` attribute on a property indicates that it isn't:

    Person:
        properties:
            # ...
            age:
                type: number
                optional: true

Any property can be given a default value, in which case it is implicitly optional, for example as follows:

    Person:
        properties:
            # ...
            age:
                type: number
                default: 42

TODO(joe): `secret` keyword for Amazon NoEcho-like cases.

##### Subtyping

Record types may subtype other record type using the `base:` element.

For example, imagine we want an `Employee` which is a special kind of `Person`:

    Employee:
        description: A record describing an employee.
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

##### Conversions

Although schemas are nominal, they also enjoy convenient structural conversions in the language without undue ceremony.

IDENTITY.

#### Classes

#### Advanced Types

MuIL supports some additional "advanced" type system features.

##### Constraints

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

##### Union and Literal Types

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

##### Type Aliases

Any type `A` can be used as an alias for another type `B`, simply by listing `B` as `A`'s base type:

    Employees:
        base: Employee[]

This can be particularly useful for union/literal enum types, such as our state example above:

    State:
        base: [ "AL", "AK", ..., "WI", "WY" ]

Now, given this new `State` type, we can simplify our `state` property example from the previous section:

    properties:
        state:
            type: State

## Data and Computations

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
lack of a need for them and, as such, attributes may be something we consider adding at a lter date.

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

