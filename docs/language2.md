# Mull (Mu's Little Language)

This design note describes Mull, Mu's Little Language.

Mull is the universal metadata format for the Mu framework, runtime, and tools.  Mull has three primary goals:

1. To completely and concisely express Mu concepts.
2. Facilitate componentization, encapsulation, and reuse.
3. Be appealing as a human language, both for reading and writing.
4. Be sufficient as a machine language, enabling inspection and toolability.

Although higher-level languages that project onto Mull, it is Mu's *lingua franca*, and hence of significant importance
to the Mu system.  Most of Mu's tools operate at the Mull abstraction level.

## Architecture

Before moving on, it is important to note that Mull is not a traditional language.  Mull programs describe the desired
state of a cloud service or collection of cloud services.  The evaluation of a Mull program results in a DAG that
captures dependencies and state that correspond to physical entities in a target cloud environment.  The Mu toolset then
takes this DAG, compares it to the existing environment, and "makes it so."

A Mull program itself is just metadata, therefore, and any computations in the Mull language itself exist solely to
determine this DAG.  This is a subtle but critical point, so it bears repeating: Mull code does not actually execute
within the target cloud environment; instead, Mull code merely describes the topology of the code and resources.

Higher-level "language bindings" are used to bind elements of Mull services to executable code.  This executable code
can come in many forms.  For example, a Mull "container" service may bind to a real, physical Docker container image.
As another instance, a Mull "lambda" service may bind to a single function written in JavaScript.  And as yet another
example, a Mull "RPC" service may bind to an entire Go program, with many endpoints implemented as Go functions.

A Mull package is incomplete without being fully bound to the program fragments that can be deployed to achieve the
semantics of a service.  As a result, this document does describe the binding process.  The Mull language itself,
however, is polyglot and remains agnostic to the choice of which language carries out these behaviors at runtime.

This should become clearer as we take a look at Mull and some real-world examples.

## A Tour of Mull

Although Mull is a new language, it borrows heavily from existing languages and metadata formats.  Many of these aspects
will be readily recognizable to developers familiar with those formats.  Mull differs in many interesting ways, however,
most notably its unique combination of metadata-orientation, strong typing, declarative expressions, and modularity.

### Formatting

### Comments

Mull provides C-style `/* */` block comments in addition to C++-style `//` line comments.

Mull has integrated documentation generation, so that any comment preceding a declaration of interest will be associated
with that declaration.  For example, a module may be annotated as follows.

    /*
    This module projects all AWS EC2 abstractions as Mu services.

    For a complete listing of AWS resource types and associated documentation, please see the AWS document site:
    @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html
    */
    module aws/ec2

For shorter comments, a line comment may be used instead.

    // A new route in a route table within a VPC. The route's target can be either a gateway attached to the VPC or a
    // NAT instance in the VPC.
    service Route {
    }

Notice that although line comments are generally shorter, they may also be multiline.

For some declarations, it makes sense for a comment to appear after, rather than before, the item.

    schema Person {
        firstName: string    // firstName is the person's given name.
        lastName: string     // lastName is the person's family name.
        optional age: number // age is the number of years this person has lived; optional for privacy reasons.
    }

Attributes about the declaration may be indicated using the `@attr` prefix, where `attr` is the attribute name.  For
example, the above module has a website URL as part of its metadata, indicated by `@website:`.  Although the set of
attributes is extensible and open-ended, Mull's document generation recognizes the following special ones:

* `@keywords` emphasizes certain keywords for search purposes.
* `@author` is the author's Mu handle.
* `@owners` is a comma-delimited list of Mu handles responsible for maintaining this item.
* `@website` provides a more detailed "homepage" with even more detailed documentation for the item.

Obviously, some of these attributes are overkill for certain declaration types.  But they can come in handy for some.

### Modules

Mull code is organized using modules.  A module is the unit of packaging, distribution, and dependency.

Module names are "Internet-like", in that each one has a protocol part, a base URL part, and a module name part:

    ModuleName = [ Protocol ] [ BaseURL ] NamePart
    Protocol   = "http://" | "https://" | "ssh://" | ...
    BaseURL    = URL* . (URL | .)* "/"
    URL        = (* valid URL characters *)
    NamePart   = (Identifier "/")* Identifier

