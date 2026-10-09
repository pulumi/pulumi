# The PCL type system: conversion and unification

This document is the specification of the type relations in this package.
A machine-checked model of these rules lives in `formal/`; its README maps
each property of §6 to its evidence. Sections 7 and 8 are the only places
that compare the specification with go-cty and with the Go implementation.

## 1. Scope

The specification covers:

- the type grammar and its normal forms (§2, §3);
- the conversion relation `conv(dst, src)` (§4);
- the unification function `unify(T₁, …, Tₙ)` (§5);
- the soundness properties and their status (§6);
- compatibility with go-cty (§7) and with the Go implementation (§8);
- how the binder uses the results (§9) and what the package may cache (§10);
- the design decisions and the rejected alternatives (§11);
- recursive object types (§12).

The specification does not cover the types of expressions, diagnostics, `Traverse`,
or `InputType`. Those use the relations below and do not change them.

## 2. The type grammar

```
T ::= bool | int | number | string | id | dynamic | none
    | const(v)                      v : bool | int | number | string value
    | enum(token, B, {v₁, …, vₙ})   B : bool | int | number | string
    | list(T) | set(T) | map(T)
    | tuple(T₁, …, Tₙ)              n ≥ 0
    | object({k₁: T₁, …, kₙ: Tₙ})   n ≥ 0, keys unique
    | union(T₁, …, Tₙ)              n ≥ 2, in normal form (§3)
    | output(T) | promise(T)
    | opaque(name)
```

### 2.1 Scalars

The five scalar types are `bool`, `int`, `number`, `string`, and `id`.
`id` is the type of a resource ID. It converts like `string` (§4.4) and is a
distinct type for equality.

An opaque type `opaque(name)` is a nominal type with no structure. The binder
uses it for schema token types, assets, archives, and resource references. Two
opaque types are equal when their names are equal. No rule of §4.2 has a shape
for an opaque type, so it converts only by C-Eq, C-Dyn, C-DynSrc, and the
union and eventual rules; every other pair is C-No.

### 2.2 Constants and enums

A constant `const(v)` is the type of one value. Its base type `base(v)` is a
function of the value: a bool value has base `bool`, a string value has base
`string`, an integral number has base `int`, and every other number has base
`number`. Two constants are equal when their values are equal.

The `null` literal has type `none`. There is no constant with a `null` value.

An enum `enum(token, B, values)` is a nominal type. Two enums are equal when
their tokens are equal. All values of an enum have the base type `B`.

### 2.3 Eventuals

`output(T)` and `promise(T)` are the eventual types. The constructors keep
these invariants:

- `output(T)` holds no `output` and no `promise` at any depth of `T`. The
  constructor applies `resolveOutputs` to `T`.
- `promise(T)` holds no `promise` at any depth of `T`. It may hold outputs.
  The constructor applies `resolvePromises` to `T`.

`resolveOutputs(T)` removes every `output` and `promise` wrapper at any depth
and rebuilds collections, objects, tuples, and unions. `resolvePromises(T)`
removes every `promise` wrapper. Constants and enums are leaves for both.

`containsOutputs(T)` is true when an `output` occurs at any depth.

### 2.4 Optional types

`optional(T)` is `union(T, none)`. There is no separate optional constructor.

### 2.5 Equality

Equality is structural. An object compares its sorted property list. A union
compares its normal-form member list. `output(T)` equals `output(U)` when `T`
equals `U`. Equality on recursive objects is coinductive: `Equals` carries the
set of `(t, other)` object pairs in flight, and a pair in the set compares
equal. The set is keyed on the pair, not on one side. A set keyed on `t`
alone is asymmetric: with `y = {a: list(y)}` and `x = {a: list(list(bool))}`,
`y.Equals(x)` would hold and `x.Equals(y)` would not.

## 3. Union normal form

`union(T₁, …, Tₙ)` denotes the result of the union constructor on the
member list `[T₁, …, Tₙ]`. The constructor computes the canonical member list
`norm([T₁, …, Tₙ])` and then builds the type:

1. Flatten: a member that is a union contributes its members.
2. Sort the members with a fixed total order `≺` on types. Any total order
   serves; the order ranks by constructor first and then by children.
3. Remove duplicates.
4. If no member remains, the result is `none`. If one member remains, the
   result is that member. Otherwise the result is the union of the members.

The normal form is syntactic. It does not absorb members: `union(int, string)`
and `union(dynamic, none)` are valid types. Unification (§5) merges members;
the constructor does not. This keeps `optional(dynamic)` distinct from
`dynamic`, which the binder needs for optional parameters.

## 4. Conversion

### 4.1 Kinds

`conv(dst, src) ∈ {No, Unsafe, Safe}` with `No < Unsafe < Safe`.

- `Safe`: every value of `src` is a value of `dst` after a total conversion.
- `Unsafe`: a conversion exists, but it can fail for some values or lose
  information.
- `No`: no conversion exists.

Two folds over kinds appear in the rules:

- `min(k₁, …, kₙ)` is the least kind; `min()` is `Safe`.
- `max(k₁, …, kₙ)` is the greatest kind; `max()` is `No`.
- `members(k₁, …, kₙ)` is `Safe` when every kᵢ is `Safe`, `No` when every kᵢ
  is `No`, and `Unsafe` otherwise.
- `cap(k)` is `min(Unsafe, k)`.
- `unsafeIf(b)` is `Unsafe` when `b` holds and `No` otherwise.

