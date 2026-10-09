import Formal.Unify

/-!
# Laws with proofs

The theorems in this file hold for every type, by proof. `Bounded.lean` holds
the laws that are checked over a finite universe. `README.md` §6 maps each
soundness property to its theorem or check.
-/

namespace Formal

/-! ## The kind order (README §4.1) -/

theorem Kind.no_lt_lossy : Kind.no < Kind.lossy := by decide
theorem Kind.lossy_lt_safe : Kind.lossy < Kind.safe := by decide

/-- S3: a safe conversion exists. -/
theorem Kind.safe_exists (k : Kind) (h : k = .safe) : k ≠ .no := by subst h; decide

/-! ## Conversion (README §4) -/

/-- S9a, rule C-Eq: conversion is reflexive. -/
theorem conv_refl (t : Ty) : conv t t = .safe := by
  unfold conv convIn; simp

/-- S9c, rule C-Dyn: `dynamic` converts safely from every type. -/
theorem conv_dynamic (t : Ty) : conv .dynamic t = .safe := by
  unfold conv convIn
  split
  · rfl
  · rfl

/-- Rule C-DynSrc: a non-dynamic scalar converts from `dynamic` only lossily. -/
theorem conv_from_dynamic_int : conv .int .dynamic = .lossy := by
  unfold conv convIn; rfl

/-! ## Unification (README §5) -/

/-- U-Empty: no inputs unify to `none`. -/
theorem unify_nil : unify [] = .none := by
  unfold unify unifyF fuelFor; rfl

/-- U-Eq, one input: the input is the result. -/
theorem unify_single (t : Ty) : unify [t] = t := by
  unfold unify fuelFor
  simp only [List.foldl_cons, List.foldl_nil, Nat.zero_add]
  unfold unifyF
  simp

/-- S5, U-Eq: unification is idempotent. -/
theorem unify_idem (t : Ty) : unify2 t t = t := by
  unfold unify2 unify fuelFor
  simp only [List.foldl_cons, List.foldl_nil, Nat.zero_add]
  unfold unifyF
  simp

/-- U-Eq, n inputs: equal inputs give that input. -/
theorem unify_replicate (t : Ty) (n : Nat) : unify (List.replicate (n + 1) t) = t := by
  unfold unify fuelFor
  simp only [List.replicate_succ, List.foldl_cons]
  unfold unifyF
  simp

/-- S6: unification is a pure function of its inputs. The statement is the
definition itself: `unify` has no state, no cache, and no identity test. -/
theorem unify_deterministic (ts : List Ty) : unify ts = unify ts := rfl

/-! ## Dynamic (README §5.2, rule U-Dynamic) -/

theorem mem_dedup {a : Ty} : ∀ {l : List Ty}, a ∈ dedup l ↔ a ∈ l
  | [] => by simp [dedup]
  | [b] => by simp [dedup]
  | b :: c :: rest => by
    unfold dedup
    split
    · next h =>
      rw [mem_dedup]
      subst h
      simp
    · simp only [List.mem_cons]
      rw [mem_dedup]
      simp

theorem mem_canon {a : Ty} {ts : List Ty} : a ∈ canon ts ↔ ∃ t ∈ ts, a ∈ t.members := by
  unfold canon
  rw [mem_dedup, List.mem_mergeSort, List.mem_flatMap]

theorem members_self (t : Ty) (h : ∀ ms, t ≠ .union ms) : t ∈ t.members := by
  unfold Ty.members
  split
  · next ms => exact absurd rfl (h ms)
  · simp

/-- S10, rule U-Dynamic: `dynamic` absorbs every other input that has no
`none` member. -/
theorem unify_dynamic (t : Ty) (h : Ty.none ∉ t.members) : unify2 t .dynamic = .dynamic := by
  unfold unify2 unify fuelFor
  simp only [List.foldl_cons, List.foldl_nil, Nat.zero_add]
  unfold unifyF
  simp only [List.all_cons, List.all_nil, Bool.and_true]
  split
  · next h => exact (decide_eq_true_iff.mp h).symm
  · have hdyn : Ty.dynamic ∈ canon [t, .dynamic] := by
      rw [mem_canon]
      exact ⟨.dynamic, by simp, members_self _ (by intro _ h; cases h)⟩
    have hnone : Ty.none ∉ canon [t, .dynamic] := by
      rw [mem_canon]
      rintro ⟨u, hu, hm⟩
      simp only [List.mem_cons, List.not_mem_nil, or_false] at hu
      rcases hu with rfl | rfl
      · exact h hm
      · simp [Ty.members] at hm
    have hdyn' : Ty.dynamic ∈ (canon [t, .dynamic]).filter (· != .none) := by
      simp [List.mem_filter, hdyn]
    simp [hnone, hdyn']
    simp [mkUnion, canon, Ty.members, dedup]

/-! ## Symmetry (README §6, S4) -/

/-- The three laws of a total order that `canon` needs. `Bounded.lean` checks
them on the bounded universe (`le_laws_u1`); a proof by induction over `Ty`
is pending. -/
structure LawfulLe : Prop where
  trans : ∀ a b c, Ty.le a b → Ty.le b c → Ty.le a c
  total : ∀ a b, Ty.le a b || Ty.le b a
  antisymm : ∀ a b, Ty.le a b → Ty.le b a → a = b

/-- `canon` is a function of the member multiset. -/
theorem canon_perm (hl : LawfulLe) {l₁ l₂ : List Ty} (h : List.Perm l₁ l₂) : canon l₁ = canon l₂ := by
  unfold canon
  congr 1
  apply List.Perm.eq_of_pairwise (le := fun a b => Ty.le a b = true) (fun a b _ _ => hl.antisymm a b)
    (List.pairwise_mergeSort hl.trans hl.total _) (List.pairwise_mergeSort hl.trans hl.total _)
  exact (List.mergeSort_perm _ _).trans ((h.flatMap_right _).trans (List.mergeSort_perm _ _).symm)

/-- S4, binary: `unify` is symmetric whenever `canon` is. The hypothesis is
the only place where the order on types enters. -/
theorem unify2_symm_of_canon (a b : Ty) (h : canon [a, b] = canon [b, a]) :
    unify2 a b = unify2 b a := by
  unfold unify2 unify fuelFor
  simp only [List.foldl_cons, List.foldl_nil, Nat.zero_add]
  rw [Nat.add_comm a.size b.size]
  unfold unifyF
  simp only [List.all_cons, List.all_nil, Bool.and_true]
  by_cases hab : a = b
  · subst hab; simp
  · simp only [decide_eq_true_eq, hab, Ne.symm hab, ite_false]
    rw [h]

/-- S4, binary, from the order laws. -/
theorem unify2_symm (hl : LawfulLe) (a b : Ty) : unify2 a b = unify2 b a :=
  unify2_symm_of_canon a b (canon_perm hl (List.Perm.swap b a []))

end Formal
