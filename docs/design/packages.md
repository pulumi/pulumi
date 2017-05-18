# Lumi Package Metadata (LumiPack)

This document describes the overall concepts, capabilities, and serialization format for LumiPack.  It also describes
the intermediate language (IL) and type system for LumiPack, something we refer to as LumiIL.

## Overview

Each package is serialized using the LumiPack format and contains four things:

* Package metadata.
* Symbol names and tokens.
* Module, type, function, and variable definitions.
* Data and computations encoded in an intermediate language (IL).

The metadata section describes attributes about the overall LumiPack, like its name and dependencies.

All data and computations are fully bound to types, and ready for interpretation/execution.  Higher level LumiLang
language compilers are responsible for performing this binding, and encoding the results.  Those results are symbol
names and tokens that are registered and available for lookup within any given LumiPack.  These symbols provide a
quick, and binding-logic-free, way of resolving any bound node to its target abstraction (module, type, or function).
From there, any data or computations associated with those abstractions may be retrieved thanks to the definitions.

Lumi's type system was designed to be supported by a broad cross-section of modern programming languages.  That said,
it's entirely possible that LumiPack exposes a construct that a certain language doesn't support.  Because LumiIL is
designed for interpretation, determinism, and predictability -- and not runtime speed -- all type coercions are checked
and throw an exception if an illegal coercion is attempted.  It is obviously a better choice to verify such conversions
where possible in the LumiLang compilers themselves, however this approach naturally accommodates dynamic languages.

LumiPack is serialized in JSON/YAML form, although in the future we may explore more efficient file formats.  (Markup is
rather verbose, yet pleasantly human-readable.)  Examples in this document will use a YAML syntax for brevity's sake.

## Metadata

Each package may contain self-describing metadata, such as a name, and optional attributes that are common in package
managers, like a description, author, website, license, and so on, for example:

    name: acmecorp/elk
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@acmecorp.com>
    website: https://acmecorp.github.io/elk
    keywords: [ elasticsearch, logstash, kibana ]
    dependencies:
        lumi: "*"
        aws: ^1.0.7

TODO(joe): describe the full informational attributes available.

Most of this information resides in the source program's `Lumi.yaml` (or `.json`) file.  It is possible for a
LumiLang compiler to generate some of this information, however, based on decorators, comments, and so on.

Note that a version number is *not* part of the metadata prelude.  The version number is managed by a version management
system outside of the purview of the package contents.  For example, packages checked into Git are managed by SHA1
hashes and tags, while packages registered with a traditional package management system might be versioned manually.
Please refer to [the dependencies design document](deps.md) for additional information on package version management.

## Symbols

As with most metadata formats, names are central to how things reference one another.  The two key concepts to
understand in how LumiPack and LumiIL encode such references are *symbols* and *tokens*.

### Abstractions

Each symbol represents one of four kinds of abstractions: module, variable, function, or a class:

* A *module* represents a collection of abstractions.  No type, function, or const can exist outside of one.

* A *variable* represents a named and optionally typed storage location.

* A *function* represents a named computation with optionally typed parameters and an optional typed return value.

* A *class* is a named abstraction that contains properties (variables) and methods (functions).  Classes have been
  designed to support both nominal and structural typing, catering to a subset of most dynamic and static languages.

Each package may denote a special "default" module, by virtue of an alias with the special name `.default`.

### Tokens

Each symbol is keyed by a token, which is a unique identifier used to reference the symbol from elsewhere inside and/or
outside of the defining module.  Each token is just a string name that encodes the entire context necessary to resolve
the associated symbol, beginning with a package name that either matches the current package or a declared dependency.

Each part of a token is delimited by a `:` character.  For example:

    aws                             # the AWS package token
    aws:ec2                         # the AWS EC2 module (inside the `aws` package)
    aws:ec2/instance:Instance       # the AWS EC2 `Instance` resource (inside the `aws:ec2/instance` package/module)
    aws:ec2/instance:Instance:image # the AWS EC2 Instance `image` property (or method)

Note that tokens are very different from package references.  A package reference, as described in [this document](
deps.md), contains a URL, version number, and so on.  A token inside of a package simply matches an existing dependency.

## Accessibility

