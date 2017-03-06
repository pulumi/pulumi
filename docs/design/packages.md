# Coconut Package Metadata (CocoPack)

This document describes the overall concepts, capabilities, and serialization format for CocoPack.  It also describes the
intermediate language (IL) and type system for CocoPack, something we refer to as CocoIL.

## Overview

Each CocoPack file is called a CocoPack and contains four things:

* Package metadata.
* Symbol names and tokens.
* Module, type, function, and variable definitions.
* Data and computations encoded in an intermediate language (IL).

The metadata section describes attributes about the overall CocoPack, like its name and version.

All data and computations are fully bound to types, and ready for interpretation/execution.  Higher level CocoLang
language compilers are responsible for performing this binding, and encoding the results.  Those results are symbol
names and tokens that are registered and available for lookup within any given CocoPack.  These symbols provide a
quick, and binding logic-free, way of resolving any bound node to its target abstraction (module, type, or function).
From there, any data or computations associated with those abstractions may be retrieved thanks to the definitions.

Coconut's type system was designed to be supported by a broad cross-section of modern programming languages.  That said,
it's entirely possible that CocoPack exposes a construct that a certain language doesn't support.  Because CocoIL is
designed for interpretation, determinism, and predictability -- and not runtime speed -- all type coercions are checked
and throw an exception if an illegal coercion is attempted.  It is obviously a better choice to verify such conversions
where possible in the CocoLang compilers themselves, however this approach naturally accomodates dynamic languages.

CocoPack is serialized in JSON/YAML form, although in the future we may explore more efficient file formats.  Examples in
this document will use a YAML syntax for brevity's sake.

## Metadata

Each package may contain self-describing metadata, such as a name, and optional attributes that are common in package
managers, like a description, author, website, license, and so on.  For example:

    name: acmecorp/elk
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@acmecorp.com>
    website: https://acmecorp.github.io/elk
    keywords: [ elasticsearch, logstash, kibana ]

TODO(joe): describe the full informational attributes available.

Most of this information is taken from the source CocoLang program and carried forward during compilation.

Note that a version number is *not* part of the metadata prelude.  The version number is managed by a version management
system outside of the purview of the package contents.  For example, packages checked into Git are managed by SHA1
hashes and tags, while packages registered with a traditional package management system might be versioned manually.

## Names

As with most metadata formats, names are central to how things reference one another.  The two key concepts to
understand in how CocoPack and CocoIL encode such references are *symbols* and *tokens*.

### Symbols

Each symbol represents one of four kinds of abstractions: module, variable, function, or a class:

* A *module* represents a collection of said abstractions.  No type, function, or const can exist outside of one.  Every
  CocoPack consists of at least one top-level module, with optional nested modules inside of it.

* A *variable* represents a named, typed storage location.

* A *function* represents a named computation with typed parameters and an optional typed return value.

* A *class* is a named abstraction that contains properties (variables) and methods (functions).  Classes have been
  designed to support both nominal and structural typing, catering to a subset of most dynamic and static languages.

### Tokens

Each symbol is keyed by a token, which is a unique identifier used to reference the symbol from elsewhere inside and/or
outside of the defining module.  Each token is just a string name that encodes the entire context necessary to resolve
it to a concrete module and, within that module, its corresponding definition:

* A protocol (e.g., `https://`).
* A base URL (e.g., `cocohub.com/`, `github.com/`, etc).
* A fully qualified name (e.g., `acmecorp`, `aws/s3/Bucket`, etc).

For example, `https://hub.coconut.com/acmecorp#latest` refers to the latest version of the `acmecorp` module, while
`https://github.com/aws/s3/Bucket` refers to the `Bucket` class exported from the `aws/s3` module (itself exported from
the `aws` package).  The URLs are present so that package managers can download dependencies appropriately.

Each CocoPack contains a concrete list of module dependencies.  For example:

    dependencies:
        - https://cocohub.com/aws#^1.0.6
        - https://cocohub.com/github#~1.5.2

