# Mu Language

[Mu Metadata](metadata.md) is written in a "YAML-like" language, Mull (Mu's little language).  (There is a "JSON-like"
alternative for those who prefer a JSON style of syntax.)  Mull isn't vanilla YAML, however, for two reasons:

1. It may be strongly typed to support better compile-time validation.
2. It may contain embedded quotations that generate data in a rich, semantically-aware way.

These two things are tightly integrated with one another so that the result works a bit like an ordinary programming
language's type system.  For example, quotations are also strongly typed, and can leverage typed metadata.

Both aspects of Mull have been heavily inspired by other systems.  For typing, by TypeScript and JSON Schema.  For
quotations, by Hashicorp's HCL, Go templates, and Jsonnet.  A key difference is that we remain true to our YAML and JSON
heritage wherever possible, so that existing tools and skillsets may be leveraged.  In general, if one of these formats
already has an answer for a question we face -- like the syntax for number literals -- we choose it.

## Strong Typing

All variables and values have types and the Mu compiler typechecks that they are used correctly.  The set of types
expressible remains very close at the core to YAML and JSON, but adds more structure for custom types.

### Basic Types

The special type `any` may refer to any type.

The basic primitive types are `bool`, `number`, and `string`.

Any type `T` can be modified by appending `[]` to turn it into an array `T[]`: e.g., `number[]`, `string[]`, `any[]`.

Similarly, two types can be paired up to make a map type using `map[K, V]`, where `K` is the type of keys used to index
into the map and `V` is the type of value inside: e.g., `map[string, number]`, `map[number, any]`, and so on.  Note that
only the primtive types `bool`, `number`, and `string` can be used as key types inside of a map.

### Tuples and Custom Types

An anonymous tuple type, which is essentially a map with known properties, can be created using `{}`: e.g., the tuple
containing two properties, a `string` and a `number`, is `{ string, number }`.  These are accessed by their ordinal
position.  If you prefer to name the properties, you can do so `{ name: string, age: number }`.

For situations when an anonymous tuple becomes too verbose or repetetive, or advanced features are required, it is
possible to define custom complex types.

Each custom type has the following attributes:

* A name.
* An optional "base" type.
* An optional default value.
* An optional description.
* Either of these:
    - An optional set of properties.
    - An optional set of value constraints.

If properties exist, the type must be a custom object type.  If base exists, these properties extend it.  Each
property is a superset of the custom type, in that it can carry any or all of those attributes, with two caveats:
instead of "base" it is simply called "type" and it may also carry an indicator of whether a property is required.

The value constraints only apply to custom types whose "base" is either `string` or `number`:

* For strings:
    - A maximum length in characters.
    - A minimum length in characters.
    - A regex pattern for legal values.
* For numbers:
    - A maximum value.
    - A minimum value.
* For strings and numbers:
    - An enum array of legal values.

TODO: we need to specify precisely where custom types may appear, how the system knows to bind them, etc.  In the
    Mu case, there are actually two places: the stack's property definitions themselves (`properties`), and the custom
    type section (`types`).  Maybe we can use some "meta-typing" to indicate this, e.g. in the Mufile schema itself.

## Embedded Quotations

A quotation allows computation to be mixed into what is otherwise just markup data.

It's important to note that quotations are *not* templates.  Although quotations are very powerful and provide
functionality often offered by templating systems, the quotations are semantically understood at compile-time.  They
are strongly typed and are generally declarative in nature.  This is in contrast to templating systems which normally
perform "dumb" textual copy-and-paste, possibly leading to output that isn't even legal syntax.

All quotations are wrapped in `${}` syntax -- such as `${foo}` -- and can reference variables, call functions, perform
conditional operations, and more.  To escape a quotation, use a double dollar sign; for example, `$${foo}` will be
rendered as the string `${foo}`, instead of the variable named `foo`'s value.

### Variables

All quotations are evaluated within a scope containing any number of named variables.