For example, `https://hub.mu.com/aws/ec2` is a valid module name.

Notice, however, that the protocol and base URL are optional in the above grammar.  That is because `https://` is the
default protocol, and `mu.hub.com/` is the default base URL, so that you can simply reference `aws/ec2` in code.

In general, we refer to `aws` as the *namespace* and `ec2` as the *simple name* in this example.

#### Declarations

Each Mull file must begin with its module name as the first non-comment element.

    module aws/ec2

All files within a directory must belong to the same module.  They can freely reference elements from one another
without needing to say anything special (unlike foreign references which require `import` statements).

A file's module name is expected to match the layout on disk.

In other words, imagining the above `module aws/ec2` declaration was inside of a file, `vpc.mu`, as follows.

    WORKSPACE
    |   aws/
    |   |   ec2/
    |   |   |   vpc.mu

This structure may be overridden.  Please refer to [the dependencies document](deps.md) for details on workspaces.

#### Imports

Following this declaration, any modules used by the given file must be listed using the `import` keyword:

    import aws/cloudformation
    import mu/infrastructure/helpers

Afterwards, the elements from those modules are available to the following source code.  These are accessed by using the
module's simple name: `cloudformation` in the former, and `helpers` in the latter, import from above.

Any conflicts result in compile-time errors.

    import aws/helpers
    import mu/infrastructure/helpers

To break the tie, an alternative name can be given.

    import aws/helpers as awshelpers
    import mu/infrastructure/helpers

In this example, elements may be accessed using the `awshelpers` and `helpers` prefixes, respectively.

For information on how imports are resolved, please see [the dependencies document](deps.md).

### Types

Mull features a mostly-nominal type system.  Given that the language is largely about dealing with data of different
shapes, converting between structurally similar types is seamless, however most abstractions are explicitly named.

There are two main kinds of types in the Mull type system.

#### Schema Types