Now, throughout the rest of the CocoPack, any symbol tokens prefixed with `https://cocohub.com/aws` and
`https://cocohub.com/github` will be resolved to the artifacts exported by the repsective packages.  Note that
dependencies are required to be acyclic.

Notice that dependency names are like ordinary token names, but must also carry a version number:

* An `#` followed by version number (e.g., `#^1.0.6`, `#6f99088`, `#latest`, etc).

This version number ensures that the same dependency used for compilation is used during evaluation.  Coconut supports
multiple versioning formats (semantic versioning, Git SHA1 hash versioning, and "tip" (`latest`)).  Please refer to
[Coconut Dependencies](deps.md) for more information about token and dependency names and the resolution process.

CocoPacks may export other symbols in the form of modules, types, variables, and functions as members.

### Naming Conventions

TODO: talk about casing.

TODO: talk about identifier naming (including `.` prefix meaning "system usage").

## Types

This section describes the core aspects of CocoIL's type system.

There is a single top-type that may refer to any record or class value: the `any` type.

All instances of classes in CocoIL are called *objects*.  They are allocated on the heap, in map-like data structures that
have strong type identity, facilitating dynamic and structural conversions, in addition to classical RTTI and OOP
patterns.  In a sense, this is a lot like how ECMAScript works.  Furthermore, there is no notion of a pointer in CocoIL
and so the exact storage location is kept hidden from CocoLang languages and their semantics.

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

In addition to the above types, function types are available, in support of lambdas.  Such a type has an optional
comma-delimited list of parameter types with an optional return type: `func(PT0,...,PTN)RT`.  For example,
`func(string,number)bool` is a function that takes a `string` and `number`, and returns a `bool`.  Function types are
unique in that they can only appear in module properties, arguments, local variables, but not class properties.  This
ensures that all data structures are serializable as JSON, an important property to the interoperability of data.

### Accessibility

CocoIL supports two levels of accessibility on all elements:

* `public` means an element is accessible outside of its container.
* `private` means an element is accessible only within its container (the default).

In these sentences, "container" refers to either the enclosing module or class.

CocoIL supports one additional level of accessibility on class members:

* `protected` means an element is accessible only within its class and subclasses.

Accessibility determines whether elements are exported outside of the package.  For an element to be exported, it, and
all of its ancestor containers, must be marked `public`.  By default, elements are *not* exported, which is useful for
internal functionality that is invoked within the package but not meant for public consumption.

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

Modules may contain a single special function, called its "initializer", to run code at module load time.  It is denoted
by the special name `.init`.  Any variables with complex initialization must be written to from this initializer.