To inject a variable's value into the markup stream, simply reference it by name; for example, `${foo}` is replaced by
the value of a variable called `foo` in the current scope.  If `foo`'s value is `"bar"`, then the result is `"bar"`.

#### Variable Types

Note that variables have types.  So "replaced by the value" does not necessarily mean that the result is a `string`.
This allows variables to carry complex structure that is emitted into the markup stream as-is.  Note that because
variable substitution works hand-in-hand with the typechecking mentioned earlier, any conflicting types that result will
lead to errors as you would expect.  This is one of the advantages to the quotation approach versus templates.

A function exists that will stringify any value, however: `${string(foo)}`.  There are also conversion functions for
other types, described below in the functions section.  They are unique because they typically require parsing.

#### Declaring New Variables

New variables may be introduced using the `let` syntax; a variable, once bound, is immutable and cannot be changed:

    ${let foo = "bar"}

A variable is unlike a property in that it is an implementation detail and not exported for public use.

A variable's type is inferred by its initialization value, although it can be specified precisely using type assertions.
In general, Mull has a structural type system and permits conversions between like-structured types.  However, if you
would like to assert that a variable is of a given type, the `type{expr}` syntax will do this, where `type` is the
desired type and `expr` is an arbitrary expression.  A compile-time error occurs if the conversion can't take place.

This can be useful when assigning maps to variables.  If the map is meant to be a custom type `myType`, you can say:

    ${let foo = myType{ a: "s", b: 42 }}

#### Accessing Properties

Properties of a resulting value may be accessed using dots.  For example, if `foo` is of a complex type with multiple
properties, `a`, `b`, and `c`, we can access them simply by saying `${foo.a}`, `${foo.b}`, and `${foo.c}`.  Those can of
course also be complex types with their own properties accessed through dotting, and so on, and so forth.

#### Accessing Array and Map Elements

Elements of array and map values are accessed using `[]`.  For example, `${foo[0]}` extracts the 0th element of an array
variable `foo`, while `${bar["baz"]}` extracts an element keyed by the string `"baz"` from the map variable `bar`.

#### Scopes, This, and Context

The scope can be customized and populated in any number of ways.  There are always two specific special variables:

* `type` refers to the current type specification.
* `this` refers to the "current object" (whatever that means in the domain-specific context; sometimes just `type`).