### 4.2 Rules

The rules apply in order. The first rule whose shapes match decides. `S` and
`D` stand for any type.

| Rule        | dst                | src                | Result                                                           |
|-------------|--------------------|--------------------|------------------------------------------------------------------|
| C-Eq        | `D`                | `D`                | `Safe`                                                           |
| C-Dyn       | `dynamic`          | `S`                | `Safe`                                                           |
| C-USrc      | `D`                | `union(S₁, …, Sₙ)` | `members(conv(D, S₁), …, conv(D, Sₙ))`                           |
| C-UDst      | `union(D₁, …, Dₙ)` | `S`                | `max(conv(D₁, S), …, conv(Dₙ, S))`                               |
| C-Out       | `output(T)`        | `S`                | `conv(T, resolveOutputs(S))`                                     |
| C-Prom      | `promise(T)`       | `output(U)`        | `No` |
| C-Prom      | `promise(T)`       | `S`                | `conv(T, resolvePromises(S))` |
| C-DynSrc    | `D`                | `dynamic`          | `Unsafe`                                                         |
| C-Const     | `const(v)`         | `const(w)`         | `Safe` if `v = w`, else `No`                                     |
| C-ConstSrc  | `const(v)`         | `S`                | `unsafeIf(conv(base(v), S) ≠ No)`                                |
| C-EnumConst | `enum(_, _, V)`    | `const(v)`         | `Safe` if `v ∈ V`, else `No`                                     |
| C-EnumSrc   | `enum(_, B, _)`    | `S`                | `unsafeIf(conv(B, S) ≠ No)`                                      |
| C-Widen     | `D`                | `const(v)`         | `conv(D, base(v))`                                               |
| C-Widen     | `D`                | `enum(_, B, _)`    | `conv(D, B)`                                                     |
| C-Scalar    | scalar             | scalar             | the table in §4.4                                                |
| C-List      | `list(T)`          | `list(U)`          | `conv(T, U)`                                                     |
| C-List      | `list(T)`          | `set(U)`           | `conv(T, U)`                                                     |
| C-List      | `list(T)`          | `tuple(U₁, …, Uₙ)` | `min(conv(T, U₁), …, conv(T, Uₙ))`                               |
| C-Set       | `set(T)`           | `set(U)`           | `conv(T, U)`                                                     |
| C-Set       | `set(T)`           | `list(U)`          | `cap(conv(T, U))`                                                |
| C-Set       | `set(T)`           | `tuple(U₁, …, Uₙ)` | `cap(min(conv(T, U₁), …, conv(T, Uₙ)))`                          |
| C-Map       | `map(T)`           | `map(U)`           | `conv(T, U)`                                                     |
| C-Map       | `map(T)`           | `object({k: Uₖ})`  | `min over k of conv(T, Uₖ)`                                      |
| C-Tuple     | `tuple(T₁, …, Tₙ)` | `tuple(U₁, …, Uₘ)` | `No` if `n ≠ m`, else `min over i of conv(Tᵢ, Uᵢ)`               |
| C-Tuple     | `tuple(T₁, …, Tₙ)` | `list(U)`          | `cap(min over i of conv(Tᵢ, U))`                                 |
| C-Tuple     | `tuple(T₁, …, Tₙ)` | `set(U)`           | `cap(min over i of conv(Tᵢ, U))`                                 |
| C-Object    | `object({k: Tₖ})`  | `object({j: Uⱼ})`  | `min over k of conv(Tₖ, Uₖ)`, where a missing `Uₖ` is `none`     |
| C-Object    | `object({k: Tₖ})`  | `map(U)`           | `cap(min over k of conv(Tₖ, U))`                                 |
| C-No        | `D`                | `S`                | `No`                                                             |

Notes on the rules:

- C-USrc precedes C-UDst. A union source is decided member by member. Each
  member picks its best destination member. This gives the precise answer:
  `union(int, string) <- union(int, bool)` is `Safe` because `bool` converts
  safely to `string`.
- C-UDst precedes C-DynSrc, so `union(bool, dynamic) <- dynamic` is `Safe`.
- C-Out precedes C-DynSrc, so `output(dynamic) <- dynamic` is `Safe` and
  `output(int) <- dynamic` is `Unsafe`.
- C-Out resolves outputs at any depth of the source. A value of type
  `map(output(bool))` lifts to `output(map(bool))`. This is the same lift that
  `liftOperationType` applies to expressions.
- C-Prom rejects only a top-level `output` source. An output nested in the
  source meets the element rules: `promise(list(output(string))) <-
  list(output(bool))` is `Safe` by C-List and C-Out, and
  `promise(list(string)) <- list(output(bool))` is `No`.
- C-Object ignores source keys that the destination does not have. A
  destination key that the source lacks is checked against `none`, so only an
  optional or dynamic property accepts an absent key.
- C-Tuple accepts only a source of the same length.
- C-Object/object, C-Object/map, and C-Map/object register their `(dst, src)`
  pair in the cycle set before they check the properties; re-entry of a
  registered pair is `Safe` (§12).
- An implementation may apply C-Out and C-Prom lazily: a context flag records
  which wrappers the source may shed, and the wrappers are removed when the
  recursion reaches them. The lazy form equals the eager rule above.

### 4.3 Laws

Let `a`, `b`, `c` be types.

