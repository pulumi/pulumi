import Formal.Unify

/-!
# Bounded checks

Every property in this file is checked by `native_decide` over a finite
universe of types. `README.md` §6 lists which property each check covers and
which universe it uses.

* `u1`: every leaf, plus one constructor over the leaves (tuples, objects and
  unions of at most two leaves).
* `u2`: `u1`, plus one constructor over a sample of `u1`.
* `u3`: a small sample used for the triple checks.
-/

namespace Formal

open Ty

def c0 : Ty := .const (.int 0)
def c1 : Ty := .const (.int 1)
def p01 : Ty := .const (.number "0.01")
def cs : Ty := .const (.string "a")
def cb : Ty := .const (.bool true)
def e12 : Ty := .enum "E" .int [.int 1, .int 2]
def e12' : Ty := .enum "F" .int [.int 1, .int 2]

def leaves : List Ty :=
  [bool, int, number, string, id, dynamic, none, c0, c1, p01, cs, cb, e12, e12']

/-- One constructor over `base`; binary shapes over `pairs`. -/
def grow (base pairs : List Ty) : List Ty :=
  base.map list ++ base.map set ++ base.map map ++ base.map output ++ base.map promise
    ++ [tuple []] ++ base.map (tuple [·])
    ++ pairs.flatMap (fun a => pairs.map fun b => tuple [a, b])
    ++ [object []] ++ base.map (fun t => object [("a", t)])
    ++ pairs.flatMap (fun a => pairs.map fun b => object [("a", a), ("b", b)])
    ++ pairs.flatMap (fun a => pairs.map fun b => mkUnion [a, b])

def u1 : List Ty := dedup ((leaves ++ grow leaves leaves).mergeSort Ty.le)

/-- Every `n`-th type of a universe. -/
def every (n : Nat) (u : List Ty) : List Ty :=
  (u.zipIdx.filter fun p => p.2 % n == 0).map (·.1)

/-- A sample of `u1` for unary constructors at depth 2. -/
def sample1 : List Ty := every 12 u1

/-- A smaller sample of `u1` for binary shapes at depth 2. -/
def sample2 : List Ty := every 80 u1

def u2 : List Ty := dedup ((u1 ++ grow sample1 sample2).mergeSort Ty.le)

/-- The triple universe. -/
def u3 : List Ty :=
  leaves ++ [list int, set bool, tuple [int], tuple [int, bool], map c0, map c1,
    object [("a", int)], object [("b", bool)], object [("a", c0), ("b", string)],
    output int, output (map c0), promise bool, optional (map c0), optional int,
    mkUnion [int, string], list (output bool), output (list bool), tuple [],
    set (tuple []), tuple [tuple [], tuple [list bool]]]

def pairs (u : List Ty) (p : Ty → Ty → Bool) : Bool :=
  u.all fun a => u.all fun b => p a b

def triples (u : List Ty) (p : Ty → Ty → Ty → Bool) : Bool :=
  u.all fun a => u.all fun b => u.all fun c => p a b c

/-! ## The checks -/

/-- S1: the result converts safely from each input. -/
def checkS1 (u : List Ty) : Bool := pairs u fun a b =>
  let r := unify2 a b
  conv r a == .safe && conv r b == .safe

/-- S1 for three inputs. -/
def checkS1n (u : List Ty) : Bool := triples u fun a b c =>
  let r := unify [a, b, c]
  conv r a == .safe && conv r b == .safe && conv r c == .safe

/-- S4: the binary result does not depend on the argument order. -/
def checkS4 (u : List Ty) : Bool := pairs u fun a b => unify2 a b == unify2 b a

/-- S4 for three inputs: every permutation gives the same result. -/
def checkS4n (u : List Ty) : Bool := triples u fun a b c =>
  let r := unify [a, b, c]
  unify [a, c, b] == r && unify [b, a, c] == r && unify [b, c, a] == r
    && unify [c, a, b] == r && unify [c, b, a] == r

/-- A witness that a binary fold is not the n-ary result: the object merge
makes `a` and `b` optional, and the `none` then reaches the map element. -/
def assocCounter : Bool :=
  let a := object [("a", int)]
  let b := object [("b", bool)]
  let c := map c0
  unify2 (unify2 a b) c == map (mkUnion [bool, int, none])
    && unify [a, b, c] == map (mkUnion [bool, int])

/-- Idempotence across results: adding the result to the inputs changes nothing. -/
def checkIdemResult (u : List Ty) : Bool := pairs u fun a b =>
  unify [a, b, unify2 a b] == unify2 a b

/-- S8: structural congruence for every collection constructor. -/
def checkS8 (u : List Ty) : Bool := pairs u fun a b =>
  let r := unify2 a b
  unify2 (list a) (list b) == list r
    && unify2 (set a) (set b) == set r
    && unify2 (map a) (map b) == map r
    && unify2 (tuple [a]) (tuple [b]) == tuple [r]
    && unify2 (object [("k", a)]) (object [("k", b)]) == object [("k", r)]
    && (containsOutputs a || containsPromises a || containsOutputs b || containsPromises b
        || (unify2 (mkOutput a) (mkOutput b) == mkOutput r
            && unify2 (mkPromise a) (mkPromise b) == mkPromise r))

/-- A witness that S8 fails for `output` on inputs with eventuals: U-Dynamic
absorbs `output(none)` while U-None keeps `none`. -/
def s8OutputCounter : Bool :=
  let a := list (output none)
  let b := list dynamic
  mkOutput (unify2 a b) == output (list dynamic)
    && unify2 (mkOutput a) (mkOutput b) == output (list (optional dynamic))

/-- S9: a safe conversion followed by a safe conversion is a safe conversion,
on the part of the universe without objects. -/
def checkSafeTrans (u : List Ty) : Bool := triples u fun a b c =>
  !(conv a b == .safe && conv b c == .safe) || conv a c == .safe

/-- The laws of `LawfulLe` on a universe: transitivity over triples, totality
and antisymmetry over pairs. -/
def checkLeLaws (u : List Ty) : Bool :=
  triples u (fun a b c => !(Ty.le a b && Ty.le b c) || Ty.le a c)
    && pairs u (fun a b => (Ty.le a b || Ty.le b a) && (!(Ty.le a b && Ty.le b a) || a == b))

/-- The hypothesis of `unify2_symm_of_canon`, directly. -/
def checkCanonComm (u : List Ty) : Bool := pairs u fun a b => canon [a, b] == canon [b, a]

/-- Bug 27: `int` and `0.01` unify to their union in both orders. -/
def bug27 : Bool :=
  unify2 int p01 == mkUnion [int, p01] && unify2 p01 int == mkUnion [int, p01]

/-- A witness that S9 fails on objects: `map(int) <- {a: int} <- {a: int, b: list(int)}`. -/
def safeTransCounter : Bool :=
  let a := map int
  let b := object [("a", int)]
  let c := object [("a", int), ("b", list int)]
  conv a b == .safe && conv b c == .safe && conv a c == .no

/-- S10: dynamic absorbs every input except a `none` member. -/
def checkDynamic (u : List Ty) : Bool := u.all fun a =>
  unify2 a dynamic == (if a.members.contains none then optional dynamic else dynamic)
    && conv dynamic a == .safe

/-- Absorption on scalars: a safe conversion between scalars decides the result. -/
def checkScalarAbsorb (u : List Ty) : Bool := pairs u fun a b =>
  !(a.cls == .scalar && b.cls == .scalar && conv b a == .safe && conv a b != .safe)
    || unify2 a b == b

/-- The fuel that `unify` supplies is enough: more fuel gives the same result. -/
def checkFuel (u : List Ty) : Bool := pairs u fun a b =>
  unifyF (fuelFor [a, b] + 8) [a, b] == unify2 a b

/-- The lazy eventual context of `convIn` agrees with eager resolution for the
destinations that occur under an eventual: an output element holds no
eventual, a promise element holds no promise. -/
def checkLazyResolve (u : List Ty) : Bool := pairs u fun a b =>
  (containsOutputs a || containsPromises a
      || convIn .inOutput a b == conv a (resolveOutputs b))
    && (containsPromises a || b.isOutput
      || convIn .inPromise a b == conv a (resolvePromises b))

/-- The result of unifying two distinct types is in merged normal form:
unifying it with any of its members gives it back. -/
def checkNormal (u : List Ty) : Bool := pairs u fun a b =>
  a == b || (let r := unify2 a b; r.members.all fun m => unify2 r m == r)

end Formal

namespace Formal

/-! ## Theorems checked by `native_decide`

The universe sizes keep the whole file under about three minutes of build
time. `checkS1` and `checkS4` run on `u2`; the other pair checks run on `u1`
or on a sample of it; the triple checks run on `u3`. -/

theorem s1_u2 : checkS1 u2 = true := by native_decide
theorem s4_u2 : checkS4 u2 = true := by native_decide
theorem s1n_u3 : checkS1n u3 = true := by native_decide
theorem s4n_u3 : checkS4n u3 = true := by native_decide
theorem s8_u1 : checkS8 (every 4 u1) = true := by native_decide
theorem idem_result_u1 : checkIdemResult u1 = true := by native_decide
theorem dynamic_u2 : checkDynamic u2 = true := by native_decide
theorem scalar_absorb_u1 : checkScalarAbsorb u1 = true := by native_decide
theorem fuel_u1 : checkFuel u1 = true := by native_decide
theorem lazy_resolve_u1 : checkLazyResolve u1 = true := by native_decide
theorem normal_u1 : checkNormal u1 = true := by native_decide
theorem safe_trans_u3 : checkSafeTrans (u3.filter fun t => !t.isObject) = true := by native_decide
theorem safe_trans_counter : safeTransCounter = true := by native_decide
theorem assoc_counter : assocCounter = true := by native_decide
theorem le_laws_u1 : checkLeLaws (every 4 u1) = true := by native_decide
theorem canon_comm_u2 : checkCanonComm u2 = true := by native_decide
theorem bug27_symm : bug27 = true := by native_decide
theorem s8_output_counter : s8OutputCounter = true := by native_decide

end Formal
