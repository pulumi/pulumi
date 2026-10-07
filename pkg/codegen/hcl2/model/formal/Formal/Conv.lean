import Formal.Norm

/-!
# Conversion

`conv dst src` is the conversion relation of `README.md` §4. Each match arm is
one rule of §4 and carries the rule name from the README. The first arm that
matches wins.
-/

namespace Formal

/-- Lifts the three-valued kind to a list of member kinds for a union source
(rule C-USrc): `safe` when every member is safe, `no` when no member converts,
`lossy` otherwise. -/
def Kind.ofMembers (ks : List Kind) : Kind :=
  if ks.all (· = .safe) then .safe
  else if ks.any (· ≠ .no) then .lossy
  else .no

/-- The least kind in a list; `safe` for the empty list. -/
def Kind.minAll (ks : List Kind) : Kind := ks.foldl min .safe

/-- The greatest kind in a list; `no` for the empty list. -/
def Kind.maxAll (ks : List Kind) : Kind := ks.foldl max .no

/-- Caps a kind at `lossy`. -/
def Kind.cap (k : Kind) : Kind := min .lossy k

/-- `lossy` when the condition holds, else `no`. -/
def Kind.lossyIf (b : Bool) : Kind := if b then .lossy else .no

/-- The eventual context of a conversion. Rule C-Out resolves every output and
promise in the source; rule C-Prom resolves every promise. The context applies
that resolution lazily, one wrapper at a time, so that the recursion stays
structural. -/
inductive Ctx where
  | plain | inOutput | inPromise
  deriving DecidableEq, Repr

/-- The size of a zipped pair is below the sizes of the zipped lists. -/
theorem sizeOf_zip_lt {as bs : List Ty} {p : Ty × Ty} (h : p ∈ as.zip bs) :
    sizeOf p.1 < sizeOf as ∧ sizeOf p.2 < sizeOf bs := by
  have ⟨h1, h2⟩ := List.of_mem_zip h
  exact ⟨List.sizeOf_lt_of_mem h1, List.sizeOf_lt_of_mem h2⟩

/-- A looked-up property type is no larger than the property list. -/
theorem sizeOf_propOr_le (ps : List (String × Ty)) (k : String) :
    sizeOf (propOr ps k) ≤ sizeOf ps := by
  unfold propOr
  split
  · next p h =>
    have := sizeOf_prop_lt (List.mem_of_find?_eq_some h)
    omega
  · cases ps <;> simp <;> omega

theorem sizeOf_base (v : Val) : sizeOf v.base = 1 := by cases v <;> rfl

theorem Val.sizeOf_pos (v : Val) : 0 < sizeOf v := by cases v <;> simp <;> omega

/-- The termination measure goes down through the C-Object rule. -/
theorem object_dec {ps qs : List (String × Ty)} {p : String × Ty} (h : p ∈ ps) :
    sizeOf p.2 + sizeOf (propOr qs p.1) < sizeOf (Ty.object ps) + sizeOf (Ty.object qs) := by
  have := sizeOf_prop_lt h
  have := sizeOf_propOr_le qs p.1
  simp only [Ty.object.sizeOf_spec]
  omega