| Law | Statement | Status |
|---|---|---|
| Reflexive | `conv(a, a) = Safe` | proved |
| Dynamic is top | `conv(dynamic, a) = Safe` | proved |
| Dynamic source | `conv(a, dynamic) = Unsafe` for scalar `a ≠ dynamic` | proved for one instance; bounded |
| Safe ⊆ Unsafe | `conv(a, b) = Safe` implies `conv(a, b) ≠ No` | proved |
| Safe transitive | `conv(a, b) = Safe ∧ conv(b, c) = Safe ⇒ conv(a, c) = Safe` | bounded without objects; not claimed with objects |
| Symmetric | `conv(a, b) = conv(b, a)` | not claimed: `number <- int` is `Safe`, `int <- number` is `Unsafe` |

§6 defines the status values; `formal/README.md` names the evidence for each
row.

Safe transitivity fails on objects because C-Object drops extra source keys
and C-Map reads every source key. Witness: `conv(map(int), {a: int}) = Safe`,
`conv({a: int}, {a: int, b: list(int)}) = Safe`, but
`conv(map(int), {a: int, b: list(int)}) = No`. go-cty has the same gap. The
soundness of unification (§6) does not depend on transitivity.

### 4.4 The scalar table

Rows are `dst`, columns are `src`. The diagonal is C-Eq.

| dst \ src | bool   | int    | number | string | id     |
|-----------|--------|--------|--------|--------|--------|
| bool      | Safe   | Unsafe | Unsafe | Unsafe | Unsafe |
| int       | Unsafe | Safe   | Unsafe | Unsafe | Unsafe |
| number    | Unsafe | Safe   | Safe   | Unsafe | Unsafe |
| string    | Safe   | Safe   | Safe   | Safe   | Safe   |
| id        | Safe   | Safe   | Safe   | Safe   | Safe   |

## 5. Unification

### 5.1 One result

`unify(T₁, …, Tₙ)` returns one type. The Go API `UnifyTypes` returns the
same type twice, as the safe result and as the unsafe result. One result is
enough because the structural rules below push every union to the leaf where
a conversion is no longer safe. A second, union-free result would not convert
safely from every input and would differ between argument orders. §11(f)
records the alternatives.

`unify` is n-ary. The binder calls it once with every input (§9). A left fold
of the binary form gives a different result in some cases (§6, associativity),
so callers must not fold.

### 5.2 Rules

The function is defined on the list of inputs. `members(T)` is the member
list of a union and `[T]` for every other type.

```
U-Empty    unify()                       = none
U-Eq       unify(T, …, T)                = T
U-Flatten  unify(T₁, …, Tₙ)              = merge(norm(members(T₁) ++ … ++ members(Tₙ)))
```

`norm` is the canonical member list of §3: it flattens, sorts with `≺`, and
removes duplicates. `merge(M)` is defined on the member list `M`:

```
U-None     N  = [none] if none ∈ M, else []
           M' = M without none
U-Dynamic  if dynamic ∈ M':            merge(M) = union(N ++ [dynamic])
U-Output   if some member of M' is an output, or some member is a promise
           and some member contains an output:
                                       merge(M) = union(N ++ [output(unify(strip(M')))])
U-Promise  else if some member of M' is a promise:
                                       merge(M) = union(N ++ [promise(unify(strip(M')))])
U-Union    else                        merge(M) = union(N ++ seq(M') ++ map(M') ++ scalar(M'))
```

`union(…)` is the constructor of §3, so a single member stands alone and no
member gives `none`.

`strip` removes one top-level `output` or `promise` wrapper from each member
and leaves other members unchanged. The `output(…)` and `promise(…)` in
U-Output and U-Promise are the constructors of §2.3, so they resolve the
nested eventuals of the result.

The three classes partition the members of `M'` that are not `none` and not
`dynamic`: the sequence class holds `list`, `set`, and `tuple`; the map class
holds `map` and `object`; the scalar class holds the five scalars, constants,
enums, and opaque types. Each class merges to at most one member, except the scalar class,
which merges to its maximal members.

```
U-Seq      seq(M') on the sequence members S of M'; [] if S is empty.
U-Tuple    if every member is a tuple of the same length n:
              [tuple(unify(S₁[1], …, Sₖ[1]), …, unify(S₁[n], …, Sₖ[n]))]
U-Set      else if every member is a set:   [set(unify(elems(S)))]
U-List     else:                            [list(unify(elems(S)))]
```

`elems(S)` lists the element type of each list and set and every element
type of each tuple.

```
U-Map      map(M') on the map members O of M'; [] if O is empty.
U-Object   if every member is an object:
              [object({k: unify(O₁[k], …, Oₖ[k]) for k in the union of the keys})]
           where a missing property Oᵢ[k] is none
U-MapOf    else: [map(unify(values(O)))]
```

`values(O)` lists the element type of each map and every property type of
each object. U-MapOf registers its canonical member list `norm(values(O))`
before the recursive call; §12.2 gives the re-entry rule.

```
U-Scalar   scalar(M') = the members s of the scalar class such that no other
           member s' of the class has below(s, s')
```

### 5.3 The scalar order

`below(s, s')` holds when `s ≠ s'`, `conv(s', s) = Safe`, and either
`conv(s, s') ≠ Safe` or `s' ≺ s`. The second condition
breaks the tie between `string` and `id`, which convert safely in both
directions: `string` wins.

The order on the scalar class is:

- `string` is above `id`, `bool`, `int`, `number`, and every constant and
  enum;