Schema types are just data.  Each type can be desurgared into a ["JSON-like"](https://tools.ietf.org/html/rfc7159)
structure, and are therefore interoperable with many languages and Internet protocols.  Although schema types are
nominal, they also enjoy convenient structural transformations in the language.

At the core, schema types are built out of primitive types:

* The special type `any` may refer to any schema type.

* The basic primitive types are `bool`, `number`, and `string`.

* Any type `T` can be modified by appending `[]` to turn it into an array `T[]`: e.g., `number[]`, `string[]`, `any[]`.

* Similarly, two types can be paired up to make a map type using `map<K, V>`, where `K` is the type of keys used to
  index into the map and `V` is the type of value inside: e.g., `map<string, number>`, `map<number, any>`, and so on.
  Note that only the primtive types `bool`, `number`, and `string` can be used as key types inside of a map.

Additionally, `number`, `string`, and array types can carry additional constraints to further subset legal values:

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

Of course, it is possible to define custom named schema types beyond these primitives.

    schema Person {
        readonly string: firstName
        readonly string: lastName
        optional age: number
        homeAddress: Address
        optional bizAddress: Address
    }
    
    schema Address {
        line1: string
        optional line2: string
        city: string
        state: string<2>
        zip: string<"[0-9]{5}(-[0-9]{4})?">
    }

Schema types may subtype other schema types.  Because there is no implementation code associated with schema types, this
behaves like structural subtyping.  For example, imagine we want an `Employee` which is a special kind of `Person`:

    schema Employee: Person {
        title: string
    }

Schema types can be strongly typed enums, constraining the value space to what is listed.  To do this, list the base
type as `enum<T>`, where `T` is either `string` or `number`, depending on how the enum is meant to be backed.

    schema State: enum<string> {
        "AL"
        "AK"
        ...
        "WI"
        "WY"
    }

Finally, schema types may give names to other structured schema types.

    schema Employees = Employee[]

Given this definition, we can use `Employees` as a type alias anywhere for an array of `Employee` schema values.

#### Service Types

Service types go beyond "just data" and represent cloud services.  They do have properties, which are schema-like, but
also describe other things like composition of other services, computations, and RPC and event-oriented service
endpoints.  Unlike schema types, instances of services have strong, persistent identity in the target cloud environment.

Before getting to custom service definitions, there are a few special service types.  These are *not* valid inside of
schema types, because they are semantically meaningful to the system and not marshalable as trivial "JSON-like" data.

Any instance of a service type is referred to a "capability".  Each is an unforgeable reference to another running
service and can be used to interact with it, either through configuration, RPC, or otherwise.  As noted above, these are
treated specially, and can only appear in certain places.  Attempting to stick one into a schema field, for example,
will lead to a compile-time failure.  Similarly, coercing to the `any` type will not work.

The weakly typed `service` type is the base primitive type from which all specific service types derive.  For
capabilities, it is more common to see the specific service type -- so that specific members of it may be accessed --
but for building generalized infrastructure and system services, the weakly typed `service` can come in handy.

A service type declaration represents a blueprint for creating a cloud service.

    service AddressBook {

A service encompasses many flavors of what are informally called "services": infrastructure resources, databases,
containers, event-oriented systems, web service applications, microservices, SaaS dependencies, ..., and so on.  This
consistency facilitates a degree of composition and reuse, plus consistency in the way they are created, configured,
updated, and managed, that are not commonly found in other languages.

A service contains up to six discrete sections.

A `ctor` (constructor) defines the logic and services encapsulated by this service.

        ctor() {
            new mu.Table {}
        }

Although it isn't stated in the source code, a `ctor` is actually just a macro.  Macros are explained later on, however,
these are computations evaluated at "compile-time", but not deployed and run in the cloud runtime environment.

A `properties` block defines a schema attached to this service instance, describing its essential settings.

        properties {
            title string
            optional persistent bool
        }

A `macros` block defines additional macros inside of this service definition.  Each macro looks just like a function,
however as noted about `ctor`s above, they are different in that they are compile-time evaluated.

        ctor {
            createTable("table1")
            createTable("table2")
        }
        macros {
            createTable(name: string) {
                new mu.Table() { name: name }
            }
        }

These are convenient for splitting lengthy `ctor`s into better factored sub-macros.  For macros shared between many
service definitions, and possibly even exported from a module, please see the Macros section below.

An `rpcs` block defines all of the RPC functions available on this service.  These are function signatures without the
bodies; the bindings must be done separately, as we describe later on in the language bindings section.

        rpcs {
            Entries(): stream<AddressBookEntry>
            CreateEntry(entry: AddressBookEntry): number
            DeleteEntry(id: number)
            ReadEntry(id: number): AddressBookEntry
            UpdateEntry(id: number, entry: AddressBookEntry)
        }

All RPC functions must deal solely in terms of schema types on the wire, since these generally map to HTTP/2-based RPC
and Internet protocols.  Notice also here the mention of a new built-in type, `stream<T>`.  This is a flow-controlled
stream that can be used to efficiently transmit a variable number of schema elements of type `T`.  It is the only
"special type" beyond schema types that can appear in the wire protocol, and can appear in input positions also.

An `events` block defines all of the events that may be subscribed to.  This permits event-driven inter-dependencies
between services that are expressed to the system as subscriptions.

        events {
            Added(state: State) AddressBookEntry
        }

All events are subject to the same restrictions as RPC functions on inputs and outputs (that is, only schema and stream
types may appear).  The input is used to control the subscription and output is the element type.  Note that the
semantics of how frequently an event fires, whether it is single-shot or repeating, and so on, are part of the semantics
of the event itself and not specified by the system.  The system does, however, specify what it means to unsubscribe.

Finally, a `services` block defines all public sub-services exported by this particular service.  Each service is
available to consumers of the outer service, and  must be assigned to by the constructor and/or invoked macros.

        ctor() {
            reporter = new AddressBookReporter { book = this }
        }
        services {
            reporter: AddressBookReporter
        }

### Variables and Constants

var
const

### Names

Accessibility

Conventions

### Macros

### Expressions

## Language Bindings

## Detailed Specification


