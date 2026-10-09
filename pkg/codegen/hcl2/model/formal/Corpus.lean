import Formal.Bounded

/-!
# Golden corpus

`lake exe corpus > corpus.json` prints a JSON array. Each entry holds two
types `a` and `b`, the conversion kinds in both directions, and the
unification result (`safe`; `unsafe` is the same type, see README §5.1).
`formal/README.md` gives the type encoding.
-/

namespace Formal

def jsonString (s : String) : String :=
  "\"" ++ (s.foldl (fun acc c =>
    acc ++ (if c = '"' then "\\\"" else if c = '\\' then "\\\\" else c.toString)) "") ++ "\""

def Val.toJson : Val → String
  | .bool b => s!"\{\"bool\":{b}}"
  | .int n => s!"\{\"int\":{n}}"
  | .number s => s!"\{\"number\":{jsonString s}}"
  | .string s => s!"\{\"string\":{jsonString s}}"

def joinComma (xs : List String) : String := String.intercalate "," xs

def Ty.toJson : Ty → String
  | .bool => "{\"kind\":\"bool\"}"
  | .int => "{\"kind\":\"int\"}"
  | .number => "{\"kind\":\"number\"}"
  | .string => "{\"kind\":\"string\"}"
  | .id => "{\"kind\":\"id\"}"
  | .dynamic => "{\"kind\":\"dynamic\"}"
  | .none => "{\"kind\":\"none\"}"
  | .const v => s!"\{\"kind\":\"const\",\"value\":{v.toJson}}"
  | .enum tok b vs =>
    s!"\{\"kind\":\"enum\",\"token\":{jsonString tok},\"base\":{b.toJson},\"values\":[{joinComma (vs.map Val.toJson)}]}"
  | .list t => s!"\{\"kind\":\"list\",\"elem\":{t.toJson}}"
  | .set t => s!"\{\"kind\":\"set\",\"elem\":{t.toJson}}"
  | .map t => s!"\{\"kind\":\"map\",\"elem\":{t.toJson}}"
  | .output t => s!"\{\"kind\":\"output\",\"elem\":{t.toJson}}"
  | .promise t => s!"\{\"kind\":\"promise\",\"elem\":{t.toJson}}"
  | .tuple ts => s!"\{\"kind\":\"tuple\",\"elems\":[{joinComma (ts.map Ty.toJson)}]}"
  | .object ps =>
    s!"\{\"kind\":\"object\",\"props\":\{{joinComma (ps.map fun p => jsonString p.1 ++ ":" ++ p.2.toJson)}}}"
  | .union ms => s!"\{\"kind\":\"union\",\"members\":[{joinComma (ms.map Ty.toJson)}]}"
decreasing_by prop_decreasing

def Kind.toJson : Kind → String
  | .no => "\"no\""
  | .lossy => "\"unsafe\""
  | .safe => "\"safe\""

/-- The corpus universe: the leaves, the triple universe, and a sample of `u1`. -/
def corpusTypes : List Ty := dedup ((leaves ++ u3 ++ every 40 u1).mergeSort Ty.le)

def entry (a b : Ty) : String :=
  let r := unify2 a b
  s!"\{\"a\":{a.toJson},\"b\":{b.toJson},\"conv_ab\":{(conv a b).toJson},\"conv_ba\":{(conv b a).toJson},\"safe\":{r.toJson},\"unsafe\":{r.toJson}}"

end Formal

open Formal in
def main : IO Unit := do
  let out ← IO.getStdout
  out.putStrLn "["
  let entries := corpusTypes.flatMap fun a => corpusTypes.map fun b => entry a b
  out.putStrLn (String.intercalate ",\n" entries)
  out.putStrLn "]"
