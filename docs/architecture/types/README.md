(types)=
(type-system)=
# Type system

In its role as a communications broker between various parties--[language
runtimes](languages), [providers](providers), [state backends](backends), and so
on--it is important that the Pulumi engine manages information consistently
according to some agreed upon set of semantics. To this end Pulumi defines a
*type system* that captures such a set of semantics. It is upon this type system
that language SDKs and providers are built. In the case of language SDKs, the
idea is that the type system should be implemented in a manner as idiomatic to
the language at hand, while remaining faithful to the specification.

(primitive-types)=
## Primitives

The core of Pulumi's type system is an extension of that offered by
[JSON](https://en.wikipedia.org/wiki/JSON), and consists of the following
building blocks:

* `Null`, which represents the absence of a value.
* `Bool`, which represents a boolean value that is either true or false.
* `Number`, which represents a 64-bit double precision [IEEE
  754](https://en.wikipedia.org/wiki/IEEE_754) floating-point number.
* `String`, which represents a sequence of
  [UTF-8](https://en.wikipedia.org/wiki/UTF-8)-encoded Unicode code points.
* [`Asset`](assets), which represents a blob, such as that which might represent
  the contents of a file or a URL.
* [`Archive`](archives), which represents a collection of assets, such as that
  which might represent a ZIP file.
* [`ResourceReference`](res-refs), which represents a reference to a
  [resource](resources) managed by Pulumi.
* `Tuple<T₀, T₁, ..., Tₙ>`, which represents a fixed-length sequence of values
  of types `T₀`, `T₁`, ..., `Tₙ`.
* `Array<T>`, which represents a variable-length sequence of values of type `T`.
* `Map<T>`, which represents an unordered mapping from `String`s to values of
  type `T`.
* `Object<K₀, V₀, K₁, V₁, ..., Kₙ, Vₙ>`, which represents an object with keys
  `K₀`, `K₁`, ..., `Kₙ` and where each `Kᵢ` is associated with a value of type
  `Vᵢ`. Keys may not be duplicated -- that is, no two keys `Kᵢ` and `Kⱼ` may be
  the same.
* `Union<T₀, T₁, ..., Tₙ>`, which represents a value that can any be of type
  `T₀`, `T₁`, ..., or `Tₙ`.
* `Enum<T, V₀, V₁, ..., Vₙ>`, which represents a value of type `T` that can be
  one of the values `V₀`, `V₁`, ..., or `Vₙ`.

(assets)=
(archives)=
(assets-archives)=
### Assets and archives

A value which is of type `Asset` or `Archive` contains some data, which must be
one of:

* a *literal* value representing the `Asset` or `Archive`'s contents -- a
  textual string for `Asset`s and a map from strings to `Asset`s or `Archive`s
  in the case of `Archive`s.
* a *path* referencing a local file.
* a *URL* referencing a local or remote file.

In the case of `Asset`s, files referred to by paths or URLs will be treated as
opaque blobs. In the case of `Archive`s, files referred to by paths or URLs must
use a supported format -- tarball archives (`.tar`), gzipped tarball archives
(`.tar.gz`), or ZIP archives (`.zip`).

Aside from its data, an `Asset` or `Archive` also carries the SHA-256 hash of
the data. This hash can be used to uniquely identify the asset (e.g. for locally
caching `Asset` or `Archive` contents).

With these definitions in place, an asset representing the string `"hello"`,
encoded in UTF-8, might be represented as any of the following:

* the literal value `"hello"` (sequence of bytes `[104, 101, 108, 108, 111]`),
  with its SHA-256 hash (`5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03`).
* the path `/path/to/file`, where `/path/to/file` contains the string `"hello"`
  (bytes `[104, 101, 108, 108, 111]`), with its SHA-256 hash.
* the URL `file:///path/to/file`, where `/path/to/file` contains the string `"hello"`
  (bytes `[104, 101, 108, 108, 111]`), with its SHA-256 hash.
* the URL `https://example.com/file.txt`, where retrieving the contents of
  `https://example.com/file.txt` yields the string `"hello"` (bytes `[104, 101,
  108, 108, 111]`), with its SHA-256 hash.

An archive consisting of two files, one named `file1` containing the string
`"hello"` and the other named `file2` containing the string `"world"` (both
encoded using UTF-8), might be represented as any of the following:

* the literal value `{ "file1": "hello", "file2": "world" }`, with an
  appropriate SHA-256 hash.
* the path `/path/to/archive.tar`, where `/path/to/archive.tar` is a tarball
  archive containing two files, one named `file1` containing the string
  `"hello"`, and the other named `file2` containing the string `"world"`, with
  an appropriate SHA-256 hash.
* the URL `file:///path/to/archive.tar.gz`, where `/path/to/archive.tar.gz` is a
  tarball archive containing two files, one named `file1` containing the string
  `"hello"`, and the other named `file2` containing the string `"world"`, with
  an appropriate SHA-256 hash.
* the URL `https://example.com/archive.zip`, where retrieving the contents of
  `https://example.com/archive.zip` yields a ZIP archive containing two files,
  one named `file1` containing the string `"hello"`, and the other named `file2`
  containing the string `"world"`, with an appropriate SHA-256 hash.

(promises)=
## Promises

A value of type `Promise<T>` represents the result of an asynchronous
computation. Promises which don't capture metadata (and thus qualify as
[outputs](outputs)) are not all that common -- plain
[](pulumirpc.ResourceProvider.Invoke)s (that is, provider function calls which
do not accept [`Input`](inputs)s or return [`Output`](outputs)) are perhaps the
primary example of their use.

(resources)=
## Resources

Resources are the fundamental unit of Pulumi-managed infrastructure, such as a
compute instance, a storage bucket, or a Kubernetes cluster. At a high level,
resources are divided into two classes: *custom resources* and *component
resources*.

(custom-resources)=
### Custom resources

*Custom resources* are cloud resources managed by a resource provider such as
AWS, Microsoft Azure, Google Cloud, or Kubernetes. Custom resources have a
well-defined lifecycle built around the differences between their actual state
and the desired state described by their input properties. They are
[implemented](provider-implementers-guide) using
[](pulumirpc.ResourceProvider.Create), [](pulumirpc.ResourceProvider.Read),
[](pulumirpc.ResourceProvider.Update), and [](pulumirpc.ResourceProvider.Delete)
operations defined by the [provider](providers).

(component-resources)=
### Component resources

[*Component
resources*](https://www.pulumi.com/docs/concepts/resources/components/) are
logical groupings of other resources that create a larger, higher-level
abstraction that encapsulates their implementation details. Component resources
have no associated lifecycle of their own -- their only lifecycle semantics are
those of their children. The inputs and outputs of a component resource are not
related in the same way as the inputs and outputs of a custom resource. *Local*
components are defined using program-level abstractions such as the
`ComponentResource` class offered by e.g. the NodeJS and Python SDKs. *Remote*
components are defined by [component providers](component-providers) and are
constructed by [registering](resource-registration) child custom or component
resources with the Pulumi engine in a [](pulumirpc.ResourceProvider.Construct)
operation.

(urns)=
### URNs

All resources have a required `String` `Name`. A resource's
[*`Type`*](https://www.pulumi.com/docs/iac/concepts/resources/names/#types)
(often captured in a [schema](schema) in the case of a custom provider resource)
specifies a set of [*input properties*](inputs) that define the resource's
desired state, and a set of [*output properties*](outputs) that represent the
last actual state that Pulumi recorded. A resource's [*uniform resource name*,
or
*`URN`*](https://www.pulumi.com/docs/iac/concepts/resources/names/#urns)[^urn-uniqueness]
serves as its identifier to the Pulumi engine and is built from the stack and
project a resource is located in, followed by its type, parent type and name:

```
urn:pulumi:<stack>::<project>::<qualified-type>::<name>
```

More formally, the
[EBNF](https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form) grammar
for a URN is as follows:

(urn-ebnf)=
```ebnf
urn = "urn:pulumi:" stack "::" project "::" qualified type name "::" name ;

stack   = string ;
project = string ;
name    = string ;
string  = (* any sequence of Unicode code points that does not contain "::" *) ;

qualified type name = [ parent type "$" ] type ;
parent type         = type ;

type       = package ":" [ module ":" ] type name ;
package    = identifier ;
module     = identifier ;
type name  = identifier ;
identifier = Unicode letter { Unicode letter | Unicode digit | "_" } ;
```

[^urn-uniqueness]:
    In an ideal world, URNs would be globally unique, but in practice there are
    a few exceptions. Most notable of these is that, within a stack
    (specifically, within a stack's [*state*](state-snapshots)), a URN may
    appear multiple times in order to identify, for instance, both a copy of a
    resource that is pending deletion and a copy that will be created to replace
    that deleted instance.

(resource-ids)=
### IDs

If a resource has been [created](pulumirpc.ResourceProvider.Create) or
[read](pulumirpc.ResourceProvider.Read) by a [provider](providers), it will also
have a `String` `ID` corresponding to the instance in the provider. IDs are
generally opaque to the engine and only have meaning in the context of the
provider that created them.

(outputs)=
### Outputs

`Output<T>` is perhaps the most important member of Pulumi's type system,
representing a node in a Pulumi program graph (such as the output of a [resource
registration](resource-registration)) that produces a value of type `T`. An
`Output<T>` value behaves like a `Promise<T>` in that it represents an
asynchronous computation that will eventually produce a value of type `T`, but
it also carries additional metadata that describes the resources on which the
value depends, whether the value is known or unknown, and whether or not the
value is secret.

#### Dependency tracking

If an `Output<T>` value is the result of a resource operation---that is, if it
is an output property of some resource---it is said to *depend* on that
resource. Output dependencies are *transitive*: if an `Output<T>` value `O₁`
depends on another `Output<T>` value `O₂`, then `O₁` also depends on any value
`O₃` that `O₂` depends on. Dependency tracking is used to ensure that the
correct order of operations is maintained when creating or updating resources.

(output-unknowns)=
#### Unknowns

An `Output<T>` may be *unknown* if it depends on the result of a resource
operation that will not be run, for example because it is part of a `pulumi
preview`.[^explicit-unknowns] Previews typically produce unknowns for properties
with values that cannot be determined until the resource is actually created or
updated. If a value of type `Output<T>` is unknown, any computation that depends
on its concrete value must not run, and must therefore also produce an unknown
`Output<T>`.

[^explicit-unknowns]:
    There are also cases where unknown values are used explicitly by the Pulumi
    engine, such as to calculate [dependent
    replacements](step-generation-dependent-replacements).

(output-secrets)=
#### Secrets

An `Output<T>` may be marked as *secret* if its concrete value contains
sensitive information. If a value of type `Output<T>` is secret, the result of
any computation that depends on its concrete value must also be secret.

(inputs)=
### Inputs

The partner of `Output<T>` is `Input<T>`, which represents a value that is
either of type `T` or `Output<T>` (and is thus defined as `Union<T,
Output<T>>`). In this manner, input properties may accept either plain values
defined in the program outright, or values which arise from the outputs of other
resources.

(property-paths)=
### Property paths

A *property path* is [JSONPath](https://en.wikipedia.org/wiki/JSONPath)-like
expression that describes a path to one or more properties within a set of
values, such as those that may comprise a resource's [input](inputs) or
[output](outputs) properties. Property paths are used in many contexts, such as
specifying a set of properties to
[`ignoreChanges`](https://www.pulumi.com/docs/iac/concepts/options/ignorechanges/)
for, or when identifying the set of properties responsible for a particular
[](pulumirpc.DiffResponse). Example property paths include:

```
root
root.nested
root["nested"]
root.double.nest
root["double"].nest
root["double"]["nest"]
root.array[0]
root.array[100]
root.array[0].nested
root.array[0][1].nested
root.nested.array[0].double[1]
root["key with \"escaped\" quotes"]
root["key with a ."]
["root key with \"escaped\" quotes"].nested
["root key with a ."][100]
root.array[*].field
root.array["*"].field
```

Note that property paths use the identifier `root` to refer to the top level of
a set of values, and not `$` as is common in JSONPath.

In this codebase, the `PropertyPath` type is used to represent property paths. A
`PropertyPath` lookup will result in a `PropertyValue`, which is a value of any
of the appropriate types in this document. An object `PropertyValue` can be
constructed from a map of `PropertyKey`s to `PropertyValue`s (that is,
`map[PropertyKey]PropertyValue`, aka `PropertyMap`) using the
`NewObjectProperty` function.

### Transformations

(inputshape)=
#### `inputShape`

The fact that `Input<T>` is a union of `T` and `Output<T>` means that care must
be taken when applying `Input` to composite type constructors. Consider the type
`Input<Array<String>>`, for instance. If we expand the definition of `Input`, we
find that such a type admits values of the types `Array<String>` and
`Output<Array<String>>`. It *does not*, however, admit values of the types
`Array<Output<String>>` or `Output<Array<Output<String>>`. This is unfortunate
since these types have natural applications when dealing with arrays of resource
outputs, for instance, and have natural mappings to and from the types that
actually are accepted. To overcome this restriction, we'd need to "push" the
`Input` type constructor inwards, resulting in the type
`Input<Array<Input<String>>>`. `inputShape` is a *type function* that captures
this "pushing in" transformation:

```
inputShape(T) = Input<
  case T of
    Tuple<U₀, U₁, ..., Uₙ> →
      Tuple<inputShape(U₀), inputShape(U₁), ..., inputShape(Uₙ)>

    Array<U> →
      Array<inputShape(U)>

    Map<U> →
      Map<inputShape(U)>

    Object<K₀, V₀, K₁, V₁, ..., Kₙ, Vₙ> →
      Object<K₀, inputShape(V₀), K₁, inputShape(V₁), ..., Kₙ, inputShape(Vₙ)>

    Union<U₀, U₁, ..., Uₙ> →
      Union<inputShape(U₀), inputShape(U₁), ..., inputShape(Uₙ)>

    Promise<U> →
      U

    Output<U> →
      U

    U →
      U
>
```

(outputshape)=
#### `outputShape`

`outputShape` defines the analogous transformation for `Output<T>` values,
allowing us to push `Output` constructors inwards to support tracking
dependency, unknown and secret metadata for composite types:

```
outputShape(T) = Output<
  case T of
    Tuple<U₀, U₁, ..., Uₙ> →
      Tuple<outputShape(U₀), outputShape(U₁), ..., outputShape(Uₙ)>

    Array<U> →
      Array<outputShape(U)>

    Map<U> →
      Map<outputShape(U)>

    Object<K₀, V₀, K₁, V₁, ..., Kₙ, Vₙ> →
      Object<K₀, outputShape(V₀), K₁, outputShape(V₁), ..., Kₙ, outputShape(Vₙ)>

    Union<U₀, U₁, ..., Uₙ> →
      Union<outputShape(U₀), outputShape(U₁), ..., outputShape(Uₙ)>

    Promise<U> →
      U

    Output<U> →
      U

    U →
      U
>
```

Resource output properties often use `outputShape`d types. While values of these
types track metadata at a granular level, accessing their values often requires
a great deal of unwrapping as nesting depth increases.

(plainshape)=
#### `plainShape`

`plainShape` is in some sense an inverse to `inputShape` and `outputShape`. It
collapses `Output`s to their underlying type argument:

```
plainShape(T) =
  case T of
    Tuple<U₀, U₁, ..., Uₙ> →
      Tuple<plainShape(U₀), plainShape(U₁), ..., plainShape(Uₙ)>

    Array<U> →
      Array<plainShape(U)>

    Map<U> →
      Map<plainShape(U)>

    Object<K₀, V₀, K₁, V₁, ..., Kₙ, Vₙ> →
      Object<K₀, plainShape(V₀), K₁, plainShape(V₁), ..., Kₙ, plainShape(Vₙ)>

    Union<U₀, U₁, ..., Uₙ> →
      Union<plainShape(U₀), plainShape(U₁), ..., plainShape(Uₙ)>

    Promise<U> →
      U

    Output<U> →
      U

    U →
      U
```

Among other things, `plainShape` is useful for typing the [`all`](output-all)
function.

(output-apply)=
(output-all)=
#### `apply` and `all`

Due to the fact that `Output<T>` values are asynchronous (they will only be
available when the resources that produce them have been created and updated)
and that they capture metadata such as dependency and secret information, they
must only be transformed in a manner that respects and preserves these
properties. To this end, the Pulumi type system specifies the following
combinators for operating on values of type `Output<T>`:

* `apply<T, U>(o: Output<T>, f: (t: T) => U): Output<U>`, which will apply a
  function `f` to the concrete value of the `o: Output<T>` when it becomes
  available and return a new `Output<U>` with the result. The result will depend
  on the union of the values depended upon by the original value and the
  callback `f`. If the original value is unknown, the callback will not be run
  and the result will be unknown. If the original value is secret, the result
  will be secret.

* `all<T₀, T₁, ..., Tₙ>(t₀: Output<T₀>, t₁: Output<T₁>, ..., tₙ: Output<Tₙ>):
  Output<plainShape(Tuple<T₀, T₁, ..., Tₙ>)>`, which combines a heterogeneous
  set of outputs into a single unwrapped tuple output. The tuple output will
  depend on the union of the values depended upon by the provided set of
  outputs. If any of the provided outputs are unknown, the tuple output will be
  unknown. If any of the provided outputs are secret, the tuple output will be
  secret.

#### `unwrap`

The `unwrap` function collapses a value of type `Output<Output<T>>` to one of
type `Output<T>`, effectively removing or "unwrapping" one layer of `Output`
metadata. In doing so metadata is combined as in other operations in order not
to lose information:

* The resulting `Output<T>`'s dependencies are the union of the dependencies of
  the original `Output<Output<T>>` and the dependencies of the inner
  `Output<T>`.
* The resulting `Output<T>` is unknown if either the original
  `Output<Output<T>>` or the inner `Output<T>` is unknown.
* The resulting `Output<T>` is secret if either the original `Output<Output<T>>`
  or the inner `Output<T>` is secret.

(res-refs)=
### Resource references

A `ResourceReference` represents a reference to a [resource](resource). Resource
references most commonly appear in the context of [component
providers](component-providers), where it is often useful for a component to be
able to accept references to other resources, or to return references to its
child components in its outputs. In order to support the rehydration of these
references into bonafide strongly-typed resources upon deserialization, a
resource reference contains both a [URN](urns) and, in the case that the
resource is not a [component](component-resources), an [ID](resource-ids) and
the version of the [provider](providers) that manages the resource. While in
principle a URN is sufficient for the purposes of uniquely identifying a
resource, including the ID and provider version means that the engine does not
have to query state to enact several common operations, such as passing an ID to
a downstream SDK or provider that does not understand full resource references.
The provider version in particular allows deserialization to ensure that the
correct version of the relevant SDK is used to rehydrate the referenced
resource; this is necessary as resource shapes may change between SDK versions.

Resource references are hydrated using the [built-in
provider](built-in-provider)'s `getResource` invoke.
