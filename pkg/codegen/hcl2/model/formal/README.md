# Formal model of PCL conversion and unification

This directory holds the Lean 4 model of `../README.md`. The model uses core
Lean only; it has no dependency on Mathlib.

## Build

Install `elan` (https://github.com/leanprover/elan). `lean-toolchain` pins
the Lean version; `elan` downloads it on the first build.

```sh
cd pkg/codegen/hcl2/model/formal
lake build          # checks every proof and every bounded check
lake build corpus   # builds the corpus exporter
./.lake/build/bin/corpus > corpus.json
```

`lake build` takes between two and a half and five minutes on an Apple M-series laptop. Most of
the time goes to `Formal/Bounded.lean`, which compiles the model to native
code and runs the bounded checks with `native_decide`. Shrink the universes
`u1`, `u2`, and `u3` in that file to trade coverage for build time.

## Files

| File | Content |
|---|---|
| `Formal/Types.lean` | the type grammar `Ty`, the value grammar `Val`, the kind `Kind` |
| `Formal/DecEq.lean` | decidable equality for `Ty`; generated, not part of the rules |
| `Formal/Norm.lean` | the order `Ty.cmp`, the union constructor `mkUnion`, `resolveOutputs`, `resolvePromises`, the classes |
| `Formal/Conv.lean` | `conv` and `convIn`: one match arm per rule of README §4 |
| `Formal/Unify.lean` | `unify` and `unifyF`: one definition per rule of README §5 |
| `Formal/Laws.lean` | theorems with proofs (README §6) |
| `Formal/Bounded.lean` | universes, checks, and `native_decide` theorems (README §6) |
| `Formal/Rec.lean` | `convR`: the conversion rules over a node graph with a cycle set that registers object/object and object/map pairs, with a termination proof (README §12) |
| `Corpus.lean` | the corpus exporter |
| `corpus.json` | the golden corpus |

Read `Conv.lean` and `Unify.lean` as the rules. The comment at the end of
each match arm names the rule in `../README.md`.

## Reading the Lean

- `Kind.no`, `Kind.lossy`, `Kind.safe` are `NoConversion`,
  `UnsafeConversion`, `SafeConversion`.
- `convIn ctx dst src` carries a context that records which eventual
  wrappers the source may shed (README §4.2, notes). `conv dst src` is
  `convIn .plain dst src`.
- `unifyF fuel ts` takes a fuel that bounds the recursion depth. `unify ts`
  supplies one unit of fuel per node of the inputs. `fuel_u1` checks that
  more fuel gives the same result.
- `mkUnion`, `mkOutput`, `mkPromise`, `mkObject` are the smart constructors.
  The raw constructors `Ty.union`, `Ty.output`, `Ty.promise`, `Ty.object`
  are for pattern matches; build types with the smart constructors so that
  the normal forms hold.

## Property → evidence

The table mirrors §6 of `../README.md` and names the evidence for each row.
"Proof" is a theorem in `Formal/Laws.lean` or a definition that the
termination checker accepts. "Bounded" is a `native_decide` theorem in
`Formal/Bounded.lean` over the universe named in the row; the universes are
described below. "Runtime tests" are the Go rapid tests.

| # | Property | Evidence |
|---|---|---|
| S1 | the result converts safely from each input | `s1_u2` (pairs over `u2`), `s1n_u3` (triples over `u3`) |
| S2 | the unsafe result converts from every input | the unsafe result is the safe result; S1 |
| S3 | the unsafe result converts from the safe result | `conv_refl`; the two results are equal |
| S4 | symmetry; permutation invariance | `unify2_symm_of_canon`, `canon_perm`, `unify2_symm` (proofs from `LawfulLe`); `le_laws_u1` (the three order laws on every fourth type of `u1`), `canon_comm_u2`, `s4_u2`, `s4n_u3`, `bug27_symm` (bounded) |
| S5 | idempotence | `unify_idem`, `unify_replicate` (proofs) |
| S6 | determinism | `unify_deterministic` (proof); the definition has no state |
| S7 | termination on recursive types | `convR` in `Formal/Rec.lean` is accepted by the termination checker (proof); `unifyF` recurses on the fuel, `fuel_u1` checks that more fuel gives the same result (bounded); the U-MapOf re-entry rule of README §12.2 is checked by the Go rapid tests over recursive objects, not by Lean |
| S8 | structural congruence | `s8_u1` (every fourth type of `u1`; `output` and `promise` on inputs without eventuals); `s8_output_counter` (the witness) |
| S9 | reflexive; Safe-transitive without objects; dynamic is top | `conv_refl`, `conv_dynamic` (proofs); `safe_trans_u3` (bounded, `u3` without objects); `safe_trans_counter` (the witness) |
| S10 | dynamic absorbs | `unify_dynamic` (proof, inputs without a `none` member); `dynamic_u2` (bounded) |
| — | `unify(a, b, a ⊔ b) = a ⊔ b` | `idem_result_u1` |
| — | the scalar class absorbs | `scalar_absorb_u1` |
| — | merged normal form | `normal_u1` |
| — | the lazy eventual context equals the eager rules | `lazy_resolve_u1` |
| — | associativity of a binary fold | `assoc_counter` (the witness) |
| — | S8 for `output` on inputs with eventuals | `s8_output_counter` (the witness) |
| — | Safe transitivity with objects | `safe_trans_counter` (the witness) |

The laws of README §4.3:

| Law | Evidence |
|---|---|
| Reflexive | `conv_refl` (proof) |
| Dynamic is top | `conv_dynamic` (proof) |
| Dynamic source | `conv_from_dynamic_int` (proof, one instance); `dynamic_u2` covers `conv(dynamic, a)` on `u2` |
| Safe ⊆ Unsafe | `Kind.safe_exists` (proof) |
| Safe transitive | `safe_trans_u3` (bounded, no objects); `safe_trans_counter` |

Induction pending means the law holds on every checked universe and a proof
by induction over `Ty` is not yet written. The open obligation for S4 is
`LawfulLe`: transitivity, totality, and antisymmetry of `Ty.le`, which
`le_laws_u1` checks. With it, `List.Perm.eq_of_pairwise` gives permutation
invariance of `canon`. The open obligation for S1 is an induction over the
fuel of `unifyF` with the per-class lemmas of README §5.2.

### Universes

- `leaves`: `bool`, `int`, `number`, `string`, `id`, `dynamic`, `none`,
  `const(0)`, `const(1)`, `const(0.01)`, `const("a")`, `const(true)`,
  `enum(E, int, {1, 2})`, `enum(F, int, {1, 2})`.
- `u1` (597 types): the leaves, each unary constructor over a leaf, tuples
  and objects with up to two leaves, and unions of two leaves.
- `u2` (1,086 types): `u1`, each unary constructor over every twelfth type
  of `u1`, and tuples, objects, and unions of pairs from every eightieth
  type of `u1`.
- `u3` (34 types): the leaves and twenty hand-picked types that cover the
  repro cases of README §8.

The universe sizes keep the build of `Formal/Bounded.lean` between two and a
half and five minutes on an Apple M-series laptop. A larger universe scales
with the square of its size for the pair checks and the cube for the triple
checks.

### Lazy resolution of eventuals

`convIn` applies the rules C-Out and C-Prom lazily: a context flag
(`Ctx.inOutput`, `Ctx.inPromise`) records which wrappers the source may shed,
and the wrappers are removed when the recursion reaches them. This keeps the
recursion structural, so the termination checker accepts `convIn` under the
measure `sizeOf dst + sizeOf src`. The check `lazy_resolve_u1` confirms that
the lazy form equals the eager rule of README §4.2 on `u1`.

### Termination on recursive object types

`Formal/Rec.lean` proves README §12.1. `convR` is defined over a reduced
grammar `RTy` (leaf, list, map, node reference) with the environment `Γ`,
the finite set `S` of reachable types (`Closed Γ S`), and the cycle set
`seen` as arguments. Lean accepts it under the lexicographic measure
`(unseenOf (allPairs S) seen, sizeOf dst + sizeOf src)`. The three arms
that register a pair (C-Object/object, C-Object/map, C-Map/object) lower the
first component by `unseen_lt`; every other arm lowers the second. The
examples `exEnv` and `mapEnv` exercise the object/object and object/map
cycles; `by decide` discharges `Closed` for a concrete environment.

## The corpus

`corpus.json` is a JSON array. Each entry has the shape:

```json
{"a": <type>, "b": <type>,
 "conv_ab": "safe" | "unsafe" | "no",
 "conv_ba": "safe" | "unsafe" | "no",
 "safe": <type>, "unsafe": <type>}
```

`conv_ab` is `conv(a, b)`: the kind of conversion from `b` to `a`.
`conv_ba` is `conv(b, a)`. `safe` is `unify(a, b)`; `unsafe` is the same
type (README §5.1).

A type is one of:

```json
{"kind": "bool" | "int" | "number" | "string" | "id" | "dynamic" | "none"}
{"kind": "const", "value": {"bool": true} | {"int": 1} | {"number": "0.01"} | {"string": "a"}}
{"kind": "enum", "token": "E", "base": <type>, "values": [<value>, ...]}
{"kind": "list" | "set" | "map" | "output" | "promise", "elem": <type>}
{"kind": "tuple", "elems": [<type>, ...]}
{"kind": "object", "props": {"a": <type>, ...}}
{"kind": "union", "members": [<type>, ...]}
```

A `number` constant carries its decimal text. The `token` of an enum is its
identity; two enums with one token are one type.

### The corpus test

`corpus_test.go` in this directory decodes each entry into `model.Type` values with the package
constructors (`NewConstType`, `NewEnumType`, `NewListType`, `NewUnionType`,
and so on), then checks:

- `a.ConversionFrom(b)` equals `conv_ab` and `b.ConversionFrom(a)` equals
  `conv_ba`;
- `UnifyTypes(a, b)` is `Equals` to `safe`.

`Equals` on unions compares members as a set, so the member order in the
corpus does not matter. The corpus covers 48 types and every pair of them;
the Lean universes in `Bounded.lean` are larger. A rapid property test in Go
covers the laws of README §6 beyond the corpus.

To regenerate the corpus after a rule change, run `lake build corpus` and
the exporter as above, and commit the new `corpus.json` together with the
rule change.
