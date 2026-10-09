import Formal.Conv

/-!
# Recursive object types: termination

README §12 encodes a recursive object type as a node in a finite graph and
runs the conversion rules with a cycle set of node pairs. This file proves
that the recursion terminates. It uses a reduced grammar that keeps only the
constructs that matter for the argument: a leaf, a list, a map, and a node
reference. The other collection rules recurse on strictly smaller types in
the same way as `list`.

The measure is lexicographic: first the number of pairs over the finite set
`S` of reachable types that are not yet in the cycle set, then the syntactic
size of the two types. A step into a pair with an object node on either side
adds that pair to the set and so decreases the first component. Every other
step keeps the set and decreases the second component.
-/

namespace Formal

/-- The reduced grammar. `ref i` is the object node `i` of the environment. -/
inductive RTy where
  | leaf (n : Nat)
  | list (t : RTy)
  | map (t : RTy)
  | ref (i : Nat)
  deriving DecidableEq, Repr

/-- An environment: node `i` is the property list `Γ[i]`. -/
abbrev Env := List (List (String × RTy))

/-- A type and every type inside it. -/
def subterms : RTy → List RTy
  | .leaf n => [.leaf n]
  | .list t => .list t :: subterms t
  | .map t => .map t :: subterms t
  | .ref i => [.ref i]

/-- `S` is closed under subterms and holds every property type of `Γ`. The
types that a conversion visits all lie in such an `S`, so the pairs that the
cycle set can hold lie in the finite set `allPairs S`. -/
structure Closed (Γ : Env) (S : List RTy) : Prop where
  sub : ∀ t ∈ S, ∀ u ∈ subterms t, u ∈ S
  env : ∀ ps ∈ Γ, ∀ p ∈ ps, p.2 ∈ S

/-- Every pair over `S`. -/
def allPairs {α} (S : List α) : List (α × α) :=
  S.flatMap fun a => S.map fun b => (a, b)

/-- The elements of `all` that are not in `seen`. -/
def unseenOf {α} [DecidableEq α] (all seen : List α) : Nat :=
  (all.filter fun q => decide (q ∉ seen)).length

theorem length_filter_mono {α} (l : List α) (f g : α → Bool) (h : ∀ x, f x = true → g x = true) :
    (l.filter f).length ≤ (l.filter g).length := by
  induction l with
  | nil => simp
  | cons a rest ih =>
    simp only [List.filter_cons]
    by_cases hf : f a = true
    · simp [hf, h a hf]; omega
    · simp only [hf]
      by_cases hg : g a = true
      · simp [hg]; omega
      · simp [hg]; exact ih

/-- Adding a pair that is in `all` and not in `seen` leaves fewer unseen pairs. -/
theorem filter_cons_lt {α} [DecidableEq α] (p : α) :
    ∀ (all seen : List α), p ∈ all → p ∉ seen →
    unseenOf all (p :: seen) < unseenOf all seen
  | [], _, h, _ => by simp at h
  | a :: rest, seen, hmem, hnot => by
    unfold unseenOf
    by_cases hap : a = p
    · subst hap
      rw [List.filter_cons_of_neg (by simp),
        List.filter_cons_of_pos (p := fun q => decide (q ∉ seen)) (l := rest) (decide_eq_true hnot)]
      have hmono := length_filter_mono rest (fun q => decide (q ∉ a :: seen))
        (fun q => decide (q ∉ seen)) (by intro x hx; simp at hx ⊢; exact hx.2)
      simp only [List.length_cons]
      omega
    · have hmem' : p ∈ rest := by
        rcases List.mem_cons.mp hmem with h | h
        · exact absurd h.symm hap
        · exact h
      have ih := filter_cons_lt p rest seen hmem' hnot
      unfold unseenOf at ih
      by_cases hs : a ∈ seen
      · rw [List.filter_cons_of_neg (by simp [hs]), List.filter_cons_of_neg (by simp [hs])]
        exact ih
      · rw [List.filter_cons_of_pos (by simp [hs, hap]), List.filter_cons_of_pos (by simp [hs])]
        simp only [List.length_cons]
        omega

theorem mem_allPairs {α} {S : List α} {a b : α} (ha : a ∈ S) (hb : b ∈ S) : (a, b) ∈ allPairs S := by
  unfold allPairs
  simp [List.mem_flatMap, List.mem_map]
  exact ⟨ha, hb⟩

theorem unseen_lt {α} [DecidableEq α] {S : List α} {seen : List (α × α)} {a b : α} (ha : a ∈ S) (hb : b ∈ S)
    (h : (a, b) ∉ seen) : unseenOf (allPairs S) ((a, b) :: seen) < unseenOf (allPairs S) seen :=
  filter_cons_lt (a, b) (allPairs S) seen (mem_allPairs ha hb) h

theorem sub_list {S : List RTy} {t : RTy} (hc : ∀ t ∈ S, ∀ u ∈ subterms t, u ∈ S)
    (h : RTy.list t ∈ S) : t ∈ S :=
  hc _ h t (by simp [subterms]; cases t <;> simp [subterms])

theorem sub_map {S : List RTy} {t : RTy} (hc : ∀ t ∈ S, ∀ u ∈ subterms t, u ∈ S)
    (h : RTy.map t ∈ S) : t ∈ S :=
  hc _ h t (by simp [subterms]; cases t <;> simp [subterms])