- `id` is above `bool`, `int`, `number`, and every constant and enum;
- `number` is above `int` and above every constant and enum with base `int`
  or `number`;
- `bool` is above every constant and enum with base `bool`;
- the base `B` of an enum is above the enum;
- an enum is above each constant whose value it contains;
- two constants with different values are incomparable;
- an opaque type is incomparable with every other member, so it always
  survives;
- `bool` and `number` are incomparable, and so are `bool` and `int`.

### 5.4 Examples

| Inputs                                                             | Result                                          |
|--------------------------------------------------------------------|-------------------------------------------------|
| `bool, number`                                                     | `union(bool, number)`                           |
| `string, number`                                                   | `string`                                        |
| `int, const(1)`                                                    | `int`                                           |
| `const(1), const(2)`                                               | `union(const(1), const(2))`                     |
| `int, const(0.01)`                                                 | `union(int, const(0.01))`                       |
| `map(const(0.01)), map(int)`                                       | `map(union(int, const(0.01)))`                  |
| `tuple(number), tuple(bool)`                                       | `tuple(union(bool, number))`                    |
| `tuple(int), tuple(int, bool)`                                     | `list(union(bool, int))`                        |
| `list(string), set(bool)`                                          | `list(string)`                                  |
| `set(bool), tuple(int)`                                            | `list(union(bool, int))`                        |
| `set(bool), set(int)`                                              | `set(union(bool, int))`                         |
| `object({a: bool}), map(int)`                                      | `map(union(bool, int))`                         |
| `object({a: int}), object({b: bool})`                              | `object({a: optional(int), b: optional(bool)})` |
| `object({}), object({a: int})`                                     | `object({a: optional(int)})`                    |
| `output(bool), bool`                                               | `output(bool)`                                  |
| `output(int), string`                                              | `output(string)`                                |
| `promise(bool), output(string)`                                    | `output(string)`                                |
| `map(output(bool)), output(map(union(const(false), const(true))))` | `output(map(union(const(false), const(true))))` |
| `optional(map(const(0))), map(const(1))`                           | `union(none, map(union(const(0), const(1))))`   |
| `bool, dynamic`                                                    | `dynamic`                                       |
| `optional(int), dynamic`                                           | `optional(dynamic)`                             |
| `none, bool`                                                       | `optional(bool)`                                |
| `enum(E, int, {1, 2}), int`                                        | `int`                                           |
| `enum(E, int, {1, 2}), const(1)`                                   | `enum(E, int, {1, 2})`                          |
| `enum(E, …), enum(F, …)`                                           | `union(enum(E, …), enum(F, …))`                 |
| `union(int, string), int`                                          | `string`                                        |
| `string, id`                                                       | `string`                                        |
| `list(int), map(int)`                                              | `union(list(int), map(int))`                    |
| `set(string), tuple(int), list(bool)`                              | `list(string)`                                  |

The result of `unify` on two distinct inputs is in merged normal form: no two
members of a result union merge further (§6, "merged normal form").

## 6. Soundness

`a ⊔ b` is `unify(a, b)`. The status values are:

- "proved": a proof holds for every type.
- "bounded": the property is checked on a finite universe of types, and a
  proof by induction is pending.
- "runtime tests": the Go rapid tests check the property.
- "not claimed": the property does not hold; a witness is given.

`formal/README.md` names the evidence for each row and describes the
universes of the bounded checks.

| # | Property | Status |
|---|---|---|
| S1 | `conv(a ⊔ b, a) = Safe` and `conv(a ⊔ b, b) = Safe` | bounded |
| S2 | the unsafe result converts at least unsafely from every input | follows from S1: the unsafe result is the safe result (§5.1) |
| S3 | the unsafe result converts from the safe result | proved: the two results are equal and `conv` is reflexive |
| S4 | `a ⊔ b = b ⊔ a` (symmetry); every permutation of n inputs gives one result | proved from the three laws of the order `≺`; the laws are bounded |
| S5 | `a ⊔ a = a` | proved |
| S6 | `unify` is a function of its inputs | proved: the definition has no state |
| S7 | `conv` and `unify` terminate on recursive types | proved for `conv` (§12.1); bounded for `unify` on trees; runtime tests for the U-MapOf re-entry rule (§12.2) |
| S8 | `C(a) ⊔ C(b) = C(a ⊔ b)` for `list`, `set`, `map`, one-element `tuple`, one-key `object` on every input; for `output` and `promise` on inputs without eventuals | bounded |
| S9 | reflexive; Safe-transitive without objects; `conv(dynamic, a) = Safe` | proved; bounded; proved |
| S10 | `a ⊔ dynamic = dynamic` when `a` has no `none` member; `= optional(dynamic)` otherwise | proved; bounded |
| — | `unify(a, b, a ⊔ b) = a ⊔ b` | bounded |
| — | the scalar class absorbs: scalar `a`, `b` with `conv(b, a) = Safe` and `conv(a, b) ≠ Safe` give `a ⊔ b = b` | bounded |
| — | merged normal form: no two members of a result union merge further | bounded |
| — | the lazy eventual context equals the eager rules C-Out and C-Prom | bounded |
| — | associativity of a binary fold | not claimed |
| — | S8 for `output` on inputs with eventuals | not claimed |
| — | Safe transitivity with objects | not claimed (§4.3) |