The sole top-level module for an executable CocoPack may also contain a special entrypoint function that is used to
perform graph evaluation.  It is denoted by the special name `.main`.  It is illegal for non-executable CocoPacks to
contain such a function, just as it is for a submodule to contain one (versus the CocoPack's top-level module).

Note that it is up to a CocoLang compiler to decide how to partition code between `.init` and `.main`.  For languages that
allow "open-ended" coding in a module definition, it is likely that such statements would appear in the `.main` method,
and that library-only CocoPacks containing such code would be rejected outright.

### Variables

A variable is a typed, named storage location.  As we will see, variables show up in several places: as module
properties, struct and class properties, function parameters, and function local variables.

Each variable definition, no matter which of the above places it appears, shares the following attributes:

* A `name` (its key).
* An required `type` token.
* An accessibility modifier (default: `private`).
* An optional `default` value.
* An optional `readonly` indicator.
* An optional informative `description`.

A class property can be marked `static` to indicate that it belongs to the class itself and not an instance object.

TODO(joe): `secret` keyword for Amazon NoEcho-like cases.

The following is an example variable that demonstrates several of these attributes:

    availabilityZoneCounts:
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

By default, variables are mutable.  The `readonly` attribute indicates that a variable cannot be written to:

    availabilityZoneCounts:
        type: map[string]number
        readonly: true
        # ...

Note that module properties are `readonly` by default.  As with most uses of `readonly` in other programming languages,
this is a shallow indication; that is, the property value cannot be changed but properties on the object to which the
property refers can be mutated freely, unless that object is immutable or a constant value.

All variables are initialized to `null` by default.  Problems may arise if the type is not nullable (more on this
later).  All loads guard against this possibility, however loading a `null` value from a non-null location will lead to
runtime failure (which can be difficult to diagnose).  It is better if CocoLang compilers ensure this cannot happen.

Finally, class properties can be marked as `primary`, to make it both publicly accessible and facilitate easy
initialization.  Any properties that will be initialized as pure data should be marked primary.  This has special
meaning with respect to construction that is described later in the section on classes.

### Functions

A function is a named executable computation, with typed parameters, and an optional typed return value.  As we will
see, functions show up in a few places: as module functions, class methods, and lambdas.

All function definitions have two following common attributes:

* An accessibility modifier (default: `private`).
* An optional list of parameters, each of which is a variable.
* An optional return type.

Module functions and class methods are required to also carry a `name` (the function's key).  Lambdas do not have one.

Class methods have a few additional optional attributes that may be set:

* An optional `static` attribute.
* An optional `sealed` attribute.
* An optional `abstract` attribute.

A `static` method does not have access to an enclosing `this` instance; instead, it is defined at the class scope.

All methods are virtual by default (and hence may be overridden), however the `sealed` attribute prevents overrides.
Conversely, the `abstract` annotation indicates that a method must be overridden by a concrete subclass.  Any class
containing even a single abstract method must itself be marked `abstract` (more on this shortly).

It is illegal to mark a `static` method as either `sealed` or `abstract`.

Module functions and lambdas are required to define a `body`, which is a CocoIL block.  Class methods often have one, but
it can be omitted, in which case the method must be abstract and concrete subclasses must provide a `body` block.

Any function can be marked with `intrinsic: true` to delegate execution to a runtime intrinsic.  This is used to
express custom resource types, but is seldom used in everyday Coconut development.  Please refer to the section on
Runtime Intrinsics later in this document for details on how this extensibility model works.

### Classes

New named `class` types can be created by composing primitives and other `class` types.  Classes are capable of
representing a multitude of constructs, from data-only, to pure interfaces, and everything in between.

Because Coconut's primitive type system is restricted to JSON-like types, all instances can be serialized as [JSON](
https://tools.ietf.org/html/rfc7159), ensuring interoperability with languages and Internet protocols.  There may be
additional constraints placed on classes to enforce contracts, but this does not alter their runtime representation.

Each custom `class` type has the following attributes:

* A `name` (its key).
* An accessibility modifier (default: `private`).
* An optional `extends` listing base type(s).
* An optional informative `description`.
* An optional set of `properties` and/or `methods`.
* An optional `sealed` attribute.
* An optional `abstract` attribute.
* An optional `record` attribute, indicating that a class is data-only.
* An optional `interface` attribute, indicating that a class has only pure functions.

For instance, here is an example of a pure data-only `Person` class:

    Person:
        description: A person.
        properties:
            firstName:
                type: string
                description: The person's given name.
                primary: true
            lastName:
                type: string
                description: The person's family name.
                primary: true
            age:
                type: number
                description: The person's current age.
                primary: true

All properties are variable definitions and methods are function definitions, per the earlier descriptions.

A class can have a constructor, which is a special method named `.ctor`, that is invoked during initialization.

A class may have a class initializer that runs code to initialize static variables.  It is denoted by the special name
`.init`.  Any of the class's static variables with complex initialization must be written to from this initializer.

As noted earlier, class properties that are pure data should be marked as `primary`, to make initialization easier.
Although classes can have constructors, they are not required to, and primary properties are set by the calling object
initializer *before* invoking that constructor.  In fact, a primary property's value *must* be provided at
initialization time (unless it is marked nullable).  This feature helps to reinforce Coconut's data-oriented viewpoint.
 
In pseudo-code, it is as though constructing a `Person` object is done as thus:

    new Person {
        firstName: "Alexander",
        lastName:  "Hamilton",
        age:       47,
    };

This is in contrast to a constructor, and/or even a hybrid between the two approaches, again in pseudo-code:

    new Person("Alexander", "Hamilton", 47);
    
    new Person(47) {
        firstName: "Alexander",
        lastName:  "Hamilton",
    };

A class marked `sealed` may not be subclassed.  Any attempt to do so leads to failure at load time.

A class may be marked `abstract` to disallow creating new instances.

A class with only primary properties and no constructor may be marked as a `record`.  A class with no properties, no
constructor, and only abstract methods may be marked as an `interface`.  This is a superset of `abstract`.

A class without a constructor and only primary properties is called a *conjurable* type.  Conjurability is helpful
because instances can be "conjured" out of thin air without regard for invariants.  Both records and interfaces are
conjurable.  Both enjoy certain benefits that will become apparent later, like structural duck typing.

### Subclassing

Classes may subclass other types using `extends` and `implements`:

* A class may subclass a single implementation class using `extends`.
* A class may subclass any number of conjurable classes using `implements`.

The distinction between these is that `extends` is used to inherit properties, methods, and possibly a constructor from
one other concrete class, while `implements` marks a particular type as explicitly convertable to certain conjurable
types.  This can be used to declare that a particular conjurable type is convertible even when duck-typing handles it.

For example, imagine we want an `Employee` which is a special kind of `Person`:

    Employee:
        description: An employee person.
        extends: Person
        properties:
            company:
                type: string
                description: The employee's current employer.
            title:
                type: string
                description: The employee's current title.

This facilitates implicit conversions from `Employee` to `Person`.  Because CocoIL preserves RTTI for objects, recovering
the fact that such an object is an `Employee` is possible using an explicit downcast.

At the moment, there is no support for redeclaration of properties, and therefore no support for covariance (i.e.,
strengthening property types).  All base-type properties are simply inherited "as-is".

### Conversions

The CocoIL type system uses a combination of subtyping and structural conversions.

At its core, CocoIL is a nominal static type system.  As such, it obeys the usual subtyping relations: upcasting from a
subclass to its superclass can be done without any explicit action; downcasting from a superclass to a given subclass
can be done explicitly with a CocoIL instruction, and it might fail should the cast be wrong.

Imagining that `Base` is the superclass and `Derived` is the subclass, then in pseudo-code:

    Base b = new Base();
    Derived d = new Derived();
    
    d = b;    // Error at compile-time, requires an explicit cast.
    d = (D)b; // Error at runtime, Base is not an instance of Derived.
    b = d;    // OK.

CocoIL also supports a dynamic `isinst` operator, to check whether a given object is of a certain class.

In addition, however, CocoIL supports structural "duck typed" conversions between conjurable types (records and
interfaces).  Recall that a conjurable type is one with only primary properties and no constructor.  Such types do not
have invariants that might be violated by free coercions between such types.  So long as the source and target are
"compatible" at compile-time, duck-type conversions between them work just fine.

What does it mean for two classes to be "compatible?"  Given a source `S` and destination `T`:

* Either:
    - `T` is an interface; or,
    - `S` and `T` are both conjurable.
* For all functions in `T`, there exists a like-named, compatible function in `S`.
* For all non-nullable properties in `T`, there exists a like-named, like-typed property in `S`.

A compatible function is one whose signature is structurally compatible through the usual subtyping rules (parameter
types are contravariant (equal or relaxed), return types are covariant (equal or strengthened).  Properties, on the
other hand, are invariant, since they are both input and output.  This is the same for lambda conversions.

The above conversions are based on static types.  CocoIL also supports dynamic coercion, dynamic castability checks, plus
dynamic property and method operations, as though the object were simply a map indexed by member names.  This is often
used in conjunction with the untyped `any` type.  These operations may, of course, fail at runtime, as is usual in
dynamic languages.  These operations respect accessibility.  These operations are described in the CocoIL section.

### Advanced Types

CocoIL supports some additional "advanced" type system features.

#### Constraints

To support rich validation, even in the presence of representations that faciliate data interoperability, CocoIL supports
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
CocoIL runtime validation will ensure that it is the case.

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

## Coconut Intermediate Language (CocoIL)

CocoIL is an AST-based intermediate language.  This is in contrast to some of its relatives which use lower-level stack or
register machines.  There are three reasons CocoIL chooses ASTs over these other forms:

* First, it simplifies the task of writing new CocoLang compilers, something important to the overall ecosystem.

* Second, performance is less important than simplicity; so, although the AST model most likely complicates certain
  backend optimizations, this matters less to an interpreted environment like Coconut than a system that compiles to
  machine code.  Especially when considering the cost of provisioning cloud infrastructure (often measured in minutes).

* Third, it makes writing tools that process CocoPacks easier, including CocoGL processing tools that reraise AST edits back
  to the original program text.

Below is an overview of the AST shapes supported by CocoIL.

### AST Nodes

TODO: this is just a dumb name listing; eventually we want to specify each and every node type.

Every CocoIL AST node derives from a common base type that includes information about its location in the source code:

    // Node is a discriminated type for all AST subtypes.
    interface Node {
        kind: NodeKind;
        loc?: Location;
    }

    // NodeType contains all legal Node implementations, effectively "sealing" Node.
    type NodeKind = ...;

    // Location is a location, possibly a region, in the source code.
    interface Location {
        file?: string;   // an optional filename.
        start: Position; // a starting position in the source text.
        end?:  Position; // an optional end position (if empty, just a point, not a range).
    }

    // Position consists of a 1-indexed `line` and a 0-indexed `column`.
    interface Position {
        line:   number; // >= 1
        column: number; // >= 0
    }

#### Definitions

    interface Definition extends Node {...}
        interface Module extends Definition {...}
        interface Variable extends Definition {...}
            interface Parameter extends Variable {...}
            interface ModuleProperty extends Variable {...}
            interface ClassProperty extends Variable, ClassMember {...}
        interface Function extends Definition {...}
            interface ModuleMethod extends Function {...}
            interface ClassMethod extends Function, ClassMember {...}
        interface Class extends Definition {...}
        interface ClassMember extends Definition {...}

#### Statements

    interface Statement extends Node {...}
        interface Block extends Statement {...}
        interface TryCatchFinally extends Statement {...}
        interface BreakStatement extends Statement {...}
        interface ContinueStatement extends Statement {...}
        interface IfStatement extends Statement {...}
        interface LabeledStatement extends Statement {...}
        interface ReturnStatement extends Statement {...}
        interface ThrowStatement extends Statement {...}
        interface WhileStatement extends Statement {...}
        interface EmptyStatement extends Statement {...}
        interface ExpressionStatement extends Statement {...}

    interface LocalVariableDeclaration

#### Expressions

    interface Expression extends Node {...}
        interface LiteralExpression extends Expression {...}
            interface NullLiteralExpression extends LiteralExpression {...}
            interface BoolLiteralExpression extends LiteralExpression {...}
            interface NumberLiteralExpression extends LiteralExpression {...}
            interface StringLiteralExpression extends LiteralExpression {...}
            interface ObjectLiteralLiteralExpression extends LiteralExpression {...}
        interface LoadVariableExpression extends Expression {...}
        interface LoadFunctionExpression extends Expression {...}
        interface LoadDynamicExpression extends Expression {...}
        interface InvokeFunctionExpression extends Expression {...}
        interface LambdaExpression extends Expression {...}
        interface UnaryOperatorExpression extends Expression {...}
        interface BinaryOperatorExpression extends Expression {...}
        interface CastExpression extends Expression {...}
        interface IsInstExpression extend Expression {...}
        interface TypeOfExpression extend Expression {...}
        interface ConditionalExpression extends Expression {...}
        interface SequenceExpression extends Expression {...}

## Interpretation

CocoPack and CocoIL are interpreted representations.  That means we do not compile them to assembly and, in certain cases,
we have made design decisions that favor correctness over performance.  The toolchain has a built in verifier that
enforces these design decisions at runtime.  This is unlike most runtimes that leverage an independent static
verification step to avoid runtime penalties.  That said, the `coco verify` command will run the verifier independently.

TODO: specify more information about the runtime context in which evaluation is performed.

TODO: specify how failures are conveyed (fail-fast, exception, etc).

## Possibly-Controversial Decisions

It's worth describing for a moment some possibly-controversial decisions about CocoPack and CocoIL.

These might come as a surprise to higher level programmers, however, it is worth remembering that CocoIL is attempting to
strike a balance between high- and low-level multi-language representations.  In doing so, some opinions had to be
discard, while others were strengthened.  And some of them aren't set in stone and may be something we revisit later.

### Values

CocoIL does not support unboxed values.  This is entirely a performance and low-level interop concern.

### Pointers

CocoIL supports pointers for implementing runtime functionality.  It does not embellish them very much, however, other
than letting the runtime (written in Go) grab ahold of them and do whatever it pleases.  For example, CocoIL does not
contain operators for indirection, arithmetic, or dereferencing.  CocoIL itself is not for systems programming.

TODO: this could evolve further as we look to adopting CocoGo, which, clearly, will have the concept of pointers.

### Generics

CocoIL does not support generics.  CocoLang languages are free to, however they must be erased at compile-time.

This admittedly sacrifices some amount of generality.  But it does so at the benefit of simplicity.  Some CocoLang
languages simply do not support generics, and so admitting them to the core would be problematic.  Furthermore,
languages like Go demonstrate that modern cloud programs of considerable complexity can be written without them.

Perhaps the most unfortunate and apparent aspect of CocoIL's lack of generics is the consequently missing composable
collection types.  To soften the blow of this, CocoIL has built-in array, map, and enumerable object types.

### Operators

CocoIL does come with a number of built-in operators.

CocoIL does not care about operator precedence, however.  All expressions are evaluated in the exact order in which they
appear in the tree.  Parenthesis nodes may be used to group expressions so that they are evaluated in a specific order.

CocoIL does not support operator overloading.  The set of operators is fixed and cannot be overridden, although a
higher-level CocoLang compiler may decide to emit calls to intrinsic functions rather than depending on CocoIL operators.

### Overloading

CocoIL does not support function overloading.

### Exceptions

Most CocoLangs have exception-based error models (with the sole exception of Go).  As a result, CocoIL supports exceptions.

At the moment, there are no throws annotations of any kind; if Go or Java become interesting CocoLang languages to support
down the road, we may wish to add optional throws annotations to drive proxy generation, including the possibility of
flowing return and exception types to Go and Java, respectively, or fail-fasting at the boundary.

It is a possibly unpopular decision, and possibly missing an opportunity to help debuggability, however we choose to
preserve the common semantic in dynamic languages of using exceptions in response to failed casts, dynamic lookups and
invokes, and so on.  Though, it's worth noting, we don't support Ruby-style `missing_method` or Python `__getattr__`.

### Threading/Async/Await

There is no multithreading in CocoIL.  And there is no I/O.  As a result, there are neither multithreading facilities nor
the commonly found `async` and `await` features in modern programming languages.

### Accessibility, Dynamicism, and Secrets

CocoIL dynamic operations respect accessibility.  This is unconventional but allows encapsulation of sensitive
information.  This is admittedly a risky guarantee to make -- since type systems are intricate things that can be
subverted in [subtle ways](https://www.microsoft.com/en-us/research/wp-content/uploads/2007/01/appsem-tcs.pdf) --
and I am honestly on the fence about it.  However, particularly given that cloud abstractions often manipulate sensitive
secrets, this seems like an important line in the sand to draw, until we are forced to do otherwise.  In practice, it
does mean that most dynamic languages will simply mark all properties as `public`, although it leaves open the door for
them to offer a `private` or `secret` annotation to protect those few fields that may deal with secret information.

### Smaller Items

CocoIL doesn't currently support "attributes" (a.k.a., decorators).  This isn't for any principled reason other than the
lack of a need for them and, as such, attributes may be something we consider adding at a later date.

CocoIL doesn't support varargs; instead, just use arrays.  The benefit of true varargs is twofold: usability -- something
that doesn't matter at the CocoIL level -- and runtime performance -- something CocoIL is less concerned about.

## Open Questions

Numeric types (long, int, etc)

