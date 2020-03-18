# HCL Syntax-Agnostic Information Model Extensions

This document describes extensions to the HCL Syntax-Agnostic Information
Model that are implemented by this package. The original specification can be
found [here](https://github.com/hashicorp/hcl/blob/v2.3.0/spec.md).

## Extended Types

### Primitive Types

The extended type system two additional primitive types, _int_.

An _int_ is an arbitrary-precision integer value. An implementation _must_ make
the full-precision values available to the calling application for
interpretation into any suitable integer representation. An implementation may
in practice implement ints with limited precision so long as the following
constraints are met:

- Integers are represented with at least 256 bits.
- An error is produced if an integer value given in source cannot be
  represented precisely.

Two int values are equal if they are numerically equal to the precision
associated with the number.

Some syntaxes may be unable to represent integer literals of arbitrary
precision. This must be defined in the syntax specification as part of its
description of mapping numeric literals to HCL values.

### Structural Types

The extended type system adds a new structural type kind, _union_.

A _union type_ is constructed of a set of types. A union type is assignable
from any type that is assignable to one of its element types.

A union type is traversed by traversing each of its element types. The result
of the traversal is the union of the results of the traversals that succeed.
When traversing a union with an element type of none, the traversal of none
successfully results in none; this allows a traversal of an optional value to
return an optional value of the appropriate type.

### Eventual Types

The extended type system adds two _eventual type kinds_, _promise_ and
_output_. These types represent values that are only available asynchronously,
and can be used by applications that produce such values to more accurately
track which values are available promptly and which are not.

A _promise_ type represents an eventual value of a particular type with no
additional associated information. A promise type is assignable from itself
or from its element type. Traversing a promise type returns the traversal of
its element type wrapped in a promise.

An _output_ type represents an eventual value of a particular type that carries
additional application-specific information. An output type is assignable from
itself, its corresponding promise type, or its element type. Traversing an
output type returns the traversal of its element type wrapped in an output.

### Null values

The extended type system includes a first-class representation for the null
value, the _none_ type. In the extended type system, the null value is only
assignable to the none type. Optional values of type T are represented by
the type `union(T, none)`.

## Type Conversions and Unification

### Primitive Type Conversions

Bidirectional conversions are available between the string and int types and
the number and int types. Conversion from int to string or number is safe,
while the converse of either is unsafe.

### Collection and Structural Type Conversions

Conversion from a type T to a union type is permitted if there is a conversion
from T to at least one of the union's element types. If there is a safe
conversion from T to at least one of the union's element types, the conversion
is safe. Otherwise, the conversion is unsafe.

### Eventual Type Conversions

Conversion from a type T to a promise with element type U is permitted if T is
a promise with element type V where V is convertible to U or if T is
convertible to U. The safety of this conversion depends on the safety of the
conversion from V or T to U.

Conversion from a type T to an output with element type U is permitted if T is
an output or promise with element type V where V is convertible to U or if T is
convertible to U. The safety of this conversion depends on the safety of the
conversion from V or T to U.

### Type Unification

The int type unifies with number by preferring number, and unifies with string
by preferring string.

Two union types unify by producing a new union type whose elements are the
concatenation of those of the two input types.

A union type unifies with another type by producing a new union whose element
types are the unification of the other type with each of the input union's
element types.

A promise type unifies with an output type by producing a new output type whose
element type is the unification of the output type's element type and the promise
type's element types.

Two promise types unify by producing a new promise type whose element type is the
unification of the element types of the two promise types.

Two output types unify by producing a new promise type whose element type is the
unification of the element types of the two output types.