S8 does not hold for `output` when an input holds an eventual. Witness: with
`a = list(output(none))` and `b = list(dynamic)`, `output(a ⊔ b)` is
`output(list(dynamic))` because U-Dynamic absorbs the member `output(none)`,
while `output(a) ⊔ output(b)` is `output(list(optional(dynamic)))` because
the resolved element `none` stays outside U-Dynamic (U-None).

Associativity is not claimed. The n-ary result differs from a binary fold:
`({a: int} ⊔ {b: bool}) ⊔ map(const(0))` is `map(union(bool, int, none))`,
because the object merge makes both keys optional, while
`unify({a: int}, {b: bool}, map(const(0)))` is `map(union(bool, int))`.
Both results satisfy S1. The binder avoids the fold (§9). go-cty is not
associative either (§7).

Symmetry is a hard requirement: no rule of §5 reads the position of an
input. U-Flatten sorts the member multiset with the total order `≺` before
any merge, and the one tie-break, `string` against `id` in U-Scalar, uses
that order. The proof of S4 has two parts: `unify(a, b) = unify(b, a)` follows
from `norm([a, b]) = norm([b, a])`, and `norm` is a function of the member
multiset when `≺` is transitive, total, and antisymmetric. The three order
laws are bounded; their proof by induction over the grammar is the one open
obligation for S4. Bug 27 is the test case: `unify(int, const(0.01))` and
`unify(const(0.01), int)` are both `union(int, const(0.01))`.

The open obligation for S1 is an induction over the depth of the inputs with
the per-class lemmas of §5.2.

## 7. go-cty compatibility

go-cty has no `int`, `id`, `none`, constants, enums, unions, outputs, or
promises. The comparison covers the common types `bool`, `number`, `string`,
`dynamic` (`any`), `list`, `set`, `map`, `tuple`, and `object`.

### 7.1 Conversion

| Pair | cty | PCL | Note |
|---|---|---|---|
| `number <- bool`, `bool <- number` | none | Unsafe | PCL keeps every scalar pair convertible |
| `string <- bool`, `string <- number` | safe | Safe | agree |
| `number <- string`, `bool <- string` | unsafe | Unsafe | agree |
| `list(B) <- list(A)`, `set <- set`, `map <- map` | element | element | agree |
| `list(B) <- set(A)` | element | element | agree |
| `set(B) <- list(A)` | unsafe | `cap` | agree |
| `set(B) <- tuple(As)` | safe if elements safe | `cap` | PCL counts the loss of order and duplicates |
| `list(B) <- tuple(As)` | elements | elements | agree |
| `tuple <- tuple` | same length, element-wise | same length, element-wise | agree |
| `tuple(Ts) <- list(U)`, `tuple(Ts) <- set(U)` | none | `cap` | PCL structural conversion |
| `object(B) <- object(A)` | extra keys dropped; missing optional keys null | extra keys dropped; missing keys checked against `none` | agree, with `optional(T)` as the optional marker |
| `map(B) <- object(A)` | every attribute | every property | agree |
| `object(B) <- map(A)` | unsafe | `cap` | agree |
| `list`/`set` <-> `map` | none | No | agree |
| `any <- T` | safe | Safe | agree |
| `T <- any` | unsafe | Unsafe | agree |
| nested `any` in a safe check | not a wildcard | not a wildcard | agree: `list(int) <- list(dynamic)` is `Unsafe` |

### 7.2 Unification

| Inputs | cty `Unify` | cty `UnifyUnsafe` | PCL | Note |
|---|---|---|---|---|
| `number, string` | `string` | `string` | `string` | agree |
| `bool, string, number` | `string` | `string` | `string` | agree |
| `bool, number` | fail | fail | `union(bool, number)` | PCL never fails |
| `string, any` | `any` | `string` | `dynamic` | agrees with safe |
| `any, any` | `any` | `any` | `dynamic` | agree |
| `list(string), set(string)` | `list(string)` | `list(string)` | `list(string)` | agree |
| `list(string), set(number)` | `list(string)` | `list(string)` | `list(string)` | agree |
| `list(number), set(string)` | fail | `list(number)` | `list(string)` | PCL gives the safe join |
| `list(string), map(string)` | fail | fail | `union(list(string), map(string))` | PCL never fails |
| `list(string), tuple(number)` | `list(string)` | `list(string)` | `list(string)` | agree |
| `tuple(number), set(string)` | `set(string)` | `set(string)` | `list(string)` | PCL: `set <- tuple` is Unsafe, so the join is a list |
| `tuple(bool), tuple(number)` | fail | fail | `tuple(union(bool, number))` | PCL never fails |
| `tuple(), tuple(string)` | `list(string)` | `list(string)` | `list(string)` | agree |
| `tuple(), object({})` | fail | fail | `union(tuple(), object({}))` | PCL never fails |
| `map(string), object({a: number})` | `map(string)` | `map(string)` | `map(string)` | agree |
| `object({a: string}), object({a: number})` | `object({a: string})` | same | `object({a: string})` | agree |
| `object({a: string}), object({a: string, b: number})` | `map(string)` | `map(string)` | `object({a: string, b: optional(number)})` | PCL keeps objects |
| `object({a: string}), object({b: bool})` | `map(string)` | `map(string)` | `object({a: optional(string), b: optional(bool)})` | PCL keeps objects |
| `object({a: number}), object({a: bool})` | fail | fail | `object({a: union(bool, number)})` | PCL never fails |
| `object({a: string}), any` | `any` | `any` | `dynamic` | agree |
| `list(string), list(any)` | `list(any)` | `list(string)` | `list(dynamic)` | agrees with safe |
| `tuple(number), any, object({num: number})` | fail | fail | `dynamic` | PCL never fails |
| `list(tuple(number)), list(tuple(string))` | `list(tuple(string))` | same | `list(tuple(string))` | agree |

