# Pulumi Type System

In its role as a broker of information between various actors--e.g. language SDKs,
resource providers, multi-language components, and statefiles--and in its role as a
programming model, it is important that Pulumi deals in values with well-defined
semantics. The _Pulumi type system_ specifies these semantics. It is the responsibility of
each language SDK and interchange format to ensure that these semantics are faithfully
implemented, ideally in as idiomatic a fashion as possible.

Note that this document describes the abstract type system rather than describing its
implementations. As long as implementations faithfully implement the semantics described by
this document, they may choose to provide simpler APIs/shorthands/etc. for the various
types and combinators. For example, the SDK for a language that natively supports
operations on [`Output<T>`](#outputt) values may not expose [`Output<T>`](#outputt) types
and combinators to the user at all.

## Primitive Types

The core primitives of the Pulumi type system form a superset of the JSON type system, and
supports the following types:

- `Null`, which represents the lack of a value
- `Bool`, which represents a boolean value
- `Number`, which represents an IEEE-754 double-precision number
- `String`, which represents a sequence of UTF-8 encoded unicode code points
- [`Asset`](#assets-and-archives), which represents a blob
- [`Archive`](#assets-and-archives), which represents a map from strings to `Asset`s or
  `Archive`s
- [`ResourceReference`](#resource-references), which represents a reference to a resource
- `Tuple<T0, T1, ... TN>`, which represents a tuple of heterogenously-typed values. Note
  that this type mainly exists for the purpose of writing the signature for [`all`].
- `Array<T>`, which represents a numbered sequence of values of a particular type
- `Map<T>`, which represents an unordered mapping from strings to values of a particular
  type
- `Union<T0, T1 ... TN>`, which represents a value of one of a fixed set of types
- `Enum<T, V0 ... VN>`, which represents one of a fixed set of values of a particular type

### Assets and Archives

An `Asset` or `Archive` may contain either literal data or a reference to a local file
located via its path or a local or remote file located via its URL.

In the case of `Asset`s, the literal data is a textual string, and the referenced file
is an opaque blob.

In the case of `Archive`s, the literal data is a map from strings to `Asset`s or `Archive`s,
and the referenced file is a TAR archive, gzipped TAR archive, or ZIP archive.

Each `Asset` or `Archive` also carries the SHA-256 hash of its contents. This hash can be
used to uniquely identify the asset (e.g. for locally caching `Asset` or `Archive`
contents).

### Resource References

A `ResourceReference` represents a reference to a resource. Although
all that is necessary to uniquely identify a resource within the context of a stack is its
URN, a `ResourceReference` also carries the resource's ID (if it is not a component) and
the version of the provider that manages the resource. If the contents of the referenced
resource must be inspected, the reference must be resolved by invoking the `getResource`
function of the engine's builtin provider. Note that this is only possible if there is a 
connection to the engine's resource monitor, e.g. within the scope of a call to `Construct`.
This implies that resource references may not be resolved within calls to other 
provider methods. Therefore, configuration values, custom resources and provider functions
should not rely on the ability to resolve resource references, and should instead treat
resource references  as either their ID (if present) or URN. If the ID is present and
empty, it should be treated as an [`Unknown`](#unknowns).

## Object Types

Object types are defined as mapping from property names to property types. Duplicate
property names are not allowed, and each property name maps to a single type.

## `Promise<T>`

A value of type `Promise<T>` represents the result of an asynchronous computation. Note
that although computations may fail, failures cannot be handled at runtime, and cause a
hard stop when attempting to access a `Promise<T>`'s concrete value.

## `Output<T>`

Perhaps the most important type in the Pulumi type system is `Output<T>`. A value of type
`Output<T>` represents a node in a Pulumi program graph, and behaves like a `Promise<T>`
that carries additional metadata that describes the resources on which the value depends,
whether the value is known or unknown, and whether or not the value is secret.

### Dependencies

If an `Output<T>` value is the result of a resource operation--e.g. if it is an output
property of some resource--it is said to _depend_ on that resource.

If a value of type `Output<T>` depends on a resource `R`, the result of any computation
that depends on its concrete value also depends on `R`.

### Unknowns 

An `Output<T>` may be unknown if it depends on the result of a resource operation that
will not be run because it is part of a `pulumi preview`. Previews typically produce
unknowns for properties with values that cannot be determined until the resource is
actually created or updated.

If a value of type `Output<T>` is unknown, any computation that depends on its concrete
value must not run, and must instead produce an unknown `Output<T>`.

### Secrets

An `Output<T>` may be marked as secret if its concrete value contains sensitive
information.

If a value of type `Output<T>` is secret, the result of any computation that depends on
its concrete value must also be secret.

## `Input<T>`

The partner of `Output<T>` is `Input<T>`, which is defined as `Union<T, Output<T>>`. In
simpler terms, a location of type `Input<T>` may accept either a plain old `T` value or an
`Output<T>` value.

## `inputShape(T)`

Although `Input<T>` gives us the ability to deal in both `T` and `Output<T>` values, it is
often the case that we want to construct _composite_ values out of multiple `Input<T>`s.
For example, consider `Input<Array<string>>`: a value of this type accepts either a
`Array<string>` or an `Output<Array<string>>`, but does not accept a value of type
`Array<Output<string>>`. In order to accept all three of these types, we need the type
`Input<Array<Input<string>>>>`. The `inputShape` type function defines an algorithm
for producing these sorts of types.

```rust
fn inputShape(T) {
	match T {
		_ => Input<T>,
		Tuple<...U> => Input<Tuple<map(...U, u => inputShape(u))>>,
		Array<U> => Input<Array<inputShape(U)>>,
		Map<U> => Input<Map<inputShape(U)>>,
		Union<...U> => Union<map(...U, u => inputShape(u))>,
		Promise<U> => Input<U>,
		Output<U> => Input<U>,
		Object<...P> => Input<Object<map(...P, (name, u) => (name, inputShape(u)))>>
	}
}
```

If we expand `Input<T>` into its underlying type, `Union<T, Output<T>>`, the types may be
clearer:

```rust
fn inputShape(T) {
	match T {
		_ => Union<T, Output<T>>,
		Tuple<...U> => Union<Tuple<map(...U, u => inputShape(u))>, Output<Tuple<map(...U, u => inputShape(u))>>>,
		Array<U> => Union<Array<inputShape(U)>, Output<Array<inputShape(U)>>>,
		Map<U> => Union<Map<inputShape(U)>, Output<Map<inputShape(U)>>>,
		Union<...U> => Union<map(...U, u => inputShape(u))>,
		Promise<U> => Union<U, Output<U>>,
		Output<U> => Union<U, Output<U>>,
		Object<...P> => Union<Object<map(...P, (name, u) => (name, inputShape(u)))>, Output<Object<map(...P, (name, u) => (name, inputShape(u)))>>>
	}
}
```

Resource input properties often use input-shaped types.

## `outputShape(T)`

Because the `Output<T>` metadata ([dependencies], [unknowns], and [secrets]) only applies
to a single value, it is necessary to represent composite metadata using nested `Output<T>`
types. Consider a variant of the `Array<string>` example from [`inputShape(T)`](#inputshapet):
in order to produce an array where each element may be an output, we need to use the type
`Output<Array<Output<string>>>`.

The `outputShape` type function defines an algorithm for producing these sorts of values.

```rust
fn outputShape(T) {
	match T {
		_ => Output<T>,
		Tuple<...U> => Output<Tuple<map(...U, u => outputShape(u))>>,
		Array<U> => Output<Array<outputShape(U)>>,
		Map<U> => Output<Map<outputShape(U)>>,
		Union<...U> => Union<map(...U, u => outputShape(u))>,
		Promise<U> => Output<U>,
		Output<U> => Output<U>,
		Object<...P> => Output<Object<map(...P, (name, u) => (name, outputShape(u)))>>
	}
}
```

Resource output properties often use output-shaped types.

Projecting output-shaped is a bit unwieldy, as values of these types often require a great
deal of unwrapping as nesting depth increases.

Instead of projecting these types in their fully-elaborated form, the various language SDKs
tend to opt to project them as a simple `Output<T>` while using an internal representation
for the concrete value that includes distinguished unknown values. This approach lets the
SDKs to allow e.g. lifted property and element access into partially-known composite
values. For example, the Node SDK will allow the user to access an element of an
`Output<[]string>` via a proxied index operator even if some elements of the array are
unknown, though it will not allow the user to access the entire value via `apply`.

## `plainShape(T)`

The final type function, `plainShape(T)`, replaces `Output<T>` types with their type
argument:

```rust
fn plainShape(T) {
	match T {
		_ => T,
		Tuple<...U> => Tuple<map(...U, u => plainShape(u))>,
		Array<U> => Array<plainShape(U)>,
		Map<U> => Map<plainShape(U)>,
		Union<...U> => Union<map(...U, u => plainShape(u)),
		Promise<U> => U,
		Output<U> => U,
		Object<...P> => Object<map(...P, (name, u) => (name, plainShape(u))>,
	}
}
```

This function is primarily useful for describing the signature of the [`all`] combinator.

## `Output<T>` Combinators

The rules described for working with `Output<T>` metadata--[dependencies], [unknowns], and
[secrets]--require special bookkeeping on the part of the consumer. There are three
primitive combinators that aid in this bookkeeping: [`apply`], [`all`], and [`unwrap`].

### `apply<T, U>(v: Output<T>, f: (T) => U): Output<U>`

The `apply` API allows its caller to access the concrete value of an `Output<T>` within
the context of a caller-supplied callback.

`apply` trivially obeys the `Output<T>` rules for [dependencies], [unknowns], and
[secrets]:

- the dependencies of the `Output<T>` argument are propagated to the result
- if the `Output<T>` argument is unknown, the callback is not run and the result is unknown
- if the `Output<T>` argument is secret, the result is secret

Note that the argument for `U` may itself be an `Output<V>`, in which case the return type
of `apply` will be `Output<Output<V>>`. The result can be unwrapped using the [`unwrap`]
combinator. A language SDK may opt to automatically unwrap such values
if its type system is flexible enough to express the unwrapping.

This API is morally equivalent to Javascript's `Promise.then` API, but with `Output<>`s in
the place of `Promise<>`s:

```typescript
class Output<T> {
	public apply<U>(func: (t: T) => U): Output<U> {
		...
	}
}
```

### `unwrap<T>(v: Output<Output<T>>): Output<T>`

The `unwrap` API transforms an `Output<Output<T>>` into an `Output<T>` according to the
`Output<T>` rules for [dependencies], [unknowns], and [secrets]:

- the result's dependencies are the union of the outer and inner `Output<>`s' dependencies
- if either the outer or inner `Output<>` is unknown, the result is unknowns
- if either the outer or inner `Output<>` is secret, the result is secret

If its type system is flexible enough, a language SDK may choose to omit a public-facing
`unwrap` API in favor of automatically unwrapping nested `Output<>`s.

### `all<T0 ... TN>(t0: Output<T0>, ... tn: Output<TN>): Output<plainShape(Tuple<T0 ... TN>)>`

The `all` API combines multiple heterogenous outputs into a single unwrapped tuple output.
The metadata from the arguments is combined as per the `Output<T>` rules for [dependencies],
[unknowns], and [secrets]:

- the result of `all` depends on the union of the dependencies of its `Output<>` arguments
- if any of the `Output<>` arguments is unknown, the result is unknown
- if any of the `Output<>` arguments is secret, the result is secret

For example, here is a simplified version of the signature for the Typescript
implementation of `all`:

```typescript
export function all<T1, T2>(values: [Output<T1>, Output<T2>]): Output<[Unwrap<T1>, Unwrap<T2>]>;
```

As in [`apply`], nested outputs must be unwrapped prior to use, though SDKs may choose to
automatically unwrap if their type system can accommodate the typing.

A variant of `all` for `Object`s is also possible:

`all<Object<...P>>(v: Object<...P>): Output<plainShape(Object<...P>)>`

This variant treats the object as a tuple of key/value pairs.

[dependencies]: #dependencies
[unknowns]: #unknowns
[secrets]: #secrets
[`apply`]: #applyt-uv-outputt-f-t--u-outputu
[`all`]: #allt0--tnt0-outputt0--tn-outputtn-outputplainshapetuplet0--tn
[`unwrap`]: #unwraptv-outputoutputt-outputt