TODO: we still don't have an elegant formalism or even a good mental "model" for the distinction between "this script"
    (e.g., `type`), and "this object" (e.g., `this`).  In Mu, this makes sense, because the primary use case for
    templating is accessing the properties set by a caller instantiating the current stack represented by the file.
    But for many other cases, you actually just want to access enclosing properties inside of the file.  I should note,
    if we went back to `parameters` and `arguments`, rather than `properties`, we'd have a distinction between the two
    rooted by a single object (with the unfortunate indirection of needing to say `arguments.foo` instead of just `foo`.

Much like your favorite programming language, properties on `this` may be accessed either by explicitly naming `this`,
as in `${this.foo}`, or simply by naming the property without a prefix, as in `${foo}`.

Mu binds `this` to represent the current stack being constructed by a given Mufile.  As a result, each property defined
inside of a stack is thus available in the form of an automatically bound variable.

Additionally, Mu makes a `ctx` variable available that is bound to information about the current compilation unit.

TODO: describe the context.

#### Operators

TODO: comparisons.

TODO: math.

TODO: concatenation.

#### Constructors

Many values are injected simply by referencing a variable, its properties, calling a function, etc.  Sometimes, however,
new values are created anew out of existing components.

New array values can be created using the `[]` operator.  For example, `${[ a, b, c ]}` constructs a three-element array
out of the variables `a`, `b`, and `c`.

New map values, on the other hand, use `{}`.  For example, `${{ "a": a, "b": b, "c": c }}` constructs a three-element
map keyed by the strings `"a"`, `"b"`, and `"c"`, and with those same variables as the keys' respective values.

Note that the type inference engine will try its best to get the types right.  In case it gets something wrong, you may
simply use a type assertion.  For example, maybe instead of a map, we meant to create a custom type named `myType`; to
say that, we simply wrap the above example in a `myType{}` type assertion: `${myType{ "a": a, "b": b, "c": c }}`.

#### A Word on Nulls

Any property whose value is set to `undefined` is omitted from the result.  This can be useful when propagating property
values between different objects.  For example, let's say in our Mufile the optional `item` property was missing, and
yet we were about to propagate that property to another object.  In one model, we would need to say:

    ${if item != null}
        item: ${item}
    ${done}

Instead, however, we can simply set the property and let the system omit the result if it is `undefined`:

    item: ${item}

### Functions

A function is easily recognizable by the presence of parenthesis: `${func(...)}`, where `...` is the optional set of
comma-delimited arguments.  For example, `${concat(a1, a2, a3)}` concatenates three arrays, `a1`, `a2`, and `a3`,
evaluating to this overall resulting array.  Below is a complete list of the built-in functions available:

TODO: list them.

TODO: include for semantic inclusion (call it import?).

TODO: for built-ins, we probably want some combination of HCL's
(https://www.terraform.io/docs/configuration/interpolation.html) and what we were getting from Sprig
(https://github.com/Masterminds/sprig)

### Conditionals

Mull also supports conditional code in two forms: `if` and `for`.  These are slightly different than their general-
purpose programming language equivalents -- in that they are "declarative" in nature -- but should feel familiar.
Being more declarative helps to encourage best practices when writing deterministic and predictable Mufiles.

Both support an expression and block statement form.  The expression forms look like function calls and are handy for
short declarative data situations, like conditionally setting a property.  The block forms are better for complex cases.

#### If

An `if` expression evaluates to a value.  For example, in `${if(foo == "bar", "a", "b")}`, the expression evaluates
to `"a"` when `foo == "bar"` is true, and `"b"` otherwise.  Note that the else part may also be omitted, in which case
the expression yields `undefined` should the predicate be false, e.g. `${if(foo == "bar", "a")}`.

A guarded `if` block, or series of them, allows more sophisticated "control flow" within a Mull file.  For example:

    ${if foo == "bar"}
        ...
    ${else if bar == "baz"}
        ...
    ${else}
        ...
    ${done}

This now probably looks more like your favorite programming language except that the bodies produce values.  These are
evaluated in order and the first one to succeed leads to the inclusion of its body in the overall markup stream.  Note
that the body here is arbitrary Mull code: markup, additional quotations, or some combination thereof.

The `${done}` terminates the overall cascade of guards.

TODO: switch statement?

#### For

Any array or map, including custom data types, may be enumerated using the `for` expression or statement.

In the array case, each element is available as the special `item` variable, whereas in the map case, each key is
available as the special `key` variable and each value as `value`.  Notice that the names are built-in to reduce the
amount of boilerplate in Mull files; remember, they are intentionally kept simple and declarative.

In expression form, `each` evaluates to an array or map of values.

For example, `${for(a, item)}` is the identity statement for an array variable `a`, while `${for(m, { key: value })}`
is the identity statement for a map variable `m`.  We can do more interesting things, like square an array of numbers,
`${for(a, item*item)}`, or prepend a constant to the keys in a map, `${for(m, { "prefix-" + key: value }}`.  By
default, the result of an `each` is an array containing the values of each iteration, in order, unless the body
evaluates to a map, in which case its keys are merged to produce a single aggregate map.  By default, keys are sorted
before enumerating a map, in order to provide a guaranteed and deterministic execution order.

In statement form, `for`'s body is repeated for each element in the array or map:

    ${for [a, b, c]}
        ...
    ${done}

The same special `item`, `key`, and `value` variables are bound in the body of a `for` statement block.

The contents of each body can be any combination of markup and/or quotations.

Note that the special built-in function `range` can be used to create a typical `for` loop:

    ${for range(0, 10)}
        ...
    ${done}