LumiIL supports two levels of accessibility on all elements:

* `public` means an element is accessible outside of its container.
* `private` means an element is accessible only within its container (the default).

In these sentences, "container" refers to either the enclosing module or class.

LumiIL supports one additional level of accessibility on class members:

* `protected` means an element is accessible only within its class and subclasses.

Accessibility determines whether elements are exported outside of the package.  For an element to be exported, it, and
all of its ancestor containers, must be marked `public`.  By default, elements are *not* exported, which is useful for
internal functionality that is invoked within the package but not meant for public consumption.

## Types

This section describes the core aspects of LumiIL's type system.

TODO: a warning to the reader; it is likely that, over time, we will move more in the direction of an ECMAScript-like
    type system, with optional/gradual typing on top.  We are straddling a line between, on one hand, source
    compatibility with ECMAScript, Python, Ruby, etc., and, on the other hand, as much static compile-time verification
    as possible.  This is obviously heavily inspired by TypeScript.  At the moment, I fear we have gone too far down the
    static typing rabbit's hole.  This unfortunately means each LumiLang's "runtime layer" will be thicker than we want.

All instances of classes in LumiIL are called *objects*.  They are allocated on the heap, in map-like data structures
that have strong type identity, facilitating dynamic and structural conversions, in addition to classical RTTI and OOP
patterns.  In a sense, this is a lot like how ECMAScript works, and indeed the runtime emulates [ECMAScript prototypes](
https://developer.mozilla.org/en-US/docs/Web/JavaScript/Inheritance_and_the_prototype_chain) in many aspects.

There are two special "top-types" that may refer to any type: `object` and `any`.  The `object` type is a standard
statically typed top-type, that any type may coerce to implicitly.  Regaining specific typing from that type, however,
must be done through explicit downcasts; it alone has no operations.  The `any` type, in contrast, enjoys dynamically
typed operations by default.  It may therefore be cast freely between types, which may lead to runtime failures at the
point of attempting to access members that are missing (versus `object` which inspects runtime type identity).

Because all instances are objects, we must talk about `null`.  By default, types do not include the special value `null`
in their domain.  To include it in a type's domain, suffix that type `T` with a question mark, as in `T?`.

TODO[pulumi/lumi#64]: at the moment, we have not yet implemented non-null types.  Instead, all types include `null`
    in the legal domain of values.  It remains to be seen whether we'll actually go forward with non-nullability.

### Primitives

At the core, all types are built out of the primitives, which represent a "JSON-like" simplistic type system:

* The basic primitive types: `bool`, `number`, and `string`.

* Any type `T` can be modified by appending `[]` to make an array type `T[]`: e.g., `number[]` and `string[]`.

* Similarly, two types can be paired up to make a map type using `map[K]V`, where `K` is the type of keys used to
  index into the map and `V` is the type of value inside: e.g., `map[string]number` and `map[string]record`, and so on.
  Note that only the primitive types `bool`, `number`, and `string` can be used as keys for a map type.  A map type with
  a value type `V` that belongs to the `record` subset of types is also a `record`; otherwise, it is a `class`.

As with JSON, all numbers are [IEEE 754 64-bit floating point numbers](
https://en.wikipedia.org/wiki/IEEE_floating_point).

TODO(joe): we likely want ints/longs.  Perhaps not in the JSON-like subset, however.  Maybe even bignum.

In addition to the above types, function types are available, in support of lambdas.  Such a type has an optional
comma-delimited list of parameter types with an optional return type: `func(PT0,...,PTN)RT`.  For example,
`func(string,number)bool` is a function that takes a `string` and `number`, and returns a `bool`.  Function types are
unique in that they can only appear in module properties, arguments, local variables, but not class properties.  This
ensures that all data structures are serializable as JSON, an important property to the interoperability of data.

## Modules

Each package contains a flat list of modules, keyed by name, at the top-level:

    modules:
        moduleA: ...
        moduleB: ...

Modules may contain a single special function, called its "initializer", to run code at module load time.  It is denoted
by the special name `.init`.  Any variables with complex initialization must be written to from this initializer.

The default module for an executable package may also contain a special entrypoint function that is invoked during graph
evaluation.  This special function is denoted by the special name `.main`.  It is illegal for non-executable packages
to contain such a function, just as it is for a non-default module to contain one.

It is entirely up to a LumiLang compiler to decide how to partition code between `.init` and `.main`.  For languages
that have explicit `main` functions, for example, presumably that would be the key to trigger generation of an
executable package with an entrypoint; and all other packages would be libraries without one.  For languages like
ECMAScript, on the other hand, which permit "open-coding" anywhere in a module's definition, every module would
presumably be a potential executable *and* library, and each one would contain an entrypoint function with this code.

### Variables

A variable is a named, optionally typed storage location.  As we will see, variables show up in several places: as
module properties, struct and class properties, function parameters, and function local variables.

Each variable definition, no matter which of the above places it appears, shares the following attributes:

* A `name` (its key).
* A optional `type` token.
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
properties, for instance, this occurs in the module initializer; for class properties, in its constructor; and so on.

By default, variables are mutable.  The `readonly` attribute indicates that a variable cannot be written to:

    availabilityZoneCounts:
        type: map[string]number
        readonly: true
        # ...

This is enforced at runtime.  As with most uses of `readonly` in other programming languages, this is a shallow
indication; that is, the property value cannot be changed but properties on the object to which the property refers can
be mutated freely, unless that object is immutable or a constant value.

All variables are initialized to `null` by default.  Problems may arise if the type is not nullable (more on this
later).  All loads guard against this possibility, however loading a `null` value from a non-null location will lead to
runtime failure (which can be difficult to diagnose).  It is better if LumiLang compilers ensure this cannot happen.

Finally, class properties can be marked as `primary`, to make it both publicly accessible and facilitate easy
initialization.  Any properties that will be initialized as pure data should be marked primary.  This has special
meaning with respect to construction that is described later in the section on classes.

TODO: the notion of primary constructors hasn't really manifest itself yet; perhaps we should nix them.

### Functions

A function is a named executable computation, with typed parameters, and an optional typed return value.  As we will
see, functions show up in a few places: as module functions, class methods, and lambdas.

All function definitions have the following common attributes:

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

Module functions and lambdas are required to define a `body`, which is a LumiIL block.  Class methods often have one, but
it can be omitted, in which case the method must be abstract and concrete subclasses must provide a `body` block.

Any function can be marked with `intrinsic: true` to delegate execution to a runtime intrinsic.  This is used to
express custom resource types, but is seldom used in everyday Lumi development.  Please refer to the section on
Runtime Intrinsics later in this document for details on how this extensibility model works.

### Classes

New named `class` types can be created by composing primitives and other `class` types.  Classes are capable of
representing a multitude of constructs, from data-only, to pure interfaces, and everything in between.

Because Lumi's primitive type system is restricted to JSON-like types, all instances can be serialized as [JSON](
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
initialization time (unless it is marked nullable).  This feature helps to reinforce Lumi's data-oriented viewpoint.
 
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
one other concrete class, while `implements` marks a particular type as explicitly convertible to certain conjurable
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

This facilitates implicit conversions from `Employee` to `Person`.  Because LumiIL preserves RTTI for objects,
recovering the fact that such an object is an `Employee` is possible using an explicit downcast later on.

At the moment, there is no support for redeclaration of properties, and therefore no support for covariance (i.e.,
strengthening property types).  All base-type properties are simply inherited "as-is".

### Conversions

The LumiIL type system uses a combination of subtyping and structural conversions.

At its core, LumiIL is a nominal static type system.  As such, it obeys the usual subtyping relations: upcasting from a
subclass to its superclass can be done without any explicit action; downcasting from a superclass to a given subclass
can be done explicitly with a LumiIL instruction, and it might fail should the cast be wrong.

Imagining that `Base` is the superclass and `Derived` is the subclass, then in pseudo-code:

    Base b = new Base();
    Derived d = new Derived();
    
    d = b;    // Error at compile-time, requires an explicit cast.
    d = (D)b; // Error at runtime, Base is not an instance of Derived.
    b = d;    // OK.

LumiIL also supports a dynamic `isinst` operator, to check whether a given object is of a certain class.

In addition, however, LumiIL supports structural "duck typed" conversions between conjurable types (records and
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

The above conversions are based on static types.  LumiIL also supports dynamic coercion, dynamic castability checks, plus
dynamic property and method operations, as though the object were simply a map indexed by member names.  This is often
used in conjunction with the untyped `any` type.  These operations may, of course, fail at runtime, as is usual in
dynamic languages.  These operations respect accessibility.  These operations are described in the LumiIL section.

### Advanced Types

LumiIL supports some additional "advanced" type system features.

TODO[pulumi/lumi#64]: at the moment, we don't support any of these.  It's unclear if we will pursue adding these to
    the type system.  It's unfortunate that CloudFormation supports them, and we do not, at the moment, so we will need
    to do something.  Enums, for sure.  But perhaps constraint types rely on library verification instead.

#### Constraints

To support rich validation, even in the presence of representations that facilitate data interoperability, LumiIL supports
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
LumiIL runtime validation will ensure that it is the case.

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

## Lumi Intermediate Language (LumiIL)

LumiIL is an AST-based intermediate language.  This is in contrast to some of its relatives which use lower-level stack
or register machines.  There are three reasons LumiIL chooses ASTs over these other forms:

* First, it simplifies the task of writing new LumiLang compilers, something important to the overall ecosystem.

* Second, performance is less important than simplicity; so, although the AST model most likely complicates certain
  backend optimizations, this matters less to an interpreted environment like Lumi than a system that compiles to
  machine code.  Especially when considering the cost of provisioning cloud infrastructure (often measured in minutes).

* Third, it makes writing tools that process LumiPacks easier, including LumiGL processing tools that reraise AST edits
  back to the original program text.

### AST Nodes

Below is an overview of LumiIL's AST types.  For the latest, please [refer to the source code](
https://github.com/pulumi/lumi/tree/master/pkg/compiler/ast).

TODO: this is just a dumb name listing; eventually we want to specify each and every node type.

Every LumiIL AST node derives from a common base type that includes information about its location in the source code:

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

    // Position consists of a 1-indexed `line` and `column`.
    interface Position {
        line:   number; // >= 1
        column: number; // >= 1
    }

#### Definitions

    interface Definition extends Node {...}
        interface Module extends Definition {...}
        interface ModuleMember extends Definition {...}
        interface Class extends ModuleMember {...}
        interface ClassMember extends Definition {...}
        interface Variable extends Definition {...}
            interface LocalVariable extends Variable {...}
            interface ModuleProperty extends Variable, ModuleMember {...}
            interface ClassProperty extends Variable, ClassMember {...}
        interface Function extends Definition {...}
            interface ModuleMethod extends Function {...}
            interface ClassMethod extends Function, ClassMember {...}

#### Statements

    interface Statement extends Node {...}
        interface Block extends Statement {...}
        interface LocalVariableDeclaration extends Statement {...}
        interface TryCatchFinally extends Statement {...}
        interface BreakStatement extends Statement {...}
        interface ContinueStatement extends Statement {...}
        interface IfStatement extends Statement {...}
        interface SwitchStatement extends Statement {...}
        interface LabeledStatement extends Statement {...}
        interface ReturnStatement extends Statement {...}
        interface ThrowStatement extends Statement {...}
        interface WhileStatement extends Statement {...}
        interface ForStatement extends Statement {...}
        interface EmptyStatement extends Statement {...}
        interface MultiStatement extends Statement {...}
        interface ExpressionStatement extends Statement {...}

#### Expressions

    interface Expression extends Node {...}
        interface Literal extends Expression {...}
            interface NullLiteral extends Literal {...}
            interface BoolLiteral extends Literal {...}
            interface NumberLiteral extends Literal {...}
            interface StringLiteral extends Literal {...}
            interface ArrayLiteral extends Literal {...}
            interface ObjectLiteral extends Literal {...}
        interface LoadExpression extends Expression {...}
            interface LoadLocationExpression extends LoadExpression {...}
            interface LoadDynamicExpression extends Expression {...}
        interface CallExpression extends Expression {...}
            interface NewExpression extends Expression {...}
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

LumiPack and LumiIL are interpreted formats.

That means we do not compile them to assembly and, in certain cases, we have made design decisions that favor
correctness over performance.  The toolchain has a built in verifier that enforces these design decisions at runtime
(which can be run explicitly with `lumi pack verify`).  This is unlike most runtimes that leverage an independent static
verification step, often at the time of machine translation, to avoid runtime penalties.

TODO: specify more information about the runtime context in which evaluation is performed.

Any failures during interpretation are conveyed in the most friendly manner.  For example, unhandled exceptions carry
with them a full stack trace, complete with line number information.  In general, the Lumi interpreted environment
should nut fundamentally carry with it any inherent disadvantages, other than the obvious one: it is a new runtime.

## Possibly-Controversial Decisions

It's worth describing for a moment some possibly-controversial decisions about LumiPack and LumiIL.

These might come as a surprise to higher level programmers, however, it is worth remembering that LumiIL is attempting
to strike a balance between high- and low-level multi-language representations.  In doing so, some opinions had to be
discarded, while others were strengthened.  And some of them aren't set in stone and may be something we revisit later.

### Values

LumiIL does not support unboxed values.  This is entirely a performance and low-level interop concern.

### Pointers

LumiIL supports pointers for implementing runtime functionality.  It does not embellish them very much, however, other
than letting the runtime (written in Go) grab a hold of them and do whatever it pleases.  For example, LumiIL does not
contain operators for indirection, arithmetic, or dereferencing.  LumiIL itself is not for systems programming.

TODO: this could evolve further as we look to adopting LumiGo, which, clearly, will have the concept of pointers.

### Generics

LumiIL does not support generics.  LumiLang languages are free to, however they must be erased at compile-time.

This admittedly sacrifices some amount of generality.  But it does so at the benefit of simplicity.  Some LumiLang
languages simply do not support generics, and so admitting them to the core would be problematic.  Furthermore,
languages like Go demonstrate that modern cloud programs of considerable complexity can be written without them.

Perhaps the most unfortunate and apparent aspect of LumiIL's lack of generics is the consequently missing composable
collection types.  To soften the blow of this, LumiIL has built-in array, map, and enumerable object types.

I suspect we will eventually need/want to go here.

### Operators

LumiIL does come with a number of built-in operators.

LumiIL does not care about operator precedence, however.  All expressions are evaluated in the exact order in which they
appear in the tree.  In fact, this is true of all of LumiIL's AST nodes: evaluation happens in tree-walk order.

### Overloading

LumiIL does not support function overloading.

LumiIL does not support operator overloading.  The set of operators is fixed and cannot be overridden, although a
higher-level LumiLang compiler may choose to emit calls to intrinsic functions rather than using LumiIL operators.

### Exceptions

Most LumiLangs have exception-based error models (with the exception of Go).  As a result, LumiIL supports exceptions.

At the moment, there are no throws annotations of any kind; if Go or Java become interesting LumiLang languages to
support down the road, we may wish to add optional throws annotations to drive proxy generation, including the
possibility of flowing return and exception types to Go and Java, respectively, or fail-fasting at the boundary.

It is a possibly unpopular decision, and possibly missing an opportunity to help debuggability, however we choose to
preserve the common semantic in dynamic languages of using exceptions in response to failed casts, dynamic lookups and
invokes, and so on.  Though, it's worth noting, we don't support Ruby-style `missing_method` or Python `__getattr__`.

### Threading/Async/Await

There is no multithreading in LumiIL.  And there is no I/O.  As a result, there are neither multithreading facilities
nor the commonly found `async` and `await` features in modern programming languages.  C'est la vie.

### Attributes

LumiIL doesn't currently support "attributes" (a.k.a., decorators).  This isn't for any principled reason other than the
lack of a need for them and, as such, attributes may be something we consider adding at a later date.  Of course, a
LumiLang compiler may very well make decisions based on the presence or absence of attributes; but they must be erased.

### Varargs

LumiIL doesn't support varargs; instead just use arrays.  The benefit of true varargs is twofold: usability -- something
that doesn't matter at the LumiIL level -- and runtime performance -- something LumiIL is less concerned about.

## Open Questions

It's unclear whether we want to stick to the simple JSON subset of numeric types, namely IEEE754 64-bit floats.  This is
particularly unfortunate for 64-bit integers, since you only get 53 bits of precision.