Where cty succeeds, PCL agrees with cty's safe `Unify` except for `set` with
`tuple` and for objects with different key sets. Where cty fails, PCL returns
a union or a structural merge. PCL never fails, so the binder never reports
"inconsistent types" for a conditional; it reports a conversion error at the
use site instead.

## 8. Compatibility with the Go implementation

The table lists every observable difference between this specification and
the package as implemented. "Bug" refers to `TODO.md` at the repository root.

### 8.1 Changes that fix a bug

| Case                                                                   | Implementation                                                        | Specification                            | Bug                   |
|------------------------------------------------------------------------|-----------------------------------------------------------------------|------------------------------------------|-----------------------|
| `UnifyTypes(optional(map(0)), map(1))`                                 | `union(none, map(0))`, which `map(1)` does not convert to             | `union(none, map(union(0, 1)))`          | 28                    |
| `UnifyTypes(map(output(false)), output(map(union(false, true))))`      | `union(map(output(false)), output(map(…)))`                           | `output(map(union(false, true)))`        | 25                    |
| `UnifyTypes(int, 0.01)` and `UnifyTypes(0.01, int)` unsafe result      | `int` and `0.01`                                                      | `union(int, 0.01)` in both orders        | 27                    |
| `UnifyTypes(map(0.01), map(int))`                                      | safe `map(union(int, 0.01))`, unsafe `map(0.01)`                      | `map(union(int, 0.01))`                  | 18                    |
| `UnifyTypes(tuple(number), tuple(bool))`                               | safe `tuple(union(bool, number))`, unsafe `tuple(number)`             | `tuple(union(bool, number))`             | 18                    |
| `UnifyTypes(set(…), tuple(…))` with the bug 24 inputs                  | assertion failure                                                     | `list(…)` that converts safely from both | 24                    |
| `UnifyTypes(set(string), tuple(int), list(bool))` and its permutations | `union(set(string), tuple(int))` or `list(string)` by order           | `list(string)`                           | §5 of `current-model` |
| `UnifyTypes(union(bool, number), bool, number)` unsafe result          | `union(bool, number)` while `UnifyTypes(bool, number)` gives `number` | `union(bool, number)` in both            | idempotence           |
| `union(bool, dynamic) <- dynamic`                                      | `Unsafe`                                                              | `Safe`                                   | —                     |
| `output(map(bool)) <- map(output(bool))`                               | `No`                                                                  | `Safe`                                   | 25                    |
| `list(a) <- list(b)` for recursive `a`, `b` after `a <- b`             | result depends on call history                                        | pure function                            | §10                   |
| `union(enum, output(enum)) <- const(v)` with `v` not in the enum       | `Safe`                                                                | `No`                                     | 7                     |

### 8.2 Changes that simplify the system

| Case                                            | Implementation                                          | Specification                                   | Reason                                                         |
|-------------------------------------------------|---------------------------------------------------------|-------------------------------------------------|----------------------------------------------------------------|
| `UnifyTypes` returns two results                | safe and unsafe differ                                  | one result, returned twice                      | §5.1, §11(f)                                                   |
| `element` and `lookup` use the unsafe result    | `element([true, 1], i)` is `number`                     | `union(bool, const(1))`                         | one result; the unsafe result was order-dependent              |
| `UnifyTypes(tuple(int), tuple(int, bool))`      | `tuple(int, optional(bool))`                            | `list(union(bool, int))`                        | §11(g); matches cty                                            |
| `tuple(int, optional(bool)) <- tuple(int)`      | `Safe`                                                  | `No`                                            | C-Tuple requires equal length; matches cty                     |
| `UnifyTypes(bool, dynamic)`                     | safe `union(bool, dynamic)`, unsafe `bool`              | `dynamic`                                       | §11(e); matches cty                                            |
| `UnifyTypes(optional(int), dynamic)`            | `union(int, dynamic, none)`                             | `optional(dynamic)`                             | U-None, U-Dynamic                                              |
| `UnifyTypes(output(int), string)`               | safe `union(string, output(int))`, unsafe `output(int)` | `output(string)`                                | §11(c)                                                         |
| `UnifyTypes(promise(bool), list(output(bool)))` | `union(…)`                                              | `output(union(bool, list(bool)))`               | U-Output                                                       |
| `UnifyTypes(set(bool), tuple(int))`             | safe `union(set(bool), tuple(int))`, unsafe `set(int)`  | `list(union(bool, int))`                        | U-List; the only join a set and a tuple both convert to safely |
| `UnifyTypes(enum, int)`                         | safe `union(int, enum)`, unsafe `enum`                  | `int`                                           | C-Widen on enums                                               |
| `int <- enum(E, int, …)`                        | `No`                                                    | `Safe`                                          | C-Widen; an enum value is a value of its base                  |
| `enum(E, …) <- enum(F, …)`, same base           | `No`                                                    | `Unsafe`                                        | C-EnumSrc applies to every non-constant source                 |
| `UnifyTypes(union(int, string), int)`           | `union(int, string)`                                    | `string`                                        | the result is in merged normal form                            |
| `null` literal type                             | `const(none, null)`                                     | `none`                                          | §2.2                                                           |
| `const(number, 3)` from a schema                | base `number`                                           | base `int`: the base is a function of the value | §2.2                                                           |
| `const(int, 1) <- const(number, 1)`             | `Safe`                                                  | the two are one type                            | §2.2                                                           |
| `UnifyTypes` with no arguments                  | `none`                                                  | `none`                                          | unchanged                                                      |
| `UnifyTypes(a, nil, …)`                         | skips the nil                                           | not part of the model                           | the caller must pass types only                                |

