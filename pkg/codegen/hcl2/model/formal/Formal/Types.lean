/-!
# The PCL type universe

This file is the type grammar of `README.md` §2 and the conversion kind of §4.
`Norm.lean` adds the union normal form and the helper functions.
-/

namespace Formal

/-- A constant value. The base type of a constant is a function of the value
(`Val.base`): an integral number is an `int`, every other number is a `number`. -/
inductive Val where
  | bool (b : Bool)
  | int (n : Int)
  /-- The decimal text of a number that is not integral, for example `"0.01"`. -/
  | number (s : String)
  | string (s : String)
  deriving DecidableEq, Repr, Ord

/-- README §2: the type grammar. Objects keep their properties sorted by key;
unions are in the normal form of §3 (`mkUnion`); outputs and promises hold
resolved element types (`mkOutput`, `mkPromise`). -/
inductive Ty where
  | bool | int | number | string | id | dynamic | none
  | const (v : Val)
  | enum (token : String) (base : Ty) (values : List Val)
  | list (elem : Ty)
  | set (elem : Ty)
  | map (elem : Ty)
  | tuple (elems : List Ty)
  | object (props : List (String × Ty))
  | union (members : List Ty)
  | output (elem : Ty)
  | promise (elem : Ty)
  deriving Repr

/-- README §4: the conversion kind. `lossy` is `UnsafeConversion` in Go. -/
inductive Kind where
  | no | lossy | safe
  deriving DecidableEq, Repr, Ord

instance : LT Kind := ltOfOrd
instance : LE Kind := leOfOrd
instance : Min Kind := minOfLe
instance : Max Kind := maxOfLe

def Val.base : Val → Ty
  | .bool _ => .bool
  | .int _ => .int
  | .number _ => .number
  | .string _ => .string

end Formal
