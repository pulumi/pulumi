import Formal.Types
import Formal.DecEq

/-!
# Normal forms and helpers

The union normal form of `README.md` §3, the eventual constructors of §2.3,
the resolve functions, and the classes of §5.2.
-/

namespace Formal

/-! ## A total order on types

The order sorts union members so that a union has one spelling. Any total
order works; this one ranks by constructor, then compares children. -/

def Ty.rank : Ty → Nat
  | .bool => 0 | .int => 1 | .number => 2 | .string => 3 | .id => 4 | .dynamic => 5
  | .none => 6 | .const _ => 7 | .enum .. => 8 | .list _ => 9 | .set _ => 10
  | .map _ => 11 | .tuple _ => 12 | .object _ => 13 | .union _ => 14
  | .output _ => 15 | .promise _ => 16

def cmpVals : List Val → List Val → Ordering
  | [], [] => .eq
  | [], _ => .lt
  | _, [] => .gt
  | a :: as, b :: bs => (compare a b).then (cmpVals as bs)

mutual
def Ty.cmp : Ty → Ty → Ordering
  | .const v, .const w => compare v w
  | .enum t b vs, .enum t' b' vs' => (compare t t').then ((Ty.cmp b b').then (cmpVals vs vs'))
  | .list a, .list b => Ty.cmp a b
  | .set a, .set b => Ty.cmp a b
  | .map a, .map b => Ty.cmp a b
  | .output a, .output b => Ty.cmp a b
  | .promise a, .promise b => Ty.cmp a b
  | .tuple as, .tuple bs => cmpList as bs
  | .union as, .union bs => cmpList as bs
  | .object as, .object bs => cmpProps as bs
  | a, b => compare a.rank b.rank
def cmpList : List Ty → List Ty → Ordering
  | [], [] => .eq
  | [], _ => .lt
  | _, [] => .gt
  | a :: as, b :: bs => (Ty.cmp a b).then (cmpList as bs)
def cmpProps : List (String × Ty) → List (String × Ty) → Ordering
  | [], [] => .eq
  | [], _ => .lt
  | _, [] => .gt
  | (k, a) :: as, (l, b) :: bs => (compare k l).then ((Ty.cmp a b).then (cmpProps as bs))
end

instance : Ord Ty := ⟨Ty.cmp⟩

/-- The sort predicate for union members. -/
def Ty.le (a b : Ty) : Bool := Ty.cmp a b != .gt

/-! ## Unions -/

/-- Removes adjacent duplicates from a sorted list. -/
def dedup : List Ty → List Ty
  | a :: b :: rest => if a = b then dedup (b :: rest) else a :: dedup (b :: rest)
  | l => l

/-- The members of a type seen as a union: a union contributes its members,
every other type contributes itself. -/
def Ty.members : Ty → List Ty
  | .union ms => ms
  | t => [t]

/-- The canonical member list: flatten, sort, remove duplicates. -/
def canon (ts : List Ty) : List Ty := dedup ((ts.flatMap Ty.members).mergeSort Ty.le)

/-- README §3: the union constructor. No members gives `none`; one member gives
that member; otherwise the canonical member list. -/
def mkUnion (ts : List Ty) : Ty :=
  match canon ts with
  | [] => .none
  | [t] => t
  | ms => .union ms

/-- `optional(T) = union(T, none)`. -/
def optional (t : Ty) : Ty := mkUnion [t, .none]

/-- Sorts properties by key. The caller supplies unique keys. -/
def mkObject (props : List (String × Ty)) : Ty :=
  .object (props.mergeSort fun a b => a.1 ≤ b.1)

/-! ## Eventuals -/

/-- The size of a property type is below the size of the property list. The
termination checker needs this fact for recursion through object properties. -/
theorem sizeOf_prop_lt {ps : List (String × Ty)} {p : String × Ty} (h : p ∈ ps) :
    sizeOf p.2 < sizeOf ps := by
  have := List.sizeOf_lt_of_mem h
  obtain ⟨k, t⟩ := p
  simp only [Prod.mk.sizeOf_spec] at this
  simp only
  omega