### 8.3 Dropped corner cases

- `tuple(T₁, …, Tₙ) <- tuple(U₁, …, Uₘ)` with `m < n` and optional `Tᵢ` for
  `i > m`. The conversion is `No`. Unification of tuples of different lengths
  gives a list, so no result of `unify` needs this rule.
- `const(none, null)`. The `null` literal is `none`.
- A constant whose base differs from its value's base.
- The unsafe result as a distinct type.
- Optional padding of tuples in unification.
- `optional(dynamic)` as a unification result from an input that has no
  `none` member. It remains a valid type for signatures.

## 9. Binder call sites

Every caller uses the one result. The call sites that use `UnifyTypes`:

| Site                                  | Inputs                                                   | Use                                            |
|---------------------------------------|----------------------------------------------------------|------------------------------------------------|
| conditional `c ? a : b`               | `unify(type(a), type(b))`                                | result type, then `liftOperationType` with `c` |
| `for` over a tuple or object          | `unify(all element or property types)` in one call       | element type                                   |
| `[*]` over a tuple                    | `unify(all element types)`                               | element type                                   |
| object literal with a non-literal key | `unify(all value types)`                                 | `map(result)`                                  |
| `element` on a tuple                  | `unify(all element types)`                               | element type                                   |
| `lookup` on an object                 | `unify(all property types)` in one call                  | element type                                   |
| `lookup` with a default               | `unify(element type, default type)`                      | return type                                    |
| `try`                                 | `unify(all argument types)`                              | return type                                    |
| `recover`                             | `unify(resolveOutputs(value), resolveOutputs(recovery))` | return type                                    |

`lookup` on an object passes every property type in one call. A fold with a
seed of `nil` is not part of the model (§6, associativity).

The sites that use `conv`:

- argument checks: `conv(InputType(param), arg) ≠ No`;
- resource and component inputs: `conv(propertyType, valueType) ≠ No`;
- `range`: `conv(InputType(bool), t) = Safe` selects a conditional,
  `conv(InputType(number), t) = Safe` selects a numeric range;
- the conversion rewrite for a union target: a member that is `AssignableFrom`,
  else a member with `Safe`, else a member with `Unsafe`;
- `min`/`max`: `conv(number, arg) ≠ No` and `conv(int, arg) ≠ Safe` selects
  the number signature.

The sites that unwrap a result before iteration accept `output(T)`,
`promise(T)`, and `optional(T)` around a collection. A union with two
non-`none` members is not iterable; with the rules of §5 such a union arises
only from inputs of different classes, for example `list` and `map`.

## 10. Caching

`conv(dst, src)` and `unify(T₁, …, Tₙ)` are pure functions of their
arguments. A cache keyed by the argument types, up to equality (§2.5), is
sound. These caches are not sound:

- a cache keyed by pointer identity when two distinct pointers are equal
  types; this gives a miss, which is harmless, or a hit on a stale type when a
  type mutates after construction, which is not;
- a cache entry written while a recursive pair is in flight (§12): the value
  of `conv(a, b)` under the coinductive assumption `conv(a', b') = Safe` for an
  outer pair `(a', b')` is correct only for the outermost call. A cache may
  store the result of the outermost call. It must not store the results of
  the inner calls that ran under the assumption.

A cache of `unify` may store the result for a sorted, deduplicated member
list, because U-Flatten makes the result a function of that list.

## 11. Design decisions

### (a) Structural first; union only at the leaves

Unification merges every member of the same class into one member (§5.2).
A union appears only between members of different classes (`list` and
`map`, `string` and `list`) and between incomparable scalars (`bool` and
`number`, `const(1)` and `const(2)`). This gives S8 by construction and
fixes bug 18.

"Same structure" is: `list`, `set`, and `tuple` are one class; `map` and
`object` are one class. Within the sequence class, tuples of one length merge
to a tuple, sets merge to a set, and any other mix merges to a list. Within
the map class, objects merge to an object and any mix with a map merges to a
map.

Rejected: a top-level rule "if `conv(a, b) = Safe` then `a ⊔ b = a`". It
conflicts with the object merge: `{} ⊔ {a: int}` would be `{}` instead of
`{a: optional(int)}`, and `union(int, string) ⊔ int` would depend on whether
the union came from a schema or from a merge. The scalar class keeps the
absorption rule (U-Scalar), where it is the whole rule.

### (b) Constants and enums

A constant unifies with its base to the base: `const(1) ⊔ int = int`. Two
constants with different values unify to their union: `const(1) ⊔ const(2) =
union(const(1), const(2))`. An enum absorbs its member constants and is
absorbed by its base.

Rejected: `const(1) ⊔ const(2) = int`. The union is more precise, converts
safely to `int` where needed, and keeps enum membership checks exact for
`c ? 1 : 2` assigned to an enum property. `lookup` and `for` consumers
already handle unions of constants from tuple literals.

Rejected: nominal `int <- enum = No`. An enum value is a value of its base.
C-Widen on enums makes `enum ⊔ int = int` and lets an enum output feed a
string property.