theorem prop_mem {Γ : Env} {S : List RTy} (hc : Closed Γ S) {i : Nat} (hi : i < Γ.length)
    {p : String × RTy} (hp : p ∈ Γ[i]) : p.2 ∈ S :=
  hc.env _ (List.getElem_mem hi) p hp

instance (Γ : Env) (S : List RTy) : Decidable (Closed Γ S) :=
  decidable_of_iff ((∀ t ∈ S, ∀ u ∈ subterms t, u ∈ S) ∧ (∀ ps ∈ Γ, ∀ p ∈ ps, p.2 ∈ S))
    ⟨fun ⟨a, b⟩ => ⟨a, b⟩, fun ⟨a, b⟩ => ⟨a, b⟩⟩

/-- README §12: the conversion rules over a node graph with a cycle set.
`Γ` is the environment, `seen` the pairs in flight. The cycle set registers
every pair with an object node on at least one side (C-Object/object,
C-Object/map, C-Map/object); re-entry is `Safe`. -/
def convR (Γ : Env) (S : List RTy) (hc : Closed Γ S) (seen : List (RTy × RTy))
    (dst src : RTy) (hd : dst ∈ S) (hs : src ∈ S) : Kind :=
  match dst, src with
  | .leaf n, .leaf m => if n = m then .safe else .no
  | .list a, .list b => convR Γ S hc seen a b (sub_list hc.sub hd) (sub_list hc.sub hs)
  | .map a, .map b => convR Γ S hc seen a b (sub_map hc.sub hd) (sub_map hc.sub hs)
  | .ref i, .ref j =>                                                  -- C-Object/object
    if h : (RTy.ref i, RTy.ref j) ∈ seen then .safe
    else if hij : i < Γ.length ∧ j < Γ.length then
      Kind.minAll (Γ[i].attach.map fun ⟨p, hp⟩ =>
        match hq : Γ[j].find? (fun q : String × RTy => q.1 = p.1) with
        | some q =>
          convR Γ S hc ((RTy.ref i, RTy.ref j) :: seen) p.2 q.2
            (prop_mem hc hij.1 hp) (prop_mem hc hij.2 (List.mem_of_find?_eq_some hq))
        | Option.none => .no)
    else .no
  | .ref i, .map b =>                                                  -- C-Object/map
    if h : (RTy.ref i, RTy.map b) ∈ seen then .safe
    else if hi : i < Γ.length then
      (Kind.minAll (Γ[i].attach.map fun ⟨p, hp⟩ =>
        convR Γ S hc ((RTy.ref i, RTy.map b) :: seen) p.2 b
          (prop_mem hc hi hp) (sub_map hc.sub hs))).cap
    else .no
  | .map a, .ref j =>                                                  -- C-Map/object
    if h : (RTy.map a, RTy.ref j) ∈ seen then .safe
    else if hj : j < Γ.length then
      Kind.minAll (Γ[j].attach.map fun ⟨q, hq⟩ =>
        convR Γ S hc ((RTy.map a, RTy.ref j) :: seen) a q.2
          (sub_map hc.sub hd) (prop_mem hc hj hq))
    else .no
  | _, _ => .no
termination_by (unseenOf (allPairs S) seen, sizeOf dst + sizeOf src)
decreasing_by
  all_goals simp_wf
  all_goals first
    | (right; omega)
    | (left; apply unseen_lt <;> assumption)

/-- The closure of the environment and two types under subterms. -/
def closure (Γ : Env) (dst src : RTy) : List RTy :=
  (Γ.flatMap fun ps => ps.flatMap fun p => subterms p.2) ++ subterms dst ++ subterms src

/-- A conversion check with an empty cycle set. The caller proves that
`closure` is closed; `by decide` does it for a concrete environment. -/
def convRec (Γ : Env) (dst src : RTy) (hc : Closed Γ (closure Γ dst src)) : Kind :=
  convR Γ _ hc [] dst src (by simp [closure]; cases dst <;> simp [subterms])
    (by simp [closure]; cases src <;> simp [subterms])

/-- Example: `a = {x: list(a)}`, `b = {x: list(b)}`, `c = {x: list(c), y: int}`:
`a` and `b` convert; `c` converts to `a` and not the converse. -/
def exEnv : Env := [[("x", .list (.ref 0))], [("x", .list (.ref 1))], [("x", .list (.ref 2)), ("y", .leaf 0)]]
example : convRec exEnv (.ref 0) (.ref 1) (by decide) = .safe := by native_decide
example : convRec exEnv (.ref 0) (.ref 2) (by decide) = .safe := by native_decide
example : convRec exEnv (.ref 2) (.ref 0) (by decide) = .no := by native_decide

/-- Example: `a = {k: map(a)}`. `conv(a, map(a))` terminates and is `Unsafe`:
C-Object/map registers `(a, map(a))`, C-Map/object registers `(map(a), a)`,
and the next C-Object/map re-enters the first pair. `conv(map(a), a)` is
`Unsafe` for the same reason: its inner C-Object/map step caps the kind. -/
def mapEnv : Env := [[("k", .map (.ref 0))]]
example : convRec mapEnv (.ref 0) (.map (.ref 0)) (by decide) = .lossy := by native_decide
example : convRec mapEnv (.map (.ref 0)) (.ref 0) (by decide) = .lossy := by native_decide

end Formal