macro "prop_decreasing" : tactic =>
  `(tactic| all_goals (first
      | decreasing_tactic
      | (simp_wf; have := sizeOf_prop_lt ‹_ ∈ _›; simp_all; omega)))

/-- The number of nodes in a type. `unify` uses it as fuel. -/
def Ty.size : Ty → Nat
  | .list t | .set t | .map t | .output t | .promise t => 1 + t.size
  | .enum _ b _ => 1 + b.size
  | .tuple ts => 1 + (ts.map Ty.size).sum
  | .object ps => 1 + (ps.map fun p => p.2.size).sum
  | .union ms => 1 + (ms.map Ty.size).sum
  | _ => 1
decreasing_by prop_decreasing

/-- Removes every `output` and `promise` wrapper at any depth. Constants and
enums are leaves. -/
def resolveOutputs : Ty → Ty
  | .list t => .list (resolveOutputs t)
  | .set t => .set (resolveOutputs t)
  | .map t => .map (resolveOutputs t)
  | .tuple ts => .tuple (ts.map resolveOutputs)
  | .object ps => .object (ps.map fun p => (p.1, resolveOutputs p.2))
  | .union ms => mkUnion (ms.map resolveOutputs)
  | .output t => resolveOutputs t
  | .promise t => resolveOutputs t
  | t => t
decreasing_by prop_decreasing

/-- Removes every `promise` wrapper at any depth and leaves outputs in place. -/
def resolvePromises : Ty → Ty
  | .list t => .list (resolvePromises t)
  | .set t => .set (resolvePromises t)
  | .map t => .map (resolvePromises t)
  | .tuple ts => .tuple (ts.map resolvePromises)
  | .object ps => .object (ps.map fun p => (p.1, resolvePromises p.2))
  | .union ms => mkUnion (ms.map resolvePromises)
  | .output t => .output (resolvePromises t)
  | .promise t => resolvePromises t
  | t => t
decreasing_by prop_decreasing

/-- True when a `promise` occurs at any depth. -/
def containsPromises : Ty → Bool
  | .list t | .set t | .map t | .output t => containsPromises t
  | .tuple ts => (ts.map containsPromises).any id
  | .object ps => (ps.map fun p => containsPromises p.2).any id
  | .union ms => (ms.map containsPromises).any id
  | .promise _ => true
  | _ => false
decreasing_by prop_decreasing

/-- True when an `output` occurs at any depth. -/
def containsOutputs : Ty → Bool
  | .list t | .set t | .map t | .promise t => containsOutputs t
  | .tuple ts => (ts.map containsOutputs).any id
  | .object ps => (ps.map fun p => containsOutputs p.2).any id
  | .union ms => (ms.map containsOutputs).any id
  | .output _ => true
  | _ => false
decreasing_by prop_decreasing

/-- `output(T)` never nests an eventual: the element is resolved. -/
def mkOutput (t : Ty) : Ty := .output (resolveOutputs t)

/-- `promise(T)` never nests a promise; it may hold outputs. -/
def mkPromise (t : Ty) : Ty := .promise (resolvePromises t)

/-- Strips one top-level `output` or `promise` wrapper. -/
def stripTop : Ty → Ty
  | .output t => t
  | .promise t => t
  | t => t

/-! ## Classes -/

/-- The classes that unification merges (README §5.2). -/
inductive Class where
  | none | dynamic | scalar | sequence | mapping | eventual | union
  deriving DecidableEq, Repr

def Ty.cls : Ty → Class
  | .none => .none
  | .dynamic => .dynamic
  | .list _ | .set _ | .tuple _ => .sequence
  | .map _ | .object _ => .mapping
  | .output _ | .promise _ => .eventual
  | .union _ => .union
  | _ => .scalar

/-- The property type of `key` in an object, or `none` when the key is absent. -/
def propOr (ps : List (String × Ty)) (key : String) : Ty :=
  match ps.find? (·.1 = key) with
  | some p => p.2
  | Option.none => .none

end Formal