### (c) Eventual lift

`output(T) ⊔ U = output(T ⊔ resolveOutputs(U))` (U-Output). A promise lifts
the same way unless some member contains an output, in which case the result
is an output. This fixes bug 25 and matches `liftOperationType`.

Rejected: `output(T) ⊔ U = union(output(T), U)`. It satisfies S1 but every
iteration site rejects the union.

### (d) None

`T ⊔ none = optional(T)`. `none` joins no class and stays a member of the
result union, so `optional(output(T)) ⊔ U` keeps the optional marker outside
the output and `optional(T)` stays detectable by `IsOptionalType`.

### (e) Dynamic

`dynamic` absorbs every member except `none` (U-Dynamic). This matches cty
in safe mode and in unsafe mode for structural inputs. cty's unsafe mode
prefers the primitive over `any`; the binder has no use for that result, and
`dynamic` converts from every input safely while `bool` converts from
`dynamic` only unsafely.

### (f) One result

One result suffices. The safe result with unions at the leaves converts
safely from every input (S1). A second result that collapses unions to a
representative is derivable, `collapse(union(bool, number)) = number`, but it
converts from `bool` only unsafely and no binder call site needs it. The two
sites that used the unsafe result, `element` and `lookup`, produce an element
type for later conversion checks, and the union serves that purpose.

### (g) Tuples of different lengths

`tuple(As) ⊔ tuple(Bs)` with `|As| ≠ |Bs|` is `list(unify(As ++ Bs))`. This
matches cty and needs no optional padding and no shorter-source rule in
C-Tuple.

Rejected: `tuple(int) ⊔ tuple(int, bool) = tuple(int, optional(bool))`. It
needs C-Tuple to accept a shorter source, which makes `tuple` conversion
differ from cty, and it reports a bind-time index error that a list defers to
run time.

### (h) Union sources

C-USrc precedes C-UDst and uses the `members` fold (§4.1): `Safe` only when
every member converts safely, `Unsafe` when some member converts. A union
value is one of its members, so a member that does not convert is a run-time
failure, not a bind-time one.

## 12. Recursive object types

An object type may refer to itself through a property. The model represents
such a type as a finite graph of object nodes. The rules of §2 to §6 are
stated on finite trees; this section specifies the graph case.

### 12.1 Conversion

`conv` carries a cycle set of `(dst, src)` pairs in flight. The rules
C-Object/object, C-Object/map, and C-Map/object register their pair: on entry
with the pair in the set, the result is `Safe` (the coinductive assumption);
otherwise the pair enters the set for the duration of the property checks.
A pair with a `map` on one side must register too: with `a = {k: map(a)}`,
C-Object/map expands `conv(a, map(a))` to `conv(map(a), a)`, and C-Map/object
expands that to `conv(a, map(a))` again. With registration,
`conv(a, map(a)) = Unsafe` and `conv(map(a), a) = Unsafe`.

Termination: let `S` be the set of types that occur in the environment, in
`dst`, or in `src`, closed under subterms; `S` is finite. Every recursive call
either adds a pair over `S` to the cycle set or keeps the set and reduces the
sum of the syntactic sizes of its arguments, where a node reference counts
as size one. The pairs over `S` are finite, so the recursion terminates.
The measure is the pair `(number of pairs over S not in the cycle set,
size(dst) + size(src))` under the lexicographic order. `formal/README.md`
names the proof.

### 12.2 Unification

`unify` carries two tables of work in flight. U-Object keeps a map from
`(a, b)` object pairs to the object under construction: on entry with the
pair in the map, the result is that object; otherwise a new empty object
enters the map, the properties are merged, and the object is filled.

U-MapOf keeps the stack of canonical member lists `norm(values(O))` in
flight. A list that is not on the stack enters it for the recursive call. On
re-entry with a list `L` already on the stack:

- if no U-Object placeholder entered the map after `L`, the result is
  `union(L)`;
- if a placeholder entered the map after `L`, `L` unifies again; the
  recursion ends at that placeholder, because every list in flight is an
  ancestor on the stack and the placeholder lies on the replayed path.

A placeholder object is not possible for a map, because a cycle through a
`map` is not a type that the grammar can name. With `a = {k: map(a)}`,
`unify(map(a), a)` is `map(union(a, map(a)))`: `values = [a, map(a)]`, the
recursive call `unify(a, map(a))` reaches U-MapOf with the same list, no
placeholder lies between the two entries, and the result is
`union(a, map(a))`. The result converts safely from both inputs and is
symmetric.

The second case keeps S8 on recursive objects. With
`a = {a: optional(a), b: map(a)}` and `b = {a: bool, b: map(b)}`,
`unify(map(a), map(b))` enters `L = [map(a), map(b)]`, U-Object on `[a, b]`
pushes the placeholder `r`, and the key `b` re-enters `L`. The plain rule
would give `r.b = union(map(a), map(b))`, while `map(unify(a, b))` gives
`r.b = map(r)`. With the refinement both give `map(r)`.

Termination of `unify` follows the same measure as `conv`: U-Object adds an
object pair, U-MapOf adds a member list over the finite set of lists of
types from `S`, and every other rule reduces the size. On trees the
recursion depth of `unify` is bounded by the size of the inputs. The U-MapOf
re-entry rule has no counterpart on trees; the Go rapid tests over recursive
objects check it.

`Equals` on objects carries a set of pairs in the same way as `conv`.
