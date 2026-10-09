import Formal.Conv

/-!
# Unification

`unify ts` is the unification function of `README.md` §5. It takes the list of
input types and returns one result type. Each definition below is one rule of
§5 and carries the rule name from the README.
-/

namespace Formal

/-! ## U-Scalar: the scalar order -/

/-- `s` is below `s'` in the scalar order (README §5.3): `s'` converts safely
from `s`, and either the converse fails or `s'` precedes `s` in the total
order (the tie-break keeps `string` over `id`). -/
def below (s s' : Ty) : Bool :=
  s != s' && conv s' s == .safe && (conv s s' != .safe || Ty.le s' s)

/-- U-Scalar: the members of a scalar class that no other member is above. -/
def scalarMax (ss : List Ty) : List Ty :=
  ss.filter fun s => !(ss.any fun s' => below s s')

/-! ## Shape predicates -/

def Ty.isTuple : Ty → Bool | .tuple _ => true | _ => false
def Ty.isSet : Ty → Bool | .set _ => true | _ => false
def Ty.isObject : Ty → Bool | .object _ => true | _ => false
def Ty.isOutput : Ty → Bool | .output _ => true | _ => false
def Ty.isPromise : Ty → Bool | .promise _ => true | _ => false

/-- The element types that a sequence contributes to a list merge. -/
def Ty.seqElems : Ty → List Ty
  | .list t | .set t => [t]
  | .tuple ts => ts
  | _ => []

/-- The value types that a mapping contributes to a map merge. -/
def Ty.mapElems : Ty → List Ty
  | .map t => [t]
  | .object ps => ps.map (·.2)
  | _ => []

def Ty.tupleLen : Ty → Nat
  | .tuple ts => ts.length
  | _ => 0

def Ty.tupleAt (i : Nat) : Ty → Ty
  | .tuple ts => ts.getD i .none
  | _ => .none

def Ty.keys : Ty → List String
  | .object ps => ps.map (·.1)
  | _ => []

def Ty.propOr (t : Ty) (k : String) : Ty :=
  match t with
  | .object ps => Formal.propOr ps k
  | _ => .none

/-- Sorted unique keys of a list of objects. -/
def allKeys (os : List Ty) : List String :=
  let ks := (os.flatMap Ty.keys).mergeSort (· ≤ ·)
  ks.foldr (fun k acc => match acc with
    | k' :: _ => if k = k' then acc else k :: acc
    | [] => [k]) []

/-! ## The unification function -/

/-- `unifyF fuel ts`: the rules of README §5.2. The fuel bounds the recursion
depth; `unify` supplies enough fuel for its inputs (`Laws.lean` checks this on
the bounded universe). -/
def unifyF : Nat → List Ty → Ty
  | 0, _ => .dynamic
  | fuel + 1, ts =>
    match ts with
    | [] => .none                                                       -- U-Empty
    | t :: rest =>
      if rest.all (· = t) then t else                                   -- U-Eq
      let ms := canon ts                                                -- U-Flatten
      let noneList := if ms.contains .none then [.none] else []         -- U-None
      let ms := ms.filter (· != .none)
      if ms.contains .dynamic then mkUnion (noneList ++ [.dynamic]) else -- U-Dynamic
      let core :=
        if ms.any Ty.isOutput || (ms.any Ty.isPromise && ms.any containsOutputs) then
          [mkOutput (unifyF fuel (ms.map stripTop))]                    -- U-Output
        else if ms.any Ty.isPromise then                                -- U-Promise
          [mkPromise (unifyF fuel (ms.map stripTop))]
        else
          seqMerge fuel (ms.filter (·.cls = .sequence))
            ++ mapMerge fuel (ms.filter (·.cls = .mapping))
            ++ scalarMax (ms.filter (·.cls = .scalar))                  -- U-Scalar
      mkUnion (noneList ++ core)                                        -- U-Union
where
  /-- U-Seq: the sequence class merges to one sequence. -/
  seqMerge (fuel : Nat) (ss : List Ty) : List Ty :=
    match ss with
    | [] => []
    | s :: _ =>
      if ss.all Ty.isTuple && ss.all (·.tupleLen = s.tupleLen) then     -- U-Tuple
        [.tuple ((List.range s.tupleLen).map fun i =>
          unifyF fuel (ss.map (Ty.tupleAt i)))]
      else if ss.all Ty.isSet then                                      -- U-Set
        [.set (unifyF fuel (ss.flatMap Ty.seqElems))]
      else                                                              -- U-List
        [.list (unifyF fuel (ss.flatMap Ty.seqElems))]
  /-- U-Map: the mapping class merges to one mapping. -/
  mapMerge (fuel : Nat) (os : List Ty) : List Ty :=
    match os with
    | [] => []
    | _ =>
      if os.all Ty.isObject then                                        -- U-Object
        [.object ((allKeys os).map fun k => (k, unifyF fuel (os.map (·.propOr k))))]
      else                                                              -- U-MapOf
        [.map (unifyF fuel (os.flatMap Ty.mapElems))]

/-- The fuel that `unify` gives `unifyF`: one unit per node of the inputs. -/
def fuelFor (ts : List Ty) : Nat := ts.foldl (fun n t => n + t.size) 0 + 1

/-- README §5: `unify ts` is the common type of the inputs. -/
def unify (ts : List Ty) : Ty := unifyF (fuelFor ts) ts

/-- The binary form. -/
def unify2 (a b : Ty) : Ty := unify [a, b]

end Formal