/-- Discharges every termination goal of `convIn`: the sum of the sizes of the
destination and the source goes down on every recursive call. -/
macro "conv_decreasing" : tactic =>
  `(tactic| all_goals first
      | exact object_dec ‹_ ∈ (_ : List (String × Ty))›
      | (simp_wf
         try simp only [sizeOf_base]
         try have := Val.sizeOf_pos ‹Val›
         try have := List.sizeOf_lt_of_mem ‹_ ∈ (_ : List Ty)›
         try have := sizeOf_prop_lt ‹_ ∈ (_ : List (String × Ty))›
         try have := sizeOf_zip_lt ‹_ ∈ List.zip (_ : List Ty) _›
         try omega))

/-- `convIn ctx dst src`: the kind of conversion from `src` to `dst` under an
eventual context. `conv` below fixes the context to `plain`. -/
def convIn (ctx : Ctx) (dst src : Ty) : Kind :=
  if dst = src then .safe else                                          -- C-Eq
  match ctx, dst, src with
  | .inOutput, dst, .output u => convIn ctx dst u                       -- C-Out (lazy)
  | .inOutput, dst, .promise u => convIn ctx dst u                      -- C-Out (lazy)
  | .inPromise, dst, .promise u => convIn ctx dst u                     -- C-Prom (lazy)
  | _, .dynamic, _ => .safe                                                -- C-Dyn
  | _, dst, .union ms => Kind.ofMembers (ms.map (convIn ctx dst))          -- C-USrc
  | _, .union ms, src => Kind.maxAll (ms.map (convIn ctx · src))           -- C-UDst
  | _, .output t, src => convIn .inOutput t src                            -- C-Out
  | _, .promise _, .output _ => .no                                          -- C-Prom
  | _, .promise t, src => convIn .inPromise t src                           -- C-Prom
  | _, _, .dynamic => .lossy                                               -- C-DynSrc
  | _, .const v, .const w => if v = w then .safe else .no                  -- C-Const
  | _, .const v, src => Kind.lossyIf (convIn ctx v.base src ≠ .no)         -- C-ConstSrc
  | _, .enum _ _ vs, .const v => if vs.contains v then .safe else .no      -- C-EnumConst
  | _, .enum _ b _, src => Kind.lossyIf (convIn ctx b src ≠ .no)           -- C-EnumSrc
  | _, dst, .const v => convIn ctx dst v.base                              -- C-Widen
  | _, dst, .enum _ b _ => convIn ctx dst b                                -- C-Widen
  | _, .number, .int => .safe                                              -- C-Scalar
  | _, .string, .bool | _, .string, .int | _, .string, .number | _, .string, .id => .safe
  | _, .id, .bool | _, .id, .int | _, .id, .number | _, .id, .string => .safe
  | _, .bool, .int | _, .bool, .number | _, .bool, .string | _, .bool, .id => .lossy
  | _, .int, .bool | _, .int, .number | _, .int, .string | _, .int, .id => .lossy
  | _, .number, .bool | _, .number, .string | _, .number, .id => .lossy
  | _, .list t, .list u => convIn ctx t u                                  -- C-List
  | _, .list t, .set u => convIn ctx t u
  | _, .list t, .tuple us => Kind.minAll (us.map (convIn ctx t))
  | _, .set t, .set u => convIn ctx t u                                    -- C-Set
  | _, .set t, .list u => (convIn ctx t u).cap
  | _, .set t, .tuple us => (Kind.minAll (us.map (convIn ctx t))).cap
  | _, .map t, .map u => convIn ctx t u                                    -- C-Map
  | _, .map t, .object qs => Kind.minAll (qs.map fun q => convIn ctx t q.2)
  | _, .tuple ts, .tuple us =>                                             -- C-Tuple
    if ts.length = us.length
    then Kind.minAll ((ts.zip us).attach.map fun ⟨p, _⟩ => convIn ctx p.1 p.2)
    else .no
  | _, .tuple ts, .list u => (Kind.minAll (ts.map (convIn ctx · u))).cap
  | _, .tuple ts, .set u => (Kind.minAll (ts.map (convIn ctx · u))).cap
  | _, .object ps, .object qs =>                                           -- C-Object
    Kind.minAll (ps.attach.map fun ⟨p, _⟩ => convIn ctx p.2 (propOr qs p.1))
  | _, .object ps, .map u => (Kind.minAll (ps.map fun p => convIn ctx p.2 u)).cap
  | _, _, _ => .no                                                         -- C-No
termination_by sizeOf dst + sizeOf src
decreasing_by conv_decreasing

/-- README §4: `conv dst src` is the kind of conversion from `src` to `dst`. -/
def conv (dst src : Ty) : Kind := convIn .plain dst src

end Formal
